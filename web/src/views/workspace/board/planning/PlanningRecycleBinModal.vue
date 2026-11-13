<template>
  <a-modal
    :open="open"
    :title="$t('planning_recycleBin')"
    :width="900"
    :footer="null"
    centered
    destroy-on-close
    @cancel="close"
  >
    <a-table
      class="recycle-table"
      :columns="columns"
      :data-source="rows"
      :loading="loading"
      :pagination="false"
      :row-selection="canEdit ? rowSelection : undefined"
      :locale="{ emptyText: $t('planning_recycleEmpty') }"
      row-key="id"
      size="small"
      :scroll="{ y: 360 }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'deleted_at'">
          <span class="muted">{{
            formatLocalDate(record.deleted_at, true)
          }}</span>
        </template>
        <template v-else-if="column.key === 'deleted_by'">
          <AssigneeCell
            :ids="[record.deleted_by]"
            :names="[record.deleted_by_name]"
          />
        </template>
        <template v-else-if="column.key === 'title'">
          <span class="title-text" :title="record.title">{{
            record.title
          }}</span>
        </template>
        <template v-else-if="column.key === 'action'">
          <span class="row-actions">
            <a @click="submitRestore([record.id])">{{
              $t("planning_recycleRestore")
            }}</a>
            <a class="danger" @click="confirmPurge([record.id])">{{
              $t("planning_recyclePurge")
            }}</a>
          </span>
        </template>
      </template>
    </a-table>

    <div class="recycle-footer">
      <a-button
        v-if="canEdit"
        :disabled="!selectedKeys.length"
        :loading="restoring"
        @click="submitRestore(selectedKeys)"
      >
        {{ $t("planning_recycleRestoreCount", { n: selectedKeys.length }) }}
      </a-button>
      <a-button
        v-if="canEdit"
        danger
        :disabled="!selectedKeys.length"
        :loading="purging"
        @click="confirmPurge(selectedKeys)"
      >
        {{ $t("planning_recyclePurgeCount", { n: selectedKeys.length }) }}
      </a-button>
      <a-button @click="close">{{ $t("common_close") }}</a-button>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { Modal, message } from "ant-design-vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import { formatLocalDate } from "@/utils/date";
import {
  listDeletedPlannings,
  purgePlannings,
  restorePlannings,
} from "@/api/planning";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], required: true },
  canEdit: { type: Boolean, default: false },
});
const emit = defineEmits(["update:open", "changed"]);
const { t: $t } = useI18n();

const rows = ref([]);
const loading = ref(false);
const restoring = ref(false);
const purging = ref(false);
const selectedKeys = ref([]);

const columns = computed(() => [
  { title: $t("planning_recycleDeletedAt"), key: "deleted_at", width: 170 },
  { title: $t("planning_recycleDeletedBy"), key: "deleted_by", width: 140 },
  { title: $t("planning_filterTitle"), key: "title", ellipsis: true },
  {
    title: $t("planning_recycleGroup"),
    key: "group_name",
    dataIndex: "group_name",
    width: 150,
    ellipsis: true,
  },
  { title: $t("workspace_action"), key: "action", width: 120 },
]);

const rowSelection = computed(() => ({
  selectedRowKeys: selectedKeys.value,
  onChange: (keys) => {
    selectedKeys.value = keys;
  },
}));

async function reload() {
  loading.value = true;
  try {
    rows.value = (await listDeletedPlannings(props.wsId)) || [];
    // Drop selections whose rows no longer exist after a restore or purge.
    const ids = new Set(rows.value.map((item) => item.id));
    selectedKeys.value = selectedKeys.value.filter((key) => ids.has(key));
  } catch {
    /* handled by interceptor */
  } finally {
    loading.value = false;
  }
}

async function submitRestore(ids) {
  if (!ids?.length || restoring.value) return;
  restoring.value = true;
  try {
    const result = await restorePlannings(props.wsId, ids);
    message.success(
      $t("planning_recycleRestoreSuccess", {
        n: result?.affected ?? ids.length,
      }),
    );
    // A restore may revive a deleted group, so the panel reloads groups as well as rows.
    emit("changed");
    await reload();
  } catch {
    /* handled by interceptor */
  } finally {
    restoring.value = false;
  }
}

function confirmPurge(ids) {
  if (!ids?.length) return;
  Modal.confirm({
    title: $t("planning_recyclePurge"),
    content: $t("planning_recyclePurgeConfirm", { n: ids.length }),
    okText: $t("common_confirm"),
    cancelText: $t("common_cancel"),
    okButtonProps: { danger: true },
    onOk: () => submitPurge(ids),
  });
}

async function submitPurge(ids) {
  purging.value = true;
  try {
    const result = await purgePlannings(props.wsId, ids);
    message.success(
      $t("planning_recyclePurgeSuccess", { n: result?.affected ?? ids.length }),
    );
    await reload();
  } catch {
    /* handled by interceptor */
  } finally {
    purging.value = false;
  }
}

function close() {
  emit("update:open", false);
}

watch(
  () => props.open,
  (value) => {
    if (!value) return;
    selectedKeys.value = [];
    reload();
  },
);
</script>

<style scoped lang="less">
.recycle-table :deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  font-weight: 600;
}
.muted {
  color: var(--app-text-secondary);
}
.title-text {
  font-weight: 500;
}
.row-actions {
  display: inline-flex;
  gap: 12px;
}
.row-actions .danger {
  color: #ff4d4f;
}
.recycle-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
</style>
