#!/bin/sh

set -eu

# 开源版：goteamsctl 安装/升级时已把 dist.tar.gz 解压为 dist/（release 目录只读挂载）。
# 这里只做产物完整性校验，校验通过后交还镜像原入口启动 Nginx。
DIST_DIR=/var/www/goteams/web/dist

if [ ! -f "$DIST_DIR/index.html" ]; then
    echo "前端产物缺失：$DIST_DIR/index.html（应由 goteamsctl 在安装/升级时解压）" >&2
    exit 1
fi

exec /docker-entrypoint.sh "$@"
