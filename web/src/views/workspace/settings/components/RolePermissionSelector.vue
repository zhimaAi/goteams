<template>
  <div class="role-permission-selector">
    <template v-if="readonly && enterpriseMode">
      <div
        v-for="group in groups"
        :key="group.group"
        class="readonly-permission-group"
      >
        <strong class="permission-group-title">{{ group.group }}</strong>
        <div class="permission-tags">
          <a-tag
            v-for="permission in grantedPermissions(group)"
            :key="permission.key"
            color="blue"
            class="permission-tag"
          >
            {{ permission.label }}
          </a-tag>
          <span
            v-if="grantedPermissions(group).length === 0"
            class="permission-empty"
          >
            {{ $t("workspace_noPermission") }}
          </span>
        </div>
      </div>
    </template>

    <template v-else-if="readonly">
      <div v-for="group in groups" :key="group.group" class="permission-card">
        <strong class="permission-group-title">{{ group.group }}</strong>
        <div class="permission-grid readonly-grid">
          <a-checkbox
            v-for="permission in group.items"
            :key="permission.key"
            :checked="isSelected(permission.key)"
            disabled
            class="readonly-permission"
            :class="{ unavailable: !isSelected(permission.key) }"
          >
            {{ permission.label }}
          </a-checkbox>
        </div>
      </div>
    </template>

    <template v-else>
      <div v-for="group in groups" :key="group.group" class="permission-card">
        <div class="permission-group-header">
          <strong class="permission-group-title">{{ group.group }}</strong>
          <a-checkbox
            :checked="isGroupSelected(group)"
            :indeterminate="isGroupIndeterminate(group)"
            :disabled="permissionKeys(group).length === 0"
            @change="toggleGroup(group, $event.target.checked)"
          >
            {{ $t("common_all") }}
          </a-checkbox>
        </div>
        <div class="permission-grid">
          <a-checkbox
            v-for="permission in group.items"
            :key="permission.key"
            :checked="isSelected(permission.key)"
            @change="togglePermissions([permission.key], $event.target.checked)"
          >
            {{ permission.label }}
          </a-checkbox>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({
  groups: { type: Array, default: () => [] },
  modelValue: { type: Array, default: () => [] },
  readonly: { type: Boolean, default: false },
  enterpriseMode: { type: Boolean, default: false },
});

const emit = defineEmits({
  "update:modelValue": (value) => Array.isArray(value),
});

const selectedPermissionKeys = computed(() => new Set(props.modelValue));

function permissionKeys(group) {
  return (group.items || [])
    .map((permission) => permission.key)
    .filter(Boolean);
}

function isSelected(key) {
  return selectedPermissionKeys.value.has(key);
}

function isEveryPermissionSelected(keys) {
  return keys.length > 0 && keys.every(isSelected);
}

function isPartiallySelected(keys) {
  const selectedCount = keys.filter(isSelected).length;
  return selectedCount > 0 && selectedCount < keys.length;
}

function isGroupSelected(group) {
  return isEveryPermissionSelected(permissionKeys(group));
}

function isGroupIndeterminate(group) {
  return isPartiallySelected(permissionKeys(group));
}

function grantedPermissions(group) {
  return (group.items || []).filter((permission) => isSelected(permission.key));
}

function toggleGroup(group, checked) {
  togglePermissions(permissionKeys(group), checked);
}

function togglePermissions(keys, checked) {
  const next = new Set(props.modelValue);
  for (const key of keys) {
    if (checked) {
      next.add(key);
    } else {
      next.delete(key);
    }
  }
  emit("update:modelValue", [...next]);
}
</script>

<style scoped lang="less">
.role-permission-selector {
  width: 100%;
}

.readonly-permission-group {
  margin-bottom: 24px;
}

.permission-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.permission-tag {
  margin: 0;
}

.permission-empty {
  color: var(--app-text-secondary);
  font-size: 13px;
}

.permission-card {
  width: 100%;
  margin-bottom: 16px;
  padding: 16px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface-muted);
}

.permission-group-title {
  display: block;
  margin-bottom: 16px;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
}

.permission-group-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;

  .permission-group-title {
    margin-bottom: 0;
  }
}

.permission-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(128px, 1fr));
  gap: 16px 24px;

  :deep(.ant-checkbox-wrapper) {
    margin: 0;
    color: var(--app-text);
    font-size: 13px;
  }
}

.readonly-permission {
  margin: 0;
  font-size: 13px;

  :deep(.ant-checkbox + span) {
    color: var(--app-text);
  }

  &.unavailable {
    :deep(.ant-checkbox + span) {
      color: var(--app-text-secondary);
    }
  }
}

@media (max-width: 980px) {
  .permission-grid {
    grid-template-columns: repeat(2, minmax(128px, 1fr));
  }
}
</style>
