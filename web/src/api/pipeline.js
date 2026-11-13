import { request } from "./request";

// ======================== Pipeline CRUD ========================

// Pipeline list
export function listPipelines(params = {}) {
  return request({
    url: "/api/pipelines",
    method: "GET",
    params,
  });
}

// Pipeline detail
export function getPipeline(pipelineId) {
  return request({
    url: `/api/pipelines/${pipelineId}`,
    method: "GET",
  });
}

// Create pipeline
export function createPipeline(data) {
  return request({
    url: "/api/pipelines",
    method: "POST",
    data,
  });
}

// Update pipeline basic info and members
export function updatePipeline(pipelineId, data) {
  return request({
    url: `/api/pipelines/${pipelineId}`,
    method: "PUT",
    data,
  });
}

// Update pipeline workflow
export function updatePipelineWorkflow(pipelineId, data) {
  return request({
    url: `/api/pipelines/${pipelineId}/workflow`,
    method: "PUT",
    data,
  });
}

// Delete pipeline
export function deletePipeline(pipelineId) {
  return request({
    url: `/api/pipelines/${pipelineId}`,
    method: "DELETE",
  });
}

// Upload pipeline icon
export function uploadPipelineIcon(pipelineId, file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: `/api/pipelines/${pipelineId}/icon`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

// ======================== Expert group CRUD ========================

// Expert group list
export function listExpertGroups(params = {}) {
  return request({
    url: "/api/expert-groups",
    method: "GET",
    params,
  });
}

// Expert group detail
export function getExpertGroup(expertGroupId) {
  return request({
    url: `/api/expert-groups/${expertGroupId}`,
    method: "GET",
  });
}

// Create expert group
export function createExpertGroup(data) {
  return request({
    url: "/api/expert-groups",
    method: "POST",
    data,
  });
}

// Update expert group
export function updateExpertGroup(expertGroupId, data) {
  return request({
    url: `/api/expert-groups/${expertGroupId}`,
    method: "PUT",
    data,
  });
}

// Delete expert group
export function deleteExpertGroup(expertGroupId) {
  return request({
    url: `/api/expert-groups/${expertGroupId}`,
    method: "DELETE",
  });
}

// ======================== Pipeline tasks ========================

// Pipeline task list
export function listPipelineTasks(params = {}) {
  return request({
    url: "/api/pipeline-tasks",
    method: "GET",
    params,
  });
}

// Pipeline task detail
export function getPipelineTaskDetail(taskId) {
  return request({
    url: `/api/pipeline-tasks/${taskId}`,
    method: "GET",
  });
}

// ======================== Client releases ========================

// Client release list (downloadable versions)
export function listClientReleases(params = {}) {
  return request({
    url: "/api/client/releases",
    method: "GET",
    params,
  });
}

// ======================== Work-item agent tab ========================

// Execution records of a work item grouped by executor (Agent tab of the
// requirement/defect detail drawer). `type` is requirement or defect.
export function listRequirementAgentTasks(wsId, reqId, type = "requirement") {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/agent-tasks`,
    method: "GET",
    params: { type },
  });
}

// One execution record detail of a work item (steps + conversations + stats).
export function getRequirementAgentTaskDetail(
  wsId,
  reqId,
  taskId,
  type = "requirement",
) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/agent-tasks/${taskId}`,
    method: "GET",
    params: { type },
  });
}
