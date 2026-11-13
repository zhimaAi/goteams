import { axios, request } from "./request";

// List API tokens owned by the current user.
export function listApiTokens() {
  return request({
    url: "/api/user/api-tokens",
    method: "GET",
  });
}

// List the permission points a token can be granted, already translated by the backend.
export function listApiTokenPermissions() {
  return request({
    url: "/api/user/api-tokens/permissions",
    method: "GET",
  });
}

// Create a token; the response includes the plaintext so the list can show it immediately.
export function createApiToken(data) {
  return request({
    url: "/api/user/api-tokens",
    method: "POST",
    data,
  });
}

// Update the remark name and permission points; enabled state and stats are untouched.
export function updateApiToken(tokenId, data) {
  return request({
    url: `/api/user/api-tokens/${tokenId}`,
    method: "PUT",
    data,
  });
}

export function setApiTokenEnabled(tokenId, enabled) {
  return request({
    url: `/api/user/api-tokens/${tokenId}/enabled`,
    method: "PUT",
    data: { enabled },
  });
}

export function deleteApiToken(tokenId) {
  return request({
    url: `/api/user/api-tokens/${tokenId}`,
    method: "DELETE",
  });
}

// Download the Agent skill package (zip) for one token; it contains the API key.
export function downloadApiTokenSkill(tokenId) {
  return axios.get(`/api/user/api-tokens/${tokenId}/skill`, {
    responseType: "blob",
  });
}

// Create a short-lived install link for one token's skill package. The link lives in Redis for
// five minutes and can be downloaded only once, so it is safe to hand to an Agent.
export function createApiTokenSkillLink(tokenId) {
  return request({
    url: `/api/user/api-tokens/${tokenId}/skill/link`,
    method: "POST",
  });
}

// Download the open API reference (markdown) covering every endpoint. It contains no key,
// so it is not bound to any token.
export function downloadApiTokenDoc() {
  return axios.get("/api/user/api-tokens/doc", {
    responseType: "blob",
  });
}
