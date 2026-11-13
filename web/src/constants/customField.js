// Custom field type keys stored in workspace_custom_fields.field_type.
export const CUSTOM_FIELD_TYPE = {
  TEXT: "text",
  TEXTAREA: "textarea",
  NUMBER: "number",
  NUMERIC: "numeric",
  PHONE: "phone",
  DATE: "date",
  DATETIME: "datetime",
  SELECT_SINGLE: "select_single",
  SELECT_MULTI: "select_multi",
  RADIO: "radio",
  CHECKBOX: "checkbox",
  USER: "user",
  USER_MULTI: "user_multi",
  CASCADER_SINGLE: "cascader_single",
  CASCADER_MULTI: "cascader_multi",
};

export const CUSTOM_FIELD_MEMBER_SCOPE = {
  ALL: "all",
  ROLES: "roles",
  WORKSPACE: "workspace",
};

export const CUSTOM_FIELD_TYPE_LABELS = {
  [CUSTOM_FIELD_TYPE.TEXT]: "settings_fieldTypeText",
  [CUSTOM_FIELD_TYPE.TEXTAREA]: "settings_fieldTypeText",
  [CUSTOM_FIELD_TYPE.NUMBER]: "settings_fieldTypeNumber",
  [CUSTOM_FIELD_TYPE.NUMERIC]: "settings_fieldTypeNumeric",
  [CUSTOM_FIELD_TYPE.PHONE]: "settings_fieldTypePhone",
  [CUSTOM_FIELD_TYPE.DATE]: "settings_fieldTypeDate",
  [CUSTOM_FIELD_TYPE.DATETIME]: "settings_fieldTypeDateTime",
  [CUSTOM_FIELD_TYPE.SELECT_SINGLE]: "settings_fieldTypeSelect",
  [CUSTOM_FIELD_TYPE.SELECT_MULTI]: "settings_fieldTypeMultiSelect",
  [CUSTOM_FIELD_TYPE.RADIO]: "settings_fieldTypeRadio",
  [CUSTOM_FIELD_TYPE.CHECKBOX]: "settings_fieldTypeCheckbox",
  [CUSTOM_FIELD_TYPE.USER]: "settings_fieldTypeSingleMember",
  [CUSTOM_FIELD_TYPE.USER_MULTI]: "settings_fieldTypeMultiMember",
  [CUSTOM_FIELD_TYPE.CASCADER_SINGLE]: "settings_fieldTypeCascaderRadio",
  [CUSTOM_FIELD_TYPE.CASCADER_MULTI]: "settings_fieldTypeCascaderCheckbox",
};

// Types offered by the field editor. Radio and checkbox remain constants only
// for compatibility while the cleanup migration removes their test data.
export const CUSTOM_FIELD_TYPE_GROUPS = [
  {
    labelKey: "settings_fieldTypeGroupInput",
    options: [
      CUSTOM_FIELD_TYPE.TEXT,
      CUSTOM_FIELD_TYPE.NUMBER,
      CUSTOM_FIELD_TYPE.PHONE,
      CUSTOM_FIELD_TYPE.NUMERIC,
    ],
  },
  {
    labelKey: "settings_fieldTypeGroupTime",
    options: [CUSTOM_FIELD_TYPE.DATE, CUSTOM_FIELD_TYPE.DATETIME],
  },
  {
    labelKey: "settings_fieldTypeGroupSelect",
    options: [CUSTOM_FIELD_TYPE.SELECT_SINGLE, CUSTOM_FIELD_TYPE.SELECT_MULTI],
  },
  {
    labelKey: "settings_fieldTypeGroupMember",
    options: [CUSTOM_FIELD_TYPE.USER, CUSTOM_FIELD_TYPE.USER_MULTI],
  },
  {
    labelKey: "settings_fieldTypeGroupCascader",
    options: [
      CUSTOM_FIELD_TYPE.CASCADER_SINGLE,
      CUSTOM_FIELD_TYPE.CASCADER_MULTI,
    ],
  },
];

