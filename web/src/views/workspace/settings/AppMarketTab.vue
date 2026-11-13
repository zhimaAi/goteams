<!--
 * AppMarketTab.vue
 * Project settings > app market: one card per optional module (currently the
 * gantt chart) with a switch that toggles it for the workspace.
 -->
<template>
  <div class="app-market">
    <div class="app-market-grid">
      <div class="app-card">
        <div class="app-card-head">
          <div class="app-card-icon"><BarChartOutlined /></div>
          <div class="app-card-info">
            <div class="app-card-name">{{ $t("gantt_app_name") }}</div>
            <div
              class="app-card-status"
              :class="{ 'app-card-status--on': ganttEnabled }"
            >
              {{
                ganttEnabled
                  ? $t("gantt_app_enabled")
                  : $t("gantt_app_disabled")
              }}
            </div>
          </div>
          <a-switch
            :checked="ganttEnabled"
            :loading="switching"
            @change="handleToggle"
          />
        </div>
        <div class="app-card-desc">{{ $t("gantt_app_desc") }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from "vue";
import { message } from "ant-design-vue";
import { BarChartOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  getWorkspaceMetadata,
  updateWorkspaceGanttEnabled,
} from "@/api/workspace";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
});

const { t } = useI18n();
const ganttEnabled = ref(false);
const switching = ref(false);

onMounted(async () => {
  const metadata = await getWorkspaceMetadata(props.wsId);
  ganttEnabled.value = !!metadata?.settings?.gantt_enabled;
});

async function handleToggle(checked) {
  switching.value = true;
  try {
    await updateWorkspaceGanttEnabled(props.wsId, { enabled: checked });
    ganttEnabled.value = checked;
    message.success(
      checked ? t("gantt_app_enable_success") : t("gantt_app_disable_success"),
    );
  } finally {
    switching.value = false;
  }
}
</script>

<style scoped lang="less">
.app-market {
  padding-top: 8px;
}
.app-market-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}
.app-card {
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  padding: 16px;
  transition: box-shadow 0.2s;
  &:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  }
}
.app-card-head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.app-card-icon {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: #e6f4ff;
  color: #1677ff;
  font-size: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.app-card-info {
  flex: 1;
  min-width: 0;
}
.app-card-name {
  font-weight: 600;
  font-size: 15px;
  line-height: 22px;
}
.app-card-status {
  font-size: 12px;
  color: #999;
  &--on {
    color: #52c41a;
  }
}
.app-card-desc {
  margin-top: 12px;
  color: #666;
  font-size: 13px;
  line-height: 20px;
}
</style>
