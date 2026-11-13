<template>
  <div class="roles-wrap">
    <!-- Left role list -->
    <div class="roles-left">
      <div class="roles-left-hd">
        <h5>
          {{
            isEnterpriseMode
              ? $t("workspace_enterpriseRoles")
              : $t("workspace_projectRoles")
          }}
        </h5>
        <a-tooltip v-if="canCreateRole" :title="$t('workspace_newRole')">
          <a-button class="create-role-button" @click="openCreate"
            ><PlusOutlined
          /></a-button>
        </a-tooltip>
      </div>
      <div class="roles-list">
        <div
          v-for="r in roles"
          :key="r.id"
          class="role-item"
          :class="{ active: r.id === selectedId }"
          @click="selectRole(r)"
        >
          <div class="role-item-top">
            <div class="role-name-wrap">
              <strong>{{ getRoleDisplayName(r.name) }}</strong>
              <span v-if="Number(r.is_system) === 1" class="preset-tag">{{
                $t("workspace_presetRole")
              }}</span>
            </div>
            <div class="role-item-actions">
              <span class="member-pill"
                ><TeamOutlined />{{
                  $t("workspace_roleMemberCount", {
                    n: Number(r.member_count) || 0,
                  })
                }}</span
              >
              <a-tooltip
                v-if="canEditRole && Number(r.is_system) !== 1"
                :title="$t('common_edit')"
              >
                <a-button
                  type="text"
                  class="edit-role-button"
                  @click.stop="openEditRole(r)"
                  ><EditOutlined
                /></a-button>
              </a-tooltip>
              <a-tooltip
                v-if="canDeleteRole && Number(r.is_system) !== 1"
                :title="$t('common_delete')"
              >
                <a-button
                  type="text"
                  class="delete-role-button"
                  @click.stop="handleDelete(r)"
                  ><DeleteOutlined
                /></a-button>
              </a-tooltip>
            </div>
          </div>
          <div class="role-item-desc">
            {{ getRoleDescription(r.description) || "-" }}
          </div>
        </div>
      </div>
    </div>
    <!-- Right-side config -->
    <div class="roles-right" v-if="selectedRole">
      <div class="roles-right-fixed">
        <div class="roles-right-hd">
          <div class="selected-role-info">
            <div class="selected-role-title">
              <h5>{{ getRoleDisplayName(selectedRole.name) }}</h5>
              <span
                v-if="Number(selectedRole.is_system) === 1"
                class="preset-tag"
                >{{ $t("workspace_presetRole") }}</span
              >
            </div>
            <div class="selected-role-meta">
              <span
                ><TeamOutlined />
                {{
                  $t("workspace_roleMemberCount", {
                    n: Number(selectedRole.member_count) || 0,
                  })
                }}</span
              >
              <span v-if="getRoleDescription(selectedRole.description)">{{
                getRoleDescription(selectedRole.description)
              }}</span>
              <span
                v-if="
                  Number(selectedRole.is_system) === 1 &&
                  !isRoleEditable(selectedRole)
                "
                class="preset-hint"
                >· {{ $t("workspace_presetRoleHint") }}</span
              >
            </div>
          </div>
          <div
            v-if="canEditRole && isRoleEditable(selectedRole)"
            class="role-config-actions"
          >
            <a-button
              v-if="
                !isEnterpriseMode &&
                canCreateRole &&
                !isPresetMemberRole(selectedRole)
              "
              type="link"
              class="copy-role-button"
              @click="openCopyRole"
            >
              {{ $t("workspace_copy_role_to_other_projects") }}
            </a-button>
            <a-button
              type="primary"
              class="save-role-button"
              :loading="saving"
              @click="saveRoleConfig"
              >{{
                isEnterpriseMode
                  ? $t("workspace_savePermission")
                  : $t("workspace_savePermissionWithCount", {
                      n: checkedPerms.length,
                    })
              }}</a-button
            >
          </div>
        </div>

        <!-- Tab navigation -->
        <div class="enterprise-config-tabs">
          <div class="permission-tab">
            <button
              type="button"
              :class="{ active: configTab === 'permissions' }"
              @click="configTab = 'permissions'"
            >
              {{ $t("workspace_functionPermissions") }}
            </button>
          </div>
          <button
            type="button"
            :class="{ active: configTab === 'roleMembers' }"
            @click="configTab = 'roleMembers'"
          >
            {{ $t("workspace_roleMembersTab") }}
          </button>
          <button
            v-if="isEnterpriseMode"
            type="button"
            :class="{ active: configTab === 'teamWorkScope' }"
            @click="configTab = 'teamWorkScope'"
          >
            {{ $t("workspace_teamWorkViewScope") }}
          </button>
          <button
            v-else
            type="button"
            :class="{ active: configTab === 'dataViewScope' }"
            @click="configTab = 'dataViewScope'"
          >
            {{ $t("workspace_dataViewScope") }}
          </button>
          <a-button
            v-if="
              canAddMembers &&
              (configTab === 'permissions' || configTab === 'roleMembers')
            "
            class="add-member-button"
            type="primary"
            ghost
            :loading="refreshingMembers"
            @click="openAddMembers"
          >
            <template #icon><PlusOutlined /></template>
            {{ $t("workspace_addMember") }}
          </a-button>
        </div>
      </div>

      <div class="roles-right-body">
        <div class="select-all-block">
          <a-checkbox
            v-if="configTab === 'permissions' && !isRoleReadonly"
            class="permission-select-all"
            :checked="allPermissionsSelected"
            :indeterminate="allPermissionsIndeterminate"
            :disabled="allPermissionKeys.length === 0"
            @change="toggleAllPermissions($event.target.checked)"
          >
            {{ $t("common_all") }}
          </a-checkbox>
        </div>

        <!-- Function permissions tab -->
        <RolePermissionSelector
          v-if="configTab === 'permissions'"
          v-model="checkedPerms"
          :groups="groupedPerms"
          :readonly="isRoleReadonly"
          :enterprise-mode="isEnterpriseMode"
        />

        <!-- Enterprise role member list tab -->
        <RoleMembersPanel
          v-else-if="configTab === 'roleMembers' && isEnterpriseMode"
          :role="selectedRole"
          :members="enterpriseMembers"
          :roles="roles"
          :departments="departments"
          @changed="loadData(selectedId)"
        />

        <!-- Project role member list tab: the project member payload carries no departments -->
        <ProjectRoleMembersPanel
          v-else-if="configTab === 'roleMembers'"
          :role="selectedRole"
          :members="projectMembers"
          :roles="roles"
          :ws-id="currentWsId"
          @changed="loadData(selectedId)"
        />

        <!-- Enterprise team work scope tab -->
        <div
          v-else-if="configTab === 'teamWorkScope'"
          class="team-work-scope-card"
        >
          <a-radio-group
            v-model:value="teamWorkViewScope"
            :disabled="isRoleReadonly"
            class="scope-radio-group"
          >
            <a-radio value="all">{{ $t("workspace_scopeAllMembers") }}</a-radio>
            <a-radio value="partial">{{
              $t("workspace_scopePartialMembers")
            }}</a-radio>
            <a-radio value="self">{{ $t("workspace_scopeSelf") }}</a-radio>
          </a-radio-group>

          <template v-if="teamWorkViewScope === 'partial'">
            <div class="scope-list-toolbar">
              <a-input
                v-model:value="scopeSearch"
                allow-clear
                autocomplete="off"
                class="scope-search"
                :placeholder="$t('workspace_scopeSearchPlaceholder')"
              >
                <template #suffix><SearchOutlined /></template>
              </a-input>
              <a-button
                v-if="!isRoleReadonly"
                type="primary"
                class="scope-add-button"
                @click="scopeModalOpen = true"
              >
                <PlusOutlined /> {{ $t("common_add") }}
              </a-button>
            </div>
            <a-table
              :columns="scopeColumns"
              :data-source="filteredScopeEntries"
              row-key="key"
              :pagination="false"
              :locale="{ emptyText: scopeEmptyText }"
              class="scope-table"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'name'">
                  <span class="scope-entry-name">
                    <ApartmentOutlined
                      v-if="record.type === 'dept'"
                      class="scope-entry-icon scope-entry-icon-dept"
                    />
                    <UserOutlined
                      v-else-if="record.type === 'user'"
                      class="scope-entry-icon scope-entry-icon-user"
                    />
                    <TeamOutlined
                      v-else
                      class="scope-entry-icon scope-entry-icon-role"
                    />
                    {{ record.name }}
                  </span>
                </template>
                <template v-else-if="column.key === 'type'">
                  <a-tag
                    :class="['scope-type-tag', `scope-type-${record.type}`]"
                    >{{ scopeTypeLabel(record.type) }}</a-tag
                  >
                </template>
                <template v-else-if="column.key === 'action'">
                  <a-button
                    v-if="!isRoleReadonly"
                    type="link"
                    danger
                    size="small"
                    @click="removeScopeEntry(record)"
                    >{{ $t("common_delete") }}</a-button
                  >
                </template>
              </template>
            </a-table>
          </template>
        </div>

        <!-- Project list data view scope tab -->
        <div v-else class="team-work-scope-card">
          <a-radio-group
            v-model:value="dataViewScope"
            :disabled="isRoleReadonly"
            class="scope-radio-group"
          >
            <a-radio value="all">{{ $t("workspace_scopeAllMembers") }}</a-radio>
            <a-radio value="partial">{{
              $t("workspace_scopePartialMembers")
            }}</a-radio>
            <a-radio value="self">{{ $t("workspace_scopeSelf") }}</a-radio>
          </a-radio-group>

          <template v-if="dataViewScope === 'partial'">
            <div class="scope-member-toolbar">
              <a-checkbox
                :checked="allDataViewMembersSelected"
                :indeterminate="dataViewMembersIndeterminate"
                :disabled="isRoleReadonly"
                class="scope-member-select-all"
                @change="toggleAllDataViewMembers($event.target.checked)"
              >
                {{ $t("common_selectAll") }}
              </a-checkbox>
            </div>
            <a-checkbox-group
              v-model:value="dataViewMemberIds"
              :disabled="isRoleReadonly"
              class="scope-member-group"
            >
              <div class="scope-member-grid">
                <a-checkbox
                  v-for="member in projectMembers"
                  :key="member.user_id"
                  :value="Number(member.user_id)"
                  class="scope-member-item"
                >
                  <a-avatar
                    :size="34"
                    :src="getMemberAvatar(member)"
                    :style="
                      getMemberAvatar(member)
                        ? avatarImgStyle
                        : avatarFallbackStyle
                    "
                    class="scope-member-avatar"
                  >
                    {{ getInitial(member) }}
                  </a-avatar>
                  <span class="scope-member-info">
                    <strong>{{
                      member.user_name || member.user_username
                    }}</strong>
                    <small>{{ member.user_username }}</small>
                  </span>
                </a-checkbox>
              </div>
            </a-checkbox-group>
          </template>
        </div>
      </div>
    </div>

    <AddProjectMembersModal
      v-if="!isEnterpriseMode"
      :key="currentWsId"
      v-model:open="addMemberOpen"
      :ws-id="currentWsId"
      :role-id="addMemberRoleId"
      @added="refreshProjectMembers"
    />

    <!-- Team work view scope picker -->
    <AddViewScopeModal
      v-model:open="scopeModalOpen"
      :enterprise-name="enterpriseName"
      :departments="departments"
      :members="enterpriseMembers"
      :roles="pickableScopeRoles"
      :existing-targets="teamWorkViewTargets"
      @confirm="applyScopeTargets"
    />

    <!-- Copy Role Dialog -->
    <a-modal
      v-model:open="copyOpen"
      :title="
        $t('workspace_copy_role_title', {
          name: getRoleDisplayName(selectedRole?.name),
        })
      "
      :ok-text="$t('common_copy')"
      :cancel-text="$t('common_cancel')"
      :confirm-loading="copying"
      :ok-button-props="{ disabled: copyTargetIds.length === 0 }"
      :destroy-on-close="true"
      @ok="handleCopyRole"
    >
      <a-spin :spinning="copyTargetsLoading">
        <p class="copy-role-hint">{{ $t("workspace_copy_role_hint") }}</p>
        <a-alert
          v-if="copyError"
          type="error"
          show-icon
          :message="copyError"
          class="copy-role-error"
        />
        <a-empty
          v-if="copyTargets.length === 0"
          :description="$t('workspace_copy_role_empty')"
        />
        <a-checkbox-group
          v-else
          v-model:value="copyTargetIds"
          class="copy-role-list"
        >
          <a-checkbox
            v-for="workspace in copyTargets"
            :key="workspace.id"
            :value="Number(workspace.id)"
            class="copy-role-item"
          >
            {{ workspace.name }}
          </a-checkbox>
        </a-checkbox-group>
      </a-spin>
    </a-modal>

    <!-- Edit Role Dialog -->
    <a-modal
      v-model:open="editRoleOpen"
      :title="$t('workspace_editRole')"
      @ok="handleEditRole"
      :ok-text="$t('common_save')"
      :cancel-text="$t('common_cancel')"
      :confirm-loading="editingRole"
      :destroy-on-close="true"
    >
      <a-form layout="vertical" style="margin-top: 12px" :model="editRoleForm">
        <a-form-item
          name="name"
          :label="$t('workspace_roleName')"
          :rules="[
            { required: true, message: $t('workspace_roleNameRequired') },
            { max: 20, message: $t('workspace_roleNameMaxChars') },
          ]"
        >
          <a-input
            v-model:value="editRoleForm.name"
            :placeholder="$t('workspace_roleNamePlaceholder')"
            :maxlength="20"
          />
        </a-form-item>
        <a-form-item
          name="description"
          :label="$t('workspace_roleDescription')"
          :rules="[{ max: 20, message: $t('workspace_roleNameMaxChars') }]"
        >
          <a-textarea
            v-model:value="editRoleForm.description"
            :placeholder="$t('workspace_roleDescPlaceholder')"
            :maxlength="20"
            :rows="2"
            show-count
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- New Role Dialog -->
    <a-modal
      v-model:open="createOpen"
      :title="$t('workspace_newRole')"
      @ok="handleCreate"
      :ok-text="$t('common_create')"
      :cancel-text="$t('common_cancel')"
      :destroy-on-close="true"
    >
      <a-form layout="vertical" style="margin-top: 12px" :model="createForm">
        <a-form-item
          name="name"
          :label="$t('workspace_roleName')"
          :rules="[
            { required: true, message: $t('workspace_roleNameRequired') },
            { max: 20, message: $t('workspace_roleNameMaxChars') },
          ]"
        >
          <a-input
            v-model:value="createForm.name"
            :placeholder="$t('workspace_roleNamePlaceholder')"
            :maxlength="20"
          />
        </a-form-item>
        <a-form-item
          name="description"
          :label="$t('workspace_roleDescription')"
          :rules="[{ max: 20, message: $t('workspace_roleNameMaxChars') }]"
        >
          <a-textarea
            v-model:value="createForm.description"
            :placeholder="$t('workspace_roleDescPlaceholder')"
            :maxlength="20"
            :rows="2"
            show-count
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";
import { message, Modal } from "ant-design-vue";
import {
  ApartmentOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  SearchOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons-vue";
import {
  listProjectRoles,
  listEnterpriseRoles,
  createEnterpriseRole,
  createProjectRole,
  updateEnterpriseRole,
  updateProjectRole,
  updateEnterpriseRolePermissions,
  updateEnterpriseRoleTeamWorkScope,
  updateProjectRolePermissions,
  updateProjectRoleDataViewScope,
  listProjectRoleCopyTargets,
  copyProjectRole,
  deleteEnterpriseRole,
  deleteProjectRole,
  listEnterpriseDepartments,
} from "@/api/workspace";
import {
  listEnterpriseMembers,
  listPermissions,
  listMembers,
} from "@/api/workspace";
import RoleMembersPanel from "./components/RoleMembersPanel.vue";
import ProjectRoleMembersPanel from "./components/ProjectRoleMembersPanel.vue";
import RolePermissionSelector from "./components/RolePermissionSelector.vue";
import AddViewScopeModal from "./components/AddViewScopeModal.vue";
import AddProjectMembersModal from "./components/AddProjectMembersModal.vue";
import { useAuthStore } from "@/stores/auth";
import { getAvatarSrc as getAvatarSrcOf, getInitial } from "@/utils/avatar";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
});

