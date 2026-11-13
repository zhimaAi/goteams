<template>
  <a-modal
    v-model:open="innerOpen"
    :title="$t('license_offline_title')"
    :footer="null"
    :width="520"
    :maskClosable="false"
    destroyOnClose
  >
    <div class="offline-activation-body">
      <div class="device-code-block">
        <div class="block-label">{{ $t('license_device_code_label') }}</div>
        <div class="device-code-row">
          <span class="device-code">{{ licenseStore.deviceCode }}</span>
          <a-button size="small" @click="copyDeviceCode">
            {{ copied ? $t('license_copied') : $t('license_copy') }}
          </a-button>
        </div>
        <p class="block-tip">{{ $t('license_device_code_tip') }}</p>
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

        <a-button
          type="primary"
          html-type="submit"
          class="submit-button"
          :loading="submitting"
          block
        >
          {{ $t('license_install_btn') }}
        </a-button>
      </a-form>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { activateLicenseOffline } from '@/api/license'
import { useLicenseStore } from '@/stores/license'
import { writeClipboardText } from '@/utils/workItemClipboard'

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
const copied = ref(false)

const form = reactive({
  licenseKey: '',
})

const rules = {
  licenseKey: [
    { required: true, message: t('license_key_required'), trigger: 'blur' },
  ],
}

async function copyDeviceCode() {
  try {
    await writeClipboardText(licenseStore.deviceCode)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    message.error(t('license_copy_failed'))
  }
}

async function handleSubmit() {
  submitting.value = true
  try {
    const result = await activateLicenseOffline(form.licenseKey.trim())
    emit('success', result)
  } catch (err) {
    message.error(err?.response?.data?.message || t('common_requestFailed'))
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="less">
.device-code-block {
  margin-bottom: 20px;
  padding: 12px 16px;
  background: #f7f8fb;
  border-radius: 8px;
}

.block-label {
  margin-bottom: 8px;
  font-size: 14px;
  color: #4b5563;
}

.device-code-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.device-code {
  font-family: monospace;
  font-size: 15px;
  font-weight: 600;
  color: #1f2937;
  word-break: break-all;
}

.block-tip {
  margin: 8px 0 0;
  font-size: 12px;
  line-height: 20px;
  color: #98a2b3;
}

.submit-button {
  height: 44px;
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
