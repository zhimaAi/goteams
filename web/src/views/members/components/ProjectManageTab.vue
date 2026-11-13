<!--
 * ProjectManageTab.vue
 * Enterprise project management: search, drag sorting, end/reopen and permanent delete.
 -->
<template>
  <div class="project-manage-tab">
    <div class="page-header">
      <a-button v-if="canCreate" type="primary" @click="createOpen = true">
        <template #icon><PlusOutlined /></template>
        {{ $t("project_manage_create") }}
      </a-button>
      <a-input-search
        v-model:value="keyword"
        class="project-search"
        allow-clear
        :placeholder="$t('project_manage_search_placeholder')"
        @search="fetchProjects"
      />
    </div>

    <div class="project-table-card">
      <a-table
        :columns="columns"
        :data-source="projects"
        :loading="loading"
        :pagination="false"
        :custom-row="customRow"
        row-key="id"
        class="project-table"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'drag'">
            <a-tooltip :title="$t('project_manage_drag_tip')">
              <HolderOutlined class="drag-handle" />
            </a-tooltip>
          </template>
          <template v-else-if="column.key === 'status'">
            <a-tag :color="isEnded(record) ? 'default' : 'success'">
              {{
                isEnded(record)
                  ? $t("project_manage_status_ended")
                  : $t("project_manage_status_normal")
              }}
            </a-tag>
          </template>
          <template v-else-if="column.key === 'created_at'">
            {{ formatLocalDate(record.created_at, true) }}
          </template>
          <template v-else-if="column.key === 'admins'">
            {{
              record.admin_names?.length ? record.admin_names.join("、") : "-"
            }}
          </template>
          <template v-else-if="column.key === 'members'">
            <span class="member-count">
              <TeamOutlined />
              {{
                $t("project_manage_member_count", {
                  count: record.member_count,
                })
              }}
            </span>
          </template>
          <template v-else-if="column.key === 'actions'">
            <a-space :size="16">
              <a @click="openProject(record)">{{
                $t("project_manage_view")
              }}</a>
              <a @click="toggleEnded(record)">
                {{
                  isEnded(record)
                    ? $t("project_manage_reopen")
                    : $t("project_manage_end")
                }}
              </a>
              <a class="danger-link" @click="openDelete(record)">{{
                $t("project_manage_delete")
              }}</a>
            </a-space>
          </template>
        </template>
        <template #emptyText>{{ $t("project_manage_empty") }}</template>
      </a-table>
    </div>

    <CreateWorkspaceModal
      v-model:open="createOpen"
      :redirect="false"
      @created="handleCreated"
    />

    <a-modal
      v-model:open="deleteOpen"
      :title="$t('project_manage_delete_confirm_title')"
      :confirm-loading="deleting"
      :ok-button-props="{ danger: true, disabled: countdown > 0 }"
      :ok-text="
        countdown > 0
          ? $t('project_manage_delete_countdown', { seconds: countdown })
          : $t('common_confirm')
      "
      :cancel-text="$t('common_cancel')"
      @ok="confirmDelete"
    >
      <p class="delete-tip">{{ $t("project_manage_delete_confirm") }}</p>
      <p class="delete-target">{{ deleteTarget?.name }}</p>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { Modal, message } from "ant-design-vue";
