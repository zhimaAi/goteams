<template>
  <div ref="pageRef" class="pipeline-page">
    <header class="pipeline-header">
      <div class="page-identity">
        <span class="page-icon"><RobotOutlined /></span>
        <h3 class="pipeline-title">{{ $t("menu_pipeline") }}</h3>
        <span class="page-description">{{ $t("menu_pipeline_desc") }}</span>
      </div>

      <a-tabs
        v-model:activeKey="activeMainTab"
        class="pipeline-main-tabs"
        @change="handleMainTabChange"
      >
        <a-tab-pane key="pipelines" :tab="$t('pipeline_tab_orchestration')" />
        <a-tab-pane key="team" :tab="$t('pipeline_tab_team')" />
        <a-tab-pane key="logs" :tab="$t('pipeline_tab_logs')" />
      </a-tabs>

      <span class="header-spacer" />
      <div class="pipeline-header-actions">
        <a-button
          type="link"
          class="setup-guide-button"
          :disabled="!canViewPipeline"
          @click="goSetupGuide"
        >
          {{ $t("pipeline_setup_guide") }}
        </a-button>
        <a-button type="primary" @click="openDownload">
          <template #icon><DownloadOutlined /></template>
          {{ $t("agent_download_client") }}
        </a-button>
      </div>
    </header>

    <div
      v-if="!canViewPipeline"
      class="pipeline-main-content pipeline-no-permission"
      role="status"
    >
      <a-empty :description="$t('pipeline_no_permission')">
        <template #image><FileProtectOutlined /></template>
      </a-empty>
    </div>

    <!-- Pipeline orchestration tab: card list + agent orchestration editor -->
    <PipelineSettingsPanel
      v-else-if="activeMainTab === 'pipelines'"
      class="pipeline-main-content"
    />

    <!-- Expert-team tab: cloud-configured expert groups (list + orchestration) -->
    <ExpertGroupSettingsPanel
      v-else-if="activeMainTab === 'team'"
      class="pipeline-main-content"
    />

    <!-- Task logs tab: client execution records synced from TeamsBoard -->
    <div v-else class="pipeline-main-content pipeline-logs">
      <div class="logs-heading">
        <h4 class="logs-title">{{ $t("pipeline_task_logs") }}</h4>
        <span class="logs-subtitle">{{
          $t("pipeline_task_logs_subtitle")
        }}</span>
      </div>

      <a-tabs
        v-model:activeKey="activePeriod"
        class="pipeline-period-tabs"
        @change="handlePeriodChange"
      >
        <a-tab-pane key="today">
          <template #tab>
            <span class="period-tab-label"
              >{{ $t("pipeline_stat_today") }}<em>{{ stats.today }}</em></span
            >
          </template>
        </a-tab-pane>
        <a-tab-pane key="yesterday">
          <template #tab>
            <span class="period-tab-label"
              >{{ $t("pipeline_stat_yesterday")
              }}<em>{{ stats.yesterday }}</em></span
            >
          </template>
        </a-tab-pane>
        <a-tab-pane key="week">
          <template #tab>
            <span class="period-tab-label"
              >{{ $t("pipeline_stat_week") }}<em>{{ stats.week }}</em></span
            >
          </template>
        </a-tab-pane>
        <a-tab-pane key="all">
          <template #tab>
            <span class="period-tab-label"
              >{{ $t("common_all") }}<em>{{ stats.all }}</em></span
            >
          </template>
        </a-tab-pane>
      </a-tabs>

      <!-- Filter area -->
      <div class="pipeline-filters">
        <a-input-search
          v-model:value="filters.keyword"
          class="pipeline-filter-search"
          :placeholder="$t('pipeline_search_placeholder')"
          allow-clear
          @change="handleSearchInputChange"
          @search="handleSearch"
        />
        <div class="pipeline-toolbar-filter pipeline-member-filter">
          <span>{{ $t("pipeline_used_member") }}</span>
          <a-select
            v-model:value="filters.user_ids"
            popup-class-name="pipeline-filter-dropdown"
            :bordered="false"
            mode="multiple"
            :max-tag-count="1"
            allow-clear
            @change="handleFilterChange"
          >
            <a-select-option
              v-for="member in filterMembers"
              :key="member.id"
              :value="Number(member.id)"
              :title="member.name || member.username"
            >
              {{ member.name || member.username || `#${member.id}` }}
            </a-select-option>
          </a-select>
        </div>
        <div class="pipeline-toolbar-filter pipeline-date-filter">
          <span>{{ $t("pipeline_filter_started_at") }}</span>
          <a-range-picker
            v-model:value="filters.started_at_range"
            :bordered="false"
            format="YYYY-MM-DD"
            allow-clear
            @change="handleStartedAtChange"
          />
        </div>
        <a-button class="clear-filter-button" @click="clearFilters">
          <ReloadOutlined /> {{ $t("common_clearFilter") }}
        </a-button>
      </div>

      <!-- Task list -->
      <a-table
        class="pipeline-task-table"
        :columns="columns"
        :data-source="taskList"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1120 }"
        size="small"
        table-layout="fixed"
        row-key="id"
        :row-class-name="pipelineTaskRowClassName"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <button
              type="button"
              class="task-title-cell"
              @click="openRequirementDetail(record)"
            >
              <WorkItemTypePill
                :type="record.work_item_type"
                :ws-id="record.workspace_id"
              />
              <span class="task-title-link" :title="record.title">
                {{ record.title }}
              </span>
            </button>
          </template>
          <template v-else-if="column.key === 'status'">
            <StatusPill
              :text="record.status_name"
              :color="record.status_color"
            />
          </template>
          <template v-else-if="column.key === 'user_name'">
            {{ record.user_name || `#${record.user_id}` }}
          </template>
          <template v-else-if="column.key === 'pipeline_name'">
            <div class="pipeline-cell">
              {{ record.pipeline_name }}
            </div>
          </template>
          <template v-else-if="column.key === 'started_at'">
            {{ formatDateTime(record.started_at) }}
          </template>
          <template v-else-if="column.key === 'conversation_rounds'">
            {{
              $t("pipeline_rounds_count", {
                count: record.conversation_rounds || 0,
              })
            }}
          </template>
          <template v-else-if="column.key === 'total_tokens'">
            <span class="token-cell" :class="{ done: record.is_done }">
              <span class="token-icon" />
              {{ formatTokens(record.total_tokens) }}
            </span>
          </template>
          <template v-else-if="column.key === 'actions'">
            <div class="table-actions">
              <a-button
                type="link"
                size="small"
                @click="openRequirementDetail(record, record.workspace_id)"
                >{{ $t("pipeline_task_detail_action") }}</a-button
              >
            </div>
          </template>
        </template>
      </a-table>
    </div>

    <!-- Task detail drawer -->
    <a-drawer
      v-model:open="detailVisible"
      width="calc(100vw - 220px)"
      root-class-name="pipeline-task-detail-drawer"
    >
      <template #title>
        <div v-if="taskDetail" class="detail-title">
          <a-tag :color="execStatusColor(taskDetail.execution_status)">
            {{ $t("pipeline_exec_" + taskDetail.execution_status) }}
          </a-tag>
          <span>{{ taskDetail.title }}</span>
        </div>
        <span v-else>{{ $t("pipeline_task_detail") }}</span>
      </template>
      <template v-if="detailLoading">
        <div class="detail-loading"><a-spin /></div>
      </template>
      <template v-else-if="taskDetail">
        <div class="detail-meta">
          <span
            ><em>{{ $t("pipeline_started_at") }}</em
            >{{ formatTime(taskDetail.started_at) }}</span
          >
          <span
            ><em>{{ $t("pipeline_member") }}</em
            >{{ taskDetail.user_name || `#${taskDetail.user_id}` }}</span
          >
          <span
            ><em>{{ $t("pipeline_conversations") }}</em
            >{{ taskDetail.conversations?.length || 0 }}</span
          >
          <span
            ><em>{{ $t("pipeline_conversation_rounds") }}</em
            >{{ taskDetail.conversation_rounds || 0 }}</span
          >
          <span
            ><em>{{ $t("pipeline_total_tokens") }}</em
            >⚡ {{ formatTokens(taskDetail.total_tokens) }}</span
          >
          <span
            ><em>{{ $t("pipeline_name") }}</em
            >{{ taskDetail.pipeline_name }}</span
          >
        </div>

        <div class="detail-content">
          <!-- Left: pipeline steps; clicking a step shows its conversation record on the right -->
          <section class="detail-steps">
            <div class="section-title">
              <span class="section-icon section-icon-blue">⌘</span>
              {{ $t("pipeline_workflow_steps") }}
              <small>{{
                $t("pipeline_steps_count", {
                  count: taskDetail.steps?.length || 0,
                })
              }}</small>
            </div>
            <a-timeline class="steps-timeline">
              <a-timeline-item
                v-for="step in taskDetail.steps || []"
                :key="step.step_key"
                :color="stepStatusColor(step.status)"
                class="timeline-step-item"
                :class="{ active: selectedStepKey === step.step_key }"
                @click="selectStep(step.step_key)"
              >
                <div class="step-item">
                  <div class="step-header">
                    <span class="step-name">{{ step.step_name }}</span>
                    <a-tag :color="stepStatusColor(step.status)" size="small">
                      {{ $t("pipeline_step_" + step.status) }}
                    </a-tag>
                  </div>
                  <div v-if="step.error_summary" class="step-error">
                    {{ step.error_summary }}
                  </div>
                  <div class="step-meta">
                    <span>{{ formatTime(step.started_at) }}</span>
                    <span>⚡ {{ formatTokens(step.total_tokens) }}</span>
                    <span
                      >{{ $t("pipeline_rounds") }}:
                      {{ step.conversation_rounds }}</span
                    >
                  </div>
                </div>
              </a-timeline-item>
            </a-timeline>
            <a-empty
              v-if="!taskDetail.steps?.length"
              :description="$t('pipeline_no_steps')"
              :image="simpleImage"
            />
          </section>

          <!-- Right: conversation record of the selected step -->
          <section class="detail-conversation">
            <div class="section-title">
              <span class="section-icon section-icon-green">▣</span>
              {{ $t("pipeline_conversation_record") }}
              <span class="section-title-divider" />
              <TaskDocumentsEntry
                :task-id="taskDetail.id"
                variant="link"
                class="section-documents-link"
              />
              <small>{{
                $t("pipeline_step_conversations_count", {
                  count: stepConversations.length,
                })
              }}</small>
            </div>
            <div v-if="stepConversations.length" class="conversation-list">
              <template
                v-for="(conversation, conversationIndex) in stepConversations"
                :key="conversationKey(conversation, conversationIndex)"
              >
                <div class="conversation-session-header">
                  <div>
                    <strong>
                      {{
                        conversation.step_name ||
                        stepName(conversation.step_key)
                      }}
                      ·
                      {{
                        $t("pipeline_rounds_count", {
                          count: conversation.conversation?.round_count || 0,
                        })
                      }}
                    </strong>
                    <span>
                      {{
                        (
                          conversation.cli_type ||
                          taskDetail.cli_type ||
                          ""
                        ).toUpperCase()
                      }}
                      <template v-if="conversation.duration_ms">
                        · {{ formatDuration(conversation.duration_ms) }}
                      </template>
                    </span>
                  </div>
                  <a-tag :color="sessionStatusColor(conversation.status)">
                    {{ $t("pipeline_session_" + conversation.status) }}
                  </a-tag>
                </div>
                <div
                  v-for="round in conversation.conversation?.rounds || []"
                  :key="round.session_uuid || round.round_no"
                  class="conversation-round"
                >
                  <div class="round-label">
                    {{ $t("pipeline_round_label", { round: round.round_no }) }}
                  </div>
                  <div
                    v-if="round.user_message"
                    class="message-row message-row-user"
                  >
                    <div
                      class="message-bubble message-user message-markdown"
                      @click="onMarkdownClick"
                      v-html="
                        renderConversationMarkdown(
                          round.user_message,
                          taskDetail.documents || [],
                        )
                      "
                    ></div>
                  </div>
                  <ExecutionProcessTimeline
                    :events="round.events"
                    :status="conversation.status"
                    :cli-type="conversation.cli_type || taskDetail.cli_type"
                    :model-name="conversation.model_name"
                    :input-tokens="Number(round.usage?.input_tokens || 0)"
                    :output-tokens="Number(round.usage?.output_tokens || 0)"
                  />
                  <div
                    v-for="aiMessage in round.assistant_messages || []"
                    :key="`${round.session_uuid}:${aiMessage.sequence}`"
                    class="message-row message-row-ai"
                  >
                    <div class="ai-avatar">
                      {{ (taskDetail.pipeline_name || "AI").slice(0, 1) }}
                    </div>
                    <div>
                      <div
                        class="message-bubble message-ai message-markdown"
                        @click="onMarkdownClick"
                        v-html="
                          renderConversationMarkdown(
                            aiMessage.content,
                            taskDetail.documents || [],
                          )
                        "
                      ></div>
                      <div class="message-time">
                        {{
                          formatMessageTime(
                            aiMessage.timestamp,
                            round.finished_at || round.started_at,
                          )
                        }}
                      </div>
                    </div>
                  </div>
                  <div
                    v-if="!round.assistant_messages?.length"
                    class="message-row message-row-ai"
                  >
                    <div class="ai-avatar">
                      {{ (taskDetail.pipeline_name || "AI").slice(0, 1) }}
                    </div>
                    <div class="message-bubble message-ai">
                      {{ $t("pipeline_no_ai_response") }}
                    </div>
                  </div>
                  <a-alert
                    v-if="round.error_summary"
                    type="error"
                    show-icon
                    :message="round.error_summary"
                    class="round-error"
                  />
                  <div class="round-meta">
                    {{ formatTime(round.finished_at || round.started_at) }}
                    · ⚡ {{ formatTokens(round.usage?.total_tokens || 0) }}
                  </div>
                </div>
              </template>
            </div>
            <a-empty
              v-else
              :description="$t('pipeline_select_step')"
              :image="simpleImage"
            />
          </section>
        </div>
      </template>
    </a-drawer>

    <!-- Requirement detail sidebar -->
    <RequirementDetailDrawer
      :open="!!currentWorkItemId"
      :ws-id="currentWsId"
      :requirement-id="currentWorkItemId"
      :initial-tab="currentDetailTab"
      :can-edit-by-type="{
        requirement: authStore.hasPermission('requirement:edit'),
        defect: authStore.hasPermission('defect:edit'),
      }"
      :can-create-by-type="{
        requirement: authStore.hasPermission('requirement:create'),
        defect: authStore.hasPermission('defect:create'),
      }"
      :navigation-items="pipelineNavigationItems"
      @close="closeRequirementDetail"
      @navigate="openRequirementDetail"
      @updated="handleRequirementUpdated"
    />

    <!-- 下载客户端弹窗（共享组件） -->
    <AgentDownloadModal v-model:open="showDownloadModal" />

    <!-- 沟通记录里的关联文档：抽屉形态读文档，不打断日志详情 -->
    <TaskDocumentsDrawer
      v-model:open="documentsDrawerOpen"
      v-model:document-id="documentsFocusId"
      :task-id="taskDetail ? taskDetail.id : null"
    />
  </div>
