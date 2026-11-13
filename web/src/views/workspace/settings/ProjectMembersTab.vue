<!--
 * ProjectMembersTab.vue
 * Project settings > project members: member list, role changes, removal, plus the
 * add-by-role and copy-from-another-project entry points.
 -->
<template>
  <div class="project-members-tab">
    <div class="tab-hd">
      <h5 class="tab-title">
        {{ $t("workspace_memberCount", { n: members.length }) }}
      </h5>
      <a-space>
        <a-button v-if="canCopy" @click="copyOpen = true">
          <CopyOutlined /> {{ $t("workspace_copyFromProject") }}
        </a-button>
        <a-dropdown v-if="canAdd" :trigger="['hover']">
          <a-button type="primary">
            <PlusOutlined /> {{ $t("workspace_addMember") }}
          </a-button>
          <template #overlay>
            <a-menu @click="({ key }) => openAdd(key)">
              <a-menu-item v-for="role in projectRoles" :key="String(role.id)">
                {{
                  $t("workspace_addRoleMember", {
                    role: getRoleDisplayName(role.name),
                  })
                }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </a-space>
    </div>

    <div class="member-table-scroll">
      <a-table
        :columns="cols"
        :data-source="members"
        :loading="loading"
        row-key="user_id"
        :pagination="false"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'user'">
            <a-space>
              <a-avatar
                :size="32"
                :src="getAvatarSrc(record)"
                :style="
                  getAvatarSrc(record) ? avatarImgStyle : avatarFallbackStyle
                "
                class="member-avatar"
              >
                {{ getInitial(record) }}
              </a-avatar>
              <div>
                <div class="member-name">
                  {{ record.user_name || "-" }}
                  <a-tag
                    v-if="isMemberDisabled(record)"
                    class="member-disabled-tag"
                  >
                    {{ $t("member_disabledTag") }}
                  </a-tag>
                </div>
                <span class="member-username">{{
                  record.user_username || "-"
                }}</span>
              </div>
            </a-space>
          </template>

          <template v-else-if="column.key === 'role'">
            <a-tag :color="isProjectAdmin(record) ? 'blue' : 'default'">
              {{ getRoleDisplayName(record.role_name) }}
            </a-tag>
          </template>

          <template v-else-if="column.key === 'action'">
            <a-space
              v-if="
                canChangeRole(record) ||
                canRemoveMember(record) ||
                canTransferOwner(record)
              "
            >
              <a-dropdown v-if="canChangeRole(record)" trigger="click">
                <a-button size="small">{{
                  $t("workspace_modifyRole")
                }}</a-button>
                <template #overlay>
                  <a-menu @click="({ key }) => handleRole(record, key)">
                    <a-menu-item
                      v-for="role in projectRoles"
                      :key="String(role.id)"
                      :disabled="Number(role.id) === Number(record.role_id)"
                    >
                      {{ getRoleDisplayName(role.name) }}
                    </a-menu-item>
                  </a-menu>
                </template>
              </a-dropdown>

              <a-button
                v-if="canTransferOwner(record)"
                size="small"
                danger
                @click="openTransferOwner"
              >
                {{ $t("owner_transfer") }}
              </a-button>

              <a-button
                v-if="canRemoveMember(record)"
                size="small"
                danger
                @click="openRemoveMember(record)"
              >
                {{ $t("common_remove") }}
              </a-button>
            </a-space>
          </template>
        </template>
      </a-table>
    </div>

    <AddProjectMembersModal
      v-model:open="addOpen"
      :ws-id="currentWsId"
      :role-id="addRoleId"
      @added="fetchData"
    />
    <CopyProjectMembersModal
      v-model:open="copyOpen"
      :ws-id="currentWsId"
      @copied="fetchData"
    />

    <!-- Transfer the default project admin role -->
    <a-modal
      v-model:open="transferOpen"
      :title="$t('owner_transferProjectTitle')"
      :confirm-loading="transferSubmitting"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      @ok="submitTransferOwner"
    >
      <p class="transfer-tip">{{ $t("owner_transferProjectTip") }}</p>
      <p class="transfer-tip">{{ $t("owner_transferProjectDemoteTip") }}</p>
      <div class="transfer-row">
        <span class="transfer-label">{{ $t("owner_transferTo") }}</span>
        <a-select
          v-model:value="transferTargetId"
          class="transfer-select"
          show-search
          option-filter-prop="label"
          :placeholder="$t('owner_transferSelectPlaceholder')"
          :options="transferCandidates"
        />
      </div>
    </a-modal>

    <!-- Remove a project member, optionally handing over their unfinished work items -->
    <a-modal
      v-model:open="removeOpen"
      :title="$t('workspace_confirmRemoveMember')"
      :confirm-loading="removeSubmitting"
      :ok-text="$t('common_remove')"
      :ok-button-props="{ danger: true }"
      :cancel-text="$t('common_cancel')"
      @ok="submitRemoveMember"
    >
      <p class="transfer-tip">{{ $t("workspace_removeProjectMemberDesc") }}</p>
      <a-spin :spinning="removePreviewLoading">
        <div v-if="removeUnfinishedCount > 0" class="transfer-row">
          <a-checkbox v-model:checked="removeTransferItems">
            {{
              $t("owner_transferUnfinishedItems", { n: removeUnfinishedCount })
            }}
          </a-checkbox>
          <span class="transfer-label">{{ $t("owner_transferTo") }}</span>
          <a-select
            v-model:value="removeItemsTargetId"
            class="transfer-select"
            show-search
            option-filter-prop="label"
            :disabled="!removeTransferItems"
            :placeholder="$t('owner_transferSelectPlaceholder')"
            :options="removeCandidates"
          />
        </div>
      </a-spin>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { CopyOutlined, PlusOutlined } from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import { getAvatarSrc, getInitial } from "@/utils/avatar";
import {
  listMembers,
  listProjectRoles,
  previewWorkspaceMemberRemoval,
  removeMember,
  transferWorkspaceOwner,
  updateMemberRole,
} from "@/api/workspace";
import AddProjectMembersModal from "./components/AddProjectMembersModal.vue";
import CopyProjectMembersModal from "./components/CopyProjectMembersModal.vue";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
});

