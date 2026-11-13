// Shared per-workspace work item type attribute state (name/icon config from
// "工作项属性" project settings). Unconfigured types keep the legacy defaults,
// so every surface degrades to the previous hard-coded labels/colors.
import { computed, reactive, watch } from "vue";
import { useI18n } from "vue-i18n";
import { getWorkItemAttributes } from "@/api/workspace";

// Default display labels/colors mirror the legacy hard-coded type rendering
// (WorkItemTypePill blue #3582fb for requirement, red #e45454 for defect).
export const DEFAULT_TYPE_LABEL_KEYS = {
  requirement: "requirement_feature",
  defect: "requirement_bug",
};
export const DEFAULT_TYPE_ICON_COLORS = {
  requirement: "#3582fb",
  defect: "#e45454",
};

const EMPTY_DATA = { requirement: null, defect: null };

// wsId (string) -> { data: {requirement, defect}, promise }.
const attrState = reactive(new Map());

function recordOf(key) {
  let record = attrState.get(key);
  if (!record) {
    record = { data: { ...EMPTY_DATA }, promise: null };
    attrState.set(key, record);
  }
  return record;
}

// ensureWorkItemTypeAttributes loads and caches both types' attributes for a workspace.
export function ensureWorkItemTypeAttributes(wsId) {
  const key = String(wsId ?? "");
  if (!key) return Promise.resolve(null);
  const record = recordOf(key);
  if (record.promise) return record.promise;
  record.promise = getWorkItemAttributes(key)
    .then((rows) => {
      const data = { requirement: null, defect: null };
      for (const row of rows || []) {
        if (row && (row.type_key === "requirement" || row.type_key === "defect")) {
          data[row.type_key] = row;
        }
      }
      record.data = data;
      return data;
    })
    .catch((err) => {
      // Drop the failed entry so the next caller can retry; keep legacy defaults meanwhile.
      attrState.delete(key);
      return null;
    })
    .finally(() => {
      const current = attrState.get(key);
      if (current) current.promise = null;
    });
  return record.promise;
}

// cacheWorkItemAttribute replaces one type's cached row (used after a successful save).
export function cacheWorkItemAttribute(wsId, typeKey, attr) {
  const key = String(wsId ?? "");
  if (!key) return;
  const data = recordOf(key).data;
  data[typeKey] = attr;
}

// useWorkItemTypeAttributes resolves display name/icon for a workspace's work item types.
// wsId may be a ref (number/string) or a plain string; loading starts immediately.
export function useWorkItemTypeAttributes(wsId) {
  const { t } = useI18n();
  const key = computed(() => {
    const raw = wsId && typeof wsId === "object" && "value" in wsId ? wsId.value : wsId;
    return String(raw ?? "");
  });
  watch(key, (k) => {
    if (k) ensureWorkItemTypeAttributes(k);
  }, { immediate: true });

  const attributes = computed(() => attrState.get(key.value)?.data || EMPTY_DATA);
  const attrOf = (typeKey) => attributes.value[typeKey] || null;
  const labelKeyOf = (typeKey) =>
    typeKey === "defect" ? DEFAULT_TYPE_LABEL_KEYS.defect : DEFAULT_TYPE_LABEL_KEYS.requirement;

  const displayName = (typeKey) => attrOf(typeKey)?.name || t(labelKeyOf(typeKey));
  const iconText = (typeKey) => attrOf(typeKey)?.icon_text || t(labelKeyOf(typeKey));
  const iconColor = (typeKey) => attrOf(typeKey)?.icon_color || DEFAULT_TYPE_ICON_COLORS[typeKey] || "";

  return { attributes, attrOf, displayName, iconText, iconColor };
}
