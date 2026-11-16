<template>
  <a-card :title="$t('status_action_title')" size="small" class="settings-card">
    <template #extra>
      <a-space>
        <a-button
          size="small"
          :disabled="!ready || saving"
          @click="openActions"
        >
          {{ $t("status_action_configure") }}
        </a-button>
        <a-button
          type="primary"
          size="small"
          :disabled="!canAddMapping"
          @click="addMapping()"
        >
          <PlusOutlined /> {{ $t("common_add") }}
        </a-button>
        <a-switch
          :checked="config.enabled"
          :aria-label="$t('status_action_title')"
          :disabled="!ready || saving"
          @change="setEnabled"
        />
      </a-space>
    </template>
    <a-table
      :columns="columns"
      :data-source="mappingRows"
      :loading="loading"
      :pagination="false"
      row-key="id"
      size="small"
    >
      <template #bodyCell="{ column, record }">
        <template
          v-if="
            mappingDraft?.id === record.id &&
            !(column.key === 'source_status_id' && mappingDraft.lockSource)
          "
        >
          <a-select
            v-if="column.key !== 'controls'"
            v-model:value="mappingDraft[column.key]"
            :aria-label="column.title"
            :options="optionsFor(column.key)"
            :disabled="saving"
            class="mapping-select"
            @change="mappingStatusChanged"
          />
          <a-space v-else :size="4">
            <a-button
              type="primary"
              size="small"
              :loading="saving"
              :disabled="!validMapping"
              @click="saveMapping"
            >
              {{ $t("common_confirm") }}
            </a-button>
            <a-button
              size="small"
              :disabled="saving"
              @click="mappingDraft = null"
            >
              {{ $t("common_cancel") }}
            </a-button>
          </a-space>
        </template>
        <template v-else-if="column.key === 'controls'">
          <a-space :size="4">
            <a-button
              type="text"
              size="small"
              :aria-label="$t('status_action_add')"
              :title="$t('status_action_add')"
              :disabled="
                !canAddMapping || !statusById.has(record.source_status_id)
              "
              @click="addMapping(record.source_status_id)"
            >
              <PlusOutlined />
            </a-button>
            <a-button
              type="text"
              size="small"
              :aria-label="$t('common_edit')"
              :disabled="!!mappingDraft || saving"
              @click="mappingDraft = { ...record }"
            >
              <EditOutlined />
            </a-button>
            <a-button
              type="text"
              size="small"
              danger
              :aria-label="$t('common_delete')"
              :disabled="saving || !!mappingDraft"
              @click="confirmMappingDelete(record)"
            >
              <DeleteOutlined />
            </a-button>
          </a-space>
        </template>
        <template v-else-if="column.key === 'action_id'">
          {{ actionNames.get(record.action_id) }}
        </template>
        <StatusPill
          v-else
          :text="statusById.get(record[column.key])?.name"
          :color="statusById.get(record[column.key])?.color"
        />
      </template>
    </a-table>
  </a-card>

  <a-modal
    :open="actionsOpen"
    :title="$t('status_action_configure')"
    :mask-closable="!saving"
    :closable="!saving"
    :keyboard="!saving"
    @cancel="closeActions"
  >
    <a-list :data-source="actionRows" size="small">
      <template #renderItem="{ item }">
        <a-list-item>
          <a-space v-if="actionDraft?.id === item.id" class="action-editor">
            <a-input
              v-model:value="actionDraft.name"
              :aria-label="$t('status_action_name')"
              :placeholder="$t('status_action_name')"
              :maxlength="5"
              :disabled="saving"
              @press-enter="saveAction"
            />
            <a-button
              type="primary"
              :disabled="!actionDraft.name.trim()"
              :loading="saving"
              @click="saveAction"
            >
              {{ $t("common_confirm") }}
            </a-button>
            <a-button :disabled="saving" @click="actionDraft = null">
              {{ $t("common_cancel") }}
            </a-button>
          </a-space>
          <template v-else>
            {{ item.name }}
            <div class="action-controls">
              <a-button
                type="text"
                size="small"
                :aria-label="$t('common_edit')"
                :disabled="!!actionDraft || saving"
                @click="actionDraft = { ...item }"
              >
                <EditOutlined />
              </a-button>
              <a-popconfirm
                :title="$t('status_action_delete_name', { name: item.name })"
                :ok-text="$t('common_delete')"
                :cancel-text="$t('common_cancel')"
                @confirm="
                  mutate(() =>
                    saveWorkspaceStatusAction(wsId, 'actions', item.id),
                  )
                "
              >
                <a-button
                  type="text"
                  size="small"
                  danger
                  :aria-label="$t('common_delete')"
                  :disabled="!!actionDraft || saving"
                >
                  <DeleteOutlined />
                </a-button>
              </a-popconfirm>
            </div>
          </template>
        </a-list-item>
      </template>
    </a-list>
    <a-button
      class="add-action"
      :disabled="!!actionDraft || saving"
      @click="actionDraft = { id: 0, name: '' }"
    >
      <PlusOutlined /> {{ $t("status_action_add") }}
    </a-button>
    <template #footer>
      <a-button type="primary" :disabled="saving" @click="closeActions">
        {{ $t("common_done") }}
      </a-button>
    </template>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { message, Modal } from "ant-design-vue";
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  getWorkspaceStatusActions,
  saveWorkspaceStatusAction,
  setWorkspaceStatusActionsEnabled,
} from "@/api/workspace";
import StatusPill from "@/components/work-item/StatusPill.vue";
import { useAuthStore } from "@/stores/auth";

