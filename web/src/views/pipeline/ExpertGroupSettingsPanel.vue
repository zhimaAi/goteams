<template>
  <div class="expert-group-settings-panel">
    <aside class="expert-group-list-panel">
      <div class="expert-group-list-header">
        <span class="panel-title">{{ $t("expert_group_list_title") }}</span>
        <button
          type="button"
          v-if="canManage"
          class="add-button"
          :disabled="scopeOptionsLoading"
          :aria-label="$t('expert_group_add')"
          @click="openCreateModal"
        >
          <PlusOutlined />
        </button>
      </div>

      <div class="expert-group-search">
        <a-input
          v-model:value="keyword"
          allow-clear
          :placeholder="$t('expert_group_search_placeholder')"
        />
      </div>

      <div class="expert-group-list-items">
        <a-empty
          v-if="!filteredList.length"
          :description="$t('expert_group_empty')"
        />
        <div
          v-for="item in filteredList"
          :key="item.id"
          class="expert-group-card"
          :class="{ active: selectedId === Number(item.id) }"
        >
          <div
            class="expert-group-card-main"
            role="button"
            tabindex="0"
            @click="selectGroup(item.id)"
            @keydown.enter="selectGroup(item.id)"
            @keydown.space.prevent="selectGroup(item.id)"
          >
            <div class="expert-group-card-head">
              <img
                :src="item.avatar || EXPERT_GROUP_AVATARS[0]"
                alt=""
                class="expert-group-card-avatar"
              />
              <div class="expert-group-card-copy">
                <span class="expert-group-card-name">{{ item.name }}</span>
                <span class="expert-group-card-description">{{
                  item.description || $t("pipeline_noDescription")
                }}</span>
              </div>
            </div>
            <div class="expert-group-card-scope">
              <span class="scope-label">{{ $t("pipeline_member_scope") }}</span>
              <span class="scope-value">
                <TeamOutlined v-if="item.scope === 'all'" class="scope-icon" />
                <span>{{ scopeText(item) }}</span>
              </span>
            </div>
            <div v-if="cardAgents(item).length" class="avatar-chain">
              <span
                v-for="(agent, index) in visibleCardAgents(item)"
                :key="`${item.id}-${index}`"
                class="agent-avatar-item"
                :class="{
                  'agent-avatar-item--leader': agent.role === 'leader',
                }"
                :title="agent.name"
              >
                <img :src="agentAvatar(agent, index)" alt="" />
                <span
                  v-if="agent.role === 'leader'"
                  class="expert-leader-badge"
                  :aria-label="$t('expert_group_agent_leader')"
                >
                  <CrownOutlined />
                </span>
              </span>
              <span v-if="hiddenCardAgentCount(item)" class="avatar-overflow">
                +{{ hiddenCardAgentCount(item) }}
              </span>
            </div>
          </div>

          <a-dropdown
            v-if="canManage && item.can_manage"
            :trigger="['click']"
            placement="bottomRight"
            @click.stop
          >
            <button
              type="button"
              class="action-btn expert-group-card-action"
              :disabled="scopeOptionsLoading"
              :aria-label="$t('common_edit')"
              aria-haspopup="menu"
              @click.stop
            >
              <MoreOutlined />
            </button>
            <template #overlay>
              <a-menu @click="handleCardAction($event, item)">
                <a-menu-item key="edit">
                  <EditOutlined /> {{ $t("expert_group_edit") }}
                </a-menu-item>
                <a-menu-item key="delete" danger>
                  <DeleteOutlined /> {{ $t("expert_group_delete") }}
                </a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </div>
      </div>
    </aside>

    <main class="expert-group-detail-panel">
      <template v-if="selectedGroup">
        <header class="detail-header">
          <div class="header-copy">
            <span class="header-eyebrow">
              <TeamOutlined />
              {{ $t("expert_group_orchestration") }}
            </span>
            <h4 class="detail-name">{{ selectedGroup.name }}</h4>
          </div>
          <div v-if="selectedGroupCanManage" class="header-actions">
            <a-button
              type="primary"
              class="add-agent-button"
              @click="openAddAgent"
            >
              <PlusOutlined />
              {{ $t("expert_group_add_agent") }}
            </a-button>
          </div>
        </header>

        <div class="detail-scroll">
          <section v-if="!leaderAgent" class="member-empty">
            <a-empty :description="$t('expert_group_leader_empty')" />
            <p>{{ $t("expert_group_leader_empty_hint") }}</p>
            <div v-if="selectedGroupCanManage" class="empty-actions">
              <a-button
                v-if="memberAgents.length"
                type="primary"
                ghost
                @click="openSetLeader"
              >
                {{ $t("expert_group_set_leader") }}
              </a-button>
              <a-button @click="openAddLeader">
                <PlusOutlined />
                {{ $t("expert_group_create_leader") }}
              </a-button>
            </div>
          </section>
          <article
            v-for="agent in orderedAgents"
            :key="agent.originalIndex"
            class="member-row"
            :class="{ 'member-row--leader': agent.role === 'leader' }"
          >
            <div class="member-content">
              <div class="member-heading">
                <img :src="agentAvatar(agent, agent.originalIndex)" alt="" />
                <strong>{{ agent.name }}</strong>
                <span v-if="agent.role === 'leader'" class="leader-badge">
                  <CrownOutlined />
                  {{ $t("expert_group_agent_leader") }}
                </span>
              </div>
              <p class="member-description">
                {{ agent.prompt || $t("expert_group_no_prompt") }}
              </p>
              <footer v-if="selectedGroupCanManage">
                <div class="member-actions">
                  <button
                    v-if="agent.role === 'leader' && memberAgents.length"
                    type="button"
                    class="leader-change"
                    @click="openSetLeader"
                  >
                    {{ $t("expert_group_change_leader") }}
                  </button>
                  <button
                    type="button"
                    class="action-btn action-btn-edit"
                    :aria-label="$t('expert_group_agent_edit_title')"
                    @click="openEditAgent(agent.originalIndex)"
                  >
                    <EditOutlined />
                  </button>
                  <button
                    v-if="agent.role !== 'leader'"
                    type="button"
                    class="action-btn action-btn-danger"
                    :aria-label="$t('expert_group_delete_agent')"
                    @click="handleRemoveAgent(agent.originalIndex)"
                  >
                    <DeleteOutlined />
                  </button>
                </div>
              </footer>
            </div>
          </article>
        </div>
      </template>
      <a-empty
        v-else
        :description="$t('expert_group_select_empty')"
        class="empty-detail"
      />
    </main>

    <!-- 新增专家团 -->
    <a-modal
      v-model:open="createModalVisible"
      :title="$t('expert_group_add')"
      :footer="null"
      :destroy-on-close="true"
      width="730px"
      centered
      class="pipeline-editor-modal"
    >
      <ExpertGroupBasicForm
        v-model="createForm"
        :member-options="memberOptions"
        :department-options="departmentOptions"
        :loading="creating"
        :show-actions="true"
        :submit-text="$t('common_add')"
        @submit="handleCreate"
        @cancel="createModalVisible = false"
      />
    </a-modal>

    <!-- 编辑专家团基本信息 -->
    <a-modal
      v-model:open="editBasicModalVisible"
      :title="$t('expert_group_edit')"
      :footer="null"
      :destroy-on-close="true"
      width="730px"
      centered
      class="pipeline-editor-modal"
    >
      <ExpertGroupBasicForm
        v-model="basicForm"
        :member-options="memberOptions"
        :department-options="departmentOptions"
        :loading="savingBasic"
        :show-actions="true"
        :submit-text="$t('common_save')"
        @submit="handleEditBasic"
        @cancel="editBasicModalVisible = false"
      />
    </a-modal>

    <!-- 专家编辑弹窗 -->
    <AgentEditorModal
      v-model:open="agentModalVisible"
      :agent="editingAgent"
      :index="editingAgentIndex"
      :role="editingRole"
      :loading="savingAgent"
      @save="handleAgentSave"
    />

    <!-- 设置团长 -->
    <a-modal
      v-model:open="setLeaderModalVisible"
      :title="$t('expert_group_set_leader')"
      :ok-text="$t('common_save')"
      :cancel-text="$t('common_cancel')"
      :width="520"
      :destroy-on-close="true"
      class="pipeline-editor-modal"
      @ok="handleSetLeaderConfirm"
    >
      <p class="set-leader-desc">{{ $t("expert_group_set_leader_hint") }}</p>
      <a-select
        v-model:value="setLeaderTarget"
        :placeholder="$t('expert_group_select_empty')"
        :options="memberAgents.map((a, i) => ({ value: i, label: a.name }))"
        style="width: 100%"
      />
    </a-modal>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { message, Modal } from "ant-design-vue";
