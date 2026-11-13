<template>
  <span
    class="work-item-type-pill"
    :class="pillClass"
    :style="pillStyle"
  >
    {{ text }}
  </span>
</template>

<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useWorkItemTypeAttributes } from "@/composables/useWorkItemTypeAttributes";

const props = defineProps({
  type: { type: String, default: "requirement" },
  // Without wsId (or before the config loads) the pill keeps the legacy label/color.
  wsId: { type: [String, Number], default: "" },
  // Static badge overrides for surfaces without a workspace session (public pages);
  // they take precedence over the per-workspace attributes fetched via wsId.
  iconText: { type: String, default: "" },
  iconColor: { type: String, default: "" },
  // Size is "small" (compact list badge, the default) or "large" (roomy badge used by
  // the public issue-invite catalog rows).
  size: { type: String, default: "small" },
});

const { t } = useI18n();
const wsIdRef = computed(() => props.wsId);
const { attrOf } = useWorkItemTypeAttributes(wsIdRef);
const attr = computed(() => attrOf(props.type));
const defaultText = computed(() =>
  t(props.type === "defect" ? "requirement_bug" : "requirement_feature"),
);
const text = computed(
  () => props.iconText || attr.value?.icon_text || defaultText.value,
);
const pillClass = computed(() => {
  const classes = [];
  if (props.size === "large") classes.push("is-large");
  if (!props.iconColor && !attr.value?.icon_color && props.type === "defect")
    classes.push("defect");
  return classes;
});
const pillStyle = computed(() => {
  const color = props.iconColor || attr.value?.icon_color || "";
  return color ? { backgroundColor: color } : null;
});
</script>

<style scoped lang="less">
.work-item-type-pill {
  display: inline-block;
  flex: 0 0 auto;
  min-width: 24px;
  height: 16px;
  padding: 0 2px;
  box-sizing: border-box;
  border-radius: 2px;
  background: #3582fb;
  color: #fff;
  font-size: 10px;
  line-height: 16px;
  text-align: center;
  white-space: nowrap;
}
.work-item-type-pill.defect {
  background: #e45454;
}
// The large variant is the roomy badge used by the public issue-invite catalog rows.
.work-item-type-pill.is-large {
  height: auto;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
}
</style>
