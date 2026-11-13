<template>
  <div
    ref="wrapRef"
    class="planning-table-wrap"
    :class="{ 'is-dragging': dragging }"
  >
    <a-table
      ref="tableRef"
      class="planning-table"
      :columns="visibleColumns"
      :data-source="rows"
      :pagination="false"
      :loading="loading"
      row-key="id"
      size="small"
      bordered
      sticky
      table-layout="fixed"
      :row-class-name="rowClassName"
      :locale="{ emptyText: $t('planning_empty') }"
      :scroll="{ x: scrollX, y: bodyHeight }"
      :custom-row="customRow"
      @resizeColumn="handleResizeColumn"
    >
      <!-- Table header: the index column renders as the list-settings entry -->
      <template #headerCell="{ column }">
        <template v-if="column.key === 'row-index'">
          <a-tooltip :title="$t('workspace_listSettings')">
            <a-button
              type="text"
              size="small"
              class="settings-btn"
              @click="$emit('open-settings')"
            >
              <SettingOutlined />
            </a-button>
          </a-tooltip>
        </template>
      </template>

      <template #bodyCell="{ column, record, index }">
        <!-- Index / row actions (show/hide controlled by CSS :hover, not reactive state, to avoid full-table re-render on hover) -->
        <template v-if="column.key === 'row-index'">
          <div class="row-index-cell">
            <span v-if="canEdit" class="row-actions">
              <span class="drag-handle" :title="$t('planning_dragHandle')">
                <MenuOutlined />
              </span>
              <a-button
                type="text"
                size="small"
                class="row-more-btn"
                :title="$t('planning_rowMore')"
                @click.stop="rowActionsRef?.openAt(record, index, $event)"
              >
                <MoreOutlined />
              </a-button>
            </span>
            <span class="row-index-number">{{ index + 1 }}</span>
          </div>
        </template>

        <!-- Planning name: double-click to edit (name only) + color -->
        <template v-else-if="column.key === 'title'">
          <div v-if="isEditing(record.id, 'title')" class="title-edit">
            <a-input
              :ref="(element) => setTitleInputRef(record.id, element)"
              v-model:value="draft.value"
              size="small"
              @press-enter="$event.target.blur()"
              @blur="commitEdit(record)"
              @keydown.esc.stop.prevent="cancelEdit"
              @click.stop
            />
            <a-tooltip :title="$t('planning_titleStrike')">
              <span
                class="strike-trigger"
                :class="{ active: draft.strike }"
                @mousedown.prevent
                @click.stop="draft.strike = draft.strike ? 0 : 1"
              >
                <StrikethroughOutlined />
              </span>
            </a-tooltip>
            <a-popover
              v-model:open="colorPickerOpen"
              trigger="click"
              placement="bottomRight"
            >
              <template #content>
                <div class="color-grid">
                  <span
                    v-for="color in PRESET_COLORS"
                    :key="color"
                    class="color-dot"
                    :class="{ selected: color === (draft.color || '#000000') }"
                    :style="{ backgroundColor: color }"
                    @mousedown.prevent
                    @click.stop="pickColor(color)"
                  />
                </div>
              </template>
              <span
                class="color-trigger"
                :style="{ backgroundColor: draft.color || '#000000' }"
                @mousedown.prevent
                @click.stop
              />
            </a-popover>
          </div>
          <div
            v-else
            class="title-display"
            @dblclick.stop="startEditIfAllowed(record, 'title', record.title)"
          >
            <span
              class="title-text"
              :class="{ 'title-struck': record.title_strike }"
              :style="{
                color: record.title_strike
                  ? undefined
                  : record.color || undefined,
              }"
              :title="record.title"
              >{{ record.title }}</span
            >
          </div>
        </template>

        <!-- Priority -->
        <template v-else-if="column.key === 'priority'">
          <a-select
            v-if="isEditing(record.id, 'priority')"
            v-model:value="draft.value"
            size="small"
            class="cell-select"
            :options="priorityOptions"
            default-open
            allow-clear
            :popup-match-select-width="false"
            popup-class-name="planning-cell-dropdown"
            @change="commitEdit(record)"
            @blur="cancelEdit"
            @keydown.esc.stop.prevent="cancelEdit"
            @click.stop
          >
            <template #option="{ value: optionValue, label }">
              <PriorityPill
                :priority="optionValue"
                :label="label"
                :color="priorityMap[optionValue]?.color"
              />
            </template>
          </a-select>
          <span v-else class="cell-value cell-center">
            <PriorityPill
              v-if="priorityMap[record.priority]"
              :priority="record.priority"
              :label="priorityMap[record.priority].label"
              :color="priorityMap[record.priority].color"
            />
            <span v-else class="cell-empty">-</span>
          </span>
        </template>

        <!-- Related requirements: relation count always shown (click to view related); on row hover, append create/relate/clear on the right -->
        <template v-else-if="column.key === 'related_items'">
          <!-- All tooltips use native title to avoid mounting many Tooltip components per row, which slows long lists -->
          <div class="relation-cell">
            <span
              class="relation-count"
              :class="{
                empty: !record.relations?.length,
                disabled: !canViewWorkItem,
              }"
              :title="relationTitle(record)"
              @click.stop="canViewWorkItem && emit('view-relations', record)"
            >
              <LinkOutlined /> {{ record.relations?.length || 0 }}
            </span>
            <template v-if="canEdit">
              <span class="relation-actions">
                <PlusCircleOutlined
                  v-if="canCreateRequirement"
                  class="relation-icon"
                  :title="$t('planning_relationCreate')"
                  @click.stop="emit('create-requirement', record)"
                />
                <ApartmentOutlined
                  v-if="canViewWorkItem"
                  class="relation-icon"
                  :title="$t('planning_relationLink')"
                  @click.stop="emit('open-relations', record)"
                />
                <DeleteOutlined
                  class="relation-icon danger"
                  :class="{ disabled: !record.relations?.length }"
                  :title="$t('planning_relationClear')"
                  @click.stop="
                    record.relations?.length && clearRelations(record)
                  "
                />
              </span>
            </template>
          </div>
        </template>

        <!-- Status -->
        <template v-else-if="column.key === 'status'">
          <a-select
            v-if="isEditing(record.id, 'status_id')"
            v-model:value="draft.value"
            size="small"
            class="cell-select"
            :options="statusOptions"
            default-open
            :popup-match-select-width="false"
            popup-class-name="planning-cell-dropdown"
            @change="commitEdit(record)"
            @blur="cancelEdit"
            @keydown.esc.stop.prevent="cancelEdit"
            @click.stop
          >
            <template #option="{ value: optionValue, label }">
              <StatusPill
                :text="label"
                :color="statusMap[optionValue]?.color"
              />
            </template>
          </a-select>
          <span v-else class="cell-value cell-center">
            <StatusPill
              v-if="statusMap[record.status_id]"
              :text="statusMap[record.status_id].name"
              :color="statusMap[record.status_id].color"
            />
            <span v-else class="cell-empty">-</span>
          </span>
        </template>

        <!-- Assignee (multi-select) -->
        <template v-else-if="column.key === 'assignee'">
          <AssigneeSelect
            v-if="isEditing(record.id, 'assignee_ids')"
            :value="record.assignee_ids"
            :members="members"
            class="cell-select"
            @save="(value) => saveAssignees(record, value)"
            @cancel="cancelEdit"
            @click.stop
          />
          <span v-else class="cell-value">
            <AssigneeCell
              :ids="record.assignee_ids || []"
              :names="assigneeNames(record)"
              :avatars="assigneeAvatars(record)"
            />
          </span>
        </template>

        <!-- Estimated start / end -->
        <template
          v-else-if="
            column.key === 'planned_start_date' ||
            column.key === 'planned_end_date'
          "
        >
          <a-date-picker
            v-if="isEditing(record.id, column.key)"
            v-model:value="draft.value"
            size="small"
            class="cell-date"
            open
            @change="commitEdit(record)"
            @open-change="(opened) => !opened && cancelEdit()"
            @keydown.esc.stop.prevent="cancelEdit"
            @click.stop
          />
          <span v-else class="cell-value">
            {{ formatLocalDate(record[column.key]) || "-" }}
          </span>
        </template>

        <!-- Custom fields -->
        <template v-else-if="column.customField">
          <PlanningCustomCell
            :record="record"
            :field="column.customField"
            :editing="isEditing(record.id, customKey(column.customField))"
            :draft="draft"
            :members="customFieldMembers"
            @commit="commitEdit(record)"
            @cancel="cancelEdit"
          />
        </template>
      </template>
      <!-- Fixed summary row inside the table: "Add a row" always follows the list bottom, same width and scroll as the table -->
      <template v-if="canEdit" #summary>
        <a-table-summary fixed>
          <a-table-summary-row class="add-row-summary">
            <a-table-summary-cell :index="0">
              <div class="add-row-actions">
                <a-button
                  type="text"
                  size="small"
                  class="add-row-btn"
                  :loading="appending"
                  @click="insertRow(rows.length - 1, 'below')"
                >
                  <template #icon><PlusOutlined /></template>
                  {{ $t("planning_addRow") }}
                </a-button>
                <a-button
                  type="text"
                  size="small"
                  class="add-row-btn"
                  @click="emit('batch-add')"
                >
                  <template #icon><PlusCircleOutlined /></template>
                  {{ $t("planning_addRows") }}
                </a-button>
              </div>
            </a-table-summary-cell>
            <a-table-summary-cell
              :index="1"
              :col-span="Math.max(1, visibleColumns.length - 1)"
            />
          </a-table-summary-row>
        </a-table-summary>
      </template>
    </a-table>

    <PlanningRowActions
      v-if="canEdit"
      ref="rowActionsRef"
      @insert-below="insertRowsBelow"
      @move-to-row="moveToRow"
      @move-to-bottom="moveToBottom"
      @toggle-archive="toggleArchive"
      @delete="removeRow"
    />

    <!-- 行悬停时的新增按钮：中心对准悬停行的左上角，挂在表格容器上，
         不受 .ant-table-body 的 overflow 裁剪。显隐与定位直接改 DOM 样式，
         不进响应式状态，避免悬停/滚动时整表重渲染。 -->
    <button
      v-if="canEdit"
      ref="insertBtnRef"
      type="button"
      class="insert-btn"
      :title="$t('planning_insertAbove')"
      @mouseleave="handleInsertBtnLeave"
      @click.stop="insertRow(hoveredIndex, 'above')"
    >
      <PlusOutlined />
    </button>

    <!-- Vertical guide line while dragging column width (same as requirement/defect lists) -->
    <div
      v-if="tableResizeGuide.visible"
      class="table-resize-guide"
      :style="{
        left: `${tableResizeGuide.left}px`,
        top: `${tableResizeGuide.top}px`,
        height: `${tableResizeGuide.height}px`,
        '--header-height': `${tableResizeGuide.headerHeight}px`,
      }"
    />
  </div>
