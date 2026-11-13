<!-- StatusTab manages work-item statuses and their workflow settings. -->
<template>
  <div class="settings-panel status-panel">
    <a-card
      :title="$t('settings_statusList')"
      size="small"
      class="settings-card"
    >
      <template #extra v-if="canManage">
        <a-space :size="4">
          <a-popconfirm
            :title="$t('settings_resetConfirm')"
            :ok-text="$t('common_confirm')"
            :cancel-text="$t('common_cancel')"
            @confirm="resetToDefault"
          >
            <a-button
              type="default"
              size="small"
              :loading="resetLoading"
              :disabled="saving"
              >{{ $t("settings_restoreDefault") }}</a-button
            >
          </a-popconfirm>
          <a-button type="primary" size="small" @click="openEditor(null)">
            <PlusOutlined /> {{ $t("settings_addStatus") }}
          </a-button>
        </a-space>
      </template>
      <a-table
        :data-source="statuses"
        :columns="statusColumns"
        :pagination="false"
        row-key="id"
        size="middle"
        :locale="{ emptyText: $t('settings_noStatus') }"
      >
        <template #headerCell="{ column }">
          <span v-if="column.key === 'initial'" class="th-with-hint">
            {{ $t("settings_initialStatus") }}
            <a-tooltip :title="$t('settings_initial_status_hint')">
              <QuestionCircleOutlined />
            </a-tooltip>
          </span>
          <span v-else-if="column.key === 'completed'" class="th-with-hint">
            {{ $t("settings_completedStatus") }}
            <a-tooltip :title="$t('settings_completed_status_hint')"
              ><QuestionCircleOutlined
            /></a-tooltip>
          </span>
          <span v-else-if="column.key === 'final'" class="th-with-hint">
            {{
              $t(
                typeKey === "planning"
                  ? "settings_endStatus"
                  : "settings_closedStatus",
              )
            }}
            <a-tooltip :title="endStatusHint">
              <QuestionCircleOutlined />
            </a-tooltip>
          </span>
        </template>
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <span
              class="enum-tag"
              :style="{ color: record.color, borderColor: record.color }"
              >{{ displayStatusName(record.name) }}</span
            >
          </template>
          <template v-else-if="column.key === 'initial'">
            <a-radio
              :checked="Number(initialStatus) === Number(record.id)"
              :disabled="!canManage || saving || resetLoading"
              @change="setInitialStatus(record.id)"
            />
          </template>
          <template v-else-if="column.key === 'completed'">
            <a-checkbox
              :checked="completedStatusSet.has(Number(record.id))"
              :disabled="
                !canManage ||
                saving ||
                resetLoading ||
                endStatusSet.has(Number(record.id))
              "
              :aria-label="$t('settings_completedStatus')"
              @change="
                (event) =>
                  toggleCompletedStatus(record.id, event.target.checked)
              "
            />
          </template>
          <template v-else-if="column.key === 'final'">
            <a-checkbox
              :checked="endStatusSet.has(Number(record.id))"
              :disabled="
                !canManage ||
                saving ||
                completedStatusSet.has(Number(record.id))
              "
              :aria-label="
                $t(
                  typeKey === 'planning'
                    ? 'settings_endStatus'
                    : 'settings_closedStatus',
                )
              "
              @change="
                (event) => toggleEndStatus(record.id, event.target.checked)
              "
            />
          </template>
          <template v-else-if="column.key === 'action'">
            <a-space v-if="canManage" :size="4">
              <a-button type="text" size="small" @click="openEditor(record)">
                <EditOutlined />
              </a-button>
              <a-popconfirm
                :title="$t('settings_delete_status_confirm')"
                :ok-text="$t('common_confirm')"
                :cancel-text="$t('common_cancel')"
                @confirm="deleteItem(record)"
              >
                <a-button type="text" size="small" danger>
                  <DeleteOutlined />
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </a-card>

    <AdvancedStatusTransitionCard
      v-if="typeKey !== 'planning'"
      :ws-id="wsId"
      :type-key="typeKey"
      :can-manage="canManage"
      :reload-key="transitionReloadKey"
    />

    <div
      v-if="typeKey === 'requirement' && !slaActivated"
      ref="agingAnchorRef"
      class="aging-control-anchor"
    >
      <TimeControlCard
        :ws-id="wsId"
        :type-key="typeKey"
        :can-manage="canManage"
      />
    </div>

    <a-modal
      v-model:open="modalOpen"
      :title="
        editingItem !== null
          ? $t('settings_editStatus')
          : $t('settings_addStatusTitle')
      "
      @ok="handleSave"
      :ok-text="$t('common_save')"
      :cancel-text="$t('common_cancel')"
      width="440px"
      :destroy-on-close="true"
    >
      <a-form layout="vertical" style="margin-top: 12px">
        <a-form-item :label="$t('settings_statusName')" required>
          <a-input
            v-model:value="editLabel"
            :placeholder="$t('settings_statusNamePlaceholder')"
            :maxlength="10"
          />
        </a-form-item>
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
import { computed, ref, onMounted, watch, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";
import { message } from "ant-design-vue";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  QuestionCircleOutlined,
} from "@ant-design/icons-vue";
import {
  listStatuses,
  createStatus,
  updateStatus,
  deleteStatus,
  getWorkspaceSettings,
  updateWorkspaceSettings,
  resetStatuses,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { typeSettingPermission } from "@/utils/typePermission";
import { AGING_CONTROL_ANCHOR } from "@/constants/statusTransition";
import AdvancedStatusTransitionCard from "./AdvancedStatusTransitionCard.vue";
import TimeControlCard from "./TimeControlCard.vue";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
});

