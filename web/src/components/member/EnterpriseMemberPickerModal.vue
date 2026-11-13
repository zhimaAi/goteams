<!--
 * EnterpriseMemberPickerModal.vue
 * Enterprise member picker shared by every "add member" entry point: pick members by
 * organization tree or by enterprise role, then confirm the ids. The caller owns the
 * candidate source and what confirming means, so it can add them to a project or to the
 * team-work display list with the same interaction.
-->
<template>
  <a-modal
    :open="open"
    :title="$t('workspace_addMember')"
    width="800px"
    wrap-class-name="member-picker-modal"
    :destroy-on-close="true"
    @update:open="handleOpenChange"
  >
    <template #footer>
      <a-button @click="handleOpenChange(false)">{{
        $t("common_cancel")
      }}</a-button>
      <a-button type="primary" :loading="saving" @click="handleSubmit">
        {{
          addableIds.length
            ? $t("workspace_confirmAddCount", { n: addableIds.length })
            : $t("workspace_confirmAdd")
        }}
      </a-button>
    </template>
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
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { message, Empty } from "ant-design-vue";
import {
  ApartmentOutlined,
  CloseOutlined,
  SearchOutlined,
  TeamOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { getAvatarSrc, getInitial } from "@/utils/avatar";

const props = defineProps({
  open: { type: Boolean, default: false },
  // Resolves the picker source data on every open; returns the member candidate payload.
  candidates: { type: Function, required: true },
  // Set by the caller while it persists the picked members; it also owns closing the modal.
  saving: { type: Boolean, default: false },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  // Members to add; the payload is the member ids excluding the already joined ones.
  confirm: (ids) => Array.isArray(ids),
});

const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const ORG_ROOT_KEY = "org-root";
const UNASSIGNED_KEY = "org-unassigned";

// No blue background behind a real image; blue initial fallback when there is none.
const avatarImgStyle = { backgroundColor: "transparent" };
const avatarFallbackStyle = { backgroundColor: "#0d68ff" };

const loading = ref(false);
const activeTab = ref("org");
const keyword = ref("");
const expandedKeys = ref([]);
const selectedIds = ref([]);
const enterpriseName = ref("");
const departments = ref([]);
const enterpriseRoles = ref([]);
const members = ref([]);

const tabItems = computed(() => [
  { key: "org", label: t("workspace_pickerByOrg") },
  { key: "role", label: t("workspace_pickerByRole") },
]);

const membersById = computed(() => {
  const map = {};
  for (const member of members.value) map[member.id] = member;
  return map;
});

const selectedMembers = computed(() =>
  selectedIds.value.map((id) => membersById.value[id]).filter(Boolean),
);

// Members already joined are selectable but never re-submitted.
const addableIds = computed(() =>
  selectedIds.value.filter((id) => !membersById.value[id]?.joined),
);

const filteredMembers = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  if (!kw) return members.value;
  return members.value.filter(
    (member) =>
      (member.name || "").toLowerCase().includes(kw) ||
      (member.username || "").toLowerCase().includes(kw),
  );
});

const departmentChildren = computed(() => {
  const map = {};
  for (const department of departments.value) {
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
      title: enterpriseName.value || t("org_unnamedEnterprise"),
      children,
    },
  ];
});

const roleTreeData = computed(() =>
  enterpriseRoles.value
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
  const picked = new Set(selectedIds.value);
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
  const picked = new Set(selectedIds.value);
  for (const id of collectMemberIds(node, [])) {
    if (info.checked) picked.add(id);
    else picked.delete(id);
  }
  // Keep the enterprise member order so the selected panel does not jump around.
  selectedIds.value = members.value
    .map((member) => member.id)
    .filter((id) => picked.has(id));
}

function unselect(id) {
  selectedIds.value = selectedIds.value.filter((item) => item !== id);
}

function collectExpandableKeys(nodes, keys) {
  for (const node of nodes) {
    if (node.member) continue;
    keys.push(node.key);
    collectExpandableKeys(node.children || [], keys);
  }
  return keys;
}

async function loadCandidates() {
  loading.value = true;
  try {
    const data = await props.candidates();
    enterpriseName.value = data?.enterprise_name || "";
    departments.value = data?.departments || [];
    enterpriseRoles.value = data?.enterprise_roles || [];
    members.value = data?.members || [];
    // Already joined members open checked, so the right panel shows the current state instead
    // of a per-row label; they stay out of the confirm payload.
    selectedIds.value = members.value
      .filter((member) => member.joined)
      .map((member) => member.id);
  } catch {
    enterpriseName.value = "";
    departments.value = [];
    enterpriseRoles.value = [];
    members.value = [];
  } finally {
    loading.value = false;
  }
}

function handleSubmit() {
  if (!selectedIds.value.length) {
    handleOpenChange(false);
    return;
  }
  if (!addableIds.value.length) {
    message.info(t("workspace_pickerAllJoined"));
    handleOpenChange(false);
    return;
  }
  // The caller persists the picked members and closes the modal when it is done.
  emit(
    "confirm",
    addableIds.value.map((id) => Number(id)),
  );
}

function handleOpenChange(value) {
  emit("update:open", value);
}

// Every open starts clean: no search word, no selection, first tab.
watch(
  () => props.open,
  async (open) => {
    if (!open) return;
    keyword.value = "";
    selectedIds.value = [];
    activeTab.value = "org";
    await loadCandidates();
  },
);

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
