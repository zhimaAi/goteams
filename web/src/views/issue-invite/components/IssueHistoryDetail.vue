<template>
  <div class="issue-history-detail-page">
    <!-- Content section -->
    <section class="issue-history-detail">
      <div class="issue-history-detail__card">
        <!-- Title -->
        <h2 class="issue-history-detail__title">{{ detail?.title }}</h2>

        <!-- Description -->
        <p
          v-if="detail?.description"
          class="issue-history-detail__description"
          v-html="renderedDescription"
        ></p>

        <!-- Attribute rows: status always first, template-enabled fields next, submit
             time last. Rows mirror the fields of the template the issue was submitted
             through, so a configured-but-empty field stays visible with a placeholder. -->
        <div class="issue-history-detail__attrs">
          <div class="issue-history-detail__attr-row">
            <span class="issue-history-detail__attr-label">{{
              $t("requirement_status")
            }}</span>
            <span class="issue-history-detail__attr-value">
              <span
                v-if="detail?.status_name"
                class="issue-history-detail__pill"
                :style="pillStyle(detail.status_color)"
                >{{ detail.status_name }}</span
              >
              <span v-else class="issue-history-detail__attr-empty">{{
                EMPTY
              }}</span>
            </span>
          </div>

          <div
            v-for="row in attributeRows"
            :key="row.key"
            class="issue-history-detail__attr-row"
          >
            <span class="issue-history-detail__attr-label">{{
              row.label
            }}</span>
            <span class="issue-history-detail__attr-value">
              <span v-if="row.text" class="issue-history-detail__attr-text">{{
                row.text
              }}</span>
              <span v-else class="issue-history-detail__attr-empty">{{
                EMPTY
              }}</span>
            </span>
          </div>

          <div class="issue-history-detail__attr-row">
            <span class="issue-history-detail__attr-label">{{
              $t("issue_invite_submit_time")
            }}</span>
            <span class="issue-history-detail__attr-value">
              <span
                v-if="detail?.created_at"
                class="issue-history-detail__attr-text"
                >{{ formatLocalDate(detail.created_at, true) }}</span
              >
              <span v-else class="issue-history-detail__attr-empty">{{
                EMPTY
              }}</span>
            </span>
          </div>
        </div>
      </div>

      <!-- Handle records -->
      <div class="issue-history-detail__records-card">
        <h3 class="issue-history-detail__records-title">
          {{ $t("issue_invite_handle_records") }}
        </h3>

        <div
          v-if="recordsLoading"
          class="issue-history-detail__records-loading"
        >
          <a-spin />
        </div>
        <a-timeline
          v-else-if="records.length"
          class="issue-history-detail__records-timeline"
        >
          <a-timeline-item v-for="(record, index) in records" :key="index">
            <template #dot>
              <span
                :class="[
                  'issue-history-detail__timeline-dot',
                  'issue-history-detail__timeline-dot' + index,
                ]"
              />
            </template>
            <p class="issue-history-detail__record-text">
              <template v-if="record.type === 'comment'">{{
                $t("issue_invite_comment_add")
              }}</template>
              <template v-else-if="record.type === 'reply'">{{
                $t("issue_invite_comment_reply", { actor: record.actor_name })
              }}</template>
              <template v-else-if="record.type === 'at'">{{
                $t("issue_invite_comment_mention", {
                  actor: record.actor_name,
                })
              }}</template>
              <template v-else-if="record.type === 'create'">{{
                $t("issue_invite_create_ticket")
              }}</template>
              <template v-else-if="record.type === 'recall'">{{
                $t("issue_invite_recall_ticket")
              }}</template>
              <template v-else>{{
                $t("issue_invite_status_changed", { status: record.to_name })
              }}</template>
            </p>
            <div
              v-if="record.content_html"
              class="issue-history-detail__record-content"
              v-html="renderSafeDescription(record.content_html)"
            />
            <p
              v-else-if="isCommentRecord(record.type)"
              class="issue-history-detail__record-content"
            >
              {{ record.content }}
            </p>
            <div
              v-if="record.attachments?.length"
              class="issue-history-detail__record-attachments"
            >
              <template
                v-for="file in recordAttachments(record)"
                :key="file.id"
              >
                <a-image
                  v-if="file.kind === 'image'"
                  :src="file.url"
                  :alt="file.name"
                  :width="120"
                />
                <video
                  v-else-if="file.kind === 'video'"
                  :src="file.url"
                  :aria-label="file.name"
                  controls
                  preload="metadata"
                />
                <a
                  v-else
                  :href="file.url"
                  :download="file.name"
                  target="_blank"
                  rel="noopener noreferrer"
                  >{{ file.name }}</a
                >
              </template>
            </div>
            <span class="issue-history-detail__record-time">{{
              formatLocalDate(record.created_at, true)
            }}</span>
          </a-timeline-item>
        </a-timeline>
        <div v-else class="issue-history-detail__records-empty">
          {{ $t("issue_invite_no_records") }}
        </div>
      </div>

      <!-- 底部固定操作栏：改并重提 / 评论 / 撤回 / 催单；评论点开走底部抽屉 -->
      <div class="issue-history-detail__bar">
        <div class="issue-history-detail__bar-actions">
          <button
            v-if="detail?.can_modify"
            type="button"
            class="issue-history-detail__bar-btn"
            :disabled="acting"
            @click="emit('edit')"
          >
            <RetweetOutlined class="issue-history-detail__bar-icon" />
            <span class="issue-history-detail__bar-label">{{
              $t("issue_invite_modify_resubmit")
            }}</span>
          </button>
          <button
            type="button"
            class="issue-history-detail__bar-btn"
            @click="commentOpen = true"
          >
            <MessageOutlined
              class="issue-history-detail__bar-icon issue-history-detail__bar-icon--message"
            />
            <span class="issue-history-detail__bar-label">{{
              $t("requirement_comment")
            }}</span>
          </button>
          <!-- 撤回 / 催单：只在后端说「现在能做」时才出现 -->
          <button
            v-if="detail?.can_recall"
            type="button"
            class="issue-history-detail__bar-btn"
            :disabled="acting"
            @click="recallOpen = true"
          >
            <!-- 撤回专用 SVG 图标：fill 用 currentColor，跟随底栏图标的灰色/选中色 -->
            <svg
              class="issue-history-detail__bar-icon issue-history-detail__bar-icon--recall"
              viewBox="0 0 1024 1024"
              width="1em"
              height="1em"
              aria-hidden="true"
            >
              <path
                d="M581.056 288.32H232.96l108.352-102.208c15.552-15.744 19.456-31.104 4.16-46.208-16.064-15.872-32.576-15.808-48.64 0l-145.92 144.768c-8.768 8.832-23.488 20.608-22.08 32.448l0.64 4.8-0.64 4.864c-1.344 11.776 6.4 18.24 14.848 26.816l147.648 145.216c16.064 15.808 38.08 20.992 54.144 5.12 15.296-15.104 3.84-38.208-11.328-53.504L233.152 353.6 581.056 352c126.464 0 250.944 111.488 250.944 236.16C832 712.832 707.52 832 581.056 832H246.4c-22.592 0-29.696 9.6-29.696 32.256s7.04 31.744 29.696 31.744H581.12C755.136 896 896 757.696 896 588.16c0-169.408-140.8-299.84-314.944-299.84z"
                fill="currentColor"
              />
              <path
                d="M323.392 192a32 32 0 1 1 0-64 32 32 0 0 1 0 64zM320.192 514.048a32 32 0 1 1 0-64 32 32 0 0 1 0 64zM237.824 896a32 32 0 1 1 0-64 32 32 0 0 1 0 64z"
                fill="currentColor"
              />
            </svg>
            <span class="issue-history-detail__bar-label">{{
              $t("issue_invite_recall_action")
            }}</span>
          </button>
          <a-tooltip v-if="showUrge" :title="urgeHint">
            <button
              type="button"
              class="issue-history-detail__bar-btn"
              :disabled="acting || !detail?.can_urge"
              @click="emit('urge')"
            >
              <ThunderboltOutlined
                class="issue-history-detail__bar-icon issue-history-detail__bar-icon--urge"
              />
              <span class="issue-history-detail__bar-label">{{
                $t("issue_invite_urge_action")
              }}</span>
            </button>
          </a-tooltip>
        </div>
      </div>
    </section>

    <!-- 撤回确认：从底部弹出的抽屉，红色主按钮在上、取消在下，两个都整宽 -->
    <a-drawer
      v-model:open="recallOpen"
      placement="bottom"
      height="auto"
      :closable="false"
      root-class-name="issue-recall-drawer"
    >
      <div class="issue-history-detail__recall">
        <h3 class="issue-history-detail__recall-title">
          {{ $t("issue_invite_recall_confirm_title") }}
        </h3>
        <p class="issue-history-detail__recall-content">
          {{ $t("issue_invite_recall_confirm_content") }}
        </p>
        <a-button
          type="primary"
          danger
          class="issue-history-detail__recall-ok"
          :loading="acting"
          @click="confirmRecall"
        >
          {{ $t("issue_invite_recall_action") }}
        </a-button>
        <a-button
          class="issue-history-detail__recall-cancel"
          :disabled="acting"
          @click="recallOpen = false"
        >
          {{ $t("common_cancel") }}
        </a-button>
      </div>
    </a-drawer>

    <!-- 评论：从底部弹出的抽屉，标题 + 关闭，输入区 + 右下角发送 -->
    <a-drawer
      v-model:open="commentOpen"
      placement="bottom"
      height="auto"
      :closable="false"
      root-class-name="issue-comment-drawer"
    >
      <div class="issue-history-detail__comment">
        <div class="issue-history-detail__comment-head">
          <h3 class="issue-history-detail__comment-title">
            {{ $t("requirement_comment") }}
          </h3>
          <CloseOutlined
            class="issue-history-detail__comment-close"
            @click="commentOpen = false"
          />
        </div>
        <div class="issue-history-detail__comment-editor">
          <a-textarea
            ref="commentInputRef"
            v-model:value="commentText"
            class="issue-history-detail__comment-input"
            :placeholder="$t('issue_invite_comment_placeholder')"
            :maxlength="2000"
            :auto-size="{ minRows: 4, maxRows: 8 }"
            @keydown.enter.exact.prevent="sendComment"
          />
          <a-button
            type="primary"
            class="issue-history-detail__comment-send"
            :disabled="!commentText.trim()"
            :loading="sending"
            @click="sendComment"
          >
            <template #icon>
              <ArrowUpOutlined />
            </template>
          </a-button>
        </div>
      </div>
    </a-drawer>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  ArrowUpOutlined,
  CloseOutlined,
  MessageOutlined,
  RetweetOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons-vue";
