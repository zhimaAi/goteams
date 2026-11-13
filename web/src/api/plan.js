import {request} from './request'

// 获取当前租户套餐信息
export function getEnterprisePlanInfo() {
  return request({
    url: '/api/enterprise/plan/info',
    method: 'GET'
  })
}

// 容量超限延期3天（仅系统管理员，每企业一次）
export function extendStorageDeadline() {
  return request({
    url: '/api/enterprise/plan/storage-extend',
    method: 'PUT'
  })
}
