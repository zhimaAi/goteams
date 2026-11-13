<template>
  <a-modal
    :open="open"
    :title="$t('workspace_listSettings')"
    :footer="null"
    width="570px"
    centered
    wrap-class-name="team-work-settings-modal"
    @update:open="$emit('update:open', $event)"
  >
    <section class="settings-section">
      <h3>{{ $t("workspace_requirementPreviewMode") }}</h3>
      <a-radio-group
        :value="previewMode"
        class="mode-group"
        button-style="solid"
        @change="handlePreviewModeChange"
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
        @change="handleViewModeChange"
      >
        <a-radio-button value="list">
          <UnorderedListOutlined /> {{ $t("workspace_listView") }}
        </a-radio-button>
        <a-radio-button value="board">
          <AppstoreOutlined /> {{ $t("workspace_boardView") }}
        </a-radio-button>
      </a-radio-group>
    </section>

    <section class="settings-section member-settings-section">
      <div class="settings-heading">
        <h3>
          {{
            $t(
              viewMode === "board"
                ? "performance_boardMemberDisplay"
                : "performance_listMemberDisplay",
            )
          }}
        </h3>
        <div class="heading-actions">
          <a-spin v-if="saving" size="small" />
          <a-button type="default" size="small" @click="pickerOpen = true">
            {{ $t("workspace_addMember") }}
          </a-button>
        </div>
      </div>
      <p class="settings-hint">
        {{ $t("performance_visibilitySettingsHint") }}
      </p>

      <a-spin :spinning="loading">
        <div ref="memberListRef" class="member-list">
          <a-empty
            v-if="!loading && displayedMembers.length === 0"
            :description="$t('common_noData')"
            class="settings-empty"
          />
          <div
            v-for="member in displayedMembers"
            :key="member.id"
            class="member-item"
            :data-member-id="member.id"
          >
            <a-tooltip :title="$t('performance_dragToSort')">
              <span class="drag-handle"><HolderOutlined /></span>
            </a-tooltip>
            <a-avatar
              :size="32"
              :src="member.avatar || undefined"
              class="member-avatar"
            >
              {{ memberInitial(member) }}
            </a-avatar>
            <span class="member-name" :title="member.name">
              {{ member.name }}
            </span>
            <a-tooltip
              :title="$t('performance_removeMember', { name: member.name })"
            >
              <a-button
                type="text"
                size="small"
                class="member-remove"
                :aria-label="
                  $t('performance_removeMember', { name: member.name })
                "
                @click="handleMemberRemove(member.id)"
              >
                <CloseOutlined />
              </a-button>
            </a-tooltip>
          </div>
        </div>
      </a-spin>

      <EnterpriseMemberPickerModal
        v-model:open="pickerOpen"
        :candidates="listTeamWorkMemberCandidates"
        :saving="saving"
        @confirm="handleMembersAdd"
      />
    </section>
  </a-modal>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import {
  AppstoreOutlined,
  CloseOutlined,
  ExportOutlined,
  HolderOutlined,
  MenuUnfoldOutlined,
  UnorderedListOutlined,
} from "@ant-design/icons-vue";
import Sortable from "sortablejs";
import EnterpriseMemberPickerModal from "@/components/member/EnterpriseMemberPickerModal.vue";
import { listTeamWorkMemberCandidates } from "@/api/workspace";

const props = defineProps({
  open: { type: Boolean, default: false },
  previewMode: { type: String, default: "drawer" },
  viewMode: { type: String, default: "list" },
  members: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  saving: { type: Boolean, default: false },
});

const emit = defineEmits([
  "update:open",
  "preview-mode-change",
  "view-mode-change",
  "members-change",
]);

const pickerOpen = ref(false);
const memberListRef = ref(null);
let memberSortable = null;
// Set while Sortable owns the rows; the list must not be torn down mid-drag.
let draggingMember = false;
// The list shows the members that are actually displayed; removing one just drops it from here.
const displayedMembers = computed(() =>
  props.members.filter((member) => member.visible),
);

// The saved snapshot holds every member, so hidden ones are re-appended by id to keep the
// payload accepted by the backend. They are re-flagged here: the members left out of the
// displayed list are exactly the ones that must stop showing.
function toFullList(visibleMembers) {
  const visibleIds = new Set(visibleMembers.map((member) => Number(member.id)));
  const hidden = props.members
    .filter((member) => !visibleIds.has(Number(member.id)))
    .sort((left, right) => Number(left.id) - Number(right.id))
    .map((member) => ({ ...member, visible: false }));
  return [...visibleMembers, ...hidden];
}

