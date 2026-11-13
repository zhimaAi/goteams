<template>
  <section class="notification-page">
    <header class="notification-header">
      <div class="notification-title">
        <BellOutlined />
        <h1>{{ $t("notification_title") }}</h1>
        <a-badge
          v-if="unreadCount"
          :count="unreadCount > 99 ? '99+' : unreadCount"
        />
      </div>
      <div class="notification-header-actions">
        <a-button
          v-if="unreadCount > 0"
          type="link"
          size="small"
          :loading="markingAll"
          @click="markAllRead"
        >
          {{ $t("notification_markAllRead") }}
        </a-button>
        <a-segmented
          v-model:value="filter"
          :options="filterOptions"
          @change="handleFilterChange"
        />
      </div>
    </header>

    <div class="notification-list" :class="{ loading }">
      <a-empty
        v-if="!loading && !items.length"
        :description="
          $t(
            filter === 'unread'
              ? 'notification_emptyUnread'
              : 'notification_empty',
          )
        "
      />
      <article
        v-for="item in items"
        :key="item.id"
        class="notification-row"
        :class="{ unread: !item.read }"
      >
        <div class="notification-actor">
          <a-avatar
            :size="40"
            :src="slaAvatar(item) || undefined"
            :style="{
              backgroundColor: slaAvatar(item) ? undefined : '#e8f1ff',
              color: '#3157e2',
            }"
            >{{
              isSlaNotification(item) ? "" : initials(item.actor_name)
            }}</a-avatar
          >
          <span :title="actorDisplayName(item)">{{
            actorDisplayName(item)
          }}</span>
        </div>
        <div class="notification-body">
          <div class="notification-content">
            <i v-if="!item.read" class="unread-dot" />{{ displayContent(item) }}
          </div>
          <div
            v-if="item.event_type === 'workspace_access_requested'"
            class="notification-reason"
          >
            {{ $t("notification_accessReason", { reason: item.content }) }}
          </div>
          <button
            class="notification-item-link"
            type="button"
            :title="
              projectNotification(item)
                ? $t(
                    item.event_type === 'workspace_access_requested'
                      ? 'notification_openMembers'
                      : 'notification_openWorkspace',
                    { name: item.workspace_name },
                  )
                : $t('notification_openItem', {
                    title: item.requirement_title,
                  })
            "
            @click="openItem(item)"
          >
            <WorkItemTypePill
              v-if="!projectNotification(item)"
              :type="item.requirement_type"
              :icon-text="item.requirement_type_icon_text"
              :icon-color="item.requirement_type_icon_color"
            />
            <a-tag
              v-if="isSlaNotification(item)"
              class="notification-priority"
              :color="item.priority_color || undefined"
              >{{ item.priority_name || $t("sla_unspecified") }}</a-tag
            >
            <span class="notification-item-title">{{
              projectNotification(item)
                ? item.workspace_name
                : item.requirement_title
            }}</span>
          </button>
          <div v-if="isSlaNotification(item)" class="notification-details">
            <span
              >{{ $t("view_field_creator") }}：{{
                item.creator_name || "—"
              }}</span
            >
            <span
              >{{ $t("sla_assignees") }}：{{ item.assignee_names || "—" }}</span
            >
            <span
              >{{ $t("notification_processingDuration") }}：{{
                processingDuration(item)
              }}</span
            >
          </div>
        </div>
        <div class="notification-meta">
          <time>{{ formatLocalDate(item.created_at, true) }}</time>
          <span>{{
            $t("notification_from", { source: item.workspace_name })
          }}</span>
        </div>
      </article>
    </div>
    <a-pagination
      v-if="total"
      v-model:current="page"
      v-model:page-size="size"
      :total="total"
      :show-size-changer="false"
      :hide-on-single-page="true"
      @change="load"
    />

    <RequirementDetailDrawer
      :open="!!detailRequirementId"
      :ws-id="detailItem?.workspace_id"
      :requirement-id="detailRequirementId"
      :mode="previewMode"
      :initial-tab="detailInitialTab"
      :navigation-items="notificationNavigationItems"
      @close="closeDetail"
      @navigate="handleDetailNavigate"
      @updated="handleDetailUpdated"
    />
  </section>
</template>

