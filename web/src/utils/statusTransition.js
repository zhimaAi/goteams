// Parses status-transition settings and resolves configured field labels.
import { DEFAULT_STATUS_TRANSITION_FORM } from "@/constants/statusTransition";
import { customFieldValues } from "@/utils/customField";
import { buildWorkItemListUpdate } from "@/utils/workItemList";

export function statusTransitionForm(
  settings,
  typeKey,
  customFields = null,
  fieldConfigs = null,
) {
  if (typeKey === "planning") return { enabled: 0, fields: [] };
  const raw = settings?.[`${typeKey}_status_transition`];
  const form =
    !raw || typeof raw !== "object"
      ? {
          enabled: DEFAULT_STATUS_TRANSITION_FORM.enabled,
          fields: DEFAULT_STATUS_TRANSITION_FORM.fields.map((item) => ({
            ...item,
          })),
        }
      : {
          enabled: Number(raw.enabled) === 0 ? 0 : 1,
          fields: Array.isArray(raw.fields)
            ? raw.fields.map((item) => ({
                key: item.key,
                required: Number(item.required) ? 1 : 0,
              }))
            : DEFAULT_STATUS_TRANSITION_FORM.fields.map((item) => ({
                ...item,
              })),
        };
  const activeCustomIds = Array.isArray(customFields)
    ? new Set(
        customFields
          .filter((item) => Number(item.is_enabled) === 1)
          .map((item) => Number(item.id)),
      )
    : null;
  const builtinEnabled = Array.isArray(fieldConfigs)
    ? new Map(
        fieldConfigs.map((item) => [item.key, Number(item.enabled) === 1]),
      )
    : null;
  form.fields = form.fields.filter((item) => {
    const customId = statusTransitionCustomFieldId(item.key);
    if (customId)
      return activeCustomIds === null || activeCustomIds.has(customId);
    if (item.key === "comment" || builtinEnabled === null) return true;
    return !builtinEnabled.has(item.key) || builtinEnabled.get(item.key);
  });
  return form;
}

// Builds a complete list projection after a successful advanced transition.
export function buildStatusTransitionListUpdate(
  workItem,
  payload,
  configuredKeys,
  metadata,
) {
  const typeMeta = metadata?.[workItem.type] || {};
  const settings = metadata?.settings || {};
  const updates = { status_id: Number(payload.status_id) };
  if (configuredKeys.has("assignee") && Object.hasOwn(payload, "assignee_ids"))
    updates.assignee_ids = payload.assignee_ids;
  if (configuredKeys.has("version") && Object.hasOwn(payload, "version_id")) {
    updates.version_id = payload.version_id;
    updates.version_name =
      (metadata?.versions || []).find(
        (item) => Number(item.id) === Number(payload.version_id),
      )?.name || "";
  }
  if (configuredKeys.has("priority") && Object.hasOwn(payload, "priority"))
    updates.priority = payload.priority;

  const startChanged =
    configuredKeys.has("planned_start_date") &&
    Object.hasOwn(payload, "planned_start_date");
  const endChanged =
    configuredKeys.has("planned_end_date") &&
    Object.hasOwn(payload, "planned_end_date");
  if (startChanged)
    updates.planned_start_date = Number(payload.planned_start_date) || 0;
  if (endChanged)
    updates.planned_end_date = Number(payload.planned_end_date) || 0;
  if (configuredKeys.has("comment") && String(payload.comment || "").trim()) {
    updates.comment_count = Number(workItem.comment_count || 0) + 1;
  }
  if (payload.custom_fields) {
    updates.custom_fields = {
      ...customFieldValues(workItem.custom_fields),
      ...payload.custom_fields,
    };
  }

  return buildWorkItemListUpdate(workItem, updates, {
    statuses: typeMeta.statuses,
    priorities: typeMeta.priorities,
    members: metadata?.members,
    finalStatusIds: settings[`${workItem.type}_final_status_ids`],
  });
}

export function isStatusTransitionEnabled(settings, typeKey) {
  return statusTransitionForm(settings, typeKey).enabled === 1;
}

export function statusTransitionCustomFieldId(key) {
  if (!String(key).startsWith("custom_")) return 0;
  const id = Number(String(key).slice(7));
  return Number.isFinite(id) && id > 0 ? id : 0;
}

export function statusTransitionFieldLabel(key, t, customFields = []) {
  if (key === "comment") return t("requirement_comment");
  if (key === "assignee") return t("workspace_field_assignee");
  if (key === "version") return t("workspace_field_iteration");
  if (key === "priority") return t("workspace_field_priority");
  if (key === "planned_start_date")
    return t("workspace_field_plannedStartDate");
  if (key === "planned_end_date") return t("workspace_field_plannedEndDate");
  const customId = statusTransitionCustomFieldId(key);
  if (customId) {
    const field = customFields.find(
      (item) => Number(item.id) === Number(customId),
    );
    return field?.name || key;
  }
  return key;
}
