// 容量超限弹窗全局触发器。
// CapacityOverLimitModal 组件在挂载时通过 registerCapacityOverLimitModal 注册打开回调，
// authStore 检测到企业容量超限（capacity_status 为 1 或 2）时调用 openCapacityOverLimitModal 触发弹窗。

let openCallback = null

export function registerCapacityOverLimitModal(callback) {
  openCallback = callback
}

export function openCapacityOverLimitModal() {
  openCallback?.()
}
