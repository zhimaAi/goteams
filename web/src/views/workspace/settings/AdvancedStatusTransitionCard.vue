<!-- AdvancedStatusTransitionCard configures fields shown during a status transition. -->
<template>
  <a-card size="small" class="settings-card" :loading="loading">
    <template #title>{{ $t("settings_advanced_transition") }}</template>
    <template #extra>
      <a-switch
        :checked="transitionForm.enabled === 1"
        :disabled="!canManage || saving || !ready"
        @change="toggleTransition"
      />
    </template>
    <template v-if="ready && transitionForm.enabled === 1">
      <div class="transition-head">
        <span>{{ $t("settings_transition_field_name") }}</span>
        <span class="transition-required-col">{{
          $t("settings_transition_required")
        }}</span>
        <span class="transition-action-col">{{ $t("settings_action") }}</span>
      </div>
      <div ref="fieldListRef">
        <div
          v-for="field in transitionForm.fields"
          :key="field.key"
          class="transition-row transition-sortable-field"
        >
          <span class="transition-field-name">
            <WorkflowOptionHandle
              v-if="canManage"
              :label="fieldLabel(field.key)"
              :disabled="orderDisabled || transitionForm.fields.length < 2"
              @move="moveField(field.key, $event)"
            />
            {{ fieldLabel(field.key) }}
          </span>
          <span class="transition-required-col">
            <a-checkbox
              :checked="field.required === 1"
              :disabled="!canManage || saving || !ready"
              @change="
                (event) => toggleFieldRequired(field.key, event.target.checked)
              "
            />
          </span>
          <span class="transition-action-col">
            <a-button
              v-if="canManage"
              :disabled="saving"
              type="text"
              size="small"
              danger
              @click="removeTransitionField(field.key)"
            >
              <DeleteOutlined />
            </a-button>
          </span>
        </div>
      </div>
      <a-button
        v-if="canManage"
        type="link"
        size="small"
        class="add-field-btn"
        :disabled="saving"
        @click="pickerOpen = true"
      >
        <PlusOutlined /> {{ $t("settings_add_transition_field") }}
      </a-button>
    </template>
  </a-card>

  <CustomStatusTransitionCard
    v-if="ready && !loading"
    :form="transitionForm"
    :statuses="statuses"
    :options="transitionFieldOptions"
    :can-manage="canManage"
    :saving="saving"
    @save="save"
  />

  <a-modal
    v-model:open="pickerOpen"
    :title="$t('settings_select_transition_field')"
    :footer="null"
    :destroy-on-close="true"
    width="360px"
  >
    <div v-if="!availableTransitionFields.length" class="picker-empty">
      {{ $t("common_noData") }}
    </div>
    <button
      v-for="item in availableTransitionFields"
      :key="item.key"
      type="button"
      class="picker-item"
      :disabled="saving"
      @click="addTransitionField(item.key)"
    >
      <span>{{ item.label }}</span>
      <a-tag v-if="item.builtin" color="blue">{{
        $t("settings_builtin")
      }}</a-tag>
    </button>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch, onBeforeUnmount } from "vue";
import { message } from "ant-design-vue";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  getWorkspaceSettings,
  listCustomFields,
  listFieldConfigs,
  updateWorkspaceSettings,
} from "@/api/workspace";
import { DEFAULT_STATUS_TRANSITION_FORM } from "@/constants/statusTransition";
import {
  statusTransitionFieldLabel,
  statusTransitionForm,
  statusTransitionFieldOptions,
} from "@/utils/statusTransition";
import CustomStatusTransitionCard from "./CustomStatusTransitionCard.vue";
import WorkflowOptionHandle from "./WorkflowOptionHandle.vue";
import { useStatusTransitionFieldOrder } from "@/composables/useStatusTransitionFieldOrder";

const props = defineProps({
  statuses: { type: Array, default: () => [] },
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
  canManage: { type: Boolean, default: false },
  reloadKey: { type: Number, default: 0 },
});
const { t: $t } = useI18n();
const pickerOpen = ref(false);
const loading = ref(true);
const ready = ref(false);
const saving = ref(false);
let loadIndex = 0;
let savedForm = null;
const fieldConfigs = ref([]);
const fieldListRef = ref(null);
const customFields = ref([]);
const transitionForm = ref({
  enabled: DEFAULT_STATUS_TRANSITION_FORM.enabled,
  fields: DEFAULT_STATUS_TRANSITION_FORM.fields.map((item) => ({ ...item })),
});

const orderedFields = computed({
  get: () => transitionForm.value.fields,
  set: (fields) => {
    transitionForm.value = { ...transitionForm.value, fields };
  },
});
const orderDisabled = computed(
  () =>
    !props.canManage ||
    saving.value ||
    loading.value ||
    !ready.value ||
    transitionForm.value.enabled !== 1,
);
const { moveField } = useStatusTransitionFieldOrder(
  orderedFields,
  () => fieldListRef.value,
  orderDisabled,
  () => save(),
);

