<template>
  <CustomFieldControl
    v-if="editing"
    v-model="draft.value"
    :field="field"
    :members="members"
    force-open
    @change="$emit('commit')"
    @blur="$emit('commit')"
    @pressEnter="$emit('commit')"
    @openChange="(open) => !open && $emit('commit')"
    @keydown.esc.stop.prevent="$emit('cancel')"
    @click.stop
  />
  <span v-else class="cf-text">
    {{ displayText }}
  </span>
</template>

<script setup>
import { computed } from "vue";
import CustomFieldControl from "@/components/common/CustomFieldControl.vue";
import { formatCustomFieldValue } from "@/utils/customField";

const props = defineProps({
  record: { type: Object, required: true },
  field: { type: Object, required: true },
  editing: { type: Boolean, default: false },
  draft: { type: Object, default: () => ({}) },
  members: { type: Array, default: () => [] },
});
defineEmits(["commit", "cancel"]);

const displayText = computed(() =>
  formatCustomFieldValue(
    props.field,
    props.record.custom_fields || {},
    props.members,
  ),
);
</script>

<style scoped lang="less">
.cf-text {
  display: inline-block;
  width: 100%;
  min-height: 22px;
}
</style>