export const OPTION_FIELD_TYPES = [
  CUSTOM_FIELD_TYPE.SELECT_SINGLE,
  CUSTOM_FIELD_TYPE.SELECT_MULTI,
  CUSTOM_FIELD_TYPE.RADIO,
  CUSTOM_FIELD_TYPE.CHECKBOX,
];

export const MEMBER_FIELD_TYPES = [
  CUSTOM_FIELD_TYPE.USER,
  CUSTOM_FIELD_TYPE.USER_MULTI,
];

export const CASCADER_FIELD_TYPES = [
  CUSTOM_FIELD_TYPE.CASCADER_SINGLE,
  CUSTOM_FIELD_TYPE.CASCADER_MULTI,
];

export const NUMERIC_FIELD_TYPES = [
  CUSTOM_FIELD_TYPE.NUMBER,
  CUSTOM_FIELD_TYPE.NUMERIC,
];

export const DATE_FIELD_TYPES = [
  CUSTOM_FIELD_TYPE.DATE,
  CUSTOM_FIELD_TYPE.DATETIME,
];

export const MULTI_VALUE_FIELD_TYPES = [
  CUSTOM_FIELD_TYPE.SELECT_MULTI,
  CUSTOM_FIELD_TYPE.CHECKBOX,
  CUSTOM_FIELD_TYPE.USER_MULTI,
  CUSTOM_FIELD_TYPE.CASCADER_MULTI,
];

export const ARRAY_VALUE_FIELD_TYPES = [
  ...MULTI_VALUE_FIELD_TYPES,
  CUSTOM_FIELD_TYPE.CASCADER_SINGLE,
];

export const TEXT_FIELD_TYPES = [
  CUSTOM_FIELD_TYPE.TEXT,
  CUSTOM_FIELD_TYPE.TEXTAREA,
  CUSTOM_FIELD_TYPE.PHONE,
];

export const CUSTOM_FIELD_PHONE_RE = /^1[3-9]\d{9}$/;

export const CUSTOM_FIELD_CASCADER_MAX_DEPTH = 4;
export const CUSTOM_FIELD_CASCADER_NAME_MAX_LENGTH = 20;

export const NUMERIC_SEARCH_OPERATORS = [
  "eq",
  "ne",
  "between",
  "empty",
  "not_empty",
];

export const NUMERIC_VIEW_OPERATORS = [
  "eq",
  "ne",
  "gt",
  "lt",
  "gte",
  "lte",
  "between",
  "empty",
  "not_empty",
];

export const MEMBER_VIEW_OPERATORS = [
  "in",
  "not_in",
  "contains",
  "not_contains",
  "eq",
  "ne",
  "empty",
  "not_empty",
];

export function isOptionFieldType(type) {
  return OPTION_FIELD_TYPES.includes(type);
}

export function isMemberFieldType(type) {
  return MEMBER_FIELD_TYPES.includes(type);
}

export function isCascaderFieldType(type) {
  return CASCADER_FIELD_TYPES.includes(type);
}

export function isNumericFieldType(type) {
  return NUMERIC_FIELD_TYPES.includes(type);
}

export function customFieldViewOperators(fieldType) {
  if (isNumericFieldType(fieldType)) return NUMERIC_VIEW_OPERATORS;
  if (isDateFieldType(fieldType)) return ["between", "empty", "not_empty"];
  if (isMemberFieldType(fieldType)) return MEMBER_VIEW_OPERATORS;
  if (TEXT_FIELD_TYPES.includes(fieldType)) {
    return ["contains", "not_contains", "empty", "not_empty", "eq", "ne"];
  }
  if (isOptionFieldType(fieldType) || isCascaderFieldType(fieldType)) {
    return ["in", "not_in", "empty", "not_empty"];
  }
  return [];
}

export function isDateFieldType(type) {
  return DATE_FIELD_TYPES.includes(type);
}

export function isArrayFieldType(type) {
  return ARRAY_VALUE_FIELD_TYPES.includes(type);
}

export function isMultiValueFieldType(type) {
  return MULTI_VALUE_FIELD_TYPES.includes(type);
}
