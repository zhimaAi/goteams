<template>
  <div class="board-navigation-menu" :class="`board-navigation-menu--${style}`">
    <a-popover
      v-model:open="open"
      trigger="hover"
      placement="bottom"
      :mouse-enter-delay="0"
      :mouse-leave-delay="0.15"
      :destroy-tooltip-on-hide="false"
      overlay-class-name="board-navigation-overlay"
    >
      <button
        type="button"
        class="board-navigation-trigger"
        :class="{ active: open || activeOverflowTab }"
      >
        {{ moreLabel }} <DownOutlined />
      </button>
      <template #content>
        <div class="board-navigation-popover">
          <!-- Header: title + count + drag hint, style switcher, edit button -->
          <div class="navigation-header">
            <div class="navigation-title">
              <span class="navigation-title-group">
                <svg
                  class="navigation-icon"
                  xmlns="http://www.w3.org/2000/svg"
                  width="16"
                  height="16"
                  viewBox="0 0 16 16"
                  fill="none"
                >
                  <path
                    d="M4.47929 8.08286C3.81109 8.75271 2.66663 8.27948 2.66663 7.33333C2.66663 7.24444 2.68329 7.16111 2.71663 7.08333C2.74996 7.00556 2.79996 6.93333 2.86663 6.86667L7.53329 2.2C7.59996 2.13333 7.67218 2.08333 7.74996 2.05C7.82774 2.01667 7.91107 2 7.99996 2C8.08885 2 8.17218 2.01667 8.24996 2.05C8.32774 2.08333 8.39996 2.13333 8.46663 2.2C8.99012 2.71431 8.99437 3.55663 8.47608 4.0762L4.47929 8.08286ZM12.6666 13.3333C11.9302 13.3333 11.3333 12.7364 11.3333 12V8.96719C11.3333 8.27321 11.0576 7.60765 10.5669 7.11693L9.42398 5.97402C9.03332 5.58336 9.03332 4.94997 9.42398 4.55931C9.8154 4.16789 10.4503 4.16877 10.8406 4.56128L13.1333 6.86667C13.1888 6.93333 13.2362 7.00556 13.2753 7.08333C13.314 7.16111 13.3333 7.24444 13.3333 7.33333V12.6667C13.3333 12.8556 13.2693 13.0138 13.1413 13.1413C13.0137 13.2693 12.8555 13.3333 12.6666 13.3333ZM3.33329 13.3333C3.1444 13.3333 2.98596 13.2693 2.85796 13.1413C2.7304 13.0138 2.66663 12.8556 2.66663 12.6667V11.7142C2.66663 11.449 2.77198 11.1946 2.95952 11.0071L3.64996 10.3167C4.02514 9.94149 4.66663 10.2072 4.66663 10.7378C4.66663 11.0667 4.93326 11.3333 5.26218 11.3333H9.33329C9.88558 11.3333 10.3333 11.781 10.3333 12.3333C10.3333 12.8856 9.88558 13.3333 9.33329 13.3333H3.33329Z"
                    fill="currentColor"
                  />
                </svg>
                <strong
                  >{{ $t("workspace_mainNavigation") }} {{ tabs.length }}/{{
                    availableCount
                  }}</strong
                >
              </span>
              <span class="navigation-title-hint">{{
                $t("workspace_navigationDragHint")
              }}</span>
            </div>
            <div class="navigation-actions">
              <!-- Style switcher: 简约 / 极简 -->
              <div v-if="canEdit" class="navigation-style-switcher">
                <button
                  v-for="opt in styleOptions"
                  :key="opt.value"
                  type="button"
                  class="style-option"
                  :class="{ active: localStyle === opt.value }"
                  @click="handleStyleChange(opt.value)"
                >
                  {{ opt.label }}
                </button>
              </div>
              <!-- Edit button -->
              <button
                v-if="canEdit"
                type="button"
                class="navigation-edit-button"
                :class="{ editing }"
                :aria-label="$t('common_edit')"
                @click.stop="editing = !editing"
              >
                <CheckOutlined v-if="editing" /><EditOutlined v-else />
              </button>
            </div>
          </div>

          <!-- Main navigation list -->
          <div ref="mainListRef" class="navigation-list" :class="{ editing }">
            <div
              v-for="item in tabs"
              :key="item.key"
              class="navigation-item"
              :class="{ active: item.key === activeKey }"
              :data-navigation-key="item.key"
              @click="!editing && selectItem(item.key)"
            >
              <HolderOutlined v-if="editing" class="navigation-drag-handle" />
              <span>{{ item.label }}</span>
              <button
                v-if="editing && canEdit"
                type="button"
                class="navigation-remove"
                :aria-label="$t('workspace_navigationRemove')"
                @click.stop="removeItem(item.key)"
              >
                <CloseOutlined />
              </button>
            </div>
          </div>

          <!-- All applications section -->
          <div class="navigation-section-title">
            <svg
              class="navigation-icon"
              xmlns="http://www.w3.org/2000/svg"
              width="16"
              height="16"
              viewBox="0 0 16 16"
              fill="none"
            >
              <path
                d="M8.66667 5.33333V2.66667C8.66667 2.47778 8.73067 2.31956 8.85867 2.192C8.98667 2.06444 9.14489 2.00044 9.33333 2H13.3333C13.5222 2 13.6807 2.064 13.8087 2.192C13.9367 2.32 14.0004 2.47822 14 2.66667V5.33333C14 5.52222 13.936 5.68067 13.808 5.80867C13.68 5.93667 13.5218 6.00044 13.3333 6H9.33333C9.14444 6 8.98622 5.936 8.85867 5.808C8.73111 5.68 8.66711 5.52178 8.66667 5.33333ZM2 8V2.66667C2 2.47778 2.064 2.31956 2.192 2.192C2.32 2.06444 2.47822 2.00044 2.66667 2H6.66667C6.85556 2 7.014 2.064 7.142 2.192C7.27 2.32 7.33378 2.47822 7.33333 2.66667V8C7.33333 8.18889 7.26933 8.34733 7.14133 8.47533C7.01333 8.60333 6.85511 8.66711 6.66667 8.66667H2.66667C2.47778 8.66667 2.31956 8.60267 2.192 8.47467C2.06444 8.34667 2.00044 8.18844 2 8ZM8.66667 13.3333V8C8.66667 7.81111 8.73067 7.65289 8.85867 7.52533C8.98667 7.39778 9.14489 7.33378 9.33333 7.33333H13.3333C13.5222 7.33333 13.6807 7.39733 13.8087 7.52533C13.9367 7.65333 14.0004 7.81156 14 8V13.3333C14 13.5222 13.936 13.6807 13.808 13.8087C13.68 13.9367 13.5218 14.0004 13.3333 14H9.33333C9.14444 14 8.98622 13.936 8.85867 13.808C8.73111 13.68 8.66711 13.5218 8.66667 13.3333ZM2 13.3333V10.6667C2 10.4778 2.064 10.3196 2.192 10.192C2.32 10.0644 2.47822 10.0004 2.66667 10H6.66667C6.85556 10 7.014 10.064 7.142 10.192C7.27 10.32 7.33378 10.4782 7.33333 10.6667V13.3333C7.33333 13.5222 7.26933 13.6807 7.14133 13.8087C7.01333 13.9367 6.85511 14.0004 6.66667 14H2.66667C2.47778 14 2.31956 13.936 2.192 13.808C2.06444 13.68 2.00044 13.5218 2 13.3333ZM3.33333 7.33333H6V3.33333H3.33333V7.33333ZM10 12.6667H12.6667V8.66667H10V12.6667ZM10 4.66667H12.6667V3.33333H10V4.66667ZM3.33333 12.6667H6V11.3333H3.33333V12.6667Z"
                fill="currentColor"
              />
            </svg>
            <span>{{ $t("workspace_allApplications") }}</span>
            <button
              v-if="canAdmin"
              type="button"
              class="navigation-app-settings"
              @click.stop="openAppSettings"
            >
              {{ $t("workspace_appSettings") }}
            </button>
          </div>
          <div class="navigation-list all-applications">
            <button
              v-for="item in availableApps"
              :key="item.key"
              type="button"
              class="navigation-item all-application-item"
              :class="{ active: item.key === activeKey }"
              @click="selectItem(item.key)"
            >
              <span>{{ item.label }}</span>
              <PlusOutlined
                v-if="editing && canEdit"
                class="navigation-add"
                @click.stop="addItem(item.key)"
              />
            </button>
            <span v-if="!availableApps.length" class="navigation-empty">{{
              $t("common_empty")
            }}</span>
          </div>

          <!-- Footer: sort label, apply to all, prevent member edit -->
          <div v-if="canAdmin" class="navigation-footer">
            <span class="navigation-footer-label">{{
              $t("workspace_mainNavigationSort")
            }}</span>
            <div class="navigation-footer-actions">
              <a-button
                type="link"
                :loading="saving"
                class="navigation-apply-all"
                @click="applyAll"
                >{{ $t("workspace_applyToAllMembers") }}</a-button
              >
              <a-checkbox
                :checked="preventMemberEdit"
                class="navigation-prevent-checkbox"
                @change="togglePreventMemberEdit"
                >{{ $t("workspace_preventMemberEdit") }}</a-checkbox
              >
            </div>
          </div>
        </div>
      </template>
    </a-popover>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import Sortable from "sortablejs";