</template>

<script setup>
import {
  ref,
  reactive,
  computed,
  watch,
  onMounted,
  onBeforeUnmount,
  nextTick,
} from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import dayjs from "dayjs";
import Sortable from "sortablejs";
import {
  SettingOutlined,
  PlusOutlined,
  MenuOutlined,
  DeleteOutlined,
  MoreOutlined,
  LinkOutlined,
  PlusCircleOutlined,
  ApartmentOutlined,
  StrikethroughOutlined,
} from "@ant-design/icons-vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import PlanningCustomCell from "./PlanningCustomCell.vue";
import PlanningRowActions from "./PlanningRowActions.vue";
import { useAuthStore } from "@/stores/auth";
import { useContactPreferencesStore } from "@/stores/contactPreferences";
import { formatLocalDate } from "@/utils/date";
import {
  customFieldValues,
  isValidPhoneFieldValue,
  parseCustomFieldValue,
  serializeCustomFieldValue,
  storedCustomFieldValue,
} from "@/utils/customField";
import {
  listPlannings,
  createPlanning,
  updatePlanning,
  deletePlanning,
  movePlanning,
  savePlanningRelations,
} from "@/api/planning";

// The 12-color preset for planning names, kept consistent with the prototype.
const PRESET_COLORS = [
  "#000000",
  "#434343",
  "#8c8c8c",
  "#bfbfbf",
  "#f5222d",
  "#fa541c",
  "#fa8c16",
  "#fadb14",
  "#52c41a",
  "#13c2c2",
  "#1677ff",
  "#722ed1",
];
const PAGE_SIZE = 50;
// Min width of the "Planning name" column (auto-fills remaining space when not dragged).
const TITLE_MIN_WIDTH = 360;
// The index column width, used in the title column's auto-width calculation.
const ROW_INDEX_WIDTH = 54;

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  groupId: { type: [String, Number], default: null },
  statuses: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  customFieldMembers: { type: Array, default: () => [] },
  // The field list output by the list-settings dialog (with project-level order and personal show/hide).
  fields: { type: Array, default: () => [] },
  filters: { type: Object, default: () => ({}) },
  canEdit: { type: Boolean, default: false },
  // After creating a group, auto-add a row and enter title-edit mode.
  autoAddRow: { type: Boolean, default: false },
});
const emit = defineEmits([
  "open-settings",
  "open-relations",
  "view-relations",
  "create-requirement",
  "batch-add",
]);
const { t: $t } = useI18n();
const authStore = useAuthStore();
const contactPreferences = useContactPreferencesStore();

