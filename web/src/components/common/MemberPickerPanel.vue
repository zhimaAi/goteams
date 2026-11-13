<!--
 * MemberPickerPanel.vue
 * Shared member selection panel: organization tree / enterprise role tabs on the left, picked
 * members on the right. Pure presentation — the caller loads the source data and decides what
 * confirming a selection does (adding to a project, saving a notification strategy, ...).
 -->
<template>
  <a-spin :spinning="loading">
    <div class="picker-body">
      <div class="picker-source">
        <a-tabs v-model:activeKey="activeTab" size="small">
          <a-tab-pane
            v-for="item in tabItems"
            :key="item.key"
            :tab="item.label"
          />
        </a-tabs>

        <a-input
          v-model:value="keyword"
          allow-clear
          autocomplete="off"
          class="picker-search"
          :placeholder="$t('workspace_pickerSearchPlaceholder')"
        >
          <template #prefix><SearchOutlined /></template>
        </a-input>

        <div class="picker-tree-wrap">
          <a-tree
            v-if="treeData.length"
            v-model:expandedKeys="expandedKeys"
            checkable
            block-node
            :selectable="false"
            :tree-data="treeData"
            :checked-keys="checkedKeys"
            @check="handleCheck"
          >
            <template #title="node">
              <span v-if="node.member" class="picker-node">
                <a-avatar
                  :size="22"
                  :src="getAvatarSrc(node.member)"
                  :style="
                    getAvatarSrc(node.member)
                      ? avatarImgStyle
                      : avatarFallbackStyle
                  "
                >
                  {{ getInitial(node.member) }}
                </a-avatar>
                <span class="picker-node-name">{{ node.title }}</span>
                <span v-if="node.member.joined" class="picker-node-joined">
                  {{ $t("workspace_joined") }}
                </span>
              </span>
              <span v-else class="picker-node">
                <ApartmentOutlined
                  v-if="activeTab === 'org'"
                  class="picker-node-icon picker-node-icon-org"
                />
                <TeamOutlined
                  v-else
                  class="picker-node-icon picker-node-icon-role"
                />
                <span class="picker-node-name">{{ node.title }}</span>
              </span>
            </template>
          </a-tree>
          <a-empty v-else :description="emptyText" :image="simpleImage" />
        </div>
      </div>

      <div class="picker-selected">
        <div class="picker-selected-title">
          {{ $t("workspace_pickerSelected", { n: selectedMembers.length }) }}
        </div>
        <div class="picker-selected-list">
          <div
            v-for="member in selectedMembers"
            :key="member.id"
            class="picker-selected-item"
          >
            <a-avatar
              :size="28"
              :src="getAvatarSrc(member)"
              :style="
                getAvatarSrc(member) ? avatarImgStyle : avatarFallbackStyle
              "
            >
              {{ getInitial(member) }}
            </a-avatar>
            <div class="picker-selected-meta">
              <div class="picker-selected-name">
                {{ member.name || member.username }}
              </div>
              <div class="picker-selected-username">
                {{ member.username }}
              </div>
            </div>
            <CloseOutlined
              class="picker-selected-remove"
              @click="unselect(member.id)"
            />
          </div>
          <a-empty
            v-if="!selectedMembers.length"
            :description="$t('workspace_pickerNoSelection')"
            :image="simpleImage"
          />
        </div>
      </div>
    </div>
  </a-spin>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { Empty } from "ant-design-vue";
import {
  ApartmentOutlined,
  CloseOutlined,
  SearchOutlined,
  TeamOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { getAvatarSrc, getInitial } from "@/utils/avatar";

const props = defineProps({
  // Organization tree root label, usually the enterprise name.
  enterpriseName: { type: String, default: "" },
  departments: { type: Array, default: () => [] },
  enterpriseRoles: { type: Array, default: () => [] },
  // Members carry id / name / username / avatar / role_id / department_ids and an optional
  // joined flag that only the project member source sets.
  members: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  selectedIds: { type: Array, default: () => [] },
});

const emit = defineEmits({
  "update:selectedIds": (ids) => Array.isArray(ids),
});

const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const ORG_ROOT_KEY = "org-root";
const UNASSIGNED_KEY = "org-unassigned";

// No blue background behind a real image; blue initial fallback when there is none.
const avatarImgStyle = { backgroundColor: "transparent" };
const avatarFallbackStyle = { backgroundColor: "#0d68ff" };

const activeTab = ref("org");
const keyword = ref("");
const expandedKeys = ref([]);

const tabItems = computed(() => [
  { key: "org", label: t("workspace_pickerByOrg") },
  { key: "role", label: t("workspace_pickerByRole") },
]);

const membersById = computed(() => {
  const map = {};
  for (const member of props.members) map[member.id] = member;
  return map;
});

const selectedMembers = computed(() =>
  props.selectedIds.map((id) => membersById.value[id]).filter(Boolean),
);

const filteredMembers = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return props.members;
  return props.members.filter(
    (member) =>
      (member.name || "").toLowerCase().includes(kw) ||
      (member.username || "").toLowerCase().includes(kw),
  );
});

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
  for (const member of filteredMembers.value) {
    if (!member.department_ids?.length) {
      unassigned.push(member);
      continue;
    }
    for (const id of member.department_ids) (map[id] ||= []).push(member);
  }
  map[UNASSIGNED_KEY] = unassigned;
  return map;
});

const membersByRole = computed(() => {
  const map = {};
  for (const member of filteredMembers.value) {
    (map[member.role_id] ||= []).push(member);
  }
  return map;
});

