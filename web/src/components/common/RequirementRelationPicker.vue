<template>
  <div v-if="!autoOpen" class="relation-control" :class="{ compact }">
    <a-button size="small" class="relation-button" @click="openPicker">
      <LinkOutlined />
      <span class="relation-button-text">{{ selectedText }}</span>
    </a-button>
    <a-tooltip v-if="modelValue" :title="clearTitle">
      <a-button
        type="text"
        size="small"
        danger
        class="unlink-button"
        @click="clearSelection"
      >
        <CloseOutlined />
      </a-button>
    </a-tooltip>
  </div>

  <a-modal
    v-model:open="open"
    :title="pickerTitle"
    :footer="multiple && !readonly ? undefined : null"
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    :width="720"
    :z-index="1100"
    destroy-on-close
    @ok="confirmMultiple"
    @cancel="handleCancel"
  >
    <div class="picker-panel">
      <div class="picker-filters">
        <a-input-search
          v-model:value="search"
          :placeholder="$t('requirement_searchRequirement')"
          allow-clear
          class="search-input"
          @search="applyFilters"
        />
        <a-select
          v-model:value="status"
          :placeholder="$t('requirement_filterStatus')"
          allow-clear
          :options="statusOptions"
          popup-class-name="relation-picker-dropdown"
          @change="applyFilters"
        />
        <a-select
          v-model:value="priority"
          :placeholder="$t('workspace_priority')"
          allow-clear
          :options="priorityOptions"
          popup-class-name="relation-picker-dropdown"
          @change="applyFilters"
        />
        <a-select
          v-model:value="assigneeId"
          :placeholder="$t('requirement_filterAssignee')"
          allow-clear
          :options="memberOptions"
          popup-class-name="relation-picker-dropdown"
          @change="applyFilters"
        />
        <a-select
          v-model:value="versionId"
          :placeholder="$t('requirement_filterVersion')"
          allow-clear
          :options="versionOptions"
          popup-class-name="relation-picker-dropdown"
          @change="applyFilters"
        />
        <a-tooltip :title="$t('common_reset')">
          <a-button class="reset-button" @click="resetFilters"
            ><ReloadOutlined
          /></a-button>
        </a-tooltip>
      </div>

      <a-spin :spinning="loading">
        <div class="requirement-list">
          <button
            v-for="item in list"
            :key="item.id"
            type="button"
            class="requirement-row"
            :class="{
              selected: String(item.id) === String(modelValue),
              'final-status-row': isFinalStatus(item),
              'readonly-row': readonly,
            }"
            @click="selectRequirement(item)"
          >
            <span class="requirement-id">#{{ item.id }}</span>
            <WorkItemTypePill :type="item.type" :ws-id="wsId" />
            <!-- Clicking the title opens the work item in a new tab instead of toggling the selection.
                 A span (not an <a>) because the row itself is a <button> and links may not nest inside it. -->
            <span
              class="requirement-title"
              :title="$t('planning_openInNewTab')"
              @click.stop="openDetail(item)"
              >{{ item.title }}</span
            >
            <StatusPill :text="item.status_name" :color="item.status_color" />
            <PriorityPill
              :priority="item.priority"
              :label="item.priority_name"
              :color="item.priority_color"
            />
            <span class="requirement-meta">{{ item.version_name || "-" }}</span>
            <span class="requirement-meta"
              ><AssigneeCell
                :ids="item.assignee_ids"
                :names="item.assignee_names"
                :avatars="item.assignee_avatars"
                :usernames="item.assignee_usernames"
            /></span>
            <CheckOutlined v-if="isSelected(item)" class="selected-icon" />
          </button>
          <a-empty
            v-if="!loading && !list.length"
            :description="$t('requirement_noMatchingRequirements')"
            :image="simpleImage"
          />
        </div>
      </a-spin>

      <a-pagination
        v-if="total > pageSize"
        :current="page"
        :page-size="pageSize"
        :total="total"
        :show-size-changer="false"
        size="small"
        class="picker-pagination"
        @change="changePage"
      />
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { Empty } from "ant-design-vue";
import {
  CheckOutlined,
  CloseOutlined,
  LinkOutlined,
  ReloadOutlined,
} from "@ant-design/icons-vue";
import {
  getRequirement,
  getWorkspaceMetadata,
  listRequirements,
} from "@/api/workspace";
import StatusPill from "@/components/work-item/StatusPill.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";

const props = defineProps({
  modelValue: { type: [String, Number, Array], default: undefined },
  wsId: { type: [String, Number], required: true },
  selectedTitle: { type: String, default: "" },
  excludeId: { type: [String, Number], default: undefined },
  categoryId: { type: [String, Number], default: undefined },
  compact: { type: Boolean, default: false },
  mode: { type: String, default: "related" },
  // Multi-select mode: modelValue is an id array; OK/Cancel appear at the bottom (used by planning relations).
  multiple: { type: Boolean, default: false },
  // Selectable work-item types; defaults to requirement only, preserving existing caller behavior.
  types: { type: Array, default: () => ["requirement"] },
  // Open the dialog on mount; do not render a trigger button.
  autoOpen: { type: Boolean, default: false },
  // When > 0, list only work items already related to this planning (the "view related" entry).
  planningId: { type: [String, Number], default: 0 },
  // Read-only: cannot check, hide the OK button (users with planning:view only).
  readonly: { type: Boolean, default: false },
});
const emit = defineEmits(["update:modelValue", "change", "close"]);
const { t: $t } = useI18n();
const router = useRouter();
const route = useRoute();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;

