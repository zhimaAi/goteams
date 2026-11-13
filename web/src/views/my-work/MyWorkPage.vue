<template>
  <div ref="pageRef" class="my-work-page">
    <header class="page-header">
      <span class="page-icon"><CalendarOutlined /></span>
      <a-typography-title :level="4" class="page-title">{{
        $t("myWork_title")
      }}</a-typography-title>
      <a-tabs
        v-model:activeKey="activeTab"
        class="work-tabs"
        @change="handleTabChange"
      >
        <a-tab-pane key="today">
          <template #tab
            ><span class="tab-label"
              >{{ $t("myWork_today") }}<em>{{ dateCount(today) }}</em></span
            ></template
          >
        </a-tab-pane>
        <a-tab-pane key="yesterday">
          <template #tab
            ><span class="tab-label"
              >{{ $t("myWork_yesterday")
              }}<em>{{ dateCount(yesterday) }}</em></span
            ></template
          >
        </a-tab-pane>
        <a-tab-pane key="tomorrow">
          <template #tab
            ><span class="tab-label"
              >{{ $t("myWork_tomorrow")
              }}<em>{{ dateCount(tomorrow) }}</em></span
            ></template
          >
        </a-tab-pane>
        <a-tab-pane key="all">
          <template #tab
            ><span class="tab-label"
              >{{ $t("myWork_allTodo") }}<em>{{ todoItems.length }}</em></span
            ></template
          >
        </a-tab-pane>
        <a-tab-pane key="done">
          <template #tab
            ><span class="tab-label"
              >{{ $t("myWork_done") }}<em>{{ doneItems.length }}</em></span
            ></template
          >
        </a-tab-pane>
        <a-tab-pane key="recent">
          <template #tab
            ><span class="tab-label"
              >{{ $t("my_work_recent_visit")
              }}<em>{{ recentVisitCount }}</em></span
            ></template
          >
        </a-tab-pane>
      </a-tabs>
      <div class="header-spacer" />
      <a-tooltip :title="$t('workspace_listSettings')">
        <a-button
          type="text"
          class="settings-button"
          :aria-label="$t('workspace_listSettings')"
          @click="settingsOpen = true"
        >
          <SettingOutlined />
        </a-button>
      </a-tooltip>
      <a-button
        type="primary"
        :disabled="!creatableWorkspaces.length"
        :aria-label="$t('myWork_createItem')"
        @click="goCreate"
      >
        <PlusOutlined />
        <span class="create-label">{{ $t("myWork_createItem") }}</span>
      </a-button>
    </header>

    <a-spin v-if="loading" class="page-loading" />
    <MyWorkKanban
      v-else-if="viewMode === 'board'"
      :columns="boardColumns"
      :selected-item-key="selectedItemKey"
      :project-colors="projectColors"
      @open="openDetail"
      @project-visibility-change="handleProjectVisibilityChange"
      @project-color-change="handleProjectColorChange"
      @project-order-change="handleProjectOrderChange"
    />
    <div v-else-if="groups.length" class="groups-scroll">
      <section
        v-for="group in groups"
        :key="group.id"
        class="work-group"
        :class="{ completed: group.items.every(isDone) }"
      >
        <div v-if="!group.flat" class="group-header">
          <button
            class="group-toggle"
            type="button"
            @click="toggleGroup(group.id)"
          >
            <CaretRightFilled v-if="collapsedGroups.has(group.id)" />
            <CaretDownFilled v-else />
            <img
              v-if="groupIconUrl(group.id)"
              :src="groupIconUrl(group.id)"
              alt=""
              class="group-icon-image"
            />
            <span
              v-else
              class="group-icon"
              :style="{ '--group-color': group.color || '#0d68ff' }"
              >{{ group.name?.trim()?.[0] || "?" }}</span
            >
            <strong>{{ group.name }}（{{ group.total }}）</strong>
            <!-- <small> {{ $t("my_work_item_unit") }}</small> -->
          </button>
          <a-button
            type="link"
            size="small"
            class="enter-project"
            @click="goWorkspace(group.id)"
          >
            {{ $t("my_work_enter_project") }} <ArrowRightOutlined />
          </a-button>
        </div>
        <a-table
          v-if="!collapsedGroups.has(group.id)"
          class="work-table"
          row-key="id"
          :columns="activeColumns"
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
                <WorkItemTypePill
                  :type="record.type"
                  :ws-id="record.workspace_id"
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
                <a v-else :title="record.title" @click="openDetail(record)">{{
                  record.title
                }}</a>
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
            <template v-else-if="column.key === 'visited_at'">
              {{ formatLocalDate(record.visited_at, true) }}
            </template>
          </template>
        </a-table>
      </section>
    </div>
    <a-empty v-else :description="$t('common_noData')" class="page-empty" />

    <MyWorkSettingsModal
      v-model:open="settingsOpen"
      :preview-mode="previewMode"
      :view-mode="viewMode"
      :projects="projectPreferences"
      :hidden-project-ids="hiddenProjectIds"
      @preview-mode-change="handlePreviewModeChange"
      @view-mode-change="handleViewModeChange"
      @project-preferences-change="handleProjectPreferencesChange"
      @restore="restoreProjectPreferences"
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
      :navigation-items="myWorkNavigationItems"
      @close="closeDetail"
      @navigate="handleDetailNavigate"
      @updated="handleDetailUpdated"
    />
  </div>
