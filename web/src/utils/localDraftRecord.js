// 本地草稿的纯规则层：不访问浏览器 API，便于脱离浏览器做单元检查。
// 读写与调度在 composables/useLocalDraft.js 中完成。

// 停止输入后多久触发保存（ms）
export const SAVE_DELAY = 350;
// 距上一草稿版本超过此时长才新增版本（ms）
export const VERSION_INTERVAL = 3 * 60 * 1000;
// 每条草稿最多保留的版本数
export const MAX_VERSIONS = 5;
// 最后保存后保留 3 天，超时整条删除（ms）
export const EXPIRE_AFTER = 3 * 24 * 60 * 60 * 1000;
// 记录结构版本；不兼容时直接删除记录（数据最长只保留 3 天，不做旧结构迁移）
export const DRAFT_SCHEMA_VERSION = 1;

/** 生成创建草稿会话 ID；HTTP 环境没有 randomUUID 时使用非安全随机后备。 */
export function createDraftSessionId(cryptoApi = globalThis.crypto) {
  return (
    cryptoApi?.randomUUID?.() ||
    `${Date.now()}-${Math.random().toString(36).slice(2)}`
  );
}

/** 创建态 key：不含工作空间和类型，便于切换范围时同 key 原地迁移 */
export function buildCreateKey(userId, sessionId) {
  return `draft-create-${userId}-${sessionId}`;
}

/** 详情 key：工作项类型不可变，因此不需要再加入类型 */
export function buildItemKey(userId, wsId, workItemId) {
  return `draft-item-${userId}-${wsId}-${workItemId}`;
}

/** 是否为当前版本可读取的草稿记录；不兼容记录由存储层删除 */
export function isCompatibleRecord(record) {
  if (!record || typeof record !== "object") return false;
  if (record.schemaVersion !== DRAFT_SCHEMA_VERSION) return false;
  if (
    typeof record.key !== "string" ||
    (record.kind !== "create" && record.kind !== "work-item") ||
    typeof record.userId !== "string" ||
    typeof record.wsId !== "string" ||
    typeof record.type !== "string" ||
    typeof record.targetId !== "string" ||
    typeof record.title !== "string" ||
    typeof record.html !== "string" ||
    !Number.isFinite(record.savedAt) ||
    record.savedAt <= 0 ||
    typeof record.isPosted !== "boolean" ||
    !Array.isArray(record.versions)
  ) {
    return false;
  }
  return record.versions.every(
    (version) =>
      version &&
      typeof version.title === "string" &&
      typeof version.html === "string" &&
      Number.isFinite(version.savedAt) &&
      version.savedAt > 0,
  );
}

/**
 * 应用一次内容保存。
 * @param {object|null} record 现有记录（null 表示首次）
 * @param {object} meta 包含 key/kind/userId/wsId/type/targetId/title/html/mode
 * @param {number} now 当前时间（ms）
 * @returns {object|null} 新记录；内容无变化或初始化空内容时返回 null（不写入）
 */
export function applySave(record, meta, now = Date.now()) {
  const title = meta.title ?? "";
  const html = meta.html ?? "";
  // 编辑器模式随内容一起缓存，恢复时才能回到富文本/markdown 原模式
  const mode = meta.mode || "";
  if (!record) {
    // 初始化空表单不创建草稿
    if (!title && !html) return null;
    return {
      schemaVersion: DRAFT_SCHEMA_VERSION,
      key: meta.key,
      kind: meta.kind,
      userId: String(meta.userId ?? ""),
      wsId: String(meta.wsId ?? ""),
      type: meta.type || "",
      targetId: String(meta.targetId ?? ""),
      title,
      html,
      mode,
      savedAt: now,
      isPosted: false,
      // TAPD 首次保存只建立最新内容，不产生历史子版本
      versions: [],
    };
  }
  // 标题和正文都没变 → 不写
  if (record.title === title && record.html === html) return null;
  return {
    ...record,
    title,
    html,
    mode,
    savedAt: now,
    isPosted: false,
    versions: appendVersion(record, { title, html, mode }, now),
  };
}

// 已有记录但没有历史版本时建立第一个版本；之后按 3 分钟切分，最多保留 5 个
function appendVersion(record, { title, html, mode }, now) {
  const versions = Array.isArray(record.versions)
    ? record.versions.slice()
    : [];
  const last = versions[versions.length - 1];
  if (!last || now - last.savedAt > VERSION_INTERVAL) {
    versions.push({ title, html, mode: mode || "", savedAt: now });
    if (versions.length > MAX_VERSIONS) versions.shift();
  }
  return versions;
}

/**
 * 标记该记录不再参与自动恢复：用户点了取消（视为正常关闭），或已自动恢复过一次。
 * 两种情形效果相同，因此共用一个标记。已标记时返回 null（不写入）。
 */
export function applySkipAutoRestore(record) {
  if (!record || record.skipAutoRestore) return null;
  return { ...record, skipAutoRestore: true };
}

/**
 * 从可见草稿（已按 savedAt 倒序）中挑出可自动恢复的一条：上次异常关闭遗留的最新内容。
 * 只认创建草稿，详情页草稿属于已存在的工作项，回填到新建表单会串内容。
 */
export function pickAutoRestorable(records) {
  return (
    (records || []).find(
      (record) =>
        record.kind === "create" && !record.isPosted && !record.skipAutoRestore,
    ) || null
  );
}

/** 标记已提交；不刷新 savedAt。已经是已提交时返回 null */
export function applyPosted(record) {
  if (!record || record.isPosted) return null;
  return { ...record, isPosted: true };
}

/** 创建页切换工作空间/类型：同 key 原地更新范围。范围没变或记录不存在时返回 null */
export function applyRetarget(record, { wsId, type }) {
  if (!record) return null;
  const nextWsId = String(wsId ?? "");
  const nextType = type || "";
  if (String(record.wsId) === nextWsId && record.type === nextType) {
    return null;
  }
  return { ...record, wsId: nextWsId, type: nextType };
}

/** 整条草稿是否过期（从最后一次保存起算，版本不单独过期） */
export function isExpired(record, now = Date.now()) {
  const savedAt = Number(record?.savedAt);
  return !Number.isFinite(savedAt) || now - savedAt > EXPIRE_AFTER;
}

/**
 * 按可见范围过滤并倒序排序（最新在上）。
 * 与 TAPD 一致：新建页和详情页共用同一个缓存池，只按用户 + 工作空间 + 类型隔离，
 * 不再按单个工作项拆分，靠条目标题区分来源。
 */
export function filterVisible(
  records,
  { userId, wsId, type },
  now = Date.now(),
) {
  return records
    .filter(isCompatibleRecord)
    .filter((record) => !isExpired(record, now))
    .filter(
      (record) =>
        String(record.userId) === String(userId) &&
        String(record.wsId) === String(wsId) &&
        record.type === type,
    )
    .sort((a, b) => b.savedAt - a.savedAt);
}
