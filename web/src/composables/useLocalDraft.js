// 本地草稿 composable：负责 localForage(IndexedDB) 读写、350ms 调度、
// 写入串行化和过期清理。纯规则在 utils/localDraftRecord.js。
import { reactive, ref } from "vue";
import localforage from "localforage";
import {
  applyPosted,
  applyRetarget,
  applySave,
  applySkipAutoRestore,
  buildCreateKey,
  buildItemKey,
  filterVisible,
  isCompatibleRecord,
  isExpired,
  pickAutoRestorable,
  SAVE_DELAY,
} from "@/utils/localDraftRecord";

// 草稿独立于其它本地数据，使用单独 object store
const draftStorage = localforage.createInstance({
  name: "goteams",
  storeName: "local_drafts",
});

/**
 * @param {object} options
 * @param {"create"|"work-item"} options.kind
 * @param {string|number} options.userId
 * @param {string|number} options.wsId
 * @param {string} options.type requirement | defect
 * @param {string|number} options.targetId 创建态 sessionId / 详情态工作项 ID
 * @param {Function} [options.onStorageError] 每次组件生命周期最多回调一次
 */
export function useLocalDraft({
  kind,
  userId,
  wsId,
  type,
  targetId,
  onStorageError,
}) {
  const scope = reactive({
    userId: String(userId ?? ""),
    wsId: String(wsId ?? ""),
    type: type || "",
    targetId: String(targetId ?? ""),
  });

  // 本次组件生命周期内最后一次成功保存的时间（ms），提示条时间源
  const lastSavedAt = ref(null);
  // 当前可见范围是否有未提交草稿
  const hasUnposted = ref(false);
  // 当前可见范围是否存在草稿记录（含已提交），决定入口是否可见
  const hasRecords = ref(false);
  // 本次生命周期是否发生过存储失败（页面据此提示一次）
  const storageFailed = ref(false);

  let saveTimer = null;
  let pending = null;
  // 所有读写进入同一队列，避免 debounce 与 markPosted/retarget 互相覆盖
  let chain = Promise.resolve();
  let errorNotified = false;

  // 保存任务入队前冻结 scope，避免工作项切换后把 A 的内容写到 B。
  function snapshotScope() {
    const snapshot = {
      userId: scope.userId,
      wsId: scope.wsId,
      type: scope.type,
      targetId: scope.targetId,
    };
    return {
      ...snapshot,
      key:
        kind === "create"
          ? buildCreateKey(snapshot.userId, snapshot.targetId)
          : buildItemKey(snapshot.userId, snapshot.wsId, snapshot.targetId),
    };
  }

  function isCurrentScope(snapshot) {
    return (
      scope.userId === snapshot.userId &&
      scope.wsId === snapshot.wsId &&
      scope.type === snapshot.type &&
      scope.targetId === snapshot.targetId
    );
  }

  function notifyError(err) {
    console.warn("[local-draft] storage failed:", err);
    if (!errorNotified) {
      errorNotified = true;
      storageFailed.value = true;
      onStorageError?.();
    }
  }

  function enqueue(task) {
    chain = chain.then(task).catch(notifyError);
    return chain;
  }

  // 删除过期、结构损坏或 schema 不兼容的记录；本功能不做旧结构迁移。
  async function readValidRecord(key, now = Date.now()) {
    const record = await draftStorage.getItem(key);
    if (!record) return null;
    if (
      !isCompatibleRecord(record) ||
      record.key !== key ||
      isExpired(record, now)
    ) {
      await draftStorage.removeItem(key);
      return null;
    }
    return record;
  }

  // 立即写入一份内容（350ms 调度到期后也走这里）
  function save(data) {
    const snapshot = snapshotScope();
    const payload = {
      title: data.title ?? "",
      html: data.html ?? "",
      mode: data.mode || "",
    };
    return enqueue(async () => {
      const record = await readValidRecord(snapshot.key);
      const next = applySave(record, {
        key: snapshot.key,
        kind,
        userId: snapshot.userId,
        wsId: snapshot.wsId,
        type: snapshot.type,
        targetId: snapshot.targetId,
        ...payload,
      });
      if (!next) return null;
      // 只有写入成功才更新时间
      await draftStorage.setItem(snapshot.key, next);
      if (isCurrentScope(snapshot)) {
        lastSavedAt.value = next.savedAt;
        hasUnposted.value = !next.isPosted;
        hasRecords.value = true;
      }
      return next;
    });
  }

  /** 用户内容变化后调用：350ms debounce */
  function schedule(data) {
    pending = data;
    clearTimeout(saveTimer);
    saveTimer = setTimeout(() => {
      const next = pending;
      pending = null;
      if (next) void save(next);
    }, SAVE_DELAY);
  }

  /** 立即处理待保存内容（提交 / SPA 离开前调用） */
  function flush() {
    clearTimeout(saveTimer);
    const next = pending;
    pending = null;
    return next ? save(next) : chain;
  }

  /** 后端创建/更新成功后调用：只标记当前草稿已提交，不刷新 savedAt */
  function markPosted() {
    const snapshot = snapshotScope();
    return enqueue(async () => {
      const record = await readValidRecord(snapshot.key);
      const next = applyPosted(record);
      if (!next) return;
      await draftStorage.setItem(snapshot.key, next);
      if (isCurrentScope(snapshot)) {
        hasUnposted.value = false;
        hasRecords.value = true;
      }
    });
  }

  /** 创建页切换工作空间/类型：同 key 原地迁移当前草稿 */
  async function retarget(next) {
    await flush();
    const snapshot = snapshotScope();
    const nextWsId =
      next.wsId === undefined ? snapshot.wsId : String(next.wsId ?? "");
    const nextType = next.type === undefined ? snapshot.type : next.type || "";
    return enqueue(async () => {
      const record = await readValidRecord(snapshot.key);
      const updated = applyRetarget(record, {
        wsId: nextWsId,
        type: nextType,
      });
      if (updated) await draftStorage.setItem(snapshot.key, updated);
      if (!isCurrentScope(snapshot)) return;
      scope.wsId = nextWsId;
      scope.type = nextType;
      hasUnposted.value = Boolean(updated && !updated.isPosted);
    });
  }

  /**
   * 详情页切换工作项/工作空间：先落盘旧会话，再换发新会话；旧记录保留在原 key。
   * 返回 Promise，调用方必须 await，避免待保存内容丢失或串写到新 target。
   */
  async function setTarget(nextTargetId, next = {}, isCurrent = () => true) {
    await flush();
    // A superseded detail load must not switch the active draft session.
    if (!isCurrent()) return;
    scope.targetId = String(nextTargetId ?? "");
    if (next.wsId !== undefined) scope.wsId = String(next.wsId ?? "");
    if (next.type !== undefined) scope.type = next.type || "";
    hasUnposted.value = false;
    hasRecords.value = false;
    lastSavedAt.value = null;
  }

  /** 「提交并继续创建」后换发新会话 key */
  function startNew(nextTargetId) {
    return setTarget(nextTargetId);
  }

  /** 清理过期记录并返回当前可见草稿（按 savedAt 倒序），同时刷新提示状态 */
  function list() {
    const snapshot = snapshotScope();
    return enqueue(async () => {
      const now = Date.now();
      // 与 TAPD 一致：创建页和详情页看同一个缓存池，不按单个工作项拆分。
      const keys = await draftStorage.keys();
      const all = [];
      for (const key of keys) {
        if (!key.startsWith("draft-")) continue;
        const record = await readValidRecord(key, now);
        if (record) all.push(record);
      }
      const visible = filterVisible(all, snapshot, now);
      if (isCurrentScope(snapshot)) {
        hasUnposted.value = visible.some((record) => !record.isPosted);
        hasRecords.value = visible.length > 0;
      }
      return visible;
    });
  }

  /**
   * 标记某条草稿不再参与自动恢复。不传 key 时作用于当前会话（用户点取消 = 正常关闭），
   * 此时先落盘待保存内容，草稿仍留在历史缓存里可手动恢复。
   */
  function skipAutoRestore(key) {
    const targetKey = key || snapshotScope().key;
    if (!key) void flush();
    return enqueue(async () => {
      const record = await readValidRecord(targetKey);
      const next = applySkipAutoRestore(record);
      if (next) await draftStorage.setItem(targetKey, next);
      return next;
    });
  }

  /**
   * 取出上次异常关闭遗留的创建草稿并立即标记，保证一条记录最多自动恢复一次。
   * 无可恢复内容时返回 null。
   */
  async function takeAutoRestorable() {
    const target = pickAutoRestorable((await list()) || []);
    if (!target) return null;
    const marked = await skipAutoRestore(target.key);
    if (!marked) return null;
    return {
      title: target.title,
      html: target.html,
      mode: target.mode || "",
    };
  }

  // 初始化时清理一次过期/不兼容数据；失败不影响页面
  void enqueue(async () => {
    const keys = await draftStorage.keys();
    const now = Date.now();
    for (const key of keys) {
      if (!key.startsWith("draft-")) continue;
      await readValidRecord(key, now);
    }
  });

  return {
    schedule,
    flush,
    markPosted,
    retarget,
    setTarget,
    startNew,
    list,
    skipAutoRestore,
    takeAutoRestorable,
    lastSavedAt,
    hasUnposted,
    hasRecords,
    storageFailed,
  };
}
