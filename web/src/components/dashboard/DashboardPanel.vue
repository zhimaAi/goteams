<template>
  <a-config-provider :theme="{ token: { colorPrimary: '#1677ff' } }">
    <div
      class="dashboard-panel"
      :class="{ 'dashboard-panel--editing': editing }"
    >
      <a-spin :spinning="loading">
        <div
          v-if="selectedDashboard"
          class="dashboard-panel__layout"
          :class="{ 'dashboard-panel__layout--collapsed': sidebarCollapsed }"
          @keydown.esc="switcherOpen = false"
        >
          <aside
            v-show="!sidebarCollapsed || switcherOpen"
            id="dashboard-view-list"
            class="dashboard-sidebar"
            :class="{ 'dashboard-sidebar--overlay': sidebarCollapsed }"
          >
            <div class="dashboard-sidebar__heading">
              <strong
                ><BarChartOutlined />
                {{ $t("dashboard_workload_views") }}</strong
              >
              <a-button
                v-if="canCreate"
                type="text"
                size="small"
                :aria-label="$t('dashboard_new')"
                @click="openDashboardDialog('create')"
              >
                <PlusOutlined />
              </a-button>
            </div>
            <div v-if="sidebarCollapsed" class="dashboard-sidebar__head">
              <a-input-search
                v-model:value="searchKeyword"
                :placeholder="$t('dashboard_search_placeholder')"
                allow-clear
              />
            </div>

            <div ref="sidebarListRef" class="dashboard-sidebar__list">
              <div
                v-for="dashboard in filteredDashboards"
                :key="dashboard.id"
                class="dashboard-sidebar__item"
                :class="{
                  'dashboard-sidebar__item--active':
                    String(dashboard.id) === String(selectedDashboard.id),
                  'dashboard-sidebar__item--sortable':
                    canSortDashboard(dashboard),
                }"
                :data-dashboard-id="dashboard.id"
              >
                <HolderOutlined
                  v-if="canSortDashboard(dashboard)"
                  class="dashboard-sidebar__drag"
                  :aria-label="$t('dashboard_sort')"
                />
                <button
                  type="button"
                  class="dashboard-sidebar__select"
                  @click="selectDashboard(dashboard.id)"
                >
                  <span>{{ dashboard.name || $t("dashboard_untitled") }}</span>
                </button>
                <a-dropdown :trigger="['click']" @click.stop>
                  <a-button
                    type="text"
                    size="small"
                    :aria-label="$t('common_more')"
                  >
                    <MoreOutlined />
                  </a-button>
                  <template #overlay>
                    <a-menu
                      @click="
                        ({ key }) => handleDashboardAction(key, dashboard)
                      "
                    >
                      <a-menu-item key="copy-link">
                        <LinkOutlined />
                        {{ $t("dashboard_copy_title_link") }}
                      </a-menu-item>
                      <a-menu-item
                        v-if="canEditDashboard(dashboard)"
                        key="rename"
                      >
                        <EditOutlined /> {{ $t("common_rename") }}
                      </a-menu-item>
                      <a-menu-item v-if="canCreate" key="save-as">
                        <CopyOutlined /> {{ $t("dashboard_save_as") }}
                      </a-menu-item>
                      <a-menu-item
                        v-if="canManageDashboard(dashboard)"
                        key="permission"
                      >
                        <SafetyCertificateOutlined />
                        {{ $t("dashboard_permission_title") }}
                      </a-menu-item>
                      <a-menu-item
                        v-if="canManageDashboard(dashboard)"
                        key="delete"
                        danger
                      >
                        <DeleteOutlined /> {{ $t("common_delete") }}
                      </a-menu-item>
                    </a-menu>
                  </template>
                </a-dropdown>
              </div>
              <a-empty
                v-if="!filteredDashboards.length"
                :description="
                  dashboards.length
                    ? $t('dashboard_search_empty')
                    : $t('dashboard_none')
                "
                :image="simpleImage"
              />
            </div>
          </aside>
          <button
            type="button"
            class="dashboard-panel__toggle"
            :aria-label="
              $t(
                sidebarCollapsed
                  ? 'dashboard_workload_expand_views'
                  : 'dashboard_workload_collapse_views',
              )
            "
            :aria-expanded="!sidebarCollapsed"
            @click="toggleSidebar"
          >
            <RightOutlined v-if="sidebarCollapsed" />
            <LeftOutlined v-else />
          </button>
          <main class="dashboard-main" @click="switcherOpen = false">
            <header class="dashboard-main__head">
              <button
                type="button"
                class="dashboard-main__toggle"
                :aria-label="
                  $t(
                    sidebarCollapsed
                      ? 'dashboard_workload_expand_views'
                      : 'dashboard_workload_collapse_views',
                  )
                "
                :aria-expanded="!sidebarCollapsed"
                @click="toggleSidebar"
              >
                <MenuUnfoldOutlined v-if="sidebarCollapsed" />
                <MenuFoldOutlined v-else />
              </button>
              <button
                v-if="sidebarCollapsed"
                class="dashboard-main__title"
                type="button"
                :aria-expanded="switcherOpen"
                aria-controls="dashboard-view-list"
                @click.stop="switcherOpen = !switcherOpen"
              >
                {{ selectedDashboard.name || $t("dashboard_untitled") }}
              </button>
              <h2 v-else class="dashboard-main__title">
                {{ selectedDashboard.name || $t("dashboard_untitled") }}
              </h2>
              <DashboardFilterPanel
                :key="`filter-${selectedDashboard.id}`"
                compact
                :model-value="userState.filters"
                :sections="filterSections"
                :members="members"
                :versions="versions"
                :default-collapsed="userState.filters_collapsed"
                @apply="applyTopFilters"
                @update:collapsed="updateFiltersCollapsed"
              />
              <div class="dashboard-main__actions">
                <template v-if="editing">
                  <a-button type="primary" @click="openCardDrawer('create')">
                    <PlusOutlined /> {{ $t("dashboard_action_add") }}
                  </a-button>
                  <a-button
                    type="primary"
                    :loading="saving"
                    @click="saveLayoutEdit"
                  >
                    <SaveOutlined /> {{ $t("common_save") }}
                  </a-button>
                  <a-button @click="cancelLayoutEdit"
                    ><CloseOutlined /> {{ $t("common_cancel") }}</a-button
                  >
                </template>
                <template v-else-if="canEditDashboard(selectedDashboard)">
                  <a-button type="primary" @click="addCard">
                    <PlusOutlined /> {{ $t("dashboard_action_add") }}
                  </a-button>
                  <a-button @click="startLayoutEdit">
                    <SettingOutlined /> {{ $t("dashboard_action_layout") }}
                  </a-button>
                  <a-button
                    v-if="canManageDashboard(selectedDashboard)"
                    @click="
                      handleDashboardAction('permission', selectedDashboard)
                    "
                  >
                    <LockOutlined /> {{ $t("dashboard_action_permission") }}
                  </a-button>
                </template>
              </div>
            </header>

            <a-alert
              v-if="filterStateSaveFailed"
              class="dashboard-main__filter-alert"
              type="warning"
              show-icon
              :message="$t('dashboard_filter_save_failed')"
            />

            <div class="dashboard-canvas">
              <div
                ref="gridRef"
                class="dashboard-grid"
                :class="{
                  'dashboard-grid--editing': editing,
                  'dashboard-grid--empty': !cardsForGrid.length && !editing,
                }"
              >
                <div
                  v-if="editing"
                  class="dashboard-grid__guides"
                  aria-hidden="true"
                >
                  <span v-for="index in guideCellCount" :key="index" />
                </div>
                <div
                  v-for="card in cardsForGrid"
                  :key="card.id"
                  class="dashboard-grid__item"
                  :class="{
                    'dashboard-grid__item--dragging':
                      String(card.id) === String(draggedCardId),
                  }"
                  :data-card-id="card.id"
                  :style="gridItemStyle(card)"
                >
                  <DashboardCard
                    :ws-id="wsId"
                    :dashboard-id="selectedDashboard.id"
                    :card="displayCard(card)"
                    :workspace="workspaceInfo"
                    :members="members"
                    :priorities="filterSections.requirement.priorities"
                    :statuses="filterSections.requirement.statuses"
                    :categories="workloadCategories"
                    :workload-members="currentMember"
                    :filters-revision="filtersRevision"
                    :editing="editing"
                    :can-edit="canEditDashboard(selectedDashboard)"
                    @action="handleCardAction"
                    @text-refresh="refreshTextCard"
                    @text-expand="openTextPreview"
                    @drag-start="startCardDrag"
                    @resize-start="startCardResize"
                  />
                </div>
                <!-- 预测落点：跟随指针实时计算，松开后卡片即落在此格 -->
                <div
                  v-if="editing && dragPreview"
                  class="dashboard-grid__placeholder"
                  :style="placeholderStyle"
                  aria-hidden="true"
                />
                <div
                  v-if="!cardsForGrid.length && !editing"
                  class="dashboard-grid__empty"
                >
                  <h1>{{ $t("dashboard_welcome") }}</h1>
                  <p>{{ $t("dashboard_welcome_description") }}</p>
                  <DashboardCardTypes
                    v-if="canEditDashboard(selectedDashboard)"
                    @select="addCard"
                  />
                </div>
              </div>
            </div>
          </main>
        </div>

        <a-empty
          v-else-if="!loading"
          class="dashboard-panel__empty"
          :description="$t('dashboard_no_visible')"
          :image="simpleImage"
        >
          <a-button
            v-if="canCreate"
            type="primary"
            @click="openDashboardDialog('create')"
          >
            <PlusOutlined /> {{ $t("dashboard_new") }}
          </a-button>
        </a-empty>
      </a-spin>

      <DashboardCardDrawer
        v-model:open="cardDrawerOpen"
        :ws-id="wsId"
        :dashboard-id="selectedDashboard?.id || 0"
        :mode="cardDrawerMode"
        :card="cardDrawerCard"
        :next-layout="nextCardLayout"
        :has-announcement="
          draftCards.some((card) => card.card_type === 'announcement')
        "
        :announcement="announcementDraft"
        :pending-file-ids="cardDrawerPendingFileIds"
        :announcement-pending-file-ids="
          announcementDraft?.pending_file_ids || []
        "
        :workspace="workspaceInfo"
        :members="members"
        :roles="roles"
        :versions="versions"
        :filter-sections="filterSections"
        @saved="saveDraftCard"
      />

      <DashboardPermissionModal
        v-model:open="permissionOpen"
        :ws-id="wsId"
        :dashboard="permissionDashboard"
        :members="members"
        :roles="roles"
        @saved="handlePermissionSaved"
      />

      <a-modal
        v-model:open="dashboardDialog.open"
        :title="dashboardDialogTitle"
        :confirm-loading="dialogSaving"
        :ok-text="$t('common_save')"
        :cancel-text="$t('common_cancel')"
        @ok="submitDashboardDialog"
        @cancel="closeDashboardDialog"
      >
        <a-select
          v-if="dashboardDialog.mode === 'create' && isIssueWorkspace"
          :value="dashboardDialog.template"
          class="dashboard-dialog-template"
          :aria-label="$t('dashboard_workload_template')"
          :options="dashboardTemplateOptions"
          @change="selectDashboardTemplate"
        />
        <a-input
          v-model:value="dashboardDialog.name"
          :maxlength="100"
          :placeholder="$t('dashboard_name_placeholder')"
          @press-enter="submitDashboardDialog"
        />
      </a-modal>

      <a-modal
        v-model:open="metricCopyOpen"
        :title="$t('dashboard_chart_copy_title')"
        :confirm-loading="metricCopySaving"
        :ok-text="$t('common_copy')"
        :cancel-text="$t('common_cancel')"
        @ok="copyMetricCard"
      >
        <a-select
          v-model:value="metricCopyTargetId"
          mode="multiple"
          option-filter-prop="label"
          class="dashboard-panel__copy-select"
          :placeholder="$t('dashboard_copy_metric_target_placeholder')"
          :options="metricCopyTargets"
        />
      </a-modal>

      <a-modal
        v-model:open="textPreviewOpen"
        :title="textPreviewCard?.title || $t('dashboard_card_text')"
        width="900px"
        :footer="null"
      >
        <RichTextEditor
          :model-value="textPreviewCard?.content || ''"
          :content-mode="textPreviewCard?.content_mode || 'rich-text'"
          :read-only="true"
          :min-height="360"
        />
      </a-modal>
    </div>
  </a-config-provider>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  provide,
  ref,
  watch,
} from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { Empty, message, Modal } from "ant-design-vue";
import Sortable from "sortablejs";
import {
  BarChartOutlined,
  LeftOutlined,
  RightOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  SaveOutlined,
  CloseOutlined,
  SettingOutlined,
  LockOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  HolderOutlined,
  LinkOutlined,
  MoreOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons-vue";
import DashboardCardTypes from "./DashboardCardTypes.vue";
import DashboardCard from "./DashboardCard.vue";
import DashboardCardDrawer from "./DashboardCardDrawer.vue";
import DashboardFilterPanel from "./DashboardFilterPanel.vue";
import DashboardPermissionModal from "./DashboardPermissionModal.vue";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import {
  copyDashboardCards,
  createDashboard,
  deleteDashboard,
  getDashboard,
  getDashboardAnnouncement,
  getDashboardUserState,
  listDashboards,
  saveDashboardAs,
  updateDashboard,
  updateDashboardLayout,
  updateDashboardOrder,
  updateDashboardUserState,
} from "@/api/dashboard";
import { listProjectRoles, listRequirementCategories } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useGanttViewSections } from "@/composables/useGanttViewSections";
import { workItemRoute } from "@/utils/workItemRoutes";
import { useDashboardWorkItems } from "@/composables/useDashboardWorkItems";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  metadata: { type: Object, default: null },
  filterFields: { type: Object, default: () => ({}) },
});

