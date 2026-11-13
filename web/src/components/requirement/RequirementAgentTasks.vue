<template>
  <div class="agent-tasks">
    <a-spin v-if="loading" class="agent-tasks-loading" />
    <a-empty
      v-else-if="!executors.length"
      :image="simpleImage"
      :description="$t('agent_tab_empty')"
      class="agent-tasks-empty"
    />
    <template v-else>
      <!-- 执行人/每次执行切换（同人多次执行以 #n 区分） -->
      <div
        class="executor-pills"
        role="tablist"
        :aria-label="$t('agent_tab_executor')"
      >
        <button
          v-for="executor in executors"
          :key="executor.key"
          type="button"
          role="tab"
          :aria-selected="activeExecutorKey === executor.key"
          :aria-label="executor.label"
          class="executor-pill"
          :class="{ 'executor-active': activeExecutorKey === executor.key }"
          @click="switchExecutor(executor)"
          @keydown.enter.prevent="switchExecutor(executor)"
        >
          <a-avatar
            v-if="executor.avatar"
            :src="executor.avatar"
            :size="20"
            class="executor-avatar"
          />
          <a-avatar
            v-else
            :size="20"
            class="executor-avatar executor-avatar-char"
          >
            {{ (executor.user_name || "?").trim().charAt(0) }}
          </a-avatar>
          <span class="executor-name">{{ executor.label }}</span>
        </button>
      </div>

      <!-- 选中的执行记录 -->
      <div v-if="activeTask" class="agent-task-detail">
        <div class="task-header">
          <span
            class="task-status-badge"
            :class="'status-' + activeTask.execution_status"
          >
            {{ $t("pipeline_exec_" + activeTask.execution_status) }}
          </span>
          <span class="task-time">{{
            formatDateTime(activeTask.started_at * 1000)
          }}</span>
          <span v-if="activeTask.conversation_rounds" class="task-rounds">
            {{
              $t("pipeline_rounds_count", {
                count: activeTask.conversation_rounds,
              })
            }}
          </span>
          <span v-if="activeTask.pipeline_name" class="task-pipeline">
            {{ $t("agent_tab_pipeline_label") }}：{{ activeTask.pipeline_name }}
          </span>
        </div>

        <!-- 执行流水线进度（对齐设计稿的视觉流水线条） -->
        <div v-if="detailSteps.length" class="step-strip">
          <div class="step-strip-title">
            <ApartmentOutlined class="step-strip-icon" />
            {{ $t("agent_tab_pipeline_label") }}：{{
              activeTask.pipeline_name || "-"
            }}
          </div>
          <div class="step-strip-nodes">
            <div
              v-for="(step, index) in detailSteps"
              :key="step.step_key"
              class="step-node"
              :class="stepNodeClass(step)"
            >
              <span class="step-node-icon">
                <span
                  v-if="step.status === 'success'"
                  class="step-state-icon step-done"
                  >✓</span
                >
                <span
                  v-else-if="step.status === 'failed'"
                  class="step-state-icon step-fail"
                  >✕</span
                >
                <span v-else class="step-state-icon" />
              </span>
              <span class="step-node-name">{{ step.step_name }}</span>
              <span
                v-if="index < detailSteps.length - 1"
                class="step-node-arrow"
                >→</span
              >
            </div>
          </div>
        </div>

        <!-- 沟通记录：标题与内容拆分，产出文档入口并入标题行 -->
        <div class="conversation-section">
          <div class="conversation-head">
            <div class="conversation-title">
              <MessageOutlined class="conversation-title-icon" />
              {{ $t("agent_tab_conversations") }}
              <span v-if="conversationMessageCount" class="conversation-count">
                ·
                {{
                  $t("agent_tab_messages", { count: conversationMessageCount })
                }}
              </span>
            </div>
            <span class="conversation-head-divider" />
            <TaskDocumentsEntry
              :task-id="activeTask.task_id"
              variant="link"
              class="documents-link"
            />
          </div>
          <a-empty
            v-if="!conversations.length"
            :image="simpleImage"
            :description="$t('agent_tab_no_conversations')"
          />
          <div v-else class="message-list-card">
            <template
              v-for="conversation in conversations"
              :key="conversation.step_key || conversation.conversation_uuid"
            >
              <template
                v-for="round in conversation.conversation?.rounds || []"
                :key="`${conversation.step_key}:${round.session_uuid || round.round_no}`"
              >
                <article
                  v-if="round.user_message"
                  class="message-card user-message"
                  :class="{
                    collapsed: isCollapsed(
                      messageKey(conversation, round, 'user'),
                    ),
                  }"
                >
                  <div class="message-card-head">
                    <div class="message-avatar user-avatar">
                      {{ $t("agent_tab_me").slice(0, 1) }}
                    </div>
                    <div class="message-card-meta">
                      <p class="message-card-name">{{ $t("agent_tab_me") }}</p>
                      <p class="message-card-time">
                        {{ formatMessageTime(round.started_at) }}
                      </p>
                    </div>
                    <button
                      type="button"
                      class="message-card-icon"
                      :aria-label="$t('common_copy')"
                      @click="copyText(round.user_message)"
                    >
                      <CopyOutlined />
                    </button>
                    <button
                      type="button"
                      class="message-card-icon"
                      :aria-label="
                        isCollapsed(messageKey(conversation, round, 'user'))
                          ? $t('common_expand')
                          : $t('common_collapse')
                      "
                      :aria-expanded="
                        !isCollapsed(messageKey(conversation, round, 'user'))
                      "
                      @click="
                        toggleCollapsed(messageKey(conversation, round, 'user'))
                      "
                    >
                      <RightOutlined
                        v-if="
                          isCollapsed(messageKey(conversation, round, 'user'))
                        "
                      />
                      <DownOutlined v-else />
                    </button>
                  </div>
                  <div
                    v-show="
                      !isCollapsed(messageKey(conversation, round, 'user'))
                    "
                    class="message-card-body message-markdown"
                    @click="onMarkdownClick"
                    v-html="
                      renderConversationMarkdown(round.user_message, documents)
                    "
                  ></div>
                </article>

                <article
                  v-if="hasAgentRound(round)"
                  class="message-card"
                  :class="{
                    collapsed: isCollapsed(
                      messageKey(conversation, round, 'agent'),
                    ),
                  }"
                >
                  <div class="message-card-head">
                    <div class="message-avatar">
                      {{ (conversation.step_name || "A").trim().charAt(0) }}
                    </div>
                    <div class="message-card-meta">
                      <p class="message-card-name">
                        {{
                          conversation.step_name ||
                          $t("pipeline_agent_orchestration")
                        }}
                      </p>
                      <p class="message-card-time">
                        {{
                          formatMessageTime(
                            round.finished_at ||
                              round.started_at ||
                              round.assistant_messages?.[0]?.timestamp,
                          )
                        }}
                      </p>
                    </div>
                    <button
                      type="button"
                      class="message-card-icon"
                      :aria-label="$t('common_copy')"
                      @click="copyText(agentRoundText(round, conversation))"
                    >
                      <CopyOutlined />
                    </button>
                    <button
                      type="button"
                      class="message-card-icon"
                      :aria-label="
                        isCollapsed(messageKey(conversation, round, 'agent'))
                          ? $t('common_expand')
                          : $t('common_collapse')
                      "
                      :aria-expanded="
                        !isCollapsed(messageKey(conversation, round, 'agent'))
                      "
                      @click="
                        toggleCollapsed(
                          messageKey(conversation, round, 'agent'),
                        )
                      "
                    >
                      <RightOutlined
                        v-if="
                          isCollapsed(messageKey(conversation, round, 'agent'))
                        "
                      />
                      <DownOutlined v-else />
                    </button>
                  </div>
                  <div
                    v-show="
                      !isCollapsed(messageKey(conversation, round, 'agent'))
                    "
                    class="message-card-detail"
                  >
                    <ExecutionProcessTimeline
                      :events="round.events"
                      :status="round.status || conversation.status"
                      :cli-type="conversation.cli_type"
                      :model-name="conversation.model_name"
                      :input-tokens="Number(round.usage?.input_tokens || 0)"
                      :output-tokens="Number(round.usage?.output_tokens || 0)"
                    />
                    <div
                      v-for="aiMessage in round.assistant_messages || []"
                      :key="`${round.session_uuid}:${aiMessage.sequence}`"
                      class="message-card-body message-markdown"
                      @click="onMarkdownClick"
                      v-html="
                        renderConversationMarkdown(aiMessage.content, documents)
                      "
                    ></div>
                    <p v-if="round.error_summary" class="conversation-error">
                      {{ round.error_summary }}
                    </p>
                  </div>
                </article>
              </template>
              <p
                v-if="showConversationError(conversation)"
                class="conversation-error"
              >
                {{ conversation.error_summary }}
              </p>
            </template>
          </div>
        </div>
      </div>

      <a-empty
        v-else
        :image="simpleImage"
        :description="$t('agent_tab_select_executor')"
        class="agent-tasks-empty"
      />
    </template>

    <!-- 沟通记录里的关联文档：点开在右侧抽屉里读，不打断当前对话 -->
    <TaskDocumentsDrawer
      v-model:open="documentsDrawerOpen"
      v-model:document-id="documentsFocusId"
      :task-id="activeTask ? activeTask.task_id : null"
    />
  </div>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { Empty } from "ant-design-vue";
