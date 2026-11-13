<template>
  <a-modal
    v-model:open="visible"
    :width="560"
    :footer="null"
    centered
    :closable="true"
    :z-index="2000"
    class="quota-exceeded-modal"
    @cancel="handleClose"
  >
    <template #title>
      <span class="modal-title">
        <WarningFilled class="warning-icon" />
        {{ $t("plan_quotaExceededTitle") }}
      </span>
    </template>

    <div class="quota-modal-body">
      <!-- Current plan card -->
      <div class="plan-card">
        <div class="plan-card-main">
          <div class="plan-icon"><CrownOutlined /></div>
          <div>
            <div class="plan-title">
              {{ $t("plan_currentVersion") }}：
              <span>{{ planName }}</span>
            </div>
            <div class="plan-description">
              {{ $t("plan_quotaExceededDescription") }}
            </div>
          </div>
        </div>
        <a-button
          v-if="planUsage.plan_type !== 'enterprise'"
          class="upgrade-btn"
          @click="handleUpgrade"
        >
          {{ $t("plan_upgradeVersion") }}
        </a-button>
      </div>

      <!-- Usage list -->
      <div class="usage-list">
        <div v-for="item in usageRows" :key="item.key" class="usage-row">
          <div class="usage-row-header">
            <span class="usage-label">{{ item.label }}</span>
            <span
              class="usage-value"
              :class="{ 'is-full': usagePercent(item) >= 100 }"
            >
              {{ usageUsedText(item) }} / {{ item.limit }}{{ usageUnit(item) }}
              <span v-if="usagePercent(item) >= 100" class="full-tag">{{
                $t("plan_full")
              }}</span>
            </span>
          </div>
          <a-progress
            :percent="usagePercent(item)"
            :show-info="false"
            :stroke-color="usageColor(item)"
            :trail-color="'#f1f5f9'"
            :size="[260, 8]"
          />
        </div>
      </div>

      <!-- Bottom hint -->
      <div v-if="upgradeHintText" class="upgrade-hint">
        <WarningFilled class="hint-icon" />
        <span>{{ upgradeHintText }}</span>
      </div>
    </div>

    <!-- Upgrade plan image modal -->
    <UpgradeRenewalModal v-model:open="upgradeOpen" />
  </a-modal>
</template>

<script setup>
import { ref, computed, watch } from "vue";
import { useI18n } from "vue-i18n";
import { CrownOutlined, WarningFilled } from "@ant-design/icons-vue";
import { getEnterprisePlanUsage } from "@/api/workspace";
import { registerQuotaModal } from "@/utils/quotaModal";
import UpgradeRenewalModal from "@/components/common/UpgradeRenewalModal.vue";

const { t } = useI18n();

const visible = ref(false);
const upgradeOpen = ref(false);
const loading = ref(false);
const planUsage = ref({ plan_type: "experience", expire_at: 0, items: [] });

const planName = computed(() =>
  planUsage.value.plan_type === "enterprise"
    ? t("plan_enterprise")
    : t("plan_experience"),
);

const usageRows = computed(() =>
  (planUsage.value.items || []).map((item) => ({
    ...item,
    label: usageLabel(item.key),
  })),
);

const upgradeHintText = computed(() => {
  if (planUsage.value.plan_type === "enterprise") return "";
  const details = (planUsage.value.items || [])
    .map((item) => {
      const limitText = enterpriseLimitText(item);
      if (!limitText) return "";
      return `${usageLabel(item.key)}${limitText}`;
    })
    .filter(Boolean)
    .join("、");
  if (!details) return "";
  return t("plan_quotaExceededHint", {
    version: planName.value,
    details,
  });
});

function usageLabel(key) {
  return t(`plan_quota_${key}`);
}

function usageUnit(record) {
  return record.key === "storage" ? "G" : "";
}

// storage used is in bytes, limit is in GB. Convert to GB with unit for display (two decimal places).
function usageUsedText(record) {
  if (record.key === "storage") {
    return (Number(record.used) / 1024 / 1024 / 1024).toFixed(2) + "G";
  }
  return record.used;
}

function usagePercent(record) {
  if (!record.limit) return 0;
  let used = Number(record.used);
  // storage used is in bytes, limit is in GB. Normalize to the same unit before calculating percentage.
  if (record.key === "storage") used = used / 1024 / 1024 / 1024;
  return Math.min(Math.round((used / Number(record.limit)) * 100), 100);
}

function usageColor(record) {
  const percent = usagePercent(record);
  if (percent >= 100) return "#ef4444";
  if (percent > 80) return "#f59e0b";
  return "#2563eb";
}

function enterpriseLimitText(record) {
  if (record.key === "storage") return `${record.enterprise_limit}G`;
  if (record.key === "member") return `${record.enterprise_limit}人`;
  if (record.key === "custom_field" || record.key === "automation_rule") {
    return `${record.enterprise_limit}个`;
  }
  if (record.key === "automation_exec") {
    return `${record.enterprise_limit}次`;
  }
  return String(record.enterprise_limit);
}

async function open() {
  visible.value = true;
  loading.value = true;
  try {
    planUsage.value = (await getEnterprisePlanUsage()) || planUsage.value;
  } catch {
    // error handled by interceptor
  } finally {
    loading.value = false;
  }
}

function handleClose() {
  visible.value = false;
}

function handleUpgrade() {
  // Open the same upgrade image modal used in MemberPage.
  upgradeOpen.value = true;
}

watch(visible, (val) => {
  if (val) open();
});

registerQuotaModal(() => {
  visible.value = true;
});
</script>

<style scoped lang="less">
.quota-exceeded-modal {
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

.quota-modal-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.plan-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #f8fafc;
}

.plan-card-main {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}

.plan-icon {
  display: flex;
  width: 44px;
  height: 44px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  color: #2563eb;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  font-size: 20px;
}

.plan-title {
  margin-bottom: 2px;
  color: #0f172a;
  font-size: 15px;
  font-weight: 600;

  span {
    color: #2563eb;
  }
}

.plan-description {
  color: #64748b;
  font-size: 12px;
}

.upgrade-btn {
  height: 34px;
  padding: 0 16px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 13px;
  color: #fff;
  border: none;
  background: linear-gradient(135deg, rgb(124, 58, 237), rgb(91, 33, 182));
  box-shadow: rgba(124, 58, 237, 0.35) 0px 4px 14px;

  &:hover,
  &:focus {
    color: #fff;
    border: none;
    background: linear-gradient(135deg, rgb(124, 58, 237), rgb(91, 33, 182));
  }
}

.usage-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.usage-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.usage-row-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.usage-label {
  color: #334155;
  font-size: 14px;
  font-weight: 500;
}

.usage-value {
  color: #64748b;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;

  &.is-full {
    color: #ef4444;
  }
}

.full-tag {
  display: inline-block;
  margin-left: 6px;
  padding: 0 6px;
  border-radius: 4px;
  color: #fff;
  background: #ef4444;
  font-size: 11px;
  font-weight: 500;
  line-height: 18px;
}

.upgrade-hint {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px 14px;
  border-radius: 8px;
  background: #fffbeb;
  color: #92400e;
  font-size: 12px;
  line-height: 1.6;

  .hint-icon {
    flex: none;
    margin-top: 2px;
    color: #f59e0b;
    font-size: 14px;
  }
}

:deep(.ant-progress-line) {
  margin: 0;
  font-size: 0;
}
</style>
