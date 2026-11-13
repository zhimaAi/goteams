const contextPrefix = "goteams-work-item-navigation:";
const contextTtl = 24 * 60 * 60 * 1000;

export function normalizeWorkItemNavigationItems(items) {
  const result = [];
  const seen = new Set();
  for (const item of Array.isArray(items) ? items : []) {
    const id = Number(item?.id);
    const workspaceId = Number(item?.workspaceId);
    if (!Number.isInteger(id) || id <= 0) continue;
    if (!Number.isInteger(workspaceId) || workspaceId <= 0) continue;
    const key = `${workspaceId}:${id}`;
    if (seen.has(key)) continue;
    seen.add(key);
    result.push({ id, workspaceId });
  }
  return result;
}

function cleanupExpiredContexts(now = Date.now()) {
  for (let index = localStorage.length - 1; index >= 0; index -= 1) {
    const key = localStorage.key(index);
    if (!key?.startsWith(contextPrefix)) continue;
    try {
      const context = JSON.parse(localStorage.getItem(key) || "null");
      if (!context?.createdAt || now - Number(context.createdAt) > contextTtl)
        localStorage.removeItem(key);
    } catch {
      localStorage.removeItem(key);
    }
  }
}

export function saveWorkItemNavigationContext(items) {
  if (typeof window === "undefined") return "";
  const normalizedItems = normalizeWorkItemNavigationItems(items);
  if (!normalizedItems.length) return "";
  try {
    cleanupExpiredContexts();
    const token =
      window.crypto?.randomUUID?.() ||
      `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    localStorage.setItem(
      `${contextPrefix}${token}`,
      JSON.stringify({ createdAt: Date.now(), items: normalizedItems }),
    );
    return token;
  } catch {
    return "";
  }
}

export function loadWorkItemNavigationContext(token) {
  if (typeof window === "undefined" || !token) return [];
  const key = `${contextPrefix}${token}`;
  try {
    const context = JSON.parse(localStorage.getItem(key) || "null");
    if (
      !context?.createdAt ||
      Date.now() - Number(context.createdAt) > contextTtl
    ) {
      localStorage.removeItem(key);
      return [];
    }
    return normalizeWorkItemNavigationItems(context.items);
  } catch {
    localStorage.removeItem(key);
    return [];
  }
}