import {
  ApartmentOutlined,
  CopyOutlined,
  DownOutlined,
  MessageOutlined,
  RightOutlined,
} from "@ant-design/icons-vue";
import dayjs from "dayjs";
import {
  getRequirementAgentTaskDetail,
  listRequirementAgentTasks,
} from "@/api/pipeline.js";
import {
  documentIdFromEvent,
  renderConversationMarkdown,
} from "./conversationMarkdown.js";
import ExecutionProcessTimeline from "./ExecutionProcessTimeline.vue";
import TaskDocumentsDrawer from "@/components/pipeline/TaskDocumentsDrawer.vue";
import TaskDocumentsEntry from "@/components/pipeline/TaskDocumentsEntry.vue";

const props = defineProps({
  wsId: { type: [Number, String], default: null },
  requirementId: { type: [Number, String], default: null },
  workItemType: { type: String, default: "requirement" },
});

const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;

// ── executor 列表（同人多次执行以 #n 区分） ──────────────
const loading = ref(false);
const executors = ref([]);
const activeExecutorKey = ref(null);
const activeTask = ref(null); // AgentTaskSummary, includes started_at etc.
const detailSteps = ref([]);
const conversations = ref([]);
// 该任务的产出文档索引，用于把沟通记录里的本地路径渲染成可点链接
const documents = ref([]);
const documentsDrawerOpen = ref(false);
const documentsFocusId = ref(null);

