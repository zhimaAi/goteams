// Parses status-transition settings and resolves configured field labels.
import {
  DEFAULT_STATUS_TRANSITION_FORM,
  DEFECT_STATUS_TRANSITION_KEYS,
  STATUS_TRANSITION_BUILTIN_KEYS,
} from "@/constants/statusTransition";
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
  form.custom_enabled = Number(raw?.custom_enabled) === 1 ? 1 : 0;
  form.status_rules = Array.isArray(raw?.status_rules)
    ? raw.status_rules.map((rule) => ({
        status_id: Number(rule.status_id),
        fields: Array.isArray(rule.fields)
          ? rule.fields.map((field) => ({
              key: field.key,
              required: Number(field.required) ? 1 : 0,
            }))
          : [],
      }))
    : [];
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
  const available = (item) => {
    if (
      typeKey !== "defect" &&
      DEFECT_STATUS_TRANSITION_KEYS.includes(item.key)
    )
      return false;
    const customId = statusTransitionCustomFieldId(item.key);
    if (customId)
      return activeCustomIds === null || activeCustomIds.has(customId);
    if (item.key === "comment" || builtinEnabled === null) return true;
    return !builtinEnabled.has(item.key) || builtinEnabled.get(item.key);
  };
  form.fields = form.fields.filter(available);
  form.status_rules = form.status_rules.map((rule) => ({
    ...rule,
    fields: rule.fields.filter(available),
  }));
  return form;
}

// An explicit empty rule overrides the defaults as well.
export function statusTransitionFields(form, targetStatusId) {
  if (form.enabled !== 1) return [];
  const rule =
    form.custom_enabled === 1
      ? form.status_rules?.find(
          (item) => item.status_id === Number(targetStatusId),
        )
      : null;
  return rule ? rule.fields : form.fields;
}

export function statusTransitionFieldOptions(
  typeKey,
  fieldConfigs,
  customFields,
  t,
) {
  const enabledKeys = new Set(
    fieldConfigs
      .filter((item) => Number(item.enabled) === 1)
      .map((item) => item.key),
  );
  const options = STATUS_TRANSITION_BUILTIN_KEYS.filter(
    (key) =>
      enabledKeys.has(key) &&
      (typeKey === "defect" || !DEFECT_STATUS_TRANSITION_KEYS.includes(key)),
  ).map((key) => ({
    key,
    label: statusTransitionFieldLabel(key, t, customFields),
    builtin: true,
  }));
  options.push({
    key: "comment",
    label: statusTransitionFieldLabel("comment", t),
    builtin: false,
  });
  for (const field of customFields) {
    if (Number(field.is_enabled) === 1)
      options.push({
        key: `custom_${field.id}`,
        label: field.name,
        builtin: false,
      });
  }
  return options;
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

  if (
    configuredKeys.has("severity_id") &&
    Object.hasOwn(payload, "severity_id")
  ) {
    updates.severity_id = payload.severity_id;
    const severity = (typeMeta.severities || []).find(
      (item) => Number(item.id) === Number(payload.severity_id),
    );
    updates.severity_name =
      severity?.label ||
      severity?.name ||
      (payload.severity_id ? String(payload.severity_id) : "");
    updates.severity_color = severity?.color || "";
  }
  if (
    configuredKeys.has("estimated_work_hours") &&
    Object.hasOwn(payload, "estimated_work_hours")
  )
    updates.estimated_work_hours = payload.estimated_work_hours;
  if (
    configuredKeys.has("responsible_ids") &&
    Object.hasOwn(payload, "responsible_ids")
  ) {
    updates.responsible_ids = payload.responsible_ids;
    updates.responsible_names = payload.responsible_ids.map(
      (id) =>
        (metadata?.members || []).find(
          (member) => Number(member.user_id) === Number(id),
        )?.user_name ||
        workItem.responsible_names?.[
          workItem.responsible_ids?.map(Number).indexOf(Number(id))
        ] ||
        "",
    );
  }
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
  if (key === "severity_id") return t("workspace_severity");
  if (key === "estimated_work_hours")
    return t("requirement_estimatedWorkHours");
  if (key === "responsible_ids") return t("workspace_field_responsible");
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

export function statusActionPayload(action) {
  return {
    status_id: Number(action.target_status_id),
    status_action: {
      mapping_id: Number(action.id),
      source_status_id: Number(action.source_status_id),
    },
  };
}
