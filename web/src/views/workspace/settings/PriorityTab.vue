<template>
  <div class="settings-panel priority-panel">
    <a-card
      :title="$t(severity ? 'workspace_severity' : 'settings_priorityList')"
      size="small"
      class="settings-card"
    >
      <template #extra v-if="canManage">
        <a-space :size="4">
          <a-popconfirm
            :title="
              $t(
                severity
                  ? 'settings_resetSeverityConfirm'
                  : 'settings_resetPriorityConfirm',
              )
            "
            :ok-text="$t('common_confirm')"
            :cancel-text="$t('common_cancel')"
            @confirm="resetToDefault"
          >
            <a-button
              type="default"
              size="small"
              :loading="resetLoading"
              :disabled="defaultSaving || optionMutationPending"
              >{{ $t("settings_restoreDefault") }}</a-button
            >
          </a-popconfirm>
          <a-button
            type="primary"
            size="small"
            :disabled="optionMutationPending"
            @click="openEditor(null)"
            ><PlusOutlined />
            {{
              $t(severity ? "settings_addSeverity" : "settings_addStatus")
            }}</a-button
          >
        </a-space>
      </template>
      <div ref="optionList" class="enum-list" :aria-busy="ordering">
        <div
          v-for="item in priorities"
          :key="item.id"
          class="enum-item workflow-option-row"
        >
          <span>
            <WorkflowOptionHandle
              v-if="canReorder && priorities.length > 1"
              :label="item.label"
              :disabled="ordering || optionOrderDisabled"
              @move="moveOption(item.id, $event)"
            />
            <span
              class="enum-tag"
              :style="{ color: item.color, borderColor: item.color }"
              >{{ item.label }}</span
            >
          </span>
          <a-space v-if="canManage" :size="8" class="row-actions">
            <a-button
              type="text"
              size="small"
              :disabled="optionMutationPending"
              @click="openEditor(item)"
              ><EditOutlined
            /></a-button>
            <a-tooltip
              v-if="item.is_builtin"
              :title="$t('settings_builtin_delete_disabled')"
              :trigger="['hover', 'focus']"
            >
              <span
                tabindex="0"
                :aria-label="$t('settings_builtin_delete_disabled')"
                style="display: inline-block"
              >
                <a-button
                  type="text"
                  size="small"
                  disabled
                  style="pointer-events: none"
                  ><DeleteOutlined
                /></a-button>
              </span>
            </a-tooltip>
            <a-button
              v-else
              type="text"
              size="small"
              :disabled="optionMutationPending"
              @click="deleteItem(item)"
              ><DeleteOutlined
            /></a-button>
          </a-space>
        </div>
        <span v-if="!priorities.length" style="color: #999">{{
          $t(severity ? "settings_noSeverity" : "settings_noPriority")
        }}</span>
      </div>
    </a-card>

    <a-card
      :title="
        $t(severity ? 'settings_defaultSeverity' : 'settings_defaultPriority')
      "
      size="small"
      class="settings-card"
    >
      <a-radio-group
        :value="defaultPriorityId"
        class="default-priority-options"
        :disabled="!canManage || defaultSaving || optionMutationPending"
        @change="saveDefaultPriority"
      >
        <a-radio :value="0">{{ $t("settings_none") }}</a-radio>
        <a-radio
          v-for="item in priorities"
          :key="item.id"
          :value="Number(item.id)"
        >
          <span
            class="enum-tag"
            :style="{ color: item.color, borderColor: item.color }"
            >{{ item.label }}</span
          >
        </a-radio>
      </a-radio-group>
    </a-card>

    <a-modal
      v-model:open="modalOpen"
      :title="
        editingItem !== null
          ? $t(severity ? 'settings_editSeverity' : 'settings_editPriority')
          : $t(severity ? 'settings_addSeverity' : 'settings_addPriority')
      "
      @ok="handleSave"
      :ok-text="$t('common_save')"
      :cancel-text="$t('common_cancel')"
      width="440px"
      :destroy-on-close="true"
    >
      <a-form layout="vertical" style="margin-top: 12px">
        <a-form-item
          :label="
            $t(severity ? 'settings_severityName' : 'settings_priorityName')
          "
          required
          ><a-input
            v-model:value="editLabel"
            :placeholder="
              $t(
                severity
                  ? 'settings_severityNamePlaceholder'
                  : 'settings_priorityNamePlaceholder',
              )
            "
            :maxlength="10"
        /></a-form-item>
        <a-form-item :label="$t('settings_color')">
          <div style="display: flex; gap: 8px; flex-wrap: wrap">
            <div
              v-for="c in presetColors"
              :key="c"
              class="color-swatch"
              :class="{ sel: editColor === c }"
              :style="{ backgroundColor: c }"
              @click="editColor = c"
            >
              <span
                v-if="editColor === c"
                :style="{ color: isLight(c) ? '#333' : '#fff' }"
                >✓</span
              >
            </div>
          </div>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
} from "@ant-design/icons-vue";
import {
  listPriorities,
  createPriority,
  updatePriority,
  updateDefaultPriority,
  deletePriority,
  resetPriorities,
  reorderPriorities,
  listSeverities,
  createSeverity,
  updateSeverity,
  updateDefaultSeverity,
  deleteSeverity,
  resetSeverities,
  reorderSeverities,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { typeSettingPermission } from "@/utils/typePermission";
import { useWorkflowOptionOrder } from "@/composables/useWorkflowOptionOrder";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";
import WorkflowOptionHandle from "./WorkflowOptionHandle.vue";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  severity: Boolean,
  typeKey: { type: String, default: "requirement" },
});