const { t: $t } = useI18n();
const route = useRoute();
const authStore = useAuthStore();

const roles = ref([]);
const selectedRole = ref(null);
const selectedId = ref(0);
const checkedPerms = ref([]);
const createOpen = ref(false);
const createForm = ref({ name: "", description: "" });
const editRoleOpen = ref(false);
const editRoleForm = ref({ id: 0, name: "", description: "" });
const editingRole = ref(false);
const permissionGroups = ref([]);
const enterpriseMembers = ref([]);
const projectMembers = ref([]);
const addMemberOpen = ref(false);
const addMemberRoleId = ref("");
const refreshingMembers = ref(false);
const enterpriseName = ref("");
const departments = ref([]);
const configTab = ref("permissions");
const teamWorkViewScope = ref("self");
const teamWorkViewTargets = ref([]);
const scopeSearch = ref("");
const scopeModalOpen = ref(false);
const dataViewScope = ref("self");
const dataViewMemberIds = ref([]);
const loading = ref(false);
const saving = ref(false);
const copyOpen = ref(false);
const copyTargets = ref([]);
const copyTargetIds = ref([]);
const copyTargetsLoading = ref(false);
const copying = ref(false);
const copyError = ref("");

// Avatar: no blue background when an image is present; blue initial fallback when no image.
const avatarImgStyle = { backgroundColor: "transparent" };
const avatarFallbackStyle = { backgroundColor: "#1677ff" };

