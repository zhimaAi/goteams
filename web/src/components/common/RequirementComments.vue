<template>
  <section
    class="comment-section"
    :class="{
      'has-comment-input': canEdit && !descriptionEditing,
      'comment-input-expanded':
        canEdit && !descriptionEditing && commentInputFocused,
    }"
  >
    <h4>{{ $t("requirement_commented") }} ({{ comments.length }})</h4>
    <a-tabs v-if="inviteFiller" v-model:active-key="commentTab" size="small">
      <a-tab-pane
        key="internal"
        :disabled="commentSubmitting || replySubmitting || commentSaving"
        :tab="`${$t('requirement_internalReplies')} (${internalCommentCount})`"
      />
      <a-tab-pane
        key="public"
        :disabled="commentSubmitting || replySubmitting || commentSaving"
        :tab="`${$t('requirement_publicReplies')} (${publicCommentCount})`"
      />
    </a-tabs>
    <a-empty
      v-if="!visibleComments.length"
      :image="simpleImage"
      :description="$t('requirement_noComments')"
    />
    <template
      v-for="{ comment: item, replyTo } in commentRenderList"
      :key="item.id"
    >
      <div
        class="comment-item"
        :class="{
          'comment-item--reply': Number(item.parent_id) > 0 && replyTo,
        }"
      >
        <a-avatar
          v-if="isInviteSubmitter(item)"
          size="small"
          class="comment-avatar--invite"
        >
          {{ $t("issue_invite_submitter_avatar") }}
        </a-avatar>
        <a-avatar v-else size="small" :src="commentAvatarUrl(item)">
          <span>{{ (item.user_name || "?")[0] }}</span>
        </a-avatar>
        <div class="comment-body">
          <div class="comment-meta">
            <strong v-if="replyTo || (isPublic && !isInviteSubmitter(item))">{{
              $t("requirement_commentReplyTitle", {
                actor: authorLabel(item),
                target: isPublic ? $t("issue_invite_filler") : replyTo,
              })
            }}</strong>
            <strong v-else>{{ authorLabel(item) }}</strong>
            <span class="muted">{{
              formatLocalDate(item.created_at, true)
            }}</span>
            <div v-if="canEdit" class="comment-operations">
              <a-button
                type="text"
                size="small"
                class="comment-operation"
                :title="$t('requirement_commentReply')"
                @click="startReply(item)"
              >
                <MessageOutlined />
              </a-button>
              <template v-if="canModifyComment(item)">
                <a-button
                  type="text"
                  size="small"
                  class="comment-operation"
                  :title="$t('common_edit')"
                  @click="startCommentEdit(item)"
                >
                  <EditOutlined />
                </a-button>
                <a-button
                  type="text"
                  size="small"
                  class="comment-operation"
                  :title="$t('common_delete')"
                  @click="deleteDetailComment(item)"
                >
                  <DeleteOutlined />
                </a-button>
              </template>
            </div>
          </div>
          <div v-if="editingCommentId === Number(item.id)" class="comment-edit">
            <RichTextEditor
              v-model="commentEditDraft"
              v-model:image-file-ids="commentEditImageFileIds"
              class="comment-edit-editor"
              :ws-id="wsId"
              content-mode="rich-text"
              plain
              :min-height="96"
              :mention-enabled="true"
              :mention-members="mentionCandidates"
            />
            <div class="comment-edit-actions">
              <span class="comment-mention-hint">{{
                $t("requirement_commentMentionHint")
              }}</span>
              <a-space>
                <a-button size="small" @click="cancelCommentEdit">{{
                  $t("common_cancel")
                }}</a-button>
                <a-button
                  type="primary"
                  size="small"
                  :disabled="!hasCommentContent(commentEditDraft)"
                  :loading="commentSaving"
                  @click="saveCommentEdit(item)"
                >
                  {{ $t("common_save") }}
                </a-button>
              </a-space>
            </div>
          </div>
          <template v-else>
            <a
              v-if="commentUrl(item.content)"
              :href="commentUrl(item.content)"
              target="_blank"
              rel="noopener noreferrer"
              >{{ String(item.content).trim() }}</a
            >
            <RichTextEditor
              v-else
              class="comment-content-viewer"
              :model-value="item.content"
              content-mode="rich-text"
              read-only
              plain
              :min-height="0"
            />
          </template>
          <div
            v-if="commentAttachments(item.id, item.content).length"
            class="comment-attachments"
          >
            <template
              v-for="file in commentAttachments(item.id, item.content)"
              :key="file.id"
            >
              <a-image
                v-if="isImageAttachment(file) && file.url"
                class="comment-image-thumbnail"
                :width="96"
                :height="72"
                :src="file.url"
                :alt="file.original_name"
                :preview="false"
                @click="openAttachmentPreview(file)"
              />
              <video
                v-else-if="isVideoAttachment(file) && file.url"
                class="comment-video"
                :src="file.url"
                :aria-label="file.original_name"
                controls
                preload="metadata"
              />
              <button
                v-else
                type="button"
                class="comment-attachment-chip"
                @click="openAttachment(file)"
              >
                <PaperClipOutlined />
                <span>{{ file.original_name }}</span>
                <small v-if="file.file_size">{{
                  formatFileSize(file.file_size)
                }}</small>
              </button>
            </template>
          </div>
        </div>
      </div>
      <div v-if="replyTargetId === Number(item.id)" class="comment-reply-box">
        <div class="comment-reply-box__title">
          {{ $t("requirement_commentReplyTo", { name: authorLabel(item) }) }}
        </div>
        <CommentRichTextInput
          v-model="replyDraft"
          v-model:image-file-ids="replyImageFileIds"
          class="comment-editor"
          :ws-id="wsId"
          :min-height="72"
          always-expanded
          full-border
          :placeholder="$t('requirement_addCommentPlaceholder')"
          :mention-enabled="true"
          :mention-members="mentionCandidates"
          @submit="submitReply"
        />
        <div class="comment-reply-box__actions">
          <span class="comment-mention-hint">{{
            $t("requirement_commentMentionHint")
          }}</span>
          <a-button
            size="small"
            :disabled="replySubmitting"
            @click="cancelReply"
          >
            {{ $t("common_cancel") }}
          </a-button>
          <a-button
            type="primary"
            size="small"
            :disabled="!hasCommentContent(replyDraft)"
            :loading="replySubmitting"
            @click="submitReply"
          >
            {{ $t("common_confirm") }}
          </a-button>
        </div>
      </div>
    </template>
    <div
      v-if="canEdit && !descriptionEditing"
      class="comment-input"
      :class="{ expanded: commentInputFocused }"
      :style="inputStyle"
    >
      <p v-if="inviteFiller" class="comment-visibility-hint">
        {{
          $t(
            isPublic
              ? "requirement_publicReplyHint"
              : "requirement_internalReplyHint",
          )
        }}
      </p>
      <CommentRichTextInput
        :key="commentTab"
        v-model="commentText"
        v-model:image-file-ids="commentImageFileIds"
        class="comment-editor"
        :ws-id="wsId"
        :expanded="commentInputFocused"
        :min-height="commentInputFocused ? 108 : 30"
        :placeholder="$t('requirement_addCommentPlaceholder')"
        :mention-enabled="true"
        :mention-members="mentionCandidates"
        @focus-change="commentInputFocused = $event"
        @submit="handleCommentSubmit"
      />
      <div class="comment-actions">
        <div class="comment-action-left">
          <span class="comment-mention-hint">{{
            $t("requirement_commentMentionHint")
          }}</span>
          <div v-if="pendingCommentFiles.length" class="pending-files">
            <span
              v-for="(file, index) in pendingCommentFiles"
              :key="`${file.name}-${file.size}-${index}`"
            >
              <PaperClipOutlined /> {{ file.name }}
              <button
                type="button"
                class="pending-remove"
                :aria-label="$t('common_remove')"
                @click="pendingCommentFiles.splice(index, 1)"
              >
                <DeleteOutlined />
              </button>
            </span>
          </div>
          <a-upload
            :show-upload-list="false"
            :before-upload="stageCommentAttachment"
            multiple
          >
            <a-button type="text" size="small">
              <PaperClipOutlined />
              {{ $t("requirement_add_attachment") }}
            </a-button>
          </a-upload>
        </div>
        <div class="comment-submit-actions">
          <a-button
            type="primary"
            class="publish-button"
            :disabled="!canPublishComment"
            :loading="commentSubmitting"
            @click="addDetailComment"
          >
            {{
              $t(isPublic ? "requirement_publishPublicReply" : "common_confirm")
            }}
          </a-button>
          <a-button
            class="cancel-button"
            :disabled="commentSubmitting"
            @click="resetCommentDraft"
          >
            {{ $t("common_cancel") }}
          </a-button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, reactive, ref, watch, onBeforeUnmount } from "vue";