const workspaceType = computed(() => props.metadata?.workspace?.workspace_type);
provide("dashboardWorkspaceType", workspaceType);
const { normalizeFilters, isCardVisible, isIssueWorkspace } =
  useDashboardWorkItems(workspaceType);

// 网格列数，与 .dashboard-grid 的 grid-template-columns 保持一致。
const GRID_COLUMNS = 12;

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const loading = ref(false);
const saving = ref(false);
const dashboards = ref([]);
const selectedDashboard = ref(null);
const roles = ref([]);
const searchKeyword = ref("");
const switcherOpen = ref(false);
const sidebarCollapsed = ref(false);
const workloadCategories = ref([]);
const sidebarListRef = ref(null);
const gridRef = ref(null);
const editing = ref(false);
const draftCards = ref([]);
const deletedCardIds = new Map();
const announcement = ref(null);
const announcementDraft = ref(null);
const announcementDirty = ref(false);
const draggedCardId = ref(null);
// 拖拽预测落点（网格坐标）、跟手浮层位置（视口坐标）与拖拽时的网格几何。
const dragPreview = ref(null);
const dragOverlay = ref(null);
const dragGeometry = ref(null);
const filtersRevision = ref(0);
const userState = ref(emptyUserState());
const filterStateSaveFailed = ref(false);
const cardDrawerOpen = ref(false);
const cardDrawerMode = ref("create");
const cardDrawerCard = ref(null);
const permissionOpen = ref(false);
const permissionDashboard = ref(null);
const metricCopyOpen = ref(false);
const metricCopySaving = ref(false);
const metricCopyCard = ref(null);
const metricCopyTargetId = ref([]);
const textPreviewOpen = ref(false);
const textPreviewCard = ref(null);
const dashboardDialog = ref({
  open: false,
  mode: "create",
  dashboard: null,
  name: "",
  template: "",
});
const dashboardTemplateOptions = computed(() => [
  { value: "", label: t("dashboard_workload_default_template") },
  { value: "workload", label: t("dashboard_chart_workload") },
  { value: "category", label: t("dashboard_chart_category") },
  { value: "sla_stats", label: t("dashboard_template_sla_stats") },
  { value: "sla_risk", label: t("dashboard_template_sla_risk") },
]);
const dialogSaving = ref(false);
let sidebarSortable = null;
let gridDragSession = null;
let dragFrame = 0;
let selectedLoadSequence = 0;
let filtersSaveTimer = null;
let filtersSaveQueue = Promise.resolve();
let resizeSession = null;