// Work-item detail is opened in a new tab; resolve() keeps the router base and history mode intact.
function openDetail(item) {
  const target = router.resolve({
    name:
      route.meta.workItemSurface === "issue" ? "IssueDetail" : "WorkItemDetail",
    params: { wsId: props.wsId, id: item.id },
  });
  window.open(target.href, "_blank", "noopener");
}

// The pending selection set in multi-select mode.
const selectedIds = ref([]);

const open = ref(false);
const loading = ref(false);
const search = ref("");
const status = ref(undefined);
const priority = ref(undefined);
const assigneeId = ref(undefined);
const versionId = ref(undefined);
const list = ref([]);
const total = ref(0);
const page = ref(1);
const pageSize = 10;
const members = ref([]);
const versions = ref([]);
const statuses = ref([]);
const priorities = ref([]);
const finalStatusIds = ref([]);
const selectedItem = ref(null);

const selectedText = computed(() => {
  if (!props.modelValue)
    return props.mode === "parent"
      ? $t("requirement_selectParentRequirement")
      : $t("requirement_linkRequirement");
  return (
    props.selectedTitle || selectedItem.value?.title || `#${props.modelValue}`
  );
});
const pickerTitle = computed(() =>
  props.mode === "parent"
    ? $t("requirement_selectParentRequirement")
    : $t("requirement_selectRelatedRequirement"),
);
const clearTitle = computed(() =>
  props.mode === "parent"
    ? $t("requirement_removeParentRequirement")
    : $t("requirement_unlinkRequirement"),
);
const statusOptions = computed(() =>
  statuses.value.map((item) => ({ value: Number(item.id), label: item.name })),
);
const priorityOptions = computed(() =>
  priorities.value.map((item) => ({
    value: Number(item.id),
    label: item.label,
  })),
);
const memberOptions = computed(() =>
  members.value.map((item) => ({ value: item.user_id, label: item.user_name })),
);
const versionOptions = computed(() =>
  versions.value.map((item) => ({ value: item.id, label: item.name })),
);

async function openPicker() {
  open.value = true;
  await Promise.all([loadOptions(), loadRequirements()]);
}

async function loadOptions() {
  try {
    const configs = await getWorkspaceMetadata(props.wsId);
    members.value = configs?.members || [];
    versions.value = configs?.versions || [];
    statuses.value = configs?.requirement?.statuses || [];
    priorities.value = configs?.requirement?.priorities || [];
    finalStatusIds.value = (
      configs?.settings?.requirement_final_status_ids || []
    ).map(Number);
  } catch {
    members.value = [];
    versions.value = [];
    statuses.value = [];
    priorities.value = [];
    finalStatusIds.value = [];
  }
}

function isFinalStatus(item) {
  return finalStatusIds.value.includes(Number(item.status_id));
}