import {
  CrownOutlined,
  DeleteOutlined,
  EditOutlined,
  MoreOutlined,
  PlusOutlined,
  TeamOutlined,
} from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import {
  listEnterpriseDepartments,
  listEnterpriseMembers,
} from "@/api/workspace";
import {
  createExpertGroup,
  deleteExpertGroup,
  getExpertGroup,
  listExpertGroups,
  updateExpertGroup,
} from "@/api/pipeline";
import ExpertGroupBasicForm from "@/components/pipeline/ExpertGroupBasicForm.vue";
import AgentEditorModal from "@/components/pipeline/AgentEditorModal.vue";
import {
  AGENT_AVATARS,
  EXPERT_GROUP_AVATARS,
  resolveAvatar,
} from "@/components/pipeline/avatars";

const { t: $t } = useI18n();
const authStore = useAuthStore();
const canManage = computed(() =>
  authStore.hasEnterprisePermission("pipeline:manage"),
);
const scopeOptionsLoading = ref(false);

const groupList = ref([]);
const keyword = ref("");
const selectedId = ref(null);
const selectedGroup = ref(null);
const memberOptions = ref([]);
const departmentOptions = ref([]);
const savingBasic = ref(false);
const savingAgent = ref(false);
const deletingId = ref(null);
const createModalVisible = ref(false);
const creating = ref(false);

