import { axios, request } from "./request";

const reportUrl = (wsId, type) => `/api/workspaces/${wsId}/reports/${type}`;
const reportResultUrl = (wsId, type, id) =>
  `${reportUrl(wsId, type)}/${id ? `runs/${id}` : "preview"}`;

export function getReportPreview(wsId, type, timezone, signal) {
  return request({
    url: reportResultUrl(wsId, type),
    params: { timezone },
    signal,
    silent: true,
    timeout: 30000,
  });
}

export function getReportOptions(wsId, type, params) {
  return request({
    url: `${reportUrl(wsId, type)}/options`,
    params,
    silent: true,
  });
}

export function generateReport(wsId, type, data) {
  return request({
    url: `${reportUrl(wsId, type)}/runs`,
    method: "post",
    data,
    timeout: 30000,
  });
}

export function listReportRuns(wsId, type, params) {
  return request({
    url: `${reportUrl(wsId, type)}/runs`,
    params,
    silent: true,
  });
}

export function getReportRun(wsId, type, id) {
  return request({ url: `${reportUrl(wsId, type)}/runs/${id}`, silent: true });
}

export function listReportItems(wsId, type, id, params) {
  return request({
    url: `${reportResultUrl(wsId, type, id)}/items`,
    params,
    timeout: id ? 15000 : 30000,
    silent: true,
  });
}

export function exportReport(wsId, type, id, timezone) {
  return axios.get(`${reportResultUrl(wsId, type, id)}/export`, {
    params: id ? undefined : { timezone },
    timeout: id ? 30000 : 45000,
    responseType: "blob",
  });
}
