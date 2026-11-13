<template>
  <a-modal
    :open="open"
    :title="$t('draft_history_cache', { count: nodeCount })"
    :footer="null"
    :width="720"
    :z-index="1300"
    :destroy-on-close="true"
    @cancel="emit('close')"
  >
    <div class="draft-modal">
      <div class="draft-modal-left">
        <template v-for="record in visibleRecords" :key="record.key">
          <div class="draft-group">
            <div
              class="draft-node draft-top"
              :class="{
                active: isActive(record, null),
                collapsed: expandedKey !== record.key,
              }"
              @click="selectTop(record)"
            >
              <span
                v-if="sortedVersions(record).length"
                class="draft-fold"
                aria-hidden="true"
              />
              <span v-else class="draft-fold-placeholder" />
              <span class="draft-name">{{
                record.title || $t("draft_untitled")
              }}</span>
              <span class="draft-time">{{ formatTime(record.savedAt) }}</span>
            </div>
            <div
              v-if="expandedKey === record.key && sortedVersions(record).length"
              class="draft-versions"
            >
              <div
                v-for="version in sortedVersions(record)"
                :key="version.savedAt"
                class="draft-node draft-version"
                :class="{ active: isActive(record, version) }"
                @click="select(record, version)"
              >
                <span class="draft-dot" />
                <span class="draft-time">{{
                  formatTime(version.savedAt)
                }}</span>
              </div>
            </div>
          </div>
        </template>
        <a-empty
          v-if="!visibleRecords.length"
          :image="simpleImage"
          :description="$t('draft_empty')"
        />
      </div>
      <div v-if="activeContent" class="draft-modal-right">
        <div class="draft-preview-title">
          {{ activeContent.title || $t("draft_untitled") }}
        </div>
        <div class="draft-preview-time">
          {{ formatTime(activeContent.savedAt) }}
        </div>
        <div class="draft-preview-body">
          <RichTextEditor
            :model-value="activeContent.html"
            read-only
            :min-height="160"
          />
        </div>
      </div>
      <div v-else class="draft-modal-right draft-empty-preview">
        <a-empty :image="simpleImage" :description="$t('draft_empty')" />
      </div>
    </div>
    <div class="draft-modal-footer">
      <div class="draft-footer-left">
        <span class="draft-expire-tip">{{
          $t("draft_cleared_after_days")
        }}</span>
        <a-checkbox v-model:checked="autoRestore">{{
          $t("draft_auto_restore")
        }}</a-checkbox>
      </div>
      <div class="draft-modal-buttons">
        <a-button @click="emit('close')">{{ $t("common_cancel") }}</a-button>
        <a-button type="primary" :disabled="!active" @click="handleRestore">
          {{ $t("draft_restore") }}
        </a-button>
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { Empty } from "ant-design-vue";
import RichTextEditor from "./RichTextEditor.vue";
import { formatLocalDate } from "@/utils/date";
import {
  isDraftAutoRestoreEnabled,
  setDraftAutoRestoreEnabled,
} from "@/utils/localDraftPreference";

const props = defineProps({
  open: { type: Boolean, default: false },
  // 已按 savedAt 倒序的可见草稿列表（由页面调用 draft.list() 取得）
  records: { type: Array, default: () => [] },
});
const emit = defineEmits(["close", "restore"]);
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const visibleRecords = computed(() =>
  Array.isArray(props.records) ? props.records : [],
);

// 当前选中的节点：{ record, version|null }，version 为 null 表示顶层最新内容
const active = ref(null);
const activeContent = computed(() =>
  active.value ? active.value.version || active.value.record : null,
);
// 同时只展开一个顶层草稿
const expandedKey = ref("");
// 异常关闭时自动恢复：浏览器本地偏好，默认开启
const autoRestore = ref(isDraftAutoRestoreEnabled());
watch(autoRestore, (enabled) => setDraftAutoRestoreEnabled(enabled));

watch(
  visibleRecords,
  (records) => {
    active.value = records.length
      ? { record: records[0], version: null }
      : null;
    expandedKey.value = records[0]?.key || "";
  },
  { immediate: true },
);

// 数量 = 顶层最新内容 + 全部历史版本
const nodeCount = computed(() =>
  visibleRecords.value.reduce(
    (count, record) => count + 1 + (record.versions?.length || 0),
    0,
  ),
);

// 记录内版本新值在数组末尾，展示时最新在上
function sortedVersions(record) {
  return [...(record.versions || [])].reverse();
}

function isActive(record, version) {
  return (
    active.value &&
    active.value.record === record &&
    active.value.version === version
  );
}

function selectTop(record) {
  active.value = { record, version: null };
  // 选择当前顶层时展开自己，其他顶层自动折叠
  expandedKey.value = record.key;
}

function select(record, version) {
  active.value = { record, version };
  expandedKey.value = record.key;
}

function formatTime(timestamp) {
  const text = formatLocalDate(Math.floor(Number(timestamp) / 1000), true);
  return text === "-" ? "-" : text.slice(0, 16);
}

function handleRestore() {
  if (!active.value) return;
  const { record, version } = active.value;
  // 恢复该时间点自己的标题、正文和编辑器模式；来源草稿保持不变
  const source = version || record;
  emit("restore", {
    title: source.title,
    html: source.html,
    mode: source.mode || "",
  });
  emit("close");
}
</script>

<style scoped lang="less">
.draft-modal {
  display: flex;
  gap: 16px;
  min-height: 320px;
}
.draft-modal-left {
  flex: 0 0 280px;
  max-height: 420px;
  overflow: auto;
  padding-right: 4px;
  border-right: 1px solid #edeef1;
}
.draft-group {
  margin-bottom: 4px;
}
.draft-node {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;

  &:hover {
    background: #f5f7fa;
  }
  &.active {
    background: #d7e6fe;
  }
}
.draft-fold,
.draft-fold-placeholder {
  flex: 0 0 10px;
  width: 10px;
}
.draft-fold {
  height: 0;
  border-top: 5px solid transparent;
  border-bottom: 5px solid transparent;
  border-left: 6px solid #8c95a8;
  transform: rotate(90deg);
  transition: transform 0.12s ease;
}
.draft-top.collapsed .draft-fold {
  transform: rotate(0deg);
}
.draft-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: #182b50;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.draft-time {
  flex-shrink: 0;
  color: #8c95a8;
}
.draft-version {
  padding-left: 26px;
  color: #8c95a8;
}
.draft-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #8c95a8;
}
.draft-modal-right {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}
.draft-empty-preview {
  align-items: center;
  justify-content: center;
}
.draft-preview-title {
  height: 38px;
  overflow: hidden;
  border-bottom: 1px solid #edeef1;
  color: #182b50;
  font-size: 14px;
  font-weight: 700;
  line-height: 38px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.draft-preview-time {
  padding: 8px 0 4px;
  color: #8c95a8;
  font-size: 12px;
}
.draft-preview-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.draft-modal-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 12px -24px -16px;
  padding: 12px 24px;
  border-top: 1px solid #edeef1;
}
.draft-footer-left {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 12px;
}
.draft-expire-tip {
  color: #8c95a8;
  font-size: 12px;
}
.draft-modal-buttons {
  display: flex;
  gap: 8px;
}
</style>