import { Empty, message, Modal } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import {
  DeleteOutlined,
  EditOutlined,
  MessageOutlined,
  PaperClipOutlined,
} from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import { formatLocalDate } from "@/utils/date";
import { closeImageViewer, openImageViewer } from "@/utils/imageViewer";
import CommentRichTextInput from "@/components/common/CommentRichTextInput.vue";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import {
  createComment,
  deleteComment,
  deleteRequirementAttachment,
  listComments,
  updateComment,
  uploadRequirementAttachments,
} from "@/api/workspace";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
  requirementId: { type: [String, Number], default: "" },
  canEdit: { type: Boolean, default: false },
  descriptionEditing: { type: Boolean, default: false },
  inputStyle: { type: Object, default: () => ({}) },
  attachments: { type: Array, default: () => [] },
  // @人 候选项目成员，每项 { user_id, user_name, user_username, user_status, removed }。
  mentionMembers: { type: Array, default: () => [] },
  // 当前工单是否来自邀填工单；是则在 @ 候选里追加一个「邀填人」。
  inviteFiller: { type: Boolean, default: false },
});
const emit = defineEmits(["refresh-attachments"]);
const { t } = useI18n();
const authStore = useAuthStore();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const comments = ref([]);
const commentTab = ref("internal");
const isPublic = computed(
  () => props.inviteFiller && commentTab.value === "public",
);
// Each scope owns its drafts so switching tabs never publishes internal text publicly.
function emptyDraft() {
  return {
    text: "",
    imageFileIds: [],
    files: [],
    editingId: 0,
    editText: "",
    editImageFileIds: [],
    replyId: 0,
    replyText: "",
    replyImageFileIds: [],
  };
}
const drafts = reactive({ internal: emptyDraft(), public: emptyDraft() });
function draftField(field) {
  return computed({
    get: () => drafts[commentTab.value][field],
    set: (value) => {
      drafts[commentTab.value][field] = value;
    },
  });
}
const commentText = draftField("text");
const editingCommentId = draftField("editingId");
const commentEditDraft = draftField("editText");
const commentEditImageFileIds = draftField("editImageFileIds");
const commentImageFileIds = draftField("imageFileIds");
const pendingCommentFiles = draftField("files");
const replyTargetId = draftField("replyId");
const replyDraft = draftField("replyText");
const replyImageFileIds = draftField("replyImageFileIds");
const commentInputFocused = ref(false);
const commentSubmitting = ref(false);
const replySubmitting = ref(false);
const commentSaving = ref(false);
const publicCommentIds = computed(() => {
  const inviteeCommentIds = new Set(
    comments.value.filter(isInviteSubmitter).map((item) => Number(item.id)),
  );
  // Old responses omit visibility; retain the same public history as the migration.
  // An explicit flag always wins so internal mentions never become public.
  return new Set(
    comments.value
      .filter((item) =>
        typeof item.is_public === "boolean"
          ? item.is_public
          : isInviteSubmitter(item) ||
            inviteeCommentIds.has(Number(item.parent_id)) ||
            Number(item.at_invite) === 1,
      )
      .map((item) => Number(item.id)),
  );
});
const publicCommentCount = computed(() => publicCommentIds.value.size);
const internalCommentCount = computed(
  () => comments.value.length - publicCommentCount.value,
);
const visibleComments = computed(() =>
  props.inviteFiller
    ? comments.value.filter(
        (item) =>
          publicCommentIds.value.has(Number(item.id)) === isPublic.value,
      )
    : comments.value,
);
watch(commentTab, () => {
  commentInputFocused.value = false;
});
const commentAvatarUrls = ref({});
let commentRequestSequence = 0;

