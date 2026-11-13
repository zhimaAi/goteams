import { request, axios } from "./request";

// ======================== Planning group ========================

export function listPlanningGroups(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/planning-groups`,
    method: "GET",
  });
}

export function createPlanningGroup(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/planning-groups`,
    method: "POST",
    data,
  });
}

export function updatePlanningGroup(wsId, groupId, data) {
  return request({
    url: `/api/workspaces/${wsId}/planning-groups/${groupId}`,
    method: "PUT",
    data,
  });
}

export function deletePlanningGroup(wsId, groupId) {
  return request({
    url: `/api/workspaces/${wsId}/planning-groups/${groupId}`,
    method: "DELETE",
  });
}

// ======================== Planning row ========================

export function listPlannings(wsId, params) {
  return request({
    url: `/api/workspaces/${wsId}/plannings`,
    method: "GET",
    params,
  });
}

export function createPlanning(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/plannings`,
    method: "POST",
    data,
  });
}

export function updatePlanning(wsId, planningId, data) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/${planningId}`,
    method: "PUT",
    data,
  });
}

export function deletePlanning(wsId, planningId) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/${planningId}`,
    method: "DELETE",
  });
}

// Drag sort: prevId/nextId of 0 mean top/bottom respectively.
export function movePlanning(wsId, planningId, data) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/${planningId}/move`,
    method: "PUT",
    data,
  });
}

// Fully overwrite the planning's related work items.
export function savePlanningRelations(wsId, planningId, itemIds) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/${planningId}/relations`,
    method: "PUT",
    data: { item_ids: itemIds },
  });
}

// Batch create: one planning name per line, appended to the end of the group.
export function batchCreatePlannings(wsId, groupId, titles) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/batch`,
    method: "POST",
    data: { group_id: groupId, titles },
  });
}

// ======================== Recycle bin ========================

export function listDeletedPlannings(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/recycle-bin`,
    method: "GET",
  });
}

export function restorePlannings(wsId, ids) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/recycle-bin/restore`,
    method: "POST",
    data: { ids },
  });
}

export function purgePlannings(wsId, ids) {
  return request({
    url: `/api/workspaces/${wsId}/plannings/recycle-bin/purge`,
    method: "POST",
    data: { ids },
  });
}

// ======================== Excel import / export ========================

function browserTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

export function downloadPlanningImportTemplate(wsId) {
  return axios.get(`/api/workspaces/${wsId}/plannings/import-template`, {
    params: { timezone: browserTimezone() },
    responseType: "blob",
  });
}

// Export follows the on-screen filters so the file matches the current list.
export function exportPlannings(wsId, params = {}) {
  return axios.get(`/api/workspaces/${wsId}/plannings/export`, {
    params: { ...params, timezone: browserTimezone() },
    responseType: "blob",
  });
}

export function importPlannings(wsId, groupId, file) {
  const data = new FormData();
  data.append("file", file);
  data.append("timezone", browserTimezone());
  return request({
    url: `/api/workspaces/${wsId}/plannings/import`,
    method: "POST",
    params: { group_id: groupId },
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}

export function previewPlanningImport(wsId, groupId, file) {
  const data = new FormData();
  data.append("file", file);
  data.append("timezone", browserTimezone());
  return request({
    url: `/api/workspaces/${wsId}/plannings/import-preview`,
    method: "POST",
    params: { group_id: groupId },
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}
