<template>
  <div class="requirement-aging">
    <div v-if="canOpenAgingSettings" class="aging-toolbar">
      <a-button type="link" size="small" @click="openAgingSettings">
        <SettingOutlined /> {{ $t("aging_timeout_setting") }}
      </a-button>
    </div>
    <a-spin v-if="loading" />
    <a-empty
      v-else-if="!nodes.length"
      :image="simpleImage"
      :description="$t('aging_no_history')"
    />
    <a-table
      v-else
      :columns="columns"
      :data-source="nodes"
      :pagination="false"
      row-key="rowKey"
      size="small"
      :row-class-name="(record) => (record.is_total ? 'aging-total-row' : '')"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status_name'">
          <span v-if="record.is_total" class="aging-total-label">{{
            $t("aging_total_label")
          }}</span>
          <StatusPill
            v-else
            :text="record.status_name"
            :color="record.status_color"
          />
        </template>
        <template v-if="column.key === 'enter_time'">
          {{
            record.enter_time > 0
              ? formatLocalDate(record.enter_time, true)
              : "-"
          }}
        </template>
        <template v-if="column.key === 'handler'">
          {{ record.handler || "-" }}
        </template>
        <template v-if="column.key === 'duration_text'">
          <a-tooltip v-if="record.tooltip" :title="record.tooltip">
            <span class="aging-duration">{{ record.duration_text }}</span>
          </a-tooltip>
          <span v-else class="aging-duration">{{ record.duration_text }}</span>
        </template>
        <template v-if="column.key === 'overdue_state'">
          <span
            class="aging-state-tag"
            :class="`aging-state-${record.overdue_state}`"
            >{{ overdueStateText(record.overdue_state) }}</span
          >
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup>
import { computed, ref, watch, onMounted } from "vue";
import { Empty } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { SettingOutlined } from "@ant-design/icons-vue";
import { getRequirementAging } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { formatLocalDate } from "@/utils/date";
import { AGING_CONTROL_ANCHOR } from "@/constants/statusTransition";
import StatusPill from "@/components/work-item/StatusPill.vue";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  requirementId: { type: [String, Number], default: "" },
  // Bumped by the detail drawer after an inline field edit (e.g. the assignee) so the rows
  // that follow the live data re-fetch without the user leaving the pane and coming back.
  reloadKey: { type: Number, default: 0 },
});

const { t: $t } = useI18n();
const router = useRouter();
const authStore = useAuthStore();
// Project admins carry requirement_setting:status by default, which is also what gates the
// Status tab and the aging card. Aging control lives in the project-scoped group
// project_settings_requirement, absent from enterprise roles, so enterprise admins are
// recognized by their enterprise project-management permission instead. Permissions are the
// only signal used here: roles are not matched by name or id (see role-permission.md).
const canOpenAgingSettings = computed(
  () =>
    authStore.hasPermission("requirement_setting:status") ||
    authStore.hasEnterprisePermission("project:manage"),
);
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const loading = ref(false);
const nodes = ref([]);
let requestSequence = 0;

const columns = [
  {
    title: $t("aging_table_status"),
    key: "status_name",
    dataIndex: "status_name",
  },
  {
    title: $t("aging_table_enter_time"),
    key: "enter_time",
    dataIndex: "enter_time",
  },
  { title: $t("aging_table_handler"), key: "handler", dataIndex: "handler" },
  {
    title: $t("aging_table_duration"),
    key: "duration_text",
    dataIndex: "duration_text",
  },
  {
    title: $t("aging_table_overdue"),
    key: "overdue_state",
    dataIndex: "overdue_state",
  },
];

function overdueStateText(state) {
  if (state === "overdue") return $t("aging_overdue");
  if (state === "warning") return $t("aging_soon");
  if (state === "not_counted") return $t("aging_not_counted");
  return $t("aging_normal");
}

function openAgingSettings() {
  const wsId = props.wsId;
  if (!wsId) return;
  const target = router.resolve({
    path: `/workspace/${wsId}/settings`,
    query: {
      section: "requirement",
      tab: "status",
      anchor: AGING_CONTROL_ANCHOR,
    },
  });
  window.open(target.href, "_blank", "noopener,noreferrer");
}

async function load() {
  const id = String(props.requirementId || "");
  if (!props.wsId || !id) return;
  const sequence = ++requestSequence;
  loading.value = true;
  try {
    const result = await getRequirementAging(props.wsId, id);
    if (sequence !== requestSequence) return;
    const list = (result && result.nodes) || [];
    list.forEach((item, index) => {
      item.rowKey = `aging-${index}`;
    });
    nodes.value = list;
  } catch {
    if (sequence === requestSequence) nodes.value = [];
  } finally {
    if (sequence === requestSequence) loading.value = false;
  }
}

onMounted(load);
watch(() => [props.wsId, props.requirementId, props.reloadKey], load);
</script>

<style scoped>
.requirement-aging {
  padding: 4px 0;
  padding-left: 16px;
}
.aging-toolbar {
  margin-bottom: 4px;
}
.aging-total-row {
  background: #fafafa;
}
.aging-total-row:hover > td {
  background: #f5f5f5 !important;
}
.aging-total-label {
  color: rgba(0, 0, 0, 0.85);
}
.aging-duration {
  cursor: help;
}
.aging-state-tag {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 10px;
  border-radius: 12px;
  font-size: 12px;
  line-height: 22px;
  white-space: nowrap;
}
.aging-state-normal {
  color: #389e0d;
  background: #f6ffed;
  border: 1px solid #b7eb8f;
}
.aging-state-warning {
  color: #d46b08;
  background: #fff7e6;
  border: 1px solid #ffd591;
}
.aging-state-overdue {
  color: #cf1322;
  background: #fff1f0;
  border: 1px solid #ffa39e;
}
.aging-state-not_counted {
  color: #8c8c8c;
  background: #fafafa;
  border: 1px solid #d9d9d9;
}
</style>
