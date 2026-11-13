<template>
  <!-- Assignee: first avatar + all names; show placeholder when no assignee -->
  <span class="assignee-cell">
    <a-tooltip v-if="assignees.length">
      <template #title>
        <div
          v-for="(assignee, index) in assignees"
          :key="assignee.id || index"
          class="assignee-tooltip-name"
        >
          {{ displayName(assignee) }}
        </div>
      </template>
      <span class="assignee-content">
        <a-avatar
          :size="20"
          :src="src(assignees[0])"
          class="assignee-avatar"
          :class="{ 'has-image': hasAvatar(assignees[0]) }"
          >{{ initials(displayName(assignees[0])) }}</a-avatar
        >
        <span class="assignee-names">{{ namesText }}</span>
      </span>
    </a-tooltip>
    <span v-else class="assignee-empty">—</span>
  </span>
</template>

<script setup>
import { computed } from "vue";
import { getAvatarSrc, getInitial } from "@/utils/avatar";

const props = defineProps({
  // Assignee array (id / name / avatar / username); supports comma-separated string or array
  ids: { type: [String, Array], default: "" },
  names: { type: [String, Array], default: "" },
  avatars: { type: [String, Array], default: "" },
  usernames: { type: [String, Array], default: "" },
  // Compatible single-value usage: pass avatar and name directly
  avatar: { type: String, default: "" },
  name: { type: String, default: "" },
});

function toArr(v) {
  if (Array.isArray(v)) return v.map((x) => String(x ?? ""));
  if (!v) return [];
  return String(v)
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s !== "");
}

const assignees = computed(() => {
  if (props.name || props.avatar) {
    return [{ id: "", name: props.name, avatar: props.avatar }];
  }
  const ids = toArr(props.ids);
  const names = toArr(props.names);
  const avatars = toArr(props.avatars);
  const usernames = toArr(props.usernames);
  const n = Math.max(
    ids.length,
    names.length,
    avatars.length,
    usernames.length,
  );
  const list = [];
  for (let i = 0; i < n; i++) {
    list.push({
      id: ids[i] || "",
      name: names[i] || "",
      username: usernames[i] || "",
      avatar: avatars[i] || "",
    });
  }
  return list;
});

const namesText = computed(() => assignees.value.map(displayName).join(";"));

function displayName(assignee) {
  return assignee.name || assignee.username || "-";
}

function src(a) {
  return getAvatarSrc({ avatar: a.avatar }) || undefined;
}
function hasAvatar(assignee) {
  return !!src(assignee);
}
function initials(name) {
  return getInitial(name);
}
</script>

<style scoped>
.assignee-cell {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  min-width: 0;
  line-height: 1;
}
.assignee-content {
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  min-width: 0;
}
.assignee-avatar {
  width: 20px;
  height: 20px;
  background: #1677ff;
  flex: 0 0 auto;
}
.assignee-avatar.has-image {
  background: transparent;
}
.assignee-names {
  min-width: 0;
  margin-left: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 16px;
}
.assignee-empty {
  color: #b7bdc7;
}
.assignee-tooltip-name + .assignee-tooltip-name {
  margin-top: 2px;
}
</style>