</template>

<script setup>
defineOptions({ name: "MyWorkPage" });

import {
  computed,
  reactive,
  ref,
  watch,
  nextTick,
  onMounted,
  onActivated,
  onBeforeUnmount,
} from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import dayjs from "dayjs";
import {
  ArrowRightOutlined,
  CalendarOutlined,
  CaretDownFilled,
  CaretRightFilled,
  EditOutlined,
  PlusOutlined,
  SettingOutlined,
} from "@ant-design/icons-vue";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import StatusTransitionModal from "@/components/common/StatusTransitionModal.vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import { formatLocalDate, plannedDateCoversDay } from "@/utils/date";
import {
  getRequirement,
  getWorkspaceSettings,
  listMembers,
  listMyWork,
  listWorkspaces,
  listPriorities,
  listRecentVisits,
  listStatuses,
  updateRequirement,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useWorkspaceStore } from "@/stores/workspace";
import { useContactPreferencesStore } from "@/stores/contactPreferences";
import { setSecondaryKey } from "@/composables/usePageTitle";
import { getRememberedTab, setRememberedTab } from "@/utils/rememberedTab";
import { buildWorkItemListUpdate, mergeWorkItem } from "@/utils/workItemList";
import {
  GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
  readGlobalListPreferences,
  readMyWorkPreferences,
  writeGlobalListPreferences,
  writeMyWorkPreferences,
} from "@/utils/boardPreferences";
import { saveWorkItemNavigationContext } from "@/utils/workItemNavigation";
import WorkItemColumnHeader from "@/components/common/WorkItemColumnHeader.vue";
import { useListColumnLayout } from "@/composables/useListColumnLayout";
import MyWorkKanban from "./components/MyWorkKanban.vue";
import MyWorkSettingsModal from "./components/MyWorkSettingsModal.vue";
import { isStatusTransitionEnabled } from "@/utils/statusTransition";