const isEnterpriseMode = computed(() => !props.wsId && !route.params.wsId);
const currentWsId = computed(() => `${props.wsId || route.params.wsId || ""}`);
const canAddMembers = computed(
  () =>
    !isEnterpriseMode.value && authStore.hasPermission("project_member:add"),
);
const canCreateRole = computed(() =>
  authStore.hasPermission(
    isEnterpriseMode.value ? "member:create_role" : "project_role:add",
  ),
);
const canEditRole = computed(() =>
  authStore.hasPermission(
    isEnterpriseMode.value
      ? "member:edit_role_permissions"
      : "project_role:edit",
  ),
);
const canDeleteRole = computed(() =>
  authStore.hasPermission(
    isEnterpriseMode.value ? "member:delete_role" : "project_role:delete",
  ),
);

const groupedPerms = computed(() => permissionGroups.value);
const allPermissionKeys = computed(() => [
  ...new Set(
    groupedPerms.value.flatMap((group) =>
      getPermissionItems(group).map(getPermissionKey).filter(Boolean),
    ),
  ),
]);
const allPermissionsSelected = computed(
  () =>
    allPermissionKeys.value.length > 0 &&
    allPermissionKeys.value.every((key) => checkedPerms.value.includes(key)),
);
const allPermissionsIndeterminate = computed(() => {
  const selectedCount = allPermissionKeys.value.filter((key) =>
    checkedPerms.value.includes(key),
  ).length;
  return selectedCount > 0 && selectedCount < allPermissionKeys.value.length;
});