const editBasicModalVisible = ref(false);
const agentModalVisible = ref(false);
const setLeaderModalVisible = ref(false);
const setLeaderTarget = ref(null);
const editingAgent = ref(null);
const editingAgentIndex = ref(-1);
const editingRole = ref("member");

const basicForm = ref(createBasicForm());
const createForm = ref(createBasicForm());

const filteredList = computed(() => {
  const value = keyword.value.trim().toLowerCase();
  if (!value) return groupList.value;
  return groupList.value.filter(
    (item) =>
      (item.name || "").toLowerCase().includes(value) ||
      (item.description || "").toLowerCase().includes(value),
  );
});

const selectedGroupCanManage = computed(
  () =>
    canManage.value &&
    !!groupList.value.find((g) => Number(g.id) === selectedId.value)
      ?.can_manage,
);

const agents = computed(() => {
  const raw = selectedGroup.value?.agents || [];
  return raw
    .map((agent, index) => ({ ...agent, originalIndex: index }))
    .sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
});

const leaderAgent = computed(
  () => agents.value.find((a) => a.role === "leader") || null,
);
const memberAgents = computed(() =>
  agents.value.filter((a) => a.role !== "leader"),
);
const orderedAgents = computed(() => [
  ...agents.value.filter((agent) => agent.role === "leader"),
  ...agents.value.filter((agent) => agent.role !== "leader"),
]);
const CARD_AGENT_LIMIT = 10;

function cardAgents(item) {
  const raw = item?.agents || [];
  return [
    ...raw.filter((agent) => agent.role === "leader"),
    ...raw.filter((agent) => agent.role !== "leader"),
  ];
}

function visibleCardAgents(item) {
  return cardAgents(item).slice(0, CARD_AGENT_LIMIT);
}

function hiddenCardAgentCount(item) {
  return Math.max(cardAgents(item).length - CARD_AGENT_LIMIT, 0);
}

function agentAvatar(agent, index) {
  return resolveAvatar(agent?.avatar, AGENT_AVATARS, index);
}

onMounted(async () => {
  await loadList();
  const target = groupList.value[0]?.id;
  if (target) await selectGroup(target);
});

function createBasicForm() {
  return {
    name: "",
    description: "",
    avatar: EXPERT_GROUP_AVATARS[0],
    scope: "all",
    memberIds: [],
    departmentIds: [],
  };
}

function scopeText(item) {
  if (item.scope === "all") return $t("pipeline_scope_all");
  if (item.scope === "departments") {
    return $t("pipeline_scope_departments_count", {
      count: item.department_count || 0,
    });
  }
  return $t("pipeline_scope_members_count", {
    count: item.scope_total ?? item.member_count ?? 0,
  });
}

