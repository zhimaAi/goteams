<template>
  <div class="display-settings-panel">
    <a-spin :spinning="loading">
      <div class="setting-group">
        <div>{{ $t("settings_template_edit_display") }}</div>
        <a-radio-group
          :value="editDisplayMode"
          :disabled="saving"
          @update:value="changeEditDisplayMode"
        >
          <a-radio value="create">
            {{ $t("settings_template_display_create") }}
          </a-radio>
          <a-radio value="view">
            {{ $t("settings_template_display_view") }}
          </a-radio>
        </a-radio-group>
      </div>

      <div class="setting-group">
        <div>{{ $t("settings_template_view_display") }}</div>
        <a-radio-group
          :value="viewDisplayMode"
          :disabled="saving"
          @update:value="changeViewDisplayMode"
        >
          <a-radio value="create">
            {{ $t("settings_template_display_create") }}
          </a-radio>
          <a-radio value="status">
            {{ $t("settings_template_display_status") }}
          </a-radio>
        </a-radio-group>
      </div>

      <div v-if="viewDisplayMode === 'status'" class="status-config">
        <div class="status-tabs">
          <a-button
            v-for="status in statuses"
            :key="status.id"
            type="text"
            :class="{
              active: Number(status.id) === Number(activeStatusId),
            }"
            :disabled="saving"
            @click="activeStatusId = status.id"
          >
            {{ status.name }}
          </a-button>
        </div>

        <div v-if="activeStatus" class="field-config">
          <div class="field-column">
            <h5>{{ $t("settings_template_available_fields") }}</h5>
            <a-input
              v-model:value="keyword"
              allow-clear
              :placeholder="$t('settings_template_search_fields')"
            />
            <div
              v-for="field in availableFields"
              :key="field.key"
              class="field-option"
            >
              <span>{{ field.label }}</span>
              <a-button
                type="link"
                size="small"
                :disabled="saving"
                @click="addField(field.key)"
              >
                {{ $t("settings_template_add_field") }}
              </a-button>
            </div>
          </div>

          <div class="field-column selected">
            <div class="selected-title">
              <h5>{{ $t("settings_template_selected_fields") }}</h5>
              <a-button size="small" :disabled="saving" @click="openCopy">
                {{ $t("settings_template_copy_status") }}
              </a-button>
            </div>
            <div ref="selectedListRef" class="selected-field-list">
              <div
                v-for="key in activeKeys"
                :key="key"
                class="field-option selected-field-row"
              >
                <div class="selected-field-main">
                  <a-tooltip :title="$t('settings_dragFieldToSort')">
                    <span class="drag-handle" :class="{ disabled: saving }">
                      <HolderOutlined />
                    </span>
                  </a-tooltip>
                  <span>{{ fieldLabel(key) }}</span>
                </div>
                <a-button
                  type="text"
                  size="small"
                  danger
                  :disabled="saving"
                  @click="removeField(key)"
                >
                  <DeleteOutlined />
                </a-button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </a-spin>

    <a-modal
      v-model:open="copyOpen"
      :title="$t('settings_template_copy_status')"
      :confirm-loading="saving"
      :mask-closable="!saving"
      @ok="copyStatus"
    >
      <a-checkbox-group v-model:value="copyTargets" class="copy-list">
        <a-checkbox
          v-for="status in statuses"
          :key="status.id"
          :value="status.id"
          :disabled="Number(status.id) === Number(activeStatusId) || saving"
        >
          {{ status.name }}
        </a-checkbox>
      </a-checkbox-group>
    </a-modal>
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
import { DeleteOutlined, HolderOutlined } from "@ant-design/icons-vue";
import { message } from "ant-design-vue";
import Sortable from "sortablejs";
import { useI18n } from "vue-i18n";
import {
  getWorkItemDisplaySettings,
  listCustomFields,
  listFieldConfigs,
  listStatuses,
  updateWorkItemDisplaySettings,
} from "@/api/workspace";
import { customTemplateFieldKey } from "@/utils/workItemTemplate";

const DETAIL_DISPLAY_BUILTIN_KEYS = new Set([
  "status",
  "priority",
  "version",
  "category",
  "assignee",
  "planned_start_date",
  "planned_end_date",
  "created_at",
  "created_by",
  "estimated_work_hours",
  "work_progress",
  "completed_work_hours",
  "remaining_work_hours",
  "overtime_work_hours",
]);