const { t: $t } = useI18n();
const route = useRoute();
const authStore = useAuthStore();
const agingAnchorRef = ref(null);
const presetColors = [
  "#8c8c8c",
  "#0d68ff",
  "#fa8c16",
  "#52c41a",
  "#ff4d4f",
  "#722ed1",
  "#13c2c2",
  "#eb2f96",
  "#2f54eb",
  "#fa541c",
  "#faad14",
];

const statuses = ref([]);
const initialStatus = ref("");
const slaActivated = ref(false);
const endStatuses = ref([]);
const completedStatuses = ref([]);
const saving = ref(false);
let savedSettings = null;
function rememberSettings() {
  savedSettings = {
    initial: initialStatus.value,
    closed: [...endStatuses.value],
    completed: [...completedStatuses.value],
  };
}
const completedStatusSet = computed(
  () => new Set(completedStatuses.value.map(Number)),
);
const resetLoading = ref(false);
const modalOpen = ref(false);
const transitionReloadKey = ref(0);
const editingItem = ref(null);
const editLabel = ref("");
const editColor = ref("#0d68ff");
const canManage = computed(() =>
  authStore.hasPermission(typeSettingPermission(props.typeKey, "status")),
);
const endStatusSet = computed(
  () => new Set((endStatuses.value || []).map(Number)),
);
const endStatusHint = computed(() =>
  props.typeKey !== "planning"
    ? $t("settings_closed_status_hint")
    : $t("settings_end_status_hint", {
        type: $t(
          props.typeKey === "defect"
            ? "settings_defect"
            : "settings_requirement",
        ),
      }),
);
const statusColumns = computed(() => [
  { key: "name", title: $t("settings_statusName"), dataIndex: "name" },
  {
    key: "initial",
    title: $t("settings_initialStatus"),
    width: 110,
    align: "center",
  },
  ...(props.typeKey === "planning"
    ? []
    : [
        {
          key: "completed",
          title: $t("settings_completedStatus"),
          width: 110,
          align: "center",
        },
      ]),
  {
    key: "final",
    title: $t(
      props.typeKey === "planning"
        ? "settings_endStatus"
        : "settings_closedStatus",
    ),
    width: 110,
    align: "center",
  },
  { key: "action", title: $t("settings_action"), width: 90, align: "center" },
]);

function isLight(h) {
  const r = parseInt(h.slice(1, 3), 16),
    g = parseInt(h.slice(3, 5), 16),
    b = parseInt(h.slice(5, 7), 16);
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.6;
}
function displayStatusName(name) {
  return name?.startsWith("workitem.")
    ? $t(name.replaceAll(".", "_"))
    : name || "";
}
async function fetchStatuses() {
  try {
    statuses.value = (await listStatuses(props.wsId, props.typeKey)) || [];
  } catch (e) {
    /* Network errors are handled centrally by the interceptor. */
  }
}

async function fetchSettings() {
  try {
    const data = (await getWorkspaceSettings(props.wsId)) || {};
    slaActivated.value = Number(data.sla_activated_at || 0) > 0;
    initialStatus.value = data[`${props.typeKey}_initial_status_id`] || "";
    endStatuses.value =
      data[
        `${props.typeKey}_${props.typeKey === "planning" ? "final" : "closed"}_status_ids`
      ] || [];
    completedStatuses.value =
      data[`${props.typeKey}_completed_status_ids`] || [];
  } catch (e) {
    /* Network errors are handled centrally by the interceptor. */
  }
}

async function saveSettings() {
  if (!canManage.value || saving.value) return;
  saving.value = true;
  try {
    const payload = {
      [`${props.typeKey}_initial_status_id`]: initialStatus.value || 0,
      [`${props.typeKey}_${props.typeKey === "planning" ? "final" : "closed"}_status_ids`]:
        endStatuses.value,
      ...(props.typeKey === "planning"
        ? {}
        : {
            [`${props.typeKey}_completed_status_ids`]: completedStatuses.value,
          }),
    };
    await updateWorkspaceSettings(props.wsId, payload);
    rememberSettings();
    message.success($t("common_saved"));
  } catch (e) {
    if (savedSettings) {
      initialStatus.value = savedSettings.initial;
      endStatuses.value = [...savedSettings.closed];
      completedStatuses.value = [...savedSettings.completed];
    }
  } finally {
    saving.value = false;
  }
}