<script setup>
import { slaDuration } from "@/utils/sla";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { BellOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { listNotifications } from "@/api/notification";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import { useAuthStore } from "@/stores/auth";
import { useNotificationStore } from "@/stores/notifications";
import {
  GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
  readGlobalListPreferences,
} from "@/utils/boardPreferences";
import { formatLocalDate } from "@/utils/date";
import systemLogo from "@/image/logo.svg";
import {
  normalizeWorkItemNavigationItems,
  saveWorkItemNavigationContext,
} from "@/utils/workItemNavigation";

const { t: $t } = useI18n();
const router = useRouter();
const authStore = useAuthStore();
const notificationStore = useNotificationStore();
const storedGlobalPreferences = readGlobalListPreferences(
  authStore.userInfo?.id,
);
const previewMode = ref(
  ["drawer", "modal", "window"].includes(storedGlobalPreferences.previewMode)
    ? storedGlobalPreferences.previewMode
    : "drawer",
);
const items = ref([]);
const total = ref(0);
const page = ref(1);
const size = ref(20);
const filter = ref("all");
const loading = ref(false);
const markingAll = ref(false);
const unreadCount = ref(0);
const filterOptions = computed(() => [
  { label: `${$t("notification_all")} (${allTotal.value})`, value: "all" },
  {
    label: `${$t("notification_unread")} (${unreadCount.value})`,
    value: "unread",
  },
]);
const allTotal = ref(0);
const initialized = ref(false);
const recentlyRead = ref([]);
const autoReadIds = new Set();
const detailItem = ref(null);
const detailRequirementId = ref(null);
const detailInitialTab = ref("");
const notificationNavigationItems = computed(() =>
  normalizeWorkItemNavigationItems(
    items.value.map((item) => ({
      id: item.requirement_id,
      workspaceId: item.workspace_id,
    })),
  ),
);

function initials(name) {
  return String(name || "?")
    .trim()
    .slice(0, 2)
    .toLowerCase();
}
// SLA 超时/预警由系统触发；无真实操作人时以系统身份（logo + GoTeams）展示。
const SLA_EVENT_TYPES = new Set([
  "sla_overdue",
  "sla_warning",
  "work_item_sla",
]);
function isSlaNotification(item) {
  return SLA_EVENT_TYPES.has(item.event_type);
}
// SLA 通知是系统通知：头像用当前项目的 logo（未设置时用系统 logo），名称固定 GoTeams。
function slaAvatar(item) {
  return isSlaNotification(item)
    ? item.workspace_icon || systemLogo
    : item.actor_avatar || "";
}
function actorDisplayName(item) {
  if (isSlaNotification(item)) return "GoTeams";
  // 催单来自外部提交人，没有站内账号，固定以「邀填人」身份展示。
  if (item.event_type === "issue_invite_urged")
    return $t("notification_issueInviteActor");
  return item.actor_name || $t("notification_unknownActor");
}
// 后端按请求语言翻译状态名；对历史遗留的包装键仅做去壳兜底。
function slaStatusName(name) {
  if (!name) return "";
  return name.replace(/^\$\{/, "").replace(/\}$/, "");
}
function displayContent(item) {
  const actor = actorDisplayName(item);
  if (item.event_type === "workspace_access_requested")
    return $t("notification_accessRequested", {
      actor,
      name: item.workspace_name,
    });
  if (item.event_type === "workspace_member_added")
    return $t("notification_workspaceMemberAdded", {
      actor,
      name: item.workspace_name,
    });
  const itemType =
    item.requirement_type === "defect"
      ? $t("notification_defect")
      : $t("notification_requirement");
  if (item.event_type === "item_updated") {
    return (
      item.content ||
      $t("notification_itemUpdated", {
        actor,
        itemType,
        field: fieldLabel(item.field_name),
      })
    );
  }
  if (item.event_type === "item_assigned")
    return $t("notification_itemAssigned", { actor, itemType });
  if (item.event_type === "mention")
    return $t("notification_mention", { actor, itemType });
  if (item.event_type === "assignee_commented")
    return $t("notification_assigneeCommented", { actor, itemType });
  if (item.event_type === "issue_invite_urged")
    return $t("notification_issueInviteUrged");
  if (item.event_type === "work_item_sla") {
    let content = {};
    try {
      content = JSON.parse(item.content || "{}");
    } catch {}
    const overdue = Math.max(
      0,
      Number(content.elapsed_seconds) - Number(content.limit_seconds),
    );
    const remaining = Math.max(
      0,
      Number(content.limit_seconds) - Number(content.elapsed_seconds),
    );
    return $t(overdue > 0 ? "sla_noticeAfter" : "sla_noticeBefore", {
      stage: content.stage || "SLA",
      time: slaDuration(overdue > 0 ? overdue : remaining, $t),
    });
  }
  if (item.event_type === "sla_overdue" || item.event_type === "sla_warning") {
    let parsed = {};
    try {
      parsed = item.content ? JSON.parse(item.content) : {};
    } catch {
      parsed = {};
    }
    // A warning carries how many minutes were left when it was sent; records created before that
    // field existed fall back to the generic wording instead of printing an empty countdown.
    const minutes = Number(parsed.minutes) || 0;
    const base =
      item.event_type === "sla_overdue"
        ? $t("notification_slaOverdue")
        : minutes > 0
          ? $t("notification_slaWarningCountdown", { minutes })
          : $t("notification_slaWarning");
    // 状态来自通知内容；历史消息没有该字段时不拼状态后缀，避免出现空括号。
    const status = slaStatusName(parsed.status);
    return status
      ? `${base}${$t("notification_currentStatusSuffix", { status })}`
      : base;
  }
  return $t("notification_commentReplied", { actor });
}
function processingDuration(item) {
  try {
    const content = JSON.parse(item.content || "{}");
    return Number.isFinite(content.limit_seconds)
      ? slaDuration(content.limit_seconds, $t)
      : "—";
  } catch {
    return "—";
  }
}
function projectNotification(item) {
  return ["workspace_access_requested", "workspace_member_added"].includes(
    item.event_type,
  );
}
function fieldLabel(field) {
  const key = {
    status: "notification_fieldStatus",
    priority: "notification_fieldPriority",
    assignee: "notification_fieldAssignee",
    title: "notification_fieldTitle",
    description: "notification_fieldDescription",
    planned_end_date: "notification_fieldDueDate",
    planned_start_date: "notification_fieldStartDate",
    version_id: "notification_fieldIteration",
    parent_id: "notification_fieldParent",
    work_hours: "notification_fieldWorkHours",
  }[field];
  return key ? $t(key) : field || $t("notification_fieldOther");
}
async function load({ preserveUnread = false } = {}) {
  if (loading.value && preserveUnread) return;
  loading.value = true;
  try {
    const data = await listNotifications({
      page: page.value,
      size: size.value,
      unread_only: filter.value === "unread" ? 1 : 0,
    });
    const incoming = data?.list || [];
    const unreadIncoming = incoming.filter((item) => !item.read);
    unreadCount.value = Number(data?.unread_total || 0) + autoReadIds.size;
    let displayItems = incoming;
    if (filter.value === "all") {
      const snapshot = preserveUnread
        ? [...unreadIncoming, ...recentlyRead.value]
        : unreadIncoming;
      recentlyRead.value = [
        ...new Map(snapshot.map((item) => [item.id, item])).values(),
      ];
      if (preserveUnread) {
        const visibleUnreadIds = new Set(
          recentlyRead.value.map((item) => item.id),
        );
        displayItems = incoming.map((item) =>
          visibleUnreadIds.has(item.id) ? { ...item, read: false } : item,
        );
      }
    }
    if (filter.value === "unread" && preserveUnread) {
      const merged = new Map(items.value.map((item) => [item.id, item]));
      incoming.forEach((item) => merged.set(item.id, item));
      items.value = [...merged.values()].sort(
        (a, b) =>
          Number(b.created_at || 0) - Number(a.created_at || 0) ||
          Number(b.id || 0) - Number(a.id || 0),
      );
      total.value = items.value.length;
    } else {
      items.value = displayItems;
      total.value = Number(data?.total || 0);
    }
    allTotal.value = filter.value === "all" ? total.value : allTotal.value;
    notificationStore.unreadCount = Number(data?.unread_total || 0);
    const unreadIds = unreadIncoming.map((item) => item.id);
    if (unreadIds.length) {
      await notificationStore.markRead(unreadIds);
      unreadIds.forEach((id) => autoReadIds.add(id));
    }
  } finally {
    loading.value = false;
  }
}
async function markAllRead() {
  if (markingAll.value || unreadCount.value <= 0) return;
  markingAll.value = true;
  try {
    await notificationStore.markAllRead();
    unreadCount.value = 0;
    autoReadIds.clear();
    recentlyRead.value = [];
    if (filter.value === "unread") {
      items.value = [];
      total.value = 0;
    } else {
      items.value = items.value.map((item) => ({ ...item, read: true }));
    }
  } catch {
    // Network errors are handled centrally by the interceptor.
  } finally {
    markingAll.value = false;
  }
}
function handleFilterChange() {
  page.value = 1;
  if (filter.value === "unread" && recentlyRead.value.length) {
    items.value = recentlyRead.value;
    total.value = items.value.length;
    recentlyRead.value = [];
    void load({ preserveUnread: true });
    return;
  }
  autoReadIds.clear();
  recentlyRead.value = [];
  void load();
}
function openItem(item) {
  if (projectNotification(item)) {
    if (!item.workspace_id) return;
    router.push({
      name:
        item.event_type === "workspace_access_requested"
          ? "WorkspaceSettings"
          : "RequirementBoard",
      params: { wsId: item.workspace_id },
      query:
        item.event_type === "workspace_access_requested"
          ? { section: "members" }
          : {},
    });
    return;
  }
  if (!item.workspace_id || !item.requirement_id) return;
  if (previewMode.value === "window") {
    const token = saveWorkItemNavigationContext(
      notificationNavigationItems.value,
    );
    const query = token ? { navContext: token } : {};
    if (item.event_type === "work_item_sla") query.detailTab = "sla";
    const target = router.resolve({
      name: "WorkItemDetail",
      params: { wsId: item.workspace_id, id: item.requirement_id },
      query,
    });
    window.open(target.href, "_blank", "noopener,noreferrer");
    return;
  }
  detailInitialTab.value =
    item.event_type === "work_item_sla"
      ? "sla"
      : item.event_type === "sla_overdue" || item.event_type === "sla_warning"
        ? "aging"
        : "";
  detailItem.value = item;
  detailRequirementId.value = item.requirement_id;
}

function closeDetail() {
  detailItem.value = null;
  detailRequirementId.value = null;
  detailInitialTab.value = "";
}

function handleDetailNavigate(id, workspaceId) {
  const target = items.value.find(
    (item) =>
      Number(item.requirement_id) === Number(id) &&
      Number(item.workspace_id) === Number(workspaceId),
  );
  if (target) detailItem.value = target;
  detailRequirementId.value = id;
}

function handleDetailUpdated(event) {
  if (event?.action !== "update" || !event.item) return;
  const title = event.item.title;
  if (title === undefined) return;
  const targetRequirementId = Number(event.id);
  const targetWorkspaceId = Number(event.workspaceId);
  items.value = items.value.map((item) =>
    Number(item.requirement_id) === targetRequirementId &&
    Number(item.workspace_id) === targetWorkspaceId
      ? { ...item, requirement_title: title }
      : item,
  );
  recentlyRead.value = recentlyRead.value.map((item) =>
    Number(item.requirement_id) === targetRequirementId &&
    Number(item.workspace_id) === targetWorkspaceId
      ? { ...item, requirement_title: title }
      : item,
  );
}

function handleSharedListPreferencesChange(event) {
  if (String(event.detail?.userId) !== String(authStore.userInfo?.id)) return;
  const patch = event.detail?.patch || {};
  if (["drawer", "modal", "window"].includes(patch.previewMode)) {
    previewMode.value = patch.previewMode;
    if (patch.previewMode === "window") closeDetail();
  }
}
watch(
  () => notificationStore.unreadCount,
  (count, previous) => {
    if (!initialized.value || count <= previous) return;
    void load({ preserveUnread: true });
  },
);
onMounted(async () => {
  window.addEventListener(
    GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
    handleSharedListPreferencesChange,
  );
  notificationStore.startPolling();
  await load();
  initialized.value = true;
});

onBeforeUnmount(() => {
  window.removeEventListener(
    GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
    handleSharedListPreferencesChange,
  );
});
</script>

<style scoped lang="less">
.notification-page {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  background: var(--app-surface);
}
.notification-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 58px;
  padding: 0 20px;
  border-bottom: 1px solid var(--app-border);
}
.notification-title {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--app-text);
}
.notification-title h1 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}
.notification-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.notification-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.notification-row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr) minmax(200px, auto);
  gap: 20px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--app-border);
}
.notification-row.unread {
  background: #f7f9fc;
}
.notification-actor {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
}
.notification-actor span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.notification-body {
  min-width: 0;
}
.notification-content {
  display: flex;
  align-items: center;
  min-height: 22px;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
}
.notification-reason {
  margin: 3px 0;
  color: var(--app-text-secondary);
  font-size: 13px;
}
.unread-dot {
  width: 6px;
  height: 6px;
  margin-right: 7px;
  border-radius: 50%;
  background: #f5222d;
  flex: 0 0 auto;
}
.notification-item-link {
  display: flex;
  align-items: center;
  max-width: 100%;
  gap: 6px;
  padding: 3px 0 0;
  border: 0;
  background: transparent;
  color: #1677ff;
  cursor: pointer;
  text-align: left;
}
.notification-priority {
  flex-shrink: 0;
  margin-inline-end: 0;
  height: 16px;
  padding: 0 2px;
  border-radius: 2px;
  font-size: 10px;
  line-height: 14px;
}
.notification-details {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 0;
  margin-top: 6px;
  color: var(--app-text-secondary);
  font-size: 12px;
  > span + span {
    margin-left: 10px;
    padding-left: 10px;
    border-left: 1px solid var(--app-border);
  }
}
.notification-item-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 40px;
}
.notification-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: center;
  min-width: 200px;
  color: var(--app-text-secondary);
  font-size: 12px;
  line-height: 22px;
}
.notification-page :deep(.ant-pagination) {
  margin: 14px 20px;
  text-align: right;
}
@media (max-width: 760px) {
  .notification-row {
    grid-template-columns: 1fr;
    gap: 4px;
  }
  .notification-meta {
    align-items: flex-start;
    min-width: 0;
  }
}
</style>
