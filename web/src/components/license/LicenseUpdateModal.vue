<template>
  <a-modal
    v-model:open="innerOpen"
    :title="$t('license_update_title')"
    :footer="null"
    :width="520"
    :maskClosable="false"
    destroyOnClose
  >
    <div class="license-update-body">
      <div class="current-license">
        <div class="current-row">
          <span class="current-label">{{ $t('license_success_corp') }}</span>
          <span class="current-value">{{ licenseStore.corpName || '-' }}</span>
        </div>
        <div class="current-row">
          <span class="current-label">{{ $t('license_success_expire') }}</span>
          <span class="current-value">{{ expireText }}</span>
        </div>
        <div class="current-row">
          <span class="current-label">{{ $t('license_current_state') }}</span>
          <span class="current-value">{{ stateText }}</span>
        </div>
      </div>

      <a-form ref="formRef" :model="form" :rules="rules" layout="vertical" @finish="handleSubmit">
        <a-form-item name="licenseKey">
          <a-textarea
            v-model:value="form.licenseKey"
            :placeholder="$t('license_key_placeholder')"
            :rows="5"
            :maxlength="4096"
          />
        </a-form-item>

        <div class="action-row">
          <a-button class="action-btn" @click="contactOpen = true">
            {{ $t('license_contact_service') }}
          </a-button>
          <a-button
            type="primary"
            html-type="submit"
            class="action-btn primary"
            :loading="submitting"
          >
            {{ $t('license_install_btn') }}
          </a-button>
        </div>
      </a-form>
    </div>

    <ContactServiceModal v-model:open="contactOpen" />
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { installLicenseKey } from '@/api/license'
import { useLicenseStore } from '@/stores/license'
import { LICENSE_STATE } from '@/constants/license'
import { formatLocalDate } from '@/utils/date'
import ContactServiceModal from './ContactServiceModal.vue'

const { t } = useI18n()
const licenseStore = useLicenseStore()

const emit = defineEmits(['update:open', 'success'])

const props = defineProps({
  open: { type: Boolean, default: false },
})

const innerOpen = computed({
  get: () => props.open,
  set: (val) => emit('update:open', val),
})

const formRef = ref()
const submitting = ref(false)
const contactOpen = ref(false)

const form = reactive({
  licenseKey: '',
})

const rules = {
  licenseKey: [
    { required: true, message: t('license_key_required'), trigger: 'blur' },
  ],
}

const expireText = computed(() =>
  licenseStore.expireAt ? formatLocalDate(licenseStore.expireAt) : '-',
)

const stateText = computed(() => {
  const state = licenseStore.state
  if (state === LICENSE_STATE.VALID || state === LICENSE_STATE.EXPIRING) {
    return t('license_state_valid')
  }
  return t(`license_state_${state}`)
})

async function handleSubmit() {
  submitting.value = true
  try {
    const result = await installLicenseKey(form.licenseKey.trim())
    message.success(t('license_update_success_toast'))
    emit('success', result)
    // 授权变更影响全局状态（配额、菜单、轮询），短暂展示成功提示后整页刷新。
    setTimeout(() => {
      window.location.reload()
    }, 800)
  } catch (err) {
    message.error(err?.response?.data?.message || t('common_requestFailed'))
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="less">
.current-license {
  margin-bottom: 20px;
  padding: 12px 16px;
  background: #f7f8fb;
  border-radius: 8px;
}

.current-row {
  display: flex;
  align-items: baseline;
  padding: 3px 0;
  font-size: 14px;
  line-height: 22px;
}

.current-label {
  flex-shrink: 0;
  width: 96px;
  color: #667085;
}

.current-value {
  color: #1f2937;
  word-break: break-all;
}

.action-row {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

.action-btn {
  flex: 1;

  &.primary {
    background: #3157e2;
    border-color: #3157e2;

    &:hover,
    &:focus {
      background: #2548c4;
      border-color: #2548c4;
    }
  }
}
</style>