</template>

<script setup>
defineOptions({ name: "AgentPage" });

import {
  ref,
  reactive,
  computed,
  nextTick,
  onMounted,
  onActivated,
  onBeforeUnmount,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Empty, message } from "ant-design-vue";
import {
  DownloadOutlined,
  FileProtectOutlined,
  RobotOutlined,
  ReloadOutlined,
} from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import AgentDownloadModal from "@/components/agent/AgentDownloadModal.vue";
import PipelineSettingsPanel from "@/views/pipeline/PipelineSettingsPanel.vue";
import ExpertGroupSettingsPanel from "@/views/pipeline/ExpertGroupSettingsPanel.vue";
import { listPipelineTasks, getPipelineTaskDetail } from "@/api/pipeline";
import ExecutionProcessTimeline from "@/components/requirement/ExecutionProcessTimeline.vue";
import {
  documentIdFromEvent,
  renderConversationMarkdown,
} from "@/components/requirement/conversationMarkdown.js";
import TaskDocumentsDrawer from "@/components/pipeline/TaskDocumentsDrawer.vue";
import TaskDocumentsEntry from "@/components/pipeline/TaskDocumentsEntry.vue";
import { setSecondaryKey } from "@/composables/usePageTitle";
import { getRememberedTab, setRememberedTab } from "@/utils/rememberedTab";

