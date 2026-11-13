<template>
  <a-modal
    :open="open"
    :title="$t('planning_batchAddTitle')"
    :width="640"
    :ok-text="$t('planning_batchAdd')"
    :cancel-text="$t('common_cancel')"
    :ok-button-props="{ disabled: !titles.length, loading: submitting }"
    :mask-closable="!submitting"
    centered
    destroy-on-close
    @ok="submit"
    @cancel="close"
  >
    <p class="batch-tip">{{ $t("planning_batchAddTip") }}</p>
    <a-textarea
      v-model:value="text"
      :rows="10"
      :placeholder="$t('planning_batchAddPlaceholder')"
      :disabled="submitting"
    />
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { batchCreatePlannings } from "@/api/planning";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], required: true },
  groupId: { type: [String, Number], default: null },
});
const emit = defineEmits(["update:open", "created"]);
const { t: $t } = useI18n();

const text = ref("");
const submitting = ref(false);

// One planning name per line; blank lines are ignored.
const titles = computed(() =>
  text.value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean),
);

async function submit() {
  if (!titles.value.length || !props.groupId || submitting.value) return;
  submitting.value = true;
  try {
    const result = await batchCreatePlannings(
      props.wsId,
      props.groupId,
      titles.value,
    );
    message.success(
      $t("planning_batchAddSuccess", {
        n: result?.created ?? titles.value.length,
      }),
    );
    emit("created");
    // Not close(): submitting is still true here, so its guard would block the close and leave
    // the modal open. That guard only exists to stop the user closing it mid-submit.
    emit("update:open", false);
  } catch {
    /* handled by interceptor */
  } finally {
    submitting.value = false;
  }
}

function close() {
  if (!submitting.value) emit("update:open", false);
}

watch(
  () => props.open,
  (value) => {
    if (value) text.value = "";
  },
);
</script>

<style scoped lang="less">
.batch-tip {
  margin-bottom: 12px;
  color: var(--app-text-secondary);
}
</style>