const router = useRouter();
const { t } = useI18n();
const authStore = useAuthStore();
const contactPreferences = useContactPreferencesStore();
const workspaceStore = useWorkspaceStore();
// Reuse the sidebar's project icon: a custom image when set, otherwise a colored
// initial tile. Keeps "My Work" consistent with the sidebar project list.
function groupIconUrl(id) {
  return workspaceStore.iconUrls[String(id)] || "";
}
const loading = ref(false);
const settingsOpen = ref(false);
const projects = ref([]);
const projectOrder = ref([]);
const hiddenProjectIds = ref([]);
const projectColors = ref({});
const globalListPreferences = readGlobalListPreferences(authStore.userInfo?.id);
const previewMode = ref(
  ["drawer", "modal", "window"].includes(globalListPreferences.previewMode)
    ? globalListPreferences.previewMode
    : "drawer",
);
const viewMode = ref(
  ["list", "board"].includes(globalListPreferences.viewMode)
    ? globalListPreferences.viewMode
    : "list",
);
// Column width, order and sort of the list, kept in the page preferences. The
// two column sets (date tabs and the recent-visit tab) share one layout.
const {
  columnWidths,
  listSort,
  resolveColumns,
  handleResizeColumn,
  moveColumn,
  cycleColumnSort,
  sortItems,
  tableWidth,
} = useListColumnLayout({
  userId: authStore.userInfo?.id,
  readPreferences: readMyWorkPreferences,
  writePreferences: writeMyWorkPreferences,
});
// The "my work" tab's secondary-menu title (i18n key).
const myWorkTabTitle = {
  all: "myWork_allTodo",
  done: "myWork_done",
  today: "myWork_today",
  yesterday: "myWork_yesterday",
  tomorrow: "myWork_tomorrow",
  recent: "my_work_recent_visit",
};
const activeTab = ref(
  getRememberedTab(
    authStore.userInfo?.id,
    "my-work",
    Object.keys(myWorkTabTitle),
    "today",
  ),
);
const items = ref([]);
const recentItems = ref([]);
const recentVisitCount = ref(0);
let myWorkRequestId = 0;
let recentVisitsRequestId = 0;
let activationCount = 0;
// Row order captured when the tab was opened. Opening a row's detail is itself a new visit,
// so without this snapshot the row would jump to the top while the user is still reading it.
const recentOrder = ref([]);
// Declared after the refs its callback touches: the immediate run would hit their temporal
// dead zone should the default tab ever become "recent".
watch(
  activeTab,
  (key) => {
    setSecondaryKey(myWorkTabTitle[key] || "");
    // Recent-visit rows load lazily so they never slow down the initial page load;
    // switching into the tab is also the only moment the ordering may change.
    if (key === "recent") fetchRecentVisits();
  },
  { immediate: true },
);
const creatableWorkspaces = ref([]);
const collapsedGroups = ref(new Set());
const editing = ref(null);
const titleDraft = ref("");
const optionCache = ref({});
const transitionOpen = ref(false);
const transitionItem = ref(null);
const detailItem = ref(null);
const detailRequirementId = ref(null);
const pageRef = ref(null);
const tableAreaWidth = ref(0);
let tableResizeObserver;
let detailRowScrollFrame;
const today = dayjs().startOf("day").unix();
const yesterday = dayjs().subtract(1, "day").startOf("day").unix();
const tomorrow = dayjs().add(1, "day").startOf("day").unix();

function boolValue(value) {
  return value === true || value === 1 || value === "1" || value === "true";
}
function isDone(item) {
  return boolValue(item?.is_done);
}
function canEdit(item) {
  return !!item && boolValue(item.can_edit);
}
const todoItems = computed(() => items.value.filter((item) => !isDone(item)));
const doneItems = computed(() => items.value.filter(isDone));
function dateCount(date) {
  return items.value.filter((item) =>
    plannedDateCoversDay(item.planned_start_date, item.planned_end_date, date),
  ).length;
}
// Recent-visit rows in the order captured when the tab was opened. Rows missing from that
// snapshot (visited since) fall to the end rather than reshuffling everything above them.
const orderedRecentItems = computed(() => {
  const rank = new Map(recentOrder.value.map((id, index) => [id, index]));
  return [...recentItems.value].sort(
    (a, b) =>
      (rank.get(String(a.id)) ?? Number.MAX_SAFE_INTEGER) -
      (rank.get(String(b.id)) ?? Number.MAX_SAFE_INTEGER),
  );
});
const visibleItems = computed(() => {
  if (activeTab.value === "recent") return orderedRecentItems.value;
  if (activeTab.value === "done") return doneItems.value;
  if (activeTab.value === "all") return todoItems.value;
  const date = { today, yesterday, tomorrow }[activeTab.value];
  return items.value.filter((item) =>
    plannedDateCoversDay(item.planned_start_date, item.planned_end_date, date),
  );
});

function normalizeProjects(list) {
  return (Array.isArray(list) ? list : [])
    .map((project) => ({
      id: String(project.id || ""),
      name: project.name || t("workspace_name"),
      color: project.color || "#0d68ff",
    }))
    .filter((project) => project.id);
}

