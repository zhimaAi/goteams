export function patchLoadedWorkItem(items, childrenMap, id, values) {
  const patch = (item) => {
    if (String(item.id) === String(id)) Object.assign(item, values);
  };

  items.forEach(patch);
  Object.values(childrenMap).forEach((state) => state.list.forEach(patch));
}

export function mergeWorkItem(target, source) {
  if (!target || !source) return target;
  Object.entries(source).forEach(([key, value]) => {
    if (value !== undefined) target[key] = value;
  });
  return target;
}

function assigneeOptionMap(item, members) {
  const previousIds = Array.isArray(item.assignee_ids)
    ? item.assignee_ids.map(Number)
    : [];
  const result = new Map(
    previousIds.map((id, index) => [
      id,
      {
        name: item.assignee_names?.[index] || "",
        avatar: item.assignee_avatars?.[index] || "",
        username: item.assignee_usernames?.[index] || "",
      },
    ]),
  );
  (members || []).forEach((member) => {
    result.set(Number(member.user_id), {
      name: member.user_name || "",
      avatar: member.user_avatar || "",
      username: member.user_username || "",
    });
  });
  return result;
}

// Builds the list projection of a work item after a successful inline update.
export function buildWorkItemListUpdate(item, updates, options = {}) {
  const next = {
    ...item,
    ...updates,
    updated_at: options.updatedAt ?? Math.floor(Date.now() / 1000),
  };

  if (Object.hasOwn(updates, "status_id")) {
    const status = (options.statuses || []).find(
      (candidate) => Number(candidate.id) === Number(updates.status_id),
    );
    next.status_name = status?.name || "";
    next.status_color = status?.color || "";
    // The cross-project lists sort by status in memory, so the row must carry
    // the new status order instead of waiting for the next fetch.
    next.status_sort_order = Number(status?.sort_order) || 0;
    next.is_done = (options.finalStatusIds || [])
      .map(Number)
      .includes(Number(updates.status_id));
    next.is_completed = (options.completedStatusIds || [])
      .map(Number)
      .includes(Number(updates.status_id));
    next.available_status_ids = null;
  }

  if (Object.hasOwn(updates, "priority")) {
    const priorityId = Number(updates.priority) || 0;
    const priority = (options.priorities || []).find(
      (candidate) => Number(candidate.id) === priorityId,
    );
    next.priority = priorityId;
    next.priority_name = priority?.name || priority?.label || "";
    next.priority_color = priority?.color || "";
    next.priority_sort_order = Number(priority?.sort_order) || 0;
  }

  if (Object.hasOwn(updates, "assignee_ids")) {
    const assigneeIds = Array.isArray(updates.assignee_ids)
      ? updates.assignee_ids.map(Number)
      : [];
    const assignees = assigneeOptionMap(item, options.members);
    next.assignee_ids = assigneeIds;
    next.assignee_names = assigneeIds.map(
      (id) => assignees.get(id)?.name || "",
    );
    next.assignee_avatars = assigneeIds.map(
      (id) => assignees.get(id)?.avatar || "",
    );
    next.assignee_usernames = assigneeIds.map(
      (id) => assignees.get(id)?.username || "",
    );
  }

  return next;
}

export function workItemMatchesFilters(item, filters = {}) {
  if (!matchesSearchFilterTree(item, filters)) return false;
  const keywords = String(filters.search || "")
    .trim()
    .toLowerCase()
    .split(/\s+/)
    .filter(Boolean);
  const title = String(item.title || "").toLowerCase();
  if (keywords.some((keyword) => !title.includes(keyword))) {
    return false;
  }
  const statusIds = (filters.statusIds || []).map(Number);
  if (filters.statusActive && !statusIds.includes(Number(item.status_id))) {
    return false;
  }
  const versionIds = (filters.versionIds || []).map(Number);
  if (versionIds.length && !versionIds.includes(Number(item.version_id) || 0)) {
    return false;
  }
  const createdByIds = (filters.createdByIds || []).map(Number);
  if (createdByIds.length && !createdByIds.includes(Number(item.created_by))) {
    return false;
  }
  const priorityIds = (filters.priorityIds || []).map(Number);
  if (priorityIds.length && !priorityIds.includes(Number(item.priority) || 0)) {
    return false;
  }
  const assigneeIds = (
    Array.isArray(item.assignee_ids) ? item.assignee_ids : []
  ).map(Number);
  const filterAssigneeIds = (filters.assigneeIds || []).map(Number);
  return (
    !filterAssigneeIds.length ||
    filterAssigneeIds.some((id) => assigneeIds.includes(id))
  );
}