import {
  CheckOutlined,
  CloseOutlined,
  DownOutlined,
  EditOutlined,
  HolderOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";

const props = defineProps({
  tabs: { type: Array, default: () => [] },
  allApps: { type: Array, default: () => [] },
  availableCount: { type: Number, default: 0 },
  moreLabel: { type: String, default: "" },
  style: { type: String, default: "simple" },
  activeKey: { type: String, default: "" },
  canEdit: { type: Boolean, default: false },
  canAdmin: { type: Boolean, default: false },
  preventMemberEdit: { type: Boolean, default: false },
  saving: { type: Boolean, default: false },
});
const { t } = useI18n();
const emit = defineEmits([
  "select",
  "order-change",
  "style-change",
  "apply-all",
  "prevent-change",
  "app-settings",
]);
const open = ref(false);
const editing = ref(false);
const localStyle = ref(props.style);
const mainListRef = ref(null);
let sortable;
const styleOptions = computed(() => [
  { label: t("workspace_navigationSimple"), value: "simple" },
  { label: t("workspace_navigationMinimal"), value: "minimal" },
]);
const availableApps = computed(() =>
  props.allApps.filter(
    (item) => !props.tabs.some((tab) => tab.key === item.key),
  ),
);
const activeOverflowTab = computed(() =>
  availableApps.value.some((item) => item.key === props.activeKey),
);
function destroySortable() {
  sortable?.destroy();
  sortable = null;
}
function setupSortable() {
  destroySortable();
  if (!mainListRef.value || !editing.value || !props.canEdit) return;
  sortable = Sortable.create(mainListRef.value, {
    animation: 150,
    handle: ".navigation-drag-handle",
    draggable: ".navigation-item",
    ghostClass: "navigation-item-ghost",
    onEnd: () =>
      emit(
        "order-change",
        [...mainListRef.value.querySelectorAll("[data-navigation-key]")].map(
          (el) => el.dataset.navigationKey,
        ),
      ),
  });
}
watch(
  () => [props.tabs.map((item) => item.key).join(","), editing.value],
  () => nextTick(setupSortable),
);
watch(
  () => props.style,
  (value) => {
    localStyle.value = value;
  },
);
function selectItem(key) {
  if (editing.value) {
    addItem(key);
    return;
  }
  emit("select", key);
  open.value = false;
}
function removeItem(key) {
  if (props.tabs.length <= 1) return;
  emit(
    "order-change",
    props.tabs.filter((item) => item.key !== key).map((item) => item.key),
  );
}
function addItem(key) {
  emit("order-change", [...props.tabs.map((item) => item.key), key]);
}
function handleStyleChange(value) {
  localStyle.value = value;
  emit("style-change", value);
}
function applyAll() {
  emit("apply-all");
}
function openAppSettings() {
  open.value = false;
  emit("app-settings");
}
function togglePreventMemberEdit(event) {
  emit("prevent-change", event.target.checked);
}
onBeforeUnmount(destroySortable);
</script>

<style scoped lang="less">
.board-navigation-menu {
  position: relative;
  flex-shrink: 0;
}
.board-navigation-trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 30px;
  padding: 4px 12px;
  border: 0;
  background: transparent;
  color: rgba(0, 0, 0, 0.88);
  cursor: pointer;
  font-size: 14px;
  line-height: 22px;
}
.board-navigation-trigger :deep(.anticon) {
  font-size: 12px;
}
.board-navigation-menu--minimal {
  margin-left: 20px;
}
.board-navigation-menu--minimal .board-navigation-trigger {
  height: 46px;
  padding: 4px 0px;
}
.board-navigation-menu--minimal .board-navigation-trigger:hover,
.board-navigation-menu--minimal .board-navigation-trigger.active {
  color: #3157e2;
}
.board-navigation-menu--simple .board-navigation-trigger:hover,
.board-navigation-menu--simple .board-navigation-trigger.active {
  background: #e5efff;
  border-radius: 6px;
  color: #3157e2;
  font-weight: 600;
}