function toggleAllPermissions(checked) {
  const next = new Set(checkedPerms.value);
  for (const key of allPermissionKeys.value) {
    if (checked) {
      next.add(key);
    } else {
      next.delete(key);
    }
  }
  checkedPerms.value = [...next];
}

// Whether the selected role allows editing. Project roles carry the backend-computed
// is_editable flag (custom roles + the preset project member role); enterprise roles
// fall back to the is_system check (custom roles only).
function isRoleEditable(role) {
  if (role?.is_editable !== undefined) {
    return Boolean(role.is_editable);
  }
  return Number(role?.is_system) !== 1;
}
// The preset project member role already exists in every project by default, so copying
// it to other projects is meaningless; the copy action is hidden for it.
// Backend returns preset role names already translated per language ("普通成员"/"Member"),
// so structural detection is used: among system project roles only the preset member role
// is editable (is_editable=true). Name matches are kept as a fallback for legacy backends.
function isPresetMemberRole(role) {
  if (!role) return false;
  const isSystem = Number(role.is_system) === 1;
  if (isSystem && Boolean(role.is_editable)) return true;
  const name = role.name || "";
  return (
    isSystem &&
    (name.includes("普通成员") ||
      name.includes("Member") ||
      name.includes("perm_role_project_member") ||
      name.includes("project_member"))
  );
}
const isRoleReadonly = computed(
  () => !isRoleEditable(selectedRole.value) || !canEditRole.value,
);

onMounted(async () => {
  await loadData();
});

watch(currentWsId, async () => {
  addMemberOpen.value = false;
  if (!isEnterpriseMode.value) {
    roles.value = [];
    selectedRole.value = null;
    await loadData();
  }
});

