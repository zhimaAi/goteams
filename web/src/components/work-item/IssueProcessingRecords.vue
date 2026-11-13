<template>
  <div
    v-if="loading"
    class="issue-records-loading"
    :class="{ 'is-horizontal': horizontal }"
  >
    <a-spin />
  </div>
  <a-button v-else-if="!loaded" @click="$emit('retry')">{{
    $t("issue_retry")
  }}</a-button>
  <a-empty
    v-else-if="!records.length"
    :description="$t('issue_no_processing_records')"
  />
  <div
    v-else
    class="issue-timeline-scroll"
    :class="{ horizontal }"
    ref="scrollEl"
    @scroll="syncScroll"
    tabindex="0"
    role="region"
    :aria-label="$t('issue_processing_records')"
  >
    <ol class="issue-timeline">
      <li v-for="item in records" :key="item.id" class="issue-event">
        <div class="issue-event-rail" aria-hidden="true">
          <img :src="haloIcon" class="issue-event-halo" alt="" />
          <img :src="dotIcon" class="issue-event-dot" alt="" />
        </div>
        <div class="issue-event-avatar" aria-hidden="true">
          <img :src="userIcon" alt="" />
        </div>
        <div class="issue-event-content">
          <div class="issue-event-title" :title="item.user_name || '—'">
            {{ item.user_name || "—" }}
          </div>
          <div class="issue-event-details">
            <div class="issue-event-operation">
              {{ formatLocalDate(item.created_at, true) }}
              {{ $t(processingActionKey(item)) }}
            </div>
            <div>
              {{ $t("workspace_assignee") }}{{ $t("issue_field_separator")
              }}{{ processingAssigneeText(item) }}
            </div>
            <div class="issue-event-status">
              <span
                >{{ $t("workspace_status")
                }}{{ $t("issue_field_separator") }}</span
              ><StatusPill
                :text="processingStatusText(item)"
                :color="
                  item.processing_snapshot?.status_color ||
                  (isStatusEvent(item) ? statusColor(item) : '')
                "
              />
            </div>
            <div class="issue-event-note">
              <span
                >{{ $t("issue_processing_note")
                }}{{ $t("issue_field_separator") }}</span
              >
              <RichTextEditor
                v-if="item.processing_note"
                :model-value="item.processing_note"
                content-mode="rich-text"
                read-only
                plain
                :min-height="0"
              />
              <span v-else>—</span>
            </div>
          </div>
        </div>
      </li>
    </ol>
  </div>
  <div
    v-if="horizontal && loaded && !loading && records.length"
    class="issue-scroll-track"
    aria-hidden="true"
  >
    <div :style="scrollThumbStyle" />
  </div>
