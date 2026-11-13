<!--
 * RoleMembersPanel.vue
 * Enterprise role member list: shows every member holding the selected role and
 * allows switching a member's enterprise role inline.
 -->
<template>
  <div class="role-members-panel">
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
      row-key="id"
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
              {{ getAvatarSrc(record) ? "" : getInitial(record.name) }}
            </a-avatar>
            <div class="user-info">
              <div class="user-name">
                {{ record.name }}
                <a-tag
                  v-if="isMemberDisabled(record)"
                  class="member-disabled-tag"
                >
                  {{ $t("member_disabledTag") }}
                </a-tag>
              </div>
              <div class="user-username">{{ record.username }}</div>
            </div>
          </div>
        </template>
        <template v-else-if="column.key === 'phone'">
          <span class="member-contact">{{ record.phone || "-" }}</span>
        </template>
        <template v-else-if="column.key === 'remark'">
          <a-tooltip :title="record.remark || undefined">
            <span class="member-remark">{{ record.remark || "-" }}</span>
          </a-tooltip>
        </template>
        <template v-else-if="column.key === 'role'">
          <a-select
            v-if="canChangeRole(record)"
            :value="Number(record.role_id)"
            :loading="changingId === record.id"
            :disabled="changingId === record.id"
            class="role-select"
            @change="(value) => handleRoleChange(record, value)"
          >
            <a-select-option
              v-for="r in roles"
              :key="Number(r.id)"
              :value="Number(r.id)"
            >
              {{ r.name }}
            </a-select-option>
          </a-select>
          <a-tag v-else class="role-tag">{{
            getRoleName(record.role_id)
          }}</a-tag>
        </template>
        <template v-else-if="column.key === 'department'">
          <a-tooltip :title="departmentNames(record) || undefined">
            <span class="member-department">{{
              departmentNames(record) || "-"
            }}</span>
          </a-tooltip>
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
import { updateEnterpriseMember } from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { getAvatarSrc, getInitial } from "@/utils/avatar";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  // Currently selected enterprise role.
  role: { type: Object, default: null },
  // All enterprise members (any role).
  members: { type: Array, default: () => [] },
  // All enterprise roles, used by the inline role select.
  roles: { type: Array, default: () => [] },
  // All departments, used to resolve department names.
  departments: { type: Array, default: () => [] },
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
  { title: $t("member_member"), key: "user", width: 240 },
  { title: $t("member_phone"), key: "phone", width: 130 },
  { title: $t("member_remark"), key: "remark", width: 180 },
  { title: $t("member_role"), key: "role", width: 160 },
  { title: $t("member_department"), key: "department", width: 150 },
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
    [member.name, member.username, member.phone].some((field) =>
      String(field || "")
        .toLowerCase()
        .includes(text),
    ),
  );
});

const emptyText = computed(() =>
  keyword.value.trim() ? $t("common_noMatch") : $t("member_roleNoMembers"),
);

const departmentsById = computed(() => {
  const map = {};
  for (const dept of props.departments) {
    map[String(dept.id)] = dept;
  }
  return map;
});

const rolesById = computed(() => {
  const map = {};
  for (const r of props.roles) {
    map[String(r.id)] = r;
  }
  return map;
});

function getRoleName(roleId) {
  return rolesById.value[String(roleId)]?.name || "-";
}

function departmentNames(record) {
  return (record.department_ids || [])
    .map((id) => departmentsById.value[String(id)]?.name)
    .filter(Boolean)
    .join(", ");
}

function formatJoinTime(timestamp) {
  const text = formatLocalDate(timestamp, true);
  return text === "-" ? text : text.slice(0, 16);
}

function isTenantSuperAdmin(record) {
  // Ownership is transferable, so it comes from the backend flag, not from id === admin_id.
  return record?.is_owner === true;
}

// Disabled members stay listed (with a tag) so admins can still see and manage them.
function isMemberDisabled(record) {
  return String(record?.status) === "0";
}

// The tenant owner cannot be demoted, so its role stays read-only.
function canChangeRole(record) {
  return authStore.hasPermission("member:edit") && !isTenantSuperAdmin(record);
}

async function handleRoleChange(record, roleId) {
  if (Number(roleId) === Number(record.role_id)) return;
  changingId.value = record.id;
  try {
    // The update endpoint is a full update: resend the current profile untouched,
    // an empty avatar leaves the stored avatar as is.
    await updateEnterpriseMember(record.id, {
      name: record.name || "",
      username: record.username || "",
      phone: record.phone || "",
      remark: record.remark || "",
      roleId: Number(roleId),
      avatar: "",
      departmentIds: (record.department_ids || []).map(Number),
    });
    message.success($t("member_roleUpdated"));
    emit("changed");
  } catch {
    /* handled by the axios interceptor */
  } finally {
    changingId.value = null;
  }
}
</script>

<style scoped lang="less">
.role-members-panel {
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

.member-department,
.member-contact,
.member-remark,
.join-time {
  color: #4a5670;
  font-size: 13px;
}

.member-remark {
  display: inline-block;
  max-width: 170px;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  white-space: nowrap;
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

:deep(.ant-table-thead > tr > th) {
  color: #6b7690;
  background: #fafbfd;
  font-weight: 500;
}
</style>
