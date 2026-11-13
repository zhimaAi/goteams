<template>
  <div class="pipeline-settings-panel">
    <aside class="pipeline-list-panel">
      <div class="pipeline-list-header">
        <span class="panel-title">{{ $t("pipeline_list_title") }}</span>
        <button
          type="button"
          v-if="canManage"
          class="add-button"
          :disabled="scopeOptionsLoading"
          :aria-label="$t('pipeline_add')"
          @click="showCreateModal"
        >
          <PlusOutlined />
        </button>
      </div>

      <div class="pipeline-search">
        <a-input-search
          v-model:value="keyword"
          allow-clear
          :placeholder="$t('pipeline_list_search_placeholder')"
        />
      </div>

      <div class="pipeline-list-items">
        <div
          v-for="item in filteredPipelineList"
          :key="item.id"
          class="pipeline-card"
          :class="{ active: selectedPipelineId === Number(item.id) }"
        >
          <div
            class="pipeline-card-main"
            role="button"
            tabindex="0"
            @click="selectPipeline(item.id)"
            @keydown.enter="selectPipeline(item.id)"
            @keydown.space.prevent="selectPipeline(item.id)"
          >
            <div class="pipeline-card-head">
              <img
                :src="item.icon || PIPELINE_AVATARS[0]"
                alt=""
                class="pipeline-card-avatar"
              />
              <div class="pipeline-card-copy">
                <span class="pipeline-card-name">{{ item.name }}</span>
                <span class="pipeline-card-description">{{
                  item.description || $t("pipeline_noDescription")
                }}</span>
              </div>
            </div>
            <div class="pipeline-card-scope">
              <span class="scope-label">{{ $t("pipeline_member_scope") }}</span>
              <span class="scope-value">
                <TeamOutlined v-if="item.scope === 'all'" class="scope-icon" />
                <span>{{ scopeText(item) }}</span>
              </span>
            </div>
            <div v-if="item.scope !== 'all'" class="pipeline-card-avatars">
              <span
                v-for="member in item.scope_members || []"
                :key="member.user_id"
                class="scope-avatar"
                :title="member.name"
              >
                <img v-if="member.avatar" :src="member.avatar" alt="" />
                <span
                  v-else
                  class="scope-avatar-char"
                  :style="{ backgroundColor: item.color || defaultColor }"
                  >{{ firstCharacter(member.name) }}</span
                >
              </span>
              <span
                v-if="
                  (item.scope_total || 0) > (item.scope_members || []).length
                "
                class="scope-avatar scope-avatar-more"
              >
                +{{ item.scope_total - (item.scope_members || []).length }}
              </span>
            </div>
            <div class="pipeline-card-footer">
              <span class="pipeline-card-steps">
                {{
                  $t("pipeline_workflow_steps_total", {
                    count: item.step_count || 0,
                  })
                }}
              </span>
            </div>
          </div>

          <a-dropdown
            v-if="canManage"
            :trigger="['click']"
            placement="bottomRight"
            @click.stop
          >
            <button
              type="button"
              class="action-btn pipeline-card-action"
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
                  <EditOutlined /> {{ $t("pipeline_edit") }}
                </a-menu-item>
                <a-menu-item key="delete" danger>
                  <DeleteOutlined /> {{ $t("pipeline_delete") }}
                </a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </div>

        <a-empty
          v-if="!filteredPipelineList.length"
          :description="$t('pipeline_empty')"
        />
      </div>
    </aside>

    <main class="pipeline-detail-panel">
      <template v-if="selectedPipeline">
        <header class="detail-header">
          <div class="header-copy">
            <span class="header-eyebrow">
              <RobotOutlined />
              {{ $t("pipeline_agent_orchestration") }}
            </span>
            <h4 class="detail-name">{{ selectedPipeline.name }}</h4>
          </div>
          <div v-if="canManage" class="header-actions">
            <a-button
              type="primary"
              class="add-agent-button"
              @click="openAddStep"
            >
              <PlusOutlined />
              {{ $t("pipeline_add_step") }}
            </a-button>
          </div>
        </header>

        <div class="step-list">
          <template v-if="sortedSteps.length">
            <div
              v-for="(step, index) in sortedSteps"
              :key="step.step_key || `step-${index}`"
              class="step-row"
            >
              <div class="step-timeline" aria-hidden="true">
                <span class="step-number">{{ index + 1 }}</span>
                <span v-if="index < sortedSteps.length - 1" class="step-line" />
              </div>
              <div class="step-content">
                <div class="step-body">
                  <div class="step-heading">
                    <img
                      :src="stepAvatar(step, index)"
                      alt=""
                      class="step-avatar"
                    />
                    <span class="step-name">{{ step.name }}</span>
                  </div>
                  <p class="step-description">
                    {{
                      step.prompt_content ||
                      step.remark ||
                      $t("pipeline_no_prompt")
                    }}
                  </p>
                </div>
                <footer v-if="canManage" class="step-footer">
                  <div class="step-actions">
                    <button
                      type="button"
                      class="action-btn"
                      :disabled="index === 0 || savingStep"
                      :aria-label="$t('pipeline_step_move_up')"
                      :title="$t('pipeline_step_move_up')"
                      @click="handleMoveStep(index, -1)"
                    >
                      <ArrowUpOutlined />
                    </button>
                    <button
                      type="button"
                      class="action-btn"
                      :disabled="index === sortedSteps.length - 1 || savingStep"
                      :aria-label="$t('pipeline_step_move_down')"
                      :title="$t('pipeline_step_move_down')"
                      @click="handleMoveStep(index, 1)"
                    >
                      <ArrowDownOutlined />
                    </button>
                    <button
                      type="button"
                      class="action-btn action-btn-edit"
                      :aria-label="$t('pipeline_step_edit_title')"
                      :title="$t('pipeline_step_edit_title')"
                      @click="openEditStep(index)"
                    >
                      <EditOutlined />
                    </button>
                    <button
                      type="button"
                      class="action-btn action-btn-danger"
                      :aria-label="$t('pipeline_delete_step')"
                      :title="$t('pipeline_delete_step')"
                      @click="handleRemoveStep(index)"
                    >
                      <DeleteOutlined />
                    </button>
                  </div>
                </footer>
              </div>
            </div>
          </template>
          <a-empty
            v-else
            :description="$t('pipeline_step_empty')"
            class="step-empty"
          >
            <a-button
              v-if="canManage"
              type="primary"
              size="small"
              @click="openAddStep"
            >
              <PlusOutlined />
              {{ $t("pipeline_add_step") }}
            </a-button>
          </a-empty>
        </div>
      </template>
      <a-empty
        v-else
        :description="$t('pipeline_select_empty')"
        class="empty-detail"
      />
    </main>

    <!-- 新增流水线 -->
    <a-modal
      v-model:open="createModalVisible"
      :title="$t('pipeline_add')"
      :footer="null"
      :destroy-on-close="true"
      width="730px"
      centered
      class="pipeline-editor-modal"
    >
      <PipelineBasicForm
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

    <!-- 编辑流水线基本信息 -->
    <a-modal
      v-model:open="editBasicModalVisible"
      :title="$t('pipeline_edit')"
      :footer="null"
      :destroy-on-close="true"
      width="730px"
      centered
      class="pipeline-editor-modal"
    >
      <PipelineBasicForm
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

    <!-- 步骤编辑弹窗 -->
    <StepEditorModal
      v-model:open="stepModalVisible"
      :step="editingStep"
      :index="editingStepIndex"
      :loading="savingStep"
      @save="handleStepSave"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { message, Modal } from "ant-design-vue";
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EditOutlined,
  MoreOutlined,
  PlusOutlined,
  RobotOutlined,
  TeamOutlined,
} from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import {
  listEnterpriseDepartments,
  listEnterpriseMembers,
} from "@/api/workspace";
import {
  createPipeline,
  deletePipeline,
  getPipeline,
  listPipelines,
  updatePipeline,
  updatePipelineWorkflow,
  uploadPipelineIcon,
} from "@/api/pipeline";
import PipelineBasicForm from "@/components/pipeline/PipelineBasicForm.vue";
import StepEditorModal from "@/components/pipeline/StepEditorModal.vue";
import {
  PIPELINE_AVATARS,
  PIPELINE_STEP_AVATARS,
  resolveAvatar,
} from "@/components/pipeline/avatars";

