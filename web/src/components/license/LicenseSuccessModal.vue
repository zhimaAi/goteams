<template>
  <a-modal
    v-model:open="innerOpen"
    :title="$t('license_success_title')"
    :footer="null"
    :width="480"
    :maskClosable="false"
    :keyboard="false"
    centered
  >
    <div class="license-success-body">
      <CheckCircleFilled class="success-icon" />
      <div class="success-rows">
        <div v-if="result?.corp_name" class="success-row">
          <span class="row-label">{{ $t('license_success_corp') }}</span>
          <span class="row-value">{{ result.corp_name }}</span>
        </div>
        <div v-if="result?.expire_at" class="success-row">
          <span class="row-label">{{ $t('license_success_expire') }}</span>
          <span class="row-value">{{ formatLocalDate(result.expire_at) }}</span>
        </div>
      </div>

      <template v-if="hasBootstrap">
        <div class="bootstrap-block">
          <div class="bootstrap-row">
            <span class="row-label">{{ $t('license_success_account') }}</span>
            <span class="row-value">{{ result.bootstrap_username }}</span>
            <a-button size="small" type="text" @click="copyValue(result.bootstrap_username)">
              {{ $t('license_copy') }}
            </a-button>
          </div>
          <div class="bootstrap-row">
            <span class="row-label">{{ $t('license_success_password') }}</span>
            <span class="row-value password">{{ result.bootstrap_password }}</span>
            <a-button size="small" type="text" @click="copyValue(result.bootstrap_password)">
              {{ $t('license_copy') }}
            </a-button>
          </div>
          <p class="bootstrap-tip">{{ $t('license_success_password_tip') }}</p>
        </div>
      </template>

      <a-button type="primary" class="login-button" block @click="emit('confirm')">
        {{ $t('license_go_login') }}
      </a-button>
    </div>
  </a-modal>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { CheckCircleFilled } from '@ant-design/icons-vue'
import { formatLocalDate } from '@/utils/date'
import { writeClipboardText } from '@/utils/workItemClipboard'

const { t } = useI18n()

const props = defineProps({
  open: { type: Boolean, default: false },
  result: { type: Object, default: null },
})
const emit = defineEmits(['update:open', 'confirm'])

const innerOpen = computed({
  get: () => props.open,
  set: (val) => emit('update:open', val),
})

// bootstrap_username/bootstrap_password only appear on first activation.
const hasBootstrap = computed(
  () => !!props.result?.bootstrap_username && !!props.result?.bootstrap_password,
)

async function copyValue(value) {
  try {
    await writeClipboardText(value)
    message.success(t('license_copied'))
  } catch {
    message.error(t('license_copy_failed'))
  }
}
</script>

<style scoped lang="less">
.license-success-body {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.success-icon {
  margin: 8px 0 20px;
  font-size: 56px;
  color: #52c41a;
}

.success-rows {
  width: 100%;
}

.success-row {
  display: flex;
  align-items: baseline;
  padding: 4px 0;
  font-size: 14px;
  line-height: 22px;
}

.row-label {
  flex-shrink: 0;
  width: 96px;
  color: #667085;
}

.row-value {
  color: #1f2937;
  word-break: break-all;

  &.password {
    font-family: monospace;
    font-weight: 600;
  }
}

.bootstrap-block {
  width: 100%;
  margin-top: 12px;
  padding: 12px 16px;
  background: #f7f8fb;
  border-radius: 8px;
}

.bootstrap-row {
  display: flex;
  align-items: center;
  padding: 3px 0;
  font-size: 14px;
  line-height: 22px;

  .row-label {
    width: 84px;
  }

  .row-value {
    flex: 1;
  }
}

.bootstrap-tip {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 20px;
  color: #d46b08;
}

.login-button {
  height: 44px;
  margin-top: 24px;
  border-radius: 6px;
  font-size: 16px;
  background: #3157e2;
  border-color: #3157e2;

  &:hover,
  &:focus {
    background: #2548c4;
    border-color: #2548c4;
  }
}
</style>