const props = defineProps({
  wsId: {
    type: [String, Number],
    default: "",
  },
  typeKey: {
    type: String,
    default: "requirement",
  },
});

const { t } = useI18n();
const loading = ref(false);
const saving = ref(false);
const editDisplayMode = ref("create");
const viewDisplayMode = ref("create");
const statuses = ref([]);
const fields = ref([]);
const statusFields = ref({});
const activeStatusId = ref(0);
const keyword = ref("");
const copyOpen = ref(false);
const copyTargets = ref([]);
const selectedListRef = ref(null);
let selectedSortable = null;

const activeStatus = computed(() =>
  statuses.value.find(
    (item) => Number(item.id) === Number(activeStatusId.value),
  ),
);
const activeKeys = computed(
  () => statusFields.value[String(activeStatusId.value)] || [],
);
const availableFields = computed(() =>
  fields.value.filter(
    (item) =>
      item.enabled &&
      !activeKeys.value.includes(item.key) &&
      (!keyword.value || item.label.includes(keyword.value)),
  ),
);

function fieldLabel(key) {
  return fields.value.find((item) => item.key === key)?.label || key;
}

function settingSnapshot() {
  return {
    editDisplayMode: editDisplayMode.value,
    viewDisplayMode: viewDisplayMode.value,
    statusFields: Object.fromEntries(
      Object.entries(statusFields.value).map(([statusId, keys]) => [
        statusId,
        [...keys],
      ]),
    ),
  };
}

function restoreSnapshot(snapshot) {
  editDisplayMode.value = snapshot.editDisplayMode;
  viewDisplayMode.value = snapshot.viewDisplayMode;
  statusFields.value = snapshot.statusFields;
}

function settingPayload() {
  return {
    edit_display_mode: editDisplayMode.value,
    view_display_mode: viewDisplayMode.value,
    status_fields: Object.entries(statusFields.value).map(
      ([statusId, fieldKeys]) => ({
        status_id: Number(statusId),
        field_keys: fieldKeys,
      }),
    ),
  };
}

async function applyChange(change) {
  if (saving.value) return false;
  const previous = settingSnapshot();
  change();
  saving.value = true;
  selectedSortable?.option("disabled", true);
  try {
    await updateWorkItemDisplaySettings(
      props.wsId,
      props.typeKey,
      settingPayload(),
    );
    message.success(t("common_saved"));
    return true;
  } catch {
    restoreSnapshot(previous);
    return false;
  } finally {
    saving.value = false;
    nextTick(setupSelectedSortable);
  }
}

async function changeEditDisplayMode(value) {
  if (value === editDisplayMode.value) return;
  await applyChange(() => {
    editDisplayMode.value = value;
  });
}

async function changeViewDisplayMode(value) {
  if (value === viewDisplayMode.value) return;
  await applyChange(() => {
    viewDisplayMode.value = value;
  });
}

async function addField(key) {
  await applyChange(() => {
    statusFields.value[String(activeStatusId.value)] = [
      ...activeKeys.value,
      key,
    ];
  });
}

async function removeField(key) {
  await applyChange(() => {
    statusFields.value[String(activeStatusId.value)] = activeKeys.value.filter(
      (item) => item !== key,
    );
  });
}

function openCopy() {
  copyTargets.value = [];
  copyOpen.value = true;
}

async function copyStatus() {
  if (!copyTargets.value.length) {
    copyOpen.value = false;
    return;
  }
  const copied = await applyChange(() => {
    for (const id of copyTargets.value) {
      statusFields.value[String(id)] = [...activeKeys.value];
    }
  });
  if (copied) copyOpen.value = false;
}

function destroySelectedSortable() {
  selectedSortable?.destroy();
  selectedSortable = null;
}

function setupSelectedSortable() {
  destroySelectedSortable();
  const element = selectedListRef.value;
  if (!element || saving.value || activeKeys.value.length < 2) return;
  selectedSortable = Sortable.create(element, {
    handle: ".drag-handle",
    draggable: ".selected-field-row",
    filter: ".ant-btn",
    preventOnFilter: true,
    animation: 180,
    easing: "cubic-bezier(0.22, 0.61, 0.36, 1)",
    forceFallback: true,
    fallbackOnBody: true,
    fallbackTolerance: 3,
    ghostClass: "selected-field-row-ghost",
    chosenClass: "selected-field-row-chosen",
    dragClass: "selected-field-row-drag",
    onEnd: handleSelectedSortEnd,
  });
}

