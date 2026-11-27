import { request } from "./request";

export function getSatisfactionConfig(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/issue-satisfaction/config`,
    method: "get",
  });
}
export function saveSatisfactionConfig(wsId, config) {
  return request({
    url: `/api/workspaces/${wsId}/issue-satisfaction/config`,
    method: "put",
    data: { ...config, expected_version: config.version },
  });
}
export function getSatisfaction(wsId, requirementId, inviteKey, openid, lang) {
  return request({
    url: inviteKey
      ? `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}/satisfaction`
      : `/api/workspaces/${wsId}/requirements/${requirementId}/satisfaction`,
    method: "get",
    headers: lang ? { Lang: lang } : {},
    params: inviteKey ? { openid } : undefined,
  });
}
export function submitSatisfaction(
  inviteKey,
  requirementId,
  openid,
  lang,
  data,
) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/requirements/${requirementId}/satisfaction`,
    method: "post",
    headers: lang ? { Lang: lang } : {},
    params: { openid },
    data,
  });
}
export function listSatisfactionStates(inviteKey, openid, lang, ids) {
  return request({
    url: `/api/public/issue-invites/${encodeURIComponent(inviteKey)}/satisfactions`,
    method: "get",
    headers: lang ? { Lang: lang } : {},
    params: { openid, requirement_ids: ids.join(",") },
  });
}
export function querySatisfactionStatistics(wsId, data, signal) {
  return request({
    url: `/api/workspaces/${wsId}/issue-statistics/satisfaction`,
    method: "post",
    data,
    signal,
  });
}