// Planning permissions do not imply work-item permissions: the relation entries hit the requirement APIs,
// so they need their own checks, otherwise the user is bounced with 403.
const canViewWorkItem = computed(
  () =>
    authStore.hasPermission("requirement:view") ||
    authStore.hasPermission("defect:view"),
);
const canCreateRequirement = computed(() =>
  authStore.hasPermission("requirement:create"),
);

const rows = ref([]);
const loading = ref(false);
const appending = ref(false);
// The hover row is only for positioning the add button; it is not rendered in the template, so no reactive state.
let hoveredId = null;
let hoveredIndex = -1;
let hoveredRowEl = null;
const tableRef = ref(null);
const rowActionsRef = ref(null);
const wrapRef = ref(null);
const hasMore = ref(false);
const undoStack = ref([]);
let sortableInstance = null;
let scrollEl = null;

// The table body's visible height, measured from the container so the table fills the remaining page space.
const bodyHeight = ref(320);
let containerObserver = null;

// The add button on the hover row (positioned via direct DOM, not reactive).
const insertBtnRef = ref(null);
let insertBtnFrame = 0;
// While drag-sorting: row positions keep changing, so the add button is never shown.
const dragging = ref(false);

// Column widths: drag to adjust and persist to the browser (same as requirement/defect lists).
const columnWidths = reactive({});
const tableResizeGuide = reactive({
  visible: false,
  left: 0,
  top: 0,
  height: 0,
  headerHeight: 0,
});
let tableResizeGuideCleanup;
const containerWidth = ref(0);

const draft = reactive({
  id: null,
  field: null,
  value: null,
  color: "",
  strike: 0,
});
const colorPickerOpen = ref(false);
const titleInputs = new Map();

// ---------------- Options and mappings ----------------
const statusMap = computed(() =>
  Object.fromEntries(props.statuses.map((item) => [item.id, item])),
);
const priorityMap = computed(() =>
  Object.fromEntries(props.priorities.map((item) => [item.id, item])),
);
const statusOptions = computed(() =>
  props.statuses.map((item) => ({ value: item.id, label: item.name })),
);
const priorityOptions = computed(() =>
  props.priorities.map((item) => ({ value: item.id, label: item.label })),
);
// Member index: every row in a long list needs the assignee, avoid scanning members per row.
const memberMap = computed(
  () => new Map(props.members.map((item) => [Number(item.user_id), item])),
);

// Status and priority tags are rendered uniformly by StatusPill / PriorityPill; no custom styles here.
function assigneeNames(record) {
  return (record.assignee_ids || []).map(
    (id) => memberMap.value.get(Number(id))?.user_name || "",
  );
}
function assigneeAvatars(record) {
  return (record.assignee_ids || []).map(
    (id) => memberMap.value.get(Number(id))?.user_avatar || "",
  );
}
// Native tooltip for the relation column: related items shown row by row; prompt to relate when none.
function relationTitle(record) {
  const relations = record.relations || [];
  if (!relations.length) return $t("planning_relationLink");
  return relations.map((item) => `#${item.id} ${item.title}`).join("\n");
}
function customKey(field) {
  return `cf_${field.id}`;
}
// Current custom-field values (same as requirement list: multi-select parsed to array, date to dayjs).
function customValue(record, field) {
  return parseCustomFieldValue(
    field,
    customFieldValues(record.custom_fields)[String(field.id)],
  );
}
function normalizeCustomDraft(field, value) {
  return serializeCustomFieldValue(field, value);
}
function toDayjs(value) {
  return value ? dayjs.unix(Number(value)) : null;
}

// Built-in column layout; column names come from the backend's label, consistent with project settings.
const BUILTIN_LAYOUT = {
  title: {},
  priority: { width: 110, align: "center" },
  related_items: { width: 180 },
  status: { width: 110, align: "center" },
  assignee: { width: 150 },
  planned_start_date: { width: 130 },
  planned_end_date: { width: 130 },
};

// Built-in column -> editable field name (same as requirement list: click the whole cell to edit).
const EDITABLE_BUILTIN_FIELDS = {
  priority: "priority",
  status: "status_id",
  assignee: "assignee_ids",
  planned_start_date: "planned_start_date",
  planned_end_date: "planned_end_date",
};

// Read the edit-draft initial value; arrays and dates must be copied/converted before passing to the control.
function draftValueOf(record, field, customField) {
  if (customField) return customValue(record, customField);
  if (field === "assignee_ids") return [...(record.assignee_ids || [])];
  if (field === "planned_start_date" || field === "planned_end_date") {
    return toDayjs(record[field]);
  }
  // Priority and status use 0 for "unset". Passing 0 straight to a-select treats it as a real
  // value and renders a bare "0" because no option matches; undefined falls back to the
  // placeholder. On submit, commitEdit's `value ?? 0` turns it back into 0.
  if (field === "priority" || field === "status_id") {
    return Number(record[field]) || undefined;
  }
  return record[field];
}

// Editable cell: the whole cell is clickable, same as the requirement/defect list's editable-table-cell.
function editableCellProps(field, customField) {
  return (record) => {
    if (!props.canEdit) return {};
    return {
      class: "editable-table-cell",
      onClick: () => {
        if (isEditing(record.id, field)) return;
        startEdit(record, field, draftValueOf(record, field, customField));
      },
    };
  };
}