const metadataRef = computed(() => props.metadata || {});
const ganttSections = useGanttViewSections(metadataRef, t);
const filterSections = computed(() => {
  const sections = ganttSections.value;
  const requirementFields =
    props.filterFields.requirement || sections.requirement.fields;
  const defectFields = props.filterFields.defect || sections.defect.fields;
  return {
    common: {
      ...sections.common,
      fields: sections.common.fields.filter(
        (field) =>
          requirementFields.some((item) => item.field === field.field) &&
          defectFields.some((item) => item.field === field.field),
      ),
    },
    requirement: { ...sections.requirement, fields: requirementFields },
    defect: { ...sections.defect, fields: defectFields },
  };
});
const members = computed(() =>
  (props.metadata?.members || []).filter((member) => !member.removed),
);
const currentMember = computed(() =>
  members.value.filter(
    (member) => String(member.user_id) === String(authStore.userInfo?.id),
  ),
);
const versions = computed(() => props.metadata?.versions || []);
const workspaceInfo = computed(() => ({
  ...(props.metadata?.workspace || {}),
  id: props.metadata?.workspace?.id || props.wsId,
  member_count: props.metadata?.workspace?.member_count ?? members.value.length,
}));
const canCreate = computed(() => authStore.hasPermission("dashboard:create"));
const filteredDashboards = computed(() => {
  const keyword = searchKeyword.value.trim().toLocaleLowerCase();
  if (!keyword) return dashboards.value;
  return dashboards.value.filter((dashboard) =>
    String(dashboard.name || "")
      .toLocaleLowerCase()
      .includes(keyword),
  );
});
const cardsForGrid = computed(() =>
  (editing.value
    ? draftCards.value
    : selectedDashboard.value?.cards || []
  ).filter(isCardVisible),
);
const dashboardDialogTitle = computed(() => {
  if (dashboardDialog.value.mode === "create") return t("dashboard_new");
  if (dashboardDialog.value.mode === "rename") return t("dashboard_rename");
  return t("dashboard_save_as");
});
const metricCopyTargets = computed(() =>
  dashboards.value
    .filter(
      (dashboard) =>
        String(dashboard.id) !== String(selectedDashboard.value?.id) &&
        canEditDashboard(dashboard),
    )
    .map((dashboard) => ({
      value: Number(dashboard.id),
      label: dashboard.name || t("dashboard_untitled"),
    })),
);
const cardDrawerPendingFileIds = computed(
  () => cardDrawerCard.value?.pending_file_ids || [],
);
const nextCardLayout = computed(() =>
  findOpenPosition(draftCards.value, { w: 4, h: 3 }, 0, 0),
);

watch(
  () => props.wsId,
  () => loadDashboards(),
);
watch(
  () => route.query.dashboardId,
  (dashboardId) => {
    if (
      !dashboardId ||
      String(dashboardId) === String(selectedDashboard.value?.id)
    )
      return;
    if (
      dashboards.value.some(
        (dashboard) => String(dashboard.id) === String(dashboardId),
      )
    ) {
      selectDashboard(dashboardId);
      return;
    }
    const fallback = dashboards.value[0];
    if (fallback) selectDashboard(fallback.id);
  },
);
watch(
  () => [
    searchKeyword.value,
    dashboards.value.length,
    editing.value,
    sidebarCollapsed.value,
    switcherOpen.value,
  ],
  () => setupSidebarSortable(),
  { flush: "post" },
);

watch(
  [
    () => props.wsId,
    () =>
      cardsForGrid.value.some(
        (card) => card.config?.chart_type === "workload_stats",
      ),
  ],
  async ([wsId, hasWorkload], _, onCleanup) => {
    workloadCategories.value = [];
    if (!wsId || !hasWorkload) return;
    let stale = false;
    onCleanup(() => {
      stale = true;
    });
    try {
      const result = await listRequirementCategories(wsId);
      if (!stale) workloadCategories.value = result?.list || [];
    } catch {
      // The shared request interceptor displays the failure.
    }
  },
  { immediate: true },
);

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value;
  switcherOpen.value = false;
  searchKeyword.value = "";
}

onMounted(async () => {
  await loadDashboards();
});
onBeforeUnmount(() => {
  selectedLoadSequence += 1;
  sidebarSortable?.destroy();
  stopCardDrag();
  clearTimeout(filtersSaveTimer);
  stopCardResize();
});

function emptyFilterGroups() {
  return {
    common: { logic: "and", conditions: [] },
    requirement: { logic: "and", conditions: [] },
    defect: { logic: "and", conditions: [] },
  };
}

function emptyUserState() {
  return { filters: emptyFilterGroups(), filters_collapsed: true };
}

function normalizeCard(raw = {}) {
  const config = raw.config || {};
  return {
    ...raw,
    id: raw.id,
    card_type: raw.card_type,
    title: raw.title || "",
    config: {
      ...config,
      filters: config.filters ? structuredCloneSafe(config.filters) : null,
    },
    content: raw.content || "",
    content_mode: raw.content_mode || "rich-text",
    // Persisted assets are already bound and deliberately never round-trip through the editor.
    file_ids: [],
    // Only assets uploaded during the current unsaved layout edit may be bound on save.
    pending_file_ids: [],
    x: Math.max(0, Number(raw.x) || 0),
    y: Math.max(0, Number(raw.y) || 0),
    w: Math.max(1, Number(raw.w) || 4),
    h: Math.max(1, Number(raw.h) || 3),
    revision: Number(raw.revision) || 0,
  };
}

function normalizeDashboard(raw = {}) {
  const cards = (raw.cards || []).map(normalizeCard);
  // 服务端允许布局里留空行，这里统一向上压实，保证卡片默认靠上、无整行空档。
  compactLayouts(cards);
  return {
    ...raw,
    id: raw.id,
    name: raw.name || "",
    cards,
    revision: Number(raw.revision) || 0,
  };
}