async function loadData(preferredRoleId = selectedId.value) {
  loading.value = true;
  try {
    if (isEnterpriseMode.value) {
      // Enterprise role mode: load enterprise roles + enterprise permission groups.
      const scope = "enterprise";
      const [roleList, permList, memberList, departmentData] =
        await Promise.all([
          listEnterpriseRoles(),
          listPermissions(scope),
          listEnterpriseMembers(),
          listEnterpriseDepartments(),
        ]);
      roles.value = roleList || [];
      permissionGroups.value = permList || [];
      enterpriseMembers.value = memberList || [];
      enterpriseName.value = departmentData?.enterprise_name || "";
      departments.value = departmentData?.items || [];
    } else {
      // Project role mode: load project roles + project permission groups + project members.
      const wsId = currentWsId.value;
      if (!wsId) {
        roles.value = [];
        permissionGroups.value = [];
        projectMembers.value = [];
        return;
      }
      const scope = "project";
      const [roleList, permList, memberList] = await Promise.all([
        listProjectRoles(wsId),
        listPermissions(scope, wsId),
        listMembers(wsId),
      ]);
      if (wsId !== currentWsId.value) return;
      roles.value = roleList || [];
      permissionGroups.value = permList || [];
      projectMembers.value = memberList || [];
    }

    // Keep the current role selected after saving or creating; default to the first role on first entry.
    if (roles.value.length > 0) {
      const nextRole =
        roles.value.find(
          (role) => Number(role.id) === Number(preferredRoleId),
        ) || roles.value[0];
      selectRole(nextRole);
    } else {
      selectedRole.value = null;
      selectedId.value = 0;
      checkedPerms.value = [];
      teamWorkViewScope.value = "self";
      teamWorkViewTargets.value = [];
      dataViewScope.value = "self";
      dataViewMemberIds.value = [];
    }
  } catch {
    /* handled */
  } finally {
    loading.value = false;
  }
}

function openAddMembers() {
  addMemberRoleId.value = selectedId.value;
  addMemberOpen.value = true;
}

async function refreshProjectMembers() {
  const wsId = currentWsId.value;
  refreshingMembers.value = true;
  try {
    const [members, projectRoles] = await Promise.all([
      listMembers(wsId),
      listProjectRoles(wsId),
    ]);
    if (wsId !== currentWsId.value) return;

    projectMembers.value = members || [];
    // Keep the current role's unsaved configuration while updating its count.
    const counts = new Map(
      projectRoles.map((role) => [role.id, role.member_count]),
    );
    for (const role of roles.value) {
      if (counts.has(role.id)) {
        role.member_count = counts.get(role.id);
      }
    }
  } catch {
    /* handled */
  } finally {
    refreshingMembers.value = false;
  }
}

function getRoleDisplayName(name) {
  return name || "-";
}

function getRoleDescription(description) {
  return description || "";
}

function getPermissionItems(group) {
  return (
    group.items || group.Items || group.permissions || group.Permissions || []
  );
}

function getPermissionKey(permission) {
  return permission.key || permission.Key || "";
}

function getMemberAvatar(member) {
  return getAvatarSrcOf(member);
}

function selectRole(r) {
  selectedId.value = r.id;
  selectedRole.value = r;
  // Use the permissions array returned by the backend.
  const perms = Array.isArray(r.permissions) ? r.permissions : [];
  checkedPerms.value = [...perms];
  teamWorkViewScope.value = r.team_work_view_scope || "self";
  teamWorkViewTargets.value = Array.isArray(r.team_work_view_targets)
    ? r.team_work_view_targets.map((target) => ({
        type: target.type,
        id: Number(target.id),
      }))
    : [];
  scopeSearch.value = "";
  // List data view scope: use the backend value; fall back to preset defaults for legacy data.
  const viewScope = r.data_view_scope || defaultDataViewScope(r);
  dataViewScope.value = viewScope;
  dataViewMemberIds.value = Array.isArray(r.data_view_member_ids)
    ? r.data_view_member_ids.map(Number)
    : [];
}

// Legacy fallback: project admins default to all members, other roles to self only.
function defaultDataViewScope(role) {
  if (Number(role.is_system) === 1) {
    const name = role.name || "";
    if (
      name.includes("perm_role_project_admin") ||
      name.includes("project_admin") ||
      name.includes("项目管理员")
    ) {
      return "all";
    }
  }
  return "self";
}

async function saveRoleConfig() {
  const r = roles.value.find((x) => x.id === selectedId.value);
  if (!r || !canEditRole.value || !isRoleEditable(r)) return;
  if (
    isEnterpriseMode.value &&
    teamWorkViewScope.value === "partial" &&
    scopeEntries.value.length === 0
  ) {
    message.warning($t("workspace_scopeTargetRequired"));
    configTab.value = "teamWorkScope";
    return;
  }
  if (
    !isEnterpriseMode.value &&
    dataViewScope.value === "partial" &&
    dataViewMemberIds.value.length === 0
  ) {
    message.warning($t("workspace_scopeMemberRequired"));
    configTab.value = "dataViewScope";
    return;
  }

  saving.value = true;
  try {
    if (isEnterpriseMode.value) {
      await Promise.all([
        updateEnterpriseRolePermissions(r.id, {
          permissions: checkedPerms.value,
        }),
        updateEnterpriseRoleTeamWorkScope(r.id, {
          viewScope: teamWorkViewScope.value,
          // Entries whose target no longer exists are dropped on save.
          targets: scopeEntries.value.map(({ type, id }) => ({ type, id })),
        }),
      ]);
    } else {
      // The permission page does not expose every project permission (e.g. workspace:view),
      // so keep the role's existing hidden permissions when saving.
      const visibleKeys = new Set(
        permissionGroups.value.flatMap((g) =>
          getPermissionItems(g).map((p) => getPermissionKey(p)),
        ),
      );
      const hiddenPerms = (r.permissions || []).filter(
        (k) => !visibleKeys.has(k),
      );
      const permissions = [...new Set([...checkedPerms.value, ...hiddenPerms])];
      await Promise.all([
        updateProjectRolePermissions(currentWsId.value, r.id, {
          permissions,
        }),
        updateProjectRoleDataViewScope(currentWsId.value, r.id, {
          viewScope: dataViewScope.value,
          memberIds: dataViewMemberIds.value,
        }),
      ]);
    }
    message.success($t("workspace_roleSaved"));
    await loadData(r.id);
  } catch {
    /* handled */
  } finally {
    saving.value = false;
  }
}