// 同一个人的第一次执行显示名字，之后依次是 名字#1、名字#2。
// 序号按该用户自己的执行先后计算，不按会话里的聊天轮次拆开。
function executionLabel(userName, index) {
  const name = userName || "?";
  return index > 0 ? `${name}#${index}` : name;
}

function executionSortKey(executor) {
  return (
    Number(executor.task?.started_at) ||
    Number(executor.task?.last_uploaded_at) ||
    0
  );
}

function buildExecutorList(executorGroups) {
  const list = [];
  executorGroups.forEach((group) => {
    const userName = group.user_name || "";
    const tasks = group.tasks || [];
    tasks.forEach((task, index) => {
      list.push({
        key: `${group.user_id}-${task.task_id}`,
        user_id: group.user_id,
        user_name: userName,
        avatar: group.avatar,
        label: executionLabel(userName, index),
        task,
      });
    });
  });
  list.sort(
    (left, right) =>
      executionSortKey(left) - executionSortKey(right) ||
      Number(left.task?.task_id) - Number(right.task?.task_id),
  );
  return list;
}

async function loadTaskDetail(executor) {
  if (!executor?.task || !props.wsId || !props.requirementId) {
    return { steps: [], conversations: [] };
  }
  return getRequirementAgentTaskDetail(
    props.wsId,
    props.requirementId,
    executor.task.task_id,
    props.workItemType,
  );
}

async function loadExecutors() {
  if (!props.wsId || !props.requirementId) return;
  loading.value = true;
  try {
    const result = await listRequirementAgentTasks(
      props.wsId,
      props.requirementId,
      props.workItemType,
    );
    const groups = Array.isArray(result) ? result : result?.items || [];
    executors.value = buildExecutorList(groups);
    if (executors.value.length) {
      await switchExecutor(executors.value[executors.value.length - 1]);
    }
  } catch {
    executors.value = [];
  } finally {
    loading.value = false;
  }
}