/* Popover container */
.board-navigation-popover {
  width: min(670px, calc(100vw - 32px));
  background: #fff;
}
:global(.board-navigation-overlay .ant-popover-inner) {
  padding: 0;
  overflow: hidden;
  border-radius: 12px;
  background: #fff;
  box-shadow:
    0 8px 10px -5px rgba(0, 0, 0, 0.08),
    0 16px 24px 2px rgba(0, 0, 0, 0.04),
    0 6px 30px 5px rgba(0, 0, 0, 0.05);
}

/* Header */
.navigation-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 16px 8px;
}
.navigation-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.navigation-title-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.navigation-title-group :deep(.anticon) {
  font-size: 16px;
  color: #000;
}
.navigation-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: #000;
}
.navigation-title-group strong {
  font-size: 14px;
  font-weight: 600;
  line-height: 22px;
  color: #000;
}
.navigation-title-hint {
  font-size: 12px;
  line-height: 20px;
  color: #8c8c8c;
}
.navigation-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

/* Style switcher (segmented control) */
.navigation-style-switcher {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: #edeff2;
}
.style-option {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 52px;
  padding: 2px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: #262626;
  font-size: 14px;
  line-height: 22px;
  font-weight: 400;
  cursor: pointer;
  white-space: nowrap;
}
.style-option.active {
  background: #fff;
  box-shadow: 0 1px 4px 0 rgba(35, 59, 129, 0.12);
  color: #2475fc;
  font-weight: 600;
}