// Column order and visibility are decided entirely by the list-settings dialog (built-in and custom fields can be mixed).
const visibleColumns = computed(() => {
  // The index column is always rendered first as a fixed column: it is locked and cannot be closed in project settings,
  // and is not part of personal show/hide preferences, so it cannot rely on fields (old saved prefs don't include it).
  const columns = [
    {
      key: "row-index",
      width: ROW_INDEX_WIDTH,
      fixed: "left",
      align: "center",
    },
  ];
  // Read the recorded value for columns already dragged, otherwise use the default width.
  const sized = (key, fallback, minWidth, maxWidth = 600) => {
    const stored = Number(columnWidths[key]);
    return {
      width: Number.isFinite(stored) && stored > 0 ? stored : fallback,
      minWidth,
      maxWidth,
      resizable: true,
    };
  };
  for (const field of props.fields) {
    if (field.visible === false) continue;
    if (field.kind === "custom") {
      const key = customKey(field.customField);
      columns.push({
        key,
        title: field.label,
        ellipsis: true,
        customField: field.customField,
        ...sized(key, 140, 80),
        customCell: editableCellProps(key, field.customField),
      });
      continue;
    }
    const layout = BUILTIN_LAYOUT[field.configKey];
    if (!layout) continue;
    const editableField = EDITABLE_BUILTIN_FIELDS[field.configKey];
    const isTitle = field.configKey === "title";
    columns.push({
      key: field.configKey,
      dataIndex: field.configKey,
      title: field.label,
      ellipsis: true,
      align: layout.align,
      ...sized(
        field.configKey,
        isTitle ? autoTitleWidth.value : layout.width,
        isTitle ? 200 : 80,
        isTitle ? 1000 : 600,
      ),
      ...(editableField
        ? { customCell: editableCellProps(editableField) }
        : {}),
    });
  }
  return columns;
});

// The title column takes the remaining width when not dragged, to avoid blank space on the right.
const autoTitleWidth = computed(() => {
  const fixed = props.fields.reduce((total, field) => {
    if (field.visible === false) return total;
    if (field.kind === "custom") {
      return (
        total + (Number(columnWidths[customKey(field.customField)]) || 140)
      );
    }
    if (field.configKey === "title") return total;
    const layout = BUILTIN_LAYOUT[field.configKey];
    if (!layout) return total;
    return (
      total + (Number(columnWidths[field.configKey]) || layout.width || 80)
    );
  }, ROW_INDEX_WIDTH);
  return Math.min(
    1000,
    Math.max(TITLE_MIN_WIDTH, (containerWidth.value || 0) - fixed),
  );
});

// Total table width: all column widths are explicit (incl. the title column's adaptive value), just sum them.
const scrollX = computed(() =>
  visibleColumns.value.reduce(
    (total, column) => total + (column.width || TITLE_MIN_WIDTH),
    0,
  ),
);

// The add button is now an overlay outside the table, so rows no longer need to raise their z-index on hover.
function rowClassName() {
  return "planning-row";
}

// ---------------- Data loading ----------------
async function fetchRows(append = false) {
  if (!props.groupId) {
    rows.value = [];
    return;
  }
  loading.value = !append;
  try {
    const last = append ? rows.value[rows.value.length - 1] : null;
    const result = await listPlannings(props.wsId, {
      group_id: props.groupId,
      size: PAGE_SIZE,
      ...props.filters,
      ...(last
        ? { cursor_id: last.id, cursor_sort_order: last.sort_order }
        : {}),
    });
    const list = result?.list || [];
    rows.value = append ? [...rows.value, ...list] : list;
    hasMore.value = !!result?.has_more;
    return true;
  } catch {
    if (!append) rows.value = [];
    return false;
  } finally {
    loading.value = false;
  }
}

// Append the next page when scrolled near the bottom (no pagination, scroll loading).
function handleScroll(event) {
  // Row positions change with scrolling, so the add button must be recomputed synchronously.
  scheduleInsertBtn();
  const el = event.target;
  if (
    hasMore.value &&
    !loading.value &&
    el.scrollHeight - el.scrollTop - el.clientHeight < 120
  ) {
    fetchRows(true);
  }
}

watch(
  () => [props.groupId, props.filters],
  () => fetchRows(false),
  { deep: true },
);

// ---------------- Inline editing ----------------
function isEditing(id, field) {
  return draft.id === id && draft.field === field;
}
function startEdit(record, field, value) {
  draft.id = record.id;
  draft.field = field;
  draft.value = value;
  // Name, color and strike-through belong to the same edit and are submitted together at the end.
  draft.color = field === "title" ? record.color || "" : "";
  draft.strike = field === "title" ? Number(record.title_strike) || 0 : 0;
  if (field !== "title") return;
  // Focus and select-all only when entering edit; putting it in the ref callback would select-all on every re-render and overwrite input.
  nextTick(() => {
    const input = titleInputs.get(String(record.id));
    input?.focus?.();
    input?.select?.();
  });
}
function startEditIfAllowed(record, field, value) {
  if (!props.canEdit) return;
  startEdit(record, field, value);
}
function cancelEdit() {
  draft.id = null;
  draft.field = null;
  draft.value = null;
  draft.color = "";
  draft.strike = 0;
  colorPickerOpen.value = false;
}

// Register the name input instance (access only; focus handling is done elsewhere).
function setTitleInputRef(recordId, element) {
  const key = String(recordId);
  if (element) titleInputs.set(key, element);
  else titleInputs.delete(key);
}

// Color selection only updates the draft and previews live, without a separate request.
function pickColor(color) {
  draft.color = color;
  colorPickerOpen.value = false;
}

// Submit edit: exit edit mode first, then assemble the update content by field.
async function commitEdit(record) {
  const field = draft.field;
  if (!field || draft.id !== record.id) return false;
  const value = draft.value;
  const color = draft.color;
  const strike = draft.strike;
  const customField = field.startsWith("cf_")
    ? props.fields.find(
        (item) =>
          item.kind === "custom" &&
          String(item.customField.id) === field.slice(3),
      )?.customField
    : null;
  if (customField && !isValidPhoneFieldValue(customField, value)) {
    message.warning($t("settings_fieldPhoneInvalid"));
    return false;
  }
  cancelEdit();
  if (field.startsWith("cf_")) {
    if (customField) return saveCustomField(record, customField, value);
    return false;
  }
  // Name: empty titles are not stored; name, color and strike-through are merged into one submit.
  if (field === "title") {
    const title = String(value ?? "").trim();
    const updates = {};
    if (title) updates.title = title;
    updates.color = color || "";
    updates.title_strike = strike;
    return saveFields(record, updates);
  }
  if (field === "planned_start_date" || field === "planned_end_date") {
    const timestamp = value ? dayjs(value).startOf("day").unix() : 0;
    return saveFields(record, { [field]: timestamp });
  }
  return saveFields(record, {
    [field]: value ?? (field === "assignee_ids" ? [] : 0),
  });
}