// @人 候选 = 项目成员 + （工单有邀填人时的）邀填人。
const mentionCandidates = computed(() => {
  // workspace_metadata 下发的成员字段是 user_name / user_username，不是 name / username；
  // 已禁用、已移出项目的成员不可 @（与负责人下拉 assignableMembers 口径一致）；
  // 自己也不进候选 —— @ 自己没意义，后端 NotificationMentionRecipients 同样跳过 actor。
  const selfId = authStore.userInfo?.id;
  const members = (props.mentionMembers || [])
    .filter(
      (member) =>
        String(member.user_status) !== "0" &&
        !member.removed &&
        (selfId == null || String(member.user_id) !== String(selfId)),
    )
    .map((member) => ({
      user_id: Number(member.user_id),
      name: member.user_name || member.user_username || "",
      username: member.user_username || "",
      invite: false,
    }));
  if (!props.inviteFiller) return members;
  return [
    ...members,
    {
      user_id: 0,
      name: t("issue_invite_filler"),
      username: "",
      invite: true,
    },
  ];
});

// 后端只按纯文本匹配 @name，不解析「邀填人」这个多语言固定词；
// 因此@邀填人的标签额外带 data-invite 标记，提交时据此置 at_invite。
function mentionsInvite(content) {
  return /data-invite="true"/.test(String(content || ""));
}

