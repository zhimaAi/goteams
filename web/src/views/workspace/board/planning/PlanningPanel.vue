<template>
  <div class="planning-panel">
    <div class="planning-toolbar">
      <button
        v-if="canEdit"
        type="button"
        class="create-toggle"
        :aria-label="$t('planning_addRow')"
        :title="$t('planning_addRow')"
        @click="tableRef?.addRow()"
      >
        <PlusOutlined />
      </button>
      <PlanningGroupTabs
        :ws-id="wsId"
        :groups="groups"
        v-model:active-group-id="activeGroupId"
        :can-edit="canEdit"
        @refresh="reloadGroups"
      />
      <a-dropdown trigger="click" placement="bottomRight">
        <a-button size="small" class="archived-toggle">
          {{ $t("planning_archivedState") }}: {{ archivedLabel }}
        </a-button>
        <template #overlay>
          <a-menu
            :selected-keys="[archivedFilter]"
            @click="({ key }) => (archivedFilter = key)"
          >
            <a-menu-item v-for="option in archivedOptions" :key="option.value">
              {{ option.label }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>
      <a-button
        size="small"
        :type="showFilter || hasActiveFilters ? 'primary' : 'default'"
        class="filter-toggle"
        @click="showFilter = !showFilter"
      >
        <FilterOutlined /> {{ $t("planning_filter") }}
      </a-button>
      <a-dropdown trigger="click" placement="bottomRight">
        <a-button size="small" class="more-toggle">
          <MoreOutlined /> {{ $t("planning_more") }}
        </a-button>
        <template #overlay>
          <a-menu @click="({ key }) => handleMoreMenu(key)">
            <a-menu-item key="recycle">
              <DeleteOutlined /> {{ $t("planning_recycleBin") }}
            </a-menu-item>
            <a-menu-item v-if="canEdit" key="import">
              <ImportOutlined /> {{ $t("planning_import") }}
            </a-menu-item>
            <a-menu-item key="export">
              <ExportOutlined /> {{ $t("planning_export") }}
            </a-menu-item>
            <a-menu-item v-if="canEdit" key="batch">
              <PlusOutlined /> {{ $t("planning_batchAdd") }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>
    </div>

    <div v-show="showFilter" class="planning-filters">
      <a-input
        v-model:value="filterForm.title"
        size="small"
        allow-clear
        class="filter-input"
        :placeholder="$t('planning_filterTitle')"
      />
      <a-select
        v-if="fieldEnabled('priority')"
        v-model:value="filterForm.priority"
        size="small"
        mode="multiple"
        allow-clear
        option-filter-prop="label"
        :max-tag-count="1"
        class="filter-select"
        popup-class-name="planning-cell-dropdown"
        :placeholder="$t('workspace_priority')"
        :options="priorityOptions"
      />
      <a-select
        v-if="fieldEnabled('related_items')"
        v-model:value="filterForm.has_relation"
        size="small"
        allow-clear
        class="filter-select"
        popup-class-name="planning-cell-dropdown"
        :placeholder="$t('planning_filterHasRelation')"
        :options="relationOptions"
      />
      <a-select
        v-if="fieldEnabled('status')"
        v-model:value="filterForm.status_id"
        size="small"
        mode="multiple"
        allow-clear
        option-filter-prop="label"
        :max-tag-count="1"
        class="filter-select"
        popup-class-name="planning-cell-dropdown"
        :placeholder="$t('workspace_field_status')"
        :options="statusOptions"
      />
      <a-select
        v-if="fieldEnabled('assignee')"
        v-model:value="filterForm.assignee_id"
        size="small"
        allow-clear
        class="filter-select"
        popup-class-name="planning-cell-dropdown"
        :placeholder="$t('workspace_field_assignee')"
        :options="memberOptions"
      />
    </div>

    <PlanningTable
      v-if="activeGroupId"
      :key="activeGroupId"
      ref="tableRef"
      :ws-id="wsId"
      :group-id="activeGroupId"
      :statuses="statuses"
      :priorities="priorities"
      :members="members"
      :custom-field-members="customFieldMembers"
      :fields="listFields"
      :filters="appliedFilters"
      :can-edit="canEdit"
      :auto-add-row="
        pendingNewGroupId !== null && pendingNewGroupId === activeGroupId
      "
      @open-settings="settingsOpen = true"
      @open-relations="openRelations"
      @view-relations="viewRelations"
      @create-requirement="goCreateRequirement"
      @batch-add="batchAddOpen = true"
    />
    <a-empty v-else :description="$t('common_noData')" />

    <ListSettingsModal
      v-model:open="settingsOpen"
      :user-id="authStore.userInfo?.id"
      :ws-id="wsId"
      type-key="planning"
      order-scope="project"
      hide-preview-mode
      show-row-index
      :fields="systemFields"
      :can-manage-order="canEdit"
      @change="applyPreferences"
      @order-changed="reloadMetadata"
    />

    <RequirementRelationPicker
      v-if="relationTarget"
      :key="`${relationTarget.id}-${relationMode}`"
      v-model="relationValue"
      :ws-id="wsId"
      :types="relationTypes"
      multiple
      auto-open
      :planning-id="relationMode === 'linked' ? relationTarget.id : 0"
      :readonly="relationMode === 'linked' && !canEdit"
      @change="saveRelations"
      @close="relationTarget = null"
    />

    <PlanningRecycleBinModal
      v-model:open="recycleOpen"
      :ws-id="wsId"
      :can-edit="canEdit"
      @changed="handleRecycleChanged"
    />

    <PlanningBatchAddModal
      v-model:open="batchAddOpen"
      :ws-id="wsId"
      :group-id="activeGroupId"
      @created="tableRef?.refresh()"
    />

    <ImportWorkItemsModal
      v-model:open="importOpen"
      :ws-id="wsId"
      type-key="planning"
      :template-api="downloadPlanningTemplate"
      :preview-api="previewPlanningFile"
      :import-api="importPlanningFile"
      @imported="tableRef?.refresh()"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";
import {
  DeleteOutlined,
  ExportOutlined,
  FilterOutlined,
  ImportOutlined,
  MoreOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import PlanningGroupTabs from "./PlanningGroupTabs.vue";
import PlanningTable from "./PlanningTable.vue";
import PlanningRecycleBinModal from "./PlanningRecycleBinModal.vue";
import PlanningBatchAddModal from "./PlanningBatchAddModal.vue";
import ListSettingsModal from "@/components/work-item/ListSettingsModal.vue";
import ImportWorkItemsModal from "@/components/work-item/ImportWorkItemsModal.vue";
import RequirementRelationPicker from "@/components/common/RequirementRelationPicker.vue";
import { useAuthStore } from "@/stores/auth";
import { getWorkspaceMetadata } from "@/api/workspace";
import {
  downloadPlanningImportTemplate,
  exportPlannings,
  importPlannings,
  listPlanningGroups,
  previewPlanningImport,
  savePlanningRelations,
} from "@/api/planning";
import { sortFieldsByOrder } from "@/utils/fieldOrder";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
});
// Related work items can only pick types the project exposes: defects disappear with the
// defect app switch, so the picker must not offer them once the app is off.
const { defectAppEnabled } = useWorkspaceApplications(
  computed(() => props.wsId),
);
const relationTypes = computed(() =>
  defectAppEnabled.value ? ["requirement", "defect"] : ["requirement"],
);
const { t: $t } = useI18n();
const router = useRouter();
const authStore = useAuthStore();

const groups = ref([]);
const activeGroupId = ref(null);
const statuses = ref([]);
const priorities = ref([]);
const members = ref([]);
const customFieldMembers = ref([]);
const fieldConfigs = ref([]);
const customFields = ref([]);
const listFields = ref([]);
const showFilter = ref(false);
// Archive state is a standalone toolbar switch: 0 unarchived (default), 1 archived, all both.
const archivedFilter = ref("0");
const settingsOpen = ref(false);
const recycleOpen = ref(false);
const batchAddOpen = ref(false);
const importOpen = ref(false);
const tableRef = ref(null);
const relationTarget = ref(null);
const relationValue = ref([]);
// picker has two modes: pick = full selector (🔀), linked = list only related (🔗 count).
const relationMode = ref("pick");

const canEdit = computed(() => authStore.hasPermission("planning:edit"));

const filterForm = reactive({
  title: undefined,
  priority: [],
  status_id: [],
  assignee_id: undefined,
  has_relation: undefined,
});

// Keep applying filled-in filters even after the filter bar is collapsed.
const appliedFilters = computed(() => {
  const result = { archived: archivedFilter.value };
  for (const [key, value] of Object.entries(filterForm)) {
    if (Array.isArray(value)) {
      if (value.length) result[key] = value.join(",");
      continue;
    }
    if (value !== undefined && value !== null && value !== "")
      result[key] = value;
  }
  return result;
});
const hasActiveFilters = computed(
  () => Object.keys(appliedFilters.value).length > 1,
);

const statusOptions = computed(() =>
  statuses.value.map((item) => ({ value: item.id, label: item.name })),
);
const priorityOptions = computed(() =>
  priorities.value.map((item) => ({ value: item.id, label: item.label })),
);
const memberOptions = computed(() =>
  members.value.map((item) => ({
    value: Number(item.user_id),
    label: item.user_name || item.user_username,
  })),
);
const relationOptions = computed(() => [
  { value: "true", label: $t("planning_filterYes") },
  { value: "false", label: $t("planning_filterNo") },
]);
const archivedOptions = computed(() => [
  { value: "0", label: $t("planning_archivedUnarchived") },
  { value: "all", label: $t("planning_archivedAll") },
  { value: "1", label: $t("planning_archivedOnly") },
]);
const archivedLabel = computed(
  () =>
    archivedOptions.value.find((item) => item.value === archivedFilter.value)
      ?.label || "",
);

// The system-field list for the list-settings dialog (built-in and custom fields mixed by project-level sort_order).
const systemFields = computed(() => {
  // The index column already has a fixed row (cannot be closed) in the list-settings dialog, so it is not listed again as a configurable field;
  // it still exists in the project settings' field management.
  const items = fieldConfigs.value
    .filter((config) => config.key !== "row_index")
    .map((config) => ({
      kind: "builtin",
      preferenceKey: `builtin:${config.key}`,
      configKey: config.key,
      label: config.label,
      enabled: Number(config.enabled) === 1,
      visible: Number(config.enabled) === 1,
      locked: Number(config.locked) === 1,
      sort_order: config.sort_order,
    }));
  const customs = customFields.value.map((field) => ({
    kind: "custom",
    preferenceKey: `custom:${field.id}`,
    customField: field,
    label: field.name,
    enabled: Number(field.is_enabled) === 1,
    visible: Number(field.is_enabled) === 1,
    sort_order: field.sort_order,
  }));
  return sortFieldsByOrder([...items, ...customs]);
});

// Fields disabled in project settings are not offered for filtering.
function fieldEnabled(key) {
  const config = fieldConfigs.value.find((item) => item.key === key);
  return !config || Number(config.enabled) === 1;
}

// Clear the selected filter values when a field is disabled, to avoid filtering by invisible conditions.
watch(fieldConfigs, () => {
  if (!fieldEnabled("priority")) filterForm.priority = [];
  if (!fieldEnabled("related_items")) filterForm.has_relation = undefined;
  if (!fieldEnabled("status")) filterForm.status_id = [];
  if (!fieldEnabled("assignee")) filterForm.assignee_id = undefined;
});

async function reloadMetadata() {
  try {
    const metadata = await getWorkspaceMetadata(props.wsId);
    const planning = metadata?.planning || {};
    statuses.value = planning.statuses || [];
    priorities.value = planning.priorities || [];
    fieldConfigs.value = planning.field_configs || [];
    customFields.value = planning.custom_fields || [];
    members.value = metadata?.members || [];
    customFieldMembers.value = metadata?.custom_field_members || [];
  } catch {
    /* handled by interceptor */
  }
}

// The target group ID to auto-add a row after creating a group (applies once).
const pendingNewGroupId = ref(null);

async function reloadGroups(preferredId, isNewGroup = false) {
  try {
    const list = await listPlanningGroups(props.wsId);
    groups.value = list || [];
    const exists = groups.value.some(
      (item) => String(item.id) === String(preferredId ?? activeGroupId.value),
    );
    if (
      preferredId &&
      groups.value.some((item) => String(item.id) === String(preferredId))
    ) {
      activeGroupId.value = preferredId;
    } else if (!exists) {
      activeGroupId.value = groups.value[0]?.id ?? null;
    }
    pendingNewGroupId.value = isNewGroup ? (preferredId ?? null) : null;
  } catch {
    /* handled by interceptor */
  }
}

// The dialog's field list already includes project-level order and personal show/hide; used directly to render the header.
function applyPreferences(preferences) {
  listFields.value = preferences?.fields || [];
}

function openRelations(record) {
  relationMode.value = "pick";
  relationTarget.value = record;
  relationValue.value = (record.relations || []).map((item) => item.id);
}

// Clicking the relation count: same dialog, but lists only work items already related to this planning;
// with edit permission, unchecking removes a single relation.
function viewRelations(record) {
  relationMode.value = "linked";
  relationTarget.value = record;
  relationValue.value = (record.relations || []).map((item) => item.id);
}

async function saveRelations(itemIds) {
  if (!relationTarget.value) return;
  try {
    await savePlanningRelations(
      props.wsId,
      relationTarget.value.id,
      itemIds || [],
    );
    relationTarget.value = null;
    tableRef.value?.refresh();
  } catch {
    /* handled by interceptor */
  }
}

// "New": carry the planning name to the create-page title and let the create API auto-relate.
function goCreateRequirement(record) {
  const query = { type: "requirement" };
  if (record?.id) query.planningId = record.id;
  // When the planning is unnamed, do not prefill the title to avoid writing the placeholder text.
  if (record?.title) query.title = record.title;
  router.push({ path: `/workspace/${props.wsId}/create`, query });
}

onMounted(async () => {
  await Promise.all([reloadMetadata(), reloadGroups()]);
});

// ---------------- "More" menu ----------------

function handleMoreMenu(key) {
  if (key === "recycle") recycleOpen.value = true;
  if (key === "import") importOpen.value = true;
  if (key === "export") exportCurrentGroup();
  if (key === "batch") batchAddOpen.value = true;
}

// Restoring can revive a deleted group, so groups and rows both need a refresh.
async function handleRecycleChanged() {
  await reloadGroups(activeGroupId.value);
  tableRef.value?.refresh();
}

function planningApiParams() {
  return { group_id: activeGroupId.value, ...appliedFilters.value };
}

function downloadPlanningTemplate() {
  return downloadPlanningImportTemplate(props.wsId);
}

function previewPlanningFile(file) {
  return previewPlanningImport(props.wsId, activeGroupId.value, file);
}

function importPlanningFile(file) {
  return importPlannings(props.wsId, activeGroupId.value, file);
}

// Export mirrors the on-screen filters so the file matches the current list.
async function exportCurrentGroup() {
  if (!activeGroupId.value) return;
  try {
    const response = await exportPlannings(props.wsId, planningApiParams());
    const disposition = response.headers?.["content-disposition"] || "";
    const match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
    const filename = match
      ? decodeURIComponent(match[1])
      : `${$t("planning_tabTitle")}.xlsx`;
    const url = URL.createObjectURL(response.data);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
    message.success($t("planning_exportSuccess"));
  } catch {
    /* handled by interceptor */
  }
}
</script>

<style scoped lang="less">
.planning-panel {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.planning-toolbar {
  position: relative;
  z-index: 5;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;

  :deep(.ant-btn) {
    height: 28px;
    padding: 0 10px;
    font-size: 12px;
  }
}
.create-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 1px solid var(--app-border);
  border-radius: 50%;
  background: var(--app-surface);
  color: #595959;
  font-size: 12px;
  cursor: pointer;
  flex-shrink: 0;

  &:hover {
    color: #1677ff;
    border-color: #1677ff;
  }
}
.filter-toggle {
  flex-shrink: 0;
}
// The archive switch is the first right-aligned control, so it carries the auto margin.
.archived-toggle {
  margin-left: auto;
  flex-shrink: 0;
  height: 28px;
  padding: 0 10px;
  border-radius: 6px;
}
.more-toggle {
  flex-shrink: 0;
}
.filter-toggle,
.more-toggle {
  height: 28px;
  padding: 0 10px;
  border-radius: 6px;
}
.planning-filters {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  margin-bottom: 8px;
  background: var(--app-surface);
  border-radius: 6px;
  border: 1px solid var(--app-border);
  flex-shrink: 0;
  flex-wrap: wrap;
  font-size: 12px;

  :deep(.ant-input),
  :deep(.ant-input-affix-wrapper),
  :deep(.ant-select),
  :deep(.ant-select-selector),
  :deep(.ant-select-selection-item),
  :deep(.ant-select-selection-placeholder) {
    font-size: 12px;
  }
  :deep(.ant-input-affix-wrapper) {
    display: flex;
    align-items: center;
    min-height: 32px;
    height: 32px;
    padding-top: 0;
    padding-bottom: 0;
  }
  :deep(.ant-input) {
    line-height: 22px;
  }
  :deep(.ant-select-selector),
  :deep(.ant-select-sm .ant-select-selector) {
    display: flex !important;
    align-items: center !important;
    min-height: 32px !important;
    height: 32px !important;
  }
  :deep(.ant-select-multiple .ant-select-selector) {
    padding-block: 0 !important;
  }
  :deep(.ant-select-selection-overflow) {
    align-items: center;
  }
  :deep(.ant-select-multiple .ant-select-selection-item) {
    display: inline-flex;
    align-items: center;
    height: 22px;
    margin-block: 0;
    line-height: 22px;
  }
  :deep(.ant-select-single .ant-select-selector .ant-select-selection-item),
  :deep(
    .ant-select-single .ant-select-selector .ant-select-selection-placeholder
  ) {
    line-height: 30px;
  }
  :deep(.ant-select-multiple .ant-select-selection-placeholder) {
    top: 50% !important;
    transform: translateY(-50%) !important;
    line-height: 22px;
  }
}
.filter-input {
  width: 140px;
}
.filter-select {
  width: 200px;
}
</style>