async function commitAssignees(record, value) {
  draft.value = value;
  const saved = await commitEdit(record);
  if (saved) await contactPreferences.recordRecent(value);
}

// Save assignees on every change (AssigneeSelect "change-to-save" mode).
// Do NOT exit edit mode here, otherwise the component unmounts and the
// dropdown disappears while saving. The edit session ends when the user
// clicks outside (handled by AssigneeSelect @cancel -> cancelEdit).
// Recent-contact ordering is deferred to dropdown close inside AssigneeSelect.
async function saveAssignees(record, value) {
  if (draft.id !== record.id || draft.field !== "assignee_ids") return;
  const newValue = Array.isArray(value) ? value : [];
  try {
    await saveFields(record, { assignee_ids: newValue });
  } catch {
    /* handled by interceptor; record retains the previous value */
  }
}

// Submit changed fields: no request if unchanged; write back locally only after a successful request (same as requirement list).
async function saveFields(record, updates) {
  const changed = Object.entries(updates).filter(
    ([field, value]) =>
      JSON.stringify(record[field] ?? null) !== JSON.stringify(value ?? null),
  );
  if (!changed.length) return false;
  try {
    await updatePlanning(props.wsId, record.id, Object.fromEntries(changed));
    for (const [field, value] of changed) record[field] = value;
    message.success($t("common_update"));
    return true;
  } catch {
    /* handled by interceptor; keep local value */
    return false;
  }
}

async function saveCustomField(record, customField, value) {
  const next = normalizeCustomDraft(customField, value);
  const previous = normalizeCustomDraft(
    customField,
    customValue(record, customField),
  );
  if (JSON.stringify(previous) === JSON.stringify(next)) return;
  try {
    await updatePlanning(props.wsId, record.id, {
      custom_fields: { [customField.id]: next },
    });
    if (!record.custom_fields) record.custom_fields = {};
    record.custom_fields[String(customField.id)] = storedCustomFieldValue(
      customField,
      next,
    );
    message.success($t("common_update"));
  } catch {
    /* handled by interceptor; keep local value */
  }
}

// ---------------- Add/delete and undo ----------------
async function insertRow(index, position) {
  if (!props.canEdit) return;
  const anchor = rows.value[index];
  const prev = position === "above" ? rows.value[index - 1] : anchor;
  const next = position === "above" ? anchor : rows.value[index + 1];
  appending.value = true;
  try {
    // The backend fills in initial status, default priority, etc.; use the returned full row directly.
    const row = await createPlanning(props.wsId, {
      group_id: Number(props.groupId),
      prev_id: prev?.id || 0,
      next_id: next?.id || 0,
    });
    const insertAt = position === "above" ? index : index + 1;
    rows.value.splice(Math.max(0, insertAt), 0, row);
    // The new row's title enters edit mode immediately, matching the prototype.
    nextTick(() => startEdit(row, "title", ""));
  } catch {
    /* handled by interceptor */
  } finally {
    appending.value = false;
  }
}

// Insert blank rows after the chosen row while preserving the current visible order.
async function insertRowsBelow(record, index, count) {
  if (!props.canEdit || !record) return;
  const amount = Math.min(50, Math.max(1, Number(count) || 1));
  if (index === rows.value.length - 1 && hasMore.value) {
    if (!(await fetchRows(true))) return;
    index = rows.value.findIndex((item) => item.id === record.id);
  }
  const next = rows.value[index + 1];
  let prev = record;
  const createdRows = [];
  appending.value = true;
  try {
    for (let offset = 0; offset < amount; offset += 1) {
      const created = await createPlanning(props.wsId, {
        group_id: Number(props.groupId),
        prev_id: prev.id,
        next_id: next?.id || 0,
      });
      createdRows.push(created);
      prev = created;
    }
    rows.value.splice(index + 1, 0, ...createdRows);
    if (createdRows.length === 1) {
      nextTick(() => startEdit(createdRows[0], "title", ""));
    }
  } catch {
    await fetchRows(false);
  } finally {
    appending.value = false;
  }
}

async function loadAllRows() {
  while (hasMore.value) {
    if (!(await fetchRows(true))) return false;
  }
  return true;
}

// Move to a one-based row number in the current filtered result.
async function moveToRow(record, rowNumber) {
  if (!props.canEdit || !record || !(await loadAllRows())) return;
  const oldIndex = rows.value.findIndex((item) => item.id === record.id);
  if (oldIndex < 0 || rows.value.length < 2) return;
  const newIndex = Math.min(
    rows.value.length - 1,
    Math.max(0, (Number(rowNumber) || 1) - 1),
  );
  if (oldIndex === newIndex) return;
  const reordered = [...rows.value];
  const [moved] = reordered.splice(oldIndex, 1);
  reordered.splice(newIndex, 0, moved);
  rows.value = reordered;
  try {
    await movePlanning(props.wsId, moved.id, {
      group_id: Number(props.groupId),
      prev_id: reordered[newIndex - 1]?.id || 0,
      next_id: reordered[newIndex + 1]?.id || 0,
    });
  } catch {
    await fetchRows(false);
  }
}

// Move to the bottom: moveToRow clamps the target index, so an oversized row number lands on the last row.
function moveToBottom(record) {
  return moveToRow(record, Number.MAX_SAFE_INTEGER);
}

// Archive / unarchive a row; it leaves the list when it stops matching the current archive filter.
async function toggleArchive(record) {
  if (!props.canEdit || !record) return;
  const next = record.is_archived ? 0 : 1;
  try {
    await updatePlanning(props.wsId, record.id, { is_archived: next });
    record.is_archived = next;
    message.success(
      $t(next ? "planning_archiveSuccess" : "planning_unarchiveSuccess"),
    );
    if (String(props.filters.archived ?? "0") !== "all") {
      rows.value = rows.value.filter((item) => item.id !== record.id);
    }
  } catch {
    /* handled by interceptor */
  }
}

