// workItemView.js
// View filter constants shared by the view editor components.
// The first operator of each field is the default after switching fields.

export const VIEW_FILTER_FIELD_OPERATORS = {
  id: ["eq"],
  title: ["contains", "not_contains", "empty", "not_empty", "eq", "ne"],
  template: ["in", "not_in", "empty", "not_empty"],
  status: ["in", "not_in", "empty", "not_empty"],
  priority: ["in", "not_in", "empty", "not_empty"],
  assignment_age: ["within_hours"],
  my_assignment_age: ["within_hours", "older_than_hours"],
  assignee: [
    "in",
    "not_in",
    "contains",
    "not_contains",
    "eq",
    "ne",
    "empty",
    "not_empty",
  ],
  creator: [
    "in",
    "not_in",
    "contains",
    "not_contains",
    "eq",
    "ne",
    "empty",
    "not_empty",
  ],
  version: ["in", "not_in", "empty", "not_empty"],
  workspace: ["in", "not_in"],
  work_item_type: ["in", "not_in"],
  category: ["in", "not_in", "empty", "not_empty"],
  aging_status: ["in", "not_in"],
  planned_start_date: ["eq", "between", "dynamic", "empty", "not_empty"],
  planned_end_date: ["eq", "between", "dynamic", "empty", "not_empty"],
  created_at: [
    "eq",
    "between",
    "dynamic",
    "empty",
    "not_empty",
    "older_than_hours",
  ],
};

export const DEFAULT_SEARCH_FILTER_FIELDS = [
  "title",
  "status",
  "assignee",
  "version",
  "creator",
];

export const SEARCH_FILTER_FIELDS = [
  "id",
  "title",
  "template",
  "status",
  "assignee",
  "version",
  "workspace",
  "work_item_type",
  "creator",
  "priority",
  "category",
  "planned_start_date",
  "planned_end_date",
  "created_at",
];

export const VIEW_DATE_FIELDS = [
  "planned_start_date",
  "planned_end_date",
  "created_at",
];

export const VIEW_DYNAMIC_UNITS = [
  { key: "month_before", unit: "month", sign: -1 },
  { key: "week_before", unit: "week", sign: -1 },
  { key: "day_before", unit: "day", sign: -1 },
  { key: "today", unit: "day", sign: 0 },
  { key: "month_after", unit: "month", sign: 1 },
  { key: "week_after", unit: "week", sign: 1 },
  { key: "day_after", unit: "day", sign: 1 },
];

export function parseDynamicDate(value) {
  const [rawOffset, rawUnit] = String(value || "").split(":");
  const offset = Number(rawOffset);
  if (!rawUnit || !Number.isFinite(offset)) return { amount: 1, key: "" };
  const sign = offset === 0 ? 0 : Math.sign(offset);
  const matched = VIEW_DYNAMIC_UNITS.find(
    (item) => item.unit === rawUnit && item.sign === sign,
  );
  if (!matched) return { amount: 1, key: "" };
  return { amount: Math.abs(offset) || 1, key: matched.key };
}

export function formatDynamicDate({ amount, key }) {
  const matched = VIEW_DYNAMIC_UNITS.find((item) => item.key === key);
  if (!matched) return "";
  const offset = matched.sign * (Number(amount) || 1);
  return `${matched.sign === 0 ? 0 : offset}:${matched.unit}`;
}

export function defaultSearchOperator(field) {
  if (field === "title") return "contains";
  if (field === "id") return "eq";
  if (
    ["planned_start_date", "planned_end_date", "created_at"].includes(field)
  ) {
    return "between";
  }
  return "in";
}

function normalizeSearchOperator(field, operator) {
  const defaultOperator = defaultSearchOperator(field);
  if (typeof operator !== "string") return defaultOperator;
  const builtinOperators = VIEW_FILTER_FIELD_OPERATORS[field];
  if (builtinOperators) {
    return builtinOperators.includes(operator) ? operator : defaultOperator;
  }
  if (/^custom:\d+$/.test(field)) {
    const customOperators = [
      "contains",
      "not_contains",
      "eq",
      "ne",
      "gt",
      "gte",
      "lt",
      "lte",
      "in",
      "not_in",
      "between",
      "empty",
      "not_empty",
    ];
    return customOperators.includes(operator) ? operator : defaultOperator;
  }
  return defaultOperator;
}

export function newSearchFilterCondition(field, operator) {
  return {
    field,
    operator: normalizeSearchOperator(field, operator),
    values: [],
  };
}

