<script setup>
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {WarningFilled} from '@ant-design/icons-vue'
import {useAuthStore} from '@/stores/auth'
import upgradeRenewalImg from '@/image/plan/upgrade-renewal.png'

const {t} = useI18n()
const authStore = useAuthStore()

// 首次超限（可延期）与延期过期（永久禁止）展示不同文案
const isFirstTime = computed(() => authStore.enterprisePlan?.capacity_status === 1)
</script>

<template>
  <div class="capacity-over-limit-page">
    <div class="blocked-card">
      <WarningFilled class="blocked-icon" />
      <h1 class="title">{{ t('capacity_over_limit_title') }}</h1>
      <p class="desc">
        {{ isFirstTime ? t('capacity_over_limit_first') : t('capacity_over_limit_second') }}
      </p>
      <p class="desc">{{ t('plan_upgradeRenewalDesc') }}</p>
      <div class="qr-code-wrap">
        <img :src="upgradeRenewalImg" :alt="t('capacity_over_limit_title')" class="qr-code" />
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.capacity-over-limit-page {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 100%;
  background: #fff;
}

.blocked-card {
  max-width: 520px;
  padding: 48px 24px;
  text-align: center;
}

.blocked-icon {
  margin-bottom: 12px;
  color: #f59e0b;
  font-size: 40px;
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

.qr-code-wrap {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}

.qr-code {
  width: 180px;
  height: 180px;
  object-fit: contain;
  border-radius: 8px;
  display: block;
}
</style>
