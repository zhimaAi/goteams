<template>
  <a-modal
    v-model:open="innerOpen"
    :title="title || $t('plan_upgradeRenewal')"
    :footer="null"
    :width="420"
    centered
    :z-index="3000"
    class="upgrade-renewal-modal"
  >
    <div class="upgrade-renewal-body">
      <p class="upgrade-desc">{{ $t("plan_upgradeRenewalDesc") }}</p>
      <div class="qr-code-wrap">
        <img
          :src="upgradeRenewalImg"
          :alt="title || $t('plan_upgradeRenewal')"
          class="qr-code"
        />
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { computed } from "vue";
import upgradeRenewalImg from "@/image/plan/upgrade-renewal.png";

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, default: "" },
});
const emit = defineEmits(["update:open"]);

const innerOpen = computed({
  get: () => props.open,
  set: (val) => emit("update:open", val),
});
</script>

<style scoped lang="less">
.upgrade-renewal-modal {
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

.upgrade-renewal-body {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.upgrade-desc {
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
