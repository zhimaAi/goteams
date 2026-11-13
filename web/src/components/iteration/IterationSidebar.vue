<!-- IterationSidebar.vue: resizable, collapsible sidebar with selectable iteration cards. -->
<template>
  <aside
    class="iteration-sidebar"
    :class="{ collapsed, 'is-resizing': resizing }"
  >
    <div v-show="!collapsed" class="iteration-sidebar__content">
      <div class="iteration-sidebar__header">
        <div class="iteration-sidebar__title">
          <strong>{{ $t("iteration_title") }}</strong>
          <span>{{ iterations.length }}</span>
        </div>
        <a-button v-if="canCreate" type="primary" @click="$emit('create')">
          <PlusOutlined /> {{ $t("workspace_newVersion") }}
        </a-button>
      </div>
      <div class="iteration-sidebar__list">
        <a-empty
          v-if="!iterations.length"
          :description="$t('requirement_noVersionData')"
          :image="simpleImage"
          class="iteration-sidebar__empty"
        />
        <a-card
          v-for="iteration in iterations"
          :key="iteration.id"
          size="small"
          hoverable
          class="iteration-card"
          :class="{
            'iteration-card--active': Number(activeId) === Number(iteration.id),
            'iteration-card--ended': iteration.is_ended,
          }"
          @click="$emit('select', iteration.id)"
        >
          <div class="iteration-card__top">
            <IterationLabel
              :name="iteration.name"
              :cross-workspace="iteration.iteration_type === 'cross'"
              class="iteration-card__name"
            />
            <div
              v-if="iteration.is_ended || canEdit || canDelete"
              class="iteration-card__actions"
            >
              <span v-if="iteration.is_ended" class="iteration-card__ended">
                {{ $t("requirement_ended") }}
              </span>
              <a-dropdown
                v-if="canEdit || canDelete"
                trigger="click"
                placement="bottomRight"
              >
                <a-button
                  type="text"
                  size="small"
                  :aria-label="$t('common_more')"
                  @click.stop
                >
                  <MoreOutlined />
                </a-button>
                <template #overlay>
                  <a-menu @click="({ key }) => $emit('action', key, iteration)">
                    <a-menu-item v-if="canEdit" key="edit">
                      <EditOutlined /> {{ $t("common_edit") }}
                    </a-menu-item>
                    <a-menu-item
                      v-if="canEdit"
                      :key="iteration.is_ended ? 'reopen' : 'end'"
                    >
                      <ReloadOutlined v-if="iteration.is_ended" />
                      <StopOutlined v-else />
                      {{
                        $t(
                          iteration.is_ended
                            ? "iteration_reopen"
                            : "iteration_end",
                        )
                      }}
                    </a-menu-item>
                    <a-menu-divider v-if="canEdit && canDelete" />
                    <a-menu-item v-if="canDelete" key="delete" danger>
                      <DeleteOutlined /> {{ $t("common_delete") }}
                    </a-menu-item>
                  </a-menu>
                </template>
              </a-dropdown>
            </div>
          </div>
          <div class="iteration-card__date">
            {{ formatLocalDate(iteration.start_date) }} ~
            {{ formatLocalDate(iteration.end_date) }}
          </div>
          <div class="iteration-card__progress">
            <a-progress
              :percent="progress(iteration)"
              size="small"
              :show-info="false"
            />
            <span>{{ progress(iteration) }}%</span>
            <span>
              {{ Number(iteration.completed_count) || 0 }}/{{
                Number(iteration.requirement_count) || 0
              }}
            </span>
          </div>
        </a-card>
      </div>
    </div>

    <span
      v-if="!collapsed"
      class="iteration-sidebar__resizer"
      role="separator"
      aria-orientation="vertical"
      :aria-valuenow="width"
      aria-valuemin="220"
      aria-valuemax="420"
      @pointerdown.prevent="startResize"
    />

    <button
      type="button"
      class="iteration-sidebar__collapse-handle"
      :class="{ collapsed }"
      :aria-label="collapsed ? $t('common_expand') : $t('common_collapse')"
      @click="toggleCollapsed"
    >
      <RightOutlined v-if="collapsed" />
      <LeftOutlined v-else />
    </button>
  </aside>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Empty } from "ant-design-vue";
import {
  DeleteOutlined,
  EditOutlined,
  LeftOutlined,
  MoreOutlined,
  PlusOutlined,
  ReloadOutlined,
  RightOutlined,
  StopOutlined,
} from "@ant-design/icons-vue";
import IterationLabel from "./IterationLabel.vue";
import { formatLocalDate } from "@/utils/date";
import { useAuthStore } from "@/stores/auth";
import {
  readScopedListPreferences,
  writeScopedListPreferences,
} from "@/utils/boardPreferences";

const MIN_WIDTH = 220;
const MAX_WIDTH = 420;
const DEFAULT_WIDTH = 320;

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  iterations: { type: Array, default: () => [] },
  activeId: { type: [String, Number], default: null },
  canCreate: { type: Boolean, default: false },
  canEdit: { type: Boolean, default: false },
  canDelete: { type: Boolean, default: false },
});
defineEmits({
  create: () => true,
  select: (id) => id !== undefined,
  action: (key, iteration) => Boolean(key && iteration),
});
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const authStore = useAuthStore();

function clampWidth(value) {
  const width = Number(value);
  return Number.isFinite(width)
    ? Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, width))
    : DEFAULT_WIDTH;
}

// Sidebar width and collapse state are remembered per user + workspace.
function readStoredState() {
  const stored = readScopedListPreferences(
    authStore.userInfo?.id,
    props.wsId,
    "iteration",
  );
  return {
    width: clampWidth(stored.iterationSidebarWidth),
    collapsed: stored.iterationSidebarCollapsed === true,
  };
}