async function removeRow(record) {
  if (!props.canEdit) return;
  const index = rows.value.findIndex((item) => item.id === record?.id);
  if (index < 0) return;
  try {
    await deletePlanning(props.wsId, record.id);
    rows.value.splice(index, 1);
    // Record the full row data and adjacent position for Ctrl+Z reconstruction.
    undoStack.value.push({
      record: JSON.parse(JSON.stringify(record)),
      prevId: rows.value[index - 1]?.id || 0,
      nextId: rows.value[index]?.id || 0,
    });
    if (undoStack.value.length > 20) undoStack.value.shift();
  } catch {
    /* handled by interceptor */
  }
}

// Ctrl+Z / Cmd+Z undo delete: recreate with the original data and reinsert at the original position.
async function handleUndo(event) {
  if (!((event.ctrlKey || event.metaKey) && event.key === "z")) return;
  // During cell editing, leave input undo to the browser.
  if (draft.id !== null) return;
  if (!props.canEdit || !undoStack.value.length) return;
  event.preventDefault();
  const entry = undoStack.value.pop();
  const source = entry.record;
  try {
    const created = await createPlanning(props.wsId, {
      group_id: Number(props.groupId),
      prev_id: entry.prevId,
      next_id: entry.nextId,
      title: source.title,
      color: source.color,
      title_strike: source.title_strike,
      is_archived: source.is_archived,
      priority: source.priority,
      status_id: source.status_id,
      assignee_ids: source.assignee_ids || [],
      planned_start_date: source.planned_start_date,
      planned_end_date: source.planned_end_date,
      custom_fields: source.custom_fields || {},
    });
    if (source.relations?.length) {
      await savePlanningRelations(
        props.wsId,
        created.id,
        source.relations.map((item) => item.id),
      );
    }
    await fetchRows(false);
    message.success($t("planning_undoDelete"));
  } catch {
    /* handled by interceptor */
  }
}

async function clearRelations(record) {
  try {
    await savePlanningRelations(props.wsId, record.id, []);
    record.relations = [];
  } catch {
    /* handled by interceptor */
  }
}

// ---------------- Row hover and drag ----------------
function customRow(record, index) {
  return {
    onContextmenu: (event) => {
      if (!props.canEdit) return;
      event.preventDefault();
      rowActionsRef.value?.openAt(record, index, event);
    },
    onMouseenter: (event) => {
      hoveredId = record.id;
      hoveredIndex = index;
      hoveredRowEl = event.currentTarget;
      scheduleInsertBtn();
    },
    onMouseleave: (event) => {
      if (hoveredId !== record.id) return;
      // Do not collapse when the mouse moves toward the add button (the button floats outside the row, a separate element).
      if (insertBtnRef.value?.contains(event.relatedTarget)) return;
      hideInsertBtn();
    },
  };
}

// Collapse when leaving the add button and not returning to the original row.
function handleInsertBtnLeave(event) {
  if (hoveredRowEl?.contains(event.relatedTarget)) return;
  hideInsertBtn();
}

// Add-button positioning: the button center aligns to the hovered row's top-left (centering done by CSS transform).
// The overlay hangs on the container; on first/last row it may cover the header and summary row (not clipped by body's overflow).
function updateInsertBtn() {
  const button = insertBtnRef.value;
  if (!button) return;
  if (dragging.value || !hoveredRowEl || !wrapRef.value) {
    button.style.display = "none";
    return;
  }
  const wrapRect = wrapRef.value.getBoundingClientRect();
  const rowRect = hoveredRowEl.getBoundingClientRect();
  const top = Math.min(
    Math.max(rowRect.top - wrapRect.top, 0),
    wrapRect.height,
  );
  // On horizontal scroll the row box shifts left (rect.left goes negative), but the index column is a left-fixed column still against the table's left edge,
  // so x must use the table container's left boundary, not the row box's left, otherwise the button is pushed out of view.
  button.style.left = "0px";
  button.style.top = `${top}px`;
  button.style.display = "inline-flex";
}

// Hover and scroll both fire at high frequency; merge with rAF to avoid repeated layout reads.
function scheduleInsertBtn() {
  if (insertBtnFrame) return;
  insertBtnFrame = window.requestAnimationFrame(() => {
    insertBtnFrame = 0;
    updateInsertBtn();
  });
}

function hideInsertBtn() {
  hoveredId = null;
  hoveredRowEl = null;
  if (insertBtnRef.value) insertBtnRef.value.style.display = "none";
}

// ---------------- Column-width drag and memory ----------------
function columnWidthStorageKey() {
  const userId = authStore.userInfo?.id;
  if (!userId || !props.wsId) return "";
  return `goteams:list-column-widths:${userId}:${props.wsId}:planning`;
}

function loadColumnWidths() {
  for (const key of Object.keys(columnWidths)) delete columnWidths[key];
  const storageKey = columnWidthStorageKey();
  if (!storageKey) return;
  try {
    const stored = JSON.parse(window.localStorage.getItem(storageKey) || "{}");
    if (!stored || typeof stored !== "object" || Array.isArray(stored)) return;
    for (const [key, width] of Object.entries(stored)) {
      const value = Number(width);
      if (Number.isFinite(value) && value > 0) columnWidths[key] = value;
    }
  } catch {
    // When browser cache is unavailable or corrupted, keep using the default column width.
  }
}

function persistColumnWidths() {
  const storageKey = columnWidthStorageKey();
  if (!storageKey) return;
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(columnWidths));
  } catch {
    // Limited browser cache should not block the current page's column-width adjustment.
  }
}

function handleResizeColumn(width, column) {
  const minWidth = column.minWidth || 80;
  const maxWidth = column.maxWidth || 600;
  columnWidths[column.key] = Math.min(maxWidth, Math.max(minWidth, width));
}

