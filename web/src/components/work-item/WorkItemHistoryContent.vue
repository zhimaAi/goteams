<template>
  <div class="history-detail-page">
    <div class="history-detail-header">
      <div class="history-detail-heading">
        <h4>{{ $t("requirement_history_detailTitle") }}</h4>
        <p class="history-detail-meta">
          {{ $t("workspace_createdBy") }}: {{ detail.user_name || "-" }}
        </p>
        <p class="history-detail-meta">
          {{ $t("requirement_change_time") }}: {{ changeTimeText }}
        </p>
      </div>
      <div class="history-detail-legend">
        <span class="legend-item">
          <i class="legend-swatch legend-swatch--removed" />
          {{ $t("requirement_history_removed") }}
        </span>
        <span class="legend-item">
          <i class="legend-swatch legend-swatch--added" />
          {{ $t("requirement_history_added") }}
        </span>
      </div>
    </div>

    <div v-if="loading" class="history-detail-loading">
      <a-spin />
    </div>
    <a-empty
      v-else-if="loadFailed"
      class="history-detail-loading"
      :description="$t('requirement_history_loadFailed')"
    />
    <template v-else>
      <p v-if="diff.diffSkipped" class="history-detail-hint">
        {{ $t("requirement_history_diffSkipped") }}
      </p>
      <div class="history-detail-panes">
        <section
          v-for="pane in panes"
          :key="pane.side"
          class="history-pane"
          :class="`history-pane--${pane.side}`"
        >
          <div class="history-pane-bar">
            <span>{{ $t(pane.labelKey) }}</span>
            <a-button
              v-if="canRestore"
              type="link"
              size="small"
              :loading="restoring === pane.side"
              :disabled="restoring !== ''"
              @click="confirmRestore(pane.side)"
            >
              {{ $t("requirement_history_restoreToVersion") }}
            </a-button>
          </div>
          <!-- Both sides are sanitized in descriptionDiff before the highlight tags are added,
               so the rendered markup is safe to inject as is. -->
          <div v-if="pane.html" class="history-pane-body" v-html="pane.html" />
          <div v-else class="history-pane-body is-empty">
            {{ $t("requirement_history_emptySide") }}
          </div>
        </section>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { Modal, message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import {
  getRequirementHistory,
  restoreRequirementHistory,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { formatLocalDate } from "@/utils/date";
import { buildDescriptionDiff } from "@/utils/descriptionDiff";

const route = useRoute();
const { t } = useI18n();
const authStore = useAuthStore();

const wsId = computed(() => route.params.wsId);
const workItemId = computed(() => route.params.id);
const historyId = computed(() => route.params.historyId);

const loading = ref(true);
const loadFailed = ref(false);
const detail = ref({});
const restoring = ref("");

const panes = computed(() => [
  {
    side: "old",
    labelKey: "requirement_before_change",
    html: diff.value.before,
  },
  {
    side: "new",
    labelKey: "requirement_after_change",
    html: diff.value.after,
  },
]);

const diff = computed(() =>
  buildDescriptionDiff(
    detail.value.old_value,
    detail.value.new_value,
    detail.value.description_mode,
  ),
);

// Granularity follows the relative labels used on the other boards, extended down to seconds because
// a change record can be only seconds old when the list is opened right after an edit.
function relativeTimeText(timestamp) {
  const seconds = Number(timestamp);
  if (!Number.isFinite(seconds) || seconds <= 0) return "";
  const elapsed = Math.max(0, Math.floor(Date.now() / 1000) - seconds);
  if (elapsed < 60) return t("requirement_history_secondsAgo", { n: elapsed });
  if (elapsed < 3600) {
    return t("requirement_history_minutesAgo", { n: Math.floor(elapsed / 60) });
  }
  if (elapsed < 86400) {
    return t("requirement_history_hoursAgo", { n: Math.floor(elapsed / 3600) });
  }
  return t("requirement_history_daysAgo", { n: Math.floor(elapsed / 86400) });
}

// The absolute time is what users compare against the work item's own history; the bracketed part
// answers "how long ago" at a glance, as in the design. It is evaluated once per load instead of
// ticking, so a page left open keeps counting from when it was opened.
const changeTimeText = computed(() => {
  const absolute = formatLocalDate(detail.value.created_at, true);
  const relative = relativeTimeText(detail.value.created_at);
  return relative ? `${absolute}(${relative})` : absolute;
});

// Restoring rewrites the work item, so it needs the edit permission of the item type; without it
// the buttons stay hidden and the backend still rejects the call.
const canRestore = computed(() => {
  if (detail.value.field_name !== "description") return false;
  const type = detail.value.work_item_type;
  if (type !== "requirement" && type !== "defect") return false;
  return authStore.hasPermission(`${type}:edit`);
});

async function loadDetail() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const result = await getRequirementHistory(
      wsId.value,
      workItemId.value,
      historyId.value,
    );
    detail.value = result || {};
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

function confirmRestore(side) {
  Modal.confirm({
    title: t("requirement_history_restoreToVersion"),
    content: t("requirement_history_restoreConfirm"),
    okText: t("common_ok"),
    cancelText: t("common_cancel"),
    // Restoring overwrites the current description, so the confirm action is styled as destructive
    // and the dialog can be dismissed with the close icon as well.
    okButtonProps: { danger: true },
    closable: true,
    // The design puts confirm on the left and cancel on the right; the class carries the rule that
    // reverses the footer (see the unscoped style block at the end of this file).
    wrapClassName: "history-restore-confirm",
    onOk: () => restore(side),
  });
}

async function restore(side) {
  restoring.value = side;
  try {
    await restoreRequirementHistory(
      wsId.value,
      workItemId.value,
      historyId.value,
      side,
    );
    // The record itself is immutable, so the panes stay as they are; the restore shows up as a new
    // record in the work item's change history.
    message.success(t("requirement_history_restoreSuccess"));
  } catch {
    message.error(t("requirement_history_restoreFailed"));
  } finally {
    restoring.value = "";
  }
}

watch(
  [wsId, workItemId, historyId],
  () => {
    loadDetail();
  },
  { immediate: true },
);
</script>

<style scoped lang="less">
// The page sits outside WorkspaceLayout (the design has no sidebar), so it cannot take its height
// from a flex parent; it fills the viewport itself, like the other standalone pages. The two panes
// scroll on their own, so the page itself never scrolls.
.history-detail-page {
  display: flex;
  height: 100vh;
  flex-direction: column;
  padding: 12px 16px 16px;
  overflow: hidden;
  background: var(--app-surface);
}

.history-detail-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--app-border);
}