function syncProjectPreferences(nextProjects) {
  const ids = nextProjects.map((project) => project.id);
  const validIds = new Set(ids);
  const storedPreferences = readMyWorkPreferences(authStore.userInfo?.id);
  const storedOrder = Array.isArray(storedPreferences.projectOrder)
    ? storedPreferences.projectOrder.map(String)
    : [];
  const previousOrder = projectOrder.value.length
    ? projectOrder.value
    : storedOrder;
  const nextOrder = [
    ...new Set(previousOrder.filter((id) => validIds.has(String(id)))),
    ...ids.filter((id) => !previousOrder.map(String).includes(id)),
  ];
  const nextHidden = Array.isArray(storedPreferences.hiddenProjectIds)
    ? storedPreferences.hiddenProjectIds
        .map(String)
        .filter((id) => validIds.has(id))
    : hiddenProjectIds.value.filter((id) => validIds.has(String(id)));
  projectOrder.value = nextOrder;
  hiddenProjectIds.value = [...new Set(nextHidden.map(String))];
  const storedColors = storedPreferences.projectColors || {};
  projectColors.value = Object.fromEntries(
    Object.entries(storedColors).filter(([id]) => validIds.has(String(id))),
  );
  writeMyWorkPreferences(authStore.userInfo?.id, {
    projectOrder: nextOrder,
    hiddenProjectIds: hiddenProjectIds.value,
    projectColors: projectColors.value,
  });
}

const projectPreferences = computed(() => {
  const rank = new Map(
    projectOrder.value.map((id, index) => [String(id), index]),
  );
  return [...projects.value].sort(
    (a, b) =>
      (rank.get(String(a.id)) ?? Number.MAX_SAFE_INTEGER) -
        (rank.get(String(b.id)) ?? Number.MAX_SAFE_INTEGER) ||
      String(a.name).localeCompare(String(b.name)),
  );
});

const boardColumns = computed(() => {
  const itemsByProject = new Map();
  for (const item of visibleItems.value) {
    const id = String(item.workspace_id);
    if (!itemsByProject.has(id)) itemsByProject.set(id, []);
    itemsByProject.get(id).push(item);
  }
  return projectPreferences.value
    .filter((project) => !hiddenProjectIds.value.includes(String(project.id)))
    .map((project) => ({
      ...project,
      items: itemsByProject.get(String(project.id)) || [],
    }));
});

const selectedItemKey = computed(() =>
  detailItem.value
    ? `${detailItem.value.workspace_id}:${detailItem.value.id}`
    : "",
);

function compareItems(a, b) {
  if (activeTab.value === "all") return Number(b.id) - Number(a.id);
  if (activeTab.value === "done")
    return Number(b.updated_at) - Number(a.updated_at);
  const dateA = Number(a.planned_start_date) || Number.MAX_SAFE_INTEGER;
  const dateB = Number(b.planned_start_date) || Number.MAX_SAFE_INTEGER;
  return (
    dateA - dateB ||
    Number(a.priority_sort_order || 999999) -
      Number(b.priority_sort_order || 999999) ||
    Number(b.id) - Number(a.id)
  );
}
const groups = computed(() => {
  // The recent-visit tab renders one flat table: its rows are ordered by visit time across
  // projects, so grouping by project would break that order and make the project column
  // redundant. No rows means no group, which lets the shared empty state show through.
  if (activeTab.value === "recent")
    return visibleItems.value.length
      ? [
          {
            id: "__recent__",
            flat: true,
            items: sortItems(visibleItems.value, activeColumns.value),
            total: visibleItems.value.length,
          },
        ]
      : [];
  const grouped = new Map();
  for (const item of visibleItems.value) {
    const id = String(item.workspace_id);
    if (!grouped.has(id))
      grouped.set(id, {
        id,
        name: item.workspace_name,
        color: item.workspace_color,
        sourceItems: [],
      });
    grouped.get(id).sourceItems.push(item);
  }
  const result = [...grouped.values()];
  const projectRank = new Map(
    projectOrder.value.map((id, index) => [String(id), index]),
  );
  for (const group of result) {
    group.sourceItems.sort(compareItems);
    // Sorting stays inside a project group; the project order below is untouched.
    group.items = sortItems(group.sourceItems, activeColumns.value);
    group.total = group.sourceItems.length;
  }
  return result.sort((a, b) => {
    const order =
      (projectRank.get(String(a.id)) ?? Number.MAX_SAFE_INTEGER) -
      (projectRank.get(String(b.id)) ?? Number.MAX_SAFE_INTEGER);
    if (order) return order;
    if (activeTab.value === "all") return Number(b.id) - Number(a.id);
    return (
      compareItems(a.sourceItems[0], b.sourceItems[0]) ||
      a.name.localeCompare(b.name)
    );
  });
});
const myWorkNavigationItems = computed(() =>
  groups.value
    .filter((group) => !collapsedGroups.value.has(group.id))
    .flatMap((group) =>
      group.items.map((item) => ({
        id: item.id,
        workspaceId: item.workspace_id,
      })),
    ),
);

