<script setup>
import {computed, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import dayjs from 'dayjs'
import {useAuthStore} from '@/stores/auth'
import UpgradeRenewalModal from '@/components/common/UpgradeRenewalModal.vue'

const {t} = useI18n()
const authStore = useAuthStore()

const expireDate = computed(() => {
  const ts = authStore.enterprisePlan?.expire_at
  if (!ts) return '-'
  return dayjs.unix(ts).format('YYYY.MM.DD')
})

// 续费升级二维码弹窗
const upgradeOpen = ref(false)
</script>

<template>
  <div class="service-expired-page">
    <div class="expired-card">
      <h1 class="title">{{ t('serviceExpired_title') }}</h1>
      <p class="desc">
        {{ t('serviceExpired_enterpriseExpiredAt', {date: expireDate}) }}
      </p>
      <p class="desc">
        {{ t('serviceExpired_restricted') }}
        <a class="link" href="mailto:support@goteams.local">{{ t('serviceExpired_contactAdmin') }}</a>
        <a class="link" href="#" @click.prevent="upgradeOpen = true">{{ t('serviceExpired_renewSystem') }}</a>
      </p>
      <p class="warning">{{ t('serviceExpired_clearDataWarning') }}</p>
    </div>

    <!-- 续费升级二维码弹窗 -->
    <UpgradeRenewalModal v-model:open="upgradeOpen" />
  </div>
</template>

<style scoped lang="less">
.service-expired-page {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 100%;
  background: #fff;
}

.expired-card {
  max-width: 560px;
  padding: 48px 24px;
  text-align: left;
}

.title {
  margin: 0 0 16px;
  font-size: 22px;
  font-weight: 600;
  color: #1f2937;
}

.desc {
  margin: 0 0 8px;
  font-size: 14px;
  line-height: 1.8;
  color: #4b5563;
}

.warning {
  margin: 12px 0 0;
  font-size: 14px;
  line-height: 1.8;
  color: #ef4444;
}

.link {
  margin: 0 4px;
  color: #2563eb;
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
}
</style>
