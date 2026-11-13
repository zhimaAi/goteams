<!--
 * ProjectRoleMembersPanel.vue
 * Project role member list: shows every project member holding the selected project role and
 * allows switching a member's project role inline. Departments are not part of the project
 * member payload, so this panel only renders member / role / join time columns.
-->
<template>
  <div class="project-role-members-panel">
    <div class="role-members-hd">
      <span class="role-members-title">
        {{
          $t("member_roleMembersHeader", {
            filtered: filteredMembers.length,
            total: roleMembers.length,
          })
        }}
      </span>
      <a-input
        v-model:value="keyword"
        allow-clear
        autocomplete="off"
        class="role-members-search"
        :placeholder="$t('member_roleMemberSearchPlaceholder')"
      >
        <template #suffix><SearchOutlined /></template>
      </a-input>
    </div>

    <a-table
      :columns="columns"
      :data-source="filteredMembers"
      row-key="user_id"
      :pagination="false"
      :locale="{ emptyText: emptyText }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'user'">
          <div class="user-cell">
            <a-avatar
              :size="40"
              :src="getAvatarSrc(record) || undefined"
              :style="
                getAvatarSrc(record) ? avatarImgStyle : avatarFallbackStyle
              "
              class="member-avatar"
            >
              {{ getAvatarSrc(record) ? "" : getInitial(record) }}
            </a-avatar>
            <div class="user-info">
              <div class="user-name">{{ record.user_name || "-" }}</div>
              <div class="user-username">{{ record.user_username || "-" }}</div>
            </div>
          </div>
        </template>
        <template v-else-if="column.key === 'role'">
          <a-select
            v-if="canChangeRole(record)"
            :value="Number(record.role_id)"
            :loading="changingId === record.user_id"
            :disabled="changingId === record.user_id"
            class="role-select"
            @change="(value) => handleRoleChange(record, value)"
          >
            <a-select-option
              v-for="r in roles"
              :key="Number(r.id)"
              :value="Number(r.id)"
            >
              {{ getRoleDisplayName(r.name) }}
            </a-select-option>
          </a-select>
          <a-tag v-else class="role-tag">{{
            getRoleName(record.role_id)
          }}</a-tag>
        </template>
        <template v-else-if="column.key === 'created_at'">
          <span class="join-time">{{ formatJoinTime(record.created_at) }}</span>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup>
import { ref, computed, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { SearchOutlined } from "@ant-design/icons-vue";
import { updateMemberRole } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { getAvatarSrc, getInitial } from "@/utils/avatar";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  // Currently selected project role.
  role: { type: Object, default: null },
  // All project members (any role), used to filter by the selected role.
  members: { type: Array, default: () => [] },
  // All project roles, used by the inline role select.
  roles: { type: Array, default: () => [] },
  // Workspace id of the current project.
  wsId: { type: [String, Number], default: "" },
});

const emit = defineEmits({
  // A member's role changed; parent should reload roles and members.
  changed: () => true,
});

const { t: $t } = useI18n();
const authStore = useAuthStore();

const keyword = ref("");
const changingId = ref(null);

// Avatar: no blue background when an image is present; blue initial fallback when no image.
const avatarImgStyle = { backgroundColor: "transparent" };
const avatarFallbackStyle = { backgroundColor: "#1677ff" };

const columns = computed(() => [
  { title: $t("member_member"), key: "user", width: 260 },
  { title: $t("member_role"), key: "role", width: 170 },
  { title: $t("member_joinTime"), key: "created_at", width: 150 },
]);

// Reset the search whenever another role is selected.
watch(
  () => props.role?.id,
  () => {
    keyword.value = "";
  },
);

const roleMembers = computed(() =>
  props.members.filter(
    (member) => Number(member.role_id) === Number(props.role?.id),
  ),
);

const filteredMembers = computed(() => {
  const text = keyword.value.trim().toLowerCase();
  if (!text) return roleMembers.value;
  return roleMembers.value.filter((member) =>
    [member.user_name, member.user_username].some((field) =>
      String(field || "")
        .toLowerCase()
        .includes(text),
    ),
  );
});

const emptyText = computed(() =>
  keyword.value.trim() ? $t("common_noMatch") : $t("member_roleNoMembers"),
);

function getRoleDisplayName(name) {
  return name || "-";
}

function getRoleName(roleId) {
  return props.roles.find((r) => Number(r.id) === Number(roleId))?.name || "-";
}

function formatJoinTime(timestamp) {
  const text = formatLocalDate(timestamp, true);
  return text === "-" ? text : text.slice(0, 16);
}

// The project owner keeps their role, and a member cannot change their own role here; both
// match the rules already applied by the project member list.
function canChangeRole(record) {
  if (record.is_owner) return false;
  return (
    authStore.hasPermission("project_member:change_role") &&
    Number(record.user_id) !== Number(authStore.userInfo?.id)
  );
}

async function handleRoleChange(record, roleId) {
  if (Number(roleId) === Number(record.role_id)) return;
  changingId.value = record.user_id;
  try {
    await updateMemberRole(props.wsId, record.user_id, {
      roleId: Number(roleId),
    });
    message.success($t("workspace_roleUpdated"));
    emit("changed");
  } catch {
    /* handled by the axios interceptor */
  } finally {
    changingId.value = null;
  }
}
</script>

<style scoped lang="less">
.project-role-members-panel {
  padding: 20px 24px;
  border: 1px solid #e5e9f0;
  border-radius: 8px;
}

.role-members-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.role-members-title {
  color: #182b50;
  font-size: 15px;
  font-weight: 600;
}

.role-members-search {
  width: 260px;
  border-radius: 6px;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.member-avatar {
  flex: none;
  color: #fff;
}

.user-info {
  min-width: 0;
}

.user-name {
  overflow: hidden;
  color: #253858;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-username {
  overflow: hidden;
  color: #98a3b5;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.role-select {
  width: 150px;
}

.role-tag {
  margin: 0;
  border-radius: 4px;
}

.join-time {
  color: #4a5670;
  font-size: 13px;
}

:deep(.ant-table-thead > tr > th) {
  color: #6b7690;
  background: #fafbfd;
  font-weight: 500;
}
</style>