const props = defineProps({
  wsId: { type: [Number, String], required: true },
  statuses: { type: Array, required: true },
  reloadKey: { type: Number, required: true },
});
const { t: $t } = useI18n();
const auth = useAuthStore();
const config = ref({ enabled: false, actions: [], mappings: [] });
const ready = ref(false);
const loading = ref(false);
const saving = ref(false);
const actionsOpen = ref(false);
const actionDraft = ref(null);
const mappingDraft = ref(null);
const statusById = computed(
  () => new Map(props.statuses.map((status) => [Number(status.id), status])),
);
const actionNames = computed(
  () => new Map(config.value.actions.map((action) => [action.id, action.name])),
);
const actionRows = computed(() =>
  actionDraft.value?.id === 0
    ? [actionDraft.value, ...config.value.actions]
    : config.value.actions,
);
const columns = computed(() => [
  {
    key: "source_status_id",
    title: $t("workspace_status"),
    width: "25%",
    customCell: (record) => ({
      rowSpan: record.sourceRowSpan,
      style: { verticalAlign: "middle" },
    }),
  },
  { key: "action_id", title: $t("status_action_name"), width: "25%" },
  { key: "target_status_id", title: $t("status_action_target"), width: "25%" },
  {
    key: "controls",
    title: $t("settings_action"),
    width: "25%",
    align: "right",
  },
]);
const mappingRows = computed(() => {
  const order = new Map(
    props.statuses.map((status, index) => [Number(status.id), index]),
  );
  const draft = mappingDraft.value;
  const rows = [...config.value.mappings];
  if (draft?.lockSource) rows.push(draft);
  rows.sort(
    (a, b) =>
      (order.get(a.source_status_id) ?? order.size) -
        (order.get(b.source_status_id) ?? order.size) ||
      a.source_status_id - b.source_status_id ||
      Number(a.id === 0) - Number(b.id === 0) ||
      a.action_id - b.action_id,
  );
  let groupStart;
  const grouped = rows.map((item) => {
    const row = { ...item, sourceRowSpan: 1 };
    if (item.id === draft?.id && !draft.lockSource) {
      groupStart = undefined;
    } else if (groupStart?.source_status_id === item.source_status_id) {
      groupStart.sourceRowSpan += 1;
      row.sourceRowSpan = 0;
    } else {
      groupStart = row;
    }
    return row;
  });
  return draft?.id === 0 && !draft.lockSource ? [draft, ...grouped] : grouped;
});
const canAddMapping = computed(
  () =>
    ready.value &&
    !saving.value &&
    !mappingDraft.value &&
    config.value.actions.length > 0 &&
    props.statuses.length >= 2,
);
const validMapping = computed(() => {
  const draft = mappingDraft.value;
  return (
    draft?.source_status_id &&
    draft.action_id &&
    draft.target_status_id &&
    draft.source_status_id !== draft.target_status_id
  );
});

