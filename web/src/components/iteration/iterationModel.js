// Iteration form and API model conversions.
import dayjs from "dayjs";

export function emptyIterationFilterOptions() {
  return {
    workspaces: [],
    statuses: [],
    priorities: [],
    members: [],
    final_status_ids: [],
  };
}

export function newIterationForm(workspaceId) {
  return {
    name: "",
    startDate: null,
    endDate: null,
    iterationType: "single",
    workspaceIds: [Number(workspaceId)],
  };
}

export function resetIterationForm(target, workspaceId, values = {}) {
  Object.assign(target, newIterationForm(workspaceId), values);
}

export function normalizeIteration(item) {
  return {
    ...item,
    id: Number(item.id),
    workspace_id: Number(item.workspace_id),
    ended_at: Number(item.ended_at) || 0,
    is_ended: item.is_ended === true || Number(item.ended_at) > 0,
    requirement_count: Number(item.requirement_count) || 0,
    completed_count: Number(item.completed_count) || 0,
    workspaces: (item.workspaces || []).map((workspace) => ({
      ...workspace,
      id: Number(workspace.id),
    })),
  };
}

export function iterationEditForm(iteration) {
  return {
    name: iteration.name,
    startDate:
      Number(iteration.start_date) > 0
        ? dayjs.unix(Number(iteration.start_date))
        : null,
    endDate:
      Number(iteration.end_date) > 0
        ? dayjs.unix(Number(iteration.end_date))
        : null,
    iterationType: iteration.iteration_type,
    workspaceIds: iteration.workspaces.map((workspace) => Number(workspace.id)),
  };
}

export function iterationPayload(form, workspaceId) {
  const name = form.name.trim();
  if (!name) return { error: "workspace_versionNameRequired" };
  if (
    form.startDate &&
    form.endDate &&
    form.startDate.isAfter(form.endDate, "day")
  ) {
    return { error: "workspace_versionDateInvalid" };
  }
  const workspaceIds = [...new Set((form.workspaceIds || []).map(Number))];
  if (form.iterationType === "cross" && workspaceIds.length < 2) {
    return { error: "iteration_crossWorkspaceRequired" };
  }
  return {
    data: {
      name,
      start_date: form.startDate ? form.startDate.startOf("day").unix() : 0,
      end_date: form.endDate ? form.endDate.startOf("day").unix() : 0,
      iteration_type: form.iterationType,
      workspace_ids:
        form.iterationType === "cross" ? workspaceIds : [Number(workspaceId)],
    },
  };
}