const orgTreeData = computed(() => {
  const children = buildDepartmentNodes("0");
  const unassigned = memberNodes(
    membersByDepartment.value[UNASSIGNED_KEY],
    UNASSIGNED_KEY,
  );
  // The unassigned bucket is the root's last child, so checking the root selects every member.
  if (unassigned.length) {
    children.push({
      key: UNASSIGNED_KEY,
      title: t("workspace_pickerUnassigned"),
      children: unassigned,
    });
  }
  if (!children.length) return [];
  return [
    {
      key: ORG_ROOT_KEY,
      title: props.enterpriseName || t("org_unnamedEnterprise"),
      children,
    },
  ];
});

const roleTreeData = computed(() =>
  props.enterpriseRoles
    .map((role) => ({
      key: `role-${role.id}`,
      title: role.name,
      children: memberNodes(membersByRole.value[role.id], `role-${role.id}`),
    }))
    // Roles with no matching member are hidden while searching.
    .filter((node) => node.children.length),
);

const treeData = computed(() =>
  activeTab.value === "org" ? orgTreeData.value : roleTreeData.value,
);

const emptyText = computed(() => {
  if (keyword.value.trim()) return t("common_noMatch");
  return activeTab.value === "org"
    ? t("workspace_pickerNoOrgData")
    : t("workspace_pickerNoRoleData");
});

// One member can appear under several departments and under a role, so node keys are scoped by
// their parent and the selection is tracked by member id instead of by node key.
const checkedKeys = computed(() => {
  const picked = new Set(props.selectedIds);
  const keys = [];
  const walk = (nodes) => {
    for (const node of nodes) {
      if (node.member) {
        if (picked.has(node.member.id)) keys.push(node.key);
        continue;
      }
      walk(node.children || []);
    }
  };
  walk(treeData.value);
  return keys;
});

function memberNodes(list, parentKey) {
  return (list || []).map((member) => ({
    key: `${parentKey}-u${member.id}`,
    title: member.name || member.username,
    member,
    isLeaf: true,
  }));
}

// A department is kept only when it or one of its descendants still holds a matching member.
function buildDepartmentNodes(parentId) {
  const nodes = [];
  for (const department of departmentChildren.value[parentId] || []) {
    const children = [
      ...buildDepartmentNodes(department.id),
      ...memberNodes(
        membersByDepartment.value[department.id],
        `dept-${department.id}`,
      ),
    ];
    if (!children.length && keyword.value.trim()) continue;
    nodes.push({
      key: `dept-${department.id}`,
      title: department.name,
      children,
    });
  }
  return nodes;
}

function collectMemberIds(node, ids) {
  if (node.member) {
    ids.push(node.member.id);
    return ids;
  }
  for (const child of node.children || []) collectMemberIds(child, ids);
  return ids;
}

function findNodeByKey(nodes, key) {
  for (const node of nodes) {
    if (node.key === key) return node;
    const hit = findNodeByKey(node.children || [], key);
    if (hit) return hit;
  }
  return null;
}

// Resolved against our own tree data instead of the event node so the handler does not depend on
// the tree component's internal node shape.
function handleCheck(_keys, info) {
  const node = findNodeByKey(treeData.value, info.node.key);
  if (!node) return;
  const picked = new Set(props.selectedIds);
  for (const id of collectMemberIds(node, [])) {
    if (info.checked) picked.add(id);
    else picked.delete(id);
  }
  // Keep the source member order so the selected panel does not jump around.
  emit(
    "update:selectedIds",
    props.members.map((member) => member.id).filter((id) => picked.has(id)),
  );
}

function unselect(id) {
  emit(
    "update:selectedIds",
    props.selectedIds.filter((item) => item !== id),
  );
}

function collectExpandableKeys(nodes, keys) {
  for (const node of nodes) {
    if (node.member) continue;
    keys.push(node.key);
    collectExpandableKeys(node.children || [], keys);
  }
  return keys;
}

// The tree starts fully expanded and re-expands whenever the search or tab reshapes it.
watch(treeData, (nodes) => {
  expandedKeys.value = collectExpandableKeys(nodes, []);
});
</script>

<style scoped lang="less">
.picker-body {
  display: flex;
  min-height: 360px;
  gap: 16px;
}

.picker-source {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  border-right: 1px solid #eef1f6;
  padding-right: 16px;
}

.picker-search {
  margin-bottom: 8px;
}

.picker-tree-wrap {
  min-height: 0;
  max-height: 360px;
  flex: 1;
  overflow: auto;
}

.picker-node {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.picker-node-icon {
  font-size: 14px;
}

.picker-node-icon-org {
  color: #0d68ff;
}

.picker-node-icon-role {
  color: #722ed1;
}

.picker-node-name {
  color: #182b50;
}

.picker-node-joined {
  color: #8c98ad;
  font-size: 12px;
}

.picker-selected {
  display: flex;
  width: 280px;
  flex: 0 0 280px;
  flex-direction: column;
}

.picker-selected-title {
  margin-bottom: 8px;
  color: #182b50;
  font-weight: 600;
}

.picker-selected-list {
  min-height: 0;
  max-height: 400px;
  flex: 1;
  overflow: auto;
}

.picker-selected-item {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
  border: 1px solid #eef1f6;
  border-radius: 8px;
  padding: 8px;
  gap: 8px;
}

.picker-selected-meta {
  min-width: 0;
  flex: 1;
}

.picker-selected-name {
  overflow: hidden;
  color: #182b50;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.picker-selected-username {
  overflow: hidden;
  color: #8c98ad;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.picker-selected-remove {
  color: #8c98ad;
  cursor: pointer;

  &:hover {
    color: #cf1322;
  }
}
</style>