const router = useRouter();
const route = useRoute();
const { t: $t } = useI18n();
const authStore = useAuthStore();
const canViewPipeline = computed(() =>
  authStore.hasEnterprisePermission("pipeline:view"),
);

const MAIN_TABS = ["pipelines", "team", "logs"];
const activeMainTab = ref(
  getRememberedTab(
    authStore.userInfo?.id,
    "pipeline-main",
    MAIN_TABS,
    "pipelines",
  ),
);
// The URL query wins over the remembered tab so deep links (e.g. the settings
// redirect) always land on the requested tab.
if (MAIN_TABS.includes(route.query.tab)) {
  activeMainTab.value = route.query.tab;
}
watch(activeMainTab, (tab) => {
  setRememberedTab(authStore.userInfo?.id, "pipeline-main", tab);
  setSecondaryKey(
    tab === "pipelines"
      ? "pipeline_tab_orchestration"
      : tab === "team"
        ? "pipeline_tab_team"
        : periodTitle[activePeriod.value] || "",
  );
});
function handleMainTabChange(tab) {
  router.replace({ query: { ...route.query, tab } });
}

const loading = ref(false);
const taskList = ref([]);
const pipelineNavigationItems = computed(() => {
  const seen = new Set();
  return taskList.value.flatMap((item) => {
    const id = Number(item.work_item_id);
    const workspaceId = Number(item.workspace_id);
    const key = `${workspaceId}:${id}`;
    if (!id || !workspaceId || seen.has(key)) return [];
    seen.add(key);
    return [{ id, workspaceId }];
  });
});
const periodTitle = {
  today: "pipeline_stat_today",
  yesterday: "pipeline_stat_yesterday",
  week: "pipeline_stat_week",
  all: "common_all",
};
const activePeriod = ref(
  getRememberedTab(
    authStore.userInfo?.id,
    "pipeline",
    Object.keys(periodTitle),
    "all",
  ),
);
watch(
  activePeriod,
  (period) => {
    if (activeMainTab.value === "logs") {
      setSecondaryKey(periodTitle[period] || "");
    }
  },
  {
    immediate: true,
  },
);
const stats = reactive({ all: 0, today: 0, yesterday: 0, week: 0 });
const filterMembers = ref([]);
const pagination = reactive({ current: 1, pageSize: 20, total: 0 });
const filters = reactive({ keyword: "", user_ids: [], started_at_range: null });

