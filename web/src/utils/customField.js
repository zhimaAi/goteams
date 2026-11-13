import dayjs from "dayjs";
import {
  CUSTOM_FIELD_MEMBER_SCOPE,
  CUSTOM_FIELD_PHONE_RE,
  CUSTOM_FIELD_TYPE,
  isArrayFieldType,
  isCascaderFieldType,
  isMemberFieldType,
  isNumericFieldType,
  isOptionFieldType,
} from "@/constants/customField";
import {
  VIEW_VALUE_ALL,
  VIEW_VALUE_EMPTY,
  VIEW_VALUE_NOT_EMPTY,
} from "@/constants/workItemView";

export function customFieldValues(value) {
  let result = value;
  if (typeof result === "string") {
    try {
      result = JSON.parse(result);
    } catch {
      return {};
    }
  }
  return result && typeof result === "object" && !Array.isArray(result)
    ? result
    : {};
}

export function emptyCustomFieldValue(field) {
  if (isArrayFieldType(field?.field_type)) return [];
  if (
    isNumericFieldType(field?.field_type) ||
    field?.field_type === CUSTOM_FIELD_TYPE.USER
  ) {
    return undefined;
  }
  if (
    field?.field_type === CUSTOM_FIELD_TYPE.DATE ||
    field?.field_type === CUSTOM_FIELD_TYPE.DATETIME
  ) {
    return null;
  }
  return undefined;
}

