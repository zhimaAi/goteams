import { request } from "./request";

// ======================== Workspace Dashboard API ========================

export function listDashboards(wsId, options = {}) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards`,
    method: "GET",
    ...options,
  });
}

export function createDashboard(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards`,
    method: "POST",
    data,
  });
}

export function getDashboard(wsId, dashboardId) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}`,
    method: "GET",
  });
}

export function updateDashboard(wsId, dashboardId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}`,
    method: "PUT",
    data,
  });
}

export function deleteDashboard(wsId, dashboardId, data = {}) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}`,
    method: "DELETE",
    data,
  });
}

export function updateDashboardOrder(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/order`,
    method: "PUT",
    data,
  });
}

export function getDashboardPermission(wsId, dashboardId) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/permission`,
    method: "GET",
  });
}

export function updateDashboardPermission(wsId, dashboardId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/permission`,
    method: "PUT",
    data,
  });
}

export function saveDashboardAs(wsId, dashboardId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/save-as`,
    method: "POST",
    data,
  });
}

export function updateDashboardLayout(wsId, dashboardId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/layout`,
    method: "PUT",
    data,
  });
}

export function copyDashboardCard(wsId, dashboardId, cardId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/cards/${cardId}/copy`,
    method: "POST",
    data,
  });
}

export function queryDashboardMetrics(wsId, dashboardId, data, options = {}) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/metrics/query`,
    method: "POST",
    data,
    ...options,
  });
}

export function getDashboardAnnouncement(wsId, dashboardId, options = {}) {
  return request({
    url: `/api/workspaces/${wsId}/dashboard-announcement`,
    method: "GET",
    params: { dashboard_id: dashboardId },
    ...options,
  });
}

export function getDashboardUserState(wsId, dashboardId, options = {}) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/user-state`,
    method: "GET",
    ...options,
  });
}

export function updateDashboardUserState(wsId, dashboardId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/user-state`,
    method: "PUT",
    data,
  });
}

function uploadDashboardAttachment(wsId, dashboardId, kind, file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/uploads/${kind}`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}

export function uploadDashboardImage(wsId, dashboardId, file) {
  return uploadDashboardAttachment(wsId, dashboardId, "image", file);
}

export function uploadDashboardFile(wsId, dashboardId, file) {
  return uploadDashboardAttachment(wsId, dashboardId, "file", file);
}

export function uploadDashboardVideo(wsId, dashboardId, file) {
  return uploadDashboardAttachment(wsId, dashboardId, "video", file);
}

export function queryDashboardCharts(wsId, dashboardId, data, options = {}) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/charts/query`,
    method: "POST",
    data,
    ...options,
  });
}
export function copyDashboardCards(wsId, dashboardId, cardId, data) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/cards/${cardId}/copy-many`,
    method: "POST",
    data,
  });
}

export function getDashboardSLAOptions(wsId, dashboardId) {
  return request({
    url: `/api/workspaces/${wsId}/dashboards/${dashboardId}/sla-options`,
    method: "GET",
  });
}
