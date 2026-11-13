import { request } from "./request";

// 开源版授权接口：状态、试用验证码、在线/离线激活、登录后安装。

// 获取授权状态（公开接口，含设备码与已验签配额）。
export function getLicenseStatus() {
  return request({
    url: "/api/system/license/status",
    method: "GET",
  });
}

// 发送在线试用邮箱验证码（公开接口）。
export function sendLicenseVerifyCode(email) {
  return request({
    url: "/api/system/license/online-verification-code",
    method: "POST",
    data: { email },
  });
}

// 在线体验激活：校验验证码并签发默认有效至2099-01-01的授权（request_id 幂等）。
export function activateLicenseOnline(params) {
  return request({
    url: "/api/system/license/activate-online",
    method: "POST",
    data: params,
  });
}

// 离线激活：粘贴运营后台签发的完整 Key（公开接口，仅未激活时可用）。
export function activateLicenseOffline(licenseKey) {
  return request({
    url: "/api/system/license/activate-offline",
    method: "POST",
    data: { license_key: licenseKey },
  });
}

// 登录后由企业管理员安装新 Key（授权过期也可用）。
export function installLicenseKey(licenseKey) {
  return request({
    url: "/api/enterprise/license/install",
    method: "POST",
    data: { license_key: licenseKey },
  });
}