async function loadRequirements() {
  loading.value = true;
  try {
    const params = {
      page: page.value,
      size: pageSize,
      root_only: 0,
    };
    // Send type for a single type; leave empty for multiple types so the backend returns requirement+defect by default.
    if (props.types.length === 1) {
      params.type = props.types[0];
    }
    if (props.mode === "parent") {
      if (props.excludeId) params.exclude_descendants_of = props.excludeId;
      if (props.categoryId !== undefined) {
        params.category_id = Number(props.categoryId) || 0;
        params.category_exact = 1;
      }
    }
    if (search.value.trim()) params.search = search.value.trim();
    if (status.value) params.status_id = status.value;
    if (priority.value) params.priority = priority.value;
    if (assigneeId.value) params.assignee_id = assigneeId.value;
    if (versionId.value) params.version_id = versionId.value;
    // linked mode: narrow results to work items related to this planning; filtering and pagination still handled by the backend.
    if (Number(props.planningId) > 0) params.planning_id = props.planningId;
    const result = await listRequirements(props.wsId, params);
    list.value = (result?.list || []).filter(
      (item) => String(item.id) !== String(props.excludeId || ""),
    );
    total.value = Math.max(
      0,
      (result?.total || 0) -
        ((result?.list || []).some(
          (item) => String(item.id) === String(props.excludeId || ""),
        )
          ? 1
          : 0),
    );
  } catch {
    list.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function applyFilters() {
  page.value = 1;
  loadRequirements();
}

function resetFilters() {
  search.value = "";
  status.value = undefined;
  priority.value = undefined;
  assigneeId.value = undefined;
  versionId.value = undefined;
  applyFilters();
}

function changePage(value) {
  page.value = value;
  loadRequirements();
}

function isSelected(item) {
  if (props.multiple) {
    return selectedIds.value.some((id) => String(id) === String(item.id));
  }
  return String(item.id) === String(props.modelValue);
}

function selectRequirement(item) {
  if (props.readonly) return;
  const id = Number(item.id);
  if (props.multiple) {
    const index = selectedIds.value.findIndex((value) => Number(value) === id);
    if (index >= 0) {
      selectedIds.value.splice(index, 1);
    } else {
      selectedIds.value.push(id);
    }
    return;
  }
  selectedItem.value = item;
  emit("update:modelValue", id);
  emit("change", { ...item, id });
  open.value = false;
}

function confirmMultiple() {
  emit("update:modelValue", [...selectedIds.value]);
  emit("change", [...selectedIds.value]);
  open.value = false;
  emit("close");
}

function handleCancel() {
  open.value = false;
  emit("close");
}

function clearSelection() {
  selectedItem.value = null;
  emit("update:modelValue", undefined);
  emit("change", null);
}

watch(
  () => props.modelValue,
  (value) => {
    if (props.multiple) {
      selectedIds.value = Array.isArray(value) ? [...value] : [];
    }
  },
  { immediate: true },
);

// autoOpen mode opens the dialog on mount (planning relation entry).
onMounted(() => {
  if (props.autoOpen) openPicker();
});

watch(
  () => props.wsId,
  () => {
    search.value = "";
    status.value = undefined;
    priority.value = undefined;
    assigneeId.value = undefined;
    versionId.value = undefined;
    page.value = 1;
    list.value = [];
    total.value = 0;
    selectedItem.value = null;
  },
);

watch(
  () => [props.modelValue, props.wsId, props.selectedTitle],
  async () => {
    // In multi-select mode modelValue is an id array; selectedItem only serves the single-select button label, don't use the array to query a single detail.
    if (props.multiple) return;
    if (!props.modelValue) {
      selectedItem.value = null;
      return;
    }
    if (props.selectedTitle) {
      selectedItem.value = { id: props.modelValue, title: props.selectedTitle };
      return;
    }
    try {
      selectedItem.value = await getRequirement(props.wsId, props.modelValue);
    } catch {
      selectedItem.value = null;
    }
  },
  { immediate: true },
);
</script>

<style scoped lang="less">
.relation-control {
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
}
.relation-control.compact {
  width: 160px;
}
.relation-button {
  display: flex;
  flex: 1;
  align-items: center;
  min-width: 0;
  max-width: 100%;
  color: #1677ff;
  border-color: #91caff;
}
.relation-button-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.unlink-button {
  flex: 0 0 28px;
  width: 28px;
  padding: 0;
}
.picker-panel {
  width: 100%;
}
.picker-filters {
  display: grid;
  grid-template-columns: minmax(190px, 2fr) repeat(4, minmax(100px, 1fr)) 32px;
  gap: 8px;
  margin-bottom: 12px;
}
.picker-filters :deep(.ant-select) {
  width: 100%;
}
.reset-button {
  width: 32px;
  padding: 0;
}
.requirement-list {
  min-height: 180px;
  max-height: 400px;
  overflow: auto;
  border: 1px solid var(--app-border);
}
.requirement-row {
  display: grid;
  grid-template-columns: 48px 30px minmax(160px, 1fr) 84px 72px 72px 72px 20px;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 44px;
  padding: 8px 10px;
  border: 0;
  border-bottom: 1px solid var(--app-border);
  color: var(--app-text);
  background: var(--app-surface);
  text-align: left;
  cursor: pointer;
}
.requirement-row:last-child {
  border-bottom: 0;
}
.requirement-row:hover,
.requirement-row.selected {
  background: color-mix(in srgb, #1677ff 10%, var(--app-surface));
}
.requirement-row.final-status-row {
  opacity: 0.5;
}
/* Read-only view (planning:view only): no clickable hint, no hover-row highlight. */
.requirement-row.readonly-row {
  cursor: default;
}
.requirement-row.readonly-row:hover {
  background: var(--app-surface);
}
.requirement-id,
.requirement-meta {
  overflow: hidden;
  color: var(--app-text-secondary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.requirement-title {
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
// Only the title reads as a link, and only on hover, so the row still feels like a checkbox row.
.requirement-title:hover {
  color: #1677ff;
  text-decoration: underline;
}
.requirement-row :deep(.status-pill),
.requirement-row :deep(.priority-pill) {
  justify-self: start;
  justify-content: center;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}
.selected-icon {
  color: #1677ff;
}
.picker-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
:global(.relation-picker-dropdown) {
  z-index: 1200 !important;
}

@media (max-width: 760px) {
  .picker-filters {
    grid-template-columns: 1fr 1fr;
  }
  .search-input {
    grid-column: 1 / -1;
  }
  .requirement-row {
    grid-template-columns: 48px 30px minmax(120px, 1fr) auto 20px;
  }
  .requirement-row :deep(.priority-pill),
  .requirement-meta {
    display: none;
  }
}
</style>
