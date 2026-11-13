import { request } from "./request";

const prefix = (wsId) => `/api/workspaces/${wsId}/automation`;

export function listAutomationRules(wsId, params) {
  return request({ url: `${prefix(wsId)}/rules`, method: "GET", params });
}

export function getAutomationRule(wsId, ruleId) {
  return request({ url: `${prefix(wsId)}/rules/${ruleId}`, method: "GET" });
}

export function createAutomationRule(wsId, data) {
  return request({ url: `${prefix(wsId)}/rules`, method: "POST", data });
}

export function updateAutomationRule(wsId, ruleId, data) {
  return request({
    url: `${prefix(wsId)}/rules/${ruleId}`,
    method: "PUT",
    data,
  });
}

export function listAutomationCopyTargets(wsId) {
  return request({
    url: `${prefix(wsId)}/copy-targets`,
    method: "GET",
  });
}

export function copyAutomationRule(wsId, ruleId, data) {
  return request({
    url: `${prefix(wsId)}/rules/${ruleId}/copy`,
    method: "POST",
    data,
  });
}

export function setAutomationRuleEnabled(wsId, ruleId, enabled) {
  return request({
    url: `${prefix(wsId)}/rules/${ruleId}/enabled`,
    method: "PATCH",
    data: { enabled },
  });
}

export function deleteAutomationRule(wsId, ruleId) {
  return request({
    url: `${prefix(wsId)}/rules/${ruleId}`,
    method: "DELETE",
  });
}

export function listAutomationExecutions(wsId, params) {
  return request({
    url: `${prefix(wsId)}/executions`,
    method: "GET",
    params,
  });
}

export function getAutomationExecution(wsId, executionId) {
  return request({
    url: `${prefix(wsId)}/executions/${executionId}`,
    method: "GET",
  });
}

export function getAutomationMetadata(wsId, type, ruleId) {
  return request({
    url: `${prefix(wsId)}/metadata`,
    method: "GET",
    params: { type, rule_id: ruleId },
  });
}

export function listAutomationUserMappings(wsId) {
  return request({ url: `${prefix(wsId)}/user-mappings`, method: "GET" });
}

// 参数映射为企业共用，只提交实际修改的成员字段。
export function saveAutomationUserMappings(wsId, items) {
  return request({
    url: `${prefix(wsId)}/user-mappings`,
    method: "PUT",
    data: { items },
  });
}