async function loadScopeOptions() {
  if (!canManage.value || scopeOptionsLoading.value) return false;
  scopeOptionsLoading.value = true;
  try {
    const [members, departments] = await Promise.all([
      listEnterpriseMembers(),
      listEnterpriseDepartments(),
    ]);
    memberOptions.value = (members || []).map((member) => ({
      ...member,
      id: Number(member.id),
    }));
    departmentOptions.value = (departments?.items || []).map((item) => ({
      id: Number(item.id),
      parentId: Number(item.parent_id),
      name: item.name,
    }));
    return true;
  } catch (e) {
    // Errors handled by the axios interceptor.
    return false;
  } finally {
    scopeOptionsLoading.value = false;
  }
}

async function loadList() {
  try {
    const res = await listExpertGroups({ page: 1, size: 100 });
    groupList.value = res.list || [];
  } catch (e) {
    // Errors handled by the axios interceptor.
  }
}

async function selectGroup(groupId) {
  const numericId = Number(groupId);
  if (!numericId) return;
  selectedId.value = numericId;
  try {
    const detail = await getExpertGroup(numericId);
    selectedGroup.value = detail;
    const listed = groupList.value.find(
      (group) => Number(group.id) === numericId,
    );
    if (listed) {
      listed.agents = detail.agents || [];
      listed.agent_count = listed.agents.length;
    }
    basicForm.value = {
      name: detail.name || "",
      description: detail.description || "",
      avatar: detail.avatar || "",
      scope: detail.scope || "all",
      memberIds: (detail.members || []).map((member) => Number(member.user_id)),
      departmentIds: (detail.department_ids || []).map(Number),
    };
  } catch (e) {
    selectedGroup.value = null;
  }
}

// ── 基本信息 ──────────────────────────────────────────────

async function openCreateModal() {
  if (!(await loadScopeOptions())) return;
  createForm.value = createBasicForm();
  createModalVisible.value = true;
}

async function openEditBasic() {
  if (!selectedGroupCanManage.value) return;
  if (!(await loadScopeOptions())) return;
  basicForm.value = {
    name: selectedGroup.value?.name || "",
    description: selectedGroup.value?.description || "",
    avatar: selectedGroup.value?.avatar || "",
    scope: selectedGroup.value?.scope || "all",
    memberIds: (selectedGroup.value?.members || []).map((member) =>
      Number(member.user_id),
    ),
    departmentIds: (selectedGroup.value?.department_ids || []).map(Number),
  };
  editBasicModalVisible.value = true;
}

function buildAgentsPayload(agentsList) {
  return agentsList.map((agent, index) => ({
    name: (agent.name || "").trim(),
    role: agent.role === "leader" ? "leader" : "member",
    avatar: agent.avatar || "",
    prompt: agent.prompt || "",
    sort_order: index,
  }));
}

async function handleEditBasic(form) {
  if (!canManage.value) return;
  if (!form.name.trim()) {
    message.warning($t("expert_group_name_required"));
    return;
  }
  const currentAgents = selectedGroup.value?.agents || [];
  savingBasic.value = true;
  try {
    await updateExpertGroup(selectedId.value, {
      name: form.name.trim(),
      description: form.description || "",
      avatar: form.avatar || "",
      status: selectedGroup.value?.status ?? 1,
      scope: form.scope || "all",
      member_ids: form.memberIds || [],
      department_ids: form.departmentIds || [],
      agents: buildAgentsPayload(currentAgents),
    });
    message.success($t("expert_group_saved"));
    editBasicModalVisible.value = false;
    await loadList();
    await selectGroup(selectedId.value);
  } catch (e) {
    // Errors handled by the axios interceptor.
  } finally {
    savingBasic.value = false;
  }
}

async function handleCreate(form) {
  if (!canManage.value) return;
  if (!form.name.trim()) {
    message.warning($t("expert_group_name_required"));
    return;
  }
  creating.value = true;
  try {
    const res = await createExpertGroup({
      name: form.name.trim(),
      description: form.description || "",
      avatar: form.avatar || "",
      status: 1,
      scope: form.scope || "all",
      member_ids: form.memberIds || [],
      department_ids: form.departmentIds || [],
      agents: [
        {
          name: $t("expert_group_agent_leader"),
          role: "leader",
          prompt: $t("expert_group_default_leader_prompt"),
          sort_order: 0,
        },
      ],
    });
    message.success($t("expert_group_created"));
    createModalVisible.value = false;
    await loadList();
    if (res?.id) await selectGroup(res.id);
  } catch (e) {
    // Errors handled by the axios interceptor.
  } finally {
    creating.value = false;
  }
}

