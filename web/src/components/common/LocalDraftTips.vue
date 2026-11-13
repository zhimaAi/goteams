<template>
  <span v-if="visible" class="local-draft-tips">
    <template v-if="timeText">
      <span class="tips-time">{{ timeText }}</span>
      {{ $t("draft_saved_to_local") }}
    </template>
    <template v-else>
      {{ $t("draft_has_local_data") }}
    </template>
    <a class="tips-link" @click="$emit('open')">{{
      $t("draft_click_to_view")
    }}</a>
  </span>
</template>

<script setup>
import { computed } from "vue";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  draft: { type: Object, required: true },
});
defineEmits(["open"]);

// 本次生命周期成功保存过 → 显示保存时间；否则可见范围内还有本地缓存 → 显示离线数据提示
const visible = computed(() =>
  Boolean(
    props.draft.lastSavedAt.value ||
    props.draft.hasUnposted.value ||
    props.draft.hasRecords.value,
  ),
);
const timeText = computed(() => {
  const timestamp = props.draft.lastSavedAt.value;
  if (!timestamp) return "";
  const text = formatLocalDate(Math.floor(Number(timestamp) / 1000), true);
  return text === "-" ? "" : text.split(" ")[1] || "";
});
</script>

<style scoped lang="less">
.local-draft-tips {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #8c95a8;
  font-size: 12px;
  line-height: 32px;
}
.tips-time {
  font-variant-numeric: tabular-nums;
}
.tips-link {
  color: #1677ff;
}
</style>