function restoreDraggedRow(item, oldIndex) {
  const container = selectedListRef.value;
  if (!container || !item) return;
  const otherRows = [...container.children].filter((node) => node !== item);
  container.insertBefore(item, otherRows[oldIndex] || null);
}

async function handleSelectedSortEnd({ oldIndex, newIndex, item }) {
  if (
    saving.value ||
    oldIndex == null ||
    newIndex == null ||
    oldIndex === newIndex
  ) {
    return;
  }
  restoreDraggedRow(item, oldIndex);
  const nextKeys = [...activeKeys.value];
  const [movedKey] = nextKeys.splice(oldIndex, 1);
  nextKeys.splice(newIndex, 0, movedKey);
  await applyChange(() => {
    statusFields.value[String(activeStatusId.value)] = nextKeys;
  });
}

async function load() {
  loading.value = true;
  try {
    const [setting, statusList, configs, customs] = await Promise.all([
      getWorkItemDisplaySettings(props.wsId, props.typeKey),
      listStatuses(props.wsId, props.typeKey),
      listFieldConfigs(props.wsId, props.typeKey),
      listCustomFields(props.wsId, props.typeKey),
    ]);
    editDisplayMode.value = setting?.edit_display_mode || "create";
    viewDisplayMode.value = setting?.view_display_mode || "create";
    statuses.value = statusList || [];
    activeStatusId.value = statuses.value[0]?.id || 0;
    fields.value = [
      ...(configs || [])
        .filter((item) => DETAIL_DISPLAY_BUILTIN_KEYS.has(item.key))
        .map((item) => ({
          key: item.key,
          label: item.label,
          enabled: Number(item.enabled) === 1,
          sortOrder: Number(item.sort_order) || 0,
        })),
      ...(customs || []).map((item) => ({
        key: customTemplateFieldKey(item.id),
        label: item.name,
        enabled: Number(item.is_enabled) === 1,
        sortOrder: Number(item.sort_order) || 0,
      })),
    ].sort((left, right) => left.sortOrder - right.sortOrder);
    statusFields.value = Object.fromEntries(
      (setting?.status_fields || []).map((item) => [
        String(item.status_id),
        Array.isArray(item.field_keys) ? [...item.field_keys] : [],
      ]),
    );
  } finally {
    loading.value = false;
  }
}

watch(() => [props.wsId, props.typeKey], load);
watch(
  () => [
    selectedListRef.value,
    activeStatusId.value,
    activeKeys.value,
    saving.value,
  ],
  () => nextTick(setupSelectedSortable),
);
onMounted(load);
onBeforeUnmount(destroySelectedSortable);
</script>

<style scoped lang="less">
.setting-group {
  margin-bottom: 24px;
  display: flex;
  align-items: center;
  gap: 24px;
}

.status-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 16px;
  border-bottom: 1px solid #eee;

  .active {
    border-bottom: 2px solid #1677ff;
    color: #1677ff;
  }
}

.field-config {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.field-column {
  min-height: 260px;
  padding: 12px;
  border: 1px solid #eee;
}

.selected-title,
.field-option,
.selected-field-main {
  display: flex;
  align-items: center;
}

.selected-title,
.field-option {
  justify-content: space-between;
}

.field-option {
  min-height: 42px;
  padding: 8px 4px;
  border-bottom: 1px solid #f2f4f7;
  background: #fff;
}

.selected-field-row {
  transition:
    opacity 0.15s ease,
    background-color 0.15s ease,
    box-shadow 0.2s ease;
}

.selected-field-main {
  min-width: 0;
  gap: 8px;
}

.drag-handle {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  color: #98a2b3;
  cursor: grab;

  &:active {
    cursor: grabbing;
  }

  &.disabled {
    color: #d0d5dd;
    cursor: default;
  }
}

.selected-field-row-ghost {
  border: 1px dashed #91caff;
  background: #f0f7ff;
  opacity: 0.6;
}

.selected-field-row-chosen {
  box-shadow: 0 3px 10px rgb(16 24 40 / 12%);
}

.selected-field-row-drag {
  opacity: 0.92;
}

.copy-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
</style>
