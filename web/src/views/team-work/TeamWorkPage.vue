<template>
  <div ref="pageRef" class="team-work-page">
    <header class="team-work-header">
      <span class="page-icon"><TeamOutlined /></span>
      <h3>{{ $t("performance_title") }}</h3>

      <a-tabs
        v-model:activeKey="activePeriod"
        class="period-tabs"
        @change="handlePeriodChange"
      >
        <a-tab-pane v-for="tab in availableTabs" :key="tab.key">
          <template #tab>
            <span class="tab-label">
              {{ $t(tab.label) }}
              <em
                v-if="
                  tab.key !== 'custom' && periodItems[tab.key] !== undefined
                "
              >
                {{ periodCounts[tab.key] }}
              </em>
            </span>
          </template>
        </a-tab-pane>
      </a-tabs>

      <PlannedDatePicker
        v-if="activePeriod === 'custom'"
        v-model:value="customDate"
        class="custom-date-picker"
        :allow-clear="false"
        format="YYYY-MM-DD"
        @change="fetchActivePeriod"
      />

      <div class="team-work-toolbar">
        <span class="toolbar-spacer" />
        <ViewDropdown
          :views="viewOptions"
          :current-view-id="activeViewId"
          :grouped="false"
          :show-actions="false"
          @select="handleViewSelect"
        />
        <a-button
          type="text"
          class="settings-button"
          @click="openMemberSettings"
        >
          <SettingOutlined /> {{ $t("performance_settings") }}
        </a-button>
      </div>
    </header>

    <a-spin v-if="loading" class="page-loading" />
    <TeamWorkKanban
      v-else-if="viewMode === 'board'"
      :columns="kanbanColumns"
      :selected-item-key="selectedItemKey"
      :member-colors="memberColors"
      @open="openDetail"
      @move="handleKanbanMove"
      @member-visibility-change="handleKanbanMemberVisibility"
      @member-color-change="handleKanbanMemberColor"
    />
    <div v-else-if="memberGroups.length" class="groups-scroll">
      <section
        v-for="group in memberGroups"
        :key="group.id"
        class="member-work-group"
      >
        <button
          type="button"
          class="group-toggle"
          @click="toggleGroup(group.id)"
        >
          <CaretRightFilled v-if="collapsedGroups.has(group.id)" />
          <CaretDownFilled v-else />
          <a-avatar
            :size="20"
            :src="group.avatar || undefined"
            class="group-avatar"
            :class="{ 'group-avatar-empty': !group.avatar }"
          >
            {{ memberInitial(group) }}
          </a-avatar>
          <strong>{{ group.name }} ({{ group.items.length }})</strong>
        </button>

        <a-table
          v-if="!collapsedGroups.has(group.id)"
          class="team-work-table"
          row-key="id"
          :columns="columns"
          :data-source="group.items"
          :pagination="false"
          :scroll="{ x: tableScrollWidth }"
          size="small"
          table-layout="fixed"
          :custom-row="rowProps"
          :row-class-name="rowClassName"
          @resizeColumn="handleResizeColumn"
        >
          <template #headerCell="{ title, column }">
            <WorkItemColumnHeader
              :title="title"
              :column="column"
              :sort="listSort"
              :drag="columnDragState"
              @sort="cycleColumnSort"
              @dragstart="startColumnDrag"
              @dragover="updateColumnDropTarget"
              @drop="finishColumnDrop"
              @dragend="clearColumnDrag"
            />
          </template>
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'title'">
              <div class="title-cell" @click.stop>
                <!-- Reading the attributes config requires workspace membership, so a row
                     the current user cannot view would answer 403; it keeps the legacy label. -->
                <WorkItemTypePill
                  :type="record.type"
                  :ws-id="canView(record) ? record.workspace_id : ''"
                />
                <a-input
                  v-if="isEditing(record, 'title')"
                  v-model:value="titleDraft"
                  size="small"
                  class="title-input"
                  autofocus
                  @press-enter="saveTitle(record)"
                  @blur="saveTitle(record)"
                />
                <a
                  v-else-if="canClickDetail(record)"
                  :title="record.title"
                  :href="
                    router.resolve({
                      name: 'WorkItemDetail',
                      params: { wsId: record.workspace_id, id: record.id },
                    }).href
                  "
                  @click.prevent="openDetail(record)"
                  >{{ record.title }}</a
                >
                <strong v-else :title="record.title">{{ record.title }}</strong>
                <a-tooltip
                  v-if="canEdit(record) && !isEditing(record, 'title')"
                  :title="$t('common_edit')"
                >
                  <EditOutlined
                    class="edit-title"
                    @click="startTitleEdit(record)"
                  />
                </a-tooltip>
              </div>
            </template>
            <template v-else-if="column.key === 'workspace'">
              <span class="workspace-name" :title="record.workspace_name">
                {{ record.workspace_name || "-" }}
              </span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-select
                v-if="isEditing(record, 'status_id')"
                :value="Number(record.status_id)"
                :options="statusOptions(record)"
                size="small"
                default-open
                autofocus
                style="width: 100px"
                @change="(value) => saveField(record, 'status_id', value)"
                @blur="editing = null"
                @click.stop
              />
              <span
                v-else
                :class="{ clickable: canEdit(record) }"
                @click.stop="startEdit(record, 'status_id')"
              >
                <StatusPill
                  :text="record.status_name"
                  :color="record.status_color"
                />
              </span>
            </template>
            <template v-else-if="column.key === 'priority'">
              <a-select
                v-if="isEditing(record, 'priority')"
                :value="Number(record.priority) || undefined"
                :options="priorityOptions(record)"
                size="small"
                allowClear
                default-open
                autofocus
                style="width: 90px"
                @change="
                  (value) => saveField(record, 'priority', value ?? null)
                "
                @blur="editing = null"
                @click.stop
              />
              <span
                v-else
                :class="{ clickable: canEdit(record) }"
                @click.stop="startEdit(record, 'priority')"
              >
                <PriorityPill
                  :priority="record.priority"
                  :label="record.priority_name"
                  :color="record.priority_color"
                />
              </span>
            </template>
            <template
              v-else-if="
                column.key === 'planned_start_date' ||
                column.key === 'planned_end_date'
              "
            >
              <PlannedDatePicker
                v-if="isEditing(record, column.key)"
                :value="plannedDateValue(record[column.key])"
                :default-picker-value="dayjs()"
                size="small"
                open
                autofocus
                format="YYYY-MM-DD"
                style="width: 108px"
                @change="
                  (value) =>
                    saveField(
                      record,
                      column.key,
                      value ? value.startOf('day').unix() : 0,
                    )
                "
                @open-change="(open) => !open && (editing = null)"
                @click.stop
              />
              <span
                v-else
                :class="{ clickable: canEdit(record) }"
                @click.stop="startEdit(record, column.key)"
              >
                {{
                  record[column.key] ? formatLocalDate(record[column.key]) : "-"
                }}
              </span>
            </template>
            <template v-else-if="column.key === 'assignee'">
              <AssigneeSelect
                v-if="isEditing(record, 'assignee_ids')"
                :value="assigneeIdsOf(record)"
                :members="membersOf(record)"
                style="width: 180px"
                @save="(value) => saveAssignees(record, value)"
                @cancel="editing = null"
                @click.stop
              />
              <span
                v-else
                class="assignee-cell-wrap"
                :class="{ clickable: canEdit(record) }"
                @click.stop="startEdit(record, 'assignee_ids')"
              >
                <AssigneeCell
                  :ids="record.assignee_ids"
                  :names="record.assignee_names"
                  :avatars="record.assignee_avatars"
                  :usernames="record.assignee_usernames"
                />
              </span>
            </template>
          </template>
        </a-table>
      </section>
    </div>
    <a-empty v-else :description="$t('common_noData')" class="page-empty" />

    <TeamWorkSettingsModal
      v-model:open="settingsOpen"
      :preview-mode="previewMode"
      :view-mode="viewMode"
      :members="memberSettings"
      :loading="settingsLoading"
      :saving="settingsSaving"
      @preview-mode-change="handlePreviewModeChange"
      @view-mode-change="handleViewModeChange"
      @members-change="handleMembersChange"
    />

    <StatusTransitionModal
      v-model:open="transitionOpen"
      :ws-id="transitionItem?.workspace_id"
      :work-item="transitionItem"
      @success="handleStatusTransitionSuccess"
    />

    <RequirementDetailDrawer
      :open="!!detailItem"
      :ws-id="detailItem?.workspace_id"
      :requirement-id="detailRequirementId"
      :can-edit="canEdit(detailItem)"
      :mode="previewMode"
      :navigation-items="teamWorkNavigationItems"
      @close="closeDetail"
      @navigate="handleDetailNavigate"
      @updated="handleDetailUpdated"
    />
  </div>
