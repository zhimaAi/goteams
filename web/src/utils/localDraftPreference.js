// 历史缓存弹窗里的「异常关闭时自动恢复」开关。属于浏览器本地偏好，与草稿数据一样不上报后端。
const STORAGE_KEY = "goteams:draft-auto-restore";

/** 是否开启异常关闭自动恢复；未设置过时默认开启 */
export function isDraftAutoRestoreEnabled() {
  if (typeof window === "undefined") return true;
  try {
    return window.localStorage.getItem(STORAGE_KEY) !== "0";
  } catch {
    return true;
  }
}

export function setDraftAutoRestoreEnabled(enabled) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, enabled ? "1" : "0");
  } catch {
    // 浏览器存储受限时开关不持久化，不影响当前页面行为。
  }
}
