import { axios, request } from "./request";

// ======================== Auth API ========================

/** Log in. */
export function login(data) {
  return request({
    url: "/api/auth/login",
    method: "POST",
    data,
  });
}

/** Authorize the TeamsBoard desktop client after the user confirms in the browser. */
export function confirmClientLoginSession(state) {
  return request({
    url: "/api/client/login-sessions/confirm",
    method: "POST",
    data: { state },
  });
}

/** Poll the handshake phase: pending / confirmed / exchanged / expired. */
export function getClientLoginSessionStatus(state) {
  return request({
    url: `/api/client/login-sessions/${encodeURIComponent(state)}/status`,
    method: "GET",
  });
}

/** Register. */
export function register(data) {
  return request({
    url: "/api/auth/register",
    method: "POST",
    data,
  });
}

/** Send the verification code. */
export function sendVerifyCode(username) {
  return request({
    url: "/api/auth/verifycode",
    method: "POST",
    data: { username },
  });
}

/** Forgot password. */
export function forgotPassword(username) {
  return request({
    url: "/api/auth/forgot-password",
    method: "POST",
    data: { username },
  });
}

/** Send registration email verification code. */
export function sendRegisterCode(email) {
  return request({
    url: "/api/auth/send-register-code",
    method: "POST",
    data: { email },
  });
}

/** Reset password. */
export function resetPassword(data) {
  return request({
    url: "/api/auth/reset-password",
    method: "POST",
    data,
  });
}

/** Fetch the slider captcha. */
export function getCaptcha() {
  return request({
    url: "/api/auth/captcha",
    method: "GET",
  });
}

/** Log out (delete the server-side session). */
export function logout(token) {
  return request({
    url: "/api/user/logout",
    method: "POST",
    // The store clears its token before Axios runs the request interceptor.
    headers: { Authorization: `Bearer ${token}` },
  });
}

/** Fetch the current user's profile. */
export function getProfile() {
  return request({
    url: "/api/user/profile",
    method: "GET",
  });
}

/** Update the current user's profile. */
export function updateProfile(data) {
  return request({
    url: "/api/user/profile",
    method: "PUT",
    data,
  });
}

/** Upload the current user's avatar. */
export function uploadAvatar(file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: "/api/user/avatar",
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

/**
 * Fetch the avatar file (with authentication, returns a blob).
 * When userId is not passed, the current logged-in user is used; when userId is passed, other members in the same tenant are used (backend isolates by admin_id).
 */
export async function getAvatarBlob(userId) {
  const response = await axios({
    url: userId ? `/api/user/avatar?userId=${userId}` : "/api/user/avatar",
    method: "GET",
    responseType: "blob",
  });
  return response.data;
}

/** Change the current user's password. */
export function changePassword(data) {
  return request({
    url: "/api/user/password",
    method: "PUT",
    data,
  });
}

/** Send the verification code used to bind or change the current user's email. */
export function sendBindEmailCode(data) {
  return request({
    url: "/api/user/email/send-code",
    method: "POST",
    data,
  });
}

/** Bind or change the current user's email with the verified code. */
export function bindEmail(data) {
  return request({
    url: "/api/user/email",
    method: "PUT",
    data,
  });
}

/** Fetch the current user's enterprise-wide contact preferences. */
export function getContactPreferences() {
  return request({
    url: "/api/user/contact-preferences",
    method: "GET",
  });
}

/** Add or remove one enterprise-wide favorite contact. */
export function updateFavoriteContact(contactUserId, isFavorite) {
  return request({
    url: `/api/user/contact-preferences/${contactUserId}/favorite`,
    method: "PUT",
    data: { is_favorite: isFavorite },
  });
}

/** Record assignees selected by the current user. */
export function recordRecentContacts(userIds) {
  return request({
    url: "/api/user/contact-preferences/recent",
    method: "PUT",
    data: { user_ids: userIds },
  });
}