const detailVisible = ref(false);
const detailLoading = ref(false);
const taskDetail = ref(null);
const selectedStepKey = ref("");
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
// 沟通记录里点开关联文档时用的抽屉状态
const documentsDrawerOpen = ref(false);
const documentsFocusId = ref(null);

// Whether the empty-list auto-redirect was already triggered, to avoid a loop.
const showDownloadModal = ref(false);
const firstLoad = ref(true);
const SETUP_GUIDE_SEEN = "pipeline_setup_guide_seen";

// Requirement detail sidebar state.
// 需求详情侧边栏状态
const currentWsId = ref("");
const currentWorkItemId = ref("");
const currentTaskId = ref("");
// Opening a work item from the task log lands on its Agent tab.
const currentDetailTab = ref("agent");
const pageRef = ref(null);
let detailRowScrollFrame;
let taskRequestId = 0;
let activationCount = 0;

// Conversations of the currently selected step, ordered by start time.
const stepConversations = computed(() => {
  if (!taskDetail.value || !selectedStepKey.value) return [];
  return (taskDetail.value.conversations || [])
    .filter((conversation) => conversation.step_key === selectedStepKey.value)
    .sort((a, b) => (a.started_at || 0) - (b.started_at || 0));
});

const columns = computed(() => [
  {
    title: $t("pipeline_task_title"),
    key: "title",
    dataIndex: "title",
    width: 350,
  },
  {
    title: $t("requirement_status"),
    key: "status",
    dataIndex: "status_name",
    width: 100,
  },
  {
    title: $t("pipeline_used_member"),
    key: "user_name",
    dataIndex: "user_name",
    width: 140,
    ellipsis: true,
  },
  {
    title: $t("pipeline_name"),
    key: "pipeline_name",
    dataIndex: "pipeline_name",
    width: 180,
    ellipsis: true,
  },
  {
    title: $t("pipeline_filter_started_at"),
    key: "started_at",
    dataIndex: "started_at",
    width: 160,
  },
  {
    title: $t("pipeline_table_conversation_rounds"),
    key: "conversation_rounds",
    dataIndex: "conversation_rounds",
    width: 130,
  },
  {
    title: $t("pipeline_total_tokens"),
    key: "total_tokens",
    dataIndex: "total_tokens",
    width: 110,
  },
  { title: $t("pipeline_action"), key: "actions", width: 80 },
]);

onMounted(() => {
  loadTasks();
});

onActivated(() => {
  if (activationCount++ > 0) loadTasks();
});

onBeforeUnmount(() => {
  if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
});

