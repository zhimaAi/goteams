<template>
  <a-tag
    :color="displayColor"
    :class="{ 'sla-stage-pill': compact }"
    :title="fullText"
    :aria-label="fullText"
  >
    {{ stage.stage_name }}{{ compact ? " " : " · " }}
    <strong v-if="compact">{{ slaStageText(stage, t) }}</strong>
    <template v-else>{{ slaStageText(stage, t) }}</template>
  </a-tag>
</template>
<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { slaStageText } from "@/utils/sla";
const props = defineProps({
  stage: { type: Object, required: true },
  compact: { type: Boolean, default: false },
});
const { t } = useI18n();
const color = computed(() => {
  if (props.stage.calculation_state === "calendar_unavailable") return "orange";
  if (
    props.stage.state === "interrupted" ||
    props.stage.state === "not_started"
  )
    return undefined;
  return props.stage.overdue_seconds > 0
    ? "red"
    : props.stage.state === "completed"
      ? "green"
      : "blue";
});
const displayColor = computed(() =>
  props.compact
    ? { blue: "#3370ff", red: "#ff3b3f", green: "#2aa34a", orange: "#d48806" }[
        color.value
      ] || "#8590a0"
    : color.value,
);
const fullText = computed(
  () => `${props.stage.stage_name} · ${slaStageText(props.stage, t)}`,
);
</script>
<style scoped lang="less">
.sla-stage-pill {
  max-width: 100%;
  margin: 0;
  padding: 1px 10px;
  overflow: hidden;
  border: 0;
  border-radius: 999px;
  color: #fff;
  font-size: 12px;
  font-weight: 500;
  line-height: 22px;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}
</style>