export function createDefaultSearchFilters(typeKey = "requirement") {
  const fields =
    typeKey === "iteration"
      ? [
          "title",
          "workspace",
          "work_item_type",
          "status",
          "priority",
          "assignee",
          "creator",
        ]
      : [
          ...DEFAULT_SEARCH_FILTER_FIELDS.slice(0, 1),
          ...(["requirement", "defect"].includes(typeKey) ? ["template"] : []),
          ...DEFAULT_SEARCH_FILTER_FIELDS.slice(1),
        ];
  return {
    logic: "and",
    conditions: fields.map(newSearchFilterCondition),
  };
}

export function normalizeSearchFilterTree(tree, typeKey = "requirement") {
  if (!tree || !Array.isArray(tree.conditions)) {
    tree = createDefaultSearchFilters(typeKey);
  }
  const allowed = new Set(
    SEARCH_FILTER_FIELDS.filter(
      (field) => field !== "category" || typeKey === "requirement",
    ),
  );
  if (typeKey === "requirement") allowed.add("aging_status");
  if (!["requirement", "defect"].includes(typeKey)) allowed.delete("template");
  const seen = new Set();
  const conditions = [];
  for (const node of tree?.conditions || []) {
    const dynamicCustom = /^custom:\d+$/.test(node?.field || "");
    if (
      !node?.field ||
      (!allowed.has(node.field) && !dynamicCustom) ||
      seen.has(node.field)
    ) {
      continue;
    }
    seen.add(node.field);
    conditions.push({
      field: node.field,
      operator: normalizeSearchOperator(node.field, node.operator),
      values: Array.isArray(node.values) ? node.values.map(String) : [],
    });
  }
  const title = conditions.find((condition) => condition.field === "title");
  const template = conditions.find(
    (condition) => condition.field === "template",
  );
  const others = conditions.filter(
    (condition) => !["title", "template"].includes(condition.field),
  );
  return {
    logic: "and",
    conditions: [
      title || newSearchFilterCondition("title"),
      ...(["requirement", "defect"].includes(typeKey)
        ? [template || newSearchFilterCondition("template")]
        : []),
      ...others,
    ],
  };
}

// Iteration filters merge the same status/priority name from several projects and work item
// types into one option whose value carries every underlying id, joined by ",".
export function expandOptionValues(values) {
  return (values || []).flatMap((value) => String(value).split(","));
}

// collapseOptionValues maps flat ids back onto the option values a picker renders; values that
// belong to no option (placeholders such as __all_end__) are kept as they are.
export function collapseOptionValues(values, options) {
  const selected = new Set((values || []).map(String));
  const optionValues = (options || []).map((option) =>
    String(option.value ?? option.id),
  );
  const covered = new Set(optionValues.flatMap((value) => value.split(",")));
  return [
    ...optionValues.filter((value) =>
      value.split(",").every((id) => selected.has(id)),
    ),
    ...[...selected].filter((value) => !covered.has(value)),
  ];
}

export function hasSearchFilterValues(tree) {
  return (tree?.conditions || []).some(
    (condition) =>
      ["empty", "not_empty"].includes(condition.operator) ||
      condition.values?.some((value) => String(value).trim().length > 0),
  );
}

// Dynamic placeholder values expanded by the backend at query time.
export const VIEW_VALUE_CURRENT_USER = "__me__";
export const VIEW_VALUE_CURRENT_ENTERPRISE_ROLE = "__current_enterprise_role__";
export const VIEW_VALUE_ALL = "__all__";
export const VIEW_VALUE_EMPTY = "__empty__";
export const VIEW_VALUE_NOT_EMPTY = "__not_empty__";
export const VIEW_VALUE_ALL_COMPLETED = "__all_completed__";
export const VIEW_VALUE_ALL_CLOSED = "__all_closed__";
export const VIEW_VALUE_ALL_END = "__all_end__";
export const VIEW_VALUE_ALL_NOT_END = "__all_not_end__";

// newViewFilterCondition returns an empty condition row (title/contains).
export function newViewFilterCondition() {
  return { field: "title", operator: "contains", values: [] };
}

// newViewFilterGroup returns a sub-group seeded with one empty condition.
export function newViewFilterGroup() {
  return { logic: "and", conditions: [newViewFilterCondition()] };
}

// normalizeViewFilterTree deep-clones a stored filter tree into an editable
// structure, filling defaults for missing fields; null yields an empty root group.
export function normalizeViewFilterTree(tree) {
  if (!tree || typeof tree !== "object" || tree.field) {
    return { logic: "and", conditions: [] };
  }
  return normalizeGroup(tree);
}

function normalizeGroup(node) {
  return {
    logic: node.logic === "or" ? "or" : "and",
    conditions: (node.conditions || []).map((child) =>
      child.field
        ? {
            field: child.field,
            operator:
              child.operator ||
              VIEW_FILTER_FIELD_OPERATORS[child.field]?.[0] ||
              "in",
            values: Array.isArray(child.values) ? child.values.map(String) : [],
          }
        : normalizeGroup(child),
    ),
  };
}