const route = useRoute();
const { t: $t } = useI18n();
const authStore = useAuthStore();
const currentWsId = computed(() =>
  String(props.wsId || route.params.wsId || ""),
);

// Avatar style: no blue background when an image is present (avoids a blue border showing through); show a blue initial fallback when no image.
const avatarImgStyle = { backgroundColor: "transparent" };
const avatarFallbackStyle = { backgroundColor: "#1677ff" };

const loading = ref(false);
const members = ref([]);
const projectRoles = ref([]);
const addOpen = ref(false);
const addRoleId = ref("");
const copyOpen = ref(false);

const cols = computed(() => [
  { title: $t("member_member"), key: "user", width: 280 },
  { title: $t("member_role"), key: "role", width: 160 },
  { title: $t("member_action"), key: "action", width: 220 },
]);

const canAdd = computed(() => authStore.hasPermission("project_member:add"));
// Copying also creates the missing project roles, so it needs the role permission too.
const canCopy = computed(
  () => canAdd.value && authStore.hasPermission("project_role:add"),
);

function getRoleDisplayName(name) {
  return name || "-";
}

function isProjectAdmin(record) {
  return Number(record.role_id) === Number(projectRoles.value[0]?.id);
}

// Disabled members stay listed here (with a tag) so admins can still see and manage them.
function isMemberDisabled(record) {
  return String(record?.user_status) === "0";
}

function canChangeRole(record) {
  if (record.is_owner) return false;
  return (
    authStore.hasPermission("project_member:change_role") &&
    Number(record.user_id) !== Number(authStore.userInfo?.id)
  );
}

// The default project admin cannot be removed; their role has to be transferred first.
function canRemoveMember(record) {
  if (record.is_owner) return false;
  return (
    authStore.hasPermission("project_member:remove") &&
    Number(record.user_id) !== Number(authStore.userInfo?.id)
  );
}

// Transferring is limited to the project owner themselves, plus the tenant owner as the only
// fallback when the project owner is unavailable.
function canTransferOwner(record) {
  if (!record.is_owner) return false;
  return (
    Number(record.user_id) === Number(authStore.userInfo?.id) ||
    authStore.isSuperAdmin()
  );
}

// ---- Ownership transfer & removal hand-over ----

