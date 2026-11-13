import { request } from "./request";

const base = (wsId) => `/api/workspaces/${wsId}/sla`;

export const getSLASettings = (wsId) =>
  request({ url: `${base(wsId)}/settings` });
export const saveSLASettings = (wsId, data) =>
  request({ url: `${base(wsId)}/settings`, method: "PUT", data });
export const getSLAMetadata = (wsId) =>
  request({ url: `${base(wsId)}/metadata` });
export const listSLAStages = (wsId, params) =>
  request({ url: `${base(wsId)}/stages`, params });
export const listSLAPolicies = (wsId, params) =>
  request({ url: `${base(wsId)}/policies`, params });
export const saveSLAStage = (wsId, data) =>
  request({
    url: `${base(wsId)}/stages${data.id ? `/${data.id}` : ""}`,
    method: data.id ? "PUT" : "POST",
    data,
  });
export const saveSLAPolicy = (wsId, data) =>
  request({
    url: `${base(wsId)}/policies${data.id ? `/${data.id}` : ""}`,
    method: data.id ? "PUT" : "POST",
    data,
  });
export const saveSLAPolicyOrder = (wsId, data) =>
  request({ url: `${base(wsId)}/policy-order`, method: "PUT", data });
export const archiveSLADefinition = (wsId, type, item) =>
  request({
    url: `${base(wsId)}/${type}/${item.id}`,
    method: "DELETE",
    data: { revision: item.revision },
  });
export const getSLASummaries = (wsId, ids) =>
  request({
    url: `${base(wsId)}/work-items`,
    params: { ids: ids.join(",") },
    silent: true,
  });
export const getRequirementSLA = (wsId, id, params) =>
  request({ url: `/api/workspaces/${wsId}/requirements/${id}/sla`, params });