// ======================== Team work view scope entries ========================

const enterpriseMembersById = computed(() => {
  const map = {};
  for (const member of enterpriseMembers.value) map[String(member.id)] = member;
  return map;
});

const departmentsById = computed(() => {
  const map = {};
  for (const dept of departments.value) map[String(dept.id)] = dept;
  return map;
});

const rolesById = computed(() => {
  const map = {};
  for (const r of roles.value) map[String(r.id)] = r;
  return map;
});

// The role being configured cannot be part of its own view scope.
const pickableScopeRoles = computed(() =>
  roles.value.filter((r) => Number(r.id) !== Number(selectedId.value)),
);

function resolveScopeEntryName(target) {
  if (target.type === "dept") {
    return departmentsById.value[String(target.id)]?.name || "";
  }
  if (target.type === "role") {
    return rolesById.value[String(target.id)]?.name || "";
  }
  const member = enterpriseMembersById.value[String(target.id)];
  return member ? member.name || member.username : "";
}

// Entries whose target has been deleted resolve to no name and are hidden.
const scopeEntries = computed(() =>
  teamWorkViewTargets.value
    .map((target) => ({
      ...target,
      key: `${target.type}-${target.id}`,
      name: resolveScopeEntryName(target),
    }))
    .filter((entry) => entry.name),
);

const filteredScopeEntries = computed(() => {
  const text = scopeSearch.value.trim().toLowerCase();
  if (!text) return scopeEntries.value;
  return scopeEntries.value.filter((entry) =>
    entry.name.toLowerCase().includes(text),
  );
});

const scopeColumns = computed(() => [
  { title: $t("workspace_scopeColumnName"), key: "name" },
  { title: $t("workspace_scopeColumnType"), key: "type", width: 120 },
  { title: $t("member_action"), key: "action", width: 90 },
]);

const scopeEmptyText = computed(() =>
  scopeSearch.value.trim()
    ? $t("workspace_scopeNoMatch")
    : $t("workspace_scopeEmpty"),
);

function scopeTypeLabel(type) {
  if (type === "dept") return $t("workspace_scopeTypeDept");
  if (type === "role") return $t("workspace_scopeTypeRole");
  return $t("workspace_scopeTypeUser");
}

function removeScopeEntry(entry) {
  teamWorkViewTargets.value = teamWorkViewTargets.value.filter(
    (target) => !(target.type === entry.type && target.id === entry.id),
  );
}

async function openCopyRole() {
  const role = selectedRole.value;
  if (
    isEnterpriseMode.value ||
    !canEditRole.value ||
    !role ||
    Number(role.is_system) === 1
  )
    return;

  copyOpen.value = true;
  copyTargetIds.value = [];
  copyTargets.value = [];
  copyError.value = "";
  copyTargetsLoading.value = true;
  try {
    copyTargets.value =
      (await listProjectRoleCopyTargets(currentWsId.value)) || [];
  } catch {
    /* handled */
  } finally {
    copyTargetsLoading.value = false;
  }
}

async function handleCopyRole() {
  const role = selectedRole.value;
  if (!role || copyTargetIds.value.length === 0) {
    message.warning($t("workspace_copy_role_select_required"));
    return;
  }

  copying.value = true;
  copyError.value = "";
  try {
    await copyProjectRole(currentWsId.value, role.id, {
      workspaceIds: copyTargetIds.value,
    });
    message.success($t("workspace_copy_role_success"));
    copyOpen.value = false;
  } catch (error) {
    // Some versions leave HTTP 409 to the caller; always surface it in the dialog.
    copyError.value =
      error?.response?.data?.message ||
      error?.message ||
      $t("common_requestFailed");
  } finally {
    copying.value = false;
  }
}

// The modal is seeded with the current entries, so its result is the new full list.
function applyScopeTargets(targets) {
  teamWorkViewTargets.value = targets;
}

// Count only ids that still belong to the current project member list.
const dataViewSelectedCount = computed(() => {
  const ids = new Set(dataViewMemberIds.value.map(Number));
  return projectMembers.value.filter((member) =>
    ids.has(Number(member.user_id)),
  ).length;
});

const allDataViewMembersSelected = computed(
  () =>
    projectMembers.value.length > 0 &&
    dataViewSelectedCount.value === projectMembers.value.length,
);

const dataViewMembersIndeterminate = computed(
  () => dataViewSelectedCount.value > 0 && !allDataViewMembersSelected.value,
);

function selectAllDataViewMembers() {
  dataViewMemberIds.value = projectMembers.value.map((member) =>
    Number(member.user_id),
  );
}

function clearDataViewMembers() {
  dataViewMemberIds.value = [];
}

// The toolbar checkbox switches between every member and none.
function toggleAllDataViewMembers(checked) {
  if (checked) {
    selectAllDataViewMembers();
  } else {
    clearDataViewMembers();
  }
}

function handleDelete(r) {
  if (!canDeleteRole.value) return;
  if (Number(r.is_system) === 1) {
    message.info($t("workspace_presetRoleNoDelete"));
    return;
  }
  Modal.confirm({
    title: $t("workspace_deleteRole"),
    content: $t("workspace_deleteRoleConfirm", {
      name: getRoleDisplayName(r.name),
    }),
    okText: $t("common_delete"),
    okButtonProps: { danger: true },
    cancelText: $t("common_cancel"),
    onOk: async () => {
      try {
        if (isEnterpriseMode.value) {
          await deleteEnterpriseRole(r.id);
        } else {
          await deleteProjectRole(currentWsId.value, r.id);
        }
        message.success($t("workspace_roleDeleted"));
        await loadData();
      } catch {
        /* handled */
      }
    },
  });
}

