import { request } from "./request";

// 读取企业 SSO 配置（登录配置 + 同步配置 + 平台元数据 + 入口/回调地址）
export function getSsoConfig(providerPreview) {
  return request({
    url: "/api/enterprise/sso",
    method: "GET",
    params: providerPreview ? { provider_preview: providerPreview } : {},
  });
}

// 保存企业 SSO 登录配置。密钥未编辑时必须省略字段，不能用空串表达「没改」
export function saveSsoConfig(data) {
  return request({
    url: "/api/enterprise/sso",
    method: "PUT",
    data,
  });
}

// 保存成员同步配置
export function saveSsoSyncConfig(data) {
  return request({
    url: "/api/enterprise/sso/sync",
    method: "PUT",
    data,
  });
}

// 手动同步：后端在请求内跑完整个通讯录拉取与逐成员写入再返回 {done}。
// done=false 表示已有同步在跑（领不到租约）。真实企业规模下耗时可达数分钟，
// 所以这里单独放宽超时——全局 15s 只适用于普通接口。
export function runSsoSync() {
  return request({
    url: "/api/enterprise/sso/sync/run",
    method: "POST",
    timeout: 600000,
  });
}

// 连接预检：用表单当前填写、尚未保存的配置探一次端点/凭证。
// 密钥未编辑时省略 app_secret，预检走已存的那把；成功回 {ok, level}，失败回 {ok:false, reason}。
export function testSsoConnection(data) {
  return request({
    url: "/api/enterprise/sso/test",
    method: "POST",
    data,
  });
}

// 公开的企业登录入口信息（匿名可读）。不满足条件时只回 enabled=false，不带原因。
export function getSsoEntry(scopeId) {
  return request({
    url: "/api/sso/entry",
    method: "GET",
    params: { scope_id: scopeId },
    silent: true,
  });
}
