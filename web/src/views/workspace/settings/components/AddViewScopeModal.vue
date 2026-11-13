<!--
 * AddViewScopeModal.vue
 * Team work view scope picker: pick departments/members from the organization tree,
 * or enterprise roles from a card list. Every node checks independently (no parent-child
 * linkage) because each checked node becomes one scope entry of its own type.
 -->
<template>
  <a-modal
    :open="open"
    :title="$t('workspace_scopeAddTitle')"
    width="640px"
    wrap-class-name="view-scope-modal"
    :destroy-on-close="true"
    @update:open="handleOpenChange"
  >
    <template #footer>
      <a-button @click="handleOpenChange(false)">{{
        $t("common_cancel")
      }}</a-button>
      <a-button type="primary" @click="handleConfirm">{{
        $t("common_confirm")
      }}</a-button>
    </template>

    <a-tabs v-model:activeKey="activeTab" size="small">
      <a-tab-pane key="org" :tab="$t('workspace_scopeTabOrg')" />
      <a-tab-pane key="role" :tab="$t('workspace_scopeTabRole')" />
    </a-tabs>

    <div v-show="activeTab === 'org'" class="scope-tree-wrap">
      <a-tree
        v-if="treeData.length"
        v-model:expandedKeys="expandedKeys"
        checkable
        check-strictly
        block-node
        :selectable="false"
        :tree-data="treeData"
        :checked-keys="checkedKeys"
        @check="handleCheck"
      >
        <template #title="node">
          <span class="scope-node">
            <template v-if="node.member">
              <a-avatar
                :size="22"
                :src="getAvatarSrc(node.member) || undefined"
                :style="
                  getAvatarSrc(node.member)
                    ? avatarImgStyle
                    : avatarFallbackStyle
                "
              >
                {{
                  getAvatarSrc(node.member) ? "" : getInitial(node.member.name)
                }}
              </a-avatar>
            </template>
            <ApartmentOutlined v-else class="scope-node-icon" />
            <span class="scope-node-name">{{ node.title }}</span>
          </span>
        </template>
      </a-tree>
      <a-empty
        v-else
        :description="$t('workspace_pickerNoOrgData')"
        :image="simpleImage"
      />
    </div>

    <div v-show="activeTab === 'role'" class="scope-role-list">
      <label
        v-for="role in roles"
        :key="role.id"
        class="scope-role-card"
        :class="{ checked: isRoleChecked(role) }"
      >
        <a-checkbox
          :checked="isRoleChecked(role)"
          @update:checked="(value) => toggleRole(role, value)"
        />
        <span class="scope-role-info">
          <span class="scope-role-name">
            <strong>{{ role.name }}</strong>
            <span v-if="Number(role.is_system) === 1" class="preset-tag">{{
              $t("workspace_presetRole")
            }}</span>
          </span>
          <small v-if="role.description">{{ role.description }}</small>
        </span>
      </label>
      <a-empty
        v-if="!roles.length"
        :description="$t('workspace_pickerNoRoleData')"
        :image="simpleImage"
      />
    </div>
  </a-modal>
</template>

<script setup>
import { ref, computed, watch } from "vue";
import { useI18n } from "vue-i18n";
import { Empty } from "ant-design-vue";
import { ApartmentOutlined } from "@ant-design/icons-vue";
import { getAvatarSrc, getInitial } from "@/utils/avatar";

const props = defineProps({
  open: { type: Boolean, default: false },
  // Root node label of the organization tree.
  enterpriseName: { type: String, default: "" },
  // Flat department list ({ id, parent_id, name }).
  departments: { type: Array, default: () => [] },
  // Enterprise members ({ id, name, username, avatar, department_ids }).
  members: { type: Array, default: () => [] },
  // Pickable enterprise roles (the role being configured is excluded by the caller).
  roles: { type: Array, default: () => [] },
  // Already chosen entries ([{ type, id }]), pre-checked when the modal opens.
  existingTargets: { type: Array, default: () => [] },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  // Confirmed entries; the caller merges them with the current list.
  confirm: (targets) => Array.isArray(targets),
});

const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const ORG_ROOT_KEY = "org-root";
const UNASSIGNED_KEY = "org-unassigned";

// No blue background behind a real image; blue initial fallback when there is none.
const avatarImgStyle = { backgroundColor: "transparent" };
const avatarFallbackStyle = { backgroundColor: "#0d68ff" };

const activeTab = ref("org");
const expandedKeys = ref([]);
// Chosen entries keyed by "type:id" so a member showing under several departments
// checks and unchecks everywhere at once.
const selected = ref(new Map());