function openEditRole(r) {
  if (!canEditRole.value) return;
  editRoleForm.value = {
    id: r.id,
    name: getRoleDisplayName(r.name) === "-" ? "" : r.name || "",
    description: getRoleDescription(r.description),
  };
  editRoleOpen.value = true;
}

async function handleEditRole() {
  if (!canEditRole.value) return;
  const name = (editRoleForm.value.name || "").trim();
  const description = (editRoleForm.value.description || "").trim();
  if (!name) {
    message.warning($t("workspace_roleNameRequired"));
    return;
  }
  if (description.length > 20) {
    message.warning($t("workspace_roleNameMaxChars"));
    return;
  }

  editingRole.value = true;
  try {
    const roleId = editRoleForm.value.id;
    if (isEnterpriseMode.value) {
      await updateEnterpriseRole(roleId, { name, description });
    } else {
      await updateProjectRole(currentWsId.value, roleId, { name, description });
    }
    message.success($t("workspace_roleSaved"));
    editRoleOpen.value = false;
    await loadData(roleId);
  } catch {
    /* handled */
  } finally {
    editingRole.value = false;
  }
}

function openCreate() {
  if (!canCreateRole.value) return;
  createForm.value.name = "";
  createForm.value.description = "";
  createOpen.value = true;
}

async function handleCreate() {
  if (!canCreateRole.value) return;
  if (!createForm.value.name) {
    message.warning($t("workspace_roleNameRequired"));
    return;
  }
  if ((createForm.value.description || "").length > 20) {
    message.warning($t("workspace_roleNameMaxChars"));
    return;
  }
  try {
    let result;
    if (isEnterpriseMode.value) {
      result = await createEnterpriseRole({
        name: createForm.value.name,
        description: createForm.value.description || "",
        permissions: ["project:create"],
      });
    } else {
      result = await createProjectRole(currentWsId.value, {
        name: createForm.value.name,
        description: createForm.value.description || "",
        // Per requirement, gantt view access is on by default for every project role.
        permissions: ["gantt:view"],
      });
    }
    message.success($t("workspace_roleCreated"));
    createOpen.value = false;
    await loadData(result?.roleId);
  } catch {
    /* handled */
  }
}
</script>

<style scoped lang="less">
.roles-wrap {
  display: flex;
  min-height: 0;
  height: 100%;
  color: #182b50;
}

.roles-left {
  display: flex;
  width: 280px;
  flex-shrink: 0;
  flex-direction: column;
  overflow: hidden;
  padding: 0 12px 16px;
  border-right: 1px solid #ebeef5;
}

.roles-left-hd {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  padding: 4px 0 16px 4px;

  h5 {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }
}

.create-role-button {
  display: inline-flex;
  width: 34px;
  height: 34px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border-color: #dbe1ea;
  border-radius: 8px;
  color: #657188;
  box-shadow: 0 2px 5px rgba(24, 43, 80, 0.08);
}

.roles-list {
  flex: 1;
  overflow-y: auto;
}

.role-item {
  position: relative;
  margin-bottom: 8px;
  padding: 13px 16px;
  cursor: pointer;
  border: 1px solid #e5e9f0;
  border-radius: 10px;
  background: #fff;
  transition:
    border-color 0.15s,
    background 0.15s,
    box-shadow 0.15s;

  &::before {
    position: absolute;
    top: 13px;
    bottom: 13px;
    left: -1px;
    width: 3px;
    border-radius: 0 3px 3px 0;
    background: transparent;
    content: "";
  }

  &:hover {
    background: #fafcff;
  }

  &.active {
    border-color: #0d68ff;
    background: #f9fbff;
    box-shadow: 0 4px 12px rgba(13, 104, 255, 0.08);

    &::before {
      background: #0d68ff;
    }

    strong {
      color: #0d68ff;
    }
  }
}

.role-item-top,
.role-item-actions,
.role-name-wrap,
.selected-role-title,
.selected-role-meta {
  display: flex;
  align-items: center;
}

.role-item-top {
  justify-content: space-between;
  gap: 8px;
}

