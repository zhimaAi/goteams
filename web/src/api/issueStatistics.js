import { request } from "./request";

export function getIssueStatisticsMetadata(wsId, signal) {
  return request({
    url: `/api/workspaces/${wsId}/issue-statistics/metadata`,
    method: "GET",
    signal,
  });
}

export function queryIssueStatistics(wsId, view, data, signal) {
  return request({
    url: `/api/workspaces/${wsId}/issue-statistics/sla/${view}`,
    method: "POST",
    data,
    signal,
  });
}

export function getIssueOverview(wsId, params, signal) {
  return request({
    url: `/api/workspaces/${wsId}/issue-statistics/overview`,
    params,
    signal,
    timeout: 35000,
  });
}

export function listIssueOverviewItems(wsId, params, signal) {
  return request({
    url: `/api/workspaces/${wsId}/issue-statistics/overview/items`,
    params,
    signal,
    timeout: 35000,
  });
}