</template>
<script setup>
import { nextTick, ref, watch } from "vue";
import { processingActionKey } from "@/utils/issueProcessingRecords";
import { formatLocalDate } from "@/utils/date";
import StatusPill from "./StatusPill.vue";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import haloIcon from "@/image/issue-detail/halo.svg";
import dotIcon from "@/image/issue-detail/dot.svg";
import userIcon from "@/image/issue-detail/user.svg";
const props = defineProps({
  records: { type: Array, default: () => [] },
  loading: Boolean,
  loaded: Boolean,
  horizontal: Boolean,
  valueText: { type: Function, required: true },
  statusColor: { type: Function, required: true },
});
defineEmits(["retry"]);
function isStatusEvent(item) {
  return item.action === "status_change" || item.action === "issue_recall";
}
function processingStatusText(item) {
  if (item.processing_snapshot)
    return item.processing_snapshot.status_name || "—";
  return isStatusEvent(item) ? props.valueText(item, item.new_value) : "—";
}
function processingAssigneeText(item) {
  const names = item.processing_snapshot?.assignee_names ?? item.assignee_names;
  return Array.isArray(names) && names.length ? names.join("、") : "—";
}
const scrollEl = ref(null);
const scrollThumbStyle = ref({ width: "100%", marginLeft: "0%" });
function syncScroll() {
  const el = scrollEl.value;
  if (!el?.scrollWidth) return;
  scrollThumbStyle.value = {
    width: `${(el.clientWidth / el.scrollWidth) * 100}%`,
    marginLeft: `${(el.scrollLeft / el.scrollWidth) * 100}%`,
  };
}
watch(scrollEl, (el, _, onCleanup) => {
  if (!el) return;
  const observer = new ResizeObserver(syncScroll);
  observer.observe(el);
  onCleanup(() => observer.disconnect());
});
watch(
  () => [props.records.length, props.horizontal],
  () => nextTick(syncScroll),
);
</script>
<style scoped lang="less">
.issue-records-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  height: calc(100% - 42px);
  min-height: 238px;
}
.issue-records-loading.is-horizontal {
  height: 242px;
}
.issue-timeline-scroll {
  padding-top: 24px;
}
.issue-timeline {
  margin: 0;
  padding: 0;
  list-style: none;
}
.issue-event {
  display: grid;
  grid-template-columns: 12px 36px minmax(0, 1fr);
  gap: 8px;
}
.issue-event-rail {
  position: relative;
}
.issue-event:not(:last-child) .issue-event-rail::before {
  position: absolute;
  top: 15px;
  bottom: -15px;
  left: 5px;
  width: 1px;
  background: #f0f1f3;
  content: "";
}
.issue-event-halo {
  position: absolute;
  top: 11px;
  left: -3px;
}
.issue-event-dot {
  position: absolute;
  top: 14px;
  left: 0;
}
.issue-event-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  margin-top: 2px;
  border-radius: 18px;
  background: #f3f4f5;
}
.issue-event-content {
  min-width: 0;
  padding: 8px 0 16px;
  font-size: 14px;
  line-height: 26px;
  overflow-wrap: anywhere;
}
.issue-event-title {
  margin-bottom: 8px;
}
.issue-event-title,
.issue-event-operation {
  color: #262626;
}
.issue-event-details {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: #8c96a2;
}
.issue-event-note {
  display: flex;
  align-items: flex-start;
  > span:first-child {
    flex: none;
  }
  > .rich-text-editor {
    flex: 1;
    min-width: 0;
  }
}
.issue-event-status {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
}
.issue-event-status :deep(.status-pill) {
  height: 24px;
  padding: 0 10px;
  border-radius: 14px;
  font-size: 14px;
  line-height: 22px;
  background: color-mix(in srgb, currentColor 7%, white);
}
.horizontal {
  min-height: 238px;
  padding: 0 0 16px;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
  scrollbar-color: #b8c2d1 #f0f2f7;
}
.horizontal::-webkit-scrollbar {
  height: 0;
}
.horizontal::-webkit-scrollbar-track {
  background: #f0f2f7;
  border-radius: 2px;
}
.horizontal::-webkit-scrollbar-thumb {
  background: #b8c2d1;
  border-radius: 2px;
}
.horizontal .issue-timeline {
  display: flex;
  gap: 16px;
  width: max-content;
  padding-top: 16px;
}
.horizontal .issue-event {
  grid-template-columns: 36px minmax(0, 1fr);
  align-content: start;
  flex: 0 0 280px;
  gap: 8px;
  width: 280px;
}
.horizontal .issue-event-rail {
  grid-column: 1 / -1;
  width: 100%;
  height: 20px;
}
.horizontal .issue-event:not(:last-child) .issue-event-rail::before {
  top: 8px;
  bottom: auto;
  left: 9px;
  width: 296px;
  height: 2px;
}
.horizontal .issue-event-halo {
  top: 0;
  left: 0;
}
.horizontal .issue-event-dot {
  top: 3px;
  left: 3px;
}
.horizontal .issue-event-avatar {
  margin-top: 2px;
}
.horizontal .issue-event-content {
  display: contents;
}
.horizontal .issue-event-details {
  grid-column: 1 / -1;
}
.issue-scroll-track {
  height: 4px;
  overflow: hidden;
  border-radius: 2px;
  background: #f0f2f7;
}
.issue-scroll-track > div {
  height: 4px;
  border-radius: 2px;
  background: #b8c2d1;
}
.horizontal .issue-event-title {
  align-self: center;
  margin-bottom: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
