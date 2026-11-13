// 配额超限弹窗全局触发器。
// QuotaExceededModal 组件在挂载时通过 registerQuotaModal 注册打开回调，
// 业务代码或请求拦截器通过 openQuotaModal 触发弹窗。

let openCallback = null;

export function registerQuotaModal(callback) {
  openCallback = callback;
}

export function openQuotaModal() {
  openCallback?.();
}
