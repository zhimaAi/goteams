// Iteration list field, filter, and column definitions.
import {
  expandOptionValues,
  VIEW_VALUE_ALL_COMPLETED,
  VIEW_VALUE_ALL_CLOSED,
  VIEW_VALUE_ALL_END,
  VIEW_VALUE_ALL_NOT_END,
  VIEW_VALUE_CURRENT_USER,
} from "@/constants/workItemView";

// mergeByName groups one status/priority name across every project and work item type into a
// single option; its value carries all underlying ids, so one pick covers them all. Without
// this, a cross-project iteration lists "Pending" once per project and per type, and picking
// one of them silently filters away the rest.
function mergeByName(items) {
  const groups = new Map();
  for (const item of items || []) {
    const group = groups.get(item.name);
    if (group) group.ids.push(String(item.id));
    else
      groups.set(item.name, {
        name: item.name,
        color: item.color,
        ids: [String(item.id)],
      });
  }
  return [...groups.values()].map((group) => ({
    id: group.ids.join(","),
    name: group.name,
    label: group.name,
    color: group.color,
  }));
}

export function iterationFilterSources(filterOptions) {
  return {
    statuses: mergeByName(filterOptions.statuses),
    priorities: mergeByName(filterOptions.priorities),
    members: (filterOptions.members || []).map((item) => ({
      user_id: item.user_id,
      user_name: item.name,
      user_username: item.username,
      user_avatar: item.avatar,
    })),
  };
}

// includeDefect mirrors the defect app switch: with the app off the type filter must not
// offer defects. sanitizeIterationSearchFilters reads the options below, so saved defect
// values are dropped from the query on their own.
export function iterationFieldDescriptors(
  filterOptions,
  t,
  includeDefect = true,
) {
  return [
    {
      field: "id",
      label: t("workspace_id"),
      control: "text",
      operator: "eq",
    },
    {
      field: "title",
      label: t("workspace_workItemTitle"),
      control: "text",
      operator: "contains",
    },
    {
      field: "workspace",
      label: t("iteration_workspaceColumn"),
      control: "multi",
      operator: "in",
      options: (filterOptions.workspaces || []).map((item) => ({
        value: String(item.id),
        label: item.name,
      })),
    },
    {
      field: "work_item_type",
      label: t("workspace_type"),
      control: "multi",
      operator: "in",
      options: [
        { value: "requirement", label: t("requirement_feature") },
        ...(includeDefect
          ? [{ value: "defect", label: t("requirement_bug") }]
          : []),
      ],
    },
    {
      field: "status",
      label: t("workspace_status"),
      control: "multi",
      operator: "in",
    },
    {
      field: "priority",
      label: t("workspace_priority"),
      control: "multi",
      operator: "in",
    },
    {
      field: "assignee",
      label: t("workspace_assignee"),
      control: "person",
      operator: "in",
    },
    {
      field: "planned_start_date",
      label: t("workspace_plannedStartDateCol"),
      control: "date",
      operator: "between",
    },
    {
      field: "planned_end_date",
      label: t("workspace_plannedEndDateCol"),
      control: "date",
      operator: "between",
    },
    {
      field: "creator",
      label: t("workspace_createdBy"),
      control: "person",
      operator: "in",
    },
  ];
}

// sanitizeIterationSearchFilters drops values the current iteration no longer offers.
// Each iteration exposes its own workspaces, statuses, priorities and members, so filters
// kept from a previously opened iteration would silently return nothing while the search
// bar shows no active condition.
// groupIdsById maps every id of a merged option back to the full id list of its group, so a
// filter saved before the merge (a single id) is upgraded to the whole group.
function groupIdsById(items) {
  const byId = new Map();
  for (const item of items || []) {
    const ids = String(item.id).split(",");
    for (const id of ids) byId.set(id, ids);
  }
  return byId;
}