// 回复按时间线平铺：顶层评论按 id 倒序，每条顶层评论下平铺它的全部回复
// （含「回复的回复」，向上归到同一个顶层评论），回复同样按 id 倒序。
const commentRenderList = computed(() => {
  const byId = new Map();
  for (const item of visibleComments.value) byId.set(Number(item.id), item);
  const rootOf = (item) => {
    let current = item;
    const guard = new Set();
    while (
      Number(current.parent_id) > 0 &&
      byId.has(Number(current.parent_id)) &&
      !guard.has(Number(current.id))
    ) {
      guard.add(Number(current.id));
      current = byId.get(Number(current.parent_id));
    }
    return current;
  };
  const threads = new Map();
  for (const item of visibleComments.value) {
    const root = rootOf(item);
    const key = Number(root.id);
    if (!threads.has(key)) threads.set(key, { root, replies: [] });
    if (key !== Number(item.id)) threads.get(key).replies.push(item);
  }
  const list = [];
  const ordered = [...threads.values()].sort(
    (a, b) => Number(b.root.id) - Number(a.root.id),
  );
  for (const thread of ordered) {
    list.push({ comment: thread.root, replyTo: "" });
    for (const reply of thread.replies) {
      const parent = byId.get(Number(reply.parent_id)) || thread.root;
      list.push({ comment: reply, replyTo: authorLabel(parent) });
    }
  }
  return list;
});

const canPublishComment = computed(
  () =>
    !commentSubmitting.value &&
    (hasCommentContent(commentText.value) ||
      pendingCommentFiles.value.length > 0),
);
const attachmentPreviewItems = computed(() =>
  props.attachments
    .filter((file) => isImageAttachment(file) && file.url)
    .filter((file) => Number(file.comment_id) > 0)
    .filter(
      (file) =>
        !comments.value.some((comment) =>
          isAttachmentEmbeddedInContent(file, comment.content),
        ),
    )
    .map((file) => ({
      id: file.id,
      name: file.original_name,
      src: file.url,
    })),
);

