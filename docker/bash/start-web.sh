#!/bin/sh

set -eu

WEB_ROOT=/var/www/goteams/web
ARCHIVE_FILE="$WEB_ROOT/dist.tar.gz"
DIST_DIR="$WEB_ROOT/dist"
TEMP_DIR="$WEB_ROOT/.dist-unpacking"

# 归档内文件清单、dist 当前文件清单、暂存的 index.html：均放在 dist 之外，避免被清理逻辑当成新增文件
ARCHIVE_LIST="$WEB_ROOT/.dist-archive-files"
DIST_LIST="$WEB_ROOT/.dist-current-files"
EXTRA_LIST="$WEB_ROOT/.dist-extra-files"
PENDING_INDEX="$WEB_ROOT/.dist-index.html"
DEPLOY_FAILURE_MARKER="$WEB_ROOT/.dist-deploy-failed"

# 归档外文件的保留时长（天）：不在本次归档中且创建时间超过该时长的文件才清理，
# 给仍持有旧 index.html 的浏览器/CDN 留出加载旧 chunk 的缓冲期
STALE_DAYS=2

cleanup_deploy_files() {
    rm -rf "$TEMP_DIR" "$ARCHIVE_LIST" "$DIST_LIST" "$EXTRA_LIST" "$PENDING_INDEX"
}

# 新资源落地失败时优先维持旧站可用，同时写入宿主可见标记，让 scripts/publish.sh 判定本次发布失败。
fallback_to_existing_dist() {
    failure_reason="$1"
    shift

    if ! printf '%s\n' "$failure_reason" >"$DEPLOY_FAILURE_MARKER"; then
        echo "前端部署失败标记写入失败：$DEPLOY_FAILURE_MARKER" >&2
    fi
    if ! cleanup_deploy_files; then
        echo "前端部署临时文件清理失败，请人工检查：$WEB_ROOT/.dist-*" >&2
    fi

    echo "$failure_reason" >&2
    if [ -f "$DIST_DIR/index.html" ]; then
        echo "前端部署失败，继续启动 Nginx 提供上一版本；scripts/publish.sh 将判定本次发布失败，需人工处理。" >&2
        exec /docker-entrypoint.sh "$@"
    fi

    echo "前端部署失败且不存在可用的旧版 index.html，无法启动站点。" >&2
    exit 1
}

if [ ! -f "$ARCHIVE_FILE" ]; then
    echo "前端产物不存在：$ARCHIVE_FILE" >&2
    exit 1
fi

now_ts="$(date +%s)"
stale_seconds=$((STALE_DAYS * 86400))

# 清理上次异常中断留下的残留及失败标记，保证本次从干净状态开始
if ! cleanup_deploy_files || ! rm -f "$DEPLOY_FAILURE_MARKER"; then
    echo "前端部署残留清理失败：$WEB_ROOT/.dist-*" >&2
    exit 1
fi
mkdir -p "$TEMP_DIR"

if ! tar -xzf "$ARCHIVE_FILE" -C "$TEMP_DIR"; then
    rm -rf "$TEMP_DIR"
    echo "前端产物解压失败：$ARCHIVE_FILE" >&2
    exit 1
fi

if [ ! -f "$TEMP_DIR/index.html" ]; then
    rm -rf "$TEMP_DIR"
    echo "前端产物不完整：归档中缺少 index.html" >&2
    exit 1
fi

# 归档文件清单（相对路径）：覆盖部署后据此识别 dist 中归档外的文件
(cd "$TEMP_DIR" && find . -type f | sed 's|^\./||' | LC_ALL=C sort) >"$ARCHIVE_LIST"

mkdir -p "$DIST_DIR"

# index.html 最后原子替换：资源全部就位后入口页才切换，避免部署中途读到引用缺失 chunk 的半成品
mv "$TEMP_DIR/index.html" "$PENDING_INDEX"