async function loadDashboards() {
  loading.value = true;
  try {
    const response = await listDashboards(props.wsId);
    dashboards.value = (Array.isArray(response) ? response : []).map(
      normalizeDashboard,
    );
    const requested = route.query.dashboardId;
    const target = dashboards.value.find(
      (dashboard) => String(dashboard.id) === String(requested),
    );
    const fallback = dashboards.value[0];
    await selectDashboard(
      target?.id || fallback?.id,
      !target && Boolean(requested) && Boolean(fallback),
    );
  } catch {
    dashboards.value = [];
    selectedDashboard.value = null;
  } finally {
    loading.value = false;
  }
}

async function loadRoles() {
  try {
    roles.value = (await listProjectRoles(props.wsId)) || [];
  } catch {
    roles.value = [];
  }
}

async function selectDashboard(dashboardId, updateRoute = true) {
  switcherOpen.value = false;
  if (!dashboardId) {
    selectedLoadSequence += 1;
    selectedDashboard.value = null;
    return;
  }
  const dashboard = dashboards.value.find(
    (item) => String(item.id) === String(dashboardId),
  );
  if (!dashboard) return;
  clearTimeout(filtersSaveTimer);
  filtersSaveTimer = null;
  if (editing.value) cancelLayoutEdit();
  selectedDashboard.value = dashboard;
  userState.value = emptyUserState();
  announcementDraft.value = null;
  cardDrawerOpen.value = false;
  if (updateRoute && String(route.query.dashboardId) !== String(dashboardId)) {
    router.replace({
      query: { ...route.query, dashboardId: String(dashboardId) },
    });
  }
  await loadSelectedDashboard(dashboardId);
}

async function loadSelectedDashboard(dashboardId) {
  const sequence = ++selectedLoadSequence;
  try {
    const detail = await getDashboard(props.wsId, dashboardId);
    if (sequence !== selectedLoadSequence) return;
    const normalized = normalizeDashboard(detail || selectedDashboard.value);
    replaceDashboard(normalized);
    selectedDashboard.value = normalized;
  } catch {
    return;
  }

  const [announcementResult, userStateResult] = await Promise.allSettled([
    getDashboardAnnouncement(props.wsId, dashboardId, { silent: true }),
    getDashboardUserState(props.wsId, dashboardId, { silent: true }),
  ]);
  if (sequence !== selectedLoadSequence) return;
  await nextTick();
  if (sequence !== selectedLoadSequence) return;
  setupSidebarSortable();

  const loadedAnnouncement =
    announcementResult.status === "fulfilled" ? announcementResult.value : null;
  const loadedState =
    userStateResult.status === "fulfilled" ? userStateResult.value : null;
  announcement.value = loadedAnnouncement;
  announcementDraft.value = announcementDraftFrom(loadedAnnouncement);
  announcementDirty.value = false;
  userState.value = {
    filters: normalizeFilters(loadedState?.filters),
    filters_collapsed: loadedState?.filters_collapsed !== false,
  };
  filterStateSaveFailed.value = userStateResult.status === "rejected";
  filtersRevision.value += 1;
}

function announcementDraftFrom(value) {
  if (!value) return null;
  return { ...value, file_ids: [], pending_file_ids: [] };
}

function replaceDashboard(next) {
  const index = dashboards.value.findIndex(
    (item) => String(item.id) === String(next.id),
  );
  if (index < 0) return;
  dashboards.value.splice(index, 1, next);
}

function canEditDashboard(dashboard) {
  return Boolean(dashboard?.can_edit);
}

function canManageDashboard(dashboard) {
  return Boolean(dashboard?.can_manage);
}

function canSortDashboard(dashboard) {
  return (
    !editing.value &&
    !searchKeyword.value.trim() &&
    Boolean(dashboard?.can_reorder)
  );
}

async function setupSidebarSortable() {
  await nextTick();
  sidebarSortable?.destroy();
  sidebarSortable = null;
  if (
    !sidebarListRef.value ||
    searchKeyword.value.trim() ||
    !dashboards.value.some(canSortDashboard)
  )
    return;
  sidebarSortable = new Sortable(sidebarListRef.value, {
    animation: 150,
    handle: ".dashboard-sidebar__drag",
    draggable: ".dashboard-sidebar__item--sortable",
    onMove: (event) =>
      canSortDashboard(
        dashboards.value.find(
          (item) => String(item.id) === event.dragged.dataset.dashboardId,
        ),
      ),
    onEnd: ({ oldIndex, newIndex }) => reorderDashboards(oldIndex, newIndex),
  });
}

async function reorderDashboards(oldIndex, newIndex) {
  if (
    oldIndex === newIndex ||
    oldIndex == null ||
    newIndex == null ||
    searchKeyword.value.trim() ||
    !canSortDashboard(dashboards.value[oldIndex])
  ) {
    return;
  }
  const next = [...dashboards.value];
  const [moved] = next.splice(oldIndex, 1);
  next.splice(newIndex, 0, moved);
  dashboards.value = next;
  try {
    await updateDashboardOrder(props.wsId, {
      ids: next.map((dashboard) => Number(dashboard.id)),
    });
    const latest = await listDashboards(props.wsId);
    const byId = new Map(latest.map((item) => [String(item.id), item]));
    dashboards.value = dashboards.value.map((item) => ({
      ...item,
      ...byId.get(String(item.id)),
      cards: item.cards,
    }));
    const selected = dashboards.value.find(
      (item) => String(item.id) === String(selectedDashboard.value?.id),
    );
    if (selected) selectedDashboard.value = selected;
  } catch {
    await loadDashboards();
  }
}

function handleDashboardAction(key, dashboard) {
  if (key === "copy-link") copyDashboardLink(dashboard);
  if (key === "rename") openDashboardDialog("rename", dashboard);
  if (key === "save-as") openDashboardDialog("save-as", dashboard);
  if (key === "permission") {
    switcherOpen.value = false;
    permissionDashboard.value = dashboard;
    permissionOpen.value = true;
    void loadRoles();
  }
  if (key === "delete") confirmDeleteDashboard(dashboard);
}

async function copyDashboardLink(dashboard) {
  const target = router.resolve(
    workItemRoute(
      route.meta.workItemSurface,
      "dashboard",
      props.wsId,
      undefined,
      { dashboardId: String(dashboard.id) },
    ),
  );
  const url = new URL(target.href, window.location.origin).href;
  try {
    const text = `${dashboard.name || t("dashboard_untitled")}\n${url}`;
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const input = document.createElement("textarea");
      input.value = text;
      input.style.position = "fixed";
      input.style.opacity = "0";
      document.body.appendChild(input);
      try {
        input.select();
        if (!document.execCommand("copy"))
          throw new Error("Clipboard unavailable");
      } finally {
        input.remove();
      }
    }
    message.success(t("dashboard_copy_title_link_success"));
  } catch {
    message.error(t("dashboard_copy_title_link_failed"));
  }
}

function openDashboardDialog(mode, dashboard = null) {
  switcherOpen.value = false;
  dashboardDialog.value = {
    open: true,
    mode,
    dashboard,
    name: mode === "rename" ? dashboard?.name || "" : "",
    template: "",
  };
}

function selectDashboardTemplate(template, option) {
  const dialog = dashboardDialog.value;
  const previousOption = dashboardTemplateOptions.value.find(
    (item) => item.value === dialog.template,
  );
  if (
    !dialog.name.trim() ||
    (dialog.template && dialog.name === previousOption?.label)
  ) {
    dialog.name = template ? option.label : "";
  }
  dialog.template = template;
}

function closeDashboardDialog() {
  dashboardDialog.value.open = false;
}

