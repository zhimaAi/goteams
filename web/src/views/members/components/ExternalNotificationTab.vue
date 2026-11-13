<template>
  <div class="external-notification-tab">
    <div class="notification-toolbar">
      <span class="notification-hint">{{
        $t("external_notification_hint")
      }}</span>
      <a-button type="primary" @click="openCreate">
        <PlusOutlined />{{ $t("external_notification_add") }}
      </a-button>
    </div>

    <div ref="tableWrapRef" class="notification-table-wrap">
      <a-table
        :columns="columns"
        :data-source="strategies"
        :loading="loading"
        row-key="id"
        :pagination="false"
        :scroll="{ x: tableScrollX, y: bodyHeight }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'"
            ><div>{{ record.name }}</div></template
          >
          <template v-else-if="column.key === 'members'">
            <a-space v-if="record.members?.length" :size="4">
              <a-avatar-group :max-count="3">
                <a-avatar
                  v-for="member in record.members"
                  :key="member.id"
                  :src="getAvatarSrc(member)"
                  >{{ getInitial(member) }}</a-avatar
                >
              </a-avatar-group>
              <span>{{
                $t("external_notification_member_count", {
                  n: record.member_ids?.length || 0,
                })
              }}</span>
            </a-space>
            <span v-else>—</span>
          </template>
          <template v-else-if="column.key === 'platform'"
            ><a-tag :color="platformMeta(record.platform).color">{{
              platformMeta(record.platform).label
            }}</a-tag></template
          >
          <template v-else-if="column.key === 'events'"
            ><a-space wrap
              ><a-tag v-for="event in record.events" :key="event">{{
                eventLabel(event)
              }}</a-tag></a-space
            ></template
          >
          <template v-else-if="column.key === 'actions'">
            <a-space>
              <a-switch
                :checked="Number(record.enabled) === 1"
                :checked-children="$t('automation_on')"
                :un-checked-children="$t('automation_off')"
                :loading="statusUpdatingId === record.id"
                @change="(enabled) => toggleStrategy(record, enabled)"
              />
              <a-button
                type="link"
                size="small"
                :loading="testingId === record.id"
                @click="testPush(record)"
                >{{ $t("external_notification_test_push") }}</a-button
              >
              <a-button type="link" size="small" @click="openEdit(record)">{{
                $t("common_edit")
              }}</a-button>

              <a-popconfirm
                :title="$t('external_notification_delete_confirm')"
                :description="$t('external_notification_delete_description')"
                :ok-text="$t('common_delete')"
                :cancel-text="$t('common_cancel')"
                @confirm="removeStrategy(record)"
              >
                <a-button type="link" danger size="small">{{
                  $t("common_delete")
                }}</a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
        <template #emptyText
          ><a-empty :description="$t('external_notification_empty')"
        /></template>
      </a-table>
    </div>

    <ExternalNotificationStrategyModal
      v-model:open="modalOpen"
      :strategy="editing"
      :members="members"
      @saved="load"
    />
  </div>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { message } from "ant-design-vue";
import { PlusOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  listEnterpriseMembers,
  listExternalNotificationStrategies,
  updateExternalNotificationStrategyStatus,
  testExternalNotificationStrategy,
  deleteExternalNotificationStrategy,
} from "@/api/workspace";
import { getAvatarSrc, getInitial } from "@/utils/avatar";
import ExternalNotificationStrategyModal from "./ExternalNotificationStrategyModal.vue";

