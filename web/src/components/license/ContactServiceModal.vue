<template>
  <a-modal
    v-model:open="innerOpen"
    :title="$t('license_contact_service_title')"
    :footer="null"
    :width="420"
    centered
  >
    <div class="contact-service-body">
      <div class="device-code-block">
        <div class="device-code-label">
          {{ $t("license_device_code_label") }}
        </div>
        <div class="device-code-row">
          <span class="device-code">{{ licenseStore.deviceCode }}</span>
          <a-button
            type="primary"
            size="small"
            :disabled="!licenseStore.deviceCode"
            @click="copyDeviceCode"
          >
            {{ copied ? $t("license_copied") : $t("license_copy") }}
          </a-button>
        </div>
      </div>
      <p class="contact-desc">{{ $t("license_contact_service_desc") }}</p>
      <div class="qr-code-wrap">
        <img
          :src="serviceQrcode"
          :alt="$t('license_contact_service_title')"
          class="qr-code"
        />
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import serviceQrcode from "@/image/plan/upgrade-renewal.png";
import { useLicenseStore } from "@/stores/license";
import { writeClipboardText } from "@/utils/workItemClipboard";

const { t } = useI18n();
const licenseStore = useLicenseStore();
const copied = ref(false);

const props = defineProps({
  open: { type: Boolean, default: false },
});
const emit = defineEmits(["update:open"]);

const innerOpen = computed({
  get: () => props.open,
  set: (val) => emit("update:open", val),
});

watch(
  () => props.open,
  (open) => {
    if (open && !licenseStore.deviceCode) licenseStore.fetchStatus();
  },
);

async function copyDeviceCode() {
  if (!licenseStore.deviceCode) return;
  try {
    await writeClipboardText(licenseStore.deviceCode);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch {
    message.error(t("license_copy_failed"));
  }
}
</script>

<style scoped lang="less">
.contact-service-body {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.device-code-block {
  width: 100%;
  margin-bottom: 16px;
  padding: 10px 12px;
  border: 1px solid #d9e2f2;
  border-radius: 8px;
  background: #f8fafc;
}

.device-code-label {
  margin-bottom: 6px;
  font-size: 12px;
  line-height: 1;
  color: #64748b;
}

.device-code-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.device-code {
  flex: 1;
  min-width: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  line-height: 32px;
  color: #1f2937;
  white-space: nowrap;
}

.contact-desc {
  margin: 0 0 20px;
  font-size: 14px;
  line-height: 1.6;
  color: #475569;
  text-align: center;
}

.qr-code-wrap {
  display: flex;
  justify-content: center;
}

.qr-code {
  width: 220px;
  height: 220px;
  object-fit: contain;
  border-radius: 8px;
  display: block;
}
</style>
