<template>
  <a-modal
    :open="open"
    :title="$t('workspace_listSettings')"
    :footer="null"
    width="570px"
    class="my-work-settings-modal"
    @update:open="$emit('update:open', $event)"
  >
    <section class="settings-section">
      <h3>{{ $t("workspace_requirementPreviewMode") }}</h3>
      <a-radio-group
        :value="previewMode"
        class="mode-group"
        button-style="solid"
        @change="$emit('preview-mode-change', $event.target.value)"
      >
        <a-radio-button value="drawer">
          <MenuUnfoldOutlined /> {{ $t("workspace_previewDrawer") }}
        </a-radio-button>
        <a-radio-button value="modal">
          <AppstoreOutlined /> {{ $t("workspace_previewModal") }}
        </a-radio-button>
        <a-radio-button value="window">
          <ExportOutlined /> {{ $t("workspace_previewWindow") }}
        </a-radio-button>
      </a-radio-group>
    </section>

    <section class="settings-section">
      <h3>{{ $t("workspace_viewMode") }}</h3>
      <a-radio-group
        :value="viewMode"
        class="mode-group"
        button-style="solid"
        @change="$emit('view-mode-change', $event.target.value)"
      >
        <a-radio-button value="list">
          <UnorderedListOutlined /> {{ $t("workspace_listView") }}
        </a-radio-button>
        <a-radio-button value="board">
          <AppstoreOutlined /> {{ $t("workspace_boardView") }}
        </a-radio-button>
      </a-radio-group>
    </section>

    <section class="settings-section project-settings-section">
      <div class="settings-heading">
        <h3>{{ $t("myWork_projectColumnDisplay") }}</h3>
        <a-button type="default" size="small" @click="$emit('restore')">
          {{ $t("workspace_restoreDefault") }}
        </a-button>
      </div>
      <p class="settings-hint">{{ $t("myWork_projectColumnHint") }}</p>
      <div ref="projectListRef" class="project-list">
        <div
          v-for="project in projects"
          :key="project.id"
          class="project-item"
          :data-project-id="project.id"
        >
          <span class="drag-handle" :title="$t('myWork_dragProjectToSort')">
            <HolderOutlined />
          </span>
          <span
            class="project-color"
            :style="{ backgroundColor: project.color || '#0d68ff' }"
          />
          <span class="project-name" :title="project.name">
            {{ project.name }}
          </span>
          <a-switch
            :checked="!hiddenProjectIds.includes(String(project.id))"
            size="small"
            @change="(visible) => toggleProject(project.id, visible)"
          />
        </div>
        <a-empty
          v-if="!projects.length"
          :description="$t('myWork_noProjects')"
          class="settings-empty"
        />
      </div>
    </section>
  </a-modal>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import Sortable from "sortablejs";
import {
  AppstoreOutlined,
  ExportOutlined,
  HolderOutlined,
  MenuUnfoldOutlined,
  UnorderedListOutlined,
} from "@ant-design/icons-vue";

const props = defineProps({
  open: { type: Boolean, default: false },
  previewMode: { type: String, default: "drawer" },
  viewMode: { type: String, default: "list" },
  projects: { type: Array, default: () => [] },
  hiddenProjectIds: { type: Array, default: () => [] },
});
const emit = defineEmits([
  "update:open",
  "preview-mode-change",
  "view-mode-change",
  "project-preferences-change",
  "restore",
]);
const projectListRef = ref(null);
let sortable;

function destroySortable() {
  sortable?.destroy();
  sortable = null;
}

function emitProjectOrder() {
  const order = [...projectListRef.value.querySelectorAll(".project-item")]
    .map((element) => element.dataset.projectId)
    .filter(Boolean);
  emit("project-preferences-change", {
    projectOrder: order,
    hiddenProjectIds: [...props.hiddenProjectIds],
  });
}

function setupSortable() {
  destroySortable();
  const element = projectListRef.value;
  if (!element || !props.projects.length) return;
  sortable = Sortable.create(element, {
    animation: 160,
    handle: ".drag-handle",
    draggable: ".project-item",
    filter: ".ant-switch",
    preventOnFilter: true,
    ghostClass: "project-item-ghost",
    chosenClass: "project-item-chosen",
    dragClass: "project-item-drag",
    onEnd: emitProjectOrder,
  });
}

watch(
  () => [props.open, props.projects.map((project) => project.id).join(",")],
  () => nextTick(setupSortable),
);
onMounted(setupSortable);
onBeforeUnmount(destroySortable);

function toggleProject(projectId, visible) {
  const id = String(projectId);
  const hidden = new Set(props.hiddenProjectIds.map(String));
  if (visible) hidden.delete(id);
  else hidden.add(id);
  emit("project-preferences-change", {
    projectOrder: props.projects.map((project) => String(project.id)),
    hiddenProjectIds: [...hidden],
  });
}
</script>

<style scoped lang="less">
.settings-section {
  padding: 16px 6px 20px;

  & + & {
    border-top: 1px solid var(--app-border);
  }

  h3 {
    margin: 0 0 12px;
    color: var(--app-text);
    font-size: 14px;
    font-weight: 600;
  }
}

.mode-group {
  display: flex;
  width: 100%;

  :deep(.ant-radio-button-wrapper) {
    display: inline-flex;
    min-width: 0;
    flex: 1;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding-inline: 8px;
  }
}

.settings-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.settings-heading h3 {
  margin-bottom: 0;
}

.settings-hint {
  margin: 8px 0 12px;
  color: var(--app-text-tertiary, #98a3b5);
  font-size: 13px;
}

.project-list {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 8px;
}

.project-item {
  display: flex;
  min-height: 52px;
  align-items: center;
  gap: 10px;
  padding: 8px 16px 8px 8px;
  background: var(--app-surface);
  transition:
    opacity 0.15s,
    background-color 0.15s;

  & + & {
    border-top: 1px solid var(--app-border);
  }

  &:hover {
    background: var(--app-bg-subtle, #f7f9fc);
  }
}

.project-item-ghost {
  border: 1px dashed #91caff;
  background: #f0f7ff;
  opacity: 0.55;
}

.project-item-chosen {
  box-shadow: 0 3px 10px rgb(16 24 40 / 12%);
}

.project-item-drag {
  opacity: 0.9;
}

.drag-handle {
  display: inline-flex;
  width: 28px;
  height: 36px;
  flex: none;
  align-items: center;
  justify-content: center;
  color: #c4cbd5;
  cursor: grab;
  font-size: 16px;

  &:active {
    cursor: grabbing;
  }
}

.project-color {
  width: 12px;
  height: 12px;
  flex: none;
  border-radius: 50%;
}

.project-name {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.settings-empty {
  margin: 24px 0;
}

:global(.my-work-settings-modal .ant-modal-content) {
  padding: 20px 24px 24px;
  border-radius: 8px;
}

:global(.my-work-settings-modal .ant-modal-header) {
  margin-bottom: 0;
}

@media (max-width: 620px) {
  .mode-group :deep(.ant-radio-button-wrapper) {
    font-size: 12px;
  }

  .settings-section {
    padding-inline: 0;
  }
}
</style>
