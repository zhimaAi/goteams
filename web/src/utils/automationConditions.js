import { filterMembersForField } from "@/utils/customField";
export function conditionFieldOptions(field, source, type) {
  if (
    ["user", "user_multi"].includes(field.type) &&
    !["assignee", "created_by"].includes(field.key)
  )
    return filterMembersForField(
      field,
      source.workspace_metadata?.custom_field_members || [],
    ).map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    }));
  if (field.options?.length)
    return field.options.map((item) => ({ value: item.key, label: item.name }));
  const group = source.workspace_metadata?.[type] || {};
  if (field.key === "status")
    return (group.statuses || []).map((item) => ({
      value: Number(item.id),
      label: item.name,
    }));
  if (field.key === "priority")
    return (group.priorities || []).map((item) => ({
      value: Number(item.id),
      label: item.label,
    }));
  if (["assignee", "created_by"].includes(field.key))
    return (source.workspace_metadata?.members || []).map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    }));
  if (field.key === "version")
    return (source.workspace_metadata?.versions || []).map((item) => ({
      value: Number(item.id),
      label: item.name,
    }));
  return [];
}
export function automationOperatorOptions(field, currentOperator, t) {
  const type = field?.type;
  const key = field?.key;
  const byType = {
    text: ["contains", "not_contains", "is", "is_not", "empty", "not_empty"],
    textarea: [
      "contains",
      "not_contains",
      "is",
      "is_not",
      "empty",
      "not_empty",
    ],
    number: ["eq", "neq", "gt", "gte", "lt", "lte", "empty", "not_empty"],
    date: [
      "before",
      "before_eq",
      "after",
      "after_eq",
      "is",
      "is_not",
      "empty",
      "not_empty",
    ],
    user: ["is", "is_not", "empty", "not_empty"],
    select_single: ["is", "is_not", "in", "not_in", "empty", "not_empty"],
    user_multi: [
      "contains_any",
      "contains_all",
      "contains_none",
      "empty",
      "not_empty",
    ],
    select_multi: [
      "contains_any",
      "contains_all",
      "contains_none",
      "empty",
      "not_empty",
    ],
  };
  const operators =
    type === "user" &&
    (["assignee", "created_by"].includes(key) ||
      ["in", "not_in"].includes(currentOperator))
      ? ["is", "is_not", "in", "not_in", "empty", "not_empty"]
      : byType[
          type === "numeric"
            ? "number"
            : type === "phone"
              ? "text"
              : type === "datetime"
                ? "date"
                : type
        ] || [];
  return operators.map((value) => ({
    value,
    label: t(`automation_operator_${value}`),
  }));
}