const { t: $t } = useI18n();
const authStore = useAuthStore();
const priorityColors = [
  "#ff4d4f",
  "#fa8c16",
  "#0d68ff",
  "#8c8c8c",
  "#52c41a",
  "#722ed1",
  "#eb2f96",
  "#faad14",
  "#13c2c2",
  "#a0d911",
  "#fa541c",
];

const presetColors = computed(() =>
  props.severity
    ? ["#e54545", "#ff8c00", "#2d8cf0", "#808695", ...priorityColors]
    : priorityColors,
);
const priorities = ref([]);
const modalOpen = ref(false);
const editingItem = ref(null);
const editLabel = ref("");
const editColor = ref("#fa8c16");
const resetLoading = ref(false);
const mutatingOption = ref(false);
const defaultPriorityId = ref(0);
const defaultSaving = ref(false);
const canManage = computed(() =>
  authStore.hasPermission(typeSettingPermission(props.typeKey, "priority")),
);
const optionList = ref(null);
const { workspaceType } = useWorkspaceApplications(computed(() => props.wsId));
const canReorder = computed(
  () =>
    canManage.value &&
    ((!props.severity && props.typeKey === "requirement") ||
      (!!workspaceType.value && workspaceType.value !== "issue")),
);
const optionOrderDisabled = computed(
  () =>
    !canReorder.value ||
    resetLoading.value ||
    mutatingOption.value ||
    defaultSaving.value ||
    modalOpen.value,
);
const { ordering, moveOption } = useWorkflowOptionOrder(
  priorities,
  () => optionList.value,
  optionOrderDisabled,
  saveOptionOrder,
  {
    animation: 180,
    easing: "cubic-bezier(0.22, 0.61, 0.36, 1)",
    forceFallback: true,
    fallbackOnBody: true,
    fallbackTolerance: 3,
    ghostClass: "enum-item-placeholder",
    fallbackClass: "enum-item-drag-preview",
  },
);
const optionMutationPending = computed(
  () => ordering.value || mutatingOption.value || resetLoading.value,
);

function isLight(h) {
  const r = parseInt(h.slice(1, 3), 16),
    g = parseInt(h.slice(3, 5), 16),
    b = parseInt(h.slice(5, 7), 16);
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.6;
}

async function fetchPriorities() {
  const wsId = props.wsId;
  const typeKey = props.typeKey;
  try {
    const res = await (props.severity
      ? listSeverities(wsId)
      : listPriorities(wsId, typeKey));
    if (wsId !== props.wsId || typeKey !== props.typeKey) return;
    priorities.value = res || [];
    defaultPriorityId.value =
      Number(
        priorities.value.find((item) => Number(item.is_default) === 1)?.id,
      ) || 0;
  } catch (e) {
    /* Business and network errors are handled centrally by the interceptor. */
  }
}

async function saveOptionOrder(ids) {
  const { wsId, typeKey, severity } = props;
  const current = () =>
    wsId === props.wsId &&
    typeKey === props.typeKey &&
    severity === props.severity;
  try {
    if (severity) {
      await reorderSeverities(wsId, ids);
    } else {
      await reorderPriorities(wsId, typeKey, ids);
    }
    message.success($t("common_saveSuccess"));
    if (wsId === props.wsId) authStore.clearProjectMetadata();
    return current();
  } catch (error) {
    if (current()) {
      if (error.response?.status === 409) {
        message.warning(
          error.response.data?.message || $t("common_requestFailed"),
        );
      }
      await fetchPriorities();
    }
    return false;
  }
}

async function saveDefaultPriority(event) {
  if (!canManage.value || defaultSaving.value || ordering.value) return;
  const wsId = props.wsId;
  const typeKey = props.typeKey;
  const previous = defaultPriorityId.value;
  const next = Number(event.target.value) || 0;
  defaultPriorityId.value = next;
  defaultSaving.value = true;
  try {
    if (props.severity) {
      await updateDefaultSeverity(wsId, next);
      authStore.clearProjectMetadata();
    } else {
      await updateDefaultPriority(wsId, typeKey, next);
    }
    if (wsId !== props.wsId || typeKey !== props.typeKey) return;
    priorities.value = priorities.value.map((item) => ({
      ...item,
      is_default: Number(item.id) === next ? 1 : 0,
    }));
    message.success($t("common_saved"));
  } catch (e) {
    if (wsId === props.wsId && typeKey === props.typeKey)
      defaultPriorityId.value = previous;
  } finally {
    defaultSaving.value = false;
  }
}