async function submitDashboardDialog() {
  if (dialogSaving.value) return;
  const name = dashboardDialog.value.name.trim();
  if (!name) {
    message.warning(t("dashboard_name_required"));
    return;
  }
  dialogSaving.value = true;
  try {
    if (dashboardDialog.value.mode === "create") {
      const created = normalizeDashboard(
        await createDashboard(props.wsId, {
          name,
          template: dashboardDialog.value.template,
        }),
      );
      dashboards.value.push(created);
      await selectDashboard(created.id);
    } else if (dashboardDialog.value.mode === "rename") {
      const source = dashboardDialog.value.dashboard;
      const updated = normalizeDashboard(
        await updateDashboard(props.wsId, source.id, {
          name,
          revision: source.revision,
        }),
      );
      replaceDashboard(updated);
      if (String(updated.id) === String(selectedDashboard.value?.id))
        selectedDashboard.value = updated;
    } else {
      const source = dashboardDialog.value.dashboard;
      const created = normalizeDashboard(
        await saveDashboardAs(props.wsId, source.id, {
          name,
          revision: source.revision,
        }),
      );
      dashboards.value.push(created);
      await selectDashboard(created.id);
    }
    closeDashboardDialog();
    message.success(t("common_saveSuccess"));
  } catch {
    // The shared request interceptor displays the failure.
  } finally {
    dialogSaving.value = false;
  }
}

function confirmDeleteDashboard(dashboard) {
  Modal.confirm({
    title: t("dashboard_delete_title"),
    content: t("dashboard_delete_content", {
      name: dashboard.name || t("dashboard_untitled"),
    }),
    okText: t("common_delete"),
    cancelText: t("common_cancel"),
    okButtonProps: { danger: true },
    onOk: async () => {
      await deleteDashboard(props.wsId, dashboard.id, {
        revision: dashboard.revision,
      });
      await loadDashboards();
    },
  });
}

function handlePermissionSaved() {
  loadDashboards();
}

function startLayoutEdit() {
  draftCards.value = (selectedDashboard.value?.cards || []).map((card) =>
    cloneCard(card),
  );
  deletedCardIds.clear();
  announcementDraft.value = announcementDraftFrom(announcement.value);
  announcementDirty.value = false;
  editing.value = true;
  // 进入编辑即压实一次，历史数据里的空行不会带到本次编辑中。
  compactDraftCards();
}

function cancelLayoutEdit() {
  cardDrawerOpen.value = false;
  editing.value = false;
  draftCards.value = [];
  deletedCardIds.clear();
  announcementDraft.value = announcementDraftFrom(announcement.value);
  announcementDirty.value = false;
  stopCardDrag();
  stopCardResize();
}

function cloneCard(card) {
  return {
    ...card,
    config: structuredCloneSafe(card.config || {}),
    file_ids: [],
    pending_file_ids: [...(card.pending_file_ids || [])],
  };
}

function structuredCloneSafe(value) {
  return JSON.parse(JSON.stringify(value || {}));
}

function addCard(type) {
  if (!editing.value) startLayoutEdit();
  openCardDrawer("create");
  if (typeof type === "string")
    cardDrawerCard.value = { card_type: type, ...nextCardLayout.value };
}

function openCardDrawer(mode, card = null) {
  if (!canEditDashboard(selectedDashboard.value)) return;
  if (!editing.value) startLayoutEdit();
  if (mode === "create" && draftCards.value.length >= 100) {
    message.warning(t("dashboard_card_limit"));
    return;
  }
  cardDrawerMode.value = mode;
  cardDrawerCard.value = card ? cloneCard(card) : null;
  cardDrawerOpen.value = true;
  if (isIssueWorkspace.value) void loadRoles();
}

function handleCardAction({ key, card }) {
  if (key === "edit") openCardDrawer("edit", card);
  if (key === "title-change") saveDraftCard(card);
  if (key === "delete") {
    if (!editing.value) startLayoutEdit();
    deleteDraftCard(card);
  }
  if (key === "copy") openMetricCopy(card);
}

function refreshTextCard() {
  if (selectedDashboard.value)
    loadSelectedDashboard(selectedDashboard.value.id);
}

function openTextPreview(card) {
  textPreviewCard.value = card;
  textPreviewOpen.value = true;
}

function saveDraftCard(card) {
  if (!editing.value) return;
  if (
    card.card_type === "announcement" &&
    draftCards.value.some(
      (item) => item.card_type === "announcement" && item.id !== card.id,
    )
  ) {
    message.warning(t("dashboard_announcement_limit"));
    return;
  }
  const pendingFileIds = card.file_ids || [];
  const draftCard = {
    ...card,
    file_ids: [],
    pending_file_ids: pendingFileIds,
  };
  if (card.card_type === "announcement") {
    announcementDraft.value = {
      ...(announcementDraft.value || {}),
      content: card.content,
      content_mode: card.content_mode,
      file_ids: [],
      pending_file_ids: pendingFileIds,
    };
    announcementDirty.value = true;
  }
  if (card.id) {
    const index = draftCards.value.findIndex(
      (item) => String(item.id) === String(card.id),
    );
    if (index >= 0) {
      draftCards.value.splice(index, 1, draftCard);
    }
    return;
  }
  const id = `tmp-${Date.now()}-${draftCards.value.length}`;
  const position = findOpenPosition(
    draftCards.value,
    { w: draftCard.w, h: draftCard.h },
    0,
    0,
  );
  draftCards.value.push({ ...draftCard, ...position, id });
  compactDraftCards();
}

function deleteDraftCard(card) {
  const index = draftCards.value.findIndex(
    (item) => String(item.id) === String(card.id),
  );
  if (index < 0) return;
  draftCards.value.splice(index, 1);
  if (card.card_type === "announcement") {
    announcementDraft.value = announcementDraftFrom(announcement.value);
    announcementDirty.value = false;
  }
  if (!String(card.id).startsWith("tmp-")) {
    deletedCardIds.set(Number(card.id), Number(card.revision));
  }
  // 删除后下方卡片补位，避免留下整行空档。
  compactDraftCards();
}

async function saveLayoutEdit() {
  if (!selectedDashboard.value) return;
  saving.value = true;
  stopCardDrag();
  try {
    // 提交前再压实一次，服务端存下的就是用户看到的最终位置。
    compactDraftCards();
    await updateDashboardLayout(props.wsId, selectedDashboard.value.id, {
      revision: selectedDashboard.value.revision,
      cards: draftCards.value.map(cardPayload),
      deleted_cards: [...deletedCardIds.entries()].map(([id, revision]) => ({
        id,
        revision,
      })),
      announcement: announcementDirty.value
        ? {
            content: announcementDraft.value?.content || "",
            content_mode: announcementDraft.value?.content_mode || "rich-text",
            file_ids: announcementDraft.value?.pending_file_ids || [],
            revision: announcementDraft.value?.revision,
          }
        : undefined,
    });
    editing.value = false;
    await loadSelectedDashboard(selectedDashboard.value.id);
    message.success(t("common_saveSuccess"));
  } catch {
    // Keep the draft in place so the user can retry after a transient failure.
  } finally {
    saving.value = false;
  }
}

function cardPayload(card) {
  return {
    ...(String(card.id).startsWith("tmp-")
      ? {}
      : { id: Number(card.id), revision: Number(card.revision) }),
    card_type: card.card_type,
    title: card.title?.trim() || "",
    config: card.config || {},
    content: card.card_type === "text" ? card.content || "" : "",
    content_mode:
      card.card_type === "text" ? card.content_mode || "rich-text" : "",
    file_ids: card.card_type === "text" ? card.pending_file_ids || [] : [],
    x: Number(card.x),
    y: Number(card.y),
    w: Number(card.w),
    h: Number(card.h),
  };
}