async function loadTasks() {
  if (!canViewPipeline.value) return;
  const requestId = ++taskRequestId;
  loading.value = true;
  try {
    const [startedAtStart, startedAtEnd] = filters.started_at_range || [];
    const res = await listPipelineTasks({
      keyword: filters.keyword,
      user_ids: filters.user_ids.length
        ? filters.user_ids.join(",")
        : undefined,
      start_at: startedAtStart
        ? startedAtStart.startOf("day").unix()
        : undefined,
      end_at: startedAtEnd
        ? startedAtEnd.add(1, "day").startOf("day").unix()
        : undefined,
      period: activePeriod.value,
      page: pagination.current,
      size: pagination.pageSize,
    });
    if (requestId !== taskRequestId) return;
    taskList.value = res.list || [];
    filterMembers.value = res.members || [];
    pagination.total = res.total || 0;
    if (res.stats) {
      stats.all = res.stats.all || 0;
      stats.today = res.stats.today || 0;
      stats.yesterday = res.stats.yesterday || 0;
      stats.week = res.stats.week || 0;
    }
    // First entry with an empty list: auto-redirect to the setup guide page (only once).
    if (firstLoad.value) {
      firstLoad.value = false;
      if (pagination.total === 0 && !localStorage.getItem(SETUP_GUIDE_SEEN)) {
        localStorage.setItem(SETUP_GUIDE_SEEN, "1");
        router.push("/pipelines/setup-guide");
      }
    }
  } catch (e) {
    // Errors are already handled by the interceptor.
  } finally {
    if (requestId === taskRequestId) loading.value = false;
  }
}

async function handleRequirementUpdated(change) {
  if (!change || change.action === "refresh") {
    await loadTasks();
    return;
  }
  taskList.value.forEach((task) => {
    if (
      String(task.work_item_id) !== String(change.id) ||
      String(task.workspace_id) !== String(change.workspaceId)
    ) {
      return;
    }
    task.status_name = change.item.status_name;
    task.status_color = change.item.status_color;
    task.is_done = change.item.is_done;
  });
}

function handlePeriodChange(period) {
  setRememberedTab(authStore.userInfo?.id, "pipeline", period);
  if (period !== "all") {
    filters.started_at_range = null;
  }
  pagination.current = 1;
  loadTasks();
}

function handleSearch() {
  pagination.current = 1;
  loadTasks();
}

function handleSearchInputChange(event) {
  if (!event.target.value) handleSearch();
}

function handleFilterChange() {
  pagination.current = 1;
  loadTasks();
}

function handleStartedAtChange(value) {
  if (value?.length === 2) {
    activePeriod.value = "all";
  }
  pagination.current = 1;
  loadTasks();
}

function clearFilters() {
  filters.keyword = "";
  filters.user_ids = [];
  filters.started_at_range = null;
  activePeriod.value = "all";
  pagination.current = 1;
  loadTasks();
}

function handleTableChange(pag) {
  pagination.current = pag.current;
  pagination.pageSize = pag.pageSize;
  loadTasks();
}

function openRequirementDetail(record, workspaceId) {
  if (record.work_item_id && record.workspace_id) {
    currentWsId.value = String(record.workspace_id);
    currentWorkItemId.value = String(record.work_item_id);
    currentTaskId.value = String(record.id);
    currentDetailTab.value = "agent";
  } else if (typeof record === "number" || typeof record === "string") {
    if (workspaceId) currentWsId.value = String(workspaceId);
    currentWorkItemId.value = String(record);
    const target = taskList.value.find(
      (item) =>
        String(item.work_item_id) === currentWorkItemId.value &&
        String(item.workspace_id) === currentWsId.value,
    );
    currentTaskId.value = target ? String(target.id) : "";
  }
}

function closeRequirementDetail() {
  currentWorkItemId.value = "";
  currentTaskId.value = "";
}

function pipelineTaskRowClassName(record) {
  return currentWorkItemId.value && String(record.id) === currentTaskId.value
    ? "detail-selected-row"
    : "";
}

function scrollSelectedDetailRowIntoView() {
  if (!currentTaskId.value) return;
  if (detailRowScrollFrame) cancelAnimationFrame(detailRowScrollFrame);
  nextTick(() => {
    detailRowScrollFrame = requestAnimationFrame(() => {
      detailRowScrollFrame = undefined;
      pageRef.value
        ?.querySelector(".pipeline-task-table .detail-selected-row")
        ?.scrollIntoView({ block: "nearest", inline: "nearest" });
    });
  });
}

watch([currentTaskId, taskList], scrollSelectedDetailRowIntoView, {
  flush: "post",
});

async function showTaskDetail(record) {
  detailVisible.value = true;
  detailLoading.value = true;
  taskDetail.value = null;
  selectedStepKey.value = "";
  try {
    taskDetail.value = await getPipelineTaskDetail(record.id);
    // Default to the current step; fall back to the first step.
    const steps = taskDetail.value?.steps || [];
    const initialKey =
      taskDetail.value?.current_step_key || steps[0]?.step_key || "";
    selectedStepKey.value = steps.some((step) => step.step_key === initialKey)
      ? initialKey
      : steps[0]?.step_key || "";
  } catch (e) {
    // Errors are already handled by the interceptor.
  } finally {
    detailLoading.value = false;
  }
}

// Clicking a step directly shows its conversation record on the right.
function selectStep(stepKey) {
  selectedStepKey.value = stepKey;
}

// 打开下载客户端弹窗（版本列表由 AgentDownloadModal 内部拉取）
function openDownload() {
  showDownloadModal.value = true;
}

function goSetupGuide() {
  router.push("/pipelines/setup-guide");
}

function execStatusColor(status) {
  const map = {
    idle: "default",
    running: "processing",
    success: "success",
    failed: "error",
    stopped: "warning",
  };
  return map[status] || "default";
}

function stepStatusColor(status) {
  const map = {
    idle: "gray",
    pending: "gray",
    running: "blue",
    success: "green",
    failed: "red",
    stopped: "orange",
  };
  return map[status] || "gray";
}