function openEditor(item) {
  if (!canManage.value || optionMutationPending.value) return;
  if (item) {
    editLabel.value = item.label || "";
    editColor.value = item.color || "#fa8c16";
    editingItem.value = item;
  } else {
    editLabel.value = "";
    editColor.value = "#fa8c16";
    editingItem.value = null;
  }
  modalOpen.value = true;
}
async function handleSave() {
  if (!canManage.value || optionMutationPending.value) return;
  if (!editLabel.value.trim()) {
    message.warning($t("settings_nameRequired"));
    return;
  }
  mutatingOption.value = true;
  try {
    if (editingItem.value) {
      const data = {
        name: editLabel.value,
        color: editColor.value,
      };
      if (props.severity) {
        await updateSeverity(props.wsId, editingItem.value.id, data);
      } else {
        await updatePriority(
          props.wsId,
          props.typeKey,
          editingItem.value.id,
          data,
        );
      }
      message.success($t("settings_updated"));
    } else {
      const data = {
        name: editLabel.value,
        color: editColor.value,
      };
      if (props.severity) {
        await createSeverity(props.wsId, data);
      } else {
        await createPriority(props.wsId, props.typeKey, data);
      }
      message.success($t("settings_added"));
    }
    if (props.severity) authStore.clearProjectMetadata();
    modalOpen.value = false;
    await fetchPriorities();
  } catch (e) {
  } finally {
    mutatingOption.value = false;
  }
}
async function deleteItem(item) {
  if (!canManage.value || optionMutationPending.value) return;
  mutatingOption.value = true;
  try {
    if (props.severity) {
      await deleteSeverity(props.wsId, item.id);
      authStore.clearProjectMetadata();
    } else {
      await deletePriority(props.wsId, props.typeKey, item.id);
    }
    message.success($t("settings_deleted"));
    await fetchPriorities();
  } catch (e) {
  } finally {
    mutatingOption.value = false;
  }
}

// Reset the current type's priorities to built-in defaults.
async function resetToDefault() {
  if (!canManage.value || defaultSaving.value || optionMutationPending.value)
    return;
  resetLoading.value = true;
  try {
    if (props.severity) {
      await resetSeverities(props.wsId);
      authStore.clearProjectMetadata();
    } else {
      await resetPriorities(props.wsId, props.typeKey);
    }
    message.success($t("settings_restored"));
    await fetchPriorities();
  } catch (e) {
  } finally {
    resetLoading.value = false;
  }
}

onMounted(fetchPriorities);
watch(
  () => [props.wsId, props.typeKey],
  () => {
    priorities.value = [];
    defaultPriorityId.value = 0;
    fetchPriorities();
  },
);
</script>

<style scoped>
.settings-panel {
  width: min(920px, 100%);
  margin: 0 auto;
}
.settings-card {
  margin-bottom: 24px;
  overflow: hidden;
  border-color: #eef1f6;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(24, 43, 80, 0.04);
}
.settings-card :deep(.ant-card-head) {
  min-height: 54px;
  padding: 0 16px;
  background: #f5f7fa;
  border-color: #eef1f6;
  font-weight: 600;
}
.settings-card :deep(.ant-card-head-title) {
  padding: 15px 0;
  font-size: 14px;
}
.settings-card :deep(.ant-card-body) {
  padding: 16px;
}
.enum-list {
  display: flex;
  flex-direction: column;
}
.enum-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 53px;
  padding: 0 4px;
  border-bottom: 1px solid #eef1f6;
}
.enum-item:last-child {
  border-bottom: 0;
}
.enum-item-placeholder {
  background: #f0f7ff;
}
.enum-item-placeholder > * {
  visibility: hidden;
}
.enum-item-drag-preview {
  background: #fff;
  border-radius: 6px;
  box-shadow: 0 3px 10px rgb(16 24 40 / 12%);
  /* Sortable applies its preview opacity inline. */
  opacity: 1 !important;
}
.enum-tag {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 10px;
  border: 1px solid;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
  line-height: 22px;
  white-space: nowrap;
  background: transparent;
}
.default-priority-options {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 20px 28px;
  min-height: 28px;
}
.default-priority-options :deep(.ant-radio-wrapper) {
  margin-inline-end: 0;
}
.row-actions :deep(.ant-btn) {
  color: #94a3b8;
  padding: 0 6px;
}
.row-actions :deep(.ant-btn:hover) {
  color: #3157e2;
  background: transparent;
}
.color-swatch {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  cursor: pointer;
  border: 2px solid transparent;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: border 0.15s;
  font-size: 12px;
  font-weight: 700;
}
.color-swatch.sel {
  border: 3px solid #333;
}
</style>