import DOMPurify from "dompurify";
import { formatLocalDate } from "@/utils/date";
import {
  createPublicIssueComment,
  getPublicIssueHandleRecords,
} from "@/api/issueInvite";

const props = defineProps({
  detail: { type: Object, default: null },
  inviteKey: { type: String, default: "" },
  openid: { type: String, default: "" },
  lang: { type: String, default: "" },
  // 撤回/催单请求进行中，按钮转圈并防重复点
  acting: { type: Boolean, default: false },
});

// 撤回、催单、改并重提都由页面去调接口，这里只负责发起
const emit = defineEmits(["recall", "urge", "edit"]);

const { t } = useI18n();

const records = ref([]);
const recordsLoading = ref(false);
const commentText = ref("");
const sending = ref(false);
// 评论抽屉开关，点底栏的「评论」弹出
const commentOpen = ref(false);
const commentInputRef = ref(null);

// 抽屉弹好后把光标放进输入框
watch(commentOpen, (open) => {
  if (open) nextTick(() => commentInputRef.value?.focus?.());
});

// 撤回确认弹窗
const recallOpen = ref(false);

function confirmRecall() {
  recallOpen.value = false;
  emit("recall");
}

async function loadRecords() {
  const requirementId = props.detail?.requirement_id;
  if (!props.inviteKey || !props.openid || !requirementId) return;
  recordsLoading.value = true;
  try {
    const data = await getPublicIssueHandleRecords(
      props.inviteKey,
      props.openid,
      requirementId,
      props.lang,
    );
    records.value = data?.records || [];
  } catch (error) {
    records.value = [];
  } finally {
    recordsLoading.value = false;
  }
}