function optionsFor(key) {
  if (key === "action_id") {
    return config.value.actions.map((action) => ({
      value: action.id,
      label: action.name,
    }));
  }
  return props.statuses
    .filter(
      (status) =>
        key !== "target_status_id" ||
        Number(status.id) !== mappingDraft.value.source_status_id,
    )
    .map((status) => ({ value: Number(status.id), label: status.name }));
}
function mappingStatusChanged() {
  if (
    mappingDraft.value.source_status_id === mappingDraft.value.target_status_id
  ) {
    mappingDraft.value.target_status_id = undefined;
  }
}
function addMapping(sourceStatusId) {
  mappingDraft.value = {
    id: 0,
    source_status_id: sourceStatusId,
    lockSource: sourceStatusId !== undefined,
  };
}
function openActions() {
  actionDraft.value = null;
  actionsOpen.value = true;
}
function closeActions() {
  if (saving.value) return;
  actionsOpen.value = false;
  actionDraft.value = null;
}

let requestSequence = 0;
async function load() {
  const sequence = ++requestSequence;
  loading.value = true;
  try {
    const result = await getWorkspaceStatusActions(props.wsId);
    if (sequence === requestSequence) {
      config.value = result;
      ready.value = true;
    }
  } catch {
    // Request interceptor reports errors.
  } finally {
    if (sequence === requestSequence) loading.value = false;
  }
}
watch(
  () => props.wsId,
  () => {
    ready.value = false;
    config.value = { enabled: false, actions: [], mappings: [] };
    actionDraft.value = null;
    mappingDraft.value = null;
    actionsOpen.value = false;
    void load();
  },
  { immediate: true },
);
watch(
  () => props.reloadKey,
  () => void load(),
);

async function mutate(request) {
  if (saving.value) return false;
  const wsId = props.wsId;
  saving.value = true;
  try {
    const result = await request();
    if (props.wsId !== wsId) return false;
    config.value = result;
    auth.clearProjectMetadata();
    return true;
  } catch (error) {
    if (
      props.wsId === wsId &&
      [400, 404, 409].includes(error.response?.status)
    ) {
      if (error.response.status === 409) {
        message.warning(
          error.response.data?.message || $t("status_action_stale"),
        );
      }
      await load();
    }
    return false;
  } finally {
    saving.value = false;
  }
}

function setEnabled(enabled) {
  return mutate(() => setWorkspaceStatusActionsEnabled(props.wsId, enabled));
}
async function saveAction() {
  const { id, name } = actionDraft.value;
  if (!name.trim()) return;
  if (
    await mutate(() =>
      saveWorkspaceStatusAction(props.wsId, "actions", id, { name }),
    )
  )
    actionDraft.value = null;
}
async function saveMapping() {
  if (!validMapping.value) return;
  const { id, source_status_id, action_id, target_status_id } =
    mappingDraft.value;
  if (
    await mutate(() =>
      saveWorkspaceStatusAction(props.wsId, "mappings", id, {
        source_status_id,
        action_id,
        target_status_id,
      }),
    )
  ) {
    mappingDraft.value = null;
  }
}
function confirmMappingDelete(record) {
  const workspaceId = props.wsId;
  Modal.confirm({
    title: $t("status_action_delete_mapping"),
    content: $t("status_action_mapping_summary", {
      source: statusById.value.get(record.source_status_id)?.name || "",
      action: actionNames.value.get(record.action_id) || "",
      target: statusById.value.get(record.target_status_id)?.name || "",
    }),
    okText: $t("common_confirm"),
    cancelText: $t("common_cancel"),
    onOk() {
      Modal.confirm({
        title: $t("status_action_delete_mapping_final"),
        okText: $t("common_delete"),
        okType: "danger",
        cancelText: $t("common_cancel"),
        onOk() {
          if (props.wsId === workspaceId)
            return mutate(() =>
              saveWorkspaceStatusAction(workspaceId, "mappings", record.id),
            );
        },
      });
    },
  });
}
</script>

<style scoped lang="less">
.mapping-select {
  width: 100%;
}
.action-editor {
  width: 100%;
}
.action-controls {
  margin-inline-start: auto;
}
.add-action {
  margin-top: 12px;
}
</style>