async function switchExecutor(executor) {
  activeExecutorKey.value = executor.key;
  activeTask.value = executor.task;
  detailSteps.value = [];
  conversations.value = [];
  documents.value = [];
  // 加载该任务的完整 step/conversation 明细
  if (!executor?.task || !props.wsId || !props.requirementId) return;
  try {
    const detail = await loadTaskDetail(executor);
    detailSteps.value = detail?.steps || [];
    conversations.value = detail?.conversations || [];
    documents.value = detail?.documents || [];
  } catch {
    detailSteps.value = [];
    conversations.value = [];
    documents.value = [];
  }
}

watch(
  () => [props.wsId, props.requirementId, props.workItemType],
  loadExecutors,
  { immediate: true },
);

const conversationMessageCount = computed(() => {
  let count = 0;
  conversations.value.forEach((conversation) => {
    (conversation.conversation?.rounds || []).forEach((round) => {
      if (round.user_message) count += 1;
      count += (round.assistant_messages || []).length;
    });
  });
  return count;
});

// ── 工具函数 ─────────────────────────────────────────────
const collapsedKeys = ref(new Set());

function messageKey(conversation, round, role) {
  return `${conversation.step_key || conversation.conversation_uuid}:${round.session_uuid || round.round_no}:${role}`;
}

function isCollapsed(key) {
  return collapsedKeys.value.has(key);
}

function toggleCollapsed(key) {
  const next = new Set(collapsedKeys.value);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  collapsedKeys.value = next;
}

function showConversationError(conversation) {
  if (!conversation.error_summary) return false;
  return !(conversation.conversation?.rounds || []).some(
    (round) => round.error_summary,
  );
}

function hasAgentRound(round) {
  return Boolean(
    (round.assistant_messages || []).length ||
    (round.events || []).length ||
    round.error_summary,
  );
}

function agentRoundText(round) {
  const parts = (round.assistant_messages || [])
    .map((item) => item.content)
    .filter(Boolean);
  if (round.error_summary) parts.push(round.error_summary);
  return parts.join("\n\n");
}

async function copyText(text) {
  const value = String(text || "").trim();
  if (!value || !navigator.clipboard) return;
  try {
    await navigator.clipboard.writeText(value);
  } catch {
    // 浏览器拒绝剪贴板时不打断阅读。
  }
}

function formatDateTime(ts) {
  if (!ts) return "-";
  return dayjs(ts).format("YY-MM-DD HH:mm");
}

function formatMessageTime(value) {
  const parsed = toDayjs(value);
  if (!parsed) return "";
  return parsed.isSame(dayjs(), "day")
    ? parsed.format("HH:mm")
    : parsed.format("MM-DD HH:mm");
}

function toDayjs(value) {
  if (value == null || value === "") return null;
  if (typeof value === "number" || /^\d+$/.test(String(value))) {
    const stamp = Number(value);
    if (!stamp) return null;
    return dayjs(stamp < 1e12 ? stamp * 1000 : stamp);
  }
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed : null;
}

function stepNodeClass(step) {
  return {
    "step-success": step.status === "success",
    "step-running": step.status === "running",
    "step-failed": step.status === "failed",
    "step-stopped": ["stopped", "interrupted"].includes(step.status),
  };
}

// 沟通记录里的文档引用只换链接，不换位置：点开在抽屉里读同一任务的文档。
function onMarkdownClick(event) {
  const documentId = documentIdFromEvent(event);
  if (!documentId) return;
  event.preventDefault();
  documentsFocusId.value = documentId;
  documentsDrawerOpen.value = true;
}
</script>

<style scoped>
.agent-tasks {
  padding: 4px 20px 32px;
  color: #1f2937;
}

.agent-tasks-loading {
  display: flex;
  width: 100%;
  padding: 48px 0;
  justify-content: center;
}

.agent-tasks-empty {
  padding: 64px 0;
}

/* ── 执行人 pill ─────────────────────────────────────── */
.executor-pills {
  display: flex;
  gap: 8px;
  margin: 0 -20px 18px;
  padding: 12px 20px;
  overflow-x: auto;
  border-bottom: 1px solid #edf0f5;
  background: #fff;
  scrollbar-width: thin;
}

.executor-pill {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 5px 13px 5px 7px;
  border: 1px solid #dfe4ee;
  border-radius: 18px;
  background: #fff;
  cursor: pointer;
  transition:
    border-color 0.2s,
    box-shadow 0.2s,
    background 0.2s;
  user-select: none;
  font: inherit;
  text-align: left;
  appearance: none;
}