function commentAttachments(commentId, content) {
  return props.attachments.filter(
    (item) =>
      Number(item.comment_id) === Number(commentId) &&
      !isAttachmentEmbeddedInContent(item, content),
  );
}
function normalizeMediaUrl(value) {
  const raw = String(value || "").trim();
  if (!raw) return "";
  try {
    return new URL(raw, window.location.origin).href;
  } catch {
    return raw;
  }
}
function isAttachmentEmbeddedInContent(file, content) {
  const fileUrl = normalizeMediaUrl(file.url || file.stored_path);
  if (!fileUrl || !content || typeof document === "undefined") return false;
  const container = document.createElement("div");
  container.innerHTML = String(content);
  const embeddedUrls = [
    ...container.querySelectorAll(
      "img[src], video[src], source[src], iframe[src], [data-editor-file], [data-editor-video]",
    ),
  ].flatMap((node) => [
    node.getAttribute("src"),
    node.getAttribute("data-file-href"),
    node.getAttribute("data-video-src"),
    node.getAttribute("data-video-original-url"),
    node.querySelector("a[href]")?.getAttribute("href"),
  ]);
  return embeddedUrls.some((url) => normalizeMediaUrl(url) === fileUrl);
}
function isImageAttachment(file) {
  if (
    String(file.content_type || "")
      .toLowerCase()
      .startsWith("image/")
  )
    return true;
  return /\.(avif|bmp|gif|jpe?g|png|svg|webp)$/i.test(file.original_name || "");
}
function isVideoAttachment(file) {
  const contentType = String(file.content_type || "").toLowerCase();
  if (["video/mp4", "video/webm"].includes(contentType)) return true;
  return /\.(mp4|webm)$/i.test(file.original_name || "");
}

function formatFileSize(size) {
  const bytes = Number(size) || 0;
  if (!bytes) return "";
  if (bytes < 1024 * 1024) return `${Math.ceil(bytes / 1024)}KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)}MB`;
}
function commentUrl(content) {
  const value = String(content || "").trim();
  if (!value || /\s/.test(value)) return "";
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:"
      ? url.href
      : "";
  } catch {
    return "";
  }
}
function hasCommentContent(content) {
  const value = String(content || "").trim();
  if (!value) return false;
  if (
    /<(?:img|video|iframe|table|hr)\b|data-editor-(?:file|video|code-block)=/i.test(
      value,
    )
  )
    return true;
  const container = document.createElement("div");
  container.innerHTML = value;
  return Boolean(container.textContent?.replace(/\u00a0/g, " ").trim());
}
function isOwnComment(item) {
  const currentUserId = authStore.userInfo?.id;
  return (
    currentUserId != null &&
    item.user_id != null &&
    String(item.user_id) === String(currentUserId)
  );
}
function canModifyComment(item) {
  return props.canEdit && isOwnComment(item);
}
function isInviteSubmitter(item) {
  return item.source === "issue_invite";
}
// 评论作者显示名：邀填人评论没有账号，统一显示「邀填人」。
function authorLabel(item) {
  if (!item) return "";
  return isInviteSubmitter(item) ? t("issue_invite_filler") : item.user_name;
}
function commentAvatarUrl(item) {
  if (isOwnComment(item)) return authStore.avatarUrl || undefined;
  return commentAvatarUrls.value[item.user_id] || undefined;
}
function clearCommentAvatarUrls() {
  Object.values(commentAvatarUrls.value).forEach((url) =>
    URL.revokeObjectURL(url),
  );
  commentAvatarUrls.value = {};
}
async function loadCommentAvatars(items) {
  clearCommentAvatarUrls();
  const urlMap = {};
  for (const item of items) {
    if (item.user_avatar && !isOwnComment(item)) {
      urlMap[Number(item.user_id)] = item.user_avatar;
    }
  }
  if (
    items.some(isOwnComment) &&
    authStore.userInfo?.avatar &&
    !authStore.avatarUrl
  ) {
    void authStore.loadAvatar().catch(() => {});
  }
  // The backend already returns a directly accessible public URL; use it directly without another request.
  commentAvatarUrls.value = { ...commentAvatarUrls.value, ...urlMap };
}
async function loadComments() {
  const requestSequence = ++commentRequestSequence;
  if (!props.open || !props.wsId || !props.requirementId) {
    comments.value = [];
    clearCommentAvatarUrls();
    return;
  }
  try {
    const items = await listComments(props.wsId, props.requirementId);
    if (requestSequence !== commentRequestSequence) return;
    comments.value = items || [];
    void loadCommentAvatars(comments.value);
  } catch {
    if (requestSequence === commentRequestSequence) comments.value = [];
  }
}
watch(
  () => [props.open, props.wsId, props.requirementId],
  () => {
    drafts.internal = emptyDraft();
    drafts.public = emptyDraft();
    commentTab.value = "internal";
    commentInputFocused.value = false;
    // 切换到其它需求时关闭仍在展示的预览，避免残留上一个需求的图片。
    closeImageViewer();
    void loadComments();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  commentRequestSequence += 1;
  clearCommentAvatarUrls();
});

