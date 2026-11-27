import { request } from "./request";

export function listIssueUrgeRecords(wsId, requirementId, params, signal) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${requirementId}/urge-records`,
    params,
    signal,
    silent: true,
  });
}