const { t } = useI18n();
const props = defineProps({
  active: { type: Boolean, default: false },
});
const strategies = ref([]);
const members = ref([]);
const loading = ref(false);
const editing = ref(null);
const modalOpen = ref(false);
const statusUpdatingId = ref(null);
const testingId = ref(null);
const platforms = computed(() => [
  { value: "feishu", label: t("external_notification_feishu"), color: "blue" },
  { value: "wecom", label: t("external_notification_wecom"), color: "green" },
  {
    value: "dingtalk",
    label: t("external_notification_dingtalk"),
    color: "cyan",
  },
]);
const columns = computed(() => [
  {
    title: t("external_notification_name"),
    key: "name",
    dataIndex: "name",
    width: 180,
  },
  { title: t("external_notification_members"), key: "members", width: 190 },
  { title: t("external_notification_platform"), key: "platform", width: 110 },
  { title: t("external_notification_events"), key: "events", width: 250 },
  {
    title: t("external_notification_today"),
    key: "today_trigger_count",
    dataIndex: "today_trigger_count",
    width: 120,
  },
  {
    title: t("external_notification_total"),
    key: "total_trigger_count",
    dataIndex: "total_trigger_count",
    width: 120,
  },
  {
    title: t("external_notification_actions"),
    key: "actions",
    fixed: "right",
    width: 250,
  },
]);
const platformMeta = (value) =>
  platforms.value.find((item) => item.value === value) || {
    label: "",
    color: "default",
  };
const tableWrapRef = ref(null);
// The tab pane has a fixed height and clips overflow, so the table body height is measured instead of growing.
const bodyHeight = ref(240);
let wrapObserver = null;
const tableScrollX = computed(() =>
  columns.value.reduce((total, column) => total + (column.width || 0), 0),
);
function updateBodyHeight() {
  const wrap = tableWrapRef.value;
  if (!wrap) return;
  const header = wrap.querySelector(".ant-table-header");
  const headerHeight = header?.getBoundingClientRect().height || 0;
  bodyHeight.value = Math.max(160, wrap.clientHeight - headerHeight);
}
const eventLabel = (value) => t(`external_notification_event_${value}`);
function openCreate() {
  editing.value = null;
  modalOpen.value = true;
}
function openEdit(record) {
  editing.value = record;
  modalOpen.value = true;
}
async function load() {
  loading.value = true;
  try {
    strategies.value = (await listExternalNotificationStrategies()) || [];
    members.value = (await listEnterpriseMembers()) || [];
  } finally {
    loading.value = false;
  }
}
async function removeStrategy(record) {
  await deleteExternalNotificationStrategy(record.id);
  strategies.value = strategies.value.filter((item) => item.id !== record.id);
  message.success(t("external_notification_deleted"));
}
async function toggleStrategy(record, enabled) {
  statusUpdatingId.value = record.id;
  const previous = record.enabled;
  record.enabled = enabled ? 1 : 0;
  try {
    await updateExternalNotificationStrategyStatus(record.id, enabled);
  } catch {
    record.enabled = previous;
  } finally {
    statusUpdatingId.value = null;
  }
}
async function testPush(record) {
  testingId.value = record.id;
  try {
    await testExternalNotificationStrategy(record.id);
    message.success(
      t("external_notification_test_success", {
        platform: platformMeta(record.platform).label,
      }),
    );
  } finally {
    testingId.value = null;
  }
}
onMounted(() => {
  load();
  updateBodyHeight();
  if (tableWrapRef.value) {
    wrapObserver = new ResizeObserver(updateBodyHeight);
    wrapObserver.observe(tableWrapRef.value);
  }
});
onBeforeUnmount(() => {
  wrapObserver?.disconnect();
  wrapObserver = null;
});
// The header only exists once the table has rendered, so re-measure after each load.
watch(loading, (value) => {
  if (!value) nextTick(updateBodyHeight);
});
// Tab 首次激活才挂载，之后组件常驻；切回时重新拉取避免列表过期。
watch(
  () => props.active,
  (active) => {
    if (active) load();
  },
);
</script>

<style scoped>
.external-notification-tab {
  display: flex;
  min-height: 0;
  height: 100%;
  flex-direction: column;
}
.notification-toolbar {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.notification-table-wrap {
  min-height: 0;
  flex: 1;
}
.notification-hint {
  color: #667085;
}
</style>