// ── 专家操作 ──────────────────────────────────────────────

function currentAgentsRaw() {
  return (selectedGroup.value?.agents || []).map((agent, index) => ({
    name: agent.name,
    role: agent.role,
    avatar: agent.avatar || "",
    prompt: agent.prompt,
    sort_order: agent.sort_order ?? index,
  }));
}

async function commitAgents(newAgents) {
  if (!selectedGroupCanManage.value) return false;
  if (!newAgents.length) {
    message.warning($t("expert_group_need_agent"));
    return false;
  }
  if (newAgents.some((a) => !(a.name || "").trim())) {
    message.warning($t("expert_group_agent_name_required"));
    return false;
  }
  if (newAgents.some((a) => !(a.prompt || "").trim())) {
    message.warning($t("expert_group_agent_prompt_required"));
    return false;
  }
  const leaders = newAgents.filter((a) => a.role === "leader");
  if (leaders.length !== 1) {
    message.warning($t("expert_group_need_leader"));
    return false;
  }
  savingAgent.value = true;
  try {
    const form = basicForm.value;
    await updateExpertGroup(selectedId.value, {
      name: (form.name || selectedGroup.value?.name || "").trim(),
      description: form.description ?? selectedGroup.value?.description ?? "",
      avatar: form.avatar ?? selectedGroup.value?.avatar ?? "",
      status: selectedGroup.value?.status ?? 1,
      scope: form.scope || selectedGroup.value?.scope || "all",
      member_ids: form.memberIds || [],
      department_ids: form.departmentIds || [],
      agents: buildAgentsPayload(newAgents),
    });
    await selectGroup(selectedId.value);
    return true;
  } catch (e) {
    message.error($t("expert_group_agent_save_failed"));
    return false;
  } finally {
    savingAgent.value = false;
  }
}

function openAddAgent() {
  editingAgent.value = null;
  editingAgentIndex.value = currentAgentsRaw().length;
  editingRole.value = "member";
  agentModalVisible.value = true;
}

function openAddLeader() {
  editingAgent.value = null;
  editingAgentIndex.value = currentAgentsRaw().length;
  editingRole.value = "leader";
  agentModalVisible.value = true;
}

function openEditAgent(index) {
  const raw = currentAgentsRaw();
  editingAgent.value = { ...raw[index] };
  editingAgentIndex.value = index;
  editingRole.value = raw[index]?.role || "member";
  agentModalVisible.value = true;
}

async function handleAgentSave(form) {
  const agentsList = currentAgentsRaw();
  const isEditing =
    editingAgentIndex.value >= 0 && editingAgentIndex.value < agentsList.length;
  if (isEditing) {
    agentsList[editingAgentIndex.value] = {
      ...agentsList[editingAgentIndex.value],
      ...form,
    };
  } else {
    agentsList.push({ ...form, sort_order: agentsList.length });
  }
  savingAgent.value = true;
  const ok = await commitAgents(agentsList);
  savingAgent.value = false;
  if (ok) {
    agentModalVisible.value = false;
    message.success($t("expert_group_saved"));
  }
}

async function handleRemoveAgent(index) {
  const agentsList = currentAgentsRaw().filter((_, i) => i !== index);
  savingAgent.value = true;
  const ok = await commitAgents(agentsList);
  savingAgent.value = false;
  if (!ok) message.error($t("expert_group_agent_delete_failed"));
}

async function handleSetLeader(index) {
  const agentsList = currentAgentsRaw();
  agentsList.forEach((agent, i) => {
    agent.role = i === index ? "leader" : "member";
  });
  savingAgent.value = true;
  const ok = await commitAgents(agentsList);
  savingAgent.value = false;
}

function openSetLeader() {
  setLeaderTarget.value = null;
  setLeaderModalVisible.value = true;
}

async function handleSetLeaderConfirm() {
  if (setLeaderTarget.value === null || setLeaderTarget.value === undefined)
    return;
  const targetIndex = memberAgents.value[setLeaderTarget.value]?.originalIndex;
  if (targetIndex === undefined) return;
  setLeaderModalVisible.value = false;
  await handleSetLeader(targetIndex);
}

// ── 列表卡片操作 ──────────────────────────────────────────