function handlePreviewModeChange(event) {
  emit("preview-mode-change", event.target.value);
}

function handleViewModeChange(event) {
  emit("view-mode-change", event.target.value);
}

function memberInitial(member) {
  return (member.name || member.username || "?").trim().charAt(0).toUpperCase();
}

function handleMemberRemove(memberId) {
  emit(
    "members-change",
    toFullList(
      displayedMembers.value.filter(
        (member) => Number(member.id) !== Number(memberId),
      ),
    ),
  );
}

function handleMembersAdd(userIds) {
  const added = new Set((userIds || []).map(Number));
  const next = [...displayedMembers.value];
  for (const member of props.members) {
    if (!added.has(Number(member.id))) continue;
    if (next.some((item) => Number(item.id) === Number(member.id))) continue;
    next.push({ ...member, visible: true });
  }
  emit("members-change", toFullList(next));
  pickerOpen.value = false;
}

function destroyMemberSortable() {
  memberSortable?.destroy();
  memberSortable = null;
}

function setupMemberSortable() {
  destroyMemberSortable();
  // Never tear the instance down mid-drag: the rows can change while a drag is running.
  if (draggingMember) return;
  if (!memberListRef.value || displayedMembers.value.length < 2) return;
  memberSortable = Sortable.create(memberListRef.value, {
    handle: ".drag-handle",
    draggable: ".member-item",
    animation: 150,
    // Native HTML5 drag inside the overflow modal does not move rows; fallback does.
    forceFallback: true,
    fallbackOnBody: true,
    ghostClass: "member-item-ghost",
    onStart() {
      draggingMember = true;
    },
    onEnd() {
      draggingMember = false;
      // Read the new order from the DOM by member id: the displayed list is a subset of the
      // saved snapshot, so array indexes do not line up.
      const byId = new Map(
        displayedMembers.value.map((member) => [String(member.id), member]),
      );
      const next = [...memberListRef.value.querySelectorAll(".member-item")]
        .map((element) => byId.get(element.dataset.memberId))
        .filter(Boolean);
      emit("members-change", toFullList(next));
    },
  });
}

watch(
  () => props.open,
  (open) => {
    if (!open) {
      destroyMemberSortable();
      return;
    }
    nextTick(setupMemberSortable);
  },
);

// Adding or removing a member changes the row count, so the instance is rebuilt once it lands.
// A pure reorder leaves the count alone and keeps the instance Sortable already moved.
watch(
  () => displayedMembers.value.length,
  () => nextTick(setupMemberSortable),
);

// Saving replaces the snapshot the DOM was sorted against; disable the instance until it settles.
watch(
  () => props.saving,
  (saving) => {
    if (saving) memberSortable?.option("disabled", true);
    else nextTick(setupMemberSortable);
  },
);

onBeforeUnmount(destroyMemberSortable);
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

.settings-heading,
.heading-actions {
  display: flex;
  align-items: center;
}

.settings-heading {
  justify-content: space-between;
  gap: 16px;

  h3 {
    margin-bottom: 0;
  }
}

.heading-actions {
  flex: none;
  gap: 8px;
}

.settings-hint {
  margin: 8px 0 12px;
  color: var(--app-text-tertiary, #98a3b5);
  font-size: 13px;
}

.member-list {
  max-height: min(400px, calc(100vh - 390px));
  overflow-y: auto;
  border: 1px solid var(--app-border);
  border-radius: 8px;
}

.settings-empty {
  margin: 32px 0;
}

.member-item {
  display: flex;
  min-height: 60px;
  align-items: center;
  gap: 10px;
  padding: 8px 16px 8px 8px;
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

.member-item-ghost {
  opacity: 0.45;
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

.member-avatar {
  flex: none;
  color: #fff;
  background: #315bea;
  font-size: 12px;
}

.member-remove {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  color: #98a3b5;

  &:hover {
    color: #cf1322;
  }
}

.member-name {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.team-work-settings-modal .ant-modal-content) {
  padding: 20px 24px 24px;
  border-radius: 8px;
}

:global(.team-work-settings-modal .ant-modal-header) {
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