async function sendComment() {
  const content = commentText.value.trim();
  if (!content) {
    message.warning(t("issue_invite_comment_empty"));
    return;
  }
  if (sending.value) return;
  sending.value = true;
  try {
    await createPublicIssueComment(
      props.inviteKey,
      props.openid,
      props.detail.requirement_id,
      content,
      props.lang,
    );
    commentText.value = "";
    commentOpen.value = false;
    await loadRecords();
  } catch (error) {
    message.error(t("issue_invite_comment_send_failed"));
  } finally {
    sending.value = false;
  }
}

onMounted(loadRecords);
// 撤回/重提后父级会换成新的详情对象，办理记录要跟着重载（撤回记录就是这么冒出来的）
watch(() => props.detail, loadRecords);

// Placeholder shown when an attribute value is empty; keeps the row readable.
const EMPTY = "—";

// 处理记录里带正文的类型：邀填人自己的评论、内部人的回复与@。
function isCommentRecord(recordType) {
  return (
    recordType === "comment" || recordType === "reply" || recordType === "at"
  );
}

// Outline pill for the status row: text and border carry the workspace-configured
// status color returned by the backend; fall back to a neutral gray when absent.
function pillStyle(color) {
  const value = color || "#8c8c9c";
  return { color: value, borderColor: value };
}