async function addDetailComment() {
  if (!canPublishComment.value) return;
  const content = commentText.value.trim();
  const files = [...pendingCommentFiles.value];
  const uploadedAttachments = [];
  commentSubmitting.value = true;
  try {
    for (const file of files) {
      const items = await uploadRequirementAttachments(
        props.wsId,
        props.requirementId,
        [file],
      );
      uploadedAttachments.push(...(items || []));
    }
    await createComment(props.wsId, props.requirementId, {
      content,
      is_public: isPublic.value,
      attachment_ids: uploadedAttachments.map((item) => Number(item.id)),
      image_file_ids: commentImageFileIds.value,
      at_invite: mentionsInvite(content),
    });
    resetCommentDraft();
  } catch {
    for (const item of uploadedAttachments) {
      try {
        await deleteRequirementAttachment(
          props.wsId,
          props.requirementId,
          item.id,
        );
      } catch (cleanupError) {
        console.error(
          "Failed to remove an unbound comment attachment",
          cleanupError,
        );
      }
    }
  } finally {
    commentSubmitting.value = false;
    await loadComments();
    emit("refresh-attachments");
  }
}
function startCommentEdit(item) {
  editingCommentId.value = Number(item.id);
  commentEditDraft.value = item.content;
  commentEditImageFileIds.value = [];
}
function cancelCommentEdit() {
  editingCommentId.value = 0;
  commentEditDraft.value = "";
  commentEditImageFileIds.value = [];
}
async function saveCommentEdit(item) {
  const content = commentEditDraft.value.trim();
  if (!hasCommentContent(commentEditDraft.value) || commentSaving.value) return;
  commentSaving.value = true;
  try {
    if (content !== item.content || commentEditImageFileIds.value.length) {
      await updateComment(props.wsId, props.requirementId, item.id, {
        content,
        image_file_ids: commentEditImageFileIds.value,
      });
      await loadComments();
    }
    cancelCommentEdit();
  } catch {
    // Retain the draft on failure, as in comment creation.
  } finally {
    commentSaving.value = false;
  }
}
function resetCommentDraft() {
  commentText.value = "";
  commentImageFileIds.value = [];
  commentInputFocused.value = false;
  pendingCommentFiles.value = [];
}
function handleCommentSubmit() {
  if (canPublishComment.value) void addDetailComment();
}
function startReply(item) {
  replyTargetId.value = Number(item.id);
  replyDraft.value = "";
  replyImageFileIds.value = [];
}
function cancelReply() {
  replyTargetId.value = 0;
  replyDraft.value = "";
  replyImageFileIds.value = [];
}
// Visibility is explicit; mentioning the submitter never publishes an internal reply.
async function submitReply() {
  const targetId = replyTargetId.value;
  const content = replyDraft.value.trim();
  if (
    !targetId ||
    !hasCommentContent(replyDraft.value) ||
    replySubmitting.value
  )
    return;
  replySubmitting.value = true;
  try {
    await createComment(props.wsId, props.requirementId, {
      content,
      parent_id: targetId,
      is_public: isPublic.value,
      image_file_ids: replyImageFileIds.value,
      at_invite: mentionsInvite(content),
    });
    cancelReply();
    await loadComments();
    emit("refresh-attachments");
  } catch {
    // 保留草稿，便于用户重试。
  } finally {
    replySubmitting.value = false;
  }
}
function deleteDetailComment(item) {
  Modal.confirm({
    title: t("requirement_commentDeleteConfirm"),
    cancelText: t("common_cancel"),
    okType: "danger",
    onOk: async () => {
      await deleteComment(props.wsId, props.requirementId, item.id);
      await loadComments();
      emit("refresh-attachments");
    },
  });
}
function stageCommentAttachment(file) {
  if (file.size > 200 * 1024 * 1024) {
    message.error(t("requirement_attachmentTooLarge"));
    return false;
  }
  pendingCommentFiles.value.push(file);
  return false;
}
function openAttachmentPreview(file) {
  const current = attachmentPreviewItems.value.findIndex(
    (item) => Number(item.id) === Number(file.id),
  );
  if (current < 0) return;
  openImageViewer({
    sources: attachmentPreviewItems.value.map((item) => item.src),
    index: current,
  });
}
async function openAttachment(file) {
  if (isImageAttachment(file)) {
    openAttachmentPreview(file);
    return;
  }
  if (!file.url) return;
  const link = document.createElement("a");
  link.href = file.url;
  link.download = file.original_name;
  link.click();
}
</script>