const width = ref(readStoredState().width);
const collapsed = ref(readStoredState().collapsed);
const resizing = ref(false);
const widthCss = computed(() => `${width.value}px`);
let resizeStartX = 0;
let resizeStartWidth = DEFAULT_WIDTH;

watch(
  () => props.wsId,
  () => {
    const stored = readStoredState();
    width.value = stored.width;
    collapsed.value = stored.collapsed;
  },
);

function persistState() {
  writeScopedListPreferences(authStore.userInfo?.id, props.wsId, "iteration", {
    iterationSidebarWidth: width.value,
    iterationSidebarCollapsed: collapsed.value,
  });
}

function toggleCollapsed() {
  collapsed.value = !collapsed.value;
  persistState();
}

function startResize(event) {
  if (collapsed.value) return;
  resizeStartX = event.clientX;
  resizeStartWidth = width.value;
  resizing.value = true;
  document.body.classList.add("iteration-sidebar-resizing");
  window.addEventListener("pointermove", handleResize);
  window.addEventListener("pointerup", stopResize, { once: true });
}

function handleResize(event) {
  width.value = Math.max(
    MIN_WIDTH,
    Math.min(MAX_WIDTH, resizeStartWidth + event.clientX - resizeStartX),
  );
}

function stopResize() {
  resizing.value = false;
  document.body.classList.remove("iteration-sidebar-resizing");
  window.removeEventListener("pointermove", handleResize);
  persistState();
}

onBeforeUnmount(() => {
  document.body.classList.remove("iteration-sidebar-resizing");
  window.removeEventListener("pointermove", handleResize);
  window.removeEventListener("pointerup", stopResize);
});

function progress(iteration) {
  const total = Number(iteration.requirement_count) || 0;
  const completed = Number(iteration.completed_count) || 0;
  return total > 0 ? Math.round((completed / total) * 100) : 0;
}
</script>

<style scoped lang="less">
.iteration-sidebar {
  position: relative;
  z-index: 6;
  display: flex;
  width: v-bind(widthCss);
  min-width: v-bind(widthCss);
  flex-direction: column;
  border-right: 1px solid #f0f0f0;
  background: var(--app-surface, #fff);
  margin-right: 16px;
  transition:
    width 0.2s ease,
    min-width 0.2s ease;
}

.iteration-sidebar.collapsed {
  width: 0;
  min-width: 0;
  border-right-color: transparent;
}

.iteration-sidebar.is-resizing {
  transition: none;
}

.iteration-sidebar__content {
  display: flex;
  width: v-bind(widthCss);
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
}

.iteration-sidebar__resizer {
  position: absolute;
  z-index: 8;
  top: 0;
  right: -3px;
  width: 6px;
  height: 100%;
  cursor: col-resize;
}

.iteration-sidebar__resizer::after {
  position: absolute;
  top: 0;
  right: 2px;
  width: 2px;
  height: 100%;
  background: #1677ff;
  content: "";
  opacity: 0;
  transition: opacity 0.15s ease;
}

.iteration-sidebar__resizer:hover::after,
:global(body.iteration-sidebar-resizing) .iteration-sidebar__resizer::after {
  opacity: 1;
}

.iteration-sidebar__collapse-handle {
  position: absolute;
  z-index: 12;
  top: 50%;
  right: -11px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 44px;
  padding: 0;
  border: 1px solid var(--app-border, #f0f0f0);
  border-left: 0;
  border-radius: 0 7px 7px 0;
  color: var(--app-text-secondary, rgba(0, 0, 0, 0.45));
  background: var(--app-surface, #fff);
  box-shadow: 2px 0 5px rgb(31 45 61 / 7%);
  font-size: 10px;
  cursor: pointer;
  transform: translateY(-50%);
}

.iteration-sidebar__collapse-handle.collapsed {
  right: auto;
  left: -16px;
  border-left: 1px solid var(--app-border, #f0f0f0);
}

.iteration-sidebar__collapse-handle:hover,
.iteration-sidebar__collapse-handle:focus-visible {
  color: #1677ff;
  background: color-mix(in srgb, #1677ff 6%, var(--app-surface, #fff));
  outline: none;
}

.iteration-sidebar__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 16px;
  border-bottom: 1px solid #f0f0f0;
}

.iteration-sidebar__title {
  display: flex;
  align-items: center;
  gap: 8px;

  span {
    color: rgba(0, 0, 0, 0.45);
  }
}

.iteration-sidebar__list {
  min-height: 0;
  flex: 1;
  padding: 8px 16px 16px;
  overflow-y: auto;
}

.iteration-sidebar__empty {
  margin-top: 64px;
}

.iteration-card {
  margin-bottom: 8px;
  border-radius: 8px;
}

.iteration-card--active {
  border-color: #1677ff;
  background: #eaf3ff;
}

.iteration-card--ended {
  opacity: 0.72;
}

.iteration-card__top,
.iteration-card__actions,
.iteration-card__progress {
  display: flex;
  align-items: center;
}

.iteration-card__top {
  justify-content: space-between;
  gap: 8px;
}

.iteration-card__name {
  min-width: 0;
  flex: 1;
  font-weight: 600;
}

.iteration-card__actions {
  flex: none;
  gap: 4px;
}

.iteration-card__ended {
  padding: 1px 5px;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
}

.iteration-card__date {
  margin: 8px 0;
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
}

.iteration-card__progress {
  gap: 8px;

  :deep(.ant-progress) {
    min-width: 80px;
    flex: 1;
  }

  span {
    flex: none;
    color: rgba(0, 0, 0, 0.65);
    font-size: 12px;
  }
}
</style>