export function sanitizeIterationSearchFilters(filters, sources, descriptors) {
  const memberIds = new Set([
    VIEW_VALUE_CURRENT_USER,
    ...(sources.members || []).map((item) => String(item.user_id)),
  ]);
  const allowedByField = new Map([
    [
      "status",
      new Set([
        VIEW_VALUE_ALL_COMPLETED,
        VIEW_VALUE_ALL_CLOSED,
        VIEW_VALUE_ALL_END,
        VIEW_VALUE_ALL_NOT_END,
        ...expandOptionValues((sources.statuses || []).map((item) => item.id)),
      ]),
    ],
    [
      "priority",
      new Set([
        "0",
        ...expandOptionValues(
          (sources.priorities || []).map((item) => item.id),
        ),
      ]),
    ],
    ["assignee", memberIds],
    ["creator", memberIds],
  ]);
  for (const descriptor of descriptors) {
    if (descriptor.options?.length) {
      allowedByField.set(
        descriptor.field,
        new Set(descriptor.options.map((option) => String(option.value))),
      );
    }
  }
  const groupsByField = new Map([
    ["status", groupIdsById(sources.statuses)],
    ["priority", groupIdsById(sources.priorities)],
  ]);
  return {
    ...filters,
    conditions: (filters.conditions || []).map((condition) => {
      const allowed = allowedByField.get(condition.field);
      if (!allowed || !condition.values?.length) return condition;
      const groups = groupsByField.get(condition.field);
      let values = condition.values
        .map(String)
        .filter((value) => allowed.has(value));
      if (groups) {
        values = [
          ...new Set(values.flatMap((value) => groups.get(value) || [value])),
        ];
      }
      return JSON.stringify(values) === JSON.stringify(condition.values)
        ? condition
        : { ...condition, values };
    }),
  };
}

export function iterationListFields(t) {
  return [
    field("title", "workspace_workItemTitle", t, true, true),
    field("workspace", "iteration_workspaceColumn", t, true),
    field("type", "workspace_type", t, false),
    field("status", "workspace_status", t, true),
    field("priority", "workspace_priority", t, true),
    field("assignee", "workspace_assignee", t, true),
    field("plannedStart", "workspace_plannedStartDateCol", t, true),
    field("plannedEnd", "workspace_plannedEndDateCol", t, true),
    field("createdBy", "workspace_createdBy", t, true),
    field("createdAt", "workspace_createdTime", t, false),
  ];
}

function field(key, labelKey, t, visible, locked = false) {
  return {
    kind: "builtin",
    preferenceKey: key,
    label: t(labelKey),
    visible,
    locked,
  };
}

export function effectiveIterationFields(definitions, preferences) {
  if (!preferences.length) return definitions;
  const byKey = new Map(definitions.map((item) => [item.preferenceKey, item]));
  const result = [];
  for (const preference of preferences) {
    const key = preference.preferenceKey || preference.key;
    const item = byKey.get(key);
    if (!item) continue;
    result.push({
      ...item,
      visible: item.locked || preference.visible !== false,
    });
    byKey.delete(key);
  }
  result.push(...byKey.values());
  return result;
}

// sortField must stay inside the backend whitelist for the iteration work-item
// list; a column without one simply gets no header sort button.
export function iterationColumns(t) {
  return {
    title: column("workspace_workItemTitle", "title", 320, t, {
      minWidth: 240,
      maxWidth: 1000,
    }),
    workspace: column("iteration_workspaceColumn", "workspace", 150, t, {
      dataIndex: "workspace_name",
      minWidth: 110,
    }),
    type: column("workspace_type", "type", 90, t, { minWidth: 80 }),
    status: column("workspace_status", "status", 110, t, {
      minWidth: 96,
      sortField: "status",
    }),
    priority: column("workspace_priority", "priority", 100, t, {
      minWidth: 96,
      sortField: "priority",
    }),
    assignee: column("workspace_assignee", "assignee", 130, t, {
      minWidth: 110,
      sortField: "assignee",
    }),
    plannedStart: column(
      "workspace_plannedStartDateCol",
      "plannedStart",
      130,
      t,
      { minWidth: 124, sortField: "planned_start_date" },
    ),
    plannedEnd: column("workspace_plannedEndDateCol", "plannedEnd", 130, t, {
      minWidth: 124,
      sortField: "planned_end_date",
    }),
    createdBy: column("workspace_createdBy", "createdBy", 110, t, {
      dataIndex: "creator_name",
      minWidth: 100,
      sortField: "created_by",
    }),
    createdAt: column("workspace_createdTime", "createdAt", 160, t, {
      minWidth: 140,
      sortField: "created_at",
    }),
  };
}

function column(titleKey, key, width, t, options = {}) {
  return {
    title: t(titleKey),
    key,
    width,
    dataIndex: options.dataIndex,
    minWidth: options.minWidth || 80,
    maxWidth: options.maxWidth || 600,
    sortField: options.sortField,
    ellipsis: key === "title",
  };
}