const { t: $t } = useI18n();
const authStore = useAuthStore();
const canManage = computed(() =>
  authStore.hasEnterprisePermission("pipeline:manage"),
);
const scopeOptionsLoading = ref(false);
const route = useRoute();

const defaultColor = "#3157E2";

const pipelineList = ref([]);
const keyword = ref("");
const selectedPipelineId = ref(null);
const selectedPipeline = ref(null);
const memberOptions = ref([]);
const departmentOptions = ref([]);
const savingBasic = ref(false);
const savingWorkflow = ref(false);
const savingStep = ref(false);
const deletingId = ref(null);
const pendingIconFile = ref(null);
const createModalVisible = ref(false);
const creating = ref(false);

const editBasicModalVisible = ref(false);
const stepModalVisible = ref(false);
const editingStep = ref(null);
const editingStepIndex = ref(-1);

const basicForm = ref(createBasicForm());
const createForm = ref(createBasicForm());
const workflowForm = ref({
  expectedWorkflowVersion: 1,
  cliType: "codex",
  steps: [],
});

const filteredPipelineList = computed(() => {
  const value = keyword.value.trim().toLowerCase();
  if (!value) return pipelineList.value;
  return pipelineList.value.filter(
    (item) =>
      (item.name || "").toLowerCase().includes(value) ||
      (item.description || "").toLowerCase().includes(value),
  );
});