function handleCardAction({ key }, item) {
  if (!canManage.value || !item.can_manage) return;
  if (key === "edit") {
    selectGroup(item.id).then(() => openEditBasic());
    return;
  }
  if (key === "delete") {
    Modal.confirm({
      title: $t("expert_group_deleteConfirm"),
      okText: $t("common_delete"),
      okType: "danger",
      cancelText: $t("common_cancel"),
      onOk: () => handleDelete(item.id),
    });
  }
}

async function handleDelete(groupId) {
  if (!canManage.value) return;
  deletingId.value = groupId;
  try {
    await deleteExpertGroup(groupId);
    message.success($t("expert_group_deleted"));
    selectedGroup.value = null;
    selectedId.value = null;
    await loadList();
    const target = groupList.value[0]?.id;
    if (target) await selectGroup(target);
  } catch (e) {
    // Errors handled by the axios interceptor.
  } finally {
    deletingId.value = null;
  }
}
</script>

<style scoped lang="less">
.expert-group-settings-panel {
  display: flex;
  flex-direction: row;
  width: 100%;
  height: 100%;
  min-height: 0;
}

// 覆盖 PipelinePage 传入的 .pipeline-main-content 的 flex-direction: column
.expert-group-settings-panel.expert-group-settings-panel {
  flex-direction: row;
}

// ── 左侧列表面板 ────────────────────────────────────────────

.expert-group-list-panel {
  display: flex;
  width: 348px;
  flex: 0 0 348px;
  flex-direction: column;
  border-right: 1px solid #d9d9d9;
  overflow: hidden;
}

.expert-group-list-header {
  display: flex;
  height: 60px;
  flex: 0 0 60px;
  align-items: center;
  gap: 4px;
  padding: 0 24px;
}

.panel-title {
  flex: 1;
  color: #1d1d1f;
  font-size: 16px;
  font-weight: 600;
}

.add-button,
.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 6px;
  color: #595959;
  background: transparent;
  cursor: pointer;
  transition:
    color 0.18s,
    background-color 0.18s;
}

.add-button {
  width: 28px;
  height: 28px;
  color: #3157e2;

  &:hover {
    background: #f2f4f7;
    color: #2475fc;
  }
}

.action-btn {
  width: 24px;
  height: 24px;

  &:hover:not(:disabled) {
    background: rgba(0, 0, 0, 0.06);
  }

  &:disabled {
    color: #cbd1da;
    cursor: not-allowed;
  }
}

.action-btn-edit {
  color: #3157e2;

  &:hover:not(:disabled) {
    color: #2475fc;
    background: rgba(49, 87, 226, 0.08);
  }
}

.action-btn-danger:hover:not(:disabled) {
  color: #ff4d4f;
  background: #fff1f0;
}

.expert-group-search {
  padding: 0 24px 12px;

  :deep(.ant-input) {
    border-radius: 6px;
    border-color: #e5e7eb;
    background: #fafafa;

    &:focus {
      border-color: #3157e2;
      background: #fff;
      box-shadow: 0 0 0 2px rgba(49, 87, 226, 0.12);
    }
  }
}

.expert-group-list-items {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8px;
  padding: 0 24px 24px;
  overflow-y: auto;
}

// ── 专家团卡片 ──────────────────────────────────────────────

.expert-group-card {
  position: relative;
  display: flex;
  flex-direction: column;
  border: 1px solid transparent;
  border-radius: 12px;
  background: #fff;
  transition: background-color 0.18s;

  &:hover {
    background: #f2f4f7;
  }

  &.active {
    border-color: transparent;
    background: #e5efff;
  }
}

.expert-group-card-main {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 132px;
  padding: 20px 12px;
  cursor: pointer;

  &:focus-visible {
    outline: 2px solid #3157e2;
    outline-offset: 2px;
    border-radius: 12px;
  }
}

.expert-group-card-head {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.expert-group-card-avatar {
  display: block;
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  border-radius: 50%;
  background: #f2f3f5;
  object-fit: cover;
}

.expert-group-card-copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
}

