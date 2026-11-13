<!-- AdvancedStatusTransitionCard configures fields shown during a status transition. -->
<template>
  <a-card size="small" class="settings-card">
    <template #title>{{ $t("settings_advanced_transition") }}</template>
    <template #extra>
      <a-switch
        :checked="transitionForm.enabled === 1"
        :disabled="!canManage"
        @change="toggleTransition"
      />
    </template>
    <template v-if="transitionForm.enabled === 1">
      <div class="transition-head">
        <span>{{ $t("settings_transition_field_name") }}</span>
        <span class="transition-required-col">{{
          $t("settings_transition_required")
        }}</span>
        <span class="transition-action-col">{{ $t("settings_action") }}</span>
      </div>
      <div
        v-for="field in transitionForm.fields"
        :key="field.key"
        class="transition-row"
      >
        <span>{{ fieldLabel(field.key) }}</span>
        <span class="transition-required-col">
          <a-checkbox
            :checked="field.required === 1"
            :disabled="!canManage"
            @change="
              (event) => toggleFieldRequired(field.key, event.target.checked)
            "
          />
        </span>
        <span class="transition-action-col">
          <a-button
            v-if="canManage"
            type="text"
            size="small"
            danger
            @click="removeTransitionField(field.key)"
          >
            <DeleteOutlined />
          </a-button>
        </span>
      </div>
      <a-button
        v-if="canManage"
        type="link"
        size="small"
        class="add-field-btn"
        @click="pickerOpen = true"
      >
        <PlusOutlined /> {{ $t("settings_add_transition_field") }}
      </a-button>
    </template>
  </a-card>

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
import { computed, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  getWorkspaceSettings,
  listCustomFields,
  listFieldConfigs,
  updateWorkspaceSettings,
} from "@/api/workspace";
import {
  DEFAULT_STATUS_TRANSITION_FORM,
  STATUS_TRANSITION_BUILTIN_KEYS,
} from "@/constants/statusTransition";
import {
  statusTransitionFieldLabel,
  statusTransitionForm,
} from "@/utils/statusTransition";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
  canManage: { type: Boolean, default: false },
  reloadKey: { type: Number, default: 0 },
});
const { t: $t } = useI18n();
const pickerOpen = ref(false);
const fieldConfigs = ref([]);
const customFields = ref([]);
const transitionForm = ref({
  enabled: DEFAULT_STATUS_TRANSITION_FORM.enabled,
  fields: DEFAULT_STATUS_TRANSITION_FORM.fields.map((item) => ({ ...item })),
});

const availableTransitionFields = computed(() => {
  const used = new Set(transitionForm.value.fields.map((item) => item.key));
  const enabledKeys = new Set(
    fieldConfigs.value
      .filter((item) => Number(item.enabled) === 1)
      .map((item) => item.key),
  );
  const options = STATUS_TRANSITION_BUILTIN_KEYS.filter(
    (key) => !used.has(key) && enabledKeys.has(key),
  ).map((key) => ({
    key,
    label: statusTransitionFieldLabel(key, $t, customFields.value),
    builtin: true,
  }));
  if (!used.has("comment")) {
    options.push({
      key: "comment",
      label: statusTransitionFieldLabel("comment", $t),
      builtin: false,
    });
  }
  for (const field of customFields.value) {
    const key = `custom_${field.id}`;
    if (Number(field.is_enabled) === 1 && !used.has(key)) {
      options.push({ key, label: field.name, builtin: false });
    }
  }
  return options;
});

function fieldLabel(key) {
  return statusTransitionFieldLabel(key, $t, customFields.value);
}

async function load() {
  try {
    const [settings, configs, fields] = await Promise.all([
      getWorkspaceSettings(props.wsId),
      listFieldConfigs(props.wsId, props.typeKey),
      listCustomFields(props.wsId, props.typeKey),
    ]);
    fieldConfigs.value = configs || [];
    customFields.value = fields || [];
    transitionForm.value = statusTransitionForm(
      settings || {},
      props.typeKey,
      customFields.value,
      fieldConfigs.value,
    );
  } catch {
    // Network errors are handled centrally by the interceptor.
  }
}

async function save() {
  if (!props.canManage) return;
  try {
    await updateWorkspaceSettings(props.wsId, {
      [`${props.typeKey}_status_transition`]: transitionForm.value,
    });
    message.success($t("common_saved"));
  } catch {
    // Network errors are handled centrally by the interceptor.
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
.transition-required-col,
.transition-action-col {
  text-align: center;
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
</style>
