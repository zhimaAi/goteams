<!--
 * CopyProjectMembersModal.vue
 * Copy the selected roles, their permission configuration and their members from another
 * project into the current one.
 -->
<template>
  <a-modal
    :open="open"
    :title="$t('workspace_copyFromProject')"
    width="600px"
    :confirm-loading="saving"
    :ok-text="$t('workspace_copyConfirm')"
    :cancel-text="$t('common_cancel')"
    :ok-button-props="{ disabled: !sourceWsId || !selectedRoleIds.length }"
    :destroy-on-close="true"
    @update:open="handleOpenChange"
    @ok="handleSubmit"
  >
    <div class="copy-section-title">
      {{ $t("workspace_copySelectProject") }}
    </div>
    <a-spin :spinning="projectsLoading">
      <a-radio-group v-model:value="sourceWsId" class="copy-project-group">
        <a-radio
          v-for="project in copyableProjects"
          :key="project.id"
          :value="String(project.id)"
          class="copy-project-item"
        >
          <div class="copy-project-name">{{ project.name }}</div>
          <div v-if="project.description" class="copy-project-desc">
            {{ project.description }}
          </div>
        </a-radio>
      </a-radio-group>
      <a-empty
        v-if="!projectsLoading && !copyableProjects.length"
        :description="$t('workspace_copyNoProject')"
        :image="simpleImage"
      />
    </a-spin>

    <template v-if="sourceWsId">
      <div class="copy-section-title copy-section-title-roles">
        {{ $t("workspace_copySelectRoles") }}
      </div>
      <a-spin :spinning="rolesLoading">
        <a-checkbox-group
          v-model:value="selectedRoleIds"
          class="copy-role-group"
        >
          <a-checkbox
            v-for="role in sourceRoles"
            :key="role.id"
            :value="Number(role.id)"
            class="copy-role-item"
          >
            <div class="copy-role-head">
              <TeamOutlined class="copy-role-icon" />
              <span class="copy-role-name">{{ role.name }}</span>
              <a-tag v-if="Number(role.is_system) === 1" color="blue">
                {{ $t("workspace_copyPresetTag") }}
              </a-tag>
              <span class="copy-role-count">
                {{
                  $t("workspace_copyRoleMemberCount", { n: role.member_count })
                }}
              </span>
            </div>
            <div v-if="role.description" class="copy-role-desc">
              {{ role.description }}
            </div>
          </a-checkbox>
        </a-checkbox-group>
        <a-empty
          v-if="!rolesLoading && !sourceRoles.length"
          :description="$t('workspace_copyNoRole')"
          :image="simpleImage"
        />
      </a-spin>
    </template>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { message, Empty } from "ant-design-vue";
import { TeamOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  copyProjectMembers,
  listProjectRoles,
  listWorkspaces,
} from "@/api/workspace";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  // Roles and members were copied; the payload is how many members joined.
  copied: (count) => typeof count === "number",
});

const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;

const projectsLoading = ref(false);
const rolesLoading = ref(false);
const saving = ref(false);
const projects = ref([]);
const sourceRoles = ref([]);
const sourceWsId = ref("");
const selectedRoleIds = ref([]);

// listWorkspaces already returns only the projects the caller participates in (all of them for
// the tenant super admin); the current project is never a copy source.
const copyableProjects = computed(() =>
  projects.value.filter((project) => String(project.id) !== String(props.wsId)),
);

async function loadProjects() {
  projectsLoading.value = true;
  try {
    projects.value = (await listWorkspaces()) || [];
  } catch {
    projects.value = [];
  } finally {
    projectsLoading.value = false;
  }
}

async function loadSourceRoles() {
  rolesLoading.value = true;
  try {
    sourceRoles.value = (await listProjectRoles(sourceWsId.value)) || [];
  } catch {
    sourceRoles.value = [];
  } finally {
    rolesLoading.value = false;
  }
}

async function handleSubmit() {
  if (!sourceWsId.value || !selectedRoleIds.value.length) return;

  saving.value = true;
  try {
    const res = await copyProjectMembers(props.wsId, {
      sourceWsId: Number(sourceWsId.value),
      roleIds: selectedRoleIds.value.map((id) => Number(id)),
    });
    const added = Number(res?.added_members || 0);
    message.success(
      t("workspace_copySuccess", {
        members: added,
        roles: Number(res?.created_roles || 0),
      }),
    );
    emit("copied", added);
    handleOpenChange(false);
  } finally {
    saving.value = false;
  }
}

function handleOpenChange(value) {
  emit("update:open", value);
}

watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    sourceWsId.value = "";
    selectedRoleIds.value = [];
    sourceRoles.value = [];
    await loadProjects();
  },
);

// Roles belong to the chosen source project, so the selection resets on every switch.
watch(sourceWsId, async (wsId) => {
  selectedRoleIds.value = [];
  sourceRoles.value = [];
  if (wsId) await loadSourceRoles();
});
</script>

<style scoped lang="less">
.copy-section-title {
  margin-bottom: 8px;
  color: #182b50;
  font-weight: 600;
}

.copy-section-title-roles {
  margin-top: 16px;
  border-top: 1px solid #eef1f6;
  padding-top: 16px;
}

.copy-project-group,
.copy-role-group {
  display: flex;
  width: 100%;
  max-height: 220px;
  flex-direction: column;
  // The groups scroll vertically; antd's default wrap would spill items into extra columns.
  flex-wrap: nowrap;
  overflow: auto;
  gap: 8px;
}

.copy-project-item,
.copy-role-item {
  display: flex;
  align-items: flex-start;
  margin: 0;
  border: 1px solid #eef1f6;
  border-radius: 8px;
  padding: 8px 12px;
}

.copy-project-name,
.copy-role-name {
  color: #182b50;
  font-size: 14px;
}

.copy-project-desc,
.copy-role-desc {
  color: #8c98ad;
  font-size: 12px;
}

.copy-role-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.copy-role-icon {
  color: #722ed1;
}

.copy-role-count {
  color: #8c98ad;
  font-size: 12px;
}
</style>