const transferOpen = ref(false);
const transferSubmitting = ref(false);
const transferTargetId = ref(undefined);
const removeOpen = ref(false);
const removeSubmitting = ref(false);
const removePreviewLoading = ref(false);
const removeTarget = ref(null);
const removeUnfinishedCount = ref(0);
const removeItemsTargetId = ref(undefined);
const removeTransferItems = ref(false);

// Candidates are the project's own members, excluding the given user.
function buildCandidates(excludedUserId) {
  return members.value
    .filter((m) => String(m.user_id) !== String(excludedUserId))
    .map((m) => ({
      value: Number(m.user_id),
      label: m.user_name || m.user_username,
    }));
}

const transferCandidates = computed(() =>
  buildCandidates(authStore.userInfo?.id),
);
const removeCandidates = computed(() =>
  buildCandidates(removeTarget.value?.user_id),
);

function openTransferOwner() {
  transferTargetId.value = undefined;
  transferOpen.value = true;
}

async function submitTransferOwner() {
  if (!transferTargetId.value) {
    message.warning($t("owner_transferSelectPlaceholder"));
    return;
  }
  transferSubmitting.value = true;
  try {
    await transferWorkspaceOwner(currentWsId.value, transferTargetId.value);
    message.success($t("owner_transferred"));
    transferOpen.value = false;
    await fetchData();
  } catch {
    /* handled by interceptor */
  } finally {
    transferSubmitting.value = false;
  }
}

async function openRemoveMember(record) {
  removeTarget.value = record;
  removeUnfinishedCount.value = 0;
  removeItemsTargetId.value = undefined;
  removeTransferItems.value = false;
  removeOpen.value = true;
  removePreviewLoading.value = true;
  try {
    const preview = await previewWorkspaceMemberRemoval(
      currentWsId.value,
      record.user_id,
    );
    removeUnfinishedCount.value = Number(preview?.unfinished_count || 0);
  } catch {
    /* handled by interceptor */
  } finally {
    removePreviewLoading.value = false;
  }
}

async function submitRemoveMember() {
  if (!removeTarget.value) return;
  if (removeTransferItems.value && !removeItemsTargetId.value) {
    message.warning($t("owner_transferSelectPlaceholder"));
    return;
  }
  const params = {};
  if (removeTransferItems.value) {
    params.transferItemsTo = removeItemsTargetId.value;
  }
  removeSubmitting.value = true;
  try {
    await removeMember(currentWsId.value, removeTarget.value.user_id, params);
    message.success($t("workspace_memberRemoved"));
    removeOpen.value = false;
    await fetchData();
  } catch {
    /* handled by interceptor */
  } finally {
    removeSubmitting.value = false;
  }
}

async function fetchData() {
  if (!currentWsId.value) return;
  loading.value = true;
  try {
    const [memberList, roleList] = await Promise.all([
      listMembers(currentWsId.value),
      listProjectRoles(currentWsId.value),
    ]);
    members.value = memberList || [];
    projectRoles.value = roleList || [];
  } finally {
    loading.value = false;
  }
}

function openAdd(roleId) {
  addRoleId.value = String(roleId);
  addOpen.value = true;
}

async function handleRole(record, roleId) {
  await updateMemberRole(currentWsId.value, record.user_id, {
    roleId: Number(roleId),
  });
  message.success($t("workspace_roleUpdated"));
  await fetchData();
}

onMounted(fetchData);
watch(currentWsId, fetchData);
</script>

<style scoped lang="less">
.project-members-tab {
  /* Fill .settings-content-body so the member list scrolls on its own instead of
     the whole tab scrolling past the header. */
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding-top: 8px;
}

.tab-hd {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
  margin-bottom: 16px;
}

.member-table-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;

  /* Keep the column titles visible while the rows scroll inside this wrapper. */
  :deep(.ant-table-thead > tr > th) {
    position: sticky;
    top: 0;
    z-index: 2;
    background: #fafafa;
  }
}

.tab-title {
  margin: 0;
  font-size: 14px;
}

.member-avatar {
  flex-shrink: 0;
}

.member-name {
  font-weight: 500;
}

.member-username {
  font-size: 12px;
  color: #999;
}

.member-disabled-tag {
  margin-left: 6px;
  padding: 0 6px;
  border-color: #ffa39e;
  color: #cf1322;
  background: #fff1f0;
  font-size: 12px;
  line-height: 18px;
}
</style>