function matchesSearchFilterTree(item, filters) {
  if (!filters.searchFilters) return true;
  let tree;
  try {
    tree =
      typeof filters.searchFilters === "string"
        ? JSON.parse(filters.searchFilters)
        : filters.searchFilters;
  } catch {
    return true;
  }
  return (tree?.conditions || []).every((condition) =>
    matchesSearchCondition(item, condition, filters),
  );
}

function matchesSearchCondition(item, condition, filters) {
  const values = (condition.values || [])
    .map((value) => String(value).trim())
    .filter(Boolean);
  const operator = condition.operator || "in";
  if (["empty", "not_empty"].includes(operator)) {
    const value = searchFieldValue(item, condition.field);
    const empty =
      value === undefined ||
      value === null ||
      value === "" ||
      value === 0 ||
      (Array.isArray(value) && value.length === 0);
    return operator === "empty" ? empty : !empty;
  }
  if (!values.length) return true;
  if (condition.field === "category") return true;
  if (condition.field === "title") {
    const title = String(item.title || "").toLowerCase();
    return values
      .join(" ")
      .toLowerCase()
      .split(/\s+/)
      .filter(Boolean)
      .every((value) => title.includes(value));
  }
  if (condition.field === "id") {
    return Number(item.id) === Number(values[0]);
  }
  const value = searchFieldValue(item, condition.field);
  if (operator === "between") {
    if (condition.field.startsWith("custom:")) {
      if (/^\d{4}-\d{2}-\d{2}$/.test(String(value || ""))) {
        const currentDate = String(value);
        return (
          (!values[0] || currentDate >= values[0]) &&
          (!values[1] || currentDate <= values[1])
        );
      }
      const currentNumber = Number(value);
      return (
        Number.isFinite(currentNumber) &&
        (!values[0] || currentNumber >= Number(values[0])) &&
        (!values[1] || currentNumber <= Number(values[1]))
      );
    }
    const current = normalizeSearchDate(value);
    if (!current) return false;
    return (
      (!values[0] || current >= values[0]) &&
      (!values[1] || current <= values[1])
    );
  }
  if (condition.field.startsWith("custom:")) {
    const custom = Array.isArray(value)
      ? value.map(String)
      : [String(value ?? "")];
    if (operator === "contains")
      return custom.some((entry) => entry.includes(values[0] || ""));
    if (operator === "not_contains")
      return !custom.some((entry) => entry.includes(values[0] || ""));
    if (operator === "eq") return custom.includes(values[0]);
    if (operator === "ne") return !custom.includes(values[0]);
    if (operator === "not_in")
      return !custom.some((entry) => values.includes(entry));
    return custom.some((entry) => values.includes(entry));
  }
  if (condition.field === "assignee") {
    const ids = (Array.isArray(item.assignee_ids) ? item.assignee_ids : []).map(
      String,
    );
    return operator === "not_in"
      ? !ids.some((id) => values.includes(id))
      : ids.some((id) => values.includes(id));
  }
  if (
    ["status", "version", "priority", "creator", "template"].includes(
      condition.field,
    )
  ) {
    if (values.some((value) => value.startsWith("__all_"))) return true;
    const current = String(Number(value) || 0);
    return operator === "not_in"
      ? !values.includes(current)
      : values.includes(current);
  }
  return true;
}

function searchFieldValue(item, field) {
  if (field.startsWith("custom:")) {
    const id = field.slice("custom:".length);
    const raw = item.custom_fields?.[id];
    if (typeof raw === "string" && raw.trim().startsWith("[")) {
      try {
        return JSON.parse(raw);
      } catch {
        return raw;
      }
    }
    return raw;
  }
  if (field === "creator") return item.created_by;
  if (field === "version") return Number(item.version_id) || 0;
  if (field === "priority") return Number(item.priority) || 0;
  if (field === "template") return Number(item.template_id) || 0;
  if (field === "status") return Number(item.status_id) || 0;
  return item[field];
}

function normalizeSearchDate(value) {
  if (value === undefined || value === null || value === "") return "";
  if (/^\d{4}-\d{2}-\d{2}$/.test(String(value))) return String(value);
  const date = new Date(Number(value) * 1000);
  if (Number.isNaN(date.getTime())) return "";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}