function displayCard(card) {
  if (card.card_type !== "announcement") return card;
  return {
    ...card,
    content: announcementDraft.value?.content || "",
    content_mode: announcementDraft.value?.content_mode || "rich-text",
    file_ids: [],
  };
}

function openMetricCopy(card) {
  if (editing.value || String(card.id).startsWith("tmp-")) {
    message.warning(t("dashboard_copy_metric_save_first"));
    return;
  }
  metricCopyCard.value = card;
  metricCopyTargetId.value = [];
  metricCopyOpen.value = true;
}

async function copyMetricCard() {
  if (!metricCopyTargetId.value.length || !metricCopyCard.value) {
    message.warning(t("dashboard_copy_metric_target_required"));
    return;
  }
  if (metricCopySaving.value) return;
  metricCopySaving.value = true;
  try {
    await copyDashboardCards(
      props.wsId,
      selectedDashboard.value.id,
      metricCopyCard.value.id,
      { target_dashboard_ids: metricCopyTargetId.value },
    );
    const results = await Promise.allSettled(
      metricCopyTargetId.value.map((id) => getDashboard(props.wsId, id)),
    );
    for (const result of results) {
      if (result.status === "fulfilled")
        replaceDashboard(normalizeDashboard(result.value));
    }
    metricCopyOpen.value = false;
    message.success(t("dashboard_copy_metric_success"));
  } catch {
    // The shared request interceptor displays the failure.
  } finally {
    metricCopySaving.value = false;
  }
}

function applyTopFilters(filters) {
  clearTimeout(filtersSaveTimer);
  userState.value.filters = normalizeFilters(filters);
  persistUserState(true);
}

defineExpose({
  searchExpanded: computed(() => !userState.value.filters_collapsed),
  toggleSearch: () =>
    updateFiltersCollapsed(!userState.value.filters_collapsed),
});

function updateFiltersCollapsed(value) {
  userState.value.filters_collapsed = value;
  clearTimeout(filtersSaveTimer);
  filtersSaveTimer = setTimeout(persistUserState, 300);
}

async function persistUserState(refreshMetrics = false) {
  if (!selectedDashboard.value) return;
  const dashboardId = selectedDashboard.value.id;
  const payload = {
    filters: structuredCloneSafe(userState.value.filters),
    filters_collapsed: userState.value.filters_collapsed,
  };
  const wsId = props.wsId;
  try {
    const save = filtersSaveQueue.then(() =>
      updateDashboardUserState(wsId, dashboardId, payload),
    );
    filtersSaveQueue = save.catch(() => {
      /* Allow a later explicit save after failure. */
    });
    await save;
    if (String(selectedDashboard.value?.id) !== String(dashboardId)) return;
    filterStateSaveFailed.value = false;
    if (refreshMetrics) filtersRevision.value += 1;
  } catch {
    if (String(selectedDashboard.value?.id) === String(dashboardId)) {
      // Keep the local selection visible, but do not refresh metrics: the server still uses its prior saved state.
      filterStateSaveFailed.value = true;
    }
  }
}

function cardGridStyle(card) {
  return {
    gridColumn: `${Math.max(0, Number(card.x) || 0) + 1} / span ${Math.max(1, Number(card.w) || 1)}`,
    gridRow: `${Math.max(0, Number(card.y) || 0) + 1} / span ${Math.max(1, Number(card.h) || 1)}`,
  };
}

const placeholderStyle = computed(() => {
  const geometry = dragGeometry.value;
  const cell = dragPreview.value;
  if (!geometry || !cell) return null;
  return {
    left: `${geometry.padLeft + cell.x * geometry.columnStep}px`,
    top: `${geometry.padTop + cell.y * geometry.rowStep}px`,
    width: `${cell.w * geometry.columnWidth + (cell.w - 1) * geometry.columnGap}px`,
    height: `${cell.h * geometry.rowHeight + (cell.h - 1) * geometry.rowGap}px`,
  };
});

const guideCellCount = computed(() => {
  const rows = cardsForGrid.value.reduce(
    (max, card) => Math.max(max, (Number(card.y) || 0) + (Number(card.h) || 1)),
    0,
  );
  return GRID_COLUMNS * Math.max(8, rows + 1);
});

// 拖拽期间不做 DOM 重排：卡片位置由网格坐标驱动，浮层只负责跟手，占位框只负责预告落点。
function gridItemStyle(card) {
  const overlay = dragOverlay.value;
  if (
    overlay &&
    String(draggedCardId.value) === String(card.id) &&
    editing.value
  ) {
    return {
      position: "fixed",
      left: "0px",
      top: "0px",
      width: `${overlay.width}px`,
      height: `${overlay.height}px`,
      margin: "0",
      transform: `translate3d(${overlay.left}px, ${overlay.top}px, 0)`,
      zIndex: 30,
      pointerEvents: "none",
    };
  }
  return cardGridStyle(card);
}

function measureGrid() {
  const grid = gridRef.value;
  if (!grid) return null;
  const rect = grid.getBoundingClientRect();
  const style = getComputedStyle(grid);
  const padLeft = parseFloat(style.paddingLeft) || 0;
  const padTop = parseFloat(style.paddingTop) || 0;
  const padRight = parseFloat(style.paddingRight) || 0;
  const columnGap = parseFloat(style.columnGap) || 0;
  const rowGap = parseFloat(style.rowGap) || 0;
  // 12 列之间有 11 个列间距，行之间同理：步长 = 单元格尺寸 + 间距。
  const columnWidth =
    (rect.width - padLeft - padRight - columnGap * (GRID_COLUMNS - 1)) /
    GRID_COLUMNS;
  const rowHeight = parseFloat(style.gridAutoRows) || 0;
  return {
    left: rect.left + padLeft,
    top: rect.top + padTop,
    padLeft,
    padTop,
    columnWidth,
    rowHeight,
    columnGap,
    rowGap,
    columnStep: columnWidth + columnGap,
    rowStep: rowHeight + rowGap,
  };
}

function cellFromCardOrigin(geometry, left, top, card) {
  return {
    x: clamp(
      Math.round((left - geometry.left) / geometry.columnStep),
      0,
      GRID_COLUMNS - card.w,
    ),
    y: Math.max(0, Math.round((top - geometry.top) / geometry.rowStep)),
  };
}

function startCardDrag({ event, card }) {
  if (!editing.value || !canEditDashboard(selectedDashboard.value)) return;
  if (event.button !== 0 || gridDragSession) return;
  const item = event.target?.closest?.(".dashboard-grid__item");
  const geometry = measureGrid();
  if (!item || !geometry) return;
  stopCardResize();
  const rect = item.getBoundingClientRect();
  gridDragSession = {
    id: card.id,
    width: rect.width,
    height: rect.height,
    offsetX: event.clientX - rect.left,
    offsetY: event.clientY - rect.top,
    pointerX: event.clientX,
    pointerY: event.clientY,
  };
  draggedCardId.value = String(card.id);
  dragGeometry.value = geometry;
  dragOverlay.value = {
    left: rect.left,
    top: rect.top,
    width: rect.width,
    height: rect.height,
  };
  dragPreview.value = { x: card.x, y: card.y, w: card.w, h: card.h };
  document.body.classList.add("dashboard-card-dragging");
  window.addEventListener("pointermove", moveCardDrag);
  window.addEventListener("pointerup", endCardDrag);
  window.addEventListener("pointercancel", stopCardDrag);
}

