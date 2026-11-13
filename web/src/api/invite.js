import { request } from "./request";

// ======================== Invitation management (enterprise member page) ========================

export function createMemberInvitation(data) {
  return request({
    url: "/api/enterprise/invitation",
    method: "POST",
    data,
  });
}

export function getMemberInvitation() {
  return request({
    url: "/api/enterprise/invitation",
    method: "GET",
  });
}

export function stopMemberInvitation() {
  return request({
    url: "/api/enterprise/invitation",
    method: "DELETE",
  });
}

// ======================== Public invitation accept page (token auth) ========================

export function getInviteInfo(token) {
  return request({
    url: `/api/invite/${token}`,
    method: "GET",
  });
}

export function uploadInviteAvatar(token, file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: `/api/invite/${token}/avatar`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

export function sendInviteEmailCode(token, data) {
  return request({
    url: `/api/invite/${token}/email-code`,
    method: "POST",
    data,
  });
}

export function acceptInvite(token, data) {
  return request({
    url: `/api/invite/${token}/accept`,
    method: "POST",
    data,
  });
}
