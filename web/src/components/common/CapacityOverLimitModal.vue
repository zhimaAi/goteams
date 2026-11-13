<template>
  <a-modal
    v-model:open="visible"
    :width="560"
    :footer="null"
    centered
    :closable="true"
    :mask-closable="false"
    :keyboard="true"
    :z-index="4000"
    class="capacity-over-limit-modal"
  >
    <template #title>
      <span class="modal-title">
        <ExclamationCircleFilled class="warning-icon" />
        {{ $t("capacity_over_limit_title") }}
      </span>
    </template>

    <div class="capacity-modal-body">
      <p class="capacity-desc">
        {{ isFirstTime ? $t("capacity_over_limit_first") : $t("capacity_over_limit_second") }}
      </p>
      <p class="capacity-contact">{{ $t("plan_upgradeRenewalDesc") }}</p>
      <div class="qr-code-wrap">
        <img
          :src="upgradeRenewalImg"
          :alt="$t('capacity_over_limit_title')"
          class="qr-code"
        />
      </div>
      <a-button
        v-if="canExtend"
        class="extend-btn"
        :loading="extending"
        @click="handleExtend"
      >
        {{ $t("capacity_extend_btn") }}
      </a-button>
    </div>
  </a-modal>
</template>

<script setup>
import {computed, ref, watch} from 'vue'
import {useI18n} from 'vue-i18n'
import {message} from 'ant-design-vue'
import {ExclamationCircleFilled} from '@ant-design/icons-vue'
import {useRoute} from 'vue-router'
import upgradeRenewalImg from '@/image/plan/upgrade-renewal.png'
import {useAuthStore} from '@/stores/auth'
import {extendStorageDeadline} from '@/api/plan'
import {registerCapacityOverLimitModal} from '@/utils/capacityOverLimitModal'

const {t} = useI18n()
const authStore = useAuthStore()
const route = useRoute()

const visible = ref(false)
const extending = ref(false)

/** First-time over limit: system admin may extend once. */
const isFirstTime = computed(() => authStore.enterprisePlan?.capacity_status === 1)

/** Extend button is only shown to the super admin on the first over limit. */
const canExtend = computed(() => isFirstTime.value && authStore.isSuperAdmin())

async function handleExtend() {
  extending.value = true
  try {
    await extendStorageDeadline()
    message.success(t('capacity_extend_success'))
    await authStore.fetchEnterprisePlanInfo()
  } catch {
    // error handled by interceptor
  } finally {
    extending.value = false
  }
}

// Once the plan status returns to normal (e.g. extension granted), close the modal automatically.
watch(
  () => authStore.enterprisePlan?.capacity_status,
  (status) => {
    if (status !== 1 && status !== 2) visible.value = false
  },
)

const AUTH_PATHS = ['/login', '/register']
const pendingOpen = ref(false)

registerCapacityOverLimitModal(() => {
  // 登录/注册流程会在跳转进系统前先触发一次检测，此时路由仍在登录页；
  // 若直接弹窗会盖在登录页上，先挂起，等路由切进系统后由下方 watch 补弹。
  if (AUTH_PATHS.includes(route.path)) {
    pendingOpen.value = true
    return
  }
  visible.value = true
})

// 补弹：登录/注册跳转期间 router.currentRoute 仍是旧路由，必须在导航完成后判断。
watch(
  () => route.path,
  () => {
    if (pendingOpen.value && !AUTH_PATHS.includes(route.path)) {
      pendingOpen.value = false
      visible.value = true
    }
  },
)
</script>

<style scoped lang="less">
.capacity-over-limit-modal {
  :deep(.ant-modal-content) {
    border-radius: 12px;
    padding: 20px 24px 24px;
  }

  :deep(.ant-modal-header) {
    margin-bottom: 16px;
  }

  :deep(.ant-modal-title) {
    font-size: 16px;
    font-weight: 600;
    color: #0f172a;
  }

  :deep(.ant-modal-body) {
    padding: 0;
  }
}

.modal-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.warning-icon {
  color: #f59e0b;
  font-size: 18px;
}

.capacity-modal-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.capacity-desc {
  margin: 0;
  font-size: 14px;
  line-height: 1.8;
  color: #334155;
  text-align: center;
}

.capacity-contact {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #94a3b8;
  text-align: center;
}

.qr-code-wrap {
  display: flex;
  justify-content: center;
}

.qr-code {
  width: 200px;
  height: 200px;
  object-fit: contain;
  border-radius: 8px;
  display: block;
}

.extend-btn {
  align-self: flex-end;
  margin-top: 8px;
  height: 36px;
  padding: 0 20px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 14px;
  color: #fff;
  border: none;
  background: linear-gradient(135deg, #2563eb, #1d4ed8);
  box-shadow: rgba(37, 99, 235, 0.35) 0 4px 14px;

  &:hover,
  &:focus {
    color: #fff;
    border: none;
    background: linear-gradient(135deg, #2563eb, #1d4ed8);
  }
}
</style>