function moveCardDrag(event) {
  const session = gridDragSession;
  if (!session) return;
  session.pointerX = event.clientX;
  session.pointerY = event.clientY;
  if (dragFrame) return;
  dragFrame = requestAnimationFrame(() => {
    dragFrame = 0;
    applyCardDragFrame();
  });
}

function applyCardDragFrame() {
  const session = gridDragSession;
  const moved = draftCards.value.find(
    (card) => String(card.id) === String(session?.id),
  );
  const geometry = measureGrid();
  if (!session || !moved || !geometry) return;
  dragGeometry.value = geometry;
  const left = session.pointerX - session.offsetX;
  const top = session.pointerY - session.offsetY;
  dragOverlay.value = { ...dragOverlay.value, left, top };
  dragPreview.value = resolveDropLayout(
    layoutSnapshot(draftCards.value),
    session.id,
    cellFromCardOrigin(geometry, left, top, moved),
  );
}

function endCardDrag() {
  const session = gridDragSession;
  if (!session) return;
  if (dragFrame) {
    cancelAnimationFrame(dragFrame);
    dragFrame = 0;
  }
  applyCardDragFrame();
  const preview = dragPreview.value;
  const cardId = session.id;
  stopCardDrag();
  if (preview) applyGridDrop(cardId, preview);
}

function stopCardDrag() {
  gridDragSession = null;
  if (dragFrame) {
    cancelAnimationFrame(dragFrame);
    dragFrame = 0;
  }
  draggedCardId.value = null;
  dragPreview.value = null;
  dragOverlay.value = null;
  dragGeometry.value = null;
  document.body.classList.remove("dashboard-card-dragging");
  window.removeEventListener("pointermove", moveCardDrag);
  window.removeEventListener("pointerup", endCardDrag);
  window.removeEventListener("pointercancel", stopCardDrag);
}