function stepName(stepKey) {
  return (
    taskDetail.value?.steps?.find((step) => step.step_key === stepKey)
      ?.step_name || stepKey
  );
}

function conversationKey(conversation, conversationIndex) {
  return [
    conversation.conversation_uuid || conversation.step_key,
    conversation.started_at,
    conversationIndex,
  ].join(":");
}

function sessionStatusColor(status) {
  const map = {
    created: "processing",
    running: "processing",
    waiting_input: "warning",
    stop_requested: "warning",
    success: "success",
    failed: "error",
    stopped: "warning",
    interrupted: "default",
  };
  return map[status] || "default";
}

function formatDateTime(ts) {
  if (!ts) return "-";
  const d = new Date(ts * 1000);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function formatTime(ts) {
  if (!ts) return "-";
  const date = parseTimestamp(ts);
  return date ? date.toLocaleString() : "-";
}

function formatTimeOnly(ts) {
  if (!ts) return "--:--:--";
  const date = parseTimestamp(ts);
  return date ? date.toLocaleTimeString() : "--:--:--";
}

function formatMessageTime(timestamp, fallback) {
  return formatTime(timestamp || fallback);
}

// 日志详情里的文档引用：点开在抽屉里读，不离开当前任务上下文。
function onMarkdownClick(event) {
  const documentId = documentIdFromEvent(event);
  if (!documentId) return;
  event.preventDefault();
  documentsFocusId.value = documentId;
  documentsDrawerOpen.value = true;
}

function parseTimestamp(timestamp) {
  const numeric =
    typeof timestamp === "number"
      ? timestamp
      : /^\d+$/.test(String(timestamp))
        ? Number(timestamp)
        : null;
  const date =
    numeric === null ? new Date(timestamp) : new Date(numeric * 1000);
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatDuration(duration) {
  if (!duration) return "";
  if (duration < 1000) return `${duration}ms`;
  if (duration < 60_000) return `${(duration / 1000).toFixed(1)}s`;
  return `${Math.floor(duration / 60_000)}m ${Math.round((duration % 60_000) / 1000)}s`;
}

function formatTokens(n) {
  if (!n) return "0";
  if (n > 1000) return (n / 1000).toFixed(1) + "K";
  return String(n);
}

function formatFileSize(bytes) {
  if (!bytes) return "-";
  const units = ["B", "KB", "MB", "GB"];
  let i = 0;
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024;
    i++;
  }
  return bytes.toFixed(1) + " " + units[i];
}
</script>

<style scoped>
.pipeline-page {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  color: var(--app-text);
}

.pipeline-header {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 24px;
  min-height: 64px;
  padding: 0 24px;
  border-bottom: 1px solid #f0f0f0;
  background: #fff;
}

.page-identity {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 10px;
}

.pipeline-title {
  margin: 0;
  color: #111827;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
  white-space: nowrap;
}

.page-description {
  color: #98a2b3;
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
}

.page-icon {
  display: inline-flex;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  align-self: center;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: #fff;
  background: #3157e2;
  font-size: 14px;
}

.pipeline-main-tabs {
  flex: 0 0 auto;
  margin-left: 8px;
}

.pipeline-main-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 0;
}

.pipeline-main-tabs :deep(.ant-tabs-nav-operations) {
  display: none;
}

.pipeline-main-content {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  overflow: hidden;
}

.pipeline-no-permission {
  align-items: center;
  justify-content: center;
}

.pipeline-no-permission :deep(.ant-empty-image) {
  height: 80px;
  color: #91b4ff;
  font-size: 72px;
  line-height: 80px;
}

.pipeline-no-permission :deep(.ant-empty-description) {
  color: #8c8c8c;
}

.pipeline-logs {
  gap: 12px;
  padding: 16px 24px 24px;
  overflow-y: auto;
}

.logs-heading {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.logs-title {
  margin: 0;
  color: #000;
  font-size: 16px;
  font-weight: 600;
}

.logs-subtitle {
  color: #8c8c8c;
  font-size: 14px;
}

.table-actions {
  display: inline-flex;
  gap: 4px;
}

.pipeline-period-tabs {
  flex: 0 0 auto;
  margin-bottom: 0;
}

.pipeline-main-tabs {
  margin-left: 12px;
}

.pipeline-period-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 0;
}

.pipeline-period-tabs :deep(.ant-tabs-nav-operations) {
  display: none;
}

.period-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.period-tab-label em {
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

.pipeline-period-tabs :deep(.ant-tabs-tab-active) .period-tab-label em {
  color: #fff;
  background: #3157e2;
}

.header-spacer {
  flex: 1;
}

.pipeline-header-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
}

.setup-guide-button {
  padding-right: 4px;
  padding-left: 4px;
}

.pipeline-header :deep(.ant-btn-primary) {
  border-radius: 6px;
}

.pipeline-filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 16px;
  padding-bottom: 16px;
}

.pipeline-filters :deep(.ant-btn) {
  height: 30px;
  padding: 0 10px;
  border-radius: 4px;
  font-size: 12px;
}

.pipeline-filter-search {
  width: 180px;
}

.pipeline-filter-search :deep(.ant-input-affix-wrapper) {
  height: 30px;
  border-radius: 4px;
  font-size: 12px;
}

.pipeline-filter-search :deep(.ant-input-affix-wrapper:hover) {
  border-color: #d0d5dd;
  background: #f0f1f3;
}

.pipeline-filter-search :deep(.ant-input) {
  background: transparent;
  font-size: 12px;
}

