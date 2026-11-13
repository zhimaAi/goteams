<template>
  <div class="planning-group-tabs">
    <div
      v-for="group in groups"
      :key="group.id"
      class="group-tab"
      :class="{ active: String(group.id) === String(activeGroupId) }"
      @click="$emit('update:activeGroupId', group.id)"
    >
      <a-input
        v-if="editingId === group.id"
        ref="renameInput"
        v-model:value="editingName"
        size="small"
        class="group-tab-input"
        :maxlength="20"
        @click.stop
        @press-enter="finishRename"
        @blur="finishRename"
      />
      <template v-else>
        <span class="group-tab-name" :title="group.name">{{ group.name }}</span>
        <!-- Rename and delete moved into this dropdown, matching the multi-dimensional-table style tabs. -->
        <a-dropdown
          v-if="canEdit && String(group.id) === String(activeGroupId)"
          trigger="click"
          placement="bottomRight"
        >
          <MoreOutlined
            class="group-tab-more"
            :title="$t('planning_groupMore')"
            @click.stop
          />
          <template #overlay>
            <a-menu @click="({ key }) => handleMenu(key, group)">
              <a-menu-item key="rename">
                <EditOutlined /> {{ $t("planning_groupRenameAction") }}
              </a-menu-item>
              <a-menu-item key="delete" :disabled="groups.length <= 1" danger>
                <DeleteOutlined /> {{ $t("planning_groupDeleteAction") }}
              </a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </template>
    </div>

    <a-dropdown
      v-if="canEdit"
      :trigger="['hover']"
      placement="bottomLeft"
      overlay-class-name="planning-group-add-dropdown"
      :get-popup-container="dropdownContainer"
    >
      <button
        type="button"
        class="group-tab-add"
        :disabled="adding"
        :aria-label="$t('planning_groupAdd')"
      >
        <span class="group-tab-add-icon" aria-hidden="true" />
      </button>
      <template #overlay>
        <a-menu @click="addGroup">
          <a-menu-item key="add">
            <PlusOutlined /> {{ $t("planning_groupAddAction") }}
          </a-menu-item>
        </a-menu>
      </template>
    </a-dropdown>

    <a-modal
      :open="!!deleteTarget"
      :title="$t('planning_groupDeleteTitle')"
      :ok-text="$t('common_confirm')"
      :cancel-text="$t('common_cancel')"
      :ok-button-props="{ danger: true, loading: deleting }"
      centered
      :width="480"
      @cancel="deleteTarget = null"
      @ok="confirmDelete"
    >
      <p class="delete-confirm-text">
        {{ $t("planning_groupDeleteConfirm", { name: deleteTarget?.name }) }}
      </p>
      <div class="delete-warning">
        {{ $t("planning_groupDeleteWarning") }}
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  DeleteOutlined,
  EditOutlined,
  MoreOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import {
  createPlanningGroup,
  updatePlanningGroup,
  deletePlanningGroup,
} from "@/api/planning";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  groups: { type: Array, default: () => [] },
  activeGroupId: { type: [String, Number], default: null },
  canEdit: { type: Boolean, default: false },
});
const emit = defineEmits(["update:activeGroupId", "refresh"]);
const { t: $t } = useI18n();

const editingId = ref(null);
const editingName = ref("");
const renameInput = ref(null);
const adding = ref(false);
const deleting = ref(false);
const deleteTarget = ref(null);

// Mount the hover menu on body so the planning table cannot cover it.
function dropdownContainer() {
  return document.body;
}

function handleMenu(key, group) {
  if (key === "rename") startRename(group);
  if (key === "delete") deleteTarget.value = group;
}

function startRename(group) {
  if (!props.canEdit) return;
  editingId.value = group.id;
  editingName.value = group.name;
  nextTick(() => renameInput.value?.[0]?.focus?.());
}

// Submit rename on blur or Enter; silently revert when the name is empty or unchanged.
async function finishRename() {
  const group = props.groups.find((item) => item.id === editingId.value);
  const name = editingName.value.trim();
  editingId.value = null;
  if (!group || !name || name === group.name) return;
  try {
    await updatePlanningGroup(props.wsId, group.id, { name });
    emit("refresh");
  } catch {
    /* handled by interceptor */
  }
}

async function addGroup() {
  adding.value = true;
  try {
    // The group name is generated in the current UI language then stored as a literal, and no longer changes with language.
    const name = $t("planning_groupDefaultName", {
      index: props.groups.length,
    }).slice(0, 20);
    const result = await createPlanningGroup(props.wsId, { name });
    // The second arg flags "new group", which the panel turns into the table's auto-added row.
    emit("refresh", result?.id, true);
  } catch {
    /* handled by interceptor */
  } finally {
    adding.value = false;
  }
}

async function confirmDelete() {
  if (!deleteTarget.value) return;
  deleting.value = true;
  try {
    await deletePlanningGroup(props.wsId, deleteTarget.value.id);
    const remaining = props.groups.filter(
      (item) => item.id !== deleteTarget.value.id,
    );
    deleteTarget.value = null;
    emit("refresh", remaining[0]?.id);
    message.success($t("workspace_deleteSuccess"));
  } catch {
    /* handled by interceptor */
  } finally {
    deleting.value = false;
  }
}
</script>

<style scoped lang="less">
.planning-group-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 0 8px;
  flex-shrink: 0;
  flex-wrap: wrap;
  font-size: 12px;
}
// Tab style follows DingTalk / Feishu multi-dimensional tables:
// unselected is plain text on transparent, selected is a white card with bold dark text.
.group-tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 6px;
  cursor: pointer;
  // Unselected tabs already carry a light grey background and a soft shadow so that every tab
  // has a visible boundary; the selected one lifts above them with a white background.
  // #f0f2f5 matches the a-segmented background used elsewhere in the project.
  background: #f0f2f5;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  color: #595959;
  font-size: 12px;
  border: 1px solid transparent;
  transition: all 0.15s;
  user-select: none;
  white-space: nowrap;
  flex-shrink: 0;

  &:hover {
    background: #e6e8eb;
  }

  &.active {
    background: #fff;
    border-color: #e8e8e8;
    color: #17181c;
    font-weight: 600;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  }
}
.group-tab-name {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.group-tab-input {
  width: 110px;
  height: 22px;
  font-size: 12px;
}
.group-tab-more {
  font-size: 12px;
  color: #8c8c8c;
  border-radius: 4px;
  padding: 2px;

  &:hover {
    color: #17181c;
    background: rgba(0, 0, 0, 0.06);
  }
}
.group-tab-add {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #8c8c8c;
  font-size: 14px;
  cursor: pointer;
  flex-shrink: 0;

  &:hover,
  &:focus-visible {
    color: #17181c;
    background: #fff;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.12);
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }
}
.group-tab-add-icon {
  display: block;
  width: 14px;
  height: 14px;
  background-color: currentColor;
  mask: url("@/image/planning/icon-layers.svg") no-repeat center / contain;
  -webkit-mask: url("@/image/planning/icon-layers.svg") no-repeat center /
    contain;
}
.delete-confirm-text {
  font-size: 15px;
  margin-bottom: 12px;
}
.delete-warning {
  background: #fff2f0;
  border: 1px solid #ffccc7;
  border-radius: 6px;
  padding: 12px 16px;
  color: #cf1322;
  font-size: 14px;
}
</style>

<style lang="less">
.planning-group-add-dropdown {
  z-index: 1100;
}
</style>