/* Edit button */
.navigation-edit-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 6px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: transparent;
  color: #262626;
  cursor: pointer;
}
.navigation-edit-button :deep(.anticon) {
  font-size: 16px;
}
.navigation-edit-button.editing {
  background: #3157e2;
  border-color: #3157e2;
  color: #fff;
}

/* Navigation lists (main + all apps) */
.navigation-list {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 0 16px 20px;
}
.navigation-item {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border: 0;
  border-radius: 6px;
  background: #f2f4f7;
  color: #262626;
  cursor: pointer;
  font-size: 14px;
  line-height: 22px;
  font-weight: 400;
}
/* Hover (手型 cursor) */
.navigation-list:not(.editing) .navigation-item:hover,
.navigation-item.all-application-item:hover {
  background: #e5efff;
  color: #3157e2;
}
/* Active / selected tab */
.navigation-item.active {
  background: #e5efff;
  color: #3157e2;
}
.navigation-drag-handle {
  color: #8c8c8c;
  cursor: grab;
}
.navigation-remove {
  position: absolute;
  top: -8px;
  right: -8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 19px;
  height: 19px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: #98a3b5;
  color: #fff;
  cursor: pointer;
  font-size: 10px;
}
.navigation-add {
  color: #3157e2;
  cursor: pointer;
}

/* All applications section */
.navigation-section-title {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 16px 16px;
  color: #000;
  font-size: 14px;
  font-weight: 600;
  line-height: 22px;
}
.navigation-section-title :deep(.anticon),
.navigation-section-title .navigation-icon {
  font-size: 16px;
  color: #000;
}
.all-applications {
  padding: 0 16px 24px;
}
.navigation-app-settings {
  margin-left: 8px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #2475fc;
  font: inherit;
  font-size: 14px;
  font-weight: 400;
  line-height: 22px;
  cursor: pointer;

  &:hover {
    color: #3157e2;
  }
}
.all-application-item {
  border: 0;
}
.navigation-empty {
  color: #8c8c8c;
  font-size: 13px;
}

/* Footer */
.navigation-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: 46px;
  padding: 0 16px;
  border-top: 1px solid #f0f0f0;
}
.navigation-footer-label {
  color: #5c687d;
  font-size: 14px;
  line-height: 22px;
}
.navigation-footer-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.navigation-apply-all {
  color: #2475fc !important;
  font-size: 14px;
  padding-inline: 0;
}
.navigation-prevent-checkbox :deep(.ant-checkbox-wrapper) {
  color: #595959;
  font-size: 14px;
}
.navigation-prevent-checkbox :deep(.ant-checkbox-checked .ant-checkbox-inner) {
  background-color: #2475fc;
  border-color: #2475fc;
}

/* Sortable ghost */
:global(.navigation-item-ghost) {
  opacity: 0.5;
  background: #e5efff;
}

@media (max-width: 768px) {
  .navigation-header,
  .navigation-footer {
    flex-direction: column;
    align-items: flex-start;
  }
  .board-navigation-popover {
    width: calc(100vw - 32px);
  }
}
</style>