const sortedSteps = computed(() => {
  const steps = [...(workflowForm.value.steps || [])];
  return steps.sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
});

onMounted(async () => {
  await loadPipelineList();
  const routePipelineId = Number(route.query.pipelineId);
  const target =
    routePipelineId > 0 &&
    pipelineList.value.some((item) => Number(item.id) === routePipelineId)
      ? routePipelineId
      : pipelineList.value[0]?.id;
  if (target) await selectPipeline(target);
});

function createBasicForm() {
  return {
    name: "",
    description: "",
    color: defaultColor,
    scope: "members",
    memberIds: [],
    departmentIds: [],
    icon: PIPELINE_AVATARS[0],
  };
}

function firstCharacter(name) {
  return (name || "?").trim().charAt(0) || "?";
}

function stepAvatar(step, index) {
  return resolveAvatar(step?.avatar, PIPELINE_STEP_AVATARS, index);
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

async function loadPipelineList() {
  try {
    const res = await listPipelines({ page: 1, size: 100 });
    pipelineList.value = res.list || [];
  } catch (e) {
    // Errors handled by the axios interceptor.
  }
}

async function selectPipeline(pipelineId) {
  const numericId = Number(pipelineId);
  if (!numericId) return;
  selectedPipelineId.value = numericId;
  try {
    const detail = await getPipeline(numericId);
    selectedPipeline.value = detail;
    basicForm.value = {
      name: detail.name || "",
      description: detail.description || "",
      color: detail.color || defaultColor,
      scope: detail.scope || "members",
      memberIds: (detail.members || []).map((member) => Number(member.user_id)),
      departmentIds: (detail.department_ids || []).map(Number),
      icon: detail.icon || "",
    };
    const loadedSteps = (detail.steps || []).map((step, index) => ({
      step_key: step.step_key,
      name: step.name,
      avatar: step.avatar || "",
      prompt_content: step.prompt_content,
      remark: step.remark,
      sort_order: step.sort_order ?? index,
    }));
    workflowForm.value = {
      expectedWorkflowVersion: detail.workflow_version,
      cliType: detail.cli_type || "",
      steps: loadedSteps,
    };
    pendingIconFile.value = null;
  } catch (e) {
    selectedPipeline.value = null;
  }
}

async function uploadPendingIcon(pipelineId, file) {
  if (!file) return;
  try {
    const result = await uploadPipelineIcon(pipelineId, file);
    basicForm.value = { ...basicForm.value, icon: result.icon || "" };
  } catch (e) {
    message.warning($t("pipeline_icon_upload_failed"));
  }
}

// ── 基本信息 ──────────────────────────────────────────────

async function showCreateModal() {
  if (!(await loadScopeOptions())) return;
  createForm.value = createBasicForm();
  createModalVisible.value = true;
}

async function openEditBasic() {
  if (!(await loadScopeOptions())) return;
  basicForm.value = {
    name: selectedPipeline.value?.name || "",
    description: selectedPipeline.value?.description || "",
    color: selectedPipeline.value?.color || defaultColor,
    scope: selectedPipeline.value?.scope || "members",
    memberIds: (selectedPipeline.value?.members || []).map((member) =>
      Number(member.user_id),
    ),
    departmentIds: (selectedPipeline.value?.department_ids || []).map(Number),
    icon: selectedPipeline.value?.icon || PIPELINE_AVATARS[0],
  };
  pendingIconFile.value = null;
  editBasicModalVisible.value = true;
}

async function handleEditBasic(form) {
  if (!canManage.value) return;
  if (!form.name.trim()) {
    message.warning($t("pipeline_name_required"));
    return;
  }
  savingBasic.value = true;
  try {
    await updatePipeline(selectedPipelineId.value, {
      name: form.name.trim(),
      description: form.description.trim(),
      color: form.color,
      icon: form.icon,
      status: selectedPipeline.value?.status ?? 1,
      scope: form.scope,
      member_ids: form.scope === "members" ? form.memberIds : [],
      department_ids: form.scope === "departments" ? form.departmentIds : [],
    });
    await uploadPendingIcon(selectedPipelineId.value, form.iconFile);
    pendingIconFile.value = null;
    message.success($t("pipeline_saved"));
    editBasicModalVisible.value = false;
    await loadPipelineList();
    await selectPipeline(selectedPipelineId.value);
  } catch (e) {
    // Errors handled by the axios interceptor.
  } finally {
    savingBasic.value = false;
  }
}

async function handleCreate(form) {
  if (!canManage.value) return;
  if (!form.name.trim()) return;
  creating.value = true;
  try {
    const result = await createPipeline({
      name: form.name.trim(),
      description: form.description.trim(),
      color: form.color,
      icon: form.icon || PIPELINE_AVATARS[0],
      cli_type: "codex",
      status: 1,
      scope: form.scope,
      member_ids: form.scope === "members" ? form.memberIds : [],
      department_ids: form.scope === "departments" ? form.departmentIds : [],
      steps: [
        {
          name: $t("pipeline_default_step_name"),
          prompt_content: $t("pipeline_default_step_prompt"),
          remark: "",
          sort_order: 0,
        },
      ],
    });
    if (form.iconFile) await uploadPendingIcon(result.id, form.iconFile);
    message.success($t("pipeline_created"));
    createModalVisible.value = false;
    await loadPipelineList();
    await selectPipeline(result.id);
  } catch (e) {
    // Errors handled by the axios interceptor.
  } finally {
    creating.value = false;
  }
}

// ── 步骤操作（只读卡片 + 弹窗编辑） ────────────────────────

function buildStepsPayload(steps) {
  return steps.map((step, index) => ({
    step_key: step.step_key || "",
    name: step.name,
    avatar: step.avatar || "",
    prompt_content: step.prompt_content,
    remark: step.remark,
    sort_order: index,
  }));
}

async function commitWorkflow(newSteps) {
  if (!canManage.value || savingWorkflow.value) return false;
  if (!newSteps.length) {
    message.warning($t("pipeline_workflow_required"));
    return false;
  }
  if (newSteps.some((step) => !String(step.name || "").trim())) {
    message.warning($t("pipeline_step_name_required"));
    return false;
  }
  if (newSteps.some((step) => !String(step.prompt_content || "").trim())) {
    message.warning($t("pipeline_step_prompt_required"));
    return false;
  }
  savingWorkflow.value = true;
  try {
    await updatePipelineWorkflow(selectedPipelineId.value, {
      expected_workflow_version: workflowForm.value.expectedWorkflowVersion,
      cli_type: workflowForm.value.cliType || "",
      steps: buildStepsPayload(newSteps),
    });
    await selectPipeline(selectedPipelineId.value);
    return true;
  } catch (e) {
    message.error($t("pipeline_step_save_failed"));
    return false;
  } finally {
    savingWorkflow.value = false;
  }
}

function openAddStep() {
  if (!canManage.value) return;
  editingStep.value = null;
  editingStepIndex.value = sortedSteps.value.length;
  stepModalVisible.value = true;
}

function openEditStep(index) {
  if (!canManage.value) return;
  editingStep.value = { ...sortedSteps.value[index] };
  editingStepIndex.value = index;
  stepModalVisible.value = true;
}

async function handleStepSave(form) {
  const steps = [...sortedSteps.value];
  const isEditing =
    editingStepIndex.value >= 0 && editingStepIndex.value < steps.length;
  if (isEditing) {
    steps[editingStepIndex.value] = {
      ...steps[editingStepIndex.value],
      ...form,
    };
  } else {
    steps.push({ ...form, sort_order: steps.length });
  }
  savingStep.value = true;
  const ok = await commitWorkflow(steps);
  savingStep.value = false;
  if (ok) {
    stepModalVisible.value = false;
    message.success($t("pipeline_saved"));
  }
}

async function handleMoveStep(index, direction) {
  const steps = [...sortedSteps.value];
  const target = index + direction;
  if (target < 0 || target >= steps.length) return;
  const [moved] = steps.splice(index, 1);
  steps.splice(target, 0, moved);
  savingStep.value = true;
  const ok = await commitWorkflow(steps);
  savingStep.value = false;
  if (!ok) message.error($t("pipeline_step_move_failed"));
}

async function handleRemoveStep(index) {
  const steps = sortedSteps.value.filter((_, i) => i !== index);
  if (!steps.length) {
    message.warning($t("pipeline_workflow_required"));
    return;
  }
  savingStep.value = true;
  const ok = await commitWorkflow(steps);
  savingStep.value = false;
  if (!ok) message.error($t("pipeline_step_delete_failed"));
}

// ── 列表卡片操作 ──────────────────────────────────────────

function handleCardAction({ key }, item) {
  if (!canManage.value) return;
  if (key === "edit") {
    selectPipeline(item.id).then(() => openEditBasic());
    return;
  }
  if (key === "delete") {
    Modal.confirm({
      title: $t("pipeline_deleteConfirm"),
      okText: $t("common_delete"),
      okType: "danger",
      cancelText: $t("common_cancel"),
      onOk: () => handleDelete(item.id),
    });
  }
}

async function handleDelete(pipelineId) {
  if (!canManage.value) return;
  const numericId = Number(pipelineId);
  deletingId.value = numericId;
  try {
    await deletePipeline(numericId);
    message.success($t("pipeline_deleted"));
    await loadPipelineList();
    if (selectedPipelineId.value === numericId) {
      selectedPipeline.value = null;
      selectedPipelineId.value = null;
      const nextPipeline = pipelineList.value[0];
      if (nextPipeline) await selectPipeline(nextPipeline.id);
    }
  } catch (e) {
    // Errors handled by the axios interceptor.
  } finally {
    deletingId.value = null;
  }
}
</script>

<style scoped lang="less">
.pipeline-settings-panel {
  display: flex;
  flex-direction: row;
  width: 100%;
  height: 100%;
  min-height: 0;
}

// 覆盖 PipelinePage 传入的 .pipeline-main-content 的 flex-direction: column
.pipeline-settings-panel.pipeline-settings-panel {
  flex-direction: row;
}

// ── 左侧列表面板 ────────────────────────────────────────────

.pipeline-list-panel {
  display: flex;
  width: 348px;
  flex: 0 0 348px;
  flex-direction: column;
  border-right: 1px solid #d9d9d9;
  overflow: hidden;
}

.pipeline-list-header {
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

.pipeline-search {
  padding: 0 24px 12px;

  :deep(.ant-input-search .ant-input) {
    border-radius: 6px;
    border-color: #e5e7eb;
    background: #fafafa;

    &:focus {
      border-color: #3157e2;
      background: #fff;
      box-shadow: 0 0 0 2px rgba(49, 87, 226, 0.12);
    }
  }

  :deep(.ant-input-search .ant-btn) {
    border-radius: 0 6px 6px 0;
    border-color: #e5e7eb;
    background: #fafafa;
  }
}

.pipeline-list-items {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8px;
  padding: 0 24px 24px;
  overflow-y: auto;
}

// ── 流水线卡片 ──────────────────────────────────────────────

.pipeline-card {
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

.pipeline-card-main {
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

.pipeline-card-head {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.pipeline-card-avatar {
  display: inline-flex;
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  object-fit: cover;
}

.pipeline-card-avatar-char {
  background: #3157e2;
}

.pipeline-card-copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
}

.pipeline-card-name {
  overflow: hidden;
  color: #1d1d1f;
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipeline-card-description {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  color: #595959;
  font-size: 12px;
  line-height: 20px;
}

.pipeline-card-scope {
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

.pipeline-card-avatars {
  display: flex;
  align-items: center;
  padding-left: 50px;
}

.scope-avatar {
  display: inline-flex;
  width: 24px;
  height: 24px;
  align-items: center;
  justify-content: center;
  border: 1px solid #fff;
  border-radius: 50%;
  overflow: hidden;

  & + .scope-avatar {
    margin-left: -8px;
  }

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.scope-avatar-char {
  display: inline-flex;
  width: 100%;
  height: 100%;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 12px;
}

.scope-avatar-more {
  border: 0;
  background: rgba(0, 0, 0, 0.45);
  color: #fff;
  font-size: 12px;
}

.pipeline-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-left: 50px;
}

.pipeline-card-steps {
  color: #8c8c8c;
  font-size: 12px;
}

.pipeline-card-action {
  position: absolute;
  z-index: 1;
  top: 12px;
  right: 12px;
  opacity: 0;
  pointer-events: none;
}

.pipeline-card:hover .pipeline-card-action,
.pipeline-card:focus-within .pipeline-card-action {
  opacity: 1;
  pointer-events: auto;
}

// ── 右侧详情面板 ────────────────────────────────────────────

.pipeline-detail-panel {
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

.step-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 24px;
}

// ── 步骤卡片（只读 + 时间线） ────────────────────────────────

.step-row {
  position: relative;
  display: flex;
  gap: 16px;
  padding: 24px 0;
  background: #fff;

  &::after {
    position: absolute;
    right: 24px;
    bottom: 0;
    left: 64px;
    height: 1px;
    background: #f0f0f0;
    content: "";
  }

  &:last-child::after {
    display: none;
  }
}

.step-timeline {
  display: flex;
  width: 24px;
  flex: 0 0 24px;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding-top: 13px;
}

.step-number {
  display: flex;
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #595959;
  background: #f0f0f0;
  font-size: 13px;
  font-weight: 500;
  line-height: 24px;
}

.step-line {
  width: 1px;
  flex: 1;
  min-height: 12px;
  background: #f0f0f0;
}

.step-content {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  gap: 20px;
}

.step-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 16px;
}

.step-heading {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 12px;
}

.step-name {
  overflow: hidden;
  color: #1d1d1f;
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.step-avatar {
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  border-radius: 50%;
  object-fit: cover;
  background: #fafafa;
}

.step-description {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  margin: 0;
  color: #595959;
  font-size: 14px;
  line-height: 26px;
}

.step-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

.step-actions {
  display: flex;
  align-items: center;
  gap: 8px;
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

.step-empty {
  margin: 120px auto;
}

.empty-detail {
  margin: auto;
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
  .pipeline-list-panel {
    width: 100%;
    flex: 0 0 auto;
    max-height: 40vh;
    border-right: 0;
    border-bottom: 1px solid #d9d9d9;
  }

  .pipeline-detail-panel {
    padding: 16px;
  }

  .detail-header {
    padding: 12px 16px;
  }

  .step-list {
    padding: 0 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .pipeline-card,
  .action-btn,
  .step-actions {
    transition: none;
  }
}
</style>