<style scoped lang="less">
.comment-visibility-hint {
  margin: 0 0 8px;
  color: var(--ant-color-text-secondary, #8c8c8c);
  font-size: 12px;
}

.comment-attachments {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.comment-image-thumbnail {
  flex: 0 0 auto;
  overflow: hidden;
  border: 1px solid #dfe4ec;
  border-radius: 6px;
  background: #f5f7fa;
  cursor: pointer;
}
.comment-image-thumbnail :deep(.ant-image-img) {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.comment-video {
  width: min(100%, 360px);
  max-height: 240px;
  border-radius: 6px;
  background: #000;
}
.comment-attachment-chip {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  height: 32px;
  gap: 6px;
  padding: 0 10px;
  border: 1px solid #dfe4ec;
  border-radius: 6px;
  background: #fff;
  color: #657188;
  cursor: pointer;
  font-size: 12px;
}
.comment-attachment-chip span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.comment-attachment-chip small {
  color: #a5adbb;
}
.comment-section {
  margin-top: 28px;
  padding: 20px 0;
  border-top: 1px solid #f0f0f0;
  padding-left: 24px;
}
.comment-section.has-comment-input {
  padding-bottom: 64px;
}
.comment-section.comment-input-expanded {
  padding-bottom: 224px;
}
.comment-section h4 {
  margin-bottom: 16px;
  color: #262626;
  font-size: 14px;
  font-weight: 600;
}
.comment-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 14px;
}
// 回复与顶层评论同一条时间线，只做缩进区分。
.comment-item--reply {
  margin-left: 34px;
}
.comment-reply-box {
  margin: 0 0 14px 34px;
  padding: 10px 12px;
  border: 1px solid #e8ecf2;
  border-radius: 8px;
  background: #fafbfc;
}
.comment-reply-box__title {
  margin-bottom: 8px;
  color: #262626;
  font-size: 13px;
  font-weight: 500;
}
.comment-reply-box__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
// 评论框、回复框、编辑框底部统一在最左侧提示 @ 能力（需求 1729 设计稿口径）。
// 提示是 nowrap 文本，必须自带 min-width: 0 + 省略号，否则它会把所在 flex 容器
// 的最小尺寸撑到文本宽度，而编辑框那一行的容器是 grid item（min-width: auto 取 min-content），
// 会把评论卡片顶出去。
.comment-mention-hint {
  flex: 0 1 auto;
  min-width: 0;
  overflow: hidden;
  color: #8c9ab1;
  font-size: 12px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
// 这两行的按钮靠右，提示用 auto 外边距顶到最左侧；评论框那行的左侧容器本身就是 flex:1，
// 不能再加 auto，否则会把「添加附件」推到确认按钮旁边。
.comment-reply-box__actions .comment-mention-hint,
.comment-edit-actions .comment-mention-hint {
  margin-right: auto;
}
.comment-edit-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.comment-item :deep(.ant-avatar) {
  flex: 0 0 auto;
  margin-top: 2px;
  background: #1677ff !important;
}
.comment-body {
  flex: 1;
  min-width: 0;
  min-height: 62px;
  padding: 10px 12px;
  border: 1px solid #e8ecf2;
  border-radius: 8px;
  box-shadow: 0 3px 8px rgba(23, 35, 60, 0.03);
  color: #595959;
  font-size: 13px;
  word-break: break-word;
}
.comment-body > a {
  overflow-wrap: anywhere;
}
.comment-content-viewer :deep(.editor-shell) {
  overflow: visible;
  border: 0;
  border-radius: 0;
}
.comment-content-viewer :deep(.editor-body) {
  overflow: visible;
}
.comment-content-viewer :deep(.rich-text-content) {
  min-height: 0;
  padding: 0;
  color: #595959;
  font-size: 13px;
}
.comment-meta {
  display: flex;
  align-items: center;
  min-height: 22px;
  gap: 8px;
  margin-bottom: 4px;
  color: #262626;
}
.comment-meta .muted {
  color: #8c8c8c;
}
.comment-operations {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 2px;
  margin-left: auto;
}
.comment-operation {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 24px;
  width: 24px;
  min-width: 24px;
  height: 24px;
  padding: 0;
  border-radius: 4px;
  color: #8c9ab1;
}
.comment-edit {
  display: grid;
  // grid 的隐式 auto 轨道以子项 min-content 为下限，而编辑器工具栏是 nowrap 的 flex 容器，
  // 其 min-content 等于全部工具项宽度之和（≈670px），会把轨道撑出评论卡片。
  // 显式把轨道下限压到 0，并允许子项收缩，宽度就交给卡片决定。
  grid-template-columns: minmax(0, 1fr);
  min-width: 0;
  gap: 8px;

  .ant-space {
    justify-content: flex-end;
  }
}
.comment-edit-editor {
  min-width: 0;
}
.comment-edit-editor :deep(.editor-shell) {
  border-radius: 6px;
}
.comment-edit-editor :deep(.editor-toolbar) {
  flex-wrap: wrap;
}
.comment-edit-editor :deep(.mode-switch-button),
.comment-edit-editor :deep(.fullscreen-button) {
  display: none;
}
.comment-edit-editor :deep(.rich-text-content) {
  padding: 10px 12px;
}
.comment-input {
  position: absolute;
  right: 292px;
  bottom: 0;
  left: 0;
  z-index: 4;
  display: block;
  justify-items: stretch;
  gap: 0;
  margin: 0;
  padding: 8px 22px;
  background: #fff;
  box-shadow: 0 -10px 12px rgba(23, 35, 60, 0.04);
}
.comment-actions {
  display: none;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 50px;
  padding: 5px 8px 5px 10px;
  border: 1px solid #dfe4ec;
  border-top-color: #f0f2f6;
  border-radius: 0 0 8px 8px;
}
.comment-input.expanded .comment-actions {
  display: flex;
  border-color: #2475fc;
  border-top-color: #f0f2f6;
}
.comment-actions :deep(.ant-btn-text) {
  color: #667389;
}
.comment-action-left {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  gap: 8px;
}
.comment-action-left :deep(.ant-upload-wrapper) {
  flex: 0 0 auto;
}
.pending-files {
  display: flex;
  flex: 0 1 auto;
  min-width: 0;
  gap: 6px;
  overflow-x: auto;
}
.pending-files > span {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  max-width: 180px;
  gap: 4px;
  overflow: hidden;
  color: #657188;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pending-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 20px;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #8c9ab1;
  cursor: pointer;
}
.comment-submit-actions {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  gap: 8px;
}
.cancel-button,
.publish-button {
  min-width: 64px;
  height: 32px;
  border-radius: 4px;
}
@media (max-width: 720px) {
  .comment-section.comment-input-expanded {
    padding-bottom: 242px;
  }
  .comment-input {
    position: fixed;
    right: 0 !important;
    bottom: 0;
    width: 100vw;
    margin: 0;
  }
  .comment-actions {
    align-items: flex-end;
  }
  .comment-action-left {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }
}
</style>