</template>

<script setup>
defineOptions({ name: "TeamWorkPage" });

import {
  computed,
  watch,
  nextTick,
  onMounted,
  onActivated,
  onBeforeUnmount,
  reactive,
  ref,
} from "vue";
import { useI18n } from "vue-i18n";
import dayjs from "dayjs";
import { message } from "ant-design-vue";
import {
  CaretDownFilled,
  CaretRightFilled,
  EditOutlined,
  SettingOutlined,
  TeamOutlined,
} from "@ant-design/icons-vue";
import { useRouter } from "vue-router";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import StatusTransitionModal from "@/components/common/StatusTransitionModal.vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import ViewDropdown from "@/components/work-item/ViewDropdown.vue";
import { formatLocalDate, plannedDateCoversDay } from "@/utils/date";
import { buildWorkItemListUpdate, mergeWorkItem } from "@/utils/workItemList";
import { isStatusTransitionEnabled } from "@/utils/statusTransition";
import {
  getRequirement,
  getWorkspaceSettings,
  listMembers,
  listPriorities,
  listStatuses,
  listTeamWork,
  listTeamWorkMembers,
  updateRequirement,
  updateTeamWorkMembers,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useContactPreferencesStore } from "@/stores/contactPreferences";
import { setSecondaryKey } from "@/composables/usePageTitle";
import { getRememberedTab, setRememberedTab } from "@/utils/rememberedTab";
import {
  readGlobalListPreferences,
  readBoardColors,
  readTeamWorkPreferences,
  GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
  writeGlobalListPreferences,
  writeTeamWorkPreferences,
} from "@/utils/boardPreferences";
import { saveWorkItemNavigationContext } from "@/utils/workItemNavigation";
import WorkItemColumnHeader from "@/components/common/WorkItemColumnHeader.vue";
import { useListColumnLayout } from "@/composables/useListColumnLayout";
import TeamWorkKanban from "./components/TeamWorkKanban.vue";
import TeamWorkSettingsModal from "./components/TeamWorkSettingsModal.vue";

const { t } = useI18n();
const router = useRouter();
const authStore = useAuthStore();
const contactPreferences = useContactPreferencesStore();

const storedGlobalPreferences = readGlobalListPreferences(
  authStore.userInfo?.id,
);
const storedTeamWorkPreferences = readTeamWorkPreferences(
  authStore.userInfo?.id,
);
// Migrate the legacy team-work-only view mode into the shared list preference.
const storedViewMode = ["list", "board"].includes(
  storedGlobalPreferences.viewMode,
)
  ? storedGlobalPreferences.viewMode
  : ["list", "board"].includes(storedTeamWorkPreferences.viewMode)
    ? storedTeamWorkPreferences.viewMode
    : "list";
