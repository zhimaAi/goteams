export const GANTT_DATE_SCALE_OPTIONS = Object.freeze([
  { value: "week", labelKey: "gantt_scale_week" },
  { value: "month", labelKey: "gantt_scale_month" },
  { value: "quarter", labelKey: "gantt_scale_quarter" },
  { value: "year", labelKey: "gantt_scale_year" },
]);

export const GANTT_DATE_SCALES = Object.freeze(
  GANTT_DATE_SCALE_OPTIONS.map((option) => option.value),
);

export const GANTT_FIELD_CELL_TYPES = Object.freeze({
  TEXT: "text",
  TITLE: "title",
  STATUS: "status",
  PRIORITY: "priority",
  ASSIGNEE: "assignee",
});

export const GANTT_LIST_FIELD_DEFINITIONS = Object.freeze([
  {
    configKey: "id",
    labelKey: "workspace_id",
    width: 88,
    cellType: GANTT_FIELD_CELL_TYPES.TEXT,
  },
  {
    configKey: "title",
    labelKey: "workspace_workItemTitle",
    width: 240,
    cellType: GANTT_FIELD_CELL_TYPES.TITLE,
  },
  {
    configKey: "status",
    labelKey: "workspace_status",
    width: 112,
    cellType: GANTT_FIELD_CELL_TYPES.STATUS,
  },
  {
    configKey: "version",
    labelKey: "requirement_filterVersion",
    width: 120,
    cellType: GANTT_FIELD_CELL_TYPES.TEXT,
  },
  {
    configKey: "priority",
    labelKey: "workspace_priority",
    width: 112,
    cellType: GANTT_FIELD_CELL_TYPES.PRIORITY,
  },
  {
    configKey: "planned_start_date",
    labelKey: "workspace_plannedStartDateCol",
    width: 128,
    cellType: GANTT_FIELD_CELL_TYPES.TEXT,
  },
  {
    configKey: "planned_end_date",
    labelKey: "workspace_plannedEndDateCol",
    width: 128,
    cellType: GANTT_FIELD_CELL_TYPES.TEXT,
  },
  {
    configKey: "assignee",
    labelKey: "workspace_assignee",
    width: 144,
    cellType: GANTT_FIELD_CELL_TYPES.ASSIGNEE,
  },
  {
    configKey: "created_at",
    labelKey: "workspace_createdTime",
    width: 160,
    cellType: GANTT_FIELD_CELL_TYPES.TEXT,
  },
  {
    configKey: "created_by",
    labelKey: "workspace_createdBy",
    width: 120,
    cellType: GANTT_FIELD_CELL_TYPES.TEXT,
  },
]);