function parseJsonValue(value) {
  if (typeof value !== "string") return value;
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

export function parseCustomFieldValue(field, raw) {
  if (raw === undefined || raw === null || raw === "") {
    return emptyCustomFieldValue(field);
  }
  const type = field?.field_type;
  const parsed = parseJsonValue(raw);
  if (type === CUSTOM_FIELD_TYPE.CASCADER_MULTI) {
    if (!Array.isArray(parsed)) return [];
    return [
      ...new Set(
        parsed
          .flatMap((item) => (Array.isArray(item) ? item : [item]))
          .map(String),
      ),
    ];
  }
  if (isArrayFieldType(type)) {
    if (Array.isArray(parsed)) return parsed.map(String);
    return parsed ? [String(parsed)] : [];
  }
  if (isNumericFieldType(type)) {
    if (parsed === "") return undefined;
    const number = Number(parsed);
    return Number.isFinite(number) ? number : parsed;
  }
  if (type === CUSTOM_FIELD_TYPE.USER) {
    return parsed === "" ? undefined : String(parsed);
  }
  if (type === CUSTOM_FIELD_TYPE.DATE || type === CUSTOM_FIELD_TYPE.DATETIME) {
    return parsed ? dayjs(parsed) : null;
  }
  return parsed;
}

export function serializeCustomFieldValue(field, value) {
  const type = field?.field_type;
  if (type === CUSTOM_FIELD_TYPE.DATE) {
    return value?.format?.("YYYY-MM-DD") || "";
  }
  if (type === CUSTOM_FIELD_TYPE.DATETIME) {
    return value?.format?.("YYYY-MM-DD HH:mm:ss") || "";
  }
  if (isArrayFieldType(type)) {
    return Array.isArray(value) ? value : [];
  }
  if (isNumericFieldType(type) || type === CUSTOM_FIELD_TYPE.USER) {
    return value ?? "";
  }
  return value ?? "";
}

export function storedCustomFieldValue(field, serialized) {
  return isArrayFieldType(field?.field_type)
    ? JSON.stringify(Array.isArray(serialized) ? serialized : [])
    : (serialized ?? "");
}

export function isValidPhoneNumber(value) {
  if (!value) return true;
  return CUSTOM_FIELD_PHONE_RE.test(String(value));
}

export function isValidPhoneFieldValue(field, value) {
  return (
    field?.field_type !== CUSTOM_FIELD_TYPE.PHONE || isValidPhoneNumber(value)
  );
}

export function filterMembersForField(field, members = []) {
  const assignable = members.filter(
    (member) => !member.removed && String(member.user_status ?? "1") !== "0",
  );
  if (field?.member_scope === CUSTOM_FIELD_MEMBER_SCOPE.WORKSPACE) {
    return assignable.filter((member) => member.is_workspace_member !== false);
  }
  if (field?.member_scope !== CUSTOM_FIELD_MEMBER_SCOPE.ROLES) {
    return assignable;
  }
  const roleIds = new Set((field.role_ids || []).map(String));
  return assignable.filter((member) =>
    roleIds.has(String(member.enterprise_role_id)),
  );
}

export function flattenCascaderNodes(nodes = [], acc = []) {
  nodes.forEach((node) => {
    acc.push(node);
    if (node.children?.length) flattenCascaderNodes(node.children, acc);
  });
  return acc;
}

export function cascaderPathKeys(nodes, targetKey, parentPath = []) {
  for (const node of nodes || []) {
    const path = [...parentPath, String(node.key)];
    if (String(node.key) === String(targetKey)) return path;
    const childPath = cascaderPathKeys(node.children, targetKey, path);
    if (childPath.length) return childPath;
  }
  return [];
}

export function formatCascaderPath(nodes, path = []) {
  const names = [];
  let current = nodes || [];
  for (const key of path) {
    const node = current.find((item) => String(item.key) === String(key));
    if (!node) return "";
    names.push(node.name);
    current = node.children || [];
  }
  return names.join("/");
}

export function formatCascaderNodePath(nodes, key) {
  const path = cascaderPathKeys(nodes, key);
  return path.length ? formatCascaderPath(nodes, path) : "";
}

export function formatCascaderNodePaths(nodes, keys = []) {
  return keys
    .map((key) => formatCascaderNodePath(nodes, key))
    .filter(Boolean)
    .join(" ｜ ");
}

export function normalizeCascaderFilterValues(fieldType, values = []) {
  const normalized = [
    ...new Set(
      (Array.isArray(values) ? values : [])
        .map((value) => String(value).trim())
        .filter(Boolean),
    ),
  ];
  return fieldType === CUSTOM_FIELD_TYPE.CASCADER_SINGLE
    ? normalized.slice(-1)
    : normalized;
}

export function normalizeCascaderFilterTree(tree, descriptors = []) {
  const fieldTypes = new Map(
    descriptors.map((descriptor) => [descriptor.field, descriptor.fieldType]),
  );
  const visit = (group) => ({
    ...group,
    conditions: (group?.conditions || []).map((node) => {
      if (!node.field) return visit(node);
      const fieldType = fieldTypes.get(node.field);
      return isCascaderFieldType(fieldType)
        ? {
            ...node,
            values: normalizeCascaderFilterValues(fieldType, node.values),
          }
        : node;
    }),
  });
  return visit(tree);
}

// normalizeCascaderViewFilterTree converts quick-search aggregate values into
// the view editor's native condition tree without changing query semantics.
export function normalizeCascaderViewFilterTree(tree, descriptors = []) {
  const multiFields = new Set(
    descriptors
      .filter(
        (descriptor) =>
          descriptor.fieldType === CUSTOM_FIELD_TYPE.CASCADER_MULTI,
      )
      .map((descriptor) => descriptor.field),
  );
  const unconstrained = Symbol("unconstrained");

  const convertLeaf = (node) => {
    const rawValues = Array.isArray(node.values) ? node.values : [];
    if (!multiFields.has(node.field) || node.operator !== "in") {
      return { ...node, values: [...rawValues] };
    }
    const values = [...new Set(rawValues.map(String))];
    const selected = new Set(values);
    if (
      selected.has(VIEW_VALUE_ALL) ||
      (selected.has(VIEW_VALUE_EMPTY) && selected.has(VIEW_VALUE_NOT_EMPTY))
    ) {
      return unconstrained;
    }
    const nodeValues = values.filter(
      (value) => value !== VIEW_VALUE_EMPTY && value !== VIEW_VALUE_NOT_EMPTY,
    );
    if (selected.has(VIEW_VALUE_NOT_EMPTY)) {
      return { ...node, operator: "not_empty", values: [] };
    }
    if (!selected.has(VIEW_VALUE_EMPTY)) {
      return { ...node, values: nodeValues };
    }
    const emptyCondition = { ...node, operator: "empty", values: [] };
    if (!nodeValues.length) return emptyCondition;
    return {
      logic: "or",
      conditions: [
        emptyCondition,
        { ...node, operator: "in", values: nodeValues },
      ],
    };
  };

  const visit = (group) => {
    const logic = group?.logic === "or" ? "or" : "and";
    const conditions = [];
    for (const node of group?.conditions || []) {
      const converted = node.field ? convertLeaf(node) : visit(node);
      if (converted === unconstrained) continue;
      conditions.push(converted);
    }
    return conditions.length ? { ...group, logic, conditions } : unconstrained;
  };

  const normalized = visit(tree);
  return normalized === unconstrained
    ? { ...tree, logic: tree?.logic === "or" ? "or" : "and", conditions: [] }
    : normalized;
}

export function optionFieldControlValue(field, value) {
  const allowed = new Set(
    (field?.options || []).map((option) => String(option.key)),
  );
  if (isArrayFieldType(field?.field_type)) {
    return (Array.isArray(value) ? value : []).filter((key) =>
      allowed.has(String(key)),
    );
  }
  return allowed.has(String(value)) ? value : undefined;
}

function optionName(field, key) {
  return (
    field.options?.find((option) => String(option.key) === String(key))?.name ||
    ""
  );
}

function memberName(members, id) {
  return (
    members.find((member) => String(member.user_id) === String(id))
      ?.user_name || id
  );
}

export function formatCustomFieldValue(field, values, members = []) {
  const raw = customFieldValues(values)[String(field.id)];
  if (raw === undefined || raw === null || raw === "") return "-";
  const value = parseCustomFieldValue(field, raw);
  const type = field.field_type;

  if (isOptionFieldType(type)) {
    const names = (Array.isArray(value) ? value : [value])
      .map((key) => optionName(field, key))
      .filter(Boolean);
    return names.length ? names.join(", ") : "-";
  }
  if (isMemberFieldType(type)) {
    const ids = Array.isArray(value) ? value : [value];
    if (!ids.length) return "-";
    return ids.map((id) => memberName(members, id)).join(", ");
  }
  if (type === CUSTOM_FIELD_TYPE.CASCADER_SINGLE) {
    return formatCascaderPath(field.cascader, value) || "-";
  }
  if (type === CUSTOM_FIELD_TYPE.CASCADER_MULTI) {
    const keys = Array.isArray(value) ? value : [];
    if (!keys.length) return "-";
    return formatCascaderNodePaths(field.cascader || [], keys) || "-";
  }
  if (type === CUSTOM_FIELD_TYPE.DATE && value?.format) {
    return value.format("YYYY-MM-DD");
  }
  if (type === CUSTOM_FIELD_TYPE.DATETIME && value?.format) {
    return value.format("YYYY-MM-DD HH:mm:ss");
  }
  if (Array.isArray(value)) {
    return value.length ? value.join(", ") : "-";
  }
  return String(value);
}

export function customFieldSearchControl(type) {
  if (TEXT_LIKE.has(type)) return ["text", "contains"];
  if (isNumericFieldType(type)) return ["number", "eq"];
  if (type === CUSTOM_FIELD_TYPE.DATE || type === CUSTOM_FIELD_TYPE.DATETIME) {
    return ["date", "between"];
  }
  if (
    type === CUSTOM_FIELD_TYPE.SELECT_SINGLE ||
    type === CUSTOM_FIELD_TYPE.RADIO
  ) {
    return ["select", "in"];
  }
  if (
    type === CUSTOM_FIELD_TYPE.SELECT_MULTI ||
    type === CUSTOM_FIELD_TYPE.CHECKBOX
  ) {
    return ["multi", "in"];
  }
  if (isMemberFieldType(type)) return ["person", "in"];
  if (isCascaderFieldType(type)) return ["cascader", "in"];
  return [];
}

const TEXT_LIKE = new Set([
  CUSTOM_FIELD_TYPE.TEXT,
  CUSTOM_FIELD_TYPE.TEXTAREA,
  CUSTOM_FIELD_TYPE.PHONE,
]);

export function cascaderDepth(nodes = []) {
  if (!nodes.length) return 0;
  return (
    1 + Math.max(...nodes.map((node) => cascaderDepth(node.children || [])))
  );
}