if (
  !["list", "board"].includes(storedGlobalPreferences.viewMode) &&
  storedViewMode !== "list"
) {
  writeGlobalListPreferences(authStore.userInfo?.id, {
    viewMode: storedViewMode,
  });
}
const previewMode = ref(
  ["drawer", "modal", "window"].includes(storedGlobalPreferences.previewMode)
    ? storedGlobalPreferences.previewMode
    : "drawer",
);
const viewMode = ref(storedViewMode);
const memberColors = ref(
  readBoardColors(storedTeamWorkPreferences.boardColors),
);
// Column width, order and sort of the list, kept in the page preferences.
const {
  listSort,
  resolveColumns,
  handleResizeColumn,
  moveColumn,
  cycleColumnSort,
  sortItems,
  tableWidth,
} = useListColumnLayout({
  userId: authStore.userInfo?.id,
  readPreferences: readTeamWorkPreferences,
  writePreferences: writeTeamWorkPreferences,
});

const periodDefinitions = [
  {
    key: "today",
    label: "performance_tabToday",
    permission: "team_work:view_today",
  },
  {
    key: "yesterday",
    label: "performance_tabYesterday",
    permission: "team_work:view_yesterday",
  },
  {
    key: "tomorrow",
    label: "performance_tabTomorrow",
    permission: "team_work:view_tomorrow",
  },
  {
    key: "unscheduled",
    label: "performance_tabUnscheduled",
    permission: "team_work:view_unscheduled",
  },
  {
    key: "scheduled",
    label: "performance_tabScheduled",
    permission: "team_work:view_scheduled",
  },
  {
    key: "custom",
    label: "performance_tabCustom",
    permission: "team_work:view_custom_date",
  },
];

const availableTabs = computed(() =>
  periodDefinitions.filter((tab) =>
    authStore.hasEnterprisePermission(tab.permission),
  ),
);
// The team-work cycle tab's secondary-menu title (i18n key).
const periodTitle = {
  today: "performance_tabToday",
  yesterday: "performance_tabYesterday",
  tomorrow: "performance_tabTomorrow",
  unscheduled: "performance_tabUnscheduled",
  scheduled: "performance_tabScheduled",
  custom: "performance_tabCustom",
};
const activePeriod = ref(
  getRememberedTab(
    authStore.userInfo?.id,
    "team-work",
    availableTabs.value.map((tab) => tab.key),
    availableTabs.value[0]?.key || "today",
  ),
);
watch(activePeriod, (k) => setSecondaryKey(periodTitle[k] || ""), {
  immediate: true,
});
const customDate = ref(dayjs());
const periodItems = ref({});
const loading = ref(false);
const collapsedGroups = ref(new Set());
const settingsOpen = ref(false);
const settingsLoading = ref(false);
const settingsSaving = ref(false);
const memberSettings = ref([]);
const periodRequestIds = new Map();
let memberSettingsRequestId = 0;
let activationCount = 0;
const editing = ref(null);
const titleDraft = ref("");
const optionCache = ref({});
const transitionOpen = ref(false);
const transitionItem = ref(null);
const detailItem = ref(null);
const detailRequirementId = ref(null);
const pageRef = ref(null);
let settingsSaveChain = Promise.resolve();
let detailRowScrollFrame;

function boolValue(value) {
  return value === true || value === 1 || value === "1" || value === "true";
}

function canView(item) {
  return !!item && boolValue(item.can_view);
}

function canOpenDetail(item) {
  return !!item && boolValue(item.can_open_detail);
}

function canClickDetail(item) {
  // Missing project permissions still route to the access application page or 403.
  return !!item && (!canView(item) || canOpenDetail(item));
}

function canEdit(item) {
  return !!item && boolValue(item.can_edit);
}

function isDone(item) {
  return boolValue(item?.is_done);
}

const activeItems = computed(() => periodItems.value[activePeriod.value] || []);
// Builtin team-work views; the ids reuse the backend builtin view keys.
const TEAM_WORK_VIEWS = [
  { id: "all", nameKey: "view_builtin_all" },
  { id: "my_created", nameKey: "view_builtin_my_created" },
  { id: "unfinished", nameKey: "view_builtin_unfinished" },
  { id: "finished", nameKey: "view_builtin_finished" },
];
const TEAM_WORK_DEFAULT_VIEW = "all";
// Cycle tabs that open on a filtered view instead of showing everything.
const TEAM_WORK_VIEW_BY_PERIOD = { scheduled: "unfinished" };
const periodViews = ref({ ...(storedTeamWorkPreferences.views || {}) });

