import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { getLicenseStatus } from "@/api/license";
import {
  LICENSE_CAN_LOGIN_STATES,
  LICENSE_EDITION_COMMERCIAL,
  LICENSE_KIND_TRIAL,
  LICENSE_POLL_INTERVAL_MS,
  LICENSE_REMINDER_DAYS,
  LICENSE_STATE,
} from "@/constants/license";

// 开源版授权状态仓库：单飞请求 + 页面可见时的 60 秒轮询。
export const useLicenseStore = defineStore("license", () => {
  // ---- State ----
  const status = ref(null); // /api/system/license/status 原始返回。
  const fetchFailed = ref(false); // 最近一次状态获取是否失败。
  let statusPromise = null; // single-flight：并发调用复用同一请求。

  // ---- Getters ----
  const edition = computed(() => status.value?.edition || "");
  // 固定开源版标识；社区版（open）没有该接口时视为非开源版。
  const isCommercial = computed(
    () => edition.value === LICENSE_EDITION_COMMERCIAL,
  );
  const state = computed(() => status.value?.status || "");
  const reason = computed(() => status.value?.reason || "");
  const canLogin = computed(
    () =>
      !!status.value?.can_login || LICENSE_CAN_LOGIN_STATES.has(state.value),
  );
  const businessEnabled = computed(
    () =>
      !!status.value?.business_enabled ||
      LICENSE_CAN_LOGIN_STATES.has(state.value),
  );
  const managementOnly = computed(
    () => isCommercial.value && canLogin.value && !businessEnabled.value,
  );
  const licenseRequired = computed(() => !!status.value?.license_required);
  const deviceCode = computed(() => status.value?.device_code || "");
  const corpName = computed(() => status.value?.corp_name || "");
  const licenseKind = computed(() => status.value?.license_kind || "");
  const activationMode = computed(() => status.value?.activation_mode || "");
  const licenseNo = computed(() => status.value?.license_no || "");
  const expireAt = computed(() => status.value?.expire_at || 0);
  const remainingDays = computed(() => status.value?.remaining_days || 0);
  const revision = computed(() => status.value?.revision || 0);
  const quotas = computed(() => status.value?.quotas || null);
  // 试用授权运行在强制在线模式，断网超宽限期会进入 expired。
  const isTrial = computed(() => licenseKind.value === LICENSE_KIND_TRIAL);
  // 授权已安装但剩余天数进入提醒阈值（侧栏/版本卡提醒）。
  const needsReminder = computed(() => {
    if (!canLogin.value || state.value === LICENSE_STATE.NOT_ACTIVATED)
      return false;
    return remainingDays.value <= LICENSE_REMINDER_DAYS;
  });

  // ---- Actions ----

  /** 拉取授权状态；并发调用共享同一请求（路由守卫、焦点事件、轮询同时触发时只打一次）。 */
  function fetchStatus() {
    if (statusPromise) return statusPromise;
    statusPromise = getLicenseStatus()
      .then((data) => {
        status.value = data || null;
        fetchFailed.value = false;
        return status.value;
      })
      .catch(() => {
        fetchFailed.value = true;
        return null;
      })
      .finally(() => {
        statusPromise = null;
      });
    return statusPromise;
  }

  /** 激活/安装 Key 成功后立即强制刷新（先刷新状态，再决定跳转）。 */
  async function refreshAfterInstall() {
    await fetchStatus();
    return status.value;
  }

  // ---- 60 秒轮询生命周期 ----
  let pollTimer = null;

  function onVisibilityChange() {
    // 页面重新可见时立即检查一次，不等下一个周期。
    if (document.visibilityState === "visible") fetchStatus();
  }

  /** 启动授权轮询（幂等）；仅页面可见时真正发请求。 */
  function startPolling() {
    if (pollTimer) return;
    document.addEventListener("visibilitychange", onVisibilityChange);
    pollTimer = setInterval(() => {
      if (document.visibilityState === "visible") fetchStatus();
    }, LICENSE_POLL_INTERVAL_MS);
    fetchStatus();
  }

  /** 停止轮询并清理事件监听（布局卸载/应用退出时必须调用）。 */
  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    document.removeEventListener("visibilitychange", onVisibilityChange);
  }

  /** 退出登录等场景清理状态缓存。 */
  function clear() {
    status.value = null;
    fetchFailed.value = false;
  }

  return {
    status,
    fetchFailed,
    edition,
    isCommercial,
    state,
    reason,
    canLogin,
    businessEnabled,
    managementOnly,
    licenseRequired,
    deviceCode,
    corpName,
    licenseKind,
    activationMode,
    licenseNo,
    expireAt,
    remainingDays,
    revision,
    quotas,
    isTrial,
    needsReminder,
    fetchStatus,
    refreshAfterInstall,
    startPolling,
    stopPolling,
    clear,
  };
});