import {
  HolderOutlined,
  PlusOutlined,
  TeamOutlined,
} from "@ant-design/icons-vue";
import {
  deleteWorkspace,
  listEnterpriseProjects,
  sortEnterpriseProjects,
  updateWorkspaceEnded,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { useWorkspaceStore } from "@/stores/workspace";
import { formatLocalDate } from "@/utils/date";
import { PROJECT_ORDER_CHANGED_EVENT } from "@/utils/projectOrder";
import CreateWorkspaceModal from "@/components/layout/CreateWorkspaceModal.vue";

const props = defineProps({
  active: { type: Boolean, default: false },
});

const { t } = useI18n();
const router = useRouter();
const authStore = useAuthStore();
const workspaceStore = useWorkspaceStore();

const projects = ref([]);
const loading = ref(false);
const keyword = ref("");
const createOpen = ref(false);
const deleteOpen = ref(false);
const deleteTarget = ref(null);
const deleting = ref(false);
const countdown = ref(0);
const dragIndex = ref(-1);
let countdownTimer = null;
let projectOrderRefreshPending = false;

const canCreate = computed(() =>
  authStore.hasEnterprisePermission("project:create"),
);

const columns = computed(() => [
  { key: "drag", width: 48 },
  { key: "name", dataIndex: "name", title: t("project_manage_col_name") },
  { key: "status", title: t("project_manage_col_status"), width: 120 },
  {
    key: "creator",
    dataIndex: "creator_name",
    title: t("project_manage_col_creator"),
    width: 140,
  },
  { key: "created_at", title: t("project_manage_col_created_at"), width: 180 },
  { key: "admins", title: t("project_manage_col_admins"), width: 200 },
  { key: "members", title: t("project_manage_col_members"), width: 140 },
  { key: "actions", title: t("project_manage_col_actions"), width: 180 },
]);

function isEnded(record) {
  return Number(record.ended_at) > 0;
}

async function fetchProjects() {
  loading.value = true;
  try {
    const list = await listEnterpriseProjects({ keyword: keyword.value });
    projects.value = Array.isArray(list) ? list : [];
  } finally {
    loading.value = false;
  }
}

function openProject(record) {
  const target = router.resolve(`/workspace/${record.id}/board`);
  window.open(target.href, "_blank");
}

function toggleEnded(record) {
  if (isEnded(record)) {
    applyEnded(record, false);
    return;
  }
  Modal.confirm({
    title: t("project_manage_end_confirm_title"),
    content: t("project_manage_end_confirm"),
    okText: t("common_confirm"),
    cancelText: t("common_cancel"),
    onOk: () => applyEnded(record, true),
  });
}

async function applyEnded(record, ended) {
  await updateWorkspaceEnded(record.id, { ended });
  message.success(
    ended
      ? t("project_manage_end_success")
      : t("project_manage_reopen_success"),
  );
  await refreshLists();
}

function openDelete(record) {
  deleteTarget.value = record;
  deleteOpen.value = true;
  // The confirm button unlocks after a 5 second countdown to avoid accidental permanent deletion.
  countdown.value = 5;
  stopCountdown();
  countdownTimer = setInterval(() => {
    countdown.value -= 1;
    if (countdown.value <= 0) stopCountdown();
  }, 1000);
}

async function confirmDelete() {
  if (countdown.value > 0 || !deleteTarget.value) return;
  deleting.value = true;
  try {
    await deleteWorkspace(deleteTarget.value.id);
    message.success(t("project_manage_delete_success"));
    deleteOpen.value = false;
    deleteTarget.value = null;
    await refreshLists();
  } finally {
    deleting.value = false;
  }
}

function stopCountdown() {
  if (!countdownTimer) return;
  clearInterval(countdownTimer);
  countdownTimer = null;
}

function customRow(record, index) {
  return {
    draggable: true,
    ondragstart: () => {
      dragIndex.value = index;
    },
    ondragover: (event) => event.preventDefault(),
    ondragend: () => {
      dragIndex.value = -1;
    },
    ondrop: () => handleDrop(index),
  };
}

async function handleDrop(targetIndex) {
  const sourceIndex = dragIndex.value;
  dragIndex.value = -1;
  if (sourceIndex < 0 || sourceIndex === targetIndex) return;
  const next = [...projects.value];
  const [moved] = next.splice(sourceIndex, 1);
  next.splice(targetIndex, 0, moved);
  projects.value = next;
  await sortEnterpriseProjects({ ids: next.map((item) => Number(item.id)) });
  message.success(t("project_manage_sort_success"));
  await workspaceStore.fetchWorkspaces(true);
}

async function handleCreated() {
  await refreshLists();
}

// The sidebar shares both the project order and the ended filter, so refresh it together with the table.
async function refreshLists() {
  await Promise.all([fetchProjects(), workspaceStore.fetchWorkspaces(true)]);
}

function handleProjectOrderChanged() {
  if (props.active) {
    fetchProjects();
    return;
  }
  projectOrderRefreshPending = true;
}

watch(
  () => props.active,
  (active) => {
    if (!active || !projectOrderRefreshPending) return;
    projectOrderRefreshPending = false;
    fetchProjects();
  },
);

onMounted(() => {
  window.addEventListener(
    PROJECT_ORDER_CHANGED_EVENT,
    handleProjectOrderChanged,
  );
  fetchProjects();
});
onUnmounted(() => {
  window.removeEventListener(
    PROJECT_ORDER_CHANGED_EVENT,
    handleProjectOrderChanged,
  );
  stopCountdown();
});
</script>

<style scoped lang="less">
.project-manage-tab {
  display: flex;
  min-height: 0;
  height: 100%;
  flex-direction: column;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.project-search {
  width: 320px;
}

// Rows overflow the tab pane height, so the table area scrolls with a sticky header.
.project-table-card {
  min-height: 0;
  flex: 1;
  overflow: auto;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  background: #fff;

  :deep(.ant-table-thead > tr > th) {
    position: sticky;
    top: 0;
    z-index: 2;
    height: 48px;
    padding: 14px 16px;
    border-bottom: 1px solid #ebeef5;
    color: #657188;
    background: #fafbfd;
    font-size: 12px;
    font-weight: 600;
  }
}

.project-table {
  :deep(.ant-table-row) {
    cursor: default;
  }
}

.drag-handle {
  color: var(--app-text-tertiary, #8c8c8c);
  cursor: grab;
}

.member-count {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.danger-link {
  color: #ff4d4f;
}

.delete-tip {
  margin-bottom: 8px;
}

.delete-target {
  margin: 0;
  color: var(--app-text-secondary, #595959);
  font-weight: 600;
}
</style>
