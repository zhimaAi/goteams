import { request } from "@/api/request";
export function getWorkspaceNavigationSettings(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/navigation-settings`,
    method: "GET",
  });
}
export function saveMyWorkspaceNavigationSettings(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/navigation-settings/me`,
    method: "PUT",
    data,
  });
}
export function saveWorkspaceNavigationSettings(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/navigation-settings`,
    method: "PUT",
    data,
  });
}
