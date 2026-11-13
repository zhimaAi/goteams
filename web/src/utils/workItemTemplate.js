import { TEMPLATE_PRESET_KEYS } from "../constants/workItemTemplate.js";

export function customTemplateFieldKey(id) {
  return `custom:${id}`;
}

export function parseCustomTemplateFieldId(key) {
  if (!String(key || "").startsWith("custom:")) return 0;
  return Number(String(key).slice(7)) || 0;
}

export function emptyTemplateField(fieldKey) {
  return { field_key: fieldKey, default_value: null, required: false };
}

export function presetTemplateFields(availableKeys) {
  const allowed = new Set(availableKeys);
  return TEMPLATE_PRESET_KEYS.filter((key) => allowed.has(key)).map(
    emptyTemplateField,
  );
}

export function isTemplateValueEmpty(value) {
  if (value == null || value === "") return true;
  if (Array.isArray(value) && value.length === 0) return true;
  return false;
}

// Current project members only. Metadata also carries removed and former
// assignees so history can render their names; those must not become template
// defaults or new assignees.
export function currentProjectMembers(members) {
  const seen = new Set();
  const result = [];
  for (const member of members || []) {
    const id = Number(member.user_id);
    if (id <= 0 || seen.has(id)) continue;
    if (member.removed || String(member.user_status) === "0") continue;
    seen.add(id);
    result.push(member);
  }
  return result;
}

export function templateAssigneeIds(value, members) {
  const allowed = new Set(
    currentProjectMembers(members).map((member) => Number(member.user_id)),
  );
  const ids = Array.isArray(value) ? value : [value];
  const seen = new Set();
  const result = [];
  for (const raw of ids) {
    const id = Number(raw);
    if (!allowed.has(id) || seen.has(id)) continue;
    seen.add(id);
    result.push(id);
  }
  return result;
}

export function resolveTemplateDateDefault(value, unix, now = new Date()) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return value;
  const mode = value.date_type;
  if (!TEMPLATE_DATE_MODES.has(mode)) return value;
  const date = new Date(now);
  date.setHours(0, 0, 0, 0);
  const days = mode === "current" ? 0 : Number(value.offset_days);
  if (!Number.isInteger(days) || days < 0) return null;
  date.setDate(date.getDate() + (mode === "before" ? -days : days));
  if (unix) return Math.floor(date.getTime() / 1000);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function templateFieldIsVisible(
  fields,
  fieldKey,
  getValue,
  alwaysVisible = () => false,
  visiting = new Set(),
) {
  if (alwaysVisible(fieldKey)) return true;
  if (visiting.has(fieldKey)) return false;
  const path = new Set(visiting).add(fieldKey);
  let linked = false;
  for (const trigger of fields || []) {
    const current = getValue(trigger.field_key);
    for (const rule of trigger.linkages || []) {
      if (!(rule.field_keys || []).includes(fieldKey)) continue;
      linked = true;
      const values = Array.isArray(current) ? current : [current];
      if (
        values.some((value) => String(value) === String(rule.value)) &&
        templateFieldIsVisible(
          fields,
          trigger.field_key,
          getValue,
          alwaysVisible,
          path,
        )
      ) {
        return true;
      }
    }
  }
  return !linked;
}

export function createTemplateFieldIsVisible(
  fields,
  fieldKey,
  getValue,
  workspaceRequired = () => false,
) {
  if (workspaceRequired(fieldKey) || fields == null) return true;
  return (
    fields.some((field) => field.field_key === fieldKey) &&
    templateFieldIsVisible(fields, fieldKey, getValue, workspaceRequired)
  );
}

const TEMPLATE_DATE_MODES = new Set(["current", "after", "before"]);