const transitionFieldOptions = computed(() =>
  statusTransitionFieldOptions(
    props.typeKey,
    fieldConfigs.value,
    customFields.value,
    $t,
  ),
);
const availableTransitionFields = computed(() => {
  const used = new Set(transitionForm.value.fields.map((item) => item.key));
  return transitionFieldOptions.value.filter((item) => !used.has(item.key));
});

function fieldLabel(key) {
  return statusTransitionFieldLabel(key, $t, customFields.value);
}

async function load() {
  const index = ++loadIndex;
  loading.value = true;
  ready.value = false;
  savedForm = null;
  pickerOpen.value = false;
  try {
    const [settings, configs, fields] = await Promise.all([
      getWorkspaceSettings(props.wsId),
      listFieldConfigs(props.wsId, props.typeKey),
      listCustomFields(props.wsId, props.typeKey),
    ]);
    if (index !== loadIndex) return;
    fieldConfigs.value = configs || [];
    customFields.value = fields || [];
    transitionForm.value = statusTransitionForm(
      settings || {},
      props.typeKey,
      customFields.value,
      fieldConfigs.value,
    );
    savedForm = JSON.parse(JSON.stringify(transitionForm.value));
    ready.value = true;
  } catch {
    // Network errors are handled centrally by the interceptor.
  } finally {
    if (index === loadIndex) loading.value = false;
  }
}

async function save(next = transitionForm.value) {
  if (!props.canManage || saving.value || !ready.value) return false;
  const restoreOnFailure = next === transitionForm.value;
  const index = loadIndex;
  saving.value = true;
  try {
    await updateWorkspaceSettings(props.wsId, {
      [`${props.typeKey}_status_transition`]: next,
    });
    if (index !== loadIndex) return false;
    transitionForm.value = next;
    savedForm = JSON.parse(JSON.stringify(next));
    message.success($t("common_saved"));
    return true;
  } catch {
    if (index === loadIndex && savedForm && restoreOnFailure)
      transitionForm.value = JSON.parse(JSON.stringify(savedForm));
    // Network errors are handled centrally by the interceptor.
    return false;
  } finally {
    saving.value = false;
  }
}

async function toggleTransition(checked) {
  transitionForm.value = {
    ...transitionForm.value,
    enabled: checked ? 1 : 0,
  };
  await save();
}

async function toggleFieldRequired(key, checked) {
  transitionForm.value = {
    ...transitionForm.value,
    fields: transitionForm.value.fields.map((item) =>
      item.key === key ? { ...item, required: checked ? 1 : 0 } : item,
    ),
  };
  await save();
}

async function removeTransitionField(key) {
  transitionForm.value = {
    ...transitionForm.value,
    fields: transitionForm.value.fields.filter((item) => item.key !== key),
  };
  await save();
}

async function addTransitionField(key) {
  if (!transitionForm.value.fields.some((item) => item.key === key)) {
    transitionForm.value = {
      ...transitionForm.value,
      fields: [...transitionForm.value.fields, { key, required: 0 }],
    };
    await save();
  }
  pickerOpen.value = false;
}

onBeforeUnmount(() => {
  loadIndex += 1;
});

watch(() => [props.wsId, props.typeKey, props.reloadKey], load, {
  immediate: true,
});
</script>

<style scoped>
.transition-head,
.transition-row {
  display: grid;
  grid-template-columns: 1fr 88px 64px;
  align-items: center;
  min-height: 44px;
  padding: 0 4px;
  border-bottom: 1px solid #eef1f6;
  color: rgba(0, 0, 0, 0.65);
}
.transition-head {
  color: rgba(0, 0, 0, 0.45);
  font-size: 13px;
}
.transition-field-name {
  display: flex;
  align-items: center;
  min-width: 0;
}
.transition-required-col {
  text-align: center;
}
.transition-action-col {
  text-align: right;
}
.add-field-btn {
  padding-left: 4px;
  margin-top: 8px;
}
.picker-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 10px 4px;
  border: 0;
  border-bottom: 1px solid #f0f0f0;
  background: transparent;
  cursor: pointer;
}
.picker-item:hover {
  background: #f5f7fa;
}
.picker-empty {
  padding: 24px 0;
  text-align: center;
  color: #999;
}

.transition-field-placeholder {
  background: var(--ant-color-primary-bg, #f0f7ff);
}
.transition-field-placeholder > * {
  visibility: hidden;
}
.transition-field-drag-preview {
  background: var(--ant-color-bg-container, #fff);
  border-radius: 6px;
  box-shadow: 0 3px 10px rgb(16 24 40 / 12%);
  /* Sortable sets the floating preview opacity inline. */
  opacity: 1 !important;
}
</style>