// The vertical guide line while dragging; persist the column width on release.
function startTableResizeGuide(event) {
  const handle = event.target.closest?.(
    ".planning-table .ant-table-resize-handle",
  );
  if (!handle || !wrapRef.value) return;
  const container = handle
    .closest(".planning-table")
    ?.querySelector(".ant-table-container");
  if (!container) return;
  tableResizeGuideCleanup?.();
  const wrapRect = wrapRef.value.getBoundingClientRect();
  const rect = container.getBoundingClientRect();
  const headerRect = container
    .querySelector(".ant-table-thead")
    ?.getBoundingClientRect();
  tableResizeGuide.visible = true;
  tableResizeGuide.left = event.clientX - wrapRect.left;
  tableResizeGuide.top = rect.top - wrapRect.top;
  tableResizeGuide.height = rect.height;
  tableResizeGuide.headerHeight = Math.min(
    rect.height,
    Math.max(0, (headerRect?.bottom || rect.top) - rect.top),
  );

  const moveGuide = (moveEvent) => {
    tableResizeGuide.left =
      Math.min(rect.right, Math.max(rect.left, moveEvent.clientX)) -
      wrapRect.left;
  };
  const stopGuide = () => {
    tableResizeGuide.visible = false;
    document.removeEventListener("mousemove", moveGuide);
    document.removeEventListener("mouseup", stopGuide);
    tableResizeGuideCleanup = undefined;
    window.requestAnimationFrame(persistColumnWidths);
  };
  tableResizeGuideCleanup = stopGuide;
  document.addEventListener("mousemove", moveGuide);
  document.addEventListener("mouseup", stopGuide);
}

// ---------------- Size adaptation ----------------
// The table body height fills the container's remaining space; when the summary row is outside the body, subtract its height too.
function updateLayoutMetrics() {
  const wrap = wrapRef.value;
  const table = tableRef.value?.$el;
  if (!wrap || !table) return;
  containerWidth.value = wrap.clientWidth;
  const header = table.querySelector(".ant-table-header");
  const body = table.querySelector(".ant-table-body");
  const summary = table.querySelector(".ant-table-summary");
  const headerHeight = header?.getBoundingClientRect().height || 0;
  const summaryHeight =
    summary && body && !body.contains(summary)
      ? summary.getBoundingClientRect().height
      : 0;
  bodyHeight.value = Math.max(
    160,
    wrap.clientHeight - headerHeight - summaryHeight,
  );
}

function setupSortable() {
  const tbody = tableRef.value?.$el?.querySelector(".ant-table-tbody");
  if (!tbody || sortableInstance) return;
  sortableInstance = Sortable.create(tbody, {
    handle: ".drag-handle",
    draggable: ".planning-row",
    animation: 150,
    scroll: true,
    ghostClass: "planning-row-ghost",
    // Hide the add button during drag to avoid it drifting with the row.
    onStart: () => {
      dragging.value = true;
      hideInsertBtn();
    },
    // The body's first row may be antd's hidden measurement row; use the draggable element's relative index only.
    onEnd: async ({ oldDraggableIndex, newDraggableIndex }) => {
      dragging.value = false;
      hideInsertBtn();
      const oldIndex = oldDraggableIndex;
      const newIndex = newDraggableIndex;
      if (oldIndex === newIndex) return;
      const moved = rows.value[oldIndex];
      const reordered = [...rows.value];
      reordered.splice(oldIndex, 1);
      reordered.splice(newIndex, 0, moved);
      rows.value = reordered;
      try {
        await movePlanning(props.wsId, moved.id, {
          group_id: Number(props.groupId),
          prev_id: reordered[newIndex - 1]?.id || 0,
          next_id: reordered[newIndex + 1]?.id || 0,
        });
      } catch {
        fetchRows(false);
      }
    },
  });
}

onMounted(async () => {
  window.addEventListener("keydown", handleUndo);
  document.addEventListener("mousedown", startTableResizeGuide, true);
  loadColumnWidths();
  await fetchRows(false);
  await nextTick();
  setupSortable();
  scrollEl = tableRef.value?.$el?.querySelector(".ant-table-body");
  scrollEl?.addEventListener("scroll", handleScroll);
  updateLayoutMetrics();
  if (wrapRef.value) {
    containerObserver = new ResizeObserver(() => updateLayoutMetrics());
    containerObserver.observe(wrapRef.value);
  }
  // New group: directly add a row and enter title-edit mode, avoiding another "add a row" click.
  if (props.autoAddRow && props.canEdit && rows.value.length === 0) {
    await insertRow(-1, "below");
  }
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleUndo);
  document.removeEventListener("mousedown", startTableResizeGuide, true);
  scrollEl?.removeEventListener("scroll", handleScroll);
  tableResizeGuideCleanup?.();
  window.cancelAnimationFrame(insertBtnFrame);
  containerObserver?.disconnect();
  containerObserver = null;
  sortableInstance?.destroy();
  sortableInstance = null;
});

// The toolbar create button always inserts at the top of the current list.
function addRow() {
  return insertRow(0, "above");
}

defineExpose({ refresh: () => fetchRows(false), addRow });
</script>