.role-name-wrap {
  min-width: 0;
  gap: 7px;

  strong {
    overflow: hidden;
    font-size: 14px;
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.preset-tag {
  flex: none;
  padding: 1px 7px;
  border-radius: 4px;
  color: #8a94a6;
  background: #f0f2f5;
  font-size: 10px;
  line-height: 18px;
}

.role-item-actions {
  flex: none;
  gap: 4px;
}

.member-pill {
  display: inline-flex;
  height: 22px;
  align-items: center;
  gap: 3px;
  padding: 0 8px;
  border-radius: 11px;
  color: #8a94a6;
  background: #eef2f7;
  font-size: 11px;
}

.edit-role-button,
.delete-role-button {
  width: 22px;
  height: 22px;
  padding: 0;
  color: #9aa5b6;
  opacity: 0;
  display: none;
  visibility: hidden;
  transition:
    opacity 0.15s,
    visibility 0.15s,
    color 0.15s,
    background 0.15s;

  &:hover {
    color: #0d68ff !important;
    background: #f0f7ff !important;
  }
}

.delete-role-button:hover {
  color: #ff4d4f !important;
  background: #fff1f0 !important;
}

.role-item:hover .edit-role-button,
.role-item:hover .delete-role-button {
  opacity: 1;
  visibility: visible;
  display: block;
}

.role-item-desc {
  overflow: hidden;
  margin-top: 6px;
  color: #98a3b5;
  font-size: 12px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

// The header and tab bar stay pinned while only the config body scrolls.
.roles-right {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  overflow: hidden;
}

.roles-right-fixed {
  flex: none;
  z-index: 5;
  padding: 0 28px;
  background: #fff;
}

.roles-right-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 28px 24px;
}

.roles-right-hd {
  display: flex;
  min-height: 72px;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 4px 0 20px;
  background: #fff;
}

.selected-role-title {
  gap: 8px;

  h5 {
    margin: 0;
    font-size: 17px;
    font-weight: 600;
  }
}

.selected-role-meta {
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 7px;
  color: #8a94a6;
  font-size: 12px;
  line-height: 18px;
}

.preset-hint {
  color: #d8952e;
}

.role-config-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.copy-role-button {
  padding: 0 4px;
}

.save-role-button {
  height: 34px;
  border-radius: 7px;
  background: #0d68ff;
}

.copy-role-error {
  margin-bottom: 12px;
}
.copy-role-hint {
  margin-bottom: 16px;
  color: #8a94a6;
}

.copy-role-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.copy-role-item {
  width: 100%;
  margin: 0;
  padding: 10px 12px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
}

.select-all-block {
  margin-bottom: 8px;
}

.enterprise-config-tabs {
  display: flex;
  height: 42px;
  align-items: stretch;
  gap: 28px;
  margin-bottom: 20px;
  border-bottom: 1px solid #ebeef5;

  .permission-tab {
    display: flex;
    align-items: stretch;
    gap: 12px;
  }

  > button:not(.add-member-button),
  .permission-tab > button {
    position: relative;
    padding: 0;
    border: 0;
    color: #4a5670;
    background: transparent;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;

    &::after {
      position: absolute;
      right: 0;
      bottom: -1px;
      left: 0;
      height: 2px;
      background: transparent;
      content: "";
    }

    &.active {
      color: #0d68ff;
      font-weight: 600;

      &::after {
        background: #0d68ff;
      }
    }
  }

  .add-member-button {
    align-self: center;
    margin-inline-start: auto;
  }
}

.team-work-scope-card {
  padding: 24px 30px 26px;
  border: 1px solid #e5e9f0;
  border-radius: 8px;
}

.scope-list-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 0 16px;
  border-top: 1px solid #ebeef5;
}

.scope-search {
  width: 260px;
  border-radius: 6px;
}

.scope-add-button {
  border-radius: 6px;
}

.scope-entry-name {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #253858;
  font-size: 14px;
}

.scope-entry-icon {
  font-size: 15px;

  &.scope-entry-icon-dept {
    color: #0d68ff;
  }

  &.scope-entry-icon-user {
    color: #13c2c2;
  }

  &.scope-entry-icon-role {
    color: #722ed1;
  }
}

.scope-type-tag {
  margin: 0;
  border-radius: 4px;

  &.scope-type-dept {
    border-color: #91caff;
    color: #0d68ff;
    background: #f0f7ff;
  }

  &.scope-type-user {
    border-color: #87e8de;
    color: #08979c;
    background: #e6fffb;
  }

  &.scope-type-role {
    border-color: #d3adf7;
    color: #722ed1;
    background: #f9f0ff;
  }
}

.scope-table {
  :deep(.ant-table-thead > tr > th) {
    color: #6b7690;
    background: #fafbfd;
    font-weight: 500;
  }
}

.scope-radio-group {
  display: flex;
  gap: 24px;
  padding-bottom: 20px;

  :deep(.ant-radio-wrapper) {
    margin: 0;
    color: #253858;
  }
}

.scope-member-toolbar {
  display: flex;
  align-items: center;
  padding: 14px 2px;
  border-top: 1px solid #ebeef5;
}

.scope-member-select-all {
  color: #253858;
  font-weight: 500;
}

.scope-member-group {
  display: block;
}

.scope-member-grid {
  display: grid;
  // auto-fill lets the column count adapt to the container width, so the grid
  // never overflows the card on narrow screens; min(180px, 100%) guards the
  // degenerate case where even one column does not fit.
  grid-template-columns: repeat(auto-fill, minmax(min(180px, 100%), 1fr));
  gap: 10px;
}

.scope-member-item {
  display: flex;
  min-width: 0;
  height: 58px;
  align-items: center !important;
  padding: 8px 12px;
  border: 1px solid #e5e9f0;
  border-radius: 7px;
  background: #fff;
  cursor: pointer;
  line-height: 1;

  :deep(.ant-checkbox) {
    flex: none;
  }

  // Ant Checkbox wraps slot content in an extra span, so layout of the avatar and member info must happen at this level.
  :deep(.ant-checkbox + span) {
    display: flex;
    min-width: 0;
    flex: 1;
    align-items: center;
    gap: 10px;
    padding-inline-start: 10px;
  }
}

.scope-member-avatar {
  flex: none;
  color: #fff;
  background: #1677ff;
}

.scope-member-info {
  display: flex;
  min-width: 0;
  flex-direction: column;

  strong,
  small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  strong {
    color: #253858;
    font-size: 14px;
    font-weight: 500;
    line-height: 20px;
  }

  small {
    color: #98a3b5;
    font-size: 11px;
    line-height: 16px;
  }
}

@media (max-width: 980px) {
  .roles-left {
    width: 250px;
  }
}
</style>
