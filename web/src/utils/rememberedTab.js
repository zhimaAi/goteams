const STORAGE_PREFIX = "goteams:last-tab";

function storageKey(userId, scope) {
  if (userId === undefined || userId === null || userId === "" || !scope) {
    return "";
  }
  return `${STORAGE_PREFIX}:${userId}:${scope}`;
}

export function getRememberedTab(userId, scope, availableTabs, fallback) {
  const tabs = Array.isArray(availableTabs) ? availableTabs : [];
  const defaultTab = tabs.includes(fallback) ? fallback : tabs[0] || fallback;
  const key = storageKey(userId, scope);
  if (!key || typeof window === "undefined") return defaultTab;

  try {
    const remembered = window.localStorage.getItem(key);
    return tabs.includes(remembered) ? remembered : defaultTab;
  } catch {
    return defaultTab;
  }
}

export function setRememberedTab(userId, scope, tab) {
  const key = storageKey(userId, scope);
  if (!key || typeof tab !== "string" || typeof window === "undefined") return;

  try {
    window.localStorage.setItem(key, tab);
  } catch {
    // 浏览器存储受限时不影响当前 Tab 切换。
  }
}