.executor-pill:hover {
  border-color: #c8d0ff;
}

.executor-pill.executor-active {
  border-color: #4361ee;
  background: #f1f4ff;
  box-shadow: 0 2px 6px rgba(67, 97, 238, 0.16);
}

.executor-pill:focus-visible {
  outline: 2px solid #91a7ff;
  outline-offset: 2px;
}

.executor-avatar {
  flex-shrink: 0;
}

.executor-avatar-char {
  background: #4361ee;
  color: #fff;
  font-size: 12px;
}

.executor-name {
  font-size: 13px;
  font-weight: 500;
  color: #26334d;
}

/* ── 任务头 ─────────────────────────────────────────── */
.task-header {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 38px;
  margin-bottom: 14px;
  padding: 0 2px;
  font-size: 13px;
  color: #68738a;
  flex-wrap: wrap;
}

.task-status-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 600;
}

.task-status-badge.status-success {
  background: #f0fff7;
  color: #21a665;
}
.task-status-badge.status-running {
  background: #e8effe;
  color: #3157e2;
}
.task-status-badge.status-failed {
  background: #fff1f0;
  color: #cf1322;
}
.task-status-badge.status-idle,
.task-status-badge.status-stopped {
  background: #f5f5f5;
  color: #8c8c8c;
}

.task-time,
.task-rounds,
.task-pipeline {
  font-variant-numeric: tabular-nums;
}

/* ── 流水线进度条 ────────────────────────────────────── */
.step-strip {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid #e8ecf3;
  border-radius: 12px;
  background: #fbfcfe;
  box-shadow: 0 1px 2px rgba(30, 48, 80, 0.04);
  margin-bottom: 22px;
}

.step-strip-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  color: #69758c;
}

.step-strip-icon {
  font-size: 13px;
}

.step-strip-nodes {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.step-node {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.step-node-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #fff;
  border: 1px solid #d9d9d9;
  flex-shrink: 0;
  font-size: 12px;
  color: #21a665;
}

.step-node.step-success .step-node-icon {
  border-color: #b7eb8f;
  background: #f6ffed;
}
.step-node.step-running .step-node-icon {
  border-color: #91caff;
  background: #e6f4ff;
}
.step-node.step-failed .step-node-icon {
  border-color: #ffa39e;
  background: #fff2f0;
  color: #cf1322;
}

.step-state-icon {
  font-weight: 600;
}
.step-done {
  color: #21a665;
}
.step-fail {
  color: #cf1322;
}

.step-node-name {
  padding: 4px 10px;
  border-radius: 6px;
  background: #f3f5f9;
  font-size: 12px;
  color: #595959;
}

.step-node.step-success .step-node-name {
  color: #21a665;
  background: #f0fff7;
}
.step-node.step-running .step-node-name {
  color: #3157e2;
  background: #e8effe;
}
.step-node.step-failed .step-node-name {
  color: #cf1322;
  background: #fff1f0;
}
.step-node.step-stopped .step-node-name {
  color: #8c8c8c;
  background: #f5f5f5;
}

.step-node-arrow {
  color: #c8c8c8;
  font-size: 12px;
  margin: 0 2px;
}

/* ── 沟通记录 ────────────────────────────────────────── */
.conversation-head {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 22px;
  margin: 0 0 12px;
}

.conversation-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: #1d1d1f;
}

.conversation-title-icon {
  color: #1d1d1f;
  font-size: 16px;
}

.conversation-head-divider {
  width: 1px;
  height: 12px;
  border-radius: 1px;
  background: #f0f0f0;
}

/* 没有产出文档时入口不渲染，只在入口前面出现的分隔线也不该留着 */
.conversation-head-divider:last-child {
  display: none;
}

.conversation-count {
  color: #8c8c8c;
  font-size: 12px;
  font-weight: 400;
}

.message-list-card {
  overflow: hidden;
  border: 1px solid #d9d9d9;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 2px 24px rgba(0, 0, 0, 0.08);
}

.message-card {
  padding: 16px;
}

.message-card + .message-card {
  border-top: 1px solid #f0f0f0;
}

.message-card.user-message {
  background: #f5f9ff;
}

.message-card-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.message-avatar {
  display: flex;
  width: 20px;
  height: 20px;
  flex: 0 0 20px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  background: #9ca3af;
  font-size: 9px;
  font-weight: 600;
}