const tableScrollWidth = computed(() => tableWidth(activeColumns.value));

function handleTabChange(key) {
  editing.value = null;
  setRememberedTab(authStore.userInfo?.id, "my-work", key);
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

function handleProjectPreferencesChange(preferences) {
  const validIds = new Set(projects.value.map((project) => String(project.id)));
  projectOrder.value = [
    ...new Set(
      (preferences.projectOrder || [])
        .map(String)
        .filter((id) => validIds.has(id)),
    ),
    ...projects.value
      .map((project) => String(project.id))
      .filter(
        (id) => !(preferences.projectOrder || []).map(String).includes(id),
      ),
  ];
  hiddenProjectIds.value = [
    ...new Set(
      (preferences.hiddenProjectIds || [])
        .map(String)
        .filter((id) => validIds.has(id)),
    ),
  ];
  writeMyWorkPreferences(authStore.userInfo?.id, {
    projectOrder: projectOrder.value,
    hiddenProjectIds: hiddenProjectIds.value,
    projectColors: projectColors.value,
  });
}

function handleProjectVisibilityChange({ id, visible }) {
  const projectId = String(id);
  const hidden = new Set(hiddenProjectIds.value.map(String));
  if (visible) hidden.delete(projectId);
  else hidden.add(projectId);
  hiddenProjectIds.value = [...hidden];
  writeMyWorkPreferences(authStore.userInfo?.id, {
    projectOrder: projectOrder.value,
    hiddenProjectIds: hiddenProjectIds.value,
    projectColors: projectColors.value,
  });
}

function handleProjectColorChange({ id, color }) {
  const projectId = String(id);
  const next = { ...projectColors.value };
  if (color) next[projectId] = color;
  else delete next[projectId];
  projectColors.value = next;
  writeMyWorkPreferences(authStore.userInfo?.id, {
    projectOrder: projectOrder.value,
    hiddenProjectIds: hiddenProjectIds.value,
    projectColors: next,
  });
}

function handleProjectOrderChange(order) {
  const validIds = new Set(projects.value.map((project) => String(project.id)));
  const normalized = [...new Set((order || []).map(String))].filter((id) =>
    validIds.has(id),
  );
  projectOrder.value = [
    ...normalized,
    ...projects.value
      .map((project) => String(project.id))
      .filter((id) => !normalized.includes(id)),
  ];
  writeMyWorkPreferences(authStore.userInfo?.id, {
    projectOrder: projectOrder.value,
    hiddenProjectIds: hiddenProjectIds.value,
    projectColors: projectColors.value,
  });
}

function restoreProjectPreferences() {
  projectOrder.value = projects.value.map((project) => String(project.id));
  hiddenProjectIds.value = [];
  projectColors.value = {};
  writeMyWorkPreferences(authStore.userInfo?.id, {
    projectOrder: projectOrder.value,
    hiddenProjectIds: [],
    projectColors: {},
  });
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

const columnDefinitions = computed(() => [
  {
    title: t("workspace_workItemTitle"),
    key: "title",
    width: 500,
    minWidth: 200,
    maxWidth: 1000,
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
    width: 118,
    minWidth: 110,
    maxWidth: 260,
    sortField: "planned_start_date",
    customCell: (record) => editableCellProps(record, "planned_start_date"),
  },
  {
    title: t("workspace_plannedEndDateCol"),
    key: "planned_end_date",
    width: 118,
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

// Columns of the recent-visit tab. Being a flat cross-project list it carries the work item
// id and its project instead of the planned dates the date-based tabs rely on.
const recentColumnDefinitions = computed(() => [
  {
    title: t("workspace_id"),
    dataIndex: "id",
    key: "id",
    width: 90,
    minWidth: 80,
    maxWidth: 200,
  },
  {
    title: t("workspace_workItemTitle"),
    key: "title",
    width: 420,
    minWidth: 200,
    maxWidth: 1000,
    ellipsis: true,
  },
  {
    title: t("my_work_belong_project"),
    dataIndex: "workspace_name",
    key: "workspace_name",
    width: 150,
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
    title: t("workspace_assignee"),
    key: "assignee",
    width: 120,
    minWidth: 110,
    maxWidth: 300,
    customCell: (record) => editableCellProps(record, "assignee_ids"),
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
    title: t("my_work_last_visit_time"),
    key: "visited_at",
    width: 170,
    minWidth: 140,
    maxWidth: 300,
  },
]);

// Until the user resizes it, the title column of the date tabs fills the
// remaining table area instead of leaving empty space at the right.
const columns = computed(() => {
  const resolved = resolveColumns(columnDefinitions.value);
  if (columnWidths.title) return resolved;
  const fixedWidth = resolved.reduce(
    (total, column) =>
      total + (column.key === "title" ? 0 : Number(column.width) || 0),
    0,
  );
  const title = resolved.find((column) => column.key === "title");
  if (title) {
    title.width = Math.min(
      1000,
      Math.max(320, tableAreaWidth.value - fixedWidth),
    );
  }
  return resolved;
});
const recentColumns = computed(() =>
  resolveColumns(recentColumnDefinitions.value),
);
const activeColumns = computed(() =>
  activeTab.value === "recent" ? recentColumns.value : columns.value,
);

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
    activeColumns.value.map((item) => item.key),
  );
}

async function fetchMyWork() {
  const requestId = ++myWorkRequestId;
  loading.value = true;
  try {
    const [result, workspaceList] = await Promise.all([
      listMyWork(),
      listWorkspaces().catch(() => null),
    ]);
    if (requestId !== myWorkRequestId) return;
    items.value = result?.list || [];
    // The recent-visit tab only needs its badge count up front; its rows load on demand.
    recentVisitCount.value = Number(result?.recent_visit_count) || 0;
    creatableWorkspaces.value = result?.creatable_workspaces || [];
    if (Array.isArray(workspaceList)) {
      projects.value = normalizeProjects(workspaceList);
      syncProjectPreferences(projects.value);
      // Make sure each project's custom icon is available, matching the sidebar.
      for (const project of projects.value)
        workspaceStore.loadWorkspaceIcon(project.id).catch(() => {});
    }
    if (detailItem.value)
      detailItem.value =
        items.value.find(
          (item) => String(item.id) === String(detailItem.value.id),
        ) || detailItem.value;
  } finally {
    if (requestId === myWorkRequestId) loading.value = false;
  }
}

// Loads the recent-visit rows. keepOrder retains the snapshot captured when the tab was
// opened, so editing a row refreshes its values in place instead of moving it to the top.
async function fetchRecentVisits(keepOrder = false) {
  const requestId = ++recentVisitsRequestId;
  if (!keepOrder) loading.value = true;
  try {
    const result = (await listRecentVisits()) || {};
    if (requestId !== recentVisitsRequestId) return;
    recentItems.value = result.list || [];
    recentVisitCount.value = recentItems.value.length;
    if (!keepOrder)
      recentOrder.value = recentItems.value.map((item) => String(item.id));
  } finally {
    if (requestId === recentVisitsRequestId) loading.value = false;
  }
}

function refreshOnActivation() {
  fetchMyWork();
  if (activeTab.value === "recent") fetchRecentVisits();
}

function sameWorkItem(item, id, workspaceId) {
  return (
    String(item.id) === String(id) &&
    String(item.workspace_id) === String(workspaceId)
  );
}

async function handleDetailUpdated(change) {
  if (!change || change.action === "refresh") {
    await fetchMyWork();
    if (activeTab.value === "recent") await fetchRecentVisits(true);
    return;
  }
  const current = items.value.find((item) =>
    sameWorkItem(item, change.id, change.workspaceId),
  );
  const recent = recentItems.value.find((item) =>
    sameWorkItem(item, change.id, change.workspaceId),
  );
  const selected = sameWorkItem(
    detailItem.value || {},
    change.id,
    change.workspaceId,
  )
    ? detailItem.value
    : null;
  const source = current || recent || selected;
  if (!source) return;

  const updated = mergeWorkItem({ ...source }, change.item);
  const currentUserId = Number(authStore.userInfo?.id);
  const remainsInMyWork = (updated.assignee_ids || [])
    .map(Number)
    .includes(currentUserId);
  let resolved = updated;
  if (remainsInMyWork) {
    if (current) resolved = mergeWorkItem(current, updated);
    else items.value = [...items.value, updated];
  } else if (current) {
    items.value = items.value.filter((item) => item !== current);
  }
  if (recent) mergeWorkItem(recent, updated);
  if (selected) detailItem.value = resolved;
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

function isEditing(item, field) {
  return editing.value?.id === item.id && editing.value?.field === field;
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
  editing.value = { id: item.id, field };
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
  editing.value = { id: item.id, field: "title" };
}
async function saveTitle(item) {
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

function toggleGroup(id) {
  const next = new Set(collapsedGroups.value);
  next.has(id) ? next.delete(id) : next.add(id);
  collapsedGroups.value = next;
}
function rowClassName(item) {
  return [
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
    String(item.workspace_id) === String(detailItem.value.workspace_id)
  );
}
function scrollSelectedDetailRowIntoView() {
  if (!detailRequirementId.value) return;
  if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
  nextTick(() => {
    detailRowScrollFrame = requestAnimationFrame(() => {
      detailRowScrollFrame = undefined;
      pageRef.value
        ?.querySelector(".work-table .detail-selected-row")
        ?.scrollIntoView({ block: "nearest", inline: "nearest" });
    });
  });
}
watch(
  [detailRequirementId, () => detailItem.value?.workspace_id, items],
  scrollSelectedDetailRowIntoView,
  { flush: "post" },
);
function rowProps(item) {
  return { onClick: () => openDetail(item) };
}
function openDetail(item) {
  if (previewMode.value === "window") {
    const token = saveWorkItemNavigationContext(myWorkNavigationItems.value);
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
  const target = items.value.find(
    (item) =>
      Number(item.id) === Number(id) &&
      Number(item.workspace_id) === Number(targetWorkspaceId),
  );
  if (target) detailItem.value = target;
  else if (Number(targetWorkspaceId) !== Number(detailItem.value?.workspace_id))
    return;
  detailRequirementId.value = id;
}
function closeDetail() {
  detailItem.value = null;
  detailRequirementId.value = null;
}
function goCreate() {
  if (!creatableWorkspaces.value.length) return;
  // 默认跳转到上次创建工作项时选择的工作空间；没有记录则取列表第一个。
  const lastWs = localStorage.getItem("goteams-last-create-ws");
  const target =
    creatableWorkspaces.value.find((item) => String(item.id) === lastWs) ||
    creatableWorkspaces.value[0];
  router.push(`/workspace/${target.id}/create`);
}

function goWorkspace(id) {
  router.push(`/workspace/${id}/board`);
}

onMounted(() => {
  window.addEventListener(
    GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
    handleSharedListPreferencesChange,
  );
  fetchMyWork();
  tableResizeObserver = new ResizeObserver(([entry]) => {
    tableAreaWidth.value = entry.contentRect.width;
  });
  if (pageRef.value) tableResizeObserver.observe(pageRef.value);
});
onActivated(() => {
  if (activationCount++ > 0) refreshOnActivation();
});
onBeforeUnmount(() => {
  window.removeEventListener(
    GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
    handleSharedListPreferencesChange,
  );
  tableResizeObserver?.disconnect();
  if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
});
</script>

<style scoped lang="less">
.my-work-page {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  color: var(--app-text);
}
.page-header {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 12px;
  border-bottom: 1px solid var(--app-border);
}
.page-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  border-radius: 6px;
  background: #0d68ff;
  color: #fff;
  font-size: 13px;
}
.page-title {
  margin: 0;
  color: var(--app-text);
  font-size: 16px;
  font-weight: 400;
  line-height: 24px;
  white-space: nowrap;
}
.work-tabs {
  flex: 0 0 auto;
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
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 6px;
  border-radius: 9px;
  background: #eef1f6;
  color: #68758c;
  font-size: 11px;
  font-style: normal;
  font-weight: 600;
  line-height: 18px;
}
.work-tabs :deep(.ant-tabs-tab-active .tab-label em) {
  background: #0d68ff;
  color: #fff;
}
.page-header :deep(.ant-btn-primary) {
  border-radius: 6px;
  transition:
    background 0.15s,
    box-shadow 0.15s;
}
.page-header :deep(.ant-btn-primary:hover),
.page-header :deep(.ant-btn-primary:focus) {
  box-shadow: 0 3px 8px rgba(49, 87, 226, 0.22);
}
.settings-button {
  flex: 0 0 auto;
}
.header-spacer {
  flex: 1;
}
.groups-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 4px 0 0;
}
.work-group {
  margin-bottom: 10px;
  overflow: hidden;
  background: var(--app-surface);
}
.group-header {
  display: flex;
  align-items: center;
  min-height: 34px;
  padding: 0 10px;
}
.work-group.completed .group-header {
  opacity: 0.55;
}
.group-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  width: auto;
  height: 34px;
  border: 0;
  background: transparent;
  color: var(--app-text);
  cursor: pointer;
  font-size: 14px;
  font-weight: 400;
  text-align: left;
}
.group-toggle strong {
  font-weight: 600;
  color: #182b50;
  font-size: 15px;
}
.group-toggle small {
  color: #68758c;
  font-size: 11px;
}
.group-toggle .anticon-caret-down,
.group-toggle .anticon-caret-right {
  color: #b7bec9;
  font-size: 11px;
}
/* Keeps the group header tile identical to the sidebar project tile (.ws-icon). */
.group-icon-image {
  display: inline-flex;
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  border-radius: 6px;
  object-fit: cover;
}
.group-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 6px;
  background: var(--group-color);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  line-height: 20px;
}
.enter-project {
  flex: 0 0 auto;
  color: #657188;
  font-size: 12px;
}
.group-toggle svg {
  color: #a2a9b5;
  font-size: 11px;
}
.title-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}
.title-cell a {
  flex: 0 1 auto;
  min-width: 0;
  overflow: hidden;
  color: var(--app-text);
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.title-input {
  width: min(280px, 70vw);
}
.edit-title {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  border-radius: 4px;
  color: #a2a9b5;
  cursor: pointer;
  font-size: 13px;
  opacity: 0;
  pointer-events: none;
  transition:
    opacity 0.15s,
    background 0.15s;
}
.edit-title:hover {
  background: #f0f1f3;
  color: #657188;
}
.work-table :deep(.ant-table-tbody > tr:hover .edit-title) {
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
.page-loading,
.page-empty {
  margin: auto;
}
:deep(.work-table .ant-table) {
  font-size: 14px;
}
:deep(.work-table .ant-table-thead > tr > th),
:deep(.work-table .ant-table-tbody > tr > td) {
  padding-right: 8px;
  padding-left: 8px;
  text-align: left;
  vertical-align: middle;
  white-space: nowrap;
}
:deep(.work-table .ant-table-thead > tr > th) {
  padding-top: 7px;
  padding-bottom: 7px;
  background: var(--app-surface-muted);
  color: #7c8597;
  font-size: 14px;
  font-weight: 400;
}
:deep(.work-table .ant-table-tbody > tr > td) {
  padding-top: 6px;
  padding-bottom: 6px;
  color: var(--app-text);
}
:deep(.work-table .ant-table-tbody > tr) {
  cursor: pointer;
}
.work-table :deep(.ant-table-tbody > tr:hover) .title-cell a {
  color: #1677ff;
}
:deep(.work-table td.editable-table-cell) {
  cursor: pointer;
}
:deep(.work-table .row-done) {
  opacity: 0.5;
}
:deep(.work-table .detail-selected-row) {
  opacity: 1;
}
:deep(.work-table .detail-selected-row > td),
:deep(.work-table .detail-selected-row:hover > td) {
  background: #eaf3ff !important;
}
:deep(.work-table .detail-selected-row .title-cell a) {
  color: #1677ff !important;
}
@media (max-width: 900px) {
  .page-header {
    flex-wrap: wrap;
    padding-bottom: 8px;
  }
  .work-tabs {
    order: 3;
    width: 100%;
    margin-left: 0;
    overflow-x: auto;
  }
  .header-spacer {
    min-width: 0;
  }
}
@media (max-width: 600px) {
  .create-label {
    display: none;
  }
}
</style>
