export const DEFAULT_BOARD_SORT = Object.freeze({
  field: "updated_at",
  direction: "asc",
});

export const GLOBAL_LIST_PREFERENCES_CHANGE_EVENT =
  "goteams:list-global-preferences-change";

export function myWorkPreferencesKey(userId) {
  return userId ? `goteams:my-work-preferences:${userId}` : "";
}

export function readMyWorkPreferences(userId) {
  const value = readListPreferences(myWorkPreferencesKey(userId));
  return {
    ...value,
    projectOrder: Array.isArray(value.projectOrder)
      ? value.projectOrder.map(String)
      : [],
    hiddenProjectIds: Array.isArray(value.hiddenProjectIds)
      ? value.hiddenProjectIds.map(String)
      : [],
    projectColors: readBoardColors(value.projectColors),
  };
}

export function writeMyWorkPreferences(userId, patch) {
  const key = myWorkPreferencesKey(userId);
  if (!key) return;
  try {
    const current = readMyWorkPreferences(userId);
    window.localStorage.setItem(key, JSON.stringify({ ...current, ...patch }));
  } catch {
    // Browser storage is optional and must not block my-work interactions.
  }
}

export const BOARD_SORT_OPTIONS = Object.freeze([
  { value: "updated_at:asc", field: "updated_at", direction: "asc" },
  { value: "updated_at:desc", field: "updated_at", direction: "desc" },
  { value: "created_at:asc", field: "created_at", direction: "asc" },
  { value: "created_at:desc", field: "created_at", direction: "desc" },
]);

export function scopedListPreferencesKey(userId, wsId, typeKey) {
  if (!userId || !wsId || !typeKey) return "";
  return `goteams:list-preferences:${userId}:${wsId}:${typeKey}`;
}

export function globalListPreferencesKey(userId) {
  return userId ? `goteams:list-global-preferences:${userId}` : "";
}

export function readListPreferences(key) {
  if (!key) return {};
  try {
    const value = JSON.parse(window.localStorage.getItem(key) || "{}");
    return value && typeof value === "object" ? value : {};
  } catch {
    return {};
  }
}

export function readScopedListPreferences(userId, wsId, typeKey) {
  return readListPreferences(scopedListPreferencesKey(userId, wsId, typeKey));
}

export function readGlobalListPreferences(userId) {
  return readListPreferences(globalListPreferencesKey(userId));
}

export function writeGlobalListPreferences(userId, patch) {
  const key = globalListPreferencesKey(userId);
  if (!key) return;
  try {
    const current = readGlobalListPreferences(userId);
    window.localStorage.setItem(key, JSON.stringify({ ...current, ...patch }));
    // localStorage does not notify other components in the same window.
    window.dispatchEvent(
      new CustomEvent(GLOBAL_LIST_PREFERENCES_CHANGE_EVENT, {
        detail: { userId: String(userId), patch },
      }),
    );
  } catch {
    // Browser storage is optional and must not block detail interactions.
  }
}

export function teamWorkPreferencesKey(userId) {
  return userId ? `goteams:team-work-preferences:${userId}` : "";
}

export function readTeamWorkPreferences(userId) {
  return readListPreferences(teamWorkPreferencesKey(userId));
}

export function writeTeamWorkPreferences(userId, patch) {
  const key = teamWorkPreferencesKey(userId);
  if (!key) return;
  try {
    const current = readTeamWorkPreferences(userId);
    window.localStorage.setItem(key, JSON.stringify({ ...current, ...patch }));
  } catch {
    // Browser storage is optional and must not block team-work interactions.
  }
}

export function writeScopedListPreferences(userId, wsId, typeKey, patch) {
  const key = scopedListPreferencesKey(userId, wsId, typeKey);
  if (!key) return;
  try {
    const current = readScopedListPreferences(userId, wsId, typeKey);
    window.localStorage.setItem(key, JSON.stringify({ ...current, ...patch }));
  } catch {
    // Browser storage is optional and must not block board interactions.
  }
}

export function listColumnWidthsKey(userId, wsId, typeKey) {
  if (!userId || !wsId || !typeKey) return "";
  return `goteams:list-column-widths:${userId}:${wsId}:${typeKey}`;
}

export function readListColumnWidths(userId, wsId, typeKey) {
  const key = listColumnWidthsKey(userId, wsId, typeKey);
  if (!key) return {};
  try {
    return parseListColumnWidths(
      JSON.parse(window.localStorage.getItem(key) || "{}"),
    );
  } catch {
    // Corrupted or unavailable storage falls back to the configured widths.
    return {};
  }
}

export function writeListColumnWidths(userId, wsId, typeKey, widths) {
  const key = listColumnWidthsKey(userId, wsId, typeKey);
  if (!key) return;
  try {
    window.localStorage.setItem(
      key,
      JSON.stringify(parseListColumnWidths(widths)),
    );
  } catch {
    // Limited browser cache should not block the current column-width change.
  }
}

function parseListColumnWidths(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value)
      .map(([key, width]) => [key, Number(width)])
      .filter(([, width]) => Number.isFinite(width) && width > 0),
  );
}

export function normalizeBoardSort(value) {
  const field = value?.field === "created_at" ? "created_at" : "updated_at";
  const direction = value?.direction === "desc" ? "desc" : "asc";
  return { field, direction };
}

export function boardSortValue(value) {
  const sort = normalizeBoardSort(value);
  return `${sort.field}:${sort.direction}`;
}

export function readBoardColors(value) {
  if (!value || typeof value !== "object") return {};
  return Object.fromEntries(
    Object.entries(value).filter(
      ([id, color]) =>
        /^\d+$/.test(id) && /^#[0-9a-f]{6}$/i.test(String(color)),
    ),
  );
}