.expert-group-card-name {
  overflow: hidden;
  color: #1d1d1f;
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.expert-group-card-description {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  color: #595959;
  font-size: 12px;
  line-height: 20px;
}

.expert-group-card-scope {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-left: 50px;
}

.scope-label {
  color: #8c8c8c;
  font-size: 12px;
}

.scope-value {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #595959;
  font-size: 12px;
}

.scope-icon {
  font-size: 12px;
}

.avatar-chain {
  display: flex;
  min-height: 25px;
  align-items: center;
  padding-left: 50px;
}

.agent-avatar-item {
  position: relative;
  display: block;
  width: 22px;
  height: 25px;
  flex: 0 0 22px;
  opacity: 0.5;
}

.expert-group-card.active .agent-avatar-item,
.agent-avatar-item--leader {
  opacity: 1;
}

.agent-avatar-item > img {
  position: absolute;
  top: 1px;
  left: 0;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #fff;
  object-fit: cover;
  box-shadow: 0 0 0 2px #fff;
}

.agent-avatar-item--leader {
  width: 34px;
  flex-basis: 34px;
  margin-right: 6px;
}

.expert-leader-badge {
  position: absolute;
  z-index: 1;
  top: -3px;
  left: 16px;
  display: inline-flex;
  width: 12px;
  height: 12px;
  align-items: center;
  justify-content: center;
  border: 1px solid #fff;
  border-radius: 50%;
  color: #fff;
  background: #3157e2;
  font-size: 7px;
  line-height: 1;
}

.avatar-overflow {
  display: inline-flex;
  width: 24px;
  height: 24px;
  align-items: center;
  justify-content: center;
  flex: 0 0 24px;
  margin-left: 8px;
  border-radius: 50%;
  color: #fff;
  background: #8c8c8c;
  font-size: 12px;
  line-height: 1;
}

.expert-group-card-action {
  position: absolute;
  z-index: 1;
  top: 12px;
  right: 12px;
  opacity: 0;
  pointer-events: none;
}

.expert-group-card:hover .expert-group-card-action,
.expert-group-card:focus-within .expert-group-card-action {
  opacity: 1;
  pointer-events: auto;
}

// ── 右侧详情面板 ────────────────────────────────────────────

.expert-group-detail-panel {
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  background: #fff;
}

.detail-header {
  display: flex;
  min-height: 84px;
  flex: 0 0 84px;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 16px 24px;
  background: #fff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.header-copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 4px;
}

.header-eyebrow {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #86868b;
  font-size: 12px;
  line-height: 20px;
}

.detail-name {
  margin: 0;
  overflow: hidden;
  color: #1d1d1f;
  font-size: 20px;
  font-weight: 600;
  line-height: 28px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.add-agent-button {
  display: inline-flex;
  align-items: center;
  height: 32px;
  padding: 0 12px;
  border-radius: 6px;
  background: #3157e2;
  box-shadow: none;
}

.detail-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 24px 24px;
}

.member-row {
  position: relative;
  padding: 24px 0;
  background: #fff;
}

.member-row::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 1px;
  background: #f0f0f0;
  content: "";
}

.member-row--leader {
  border-bottom: 1px solid #f0f0f0;
}

.member-row--leader::after {
  display: none;
}

.member-content {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 20px;
}