.pipeline-toolbar-filter {
  position: relative;
  display: inline-flex;
  height: 30px;
  flex-shrink: 0;
  align-items: center;
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 4px;
  background: var(--app-surface);
  transition:
    border-color 0.15s,
    background 0.15s;
}

.pipeline-toolbar-filter:hover {
  border-color: #d0d5dd;
  background: #f0f1f3;
}

.pipeline-toolbar-filter > span {
  position: absolute;
  left: 8px;
  z-index: 1;
  overflow: hidden;
  color: var(--app-text);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
  pointer-events: none;
}

.pipeline-member-filter {
  width: 180px;
}

.pipeline-member-filter > span {
  max-width: 58px;
}

.pipeline-member-filter :deep(.ant-select) {
  position: absolute;
  inset: 0;
  width: 100%;
  font-size: 12px;
}

.pipeline-member-filter :deep(.ant-select-selector) {
  height: 28px !important;
  align-items: center;
  padding: 0 22px 0 72px !important;
  background: transparent !important;
}

.pipeline-member-filter :deep(.ant-select-selection-item),
.pipeline-member-filter :deep(.ant-select-selection-placeholder) {
  color: var(--app-text-secondary);
  font-size: 12px;
}

.pipeline-date-filter {
  width: 310px;
}

.pipeline-date-filter > span {
  max-width: 82px;
}

.pipeline-date-filter :deep(.ant-picker) {
  width: 100%;
  height: 28px;
  padding: 0 8px 0 96px;
  background: transparent;
  font-size: 12px;
}

.pipeline-date-filter :deep(.ant-picker-input > input) {
  font-size: 12px;
}

.clear-filter-button {
  flex-shrink: 0;
  color: var(--app-text-secondary);
}

:global(.pipeline-filter-dropdown) {
  width: 180px !important;
  min-width: 180px !important;
}

.pipeline-task-table {
  /* 表格高度由内容撑开，滚动交给 .pipeline-logs；若允许被压缩会裁掉分页 */
  flex: 0 0 auto;
}

.pipeline-task-table :deep(.ant-table) {
  font-size: 14px;
}

.pipeline-task-table :deep(.ant-table-thead > tr > th),
.pipeline-task-table :deep(.ant-table-tbody > tr > td) {
  padding: 10px 8px;
  text-align: left;
  vertical-align: middle;
  white-space: nowrap;
}

.pipeline-task-table :deep(.ant-table-thead > tr > th) {
  color: #7c8597;
  background: var(--app-surface-muted);
  font-size: 12px;
  font-weight: 400;
}

.pipeline-task-table :deep(.ant-table-tbody > tr.detail-selected-row > td),
.pipeline-task-table
  :deep(.ant-table-tbody > tr.detail-selected-row:hover > td) {
  background: #eaf3ff !important;
}

.pipeline-task-table
  :deep(.ant-table-tbody > tr.detail-selected-row .task-title-link) {
  color: #1677ff !important;
}

.task-title-cell {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 6px;
  padding: 0;
  overflow: hidden;
  border: 0;
  color: var(--app-text);
  background: transparent;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.task-title-link {
  min-width: 0;
  overflow: hidden;
  color: #30405f;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-title-cell:hover .task-title-link {
  color: #1677ff;
}

.pipeline-cell {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipeline-color-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  flex: 0 0 10px;
  border-radius: 50%;
}

.token-cell {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #30405f;
}

.token-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 14px;
  background: #f5a300;
  -webkit-mask: url("@/image/agent/lightning.svg") center / contain no-repeat;
  mask: url("@/image/agent/lightning.svg") center / contain no-repeat;
}

.token-cell.done .token-icon {
  background: #8c95a8;
}

.summary-text {
  color: #666;
  white-space: pre-wrap;
}

.detail-loading {
  display: flex;
  justify-content: center;
  padding-top: 120px;
}

.detail-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  font-weight: 600;
}

.detail-meta {
  display: flex;
  align-items: center;
  gap: 28px;
  min-height: 42px;
  padding: 0 16px;
  color: #303133;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  font-size: 13px;
  flex-wrap: wrap;
}

.detail-meta span {
  display: inline-flex;
  gap: 6px;
}

.detail-meta em {
  color: #8c8c8c;
  font-style: normal;
}

.detail-content {
  display: grid;
  flex: 1 1 auto;
  grid-template-columns: minmax(360px, 42%) minmax(460px, 58%);
  min-height: 0;
  margin-top: 12px;
  border-top: 1px solid #f0f0f0;
}

.detail-steps,
.detail-conversation {
  min-width: 0;
  padding: 16px;
  overflow-y: auto;
}

.detail-steps {
  border-right: 1px solid #f0f0f0;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-bottom: 18px;
  color: #262626;
  font-weight: 600;
}

.section-title small {
  margin-left: auto;
  color: #b0b0b0;
  font-size: 12px;
  font-weight: 400;
}

/* 沟通记录标题行：标题与产出文档入口同行 */
.detail-conversation .section-title {
  gap: 12px;
  min-height: 22px;
  font-size: 14px;
  color: #1d1d1f;
}

.section-title-divider {
  width: 1px;
  height: 12px;
  border-radius: 1px;
  background: #f0f0f0;
}

/* 没有产出文档时入口不渲染，紧邻的分隔线也一并隐藏 */
.section-title-divider:has(+ small) {
  display: none;
}

.section-documents-link {
  flex: 0 0 auto;
}

.section-icon {
  font-size: 16px;
  font-weight: 700;
}

.section-icon-blue {
  color: #1677ff;
}