.message-avatar.user-avatar {
  color: #64748b;
  background: #e2e8f0;
}

.message-card-meta {
  display: flex;
  min-width: 0;
  flex: 1;
  align-items: baseline;
  gap: 9px;
}

.message-card-name {
  margin: 0;
  color: #262626;
  font-size: 14px;
  font-weight: 600;
  line-height: 22px;
}

.message-card-time {
  margin: 0;
  color: #8c8c8c;
  font-size: 12px;
  line-height: 16px;
}

.message-card-icon {
  display: inline-flex;
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 6px;
  color: #c4c9d1;
  background: transparent;
  cursor: pointer;
}

.message-card-icon:hover {
  color: #6b7280;
  background: #f3f4f6;
}

.message-card-body {
  margin-top: 16px;
  padding-left: 32px;
  color: #262626;
  font-size: 14px;
  line-height: 32px;
  overflow-wrap: anywhere;
}

.message-markdown {
  white-space: normal;
  overflow-wrap: anywhere;
  word-break: normal;
  color: #262626;
  font-size: 14px;
  line-height: 32px;
}

.message-markdown :deep(h1),
.message-markdown :deep(h2),
.message-markdown :deep(h3),
.message-markdown :deep(h4),
.message-markdown :deep(h5),
.message-markdown :deep(h6) {
  color: #1f2430;
  font-weight: 650;
  line-height: 1.35;
  margin: 0.9em 0 0.4em;
}

.message-markdown :deep(h1) {
  font-size: 1.35em;
}

.message-markdown :deep(h2) {
  font-size: 1.2em;
}

.message-markdown :deep(h3) {
  font-size: 1.08em;
}

.message-markdown :deep(h4),
.message-markdown :deep(h5),
.message-markdown :deep(h6) {
  font-size: 1em;
}

.message-markdown :deep(p),
.message-markdown :deep(ul),
.message-markdown :deep(ol),
.message-markdown :deep(blockquote),
.message-markdown :deep(pre),
.message-markdown :deep(table) {
  margin: 0.7em 0;
}

.message-markdown :deep(> :first-child) {
  margin-top: 0;
}

.message-markdown :deep(> :last-child) {
  margin-bottom: 0;
}

.message-markdown :deep(ul),
.message-markdown :deep(ol) {
  padding-left: 1.4em;
}

.message-markdown :deep(ul) {
  list-style: disc;
}

.message-markdown :deep(ol) {
  list-style: decimal;
}

.message-markdown :deep(li > p) {
  margin: 0.15em 0;
}

.message-markdown :deep(a) {
  color: #3a7a3a;
  text-decoration: none;
}

.message-markdown :deep(a:hover) {
  text-decoration: underline;
}

.message-markdown :deep(code) {
  padding: 0.15em 0.35em;
  border-radius: 4px;
  background: #f3f6f1;
  color: #b04b67;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 0.88em;
}

.message-markdown :deep(pre) {
  overflow-x: auto;
  padding: 12px 14px;
  border-radius: 7px;
  background: #1f2430;
  color: #e8eaf0;
  font-size: 0.92em;
  line-height: 1.6;
  white-space: pre;
}

.message-markdown :deep(pre code) {
  padding: 0;
  background: transparent;
  color: inherit;
  white-space: pre;
}

.message-markdown :deep(blockquote) {
  margin-left: 0;
  padding-left: 12px;
  border-left: 3px solid #9fb39a;
  color: #686f7c;
}

.message-markdown :deep(table) {
  width: 100%;
  border-collapse: collapse;
}

.message-markdown :deep(th),
.message-markdown :deep(td) {
  padding: 6px 8px;
  border: 1px solid #dfe3ea;
  text-align: left;
}

.message-markdown :deep(th) {
  background: #f7f9f4;
}

.message-markdown :deep(img) {
  max-width: 100%;
}

.message-markdown :deep(hr) {
  margin: 0.8em 0;
  border: 0;
  border-top: 1px solid #e7eaf0;
}

.message-markdown :deep(.markdown-doc-reference) {
  display: inline-block;
  max-width: 220px;
  vertical-align: -3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conversation-error {
  padding: 10px 12px;
  border: 1px solid #ffccc7;
  border-radius: 8px;
  background: #fff1f0;
  color: #cf1322;
  font-size: 12px;
  margin-top: 6px;
}

.documents-link {
  flex: 0 0 auto;
}
</style>
