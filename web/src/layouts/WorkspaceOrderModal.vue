<template>
  <a-modal
    :open="open"
    :title="$t('workspace_projectOrder')"
    width="526px"
    :confirm-loading="saving"
    :ok-text="$t('common_save')"
    :cancel-text="$t('common_cancel')"
    @ok="save"
    @cancel="close"
  >
    <p class="order-hint">{{ $t("workspace_projectOrderHint") }}</p>
    <div ref="listRef" class="order-list">
      <div
        v-for="(workspace, index) in orderedWorkspaces"
        :key="workspace.id"
        class="order-item"
        :data-workspace-id="workspace.id"
      >
        <HolderOutlined
          class="order-handle"
          :title="$t('workspace_projectOrderDrag')"
        />
        <img
          v-if="iconUrls[workspace.id]"
          :src="iconUrls[workspace.id]"
          alt=""
          class="workspace-icon-image"
        />
        <span
          v-else
          class="workspace-icon"
          :style="{ backgroundColor: workspace.color || '#0d68ff' }"
        >
          {{ workspace.name.trim().charAt(0) }}
        </span>
        <span class="workspace-name" :title="workspace.name">
          {{ workspace.name }}
        </span>
        <span class="workspace-index">{{ index + 1 }}</span>
      </div>
      <a-empty
        v-if="!orderedWorkspaces.length"
        :description="$t('workspace_noWorkspace')"
      />
    </div>
    <template #footer>
      <div class="order-footer">
        <a-checkbox v-if="canApplyToAll" v-model:checked="applyToAll">
          {{ $t("workspace_applyToAll") }}
        </a-checkbox>
        <span v-else />
        <a-space>
          <a-button :disabled="saving" @click="close">
            {{ $t("common_cancel") }}
          </a-button>
          <a-button
            type="primary"
            :loading="saving"
            :disabled="!orderedWorkspaces.length"
            @click="save"
          >
            {{ $t("common_save") }}
          </a-button>
        </a-space>
      </div>
    </template>
  </a-modal>
</template>

<script setup>
import { nextTick, onBeforeUnmount, ref, watch } from "vue";
import Sortable from "sortablejs";
import { HolderOutlined } from "@ant-design/icons-vue";

const props = defineProps({
  open: { type: Boolean, default: false },
  workspaces: { type: Array, default: () => [] },
  iconUrls: { type: Object, default: () => ({}) },
  canApplyToAll: { type: Boolean, default: false },
  saving: { type: Boolean, default: false },
});
const emit = defineEmits(["update:open", "save"]);

const listRef = ref(null);
const orderedWorkspaces = ref([]);
const applyToAll = ref(false);
let sortable;

function destroySortable() {
  sortable?.destroy();
  sortable = null;
}

function setupSortable() {
  destroySortable();
  if (!listRef.value || orderedWorkspaces.value.length < 2) return;
  sortable = Sortable.create(listRef.value, {
    animation: 160,
    handle: ".order-handle",
    draggable: ".order-item",
    ghostClass: "order-item-ghost",
    onEnd() {
      const byId = new Map(
        orderedWorkspaces.value.map((workspace) => [
          String(workspace.id),
          workspace,
        ]),
      );
      orderedWorkspaces.value = [
        ...listRef.value.querySelectorAll(".order-item"),
      ]
        .map((element) => byId.get(element.dataset.workspaceId))
        .filter(Boolean);
    },
  });
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      destroySortable();
      return;
    }
    orderedWorkspaces.value = [...props.workspaces];
    applyToAll.value = false;
    nextTick(setupSortable);
  },
);

watch(
  () => props.saving,
  (saving) => sortable?.option("disabled", saving),
);

onBeforeUnmount(destroySortable);

function close() {
  if (!props.saving) emit("update:open", false);
}

function save() {
  if (props.saving) return;
  emit("save", {
    ids: orderedWorkspaces.value.map((workspace) => Number(workspace.id)),
    apply_to_all: props.canApplyToAll && applyToAll.value,
  });
}
</script>

<style scoped lang="less">
.order-hint {
  margin: 0 0 16px;
  color: var(--app-text-secondary, #667085);
}

.order-list {
  display: flex;
  max-height: 420px;
  flex-direction: column;
  gap: 8px;
  overflow-y: auto;
}

.order-item {
  display: flex;
  min-height: 60px;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-radius: 8px;
  background: var(--app-surface-subtle, #f7f9fc);
}

.order-handle {
  color: var(--app-text-tertiary, #98a2b3);
  cursor: grab;
  font-size: 18px;
}

.workspace-icon,
.workspace-icon-image {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 7px;
}

.workspace-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 600;
}

.workspace-icon-image {
  object-fit: cover;
}

.workspace-name {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--app-text, #1d2939);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workspace-index {
  color: var(--app-text-tertiary, #98a2b3);
}

.order-item-ghost {
  opacity: 0.45;
}

.order-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
