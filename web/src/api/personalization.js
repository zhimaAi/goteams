import { request } from "./request";

export function getPersonalization() {
  return request({ url: "/api/enterprise/personalization", method: "GET" });
}

export function savePersonalization(data) {
  return request({
    url: "/api/enterprise/personalization",
    method: "PUT",
    data,
  });
}

export function uploadPersonalizationLogo(file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: "/api/enterprise/personalization/logo",
    method: "POST",
    headers: { "Content-Type": "multipart/form-data" },
    data,
  });
}

export function getPublicBranding() {
  return request({ url: "/api/site/branding", method: "GET", silent: true });
}

export function getTenantBranding() {
  return request({ url: "/api/user/branding", method: "GET", silent: true });
}