# 覆盖式部署：归档内同名文件覆盖、新增文件写入，归档外的文件保持不动。
# 不保留归档中的 owner/group/mtime，避免无业务收益的元数据复制扩大失败面。
if ! cp -R "$TEMP_DIR/." "$DIST_DIR/"; then
    fallback_to_existing_dist "前端产物覆盖部署失败：$TEMP_DIR -> $DIST_DIR" "$@"
fi

# index.html 若被异常创建为目录，固定范围删除后自动修复，避免 mv 将入口文件搬进目录却返回成功。
if [ -d "$DIST_DIR/index.html" ]; then
    echo "检测到异常的 index.html 目录，删除后重新落地入口文件：$DIST_DIR/index.html" >&2
    if ! rm -rf "$DIST_DIR/index.html"; then
        fallback_to_existing_dist "异常的 index.html 目录删除失败：$DIST_DIR/index.html" "$@"
    fi
fi
if ! mv "$PENDING_INDEX" "$DIST_DIR/index.html"; then
    fallback_to_existing_dist "前端入口文件落地失败：$PENDING_INDEX -> $DIST_DIR/index.html" "$@"
fi
if [ ! -f "$DIST_DIR/index.html" ]; then
    fallback_to_existing_dist "前端入口文件落地校验失败：$DIST_DIR/index.html 不是普通文件" "$@"
fi

# 残留清理：仅针对「不在本次归档中」且「创建时间超过 STALE_DAYS 天」的文件
(cd "$DIST_DIR" && find . -type f | sed 's|^\./||' | LC_ALL=C sort) >"$DIST_LIST"
LC_ALL=C comm -23 "$DIST_LIST" "$ARCHIVE_LIST" >"$EXTRA_LIST"

removed_count=0
kept_count=0
while IFS= read -r rel_path; do
    [ -n "${rel_path}" ] || continue
    stale_file="$DIST_DIR/${rel_path}"
    [ -f "${stale_file}" ] || continue

    # %W 为创建时间（birth time），文件系统不支持时返回 0；此时退回 %Z（ctime）。
    # ctime 恒不早于创建时间，只会延后清理，不会提前误删。
    created_ts="$(stat -c %W "${stale_file}" 2>/dev/null || echo 0)"
    case "${created_ts}" in
    '' | *[!0-9]*) created_ts=0 ;;
    esac
    [ "${created_ts}" -gt 0 ] || created_ts="$(stat -c %Z "${stale_file}" 2>/dev/null || echo "${now_ts}")"

    if [ $((now_ts - created_ts)) -lt "${stale_seconds}" ]; then
        kept_count=$((kept_count + 1))
        continue
    fi

    if rm -f "${stale_file}"; then
        echo "清理残留文件（不在归档中且早于 ${STALE_DAYS} 天）：${rel_path}"
        removed_count=$((removed_count + 1))
    else
        echo "清理残留文件失败：${rel_path}" >&2
    fi
done <"$EXTRA_LIST"

# 清理残留文件后回收空目录（mindepth 1 保证 dist 自身不会被删）
if ! find "$DIST_DIR" -mindepth 1 -type d -empty -delete; then
    echo "前端部署空目录清理失败，请人工检查：$DIST_DIR" >&2
fi

archive_file_count="$(grep -c . "$ARCHIVE_LIST" || true)"
extra_file_count="$(grep -c . "$EXTRA_LIST" || true)"
if ! cleanup_deploy_files; then
    echo "前端部署临时文件清理失败，请人工检查：$WEB_ROOT/.dist-*" >&2
fi

echo "前端产物已覆盖部署到：$DIST_DIR"
echo "$(date '+%Y-%m-%d %H:%M:%S') 归档文件 ${archive_file_count} 个；归档外文件 ${extra_file_count} 个，清理 ${removed_count} 个，保留 ${kept_count} 个（不足 ${STALE_DAYS} 天）"

# 交还给镜像原入口，继续执行模板渲染和 Nginx 启动流程。
exec /docker-entrypoint.sh "$@"