.section-icon-green {
  color: #1d1d1f;
}

.steps-timeline {
  padding: 4px 8px;
}

.timeline-step-item {
  cursor: pointer;
}

.timeline-step-item :deep(.ant-timeline-item-content) {
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  transition: all 0.18s ease;
}

.timeline-step-item:hover :deep(.ant-timeline-item-content) {
  background: #f5f7fa;
}

.timeline-step-item.active :deep(.ant-timeline-item-content) {
  background: #eef5ff;
  border-color: #91caff;
}

.step-item {
  padding: 0 0 4px 4px;
}

.step-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.step-name {
  color: #262626;
  font-weight: 600;
}

.step-error {
  color: #ff4d4f;
  font-size: 13px;
  margin: 4px 0;
}

.step-meta {
  display: flex;
  gap: 16px;
  color: #999;
  font-size: 12px;
  flex-wrap: wrap;
}

.conversation-list {
  padding: 0 8px 24px;
}

.conversation-session-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 22px;
  padding: 12px 14px;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
}

.conversation-session-header > div {
  display: grid;
  gap: 4px;
}

.conversation-session-header span {
  color: #8c8c8c;
  font-size: 12px;
}

.conversation-round + .conversation-round {
  margin-top: 22px;
}

.round-label {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px;
  color: #8c8c8c;
  font-size: 12px;
}

.round-label::before,
.round-label::after {
  height: 1px;
  background: #f0f0f0;
  content: "";
}

.round-label::before {
  width: 18px;
}

.round-label::after {
  flex: 1;
}

.round-error {
  margin: 8px 0 12px 40px;
}

.round-meta {
  margin: -4px 4px 0;
  color: #b0b0b0;
  font-size: 11px;
  text-align: right;
}

.message-row {
  display: flex;
  width: 100%;
  margin-bottom: 14px;
}

.message-row-user {
  justify-content: flex-end;
}

.message-row-ai {
  align-items: flex-start;
  gap: 10px;
}

.message-bubble {
  max-width: min(82%, 720px);
  padding: 11px 14px;
  border-radius: 10px;
  line-height: 1.65;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.message-markdown {
  white-space: normal;
}

.message-markdown :deep(p),
.message-markdown :deep(ul),
.message-markdown :deep(ol),
.message-markdown :deep(pre) {
  margin: 0.6em 0;
}

.message-markdown :deep(> :first-child) {
  margin-top: 0;
}

.message-markdown :deep(> :last-child) {
  margin-bottom: 0;
}

.message-markdown :deep(ul) {
  padding-left: 1.3em;
  list-style: disc;
}

.message-markdown :deep(ol) {
  padding-left: 1.3em;
  list-style: decimal;
}

.message-markdown :deep(pre) {
  padding: 10px 12px;
  overflow: auto;
  border-radius: 7px;
  background: #1f2430;
  color: #e8eaf0;
  white-space: pre;
}

.message-markdown :deep(code) {
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 0.9em;
}

.message-user {
  color: #1677ff;
  background: #e8f3ff;
  border-top-right-radius: 3px;
}

.message-ai {
  color: #303133;
  background: #f5f5f5;
  border-top-left-radius: 3px;
}

.ai-avatar {
  display: flex;
  flex: 0 0 30px;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  color: #fff;
  background: #1677ff;
  border-radius: 50%;
  font-size: 13px;
  font-weight: 600;
}

.message-time {
  margin: 5px 2px 0;
  color: #b0b0b0;
  font-size: 11px;
}

:global(.pipeline-task-detail-drawer .ant-drawer-body) {
  display: flex;
  flex-direction: column;
  padding: 12px 16px 0;
  overflow: hidden;
}

@media (max-width: 1000px) {
  :global(.pipeline-task-detail-drawer .ant-drawer-content-wrapper) {
    width: 100vw !important;
  }

  .detail-content {
    display: block;
    flex: 0 0 auto;
    height: auto;
  }

  .detail-steps,
  .detail-conversation {
    overflow: visible;
  }

  .detail-steps {
    border-right: 0;
    border-bottom: 1px solid #f0f0f0;
  }

  :global(.pipeline-task-detail-drawer .ant-drawer-body) {
    overflow-y: auto;
  }
}
.download-tip {
  margin-bottom: 16px;
  color: rgba(0, 0, 0, 0.65);
}
.release-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.release-card {
  position: relative;
  flex: 1 1 200px;
  min-width: 200px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: #fff;
}
.release-card.release-recommended {
  border-color: #52c41a;
  box-shadow: 0 0 0 2px rgba(82, 196, 26, 0.15);
}
.release-badge {
  position: absolute;
  top: 8px;
  right: 8px;
}
.release-os {
  font-size: 16px;
  font-weight: 600;
  color: rgba(0, 0, 0, 0.85);
}
.release-arch {
  color: rgba(0, 0, 0, 0.65);
  font-size: 13px;
}
.release-desc {
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
  min-height: 32px;
}
.release-card .ant-btn {
  margin-top: auto;
}

@media (max-width: 1200px) {
  .pipeline-main-tabs {
    margin-left: 12px;
  }
}

@media (max-width: 960px) {
  .pipeline-header {
    flex-wrap: wrap;
    padding-bottom: 8px;
  }

  .page-description {
    display: none;
  }

  .pipeline-main-tabs {
    width: 100%;
    order: 3;
    margin-left: 0;
    overflow-x: auto;
  }

  .pipeline-logs {
    padding: 12px 16px 16px;
  }

  .header-spacer {
    min-width: 0;
  }
}
</style>
