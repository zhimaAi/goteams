import { request } from "./request";

export function getAutoAssignment(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/auto-assignment`,
    method: "GET",
  });
}
export function saveAutoAssignment(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/auto-assignment`,
    method: "PUT",
    data,
  });
}