function defaultViewFor(period) {
  return TEAM_WORK_VIEW_BY_PERIOD[period] || TEAM_WORK_DEFAULT_VIEW;
}
function viewIdFor(period) {
  const remembered = periodViews.value[period];
  return TEAM_WORK_VIEWS.some((view) => view.id === remembered)
    ? remembered
    : defaultViewFor(period);
}
const activeViewId = computed(() => viewIdFor(activePeriod.value));
// Tab counts follow each period's selected view, matching the filtered list.
const periodCounts = computed(() => {
  const counts = {};
  for (const [period, items] of Object.entries(periodItems.value)) {
    counts[period] = items.filter((item) =>
      matchesView(item, viewIdFor(period)),
    ).length;
  }
  return counts;
});
const viewOptions = computed(() =>
  TEAM_WORK_VIEWS.map((view) => ({
    id: view.id,
    name: t(view.nameKey),
    builtin_key: view.id,
    view_type: "system",
    enabled: true,
    is_default: defaultViewFor(activePeriod.value) === view.id,
  })),
);
function matchesView(item, viewId) {
  if (viewId === "my_created") {
    return Number(item.created_by) === Number(authStore.userInfo?.id);
  }
  if (viewId === "unfinished") return !isDone(item);
  if (viewId === "finished") return isDone(item);
  return true;
}
// The view only filters what is displayed; tab counts apply the same filter.
const viewFilteredItems = computed(() =>
  activeItems.value.filter((item) => matchesView(item, activeViewId.value)),
);
function handleViewSelect(viewId) {
  periodViews.value = { ...periodViews.value, [activePeriod.value]: viewId };
  writeTeamWorkPreferences(authStore.userInfo?.id, {
    views: periodViews.value,
  });
}
const memberGroups = computed(() => {
  const currentOrder = new Map(
    memberSettings.value.map((member, index) => [String(member.id), index]),
  );
  const groups = new Map();
  for (const item of viewFilteredItems.value) {
    const id = String(item.assignee_id);
    if (!groups.has(id)) {
      groups.set(id, {
        id,
        name: item.assignee_name || item.assignee_username || "-",
        avatar: item.assignee_avatar || "",
        sortOrder: Number(item.member_sort_order) || Number.MAX_SAFE_INTEGER,
        items: [],
      });
    }
    groups.get(id).items.push(item);
  }
  return (
    [...groups.values()]
      .sort((left, right) => {
        const leftOrder = currentOrder.get(left.id) ?? left.sortOrder;
        const rightOrder = currentOrder.get(right.id) ?? right.sortOrder;
        return leftOrder - rightOrder || Number(left.id) - Number(right.id);
      })
      // Sorting stays inside a member group; the member order above is untouched.
      .map((group) => ({
        ...group,
        items: sortItems(group.items, columns.value),
      }))
  );
});
const kanbanColumns = computed(() =>
  memberSettings.value
    .filter((member) => member.visible)
    .map((member) => ({
      ...member,
      items: viewFilteredItems.value.filter(
        (item) => Number(item.assignee_id) === Number(member.id),
      ),
    })),
);
const selectedItemKey = computed(() =>
  detailItem.value
    ? `${detailItem.value.workspace_id}:${detailItem.value.id}`
    : "",
);
const teamWorkNavigationItems = computed(() =>
  uniqueNavigationItems(
    (viewMode.value === "board"
      ? kanbanColumns.value.flatMap((column) => column.items)
      : memberGroups.value
          .filter((group) => !collapsedGroups.value.has(group.id))
          .flatMap((group) => group.items)
    )
      .filter(canOpenDetail)
      .map((item) => ({
        id: item.id,
        workspaceId: item.workspace_id,
      })),
  ),
);