async function setInitialStatus(id) {
  if (saving.value) return;
  initialStatus.value = id;
  await saveSettings();
}

async function toggleEndStatus(id, checked) {
  if (saving.value || (checked && completedStatusSet.value.has(Number(id))))
    return;
  const next = new Set(endStatusSet.value);
  if (checked) next.add(Number(id));
  else next.delete(Number(id));
  endStatuses.value = Array.from(next);
  await saveSettings();
}

async function toggleCompletedStatus(id, checked) {
  if (saving.value || (checked && endStatusSet.value.has(Number(id)))) return;
  const next = new Set(completedStatusSet.value);
  if (checked) next.add(Number(id));
  else next.delete(Number(id));
  completedStatuses.value = Array.from(next);
  await saveSettings();
}

async function resetToDefault() {
  if (!canManage.value || saving.value || resetLoading.value) return;
  resetLoading.value = true;
  try {
    await resetStatuses(props.wsId, props.typeKey);
    message.success($t("settings_restored"));
    await fetchStatuses();
    await fetchSettings();
    transitionReloadKey.value += 1;
  } catch (e) {
    /* Network errors are handled centrally by the interceptor. */
  } finally {
    resetLoading.value = false;
  }
}

function openEditor(item) {
  if (!canManage.value) return;
  if (item) {
    editLabel.value = item.name || "";
    editColor.value = item.color || "#0d68ff";
    editingItem.value = item;
  } else {
    editLabel.value = "";
    editColor.value = "#0d68ff";
    editingItem.value = null;
  }
  modalOpen.value = true;
}
async function handleSave() {
  if (!canManage.value) return;
  if (!editLabel.value) {
    message.warning($t("settings_nameRequired"));
    return;
  }
  try {
    if (editingItem.value) {
      await updateStatus(props.wsId, props.typeKey, editingItem.value.id, {
        name: editLabel.value,
        color: editColor.value,
        sort_order: editingItem.value.sort_order,
      });
      message.success($t("settings_updated"));
    } else {
      await createStatus(props.wsId, props.typeKey, {
        name: editLabel.value,
        color: editColor.value,
        sort_order: statuses.value.length + 1,
      });
      message.success($t("settings_added"));
    }
    modalOpen.value = false;
    await fetchStatuses();
  } catch (e) {}
}
async function deleteItem(item) {
  if (!canManage.value) return;
  try {
    await deleteStatus(props.wsId, props.typeKey, item.id);
    message.success($t("settings_deleted"));
    await fetchStatuses();
    await fetchSettings();
  } catch (e) {}
}

let anchorScrollFrame = 0;

// The requirement aging tab deep-links here with `?anchor=`; the status list above and the
// transition card load at the same time as this one, so scroll again on the next frame to
// settle on the final position of the aging card.
function scrollToAgingAnchor() {
  if (route.query.anchor !== AGING_CONTROL_ANCHOR || !agingAnchorRef.value)
    return;
  cancelAnimationFrame(anchorScrollFrame);
  const scroll = () => agingAnchorRef.value?.scrollIntoView({ block: "start" });
  nextTick(() => {
    scroll();
    anchorScrollFrame = requestAnimationFrame(() => {
      anchorScrollFrame = 0;
      scroll();
    });
  });
}

async function loadPanel() {
  await Promise.all([fetchStatuses(), fetchSettings()]);
  scrollToAgingAnchor();
}

onMounted(loadPanel);
watch(() => [props.wsId, props.typeKey], loadPanel);
</script>

<style scoped>
.settings-panel {
  width: min(920px, 100%);
  margin: 0 auto;
}
/* Keeps the aging card clear of the sticky sub-tab bar when a deep link scrolls it into view. */
.aging-control-anchor {
  scroll-margin-top: 44px;
}
.settings-panel :deep(.settings-card) {
  margin-bottom: 24px;
  overflow: hidden;
  border-color: #eef1f6;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(24, 43, 80, 0.04);
}
.settings-panel :deep(.settings-card .ant-card-head) {
  min-height: 54px;
  padding: 0 16px;
  background: #f5f7fa;
  border-color: #eef1f6;
  font-weight: 600;
}
.settings-panel :deep(.settings-card .ant-card-head-title) {
  padding: 15px 0;
  font-size: 14px;
}
.settings-panel :deep(.settings-card .ant-card-body) {
  padding: 16px;
}
.th-with-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.enum-tag {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 10px;
  border-radius: 12px;
  font-size: 12px;
  line-height: 22px;
  white-space: nowrap;
  background: transparent;
  border: 1px solid;
  font-weight: 400;
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
