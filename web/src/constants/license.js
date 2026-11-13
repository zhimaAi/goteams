// 开源版授权固定枚举：与后端 define/license.go 保持一致，
// 供 request 拦截器、/license 页面、侧栏提醒等模块共用。

// 固定开源版标识（/api/system/license/status 返回的 edition）。
export const LICENSE_EDITION_COMMERCIAL = "commercial";

// 授权状态（镜像后端 LicenseState* 常量）。
export const LICENSE_STATE = {
  NOT_ACTIVATED: "not_activated",
  VALID: "valid",
  EXPIRING: "expiring",
  EXPIRED: "expired",
  INVALID: "invalid_signature",
  DEVICE_MISMATCH: "device_mismatch",
  NOT_YET_VALID: "not_yet_valid",
  CLOCK_INVALID: "clock_invalid",
  CONFIG_ERROR: "config_error",
};

// 门禁 403 响应与状态接口携带的固定 reason（镜像后端 LicenseReason* 常量）。
export const LICENSE_REASON = {
  NOT_ACTIVATED: "license_not_activated",
  EXPIRED: "license_expired",
  INVALID: "license_invalid",
  DEVICE_MISMATCH: "license_device_mismatch",
  CLOCK_INVALID: "license_clock_invalid",
  CONFIG_ERROR: "license_config_error",
};

// 允许登录与使用业务接口的状态。
export const LICENSE_CAN_LOGIN_STATES = new Set([
  LICENSE_STATE.VALID,
  LICENSE_STATE.EXPIRING,
]);

// 授权在线/离线模式。
export const LICENSE_ACTIVATION_MODE = {
  ONLINE: "online",
  OFFLINE: "offline",
};

// 试用授权类型（试用为强制在线模式）。
export const LICENSE_KIND_TRIAL = "trial";

// 侧栏/版本卡剩余天数提醒阈值。
export const LICENSE_REMINDER_DAYS = 15;

// 授权状态轮询间隔（毫秒）。
export const LICENSE_POLL_INTERVAL_MS = 60000;

// 授权页固定路由。
export const LICENSE_ROUTE_PATH = "/license";
