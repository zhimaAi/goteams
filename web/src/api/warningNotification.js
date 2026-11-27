import { request } from "./request";
const base = (wsId) => `/api/workspaces/${wsId}/warning-notifications`;
export function listWarningRules(wsId, params) {
  return request({ url: base(wsId), method: "GET", params });
}
export function getWarningMetadata(wsId, params) {
  return request({ url: `${base(wsId)}/metadata`, method: "GET", params });
}
export function saveWarningRule(wsId, rule) {
  return request({
    url: base(wsId) + (rule.id ? `/${rule.id}` : ""),
    method: rule.id ? "PUT" : "POST",
    data: rule,
  });
}
export function deleteWarningRule(wsId, id) {
  return request({ url: `${base(wsId)}/${id}`, method: "DELETE" });
}

export function setWarningRuleEnabled(wsId, id, enabled) {
  return request({
    url: `${base(wsId)}/${id}/enabled`,
    method: "PATCH",
    data: { enabled },
  });
}
