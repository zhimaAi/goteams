<template>
  <a-modal
    :open="open"
    :title="title"
    :confirm-loading="submitting"
    :ok-button-props="{ disabled: !selectedValue }"
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    width="520px"
    @ok="submit"
    @cancel="$emit('close')"
  >
    <a-spin :spinning="loading">
      <p class="action-notice">{{ notice }}</p>
      <div class="action-field">
        <label>{{ fieldLabel }}</label>
        <a-select
          v-model:value="selectedValue"
          show-search
          option-filter-prop="label"
          :placeholder="placeholder"
          style="width: 100%"
        >
          <a-select-option
            v-for="item in options"
            :key="optionKey(item)"
            :value="optionKey(item)"
            :label="optionLabel(item)"
          >
            {{ optionLabel(item) }}
          </a-select-option>
        </a-select>
      </div>
    </a-spin>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  convertWorkItem,
  listWorkItemActionTargets,
  listWorkItemMergeTargets,
  mergeWorkItem,
  moveWorkItem,
} from "@/api/workspace";

const props = defineProps({
  open: { type: Boolean, default: false },
  action: { type: String, default: "" },
  wsId: { type: [String, Number], default: "" },
  workItemId: { type: [String, Number], default: "" },
  sourceType: { type: String, default: "requirement" },
  targetType: { type: String, default: "requirement" },
});
const emit = defineEmits(["close", "success"]);
const { t } = useI18n();
const loading = ref(false);
const submitting = ref(false);
const options = ref([]);
const selectedValue = ref(undefined);

const isMerge = computed(() => props.action === "merge");
const title = computed(() =>
  t(`workItemAction_${props.action}Title`, { type: typeText.value }),
);
const typeText = computed(() =>
  t(
    props.sourceType === "defect"
      ? "workItemAction_defect"
      : "workItemAction_requirement",
  ),
);
const notice = computed(() => {
  if (props.action === "merge") return t("workItemAction_mergeNotice");
  if (props.action === "convert") {
    return props.sourceType === "requirement"
      ? t("workItemAction_convertRequirementNotice")
      : t("workItemAction_transferNotice");
  }
  return props.sourceType === "requirement"
    ? t("workItemAction_moveRequirementNotice")
    : t("workItemAction_transferNotice");
});
const fieldLabel = computed(() =>
  t(
    isMerge.value
      ? "workItemAction_targetDefect"
      : "workItemAction_targetProject",
  ),
);
const placeholder = computed(() =>
  t(
    isMerge.value
      ? "workItemAction_selectDefect"
      : "workItemAction_selectProject",
  ),
);

watch(
  () => props.open,
  (value) => {
    if (value) loadOptions();
    else {
      options.value = [];
      selectedValue.value = undefined;
    }
  },
);

async function loadOptions() {
  loading.value = true;
  try {
    options.value = isMerge.value
      ? await listWorkItemMergeTargets(props.wsId, props.workItemId)
      : await listWorkItemActionTargets({
          sourceWsId: props.wsId,
          type: props.targetType,
          action: props.action,
        });
  } catch {
    options.value = [];
  } finally {
    loading.value = false;
  }
}

function optionKey(item) {
  return isMerge.value ? `${item.workspace_id}:${item.id}` : Number(item.id);
}

function optionLabel(item) {
  return isMerge.value ? `[${item.workspace_name}] ${item.title}` : item.name;
}

async function submit() {
  if (!selectedValue.value) return;
  submitting.value = true;
  try {
    let result;
    if (props.action === "move") {
      result = await moveWorkItem(props.wsId, props.workItemId, {
        target_workspace_id: Number(selectedValue.value),
      });
    } else if (props.action === "convert") {
      result = await convertWorkItem(props.wsId, props.workItemId, {
        target_workspace_id: Number(selectedValue.value),
        target_type: props.targetType,
      });
    } else {
      const [workspaceId, targetId] = String(selectedValue.value).split(":");
      result = await mergeWorkItem(props.wsId, props.workItemId, {
        target_workspace_id: Number(workspaceId),
        target_id: Number(targetId),
      });
    }
    emit("success", result);
  } catch {
    /* handled by the request interceptor */
  } finally {
    submitting.value = false;
  }
}
</script>

<style scoped lang="less">
.action-notice {
  margin: 0 0 24px;
  color: var(--app-text-secondary);
  line-height: 1.6;
  white-space: pre-line;
}

.action-field {
  display: grid;
  gap: 8px;
  min-height: 96px;
}

.action-field label {
  color: var(--app-text);
  font-weight: 500;
}
</style>