// 把指针算出的目标格落到实际布局：先插入目标格，被压住的卡片依次下移，再整体向上压实。
function applyGridDrop(cardId, target) {
  const layout = layoutSnapshot(draftCards.value);
  const result = resolveDropLayout(layout, cardId, target);
  if (!result) return;
  for (const item of layout) {
    const card = draftCards.value.find(
      (entry) => String(entry.id) === String(item.id),
    );
    if (!card) continue;
    card.x = item.x;
    card.y = item.y;
  }
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

// 只取布局字段做快照：拖拽时每帧都要试算，避免复制卡片正文与配置。
function layoutSnapshot(cards) {
  return cards.map((card) => ({
    id: card.id,
    hidden: !isCardVisible(card),
    x: Math.max(0, Number(card.x) || 0),
    y: Math.max(0, Number(card.y) || 0),
    w: Math.max(1, Number(card.w) || 1),
    h: Math.max(1, Number(card.h) || 1),
  }));
}

function columnsOverlap(left, right) {
  return left.x < right.x + right.w && right.x < left.x + left.w;
}

// 被锚点压住的卡片依次下移，直到整片区域不再重叠。
function pushOverlappingLayouts(items, anchorId) {
  const queue = [String(anchorId)];
  const visited = new Set();
  let moves = 0;
  while (queue.length && moves < items.length * 2) {
    const currentId = queue.shift();
    if (visited.has(currentId)) continue;
    visited.add(currentId);
    const anchor = items.find((item) => String(item.id) === currentId);
    if (!anchor) continue;
    for (const item of items) {
      if (String(item.id) === currentId || !layoutsOverlap(anchor, item))
        continue;
      item.y = anchor.y + anchor.h;
      queue.push(String(item.id));
      moves += 1;
    }
  }
}

// 逐卡向上压实：按 y 升序把每张卡放到最靠上的可放置行。
// 压实后布局不再含整行空档，因此拖拽预告的位置与保存后的结果一致。
function compactLayouts(items) {
  const hidden = (item) => item.hidden ?? !isCardVisible(item);
  const sorted = [...items].sort(
    (a, b) => Number(hidden(a)) - Number(hidden(b)) || a.y - b.y || a.x - b.x,
  );
  const placed = [];
  for (const item of sorted) {
    let row = 0;
    let settled = false;
    while (!settled) {
      settled = true;
      for (const other of placed) {
        if (!columnsOverlap(item, other)) continue;
        if (row < other.y + other.h && other.y < row + item.h) {
          row = other.y + other.h;
          settled = false;
        }
      }
    }
    item.y = row;
    placed.push(item);
  }
}

// 在快照上试算落位：插入目标格 → 压住的卡片下移 → 整体向上压实。
function resolveDropLayout(items, cardId, target) {
  const moved = items.find((item) => String(item.id) === String(cardId));
  if (!moved) return null;
  moved.x = clamp(target.x, 0, GRID_COLUMNS - moved.w);
  moved.y = Math.max(0, Number(target.y) || 0);
  pushOverlappingLayouts(items, moved.id);
  compactLayouts(items);
  return { x: moved.x, y: moved.y, w: moved.w, h: moved.h };
}

// 把草稿卡片按压实后的坐标回写（原地修改，保留响应式引用）。
function compactDraftCards() {
  const layout = layoutSnapshot(draftCards.value);
  compactLayouts(layout);
  for (const item of layout) {
    const card = draftCards.value.find(
      (entry) => String(entry.id) === String(item.id),
    );
    if (!card) continue;
    card.x = item.x;
    card.y = item.y;
  }
}

function findOpenPosition(
  cards,
  layout,
  startX = 0,
  startY = 0,
  excludeId = null,
) {
  const w = Math.min(12, Math.max(1, Number(layout.w) || 1));
  const h = Math.min(1000000, Math.max(1, Number(layout.h) || 1));
  const origin = {
    x: Math.max(0, Math.min(12 - w, Number(startX) || 0)),
    y: Math.max(0, Math.min(1000000 - h, Number(startY) || 0)),
    w,
    h,
  };
  if (canPlace(cards, origin, excludeId)) return origin;
  const candidateRows = [
    ...new Set([0, ...cards.map((card) => Number(card.y) + Number(card.h))]),
  ].sort((a, b) => a - b);
  for (const y of candidateRows) {
    if (y > 1000000 - h) continue;
    for (let x = 0; x <= 12 - w; x += 1) {
      const candidate = { x, y, w, h };
      if (canPlace(cards, candidate, excludeId)) return candidate;
    }
  }
  return null;
}

function canPlace(cards, candidate, excludeId) {
  return cards.every((card) => {
    if (String(card.id) === String(excludeId)) return true;
    return !layoutsOverlap(candidate, card);
  });
}

function layoutsOverlap(left, right) {
  return (
    left.x < right.x + right.w &&
    right.x < left.x + left.w &&
    left.y < right.y + right.h &&
    right.y < left.y + left.h
  );
}

function startCardResize({ event, card }) {
  if (!editing.value || !gridRef.value) return;
  resizeSession = {
    id: card.id,
    origin: { x: card.x, y: card.y, w: card.w, h: card.h },
    startX: event.clientX,
    startY: event.clientY,
    columnWidth:
      (gridRef.value.clientWidth -
        parseFloat(getComputedStyle(gridRef.value).paddingLeft) * 2 +
        parseFloat(getComputedStyle(gridRef.value).columnGap)) /
      12,
    rowHeight:
      parseFloat(getComputedStyle(gridRef.value).gridAutoRows) +
      parseFloat(getComputedStyle(gridRef.value).rowGap),
  };
  window.addEventListener("pointermove", resizeCard);
  window.addEventListener("pointerup", stopCardResize, { once: true });
  window.addEventListener("pointercancel", stopCardResize, { once: true });
}

function resizeCard(event) {
  if (!resizeSession) return;
  const card = draftCards.value.find(
    (item) => String(item.id) === String(resizeSession.id),
  );
  if (!card) return;
  const next = {
    x: resizeSession.origin.x,
    y: resizeSession.origin.y,
    w: Math.max(
      1,
      Math.min(
        12 - resizeSession.origin.x,
        resizeSession.origin.w +
          Math.round(
            (event.clientX - resizeSession.startX) / resizeSession.columnWidth,
          ),
      ),
    ),
    h: Math.max(
      1,
      resizeSession.origin.h +
        Math.round(
          (event.clientY - resizeSession.startY) / resizeSession.rowHeight,
        ),
    ),
  };
  if (next.y + next.h > 1000000) return;
  card.w = next.w;
  card.h = next.h;
  pushOverlappingCards(card.id);
}

function pushOverlappingCards(anchorId) {
  pushOverlappingLayouts(draftCards.value, anchorId);
}

function stopCardResize() {
  const wasResizing = Boolean(resizeSession);
  resizeSession = null;
  window.removeEventListener("pointermove", resizeCard);
  window.removeEventListener("pointerup", stopCardResize);
  window.removeEventListener("pointercancel", stopCardResize);
  // 缩小卡片后留下的空档在松手时统一向上压实。
  if (wasResizing && editing.value) compactDraftCards();
}
</script>

<style scoped lang="less">
.dashboard-panel {
  min-height: 0;
  height: 0;
  flex: 1;
  overflow-x: hidden;
  overflow-y: auto;
  color: #15284b;
}
.dashboard-panel__layout {
  position: relative;
  display: grid;
  grid-template-columns: clamp(150px, 16vw, 224px) minmax(0, 1fr);
  min-height: calc(100vh - 120px);
}
.dashboard-panel__layout--collapsed {
  grid-template-columns: minmax(0, 1fr);
}
.dashboard-panel__toggle {
  position: absolute;
  top: min(45vh, 400px);
  left: clamp(150px, 16vw, 224px);
  z-index: 21;
  width: 20px;
  height: 42px;
  padding: 0;
  transform: translateX(-50%);
  border: 1px solid #f0f0f0;
  border-radius: 7px;
  background: #fff;
  color: #8c8c8c;
  cursor: pointer;
}
.dashboard-panel__layout--collapsed .dashboard-panel__toggle {
  left: 0;
  transform: none;
}
.dashboard-dialog-template {
  width: 100%;
  margin-bottom: 12px;
}
.dashboard-main {
  min-width: 0;
}
.dashboard-main__head {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 12px;
  padding: 0 20px 14px;
  background: #fff;
}
.dashboard-panel--editing .dashboard-main__head {
  position: sticky;
  top: 0;
  z-index: 20;
  box-shadow: 0 2px 8px rgb(21 40 75 / 8%);
}
.dashboard-main__title,
.dashboard-main__toggle {
  flex: none;
  border: 0;
  background: transparent;
  color: #262626;
}
.dashboard-main__title {
  max-width: 280px;
  margin: 0;
  padding: 4px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 18px;
  font-weight: 600;
}
button.dashboard-main__title,
.dashboard-main__toggle {
  cursor: pointer;
}
.dashboard-main__actions {
  display: flex;
  flex: none;
  gap: 8px;
  margin-left: auto;
}
.dashboard-main__filter-alert {
  margin: 0 20px 12px;
}
.dashboard-sidebar {
  min-width: 0;
  color: #262626;
  background: #fff;
  border-right: 1px solid #f0f0f0;
}
.dashboard-sidebar--overlay {
  position: absolute;
  top: 38px;
  left: 20px;
  z-index: 30;
  width: min(260px, calc(100vw - 48px));
  border: 0;
  border-radius: 8px;
  box-shadow: 0 6px 24px rgb(0 0 0 / 12%);
}
.dashboard-sidebar__heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
}
.dashboard-sidebar__head {
  padding: 14px 14px 8px;
}
.dashboard-sidebar__list {
  max-height: 65vh;
  overflow: auto;
  padding: 0 8px 8px;
}
.dashboard-sidebar__item {
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 42px;
  border-radius: 6px;
  &:hover {
    background: #f5f7fa;
  }
}
.dashboard-sidebar__item--active {
  color: #1677ff;
  background: #e6f0ff;
}
.dashboard-sidebar__drag {
  flex: 0 0 22px;
  color: #bfbfbf;
  cursor: grab;
}
.dashboard-sidebar__select {
  flex: 1;
  min-width: 0;
  padding: 10px 4px;
  text-align: left;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
}
.dashboard-sidebar__select span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dashboard-canvas {
  padding: 12px;
  min-height: calc(100vh - 170px);
  background: #f0f2f5;
}
.dashboard-grid {
  position: relative;
  min-height: calc(100vh - 210px);
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  grid-auto-rows: 88px;
  align-content: start;
  gap: 12px;
}
.dashboard-grid--editing {
  padding: 8px;
}
.dashboard-grid--empty {
  min-height: 0;
  grid-auto-rows: auto;
}
.dashboard-grid__guides {
  position: absolute;
  inset: 8px;
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  grid-auto-rows: 88px;
  gap: 12px;
  overflow: hidden;
  pointer-events: none;
  span {
    border: 1px dashed #cfd7e4;
    border-radius: 4px;
  }
}
.dashboard-grid__item {
  min-width: 0;
  min-height: 0;
  z-index: 1;
}
.dashboard-grid--editing .dashboard-grid__item {
  cursor: move;
}
.dashboard-grid__item--dragging {
  // 半透明跟手：透出下方的预测落点框，同时保留卡片轮廓作为拖拽反馈。
  opacity: 0.55;
  cursor: grabbing;
}
.dashboard-grid__item--dragging :deep(.dashboard-card) {
  box-shadow: 0 14px 32px rgb(21 40 75 / 24%);
}
// 预测落点：绝对定位在网格上，位置由单元格坐标换算，不受 DOM 顺序影响。
.dashboard-grid__placeholder {
  position: absolute;
  z-index: 2;
  border: 2px dashed #1677ff;
  border-radius: 10px;
  background: rgb(22 119 255 / 16%);
  pointer-events: none;
}
:global(body.dashboard-card-dragging) {
  cursor: grabbing;
  user-select: none;
}
.dashboard-grid__empty {
  grid-column: 1 / -1;
  min-height: 0;
  padding: 54px 32px;
  background: #fff;
  border-radius: 10px;
  text-align: center;
}
.dashboard-grid__empty h1 {
  margin: 0 0 12px;
  color: #15284b;
  font-size: 24px;
  font-weight: 600;
}
.dashboard-grid__empty > p {
  max-width: 710px;
  margin: 0 auto 34px;
  color: #8c96a6;
  font-size: 15px;
  line-height: 1.9;
}
.dashboard-grid__empty :deep(.dashboard-card-types) {
  max-width: 900px;
  margin: auto;
  text-align: left;
}
.dashboard-panel__empty {
  padding: 80px 0;
}
.dashboard-panel__copy-select {
  width: 100%;
}
@media (max-width: 900px) {
  .dashboard-main__head {
    flex-wrap: wrap;
    padding-inline: 12px;
  }
  .dashboard-main__actions {
    flex-wrap: wrap;
  }
  .dashboard-canvas {
    padding: 12px;
  }
  .dashboard-grid__empty {
    padding: 32px 16px;
  }
}
</style>