function uniqueNavigationItems(items) {
  const seen = new Set();
  return items.filter((item) => {
    const key = `${item.workspaceId}:${item.id}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

const columnDefinitions = computed(() => [
  {
    title: t("workspace_workItemTitle"),
    key: "title",
    width: 430,
    minWidth: 240,
    maxWidth: 1000,
    ellipsis: true,
  },
  {
    title: t("performance_project"),
    key: "workspace",
    width: 160,
    minWidth: 120,
    maxWidth: 400,
    ellipsis: true,
  },
  {
    title: t("workspace_status"),
    key: "status",
    width: 110,
    minWidth: 96,
    maxWidth: 300,
    sortField: "status",
    customCell: (record) => editableCellProps(record, "status_id"),
  },
  {
    title: t("workspace_priority"),
    key: "priority",
    width: 100,
    minWidth: 96,
    maxWidth: 300,
    sortField: "priority",
    customCell: (record) => editableCellProps(record, "priority"),
  },
  {
    title: t("workspace_plannedStartDateCol"),
    key: "planned_start_date",
    width: 120,
    minWidth: 110,
    maxWidth: 260,
    sortField: "planned_start_date",
    customCell: (record) => editableCellProps(record, "planned_start_date"),
  },
  {
    title: t("workspace_plannedEndDateCol"),
    key: "planned_end_date",
    width: 120,
    minWidth: 110,
    maxWidth: 260,
    sortField: "planned_end_date",
    customCell: (record) => editableCellProps(record, "planned_end_date"),
  },
  {
    title: t("workspace_assignee"),
    key: "assignee",
    width: 120,
    minWidth: 110,
    maxWidth: 300,
    customCell: (record) => editableCellProps(record, "assignee_ids"),
  },
]);
const columns = computed(() => resolveColumns(columnDefinitions.value));
const tableScrollWidth = computed(() => tableWidth(columns.value));

const columnDragState = reactive({ sourceKey: "", targetKey: "", side: "" });

function startColumnDrag(event, column) {
  columnDragState.sourceKey = column.key;
}

function updateColumnDropTarget(event, column) {
  if (!columnDragState.sourceKey) return;
  const rect = event.currentTarget.getBoundingClientRect();
  columnDragState.targetKey = column.key;
  columnDragState.side =
    event.clientX < rect.left + rect.width / 2 ? "before" : "after";
}

function clearColumnDrag() {
  columnDragState.sourceKey = "";
  columnDragState.targetKey = "";
  columnDragState.side = "";
}

function finishColumnDrop(event, column) {
  const sourceKey = columnDragState.sourceKey;
  const side = columnDragState.side || "before";
  clearColumnDrag();
  if (!sourceKey || sourceKey === column.key) return;
  moveColumn(
    sourceKey,
    column.key,
    side,
    columns.value.map((item) => item.key),
  );
}

async function fetchPeriod(period) {
  const requestId = (periodRequestIds.get(period) || 0) + 1;
  periodRequestIds.set(period, requestId);
  const params = { period };
  if (period === "custom") {
    params.date = customDate.value.format("YYYY-MM-DD");
  }
  const result = (await listTeamWork(params)) || {};
  if (periodRequestIds.get(period) !== requestId) return;
  periodItems.value = {
    ...periodItems.value,
    [period]: result.list || [],
  };
}

async function refreshLoadedPeriods() {
  await Promise.all(Object.keys(periodItems.value).map(fetchPeriod));
  if (!detailItem.value) return;
  const updatedItem = Object.values(periodItems.value)
    .flat()
    .find(
      (item) =>
        String(item.id) === String(detailItem.value.id) &&
        String(item.workspace_id) === String(detailItem.value.workspace_id) &&
        String(item.assignee_id) === String(detailItem.value.assignee_id),
    );
  if (updatedItem) detailItem.value = updatedItem;
}

async function loadMemberSettings() {
  const requestId = ++memberSettingsRequestId;
  const list = (await listTeamWorkMembers()) || [];
  if (requestId !== memberSettingsRequestId) return;
  memberSettings.value = list.map((member) => ({
    id: Number(member.id),
    name: member.name || member.username || "-",
    username: member.username || "",
    avatar: member.avatar || "",
    visible: Number(member.is_visible) === 1,
  }));
}

function periodDate(period) {
  if (period === "yesterday") return dayjs().subtract(1, "day");
  if (period === "tomorrow") return dayjs().add(1, "day");
  if (period === "custom") return customDate.value;
  return dayjs();
}

function matchesTeamWorkPeriod(item, period) {
  if (period === "scheduled") {
    return (
      Number(item.planned_start_date) > 0 || Number(item.planned_end_date) > 0
    );
  }
  if (period === "unscheduled") {
    return (
      !Number(item.planned_start_date) &&
      !Number(item.planned_end_date) &&
      !boolValue(item.is_done)
    );
  }
  return plannedDateCoversDay(
    item.planned_start_date,
    item.planned_end_date,
    periodDate(period).startOf("day").unix(),
  );
}

function teamWorkRows(item, previousRows) {
  const previousByAssignee = new Map(
    previousRows.map((row) => [Number(row.assignee_id), row]),
  );
  return (item.assignee_ids || []).map(Number).flatMap((assigneeId) => {
    const member = memberSettings.value.find(
      (candidate) => Number(candidate.id) === assigneeId,
    );
    const previous = previousByAssignee.get(assigneeId);
    if (!previous && (!member || !member.visible)) return [];
    const row = mergeWorkItem(
      previous ? { ...previous } : { ...previousRows[0] },
      item,
    );
    const index = item.assignee_ids.map(Number).indexOf(assigneeId);
    row.assignee_id = assigneeId;
    row.assignee_name = item.assignee_names?.[index] || "";
    row.assignee_username = item.assignee_usernames?.[index] || "";
    row.assignee_avatar = item.assignee_avatars?.[index] || "";
    row.member_sort_order = member
      ? memberSettings.value.indexOf(member) + 1
      : previous?.member_sort_order || Number.MAX_SAFE_INTEGER;
    return [row];
  });
}

function replaceTeamWorkRows(rows, updatedRows, item) {
  const updatedByAssignee = new Map(
    updatedRows.map((row) => [Number(row.assignee_id), row]),
  );
  const nextRows = [];
  for (const row of rows) {
    const isTarget =
      String(row.id) === String(item.id) &&
      String(row.workspace_id) === String(item.workspace_id);
    if (!isTarget) {
      nextRows.push(row);
      continue;
    }
    const updated = updatedByAssignee.get(Number(row.assignee_id));
    if (updated) {
      nextRows.push(updated);
      updatedByAssignee.delete(Number(row.assignee_id));
    }
  }
  return [...nextRows, ...updatedByAssignee.values()];
}

async function handleDetailUpdated(change) {
  if (!change || change.action === "refresh") {
    await refreshLoadedPeriods();
    return;
  }
  const selected =
    detailItem.value &&
    String(detailItem.value.id) === String(change.id) &&
    String(detailItem.value.workspace_id) === String(change.workspaceId);
  const previousRows = Object.values(periodItems.value)
    .flat()
    .filter(
      (row) =>
        String(row.id) === String(change.id) &&
        String(row.workspace_id) === String(change.workspaceId),
    );
  const source = selected ? detailItem.value : previousRows[0];
  if (!source) return;
  const updatedDetailItem = mergeWorkItem({ ...source }, change.item);
  const nextPeriods = { ...periodItems.value };
  Object.entries(nextPeriods).forEach(([period, rows]) => {
    const replacementRows = matchesTeamWorkPeriod(updatedDetailItem, period)
      ? teamWorkRows(updatedDetailItem, previousRows)
      : [];
    // Keep the current row positions for every inline/detail update. New rows
    // (for example, newly added assignees) are appended after existing rows.
    nextPeriods[period] = replaceTeamWorkRows(
      rows,
      replacementRows,
      updatedDetailItem,
    );
  });
  periodItems.value = nextPeriods;
  if (selected) detailItem.value = updatedDetailItem;
}

async function fetchActivePeriod() {
  loading.value = true;
  try {
    await fetchPeriod(activePeriod.value);
  } finally {
    loading.value = false;
  }
}

async function handlePeriodChange(period) {
  setRememberedTab(authStore.userInfo?.id, "team-work", period);
  editing.value = null;
  if (
    activePeriod.value === "custom" ||
    !periodItems.value[activePeriod.value]
  ) {
    await fetchActivePeriod();
  }
}

function cacheKey(item) {
  return `${item.workspace_id}:${item.type}`;
}

async function ensureOptions(item, field) {
  const key = cacheKey(item);
  const cached = optionCache.value[key] || {};
  if (field === "status_id") {
    const loadStatuses = !cached.statuses;
    const loadSettings = cached.finalStatusIds === undefined;
    const loadTransitions = !Array.isArray(item.available_status_ids);
    const [statuses, settings, detail] = await Promise.all([
      loadStatuses
        ? listStatuses(item.workspace_id, item.type)
        : Promise.resolve(null),
      loadSettings
        ? getWorkspaceSettings(item.workspace_id)
        : Promise.resolve(null),
      loadTransitions
        ? getRequirement(item.workspace_id, item.id)
        : Promise.resolve(null),
    ]);
    if (loadStatuses) cached.statuses = statuses || [];
    if (loadSettings) {
      cached.settings = settings || {};
      cached.finalStatusIds = settings?.[`${item.type}_final_status_ids`] || [];
      cached.completedStatusIds =
        settings?.[`${item.type}_completed_status_ids`] || [];
    }
    if (loadTransitions)
      item.available_status_ids = detail?.available_status_ids || [];
  }
  if (field === "priority" && !cached.priorities)
    cached.priorities =
      (await listPriorities(item.workspace_id, item.type)) || [];
  if (field === "assignee_ids" && !cached.members)
    cached.members = (await listMembers(item.workspace_id)) || [];
  optionCache.value[key] = cached;
}

function statusOptions(item) {
  const all = optionCache.value[cacheKey(item)]?.statuses || [];
  const ids = new Set((item.available_status_ids || []).map(Number));
  ids.add(Number(item.status_id));
  return all
    .filter(
      (option) => !item.available_status_ids || ids.has(Number(option.id)),
    )
    .map((option) => ({ value: Number(option.id), label: option.name }));
}

function priorityOptions(item) {
  return (optionCache.value[cacheKey(item)]?.priorities || []).map(
    (option) => ({
      value: Number(option.id),
      label: option.name || option.label,
    }),
  );
}

function membersOf(item) {
  return optionCache.value[cacheKey(item)]?.members || [];
}

function editingRowKey(item) {
  return `${item.id}:${item.assignee_id}`;
}

function isEditing(item, field) {
  return (
    editing.value?.rowKey === editingRowKey(item) &&
    editing.value?.field === field
  );
}

function editableCellProps(item, field) {
  if (!canEdit(item)) return {};
  return {
    class: "editable-table-cell",
    onClick: (event) => {
      event.stopPropagation();
      startEdit(item, field);
    },
  };
}

function plannedDateValue(value) {
  const timestamp = Number(value);
  return timestamp > 0 ? dayjs.unix(timestamp) : null;
}

async function startEdit(item, field) {
  if (!canEdit(item)) return;
  await ensureOptions(item, field);
  if (
    field === "status_id" &&
    isStatusTransitionEnabled(
      optionCache.value[cacheKey(item)]?.settings,
      item.type,
    )
  ) {
    transitionItem.value = item;
    transitionOpen.value = true;
    return;
  }
  editing.value = { rowKey: editingRowKey(item), field };
}

async function handleStatusTransitionSuccess(patch) {
  const item = transitionItem.value;
  if (!item) return;
  await handleDetailUpdated({
    action: "update",
    id: item.id,
    workspaceId: item.workspace_id,
    item: patch,
  });
}

function startTitleEdit(item) {
  titleDraft.value = item.title;
  editing.value = { rowKey: editingRowKey(item), field: "title" };
}

async function saveTitle(item) {
  if (!isEditing(item, "title")) return;
  const title = titleDraft.value.trim();
  editing.value = null;
  if (title && title !== item.title) await saveField(item, "title", title);
}

function assigneeIdsOf(record) {
  return Array.isArray(record.assignee_ids)
    ? record.assignee_ids.map(Number)
    : [];
}

async function saveField(item, field, value, opts = {}) {
  // keepEditing: do not exit edit mode (used for assignee change-to-save so
  // the dropdown stays open while saving instead of unmounting the component).
  if (!opts.keepEditing) editing.value = null;
  const updates = { [field]: value };
  await updateRequirement(item.workspace_id, item.id, updates);
  const cached = optionCache.value[cacheKey(item)] || {};
  const updated = buildWorkItemListUpdate(item, updates, {
    statuses: cached.statuses,
    priorities: cached.priorities,
    members: cached.members,
    finalStatusIds: cached.finalStatusIds,
    completedStatusIds: cached.completedStatusIds,
    updatedAt: dayjs().unix(),
  });
  await handleDetailUpdated({
    action: "update",
    id: item.id,
    workspaceId: item.workspace_id,
    changedFields: Object.keys(updates),
    item: updated,
  });
}

// Save assignees on every change (AssigneeSelect "change-to-save" mode).
// Keep the edit session open so the dropdown stays mounted while saving;
// it ends when the user clicks outside (handled by @cancel -> editing = null).
// Recent-contact ordering is deferred to dropdown close inside AssigneeSelect.
async function saveAssignees(item, value) {
  await saveField(item, "assignee_ids", value, { keepEditing: true });
}

function rowProps(item) {
  return canClickDetail(item) ? { onClick: () => openDetail(item) } : {};
}

function rowClassName(item) {
  return [
    canClickDetail(item) ? "viewable-row" : "",
    isDone(item) ? "row-done" : "",
    isDetailSelected(item) ? "detail-selected-row" : "",
  ]
    .filter(Boolean)
    .join(" ");
}

function isDetailSelected(item) {
  return (
    detailItem.value &&
    String(item.id) === String(detailRequirementId.value) &&
    String(item.workspace_id) === String(detailItem.value.workspace_id) &&
    String(item.assignee_id) === String(detailItem.value.assignee_id)
  );
}

function scrollSelectedDetailRowIntoView() {
  if (
    !detailRequirementId.value ||
    previewMode.value !== "drawer" ||
    viewMode.value !== "list"
  )
    return;
  if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
  nextTick(() => {
    detailRowScrollFrame = requestAnimationFrame(() => {
      detailRowScrollFrame = undefined;
      pageRef.value
        ?.querySelector(".team-work-table .detail-selected-row")
        ?.scrollIntoView({ block: "nearest", inline: "nearest" });
    });
  });
}

watch(
  [
    detailRequirementId,
    () => detailItem.value?.workspace_id,
    () => detailItem.value?.assignee_id,
    viewFilteredItems,
    previewMode,
    viewMode,
  ],
  scrollSelectedDetailRowIntoView,
  { flush: "post" },
);

function openDetail(item) {
  if (!canClickDetail(item)) return;
  if (!canView(item)) {
    const target = router.resolve({
      name: "WorkItemDetail",
      params: { wsId: item.workspace_id, id: item.id },
    });
    if (previewMode.value === "window")
      window.open(target.href, "_blank", "noopener,noreferrer");
    else router.push(target.href);
    return;
  }
  if (previewMode.value === "window") {
    const token = saveWorkItemNavigationContext(teamWorkNavigationItems.value);
    const query = token ? { navContext: token } : {};
    const target = router.resolve({
      name: "WorkItemDetail",
      params: { wsId: item.workspace_id, id: item.id },
      query,
    });
    window.open(target.href, "_blank", "noopener,noreferrer");
    return;
  }
  detailItem.value = item;
  detailRequirementId.value = item.id;
}

function handleDetailNavigate(id, workspaceId) {
  const targetWorkspaceId = workspaceId || detailItem.value?.workspace_id;
  const candidates = viewFilteredItems.value.filter(
    (item) =>
      Number(item.id) === Number(id) &&
      Number(item.workspace_id) === Number(targetWorkspaceId),
  );
  const target =
    candidates.find(
      (item) =>
        String(item.assignee_id) === String(detailItem.value?.assignee_id),
    ) || candidates[0];
  if (target && !canOpenDetail(target)) return;
  if (target) detailItem.value = target;
  else if (Number(targetWorkspaceId) !== Number(detailItem.value?.workspace_id))
    return;
  detailRequirementId.value = id;
}

function closeDetail() {
  detailItem.value = null;
  detailRequirementId.value = null;
}

function toggleGroup(id) {
  const next = new Set(collapsedGroups.value);
  next.has(id) ? next.delete(id) : next.add(id);
  collapsedGroups.value = next;
}

function memberInitial(member) {
  return (member.name || member.username || "?").trim().charAt(0).toUpperCase();
}

async function openMemberSettings() {
  settingsOpen.value = true;
  settingsLoading.value = true;
  try {
    await settingsSaveChain.catch(() => {});
    await loadMemberSettings();
  } finally {
    settingsLoading.value = false;
  }
}

function handlePreviewModeChange(mode) {
  if (!["drawer", "modal", "window"].includes(mode)) return;
  previewMode.value = mode;
  writeGlobalListPreferences(authStore.userInfo?.id, { previewMode: mode });
  if (mode === "window") closeDetail();
}

function handleViewModeChange(mode) {
  if (!["list", "board"].includes(mode)) return;
  viewMode.value = mode;
  editing.value = null;
  writeGlobalListPreferences(authStore.userInfo?.id, { viewMode: mode });
}

function handleSharedListPreferencesChange(event) {
  if (String(event.detail?.userId) !== String(authStore.userInfo?.id)) return;
  const patch = event.detail?.patch || {};
  if (["drawer", "modal", "window"].includes(patch.previewMode)) {
    previewMode.value = patch.previewMode;
    if (patch.previewMode === "window") closeDetail();
  }
  if (["list", "board"].includes(patch.viewMode)) {
    viewMode.value = patch.viewMode;
    editing.value = null;
  }
}

function handleMembersChange(members) {
  memberSettings.value = members.map((member) => ({ ...member }));
  queueMemberSettingsSave();
}

function handleKanbanMemberVisibility({ id, visible }) {
  handleMembersChange(
    memberSettings.value.map((member) =>
      Number(member.id) === Number(id) ? { ...member, visible } : member,
    ),
  );
}

function handleKanbanMemberColor({ id, color }) {
  const next = { ...memberColors.value };
  if (color) next[String(id)] = color;
  else delete next[String(id)];
  memberColors.value = readBoardColors(next);
  writeTeamWorkPreferences(authStore.userInfo?.id, {
    boardColors: memberColors.value,
  });
}

async function handleKanbanMove({ item, sourceMemberId, targetMemberId }) {
  if (!canEdit(item)) return;
  try {
    await ensureOptions(item, "assignee_ids");
    const projectMembers = membersOf(item);
    const targetMember = projectMembers.find(
      (member) => Number(member.user_id) === Number(targetMemberId),
    );
    if (!targetMember) {
      message.warning(t("performance_memberNotInProject"));
      return;
    }
    const currentIds = assigneeIdsOf(item);
    const nextIds = currentIds.filter(
      (id) => Number(id) !== Number(sourceMemberId),
    );
    if (!nextIds.some((id) => Number(id) === Number(targetMemberId))) {
      nextIds.push(Number(targetMemberId));
    }
    if (currentIds.join(",") === nextIds.join(",")) return;
    await saveField(item, "assignee_ids", nextIds);
  } catch {
    // The request interceptor reports the failure; Sortable has already restored the card.
  }
}

function queueMemberSettingsSave() {
  const members = memberSettings.value.map((member) => ({
    userId: member.id,
    visible: member.visible,
  }));
  settingsSaving.value = true;
  // Commit the full snapshot serially to prevent earlier requests from overwriting the latest order when toggling or dragging in quick succession.
  const saveTask = settingsSaveChain
    .catch(() => {})
    .then(async () => {
      await updateTeamWorkMembers({ members });
      await Promise.all(Object.keys(periodItems.value).map(fetchPeriod));
    });
  settingsSaveChain = saveTask;
  saveTask
    .finally(() => {
      if (settingsSaveChain === saveTask) settingsSaving.value = false;
    })
    .catch(() => {});
}

function refreshOnActivation() {
  const standardPeriods = availableTabs.value
    .filter((tab) => tab.key !== "custom")
    .map((tab) => tab.key);
  const periods = [...standardPeriods];
  if (activePeriod.value === "custom") periods.push("custom");
  loading.value = true;
  Promise.all([loadMemberSettings(), ...periods.map(fetchPeriod)])
    .finally(() => {
      loading.value = false;
    })
    .catch(() => {});
}

onMounted(async () => {
  window.addEventListener(
    GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
    handleSharedListPreferencesChange,
  );
  const standardPeriods = availableTabs.value
    .filter((tab) => tab.key !== "custom")
    .map((tab) => tab.key);
  const initialPeriods = [...standardPeriods];
  if (activePeriod.value === "custom") initialPeriods.push("custom");
  loading.value = true;
  try {
    await Promise.all([
      loadMemberSettings(),
      ...initialPeriods.map(fetchPeriod),
    ]);
  } finally {
    loading.value = false;
  }
});

onActivated(() => {
  if (activationCount++ > 0) refreshOnActivation();
});

onBeforeUnmount(() => {
  window.removeEventListener(
    GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
    handleSharedListPreferencesChange,
  );
  if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
});
</script>

<style scoped lang="less">
.team-work-page {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  color: var(--app-text);
}

.team-work-header {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid var(--app-border);

  h3 {
    flex: none;
    margin: 0;
    color: var(--app-text);
    font-size: 16px;
    font-weight: 400;
    line-height: 24px;
  }
}

.page-icon {
  display: inline-flex;
  width: 24px;
  height: 24px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: #fff;
  background: #0d68ff;
  font-size: 13px;
}

.period-tabs {
  min-width: 0;
  margin-left: 92px;
  margin-bottom: 0;

  :deep(.ant-tabs-nav) {
    margin-bottom: 0;
  }
  :deep(.ant-tabs-nav-operations) {
    display: none;
  }
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.tab-label em {
  display: inline-flex;
  min-width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  padding: 0 6px;
  border-radius: 9px;
  color: #68758c;
  background: #eef1f6;
  font-size: 11px;
  font-style: normal;
  font-weight: 600;
  line-height: 18px;
}

.period-tabs :deep(.ant-tabs-tab-active .tab-label em) {
  color: #fff;
  background: #0d68ff;
}

.custom-date-picker {
  width: 132px;
}

.team-work-toolbar {
  display: flex;
  min-height: 44px;
  flex: none;
  align-items: center;
  margin-left: auto;
  // border-bottom: 1px solid var(--app-border);
}

.toolbar-spacer {
  flex: 1;
}

.settings-button {
  display: inline-flex;
  height: 32px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 0 8px;
  color: var(--app-text-secondary);
  font-size: 13px;
}

.team-work-page > :deep(.team-work-kanban) {
  min-height: 0;
  flex: 1;
}

.page-loading,
.page-empty {
  margin: auto;
}

.groups-scroll {
  flex: 1;
  overflow: auto;
  padding: 22px 0 40px;
}

.member-work-group + .member-work-group {
  margin-top: 36px;
}

.group-toggle {
  display: flex;
  height: 34px;
  align-items: center;
  gap: 8px;
  padding: 0 4px;
  border: 0;
  color: #182b50;
  background: transparent;
  cursor: pointer;
  font-size: 15px;

  :deep(.anticon) {
    color: #b7bec9;
    font-size: 11px;
  }

  strong {
    font-weight: 600;
  }
}

.group-avatar {
  flex: none;
  color: #fff;
  font-size: 12px;
  border: none;
  &.group-avatar-empty {
    background: #315bea;
  }
}

.team-work-table {
  :deep(.ant-table) {
    color: #182b50;
    font-size: 13px;
  }

  :deep(.ant-table-container) {
    border: 0;
  }

  :deep(.ant-table-thead > tr > th) {
    height: 40px;
    padding: 8px 10px;
    border-bottom: 1px solid #ebeef5;
    color: #8a96a8;
    background: #fafafa;
    font-weight: 400;
  }

  :deep(.ant-table-tbody > tr > td) {
    height: 48px;
    padding: 8px 10px;
    border-bottom: 1px solid #ebeef5;
  }

  :deep(.ant-table-tbody > tr:hover > td) {
    background: #fafcff;
  }

  :deep(.ant-table-tbody > tr.viewable-row) {
    cursor: pointer;
  }

  :deep(.ant-table-tbody > tr.row-done) {
    opacity: 0.5;
  }

  :deep(.ant-table-tbody > tr.detail-selected-row) {
    opacity: 1;
  }

  :deep(.ant-table-tbody > tr.detail-selected-row > td),
  :deep(.ant-table-tbody > tr.detail-selected-row:hover > td) {
    background: #eaf3ff !important;
  }

  :deep(.ant-table-tbody > tr.detail-selected-row .title-cell a),
  :deep(.ant-table-tbody > tr.detail-selected-row .title-cell strong) {
    color: #1677ff !important;
  }

  :deep(td.editable-table-cell) {
    cursor: pointer;
  }
}

.title-cell {
  width: 100%;
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 7px;

  a,
  strong {
    min-width: 0;
    flex: 0 1 auto;
    overflow: hidden;
    color: #182b50;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  a:hover {
    color: #1677ff;
  }
}

.workspace-name {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.title-input {
  width: min(280px, 70vw);
}

.edit-title {
  display: inline-flex;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  color: #a2a9b5;
  cursor: pointer;
  font-size: 13px;
  opacity: 0;
  pointer-events: none;
  transition:
    opacity 0.15s,
    background 0.15s;

  &:hover {
    color: #657188;
    background: #f0f1f3;
  }
}

.team-work-table :deep(.ant-table-tbody > tr:hover .edit-title) {
  opacity: 1;
  pointer-events: auto;
}

.clickable {
  cursor: pointer;
}
.assignee-cell-wrap {
  display: flex;
  align-items: center;
  width: 100%;
  min-width: 0;
  overflow: hidden;
}

@media (max-width: 1200px) {
  .period-tabs {
    margin-left: 50px;
  }
}

@media (max-width: 860px) {
  .team-work-header {
    flex-wrap: wrap;
    padding: 10px 0;
  }

  .period-tabs {
    width: 100%;
    order: 3;
    margin-left: 0;
  }

  .custom-date-picker {
    order: 4;
  }
}
</style>