const departmentChildren = computed(() => {
  const map = {};
  for (const department of props.departments) {
    (map[department.parent_id] ||= []).push(department);
  }
  return map;
});

const membersByDepartment = computed(() => {
  const map = {};
  const unassigned = [];
  for (const member of props.members) {
    if (!member.department_ids?.length) {
      unassigned.push(member);
      continue;
    }
    for (const id of member.department_ids) (map[id] ||= []).push(member);
  }
  map[UNASSIGNED_KEY] = unassigned;
  return map;
});

const treeData = computed(() => {
  const children = buildDepartmentNodes("0");
  const unassigned = memberNodes(
    membersByDepartment.value[UNASSIGNED_KEY],
    UNASSIGNED_KEY,
  );
  if (unassigned.length) {
    children.push({
      key: UNASSIGNED_KEY,
      title: t("workspace_pickerUnassigned"),
      // The bucket is not a real department, so it cannot become a scope entry.
      disableCheckbox: true,
      children: unassigned,
    });
  }
  if (!children.length) return [];
  return [
    {
      key: ORG_ROOT_KEY,
      title: props.enterpriseName || t("org_unnamedEnterprise"),
      // "Every member" is expressed by the all-members radio, not by a root entry.
      disableCheckbox: true,
      children,
    },
  ];
});

function buildDepartmentNodes(parentId) {
  const nodes = [];
  for (const department of departmentChildren.value[parentId] || []) {
    nodes.push({
      key: `dept-${department.id}`,
      title: department.name,
      dept: department,
      children: [
        ...buildDepartmentNodes(String(department.id)),
        ...memberNodes(
          membersByDepartment.value[department.id],
          `dept-${department.id}`,
        ),
      ],
    });
  }
  return nodes;
}

function memberNodes(list, scope) {
  return (list || []).map((member) => ({
    key: `${scope}-u${member.id}`,
    title: member.name || member.username,
    member,
  }));
}

const checkedKeys = computed(() => {
  const keys = [];
  const walk = (nodes) => {
    for (const node of nodes) {
      if (node.member && selected.value.has(`user:${node.member.id}`)) {
        keys.push(node.key);
      } else if (node.dept && selected.value.has(`dept:${node.dept.id}`)) {
        keys.push(node.key);
      }
      if (node.children) walk(node.children);
    }
  };
  walk(treeData.value);
  return { checked: keys, halfChecked: [] };
});

function toggleEntry(type, id, checked) {
  const key = `${type}:${id}`;
  const next = new Map(selected.value);
  if (checked) {
    next.set(key, { type, id: Number(id) });
  } else {
    next.delete(key);
  }
  selected.value = next;
}

function handleCheck(keys, event) {
  const node = event.node;
  if (node.member) {
    toggleEntry("user", node.member.id, event.checked);
  } else if (node.dept) {
    toggleEntry("dept", node.dept.id, event.checked);
  }
}

function isRoleChecked(role) {
  return selected.value.has(`role:${role.id}`);
}

function toggleRole(role, checked) {
  toggleEntry("role", role.id, checked);
}

function collectExpandedKeys() {
  const keys = [ORG_ROOT_KEY, UNASSIGNED_KEY];
  for (const department of props.departments) {
    keys.push(`dept-${department.id}`);
  }
  return keys;
}

// Reset to a fresh state seeded with the current entries every time the modal opens.
watch(
  () => props.open,
  (open) => {
    if (!open) return;
    activeTab.value = "org";
    expandedKeys.value = collectExpandedKeys();
    const next = new Map();
    for (const target of props.existingTargets) {
      next.set(`${target.type}:${target.id}`, {
        type: target.type,
        id: Number(target.id),
      });
    }
    selected.value = next;
  },
);

function handleOpenChange(value) {
  emit("update:open", value);
}

function handleConfirm() {
  emit("confirm", [...selected.value.values()]);
  emit("update:open", false);
}
</script>

<style scoped lang="less">
.scope-tree-wrap {
  max-height: 420px;
  overflow-y: auto;
  padding: 4px 0 8px;
}

.scope-node {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.scope-node-icon {
  color: #0d68ff;
  font-size: 15px;
}

.scope-node-name {
  color: #253858;
  font-size: 14px;
}

.scope-role-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 420px;
  overflow-y: auto;
  padding: 4px 0 8px;
}

.scope-role-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid #e5e9f0;
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.15s;

  &:hover,
  &.checked {
    border-color: #0d68ff;
  }
}

.scope-role-info {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;

  small {
    color: #98a3b5;
    font-size: 12px;
  }
}

.scope-role-name {
  display: flex;
  align-items: center;
  gap: 7px;

  strong {
    color: #253858;
    font-size: 14px;
    font-weight: 600;
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
</style>