<style scoped lang="less">
.planning-table-wrap {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  position: relative;
}
// During drag sorting, keep only the drag handle; hide the row menu and relation buttons to avoid mis-clicks and interference.
.planning-table-wrap.is-dragging .row-more-btn,
.planning-table-wrap.is-dragging .relation-actions {
  visibility: hidden;
}
.planning-table {
  flex: 1;
  min-height: 0;
  background: #fff;
  border-radius: 8px;
  overflow: hidden;

  // Fill the container layer by layer so the table area fully fills the page (blank below when data is short, no grid drawn).
  :deep(.ant-spin-nested-loading),
  :deep(.ant-spin-container),
  :deep(.ant-table),
  :deep(.ant-table-container) {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }
  // scroll.y only sets max-height on the body; it won't expand on its own when data is short,
  // so rely on flex to fill the remaining space so the summary row stays pinned to the bottom.
  :deep(.ant-table-body) {
    flex: 1;
    min-height: 0;
  }

  // Row height 32px, single-line ellipsis; the index column needs overflow visible to host the floating insert buttons above/below.
  :deep(.planning-row td) {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-top: 0 !important;
    padding-bottom: 0 !important;
    height: 32px;
  }
  :deep(.planning-row td:first-child),
  :deep(.ant-table-cell-fix-left) {
    overflow: visible;
  }
  :deep(.ant-table-cell) {
    padding: 6px 8px !important;
  }
  :deep(.ant-table-thead > tr > th) {
    padding: 4px 8px !important;
    background: #fafafa;
    font-weight: 600;
  }
  // Editable cell: keep the same whole-cell click experience as the requirement/defect list.
  :deep(td.editable-table-cell) {
    cursor: pointer;
  }
  :deep(td.editable-table-cell > *) {
    width: 100%;
  }
  :deep(td.editable-table-cell .ant-select),
  :deep(td.editable-table-cell .ant-picker),
  :deep(td.editable-table-cell .ant-input-number) {
    width: 100% !important;
  }
  // Row hover background (incl. left fixed columns).
  :deep(.ant-table-tbody > tr.ant-table-row:hover > td),
  :deep(.ant-table-tbody > tr > td.ant-table-cell-row-hover) {
    background: #f3f4f7;
  }
  // Show the index column's drag/more actions on row hover.
  // All via CSS, no reactive state, otherwise every hover in/out re-renders the whole table.
  :deep(.ant-table-tbody > tr:hover) .row-actions {
    visibility: visible;
  }
  :deep(.ant-table-tbody > tr:hover) .row-index-number {
    visibility: hidden;
  }
}
.settings-btn {
  color: #8c8c8c;
  padding: 0;
}
.row-index-cell {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 20px;
}
.row-index-number {
  position: absolute;
  color: #bfbfbf;
  font-size: 13px;
  user-select: none;
}
.row-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  // Keep the placeholder in non-hover state to avoid index-column width jitter.
  visibility: hidden;
}
// The relation column's action icons keep a placeholder and only appear
// when the pointer is inside the relation cell itself (not merely the row).
.relation-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  visibility: hidden;
}
.relation-cell:hover .relation-actions {
  visibility: visible;
}
.drag-handle {
  cursor: grab;
  color: #8c8c8c;
  padding: 0 2px;

  &:active {
    cursor: grabbing;
  }
}
.row-more-btn {
  padding: 0 2px;
  color: #8c8c8c;

  &:hover {
    color: #1677ff;
  }
}
// The add button on row hover: center aligns to the row's top-left; enlarges and turns blue when hovering the button itself.
.insert-btn {
  position: absolute;
  display: none;
  align-items: center;
  justify-content: center;
  width: 13.13px;
  height: 13.13px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: #17181c;
  color: #fff;
  font-size: 8px;
  line-height: 1;
  cursor: pointer;
  // The anchor point is the row's top-left; shift by half so the button center coincides (stays centered when enlarged).
  transform: translate(-50%, -50%);
  // It must cover the sticky header and summary row, so its z-index is above the table internals.
  z-index: 100;
  transition: all 0.15s;

  // Visually just a 13px dot, but the hit area is expanded to 26px, otherwise it sits on the row edge and is nearly unclickable.
  &::before {
    content: "";
    position: absolute;
    inset: -6px;
  }

  &:hover {
    width: 16.41px;
    height: 16.41px;
    background: #256cfb;
    font-size: 10px;
  }
}
// Column-width drag guide line: darker on the header segment, lighter on the body segment.
.table-resize-guide {
  position: absolute;
  width: 1px;
  background: linear-gradient(
    to bottom,
    #1677ff 0,
    #1677ff var(--header-height),
    rgba(22, 119, 255, 0.35) var(--header-height),
    rgba(22, 119, 255, 0.35) 100%
  );
  pointer-events: none;
  z-index: 30;
}
.title-edit {
  display: flex;
  align-items: center;
  gap: 6px;
}
.title-display {
  display: flex;
  align-items: center;
  min-width: 0;
  // The double-click hot area covers the whole cell, so a new planning with an empty name can still enter edit on double-click.
  min-height: 24px;
  cursor: text;
}
.title-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
// Struck-through planning names drop to light grey, matching the prototype's "done" look.
.title-struck {
  text-decoration: line-through;
  color: #bfbfbf;
}
// Strike-through toggle sits next to the colour picker while the name is being edited.
.strike-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  border: 1px solid #d9d9d9;
  color: #595959;
  cursor: pointer;
  flex-shrink: 0;

  &.active {
    border-color: #1677ff;
    color: #1677ff;
  }
}
.color-trigger {
  width: 22px;
  height: 22px;
  border-radius: 4px;
  border: 1px solid #d9d9d9;
  cursor: pointer;
  flex-shrink: 0;
}
.color-grid {
  display: grid;
  grid-template-columns: repeat(4, 26px);
  gap: 6px;
}
.color-dot {
  width: 26px;
  height: 26px;
  border-radius: 4px;
  cursor: pointer;
  border: 1px solid #d9d9d9;
  box-sizing: border-box;

  &.selected {
    border: 2px solid #1677ff;
  }
}
.cell-value {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  width: 100%;
}
.cell-empty {
  color: #bfbfbf;
  font-size: 12px;
}
.cell-select,
.cell-date {
  width: 100%;
}
// Once a column is dragged narrow, the dropdown panel shrinks with the select and the status /
// priority tags inside the options get clipped. popup-match-select-width=false alone is not
// enough: antd's option container is overflow:hidden, so inline tags cannot push the panel wider
// and a minimum width is still needed as a floor.
:global(.planning-cell-dropdown) {
  min-width: 160px !important;
}
.relation-count {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  color: #1677ff;
  font-size: 13px;
  font-weight: 500;

  // Mid grey when there is no relation (same level as the delete icon), aligned with the icon
  // group that appears on hover.
  &.empty {
    color: #8c8c8c;
    font-weight: 400;
  }

  // Without work-item view permission the relation list cannot be opened, so drop the clickable hint.
  &.disabled {
    cursor: default;
  }
}
// The summary row is "Add a row", a toolbar rather than a data row, so drop the
// column grid (bordered table's border-inline-end and the left-fixed splitter).
:deep(.add-row-summary > td) {
  background: #fafafa;
  padding: 4px 8px !important;
  border-inline-end: none !important;
}
:deep(.add-row-summary > .ant-table-cell-fix-left-last::after) {
  display: none;
}
:deep(.add-row-summary > td:first-child) {
  min-width: 240px;
}
.add-row-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
// Grey at rest, blue on hover: these are secondary entry points and need not hold the primary
// colour permanently.
.add-row-btn {
  color: #8c8c8c;
  font-size: 13px;
  padding: 0 6px;

  &:hover {
    color: #1677ff;
  }
}
.cell-center {
  justify-content: center;
}
.relation-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.relation-icon {
  color: #1677ff;
  cursor: pointer;
  font-size: 13px;

  &.danger {
    color: #ff4d4f;
  }
  &.disabled {
    color: #d9d9d9;
    cursor: not-allowed;
  }
}
:deep(.planning-row-ghost) {
  opacity: 0.5;
  background: #e6f4ff;
}
</style>