// The description may contain the server-appended attachment block (inline <img>/<a>);
// sanitize before rendering because the description text itself is submitter-controlled.
const renderedDescription = computed(() =>
  renderSafeDescription(props.detail?.description || ""),
);

// Legacy rows saved before the server appended <video> tags still carry video files as
// plain links; upgrade them to an inline player here. The <video> element is built after
// sanitization from the already-safe href, so no markup bypasses DOMPurify.
const VIDEO_FILE_RE = /\.(mp4|webm|ogg|mov)([?#]|$)/i;

function attachmentURLKey(value) {
  try {
    const url = new URL(value, window.location.href);
    return url.origin + url.pathname;
  } catch {
    return "";
  }
}

// Uploaded files already embedded in the rich reply should not appear a second time.
function recordAttachments(record) {
  const container = document.createElement("div");
  container.innerHTML = DOMPurify.sanitize(record.content_html || "");
  const embedded = new Set(
    [...container.querySelectorAll("[src], a[href]")].map((node) =>
      attachmentURLKey(node.getAttribute("src") || node.getAttribute("href")),
    ),
  );
  return (record.attachments || []).filter(
    (file) => !embedded.has(attachmentURLKey(file.url)),
  );
}

function renderSafeDescription(raw) {
  const container = document.createElement("div");
  container.innerHTML = DOMPurify.sanitize(raw);
  for (const link of container.querySelectorAll("a[href]")) {
    const href = link.getAttribute("href");
    let path = "";
    try {
      path = new URL(href, window.location.href).pathname;
    } catch (error) {
      continue;
    }
    if (!VIDEO_FILE_RE.test(path)) continue;
    const video = document.createElement("video");
    video.controls = true;
    video.preload = "metadata";
    video.src = href;
    link.replaceWith(video);
  }
  return container.innerHTML;
}

function formatFieldValue(value) {
  if (value === undefined || value === null) {
    return "";
  }
  if (Array.isArray(value)) {
    return value
      .map((item) => (item === null || item === undefined ? "" : String(item)))
      .filter(Boolean)
      .join("、");
  }
  return String(value);
}

// Date custom fields are stored as UTC ISO strings; render them in the browser timezone.
function formatDateFieldValue(value) {
  if (value === undefined || value === null || value === "") {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  const pad = (n) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function formatCustomFieldValue(field) {
  if (field?.type === "date") {
    return formatDateFieldValue(field.value);
  }
  return formatFieldValue(field?.value);
}

// 内置字段在模板里没有配置名，按 field_key 取文案（跟填写表单同一套 key）
const BUILTIN_LABEL_KEYS = {
  version: "workspace_field_iteration",
  category: "workspace_field_category",
  priority: "workspace_field_priority",
  planned_start_date: "workspace_field_plannedStartDate",
  planned_end_date: "workspace_field_plannedEndDate",
  assignee: "workspace_field_assignee",
  estimated_work_hours: "requirement_estimatedWorkHours",
  completed_work_hours: "requirement_completedWorkHours",
};

// 计划起止后端给的是时间戳，要按浏览器时区转成日期时间
const BUILTIN_DATE_KEYS = new Set(["planned_start_date", "planned_end_date"]);

function fieldLabel(field) {
  if (field.name) return field.name;
  const key = BUILTIN_LABEL_KEYS[field.field_key];
  return key ? t(key) : field.field_key;
}

function fieldText(field) {
  if (BUILTIN_DATE_KEYS.has(field.field_key)) {
    return formatLocalDate(Number(field.value), true);
  }
  return formatCustomFieldValue(field);
}

// 属性行直接用后端给的 fields：内置字段在前、自定义字段在后，都是展示值。
// 提交时留空的字段后端不会带出来，这里也就不会有空行。
const attributeRows = computed(() =>
  (props.detail?.fields || []).map((field) => ({
    key: field.field_key,
    label: fieldLabel(field),
    text: fieldText(field),
  })),
);

// 催单按钮出现在工单还没结束也没撤回的时候：能催时后端给 can_urge，
// 冷却中给 urge_available_at，两个都为假说明不该显示这个按钮。
const showUrge = computed(() => {
  const detail = props.detail || {};
  return !!detail.can_urge || Number(detail.urge_available_at) > 0;
});

const urgeHint = computed(() => {
  const detail = props.detail || {};
  if (detail.can_urge) return "";
  return t("issue_invite_urge_wait", {
    time: formatLocalDate(Number(detail.urge_available_at), true),
  });
});
</script>

<style scoped lang="less">
.issue-history-detail-page {
  width: 100%;
}

.issue-history-detail {
  // 给底部固定操作栏留出空间
  padding: 12px 16px 76px;

  &__card {
    background: #fff;
    border-radius: 8px;
    padding: 16px;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
  }

  &__title {
    margin: 0 0 8px;
    font-size: 18px;
    font-weight: 600;
    line-height: 26px;
    color: #1d2129;
  }

  &__description {
    margin: 0 0 12px;
    font-size: 14px;
    line-height: 22px;
    color: #4e5969;
    white-space: pre-wrap;
    word-break: break-word;

    // The description is injected via v-html, so its inner img/a never carry the
    // scoped attribute; :deep keeps the constraint rules from being dropped.
    :deep(img) {
      max-width: 100%;
      height: auto;
      border-radius: 4px;
      display: block;
      margin: 4px 0;
    }

    :deep(video) {
      max-width: 100%;
      display: block;
      margin: 4px 0;
      border-radius: 4px;
    }

    :deep(a) {
      color: #0d68ff;
      word-break: break-all;
    }
  }

  // Attribute list below the content: label on the left, value on the right, hairline
  // separators between rows (mirrors the workspace requirement detail style).
  &__attrs {
    display: flex;
    flex-direction: column;
  }

  &__attr-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 10px 0;

    & + & {
      border-top: 1px solid #f2f3f5;
    }
  }

  &__attr-label {
    flex-shrink: 0;
    font-size: 13px;
    line-height: 20px;
    color: #86909c;
  }

  &__attr-value {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    min-width: 0;
    font-size: 13px;
    line-height: 20px;
    color: #1d2129;
    text-align: right;
    word-break: break-word;
  }

  &__attr-text {
    overflow: hidden;
  }

  &__attr-empty {
    color: #c9cdd4;
  }

  &__pill {
    display: inline-flex;
    align-items: center;
    padding: 2px 8px;
    font-size: 12px;
    line-height: 20px;
    border: 1px solid transparent;
    border-radius: 12px;
    white-space: nowrap;
  }

  // 撤回确认抽屉：标题正文居中，确定/取消两个整宽按钮上下排
  &__recall {
    display: flex;
    flex-direction: column;
    gap: 12px;
    text-align: center;
  }

  &__recall-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
    color: #1d2129;
  }

  &__recall-content {
    margin: 0 0 8px;
    font-size: 13px;
    line-height: 20px;
    color: #4e5969;
  }

  &__recall-ok,
  &__recall-cancel {
    width: 100%;
    height: 40px;
    border-radius: 6px;
  }

  &__recall-cancel {
    border-color: #d9d9d9;
  }

  &__records-card {
    margin-top: 12px;
    background: #fff;
    border-radius: 8px;
    padding: 16px;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
  }

  &__records-title {
    margin: 0 0 20px;
    font-size: 15px;
    font-weight: 600;
    line-height: 22px;
    color: #1d2129;
  }

  &__records-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px 0;
  }

  &__records-empty {
    padding: 24px 0;
    font-size: 13px;
    line-height: 20px;
    color: #86909c;
    text-align: center;
  }

  // Timeline: the rail runs along the left edge of the card and the brand-blue ring
  // marks each record. antd Timeline draws the connector line through the dots; we
  // only recolor it and shrink the head so the ring sits flush on the rail.
  &__records-timeline {
    margin-top: 4px;

    :deep(.ant-timeline-item-tail) {
      border-left-color: #d9d9d9;
    }

    :deep(.ant-timeline-item-head) {
      background: transparent;
    }
  }

  // Brand-blue ring dot (matches the reference image). 10px content + 2px border =
  // 14px, matching the head box above.
  &__timeline-dot {
    display: block;
    width: 12px;
    height: 12px;
    border: 2px solid #a6b1c3;
    border-radius: 50%;
    background: #fff;
    box-sizing: border-box;
    position: relative;
    &::before {
      position: absolute;
      content: "";
      width: 4px;
      height: 4px;
      top: 2px;
      left: 2px;
      background: #a6b1c3;
      border-radius: 100%;
    }
  }
  &__timeline-dot0 {
    border: 2px solid #2475fc;
    &::before {
      background: #2475fc;
    }
  }

  &__record-text {
    margin: 0;
    font-size: 14px;
    line-height: 22px;
    color: #1d2129;
    white-space: pre-wrap;
    word-break: break-word;
    color: var(--02, #262626);
    font-weight: 600;
  }

  &__record-content {
    margin: 2px 0 0;
    font-size: 14px;
    line-height: 22px;
    color: #595959;
    white-space: pre-wrap;
    word-break: break-word;

    :deep(img),
    :deep(video) {
      max-width: 100%;
      height: auto;
      border-radius: 6px;
    }

    :deep(p) {
      margin: 0 0 4px;
    }

    :deep(a) {
      overflow-wrap: anywhere;
    }
  }

  &__record-attachments {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 8px;
    margin-top: 8px;

    video {
      max-width: 100%;
      max-height: 240px;
      border-radius: 6px;
    }

    a {
      overflow-wrap: anywhere;
    }
  }

  &__record-time {
    display: block;
    margin: 4px 0 0;
    font-size: 13px;
    line-height: 20px;
    color: #86909c;
  }

  // 底部固定操作栏：跟着 998px 页面列居中，按钮是「图标在上、文字在下」
  &__bar {
    position: fixed;
    left: 50%;
    bottom: 0;
    transform: translateX(-50%);
    z-index: 20;
    width: 100%;
    max-width: 998px;
    box-sizing: border-box;
    padding: 6px 16px calc(6px + env(safe-area-inset-bottom, 0px));
    background: #fff;
    border-top: 1px solid #f0f0f0;
    box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.04);
  }

  // 评论抽屉内部：标题行 + 输入框 + 右下角发送
  &__comment {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  &__comment-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  &__comment-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    line-height: 24px;
    color: #1d2129;
  }

  &__comment-close {
    color: #86909c;
    font-size: 16px;
    cursor: pointer;
  }

  // 输入框作为定位参照，发送按钮叠在右下角；底部留出按钮的高度，正文不会压到按钮
  &__comment-editor {
    position: relative;
  }

  &__comment-input {
    padding: 10px 12px 48px;
    border-color: #e5e6eb;
    border-radius: 10px;
    font-size: 14px;
    line-height: 22px;
  }

  // 有内容时浅蓝点亮，空内容时整颗灰掉
  &__comment-send {
    position: absolute;
    right: 8px;
    bottom: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    background: #3157e2;
    border-color: #3157e2;
    transition: all cubic-bezier(0.075, 0.82, 0.165, 1) 0.2s;

    &:not([disabled]):hover,
    &:not([disabled]):focus {
      opacity: 0.9;
    }

    &:disabled {
      background: #f2f3f5;
      border-color: #f2f3f5;
      color: #b8bfc7;
    }
  }

  &__bar-actions {
    display: flex;
    align-items: center;
    justify-content: space-evenly;
  }

  &__bar-btn {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    padding: 2px 12px;
    background: transparent;
    border: 0;
    cursor: pointer;

    &:disabled {
      cursor: not-allowed;
      opacity: 0.5;
    }
  }

  // 三个图标共用同一个 20px 盒子，保证占位一致；再按各自字形微调字号：
  // 同字号下气泡偏大、回退箭头偏小、闪电又细又小，光学大小对不齐
  &__bar-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    font-size: 20px;
    line-height: 1;
    color: #86909c;
  }

  &__bar-icon--message {
    font-size: 18px;
  }

  &__bar-icon--recall {
    font-size: 22px;
  }

  &__bar-icon--urge {
    font-size: 21px;
  }

  &__bar-label {
    font-size: 13px;
    line-height: 20px;
    color: #1d2129;
    white-space: nowrap;
  }
}

@media (max-width: 959px) {
  // The drawer is portaled outside the page; prevent iOS input-focus zoom here too.
  .issue-history-detail__comment-input {
    font-size: 16px;
  }
}
</style>

<style lang="less">
// 抽屉本体由 a-drawer 渲染并 teleport 到 body，scoped 选不到，圆角和留白只能写在顶层
.issue-recall-drawer {
  .ant-drawer-content {
    border-radius: 16px 16px 0 0;
  }

  .ant-drawer-body {
    padding: 20px 16px calc(20px + env(safe-area-inset-bottom));
  }
}

// 评论抽屉与撤回抽屉同一套圆角，内边距略小
.issue-comment-drawer {
  .ant-drawer-content {
    border-radius: 16px 16px 0 0;
  }

  .ant-drawer-body {
    padding: 16px 16px calc(16px + env(safe-area-inset-bottom));
  }
}
</style>
