<template>
  <div class="online-activation-form">
    <a-form ref="formRef" :model="form" :rules="rules" layout="vertical" @finish="handleSubmit">
      <a-form-item name="corpName">
        <a-input
          v-model:value="form.corpName"
          :placeholder="$t('license_corp_name_placeholder')"
          :maxlength="100"
          size="large"
        >
          <template #prefix>
            <img :src="iconEnterprise" class="input-icon" alt="" />
          </template>
        </a-input>
      </a-form-item>

      <a-form-item name="phone">
        <a-input
          v-model:value="form.phone"
          :placeholder="$t('license_phone_placeholder')"
          size="large"
        >
          <template #prefix>
            <img :src="iconUser" class="input-icon" alt="" />
          </template>
        </a-input>
      </a-form-item>

      <a-form-item name="email">
        <a-input
          v-model:value="form.email"
          :placeholder="$t('license_email_placeholder')"
          size="large"
        >
          <template #prefix>
            <img :src="iconEmail" class="input-icon" alt="" />
          </template>
        </a-input>
      </a-form-item>

      <a-form-item name="verifyCode">
        <div class="verify-code-row">
          <a-input
            v-model:value="form.verifyCode"
            :placeholder="$t('license_verify_code_placeholder')"
            :maxlength="6"
            size="large"
            class="verify-code-input"
          >
            <template #prefix>
              <img :src="iconLock" class="input-icon" alt="" />
            </template>
          </a-input>
          <a-button
            class="send-code-btn"
            :disabled="countdown > 0"
            :loading="sendCodeLoading"
            @click="handleSendCode"
          >
            {{ countdown > 0 ? $t('license_resend_code', { s: countdown }) : $t('license_get_code') }}
          </a-button>
        </div>
      </a-form-item>

      <a-button
        type="primary"
        html-type="submit"
        class="submit-button"
        :loading="submitting"
        block
      >
        {{ $t('license_activate_now') }}
      </a-button>
    </a-form>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { sendLicenseVerifyCode, activateLicenseOnline } from '@/api/license'
import iconEnterprise from '@/image/login/icon-enterprise.svg'
import iconEmail from '@/image/login/icon-email.svg'
import iconLock from '@/image/login/icon-lock.svg'
import iconUser from '@/image/login/icon-user.svg'

const { t } = useI18n()

const emit = defineEmits(['success'])

const formRef = ref()
const submitting = ref(false)
const sendCodeLoading = ref(false)
const countdown = ref(0)
let countdownTimer = null

const form = reactive({
  corpName: '',
  phone: '',
  email: '',
  verifyCode: '',
})

// request_id stays fixed across retries of the same form session so a retried
// submission after a network failure stays idempotent on the operations backend.
const requestId = ref('')

// randomUUID only exists in secure contexts; plain-HTTP LAN deployments need the fallback.
function generateRequestId() {
  if (window.crypto && typeof window.crypto.randomUUID === 'function') {
    return window.crypto.randomUUID()
  }
  return (
    'req-' +
    Date.now().toString(36) +
    '-' +
    Math.random().toString(36).slice(2, 12) +
    '-' +
    Math.random().toString(36).slice(2, 12)
  )
}

onMounted(() => {
  requestId.value = generateRequestId()
})

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer)
})

const phonePattern = /^\+?[0-9]{5,20}$/

const rules = {
  corpName: [
    { required: true, message: t('license_corp_name_required'), trigger: 'blur' },
  ],
  phone: [
    { required: true, message: t('license_phone_required'), trigger: 'blur' },
    { pattern: phonePattern, message: t('license_phone_invalid'), trigger: 'blur' },
  ],
  email: [
    { required: true, message: t('license_email_required'), trigger: 'blur' },
    { type: 'email', message: t('license_email_invalid'), trigger: 'blur' },
  ],
  verifyCode: [
    { required: true, message: t('license_code_required'), trigger: 'blur' },
    { pattern: /^\d{6}$/, message: t('license_code_format'), trigger: 'blur' },
  ],
}

function startCountdown(seconds = 60) {
  countdown.value = seconds
  if (countdownTimer) clearInterval(countdownTimer)
  countdownTimer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) {
      clearInterval(countdownTimer)
      countdownTimer = null
    }
  }, 1000)
}

async function handleSendCode() {
  if (!form.email) {
    message.warning(t('license_email_required'))
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
    message.warning(t('license_email_invalid'))
    return
  }
  sendCodeLoading.value = true
  try {
    await sendLicenseVerifyCode(form.email)
    message.success(t('license_code_sent'))
    startCountdown()
  } catch (err) {
    message.error(err?.response?.data?.message || t('common_requestFailed'))
  } finally {
    sendCodeLoading.value = false
  }
}

async function handleSubmit() {
  submitting.value = true
  try {
    const result = await activateLicenseOnline({
      corp_name: form.corpName.trim(),
      phone: form.phone.trim(),
      email: form.email.trim(),
      verify_code: form.verifyCode.trim(),
      request_id: requestId.value,
    })
    emit('success', result)
  } catch (err) {
    message.error(err?.response?.data?.message || t('common_requestFailed'))
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="less">
.input-icon {
  width: 16px;
  height: 16px;
  display: block;
}

.verify-code-row {
  display: flex;
  gap: 16px;
  align-items: center;
}

.verify-code-input {
  flex: 1;
}

.send-code-btn {
  width: 136px;
  height: 48px;
  padding: 12px 16px;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  background: #fff;
  color: #595959;
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
  white-space: nowrap;
  box-shadow: none;

  &:hover:not(:disabled) {
    border-color: #3157e2;
    color: #3157e2;
  }

  &:disabled {
    color: #d9d9d9;
    border-color: #d9d9d9;
    cursor: not-allowed;
  }
}

.submit-button {
  height: 48px;
  border-radius: 6px;
  font-size: 16px;
  line-height: 24px;
  background: #3157e2;
  border-color: #3157e2;

  &:hover,
  &:focus {
    background: #2548c4;
    border-color: #2548c4;
  }
}

@media (max-width: 480px) {
  .verify-code-row {
    flex-direction: column;
    gap: 8px;
  }

  .send-code-btn {
    width: 100%;
  }
}
</style>
