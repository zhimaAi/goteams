<template>
  <div v-if="rows.length" class="exec-process">
    <button
      type="button"
      class="exec-summary"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <ThunderboltOutlined class="exec-summary-glyph" />
      <span class="exec-summary-title">{{ t("agent_process_title") }}</span>
      <span v-if="summaryChip" class="exec-summary-chip">{{
        summaryChip
      }}</span>
      <span class="exec-summary-gap" />
      <span class="exec-summary-state" :class="stateClass">
        <CheckOutlined v-if="succeeded" />
        <CloseOutlined v-else-if="failed" />
        <MinusCircleOutlined v-else-if="interrupted" />
        <LoadingOutlined v-else-if="running" spin />
        {{ stateText }}
      </span>
      <DownOutlined v-if="expanded" class="exec-caret" />
      <RightOutlined v-else class="exec-caret" />
    </button>

    <div v-if="expanded" class="exec-body">
      <div v-for="row in rows" :key="row.key" class="exec-row">
        <button
          v-if="row.kind === 'thinking'"
          type="button"
          class="exec-row-head"
          :aria-expanded="isOpen(row)"
          @click="toggle(row)"
        >
          <BulbOutlined class="exec-row-icon icon-thinking" />
          <span class="exec-row-label">{{ t("agent_process_thinking") }}</span>
          <span class="exec-row-gap" />
          <span v-if="row.durationText" class="exec-duration is-done">
            <i class="exec-dot" />{{ row.durationText }}
          </span>
          <span class="exec-row-caret">
            <MinusOutlined v-if="isOpen(row)" />
            <PlusOutlined v-else />
          </span>
        </button>
        <div v-else-if="row.kind === 'output'" class="exec-row-head is-static">
          <MessageOutlined class="exec-row-icon" />
          <span class="exec-row-label">{{ t("agent_process_output") }}</span>
          <span class="exec-row-gap" />
        </div>
        <div
          v-else-if="row.kind === 'permission'"
          class="exec-row-head is-static"
        >
          <StopOutlined class="exec-row-icon icon-permission" />
          <span class="exec-row-label is-permission">{{
            t("agent_process_permission")
          }}</span>
          <span class="exec-row-gap" />
        </div>
        <button
          v-else
          type="button"
          class="exec-row-head"
          :aria-expanded="isOpen(row)"
          @click="toggle(row)"
        >
          <CheckCircleOutlined
            v-if="row.result"
            class="exec-row-icon icon-done"
          />
          <MinusCircleOutlined v-else class="exec-row-icon" />
          <span class="exec-row-tool">{{ row.name }}</span>
          <span v-if="row.args" class="exec-row-args" :title="row.args">{{
            row.args
          }}</span>
          <span class="exec-row-gap" />
          <span
            v-if="row.durationText"
            class="exec-duration"
            :class="row.result ? 'is-done' : ''"
          >
            <i class="exec-dot" />{{ row.durationText }}
          </span>
          <span v-if="row.result" class="exec-row-state is-done">{{
            t("agent_process_done")
          }}</span>
          <span class="exec-row-caret">
            <MinusOutlined v-if="isOpen(row)" />
            <PlusOutlined v-else />
          </span>
        </button>

        <div
          v-if="row.kind === 'thinking' && isOpen(row)"
          class="exec-row-text"
        >
          {{ row.content }}
        </div>
        <div v-else-if="row.kind === 'output'" class="exec-row-text">
          {{ row.content }}
        </div>
        <div
          v-else-if="row.kind === 'permission'"
          class="exec-row-text is-permission-text"
        >
          {{ row.content }}
        </div>
        <div v-else-if="isOpen(row) && row.result" class="exec-tool-result">
          <p class="exec-result-label">{{ t("agent_process_result") }}</p>
          <p class="exec-row-text is-plain">{{ row.result }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  BulbOutlined,
  CheckCircleOutlined,
  CheckOutlined,
  CloseOutlined,
  DownOutlined,
  LoadingOutlined,
  MessageOutlined,
  MinusCircleOutlined,
  MinusOutlined,
  PlusOutlined,
  RightOutlined,
  StopOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons-vue";
import { buildExecutionProcessRows } from "./processEvents.js";

const props = defineProps({
  events: { type: Array, default: () => [] },
  status: { type: String, default: "" },
  cliType: { type: String, default: "" },
  modelName: { type: String, default: "" },
  inputTokens: { type: Number, default: 0 },
  outputTokens: { type: Number, default: 0 },
});

const { t } = useI18n();
const expanded = ref(false);
const openOverrides = ref({});

const CLI_NAMES = {
  claude: "Claude Code",
  codebuddy: "CodeBuddy Code",
  codex: "Codex CLI",
  cursor: "Cursor Agent",
  opencode: "OpenCode",
  copilot: "GitHub Copilot CLI",
  grok: "Grok CLI",
  hermes: "Hermes",
  kimi: "Kimi Code",
  qoder: "Qoder CLI",
  "qoder-cn": "Qoder CLI (CN)",
  qwen: "Qwen Code",
  openclaw: "OpenClaw",
  pi: "Pi Agent",
};

const rows = computed(() => buildExecutionProcessRows(props.events));
const running = computed(() =>
  ["created", "running", "waiting_input", "stop_requested"].includes(
    props.status,
  ),
);
const failed = computed(() => props.status === "failed");
const interrupted = computed(() =>
  ["stopped", "interrupted"].includes(props.status),
);
const succeeded = computed(() => props.status === "success");
const stateText = computed(() => {
  if (running.value) return t("agent_process_running");
  if (failed.value) return t("agent_process_failed");
  if (props.status === "stopped") return t("pipeline_exec_stopped");
  if (props.status === "interrupted") return t("pipeline_session_interrupted");
  if (succeeded.value) return t("agent_process_state_done");
  return t("pipeline_exec_idle");
});
const stateClass = computed(() => ({
  "is-running": running.value,
  "is-failed": failed.value,
  "is-done": succeeded.value,
}));
const toolCount = computed(
  () => rows.value.filter((row) => row.kind === "tool").length,
);
const summaryChip = computed(() => {
  const parts = [];
  const cli = String(props.cliType || "").trim();
  if (cli) parts.push(CLI_NAMES[cli] || cli.toUpperCase());
  if (props.modelName) parts.push(props.modelName);
  if (toolCount.value) {
    parts.push(t("agent_process_tool_count", { count: toolCount.value }));
  }
  const usage = usageText.value;
  if (usage) parts.push(usage);
  return parts.join(" · ");
});
const usageText = computed(() => {
  const input = Number(props.inputTokens) || 0;
  const output = Number(props.outputTokens) || 0;
  if (input <= 0 && output <= 0) return "";
  return `↑${formatTokens(input)} ↓${formatTokens(output)}`;
});

watch(
  () => props.status,
  (status) => {
    if (status === "running" || status === "created") expanded.value = true;
  },
  { immediate: true },
);

function formatTokens(value) {
  if (value >= 1000) return `${(value / 1000).toFixed(1)}k`;
  return String(value);
}

function isOpen(row) {
  if (Object.prototype.hasOwnProperty.call(openOverrides.value, row.key)) {
    return openOverrides.value[row.key];
  }
  return false;
}

function toggle(row) {
  openOverrides.value = {
    ...openOverrides.value,
    [row.key]: !isOpen(row),
  };
}
</script>

<style scoped>
.exec-process {
  margin: 4px 0 8px;
  padding: 8px 12px;
  border: 1px solid #eef0f3;
  border-radius: 10px;
  background: #fff;
}

.exec-summary {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 6px;
  padding: 3px 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.exec-summary-glyph {
  flex: 0 0 auto;
  color: #8b8b8b;
  font-size: 13px;
}

.exec-summary-title {
  flex: 0 0 auto;
  color: #1f2328;
  font-size: 14px;
  font-weight: 500;
}

.exec-summary-chip {
  min-width: 0;
  overflow: hidden;
  padding: 1px 8px;
  border-radius: 6px;
  background: #f2f3f5;
  color: #8b8b8b;
  font-size: 12px;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.exec-summary-gap {
  flex: 1;
  min-width: 8px;
}

.exec-summary-state {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 4px;
  color: #8b8b8b;
  font-size: 12px;
}

.exec-summary-state.is-done {
  color: #16a34a;
}

.exec-summary-state.is-running {
  color: #185fa5;
}

.exec-summary-state.is-failed {
  color: #dc2626;
}

.exec-body {
  display: flex;
  flex-direction: column;
  padding: 2px 0 2px 1px;
}

.exec-row {
  min-width: 0;
}

.exec-row + .exec-row {
  border-top: 1px solid #f4f5f7;
}

.exec-row-head {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 6px;
  padding: 4px 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.exec-row-head.is-static {
  cursor: default;
}

.exec-row-icon {
  flex: 0 0 auto;
  color: #b9bcc2;
  font-size: 13px;
}

.icon-thinking,
.icon-done {
  color: #8b8b8b;
}

.icon-permission {
  color: #dc2626;
}

.exec-row-label {
  flex: 0 0 auto;
  color: #595959;
  font-size: 13px;
}

.exec-row-label.is-permission {
  color: #dc2626;
}

.exec-row-tool {
  flex: 0 0 auto;
  color: #1f2328;
  font-family: "SF Mono", Menlo, Consolas, monospace;
  font-size: 13px;
  font-weight: 500;
}

.exec-row-args {
  min-width: 0;
  overflow: hidden;
  color: #9ca3af;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.exec-row-gap {
  flex: 1;
  min-width: 8px;
}

.exec-duration {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 4px;
  color: #6b7280;
  font-size: 12px;
}

.exec-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #16a34a;
}

.exec-caret {
  flex: 0 0 auto;
  color: #c4c7cc;
  font-size: 10px;
}

.exec-row-caret {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  color: #8b8b8b;
  background: #fff;
  font-size: 10px;
}

.exec-row-state {
  flex: 0 0 auto;
  color: #9ca3af;
  font-size: 12px;
  white-space: nowrap;
}

.exec-row-text {
  margin: 2px 0 8px 19px;
  padding: 8px 10px;
  border-radius: 8px;
  background: #f2f3f5;
  color: #4b5563;
  font-family: "SF Mono", Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.exec-row-text.is-permission-text {
  background: #fef2f2;
  color: #a32d2d;
}

.exec-result-label {
  margin: 0;
  padding-left: 19px;
  color: #9ca3af;
  font-size: 12px;
}

.exec-row-text.is-plain {
  margin-bottom: 8px;
}
</style>