.history-detail-heading h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--app-text);
}

.history-detail-meta {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--app-text-secondary);
}

.history-detail-legend {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 16px;
  font-size: 12px;
  color: var(--app-text-secondary);
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.legend-swatch {
  width: 12px;
  height: 12px;
  border-radius: 2px;
}

.legend-swatch--removed {
  background: #ffccc7;
}

.legend-swatch--added {
  background: #b7eb8f;
}

.history-detail-hint {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--app-text-secondary);
}

.history-detail-panes {
  display: grid;
  flex: 1;
  min-height: 0;
  margin-top: 10px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.history-pane {
  display: flex;
  min-width: 0;
  flex-direction: column;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  overflow: hidden;
}

// The restore action sits right next to the pane label, as in the design, instead of being pushed
// to the far edge of the bar.
.history-pane-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-surface-muted);
  font-size: 13px;
  color: var(--app-text);
}

.history-pane-bar :deep(.ant-btn-link) {
  height: auto;
  padding: 0;
  font-size: 13px;
}

.history-pane-body {
  flex: 1;
  min-width: 0;
  padding: 10px;
  overflow: auto;
  font-size: 13px;
  line-height: 1.7;
  color: var(--app-text);
  word-break: break-word;
}

.history-pane-body.is-empty {
  color: var(--app-text-secondary);
}

.history-detail-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
}

// The highlight markup is produced by the diff, so the styles have to pierce scoped CSS.
.history-pane-body :deep(del) {
  background: #ffecec;
  color: #cf1322;
  text-decoration: line-through;
}

.history-pane-body :deep(ins) {
  background: #eaffe8;
  color: #237804;
  text-decoration: none;
}

.history-pane-body :deep(.is-removed) {
  background: #fff1f0;
}

.history-pane-body :deep(.is-added) {
  background: #f6ffed;
}

// Stored descriptions may carry images and media from the editor; keep them inside the pane.
.history-pane-body :deep(img),
.history-pane-body :deep(video) {
  max-width: 100%;
  height: auto;
}
</style>

<!--
  The confirmation dialog is rendered into <body>, so a scoped style cannot reach it. The rule is
  confined by the wrapClassName set on this one dialog instead of by scoping.
-->
<style lang="less">
.history-restore-confirm .ant-modal-confirm-btns {
  display: flex;
  // row-reverse puts the confirm action first from the left, matching the design; flex-start keeps
  // the button group right-aligned as before because the reversed main axis starts on the right.
  flex-direction: row-reverse;
  justify-content: flex-start;
  // antd spaces these buttons with the whitespace between them, which only renders while they are
  // inline-level. Turning the footer into a flex container drops that whitespace, so the gap has to
  // carry the spacing instead.
  gap: 8px;
}

// The design draws both dismiss affordances (the close icon and the cancel button) in the brand
// blue, while the confirm action keeps the destructive red supplied by okButtonProps. The colour is
// the same #3157E2 that App.vue passes to antd as colorPrimary.
.history-restore-confirm .ant-modal-close,
.history-restore-confirm .ant-modal-close:hover,
.history-restore-confirm .ant-modal-close:focus,
.history-restore-confirm
  .ant-modal-confirm-btns
  .ant-btn:not(.ant-btn-primary) {
  color: #3157e2;
}
</style>