.member-heading {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.member-heading > img {
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  border-radius: 50%;
  object-fit: cover;
}

.member-heading strong {
  overflow: hidden;
  color: #1d1d1f;
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.leader-badge {
  display: inline-flex;
  height: 20px;
  flex: 0 0 auto;
  align-items: center;
  gap: 3px;
  padding: 0 5px;
  border-radius: 6px;
  color: #3157e2;
  background: #e5efff;
  font-size: 12px;
  line-height: 20px;
}

.member-description {
  display: -webkit-box;
  overflow: hidden;
  margin: 0;
  color: #595959;
  font-size: 14px;
  line-height: 26px;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.member-row footer {
  display: flex;
  min-height: 26px;
  align-items: center;
  justify-content: flex-end;
}

.member-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.leader-change {
  height: 24px;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  color: #3157e2;
  background: transparent;
  cursor: pointer;
  font-size: 12px;
  line-height: 24px;
}

.leader-change:hover {
  background: rgba(49, 87, 226, 0.08);
}

.member-empty {
  display: flex;
  min-height: 280px;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  padding: 32px 16px;
  text-align: center;
}

.member-empty :deep(.ant-empty) {
  margin: 0;
}

.member-empty :deep(.ant-empty-description) {
  color: #262626;
  font-size: 16px;
  font-weight: 600;
}

.member-empty p {
  margin: 4px 0 18px;
  color: #8c8c8c;
  font-size: 14px;
  line-height: 22px;
}

.empty-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.empty-detail {
  margin: auto;
}

.set-leader-desc {
  margin-bottom: 12px;
  color: #595959;
  font-size: 13px;
}

// ── Modal 弹窗样式（对齐客户端 PipelineEditorModal） ────────

:deep(.pipeline-editor-modal) {
  max-width: calc(100vw - 32px);
  padding-bottom: 0;
}

:deep(.pipeline-editor-modal .ant-modal-content) {
  overflow: hidden;
  padding: 0;
  border-radius: 24px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.16);
}

:deep(.pipeline-editor-modal .ant-modal-header) {
  margin-bottom: 0;
  padding: 24px;
  border-radius: 24px 24px 0 0;
  background: #fff;
}

:deep(.pipeline-editor-modal .ant-modal-title) {
  color: #262626;
  font-size: 24px;
  font-weight: 600;
  line-height: 32px;
}

:deep(.pipeline-editor-modal .ant-modal-body) {
  max-height: calc(100vh - 184px);
  overflow-y: auto;
  padding: 16px 48px 24px;
}

:deep(.pipeline-editor-modal .ant-modal-footer) {
  margin-top: 0;
  padding: 16px 48px;
  border-top: 1px solid #d9d9d9;
}

:deep(.pipeline-editor-modal .ant-modal-footer .ant-btn) {
  min-width: 65px;
  height: 32px;
  padding: 5px 16px;
  border-radius: 6px;
  font-size: 14px;
  line-height: 22px;
}

:deep(.pipeline-editor-modal .ant-form-item-label) {
  padding-bottom: 8px;
}

:deep(.pipeline-editor-modal .ant-form-item-label > label) {
  height: 22px;
  color: #262626;
  font-size: 14px;
  font-weight: 600;
  line-height: 22px;
}

:deep(.pipeline-editor-modal .ant-input),
:deep(.pipeline-editor-modal .ant-input-affix-wrapper),
:deep(.pipeline-editor-modal .ant-select .ant-select-selector),
:deep(.pipeline-editor-modal .ant-tree-select .ant-select-selector) {
  min-height: 40px;
  border-color: #d9d9d9 !important;
  border-radius: 12px !important;
  color: #262626;
  font-size: 14px;
}

:deep(.pipeline-editor-modal .ant-input) {
  padding: 8px 12px;
  line-height: 22px;
}

:deep(.pipeline-editor-modal textarea.ant-input) {
  height: 128px;
  min-height: 128px;
  resize: none;
}

:deep(.pipeline-editor-modal .ant-select .ant-select-selector) {
  padding: 4px 12px !important;
}

:deep(.pipeline-editor-modal .ant-select-selection-item),
:deep(.pipeline-editor-modal .ant-select-selection-placeholder) {
  line-height: 30px !important;
}

:deep(.pipeline-editor-modal .ant-input:hover),
:deep(
  .pipeline-editor-modal
    .ant-select:not(.ant-select-disabled):hover
    .ant-select-selector
) {
  border-color: #3157e2 !important;
}

:deep(.pipeline-editor-modal .ant-input:focus),
:deep(.pipeline-editor-modal .ant-input-focused),
:deep(.pipeline-editor-modal .ant-select-focused .ant-select-selector) {
  border-color: #3157e2 !important;
  box-shadow: 0 0 0 2px rgba(49, 87, 226, 0.15) !important;
}

:deep(.pipeline-editor-modal .ant-radio-wrapper) {
  font-size: 14px;
}

@media (max-width: 760px) {
  :deep(.pipeline-editor-modal .ant-modal-header) {
    padding: 20px 24px;
  }

  :deep(.pipeline-editor-modal .ant-modal-title) {
    font-size: 20px;
    line-height: 28px;
  }

  :deep(.pipeline-editor-modal .ant-modal-body) {
    padding: 16px 24px 24px;
  }

  :deep(.pipeline-editor-modal .ant-modal-footer) {
    padding: 16px 24px;
  }
}

@media (max-width: 900px) {
  .expert-group-list-panel {
    width: 100%;
    flex: 0 0 auto;
    max-height: 40vh;
    border-right: 0;
    border-bottom: 1px solid #d9d9d9;
  }

  .detail-scroll {
    padding: 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .expert-group-card,
  .member-card,
  .member-actions,
  .action-btn {
    transition: none;
  }
}
</style>
