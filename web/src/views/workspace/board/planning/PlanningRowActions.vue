<template>
  <a-dropdown
    v-model:open="open"
    :trigger="[]"
    placement="bottomLeft"
    @open-change="handleOpenChange"
  >
    <span class="row-menu-anchor" :style="anchorStyle" />
    <template #overlay>
      <div
        class="row-menu"
        data-planning-row-actions
        @click.stop
        @contextmenu.prevent
      >
        <div class="row-menu-item">
          <button type="button" class="row-menu-label" @click="insertBelow">
            <ArrowDownOutlined />
            <span>{{ $t("planning_insertBelow") }}</span>
          </button>
          <a-input
            :value="insertCount"
            size="small"
            class="row-menu-count"
            :title="$t('planning_insertMax')"
            @update:value="onInsertCount"
            @press-enter="insertBelow"
          />
          <span>{{ $t("planning_rowUnit") }}</span>
        </div>
        <div class="row-menu-item">
          <button type="button" class="row-menu-label" @click="moveToRow">
            <ArrowRightOutlined />
            <span>{{ $t("planning_moveToRow") }}</span>
          </button>
          <a-input-number
            v-model:value="targetRow"
            size="small"
            class="row-menu-count"
            :min="1"
            :precision="0"
            :controls="false"
            @press-enter="moveToRow"
          />
          <span>{{ $t("planning_rowUnit") }}</span>
        </div>
        <button type="button" class="row-menu-action" @click="moveToBottom">
          <VerticalAlignBottomOutlined />
          <span>{{ $t("planning_moveToBottom") }}</span>
        </button>
        <button type="button" class="row-menu-action" @click="toggleArchive">
          <InboxOutlined />
          <span>{{
            target?.is_archived
              ? $t("planning_unarchive")
              : $t("planning_archive")
          }}</span>
        </button>
        <button type="button" class="row-menu-delete" @click="remove">
          <DeleteOutlined />
          <span>{{ $t("common_delete") }}</span>
        </button>
      </div>
    </template>
  </a-dropdown>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  ArrowDownOutlined,
  ArrowRightOutlined,
  DeleteOutlined,
  InboxOutlined,
  VerticalAlignBottomOutlined,
} from "@ant-design/icons-vue";

const emit = defineEmits([
  "insert-below",
  "move-to-row",
  "move-to-bottom",
  "toggle-archive",
  "delete",
]);
const { t: $t } = useI18n();

const open = ref(false);
const x = ref(0);
const y = ref(0);
const insertCount = ref(1);
const targetRow = ref(1);
// The archive entry switches its label by row state, so the target row has to be reactive.
const target = ref(null);
let targetIndex = -1;

const anchorStyle = computed(() => ({
  left: `${x.value}px`,
  top: `${y.value}px`,
}));

async function openAt(record, index, event) {
  const rect = event.currentTarget?.getBoundingClientRect?.();
  open.value = false;
  await nextTick();
  target.value = record;
  targetIndex = index;
  insertCount.value = 1;
  targetRow.value = 1;
  x.value = event.clientX || rect?.left || 0;
  y.value = event.clientY || rect?.bottom || 0;
  open.value = true;
}

function close() {
  open.value = false;
}

function clampInsertCount(value) {
  return Math.min(50, Math.max(1, Number(value) || 1));
}

function onInsertCount(value) {
  const digits = String(value ?? "").replace(/[^\d]/g, "");
  if (digits === "") {
    insertCount.value = "";
    return;
  }
  const next = Number(digits);
  insertCount.value = next > 50 ? 50 : next;
}

function insertBelow() {
  const amount = clampInsertCount(insertCount.value);
  insertCount.value = amount;
  emit("insert-below", target.value, targetIndex, amount);
  close();
}

function moveToRow() {
  emit("move-to-row", target.value, targetRow.value || 1);
  close();
}

function remove() {
  emit("delete", target.value);
  close();
}

// Both entries only report the target row; the table owns the reorder and archive calls.
function moveToBottom() {
  emit("move-to-bottom", target.value);
  close();
}

function toggleArchive() {
  emit("toggle-archive", target.value);
  close();
}

function handleOpenChange(value) {
  if (!value) {
    target.value = null;
    targetIndex = -1;
  }
}

function handleDocumentMouseDown(event) {
  if (
    !open.value ||
    event.button !== 0 ||
    event.target?.closest?.("[data-planning-row-actions]")
  ) {
    return;
  }
  close();
}

onMounted(() =>
  document.addEventListener("mousedown", handleDocumentMouseDown),
);
onBeforeUnmount(() =>
  document.removeEventListener("mousedown", handleDocumentMouseDown),
);

defineExpose({ openAt, close });
</script>

<style scoped lang="less">
.row-menu-anchor {
  position: fixed;
  z-index: -1;
  width: 1px;
  height: 1px;
}
.row-menu {
  min-width: 250px;
  padding: 8px;
  border-radius: 8px;
  background: var(--app-surface);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.12);
  color: var(--app-text);
}
.row-menu-item {
  display: flex;
  align-items: center;
  height: 40px;
  gap: 8px;

  :deep(.row-menu-count) {
    width: 64px;
  }
}
.row-menu-label,
.row-menu-action,
.row-menu-delete {
  display: inline-flex;
  align-items: center;
  border: 0;
  background: transparent;
  cursor: pointer;
  gap: 8px;
  font: inherit;
}
.row-menu-label {
  flex: 1;
  padding: 8px;
  color: var(--app-text);
  text-align: left;
}
.row-menu-action,
.row-menu-delete {
  width: 100%;
  height: 40px;
  padding: 8px;
}
.row-menu-action {
  color: var(--app-text);
}
.row-menu-delete {
  color: #ff4d4f;
}
.row-menu-label:hover,
.row-menu-action:hover,
.row-menu-delete:hover {
  border-radius: 6px;
  background: var(--app-surface-muted);
}
</style>
