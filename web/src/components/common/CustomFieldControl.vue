<!--
 * CustomFieldControl.vue
 * Shared editor for one custom field, used by create / detail / list / planning.
 -->
<template>
  <div class="custom-field-control">
    <a-input
      v-if="type === CUSTOM_FIELD_TYPE.TEXT || !type"
      :value="modelValue"
      :size="size"
      :disabled="disabled"
      :placeholder="inputPlaceholder"
      :status="status"
      allow-clear
      @update:value="emitValue"
      @pressEnter="$emit('pressEnter')"
      @blur="$emit('blur')"
    />
    <a-textarea
      v-else-if="type === CUSTOM_FIELD_TYPE.TEXTAREA"
      :value="modelValue"
      :disabled="disabled"
      :placeholder="inputPlaceholder"
      :rows="2"
      @update:value="emitValue"
      @blur="$emit('blur')"
    />
    <a-input
      v-else-if="type === CUSTOM_FIELD_TYPE.PHONE"
      ref="phoneInputRef"
      class="custom-field-control__phone"
      :value="modelValue"
      :size="size"
      :disabled="disabled"
      :placeholder="inputPlaceholder"
      :status="status"
      allow-clear
      @update:value="emitValue"
      @focus="onPhoneFocus"
      @pressEnter="onPhonePressEnter"
      @blur="onPhoneBlur"
    />
    <a-input-number
      v-else-if="
        type === CUSTOM_FIELD_TYPE.NUMBER || type === CUSTOM_FIELD_TYPE.NUMERIC
      "
      :value="modelValue"
      :size="size"
      :disabled="disabled"
      :placeholder="inputPlaceholder"
      :precision="integerOnly ? 0 : undefined"
      :controls="false"
      style="width: 100%"
      @update:value="emitValue"
      @pressEnter="$emit('pressEnter')"
      @blur="$emit('blur')"
    />
    <a-date-picker
      v-else-if="type === CUSTOM_FIELD_TYPE.DATE"
      :value="modelValue"
      :size="size"
      :disabled="disabled"
      :placeholder="selectPlaceholder"
      :open="forceOpen || undefined"
      style="width: 100%"
      @update:value="emitValue"
      @change="$emit('change')"
      @openChange="onOpenChange"
    />
    <a-date-picker
      v-else-if="type === CUSTOM_FIELD_TYPE.DATETIME"
      :value="modelValue"
      :size="size"
      :disabled="disabled"
      :placeholder="selectPlaceholder"
      show-time
      :open="forceOpen || undefined"
      style="width: 100%"
      @update:value="emitValue"
      @change="$emit('change')"
      @openChange="onOpenChange"
    />
    <a-radio-group
      v-else-if="type === CUSTOM_FIELD_TYPE.RADIO"
      :value="optionControlValue"
      :disabled="disabled"
      :size="size"
      button-style="solid"
      class="custom-field-control__buttons"
      @update:value="onRadioChange"
    >
      <a-radio-button
        v-for="option in optionItems"
        :key="option.value"
        :value="option.value"
      >
        {{ option.label }}
      </a-radio-button>
    </a-radio-group>
    <a-select
      v-else-if="optionSelect"
      :value="optionControlValue"
      :size="size"
      :disabled="disabled"
      :placeholder="selectPlaceholder"
      :mode="multiValue ? 'multiple' : undefined"
      :options="optionItems"
      :default-open="forceOpen"
      allow-clear
      show-search
      option-filter-prop="label"
      style="width: 100%"
      @update:value="emitValue"
      @change="onSelectChange"
      @dropdownVisibleChange="onOpenChange"
    />
    <a-select
      v-else-if="
        type === CUSTOM_FIELD_TYPE.USER || type === CUSTOM_FIELD_TYPE.USER_MULTI
      "
      :value="memberControlValue"
      :size="size"
      :disabled="disabled"
      :placeholder="selectPlaceholder"
      :mode="multiValue ? 'multiple' : undefined"
      :options="memberItems"
      :default-open="forceOpen"
      allow-clear
      show-search
      option-filter-prop="label"
      option-label-prop="label"
      style="width: 100%"
      @update:value="emitValue"
      @change="onSelectChange"
      @dropdownVisibleChange="onOpenChange"
    />
    <CustomFieldCascader
      v-else-if="isCascaderFieldType(type)"
      :nodes="field.cascader || []"
      :model-value="modelValue"
      :multiple="type === CUSTOM_FIELD_TYPE.CASCADER_MULTI"
      :size="size"
      :disabled="disabled"
      :force-open="forceOpen"
      :placeholder="selectPlaceholder"
      @update:model-value="emitValue"
      @change="onSelectChange"
      @open-change="onOpenChange"
    />
    <a-input
      v-else
      :value="modelValue"
      :size="size"
      :disabled="disabled"
      :placeholder="inputPlaceholder"
      @update:value="emitValue"
      @pressEnter="$emit('pressEnter')"
      @blur="$emit('blur')"
    />
    <Teleport to="body">
      <div
        v-if="phoneInvalid"
        class="phone-field-tip"
        :class="{ 'phone-field-tip--below': phoneTip.below }"
        :style="{ left: `${phoneTip.left}px`, top: `${phoneTip.top}px` }"
        role="alert"
      >
        <ExclamationCircleFilled class="phone-field-tip__icon" />
        <span>{{ t("settings_fieldPhoneInvalid") }}</span>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ExclamationCircleFilled } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import CustomFieldCascader from "./CustomFieldCascader.vue";
import {
  CUSTOM_FIELD_TYPE,
  isCascaderFieldType,
  isMultiValueFieldType,
  isOptionFieldType,
} from "@/constants/customField";
import {
  filterMembersForField,
  isValidPhoneNumber,
  optionFieldControlValue,
} from "@/utils/customField";

const props = defineProps({
  field: { type: Object, required: true },
  modelValue: { default: undefined },
  members: { type: Array, default: () => [] },
  size: { type: String, default: "small" },
  disabled: { type: Boolean, default: false },
  forceOpen: { type: Boolean, default: false },
  status: { type: String, default: undefined },
});

const emit = defineEmits({
  "update:modelValue": () => true,
  change: () => true,
  blur: () => true,
  pressEnter: () => true,
  openChange: (open) => true,
});

const { t } = useI18n();
const type = computed(() => props.field?.field_type || CUSTOM_FIELD_TYPE.TEXT);
const phoneInputRef = ref(null);
const phoneBlurred = ref(false);
const phoneTip = ref({ left: 0, top: 0, below: false });
const phoneInvalid = computed(
  () =>
    phoneBlurred.value &&
    type.value === CUSTOM_FIELD_TYPE.PHONE &&
    !isValidPhoneNumber(props.modelValue),
);
const integerOnly = computed(() => Number(props.field?.integer_only) === 1);
const inputPlaceholder = computed(() =>
  t("settings_fieldInputPlaceholder", { name: props.field?.name || "" }),
);
const selectPlaceholder = computed(() =>
  t("settings_fieldSelectPlaceholder", { name: props.field?.name || "" }),
);
const optionSelect = computed(() => isOptionFieldType(type.value));
const multiValue = computed(() => isMultiValueFieldType(type.value));
const optionItems = computed(() =>
  (props.field?.options || []).map((option) => ({
    value: option.key,
    label: option.name,
  })),
);
const optionControlValue = computed(() =>
  optionFieldControlValue(props.field, props.modelValue),
);
const selectedMemberIds = computed(
  () =>
    new Set(
      (multiValue.value
        ? Array.isArray(props.modelValue)
          ? props.modelValue
          : []
        : [props.modelValue]
      )
        .map(String)
        .filter(Boolean),
    ),
);
const memberItems = computed(() => {
  const candidates = new Map(
    filterMembersForField(props.field, props.members).map((member) => [
      String(member.user_id),
      member,
    ]),
  );
  props.members.forEach((member) => {
    const id = String(member.user_id);
    if (selectedMemberIds.value.has(id) && !candidates.has(id)) {
      candidates.set(id, member);
    }
  });
  return [...candidates].map(([value, member]) => ({
    value,
    label: member.user_name || member.user_username,
  }));
});
const memberControlValue = computed(() => {
  const allowed = new Set(memberItems.value.map((item) => item.value));
  if (multiValue.value) {
    return (Array.isArray(props.modelValue) ? props.modelValue : [])
      .map(String)
      .filter((value) => allowed.has(value));
  }
  const value = String(props.modelValue ?? "");
  return allowed.has(value) ? value : undefined;
});
function emitValue(value) {
  emit("update:modelValue", value);
}

function measurePhoneTip() {
  const el = phoneInputRef.value?.$el;
  if (!(el instanceof HTMLElement)) return;
  const rect = el.getBoundingClientRect();
  const below = rect.top < 72;
  phoneTip.value = {
    left: rect.left + rect.width / 2,
    top: below ? rect.bottom : rect.top,
    below,
  };
}

function onPhoneTipScroll() {
  if (phoneInvalid.value) measurePhoneTip();
}

function onPhoneFocus() {
  phoneBlurred.value = false;
}

function onPhoneBlur() {
  measurePhoneTip();
  phoneBlurred.value = true;
  emit("blur");
}

function onPhonePressEnter() {
  measurePhoneTip();
  phoneBlurred.value = true;
  emit("pressEnter");
}

watch(phoneInvalid, (open) => {
  if (open) {
    measurePhoneTip();
    window.addEventListener("scroll", onPhoneTipScroll, true);
    window.addEventListener("resize", onPhoneTipScroll);
    return;
  }
  window.removeEventListener("scroll", onPhoneTipScroll, true);
  window.removeEventListener("resize", onPhoneTipScroll);
});

onBeforeUnmount(() => {
  window.removeEventListener("scroll", onPhoneTipScroll, true);
  window.removeEventListener("resize", onPhoneTipScroll);
});

function onRadioChange(value) {
  emitValue(value);
  emit("change");
}

function onSelectChange() {
  if (!multiValue.value) emit("change");
}

function onOpenChange(open) {
  emit("openChange", open);
}
</script>

<style scoped lang="less">
.custom-field-control {
  width: 100%;
  min-width: 0;

  :deep(.ant-select-selector),
  :deep(.ant-select-selection-overflow) {
    min-width: 0;
    max-width: 100%;
  }

  :deep(.ant-select-selection-overflow-item) {
    min-width: 0;
    max-width: calc(100% - 16px);
  }

  :deep(.ant-select-selection-placeholder) {
    width: calc(100% - 16px);
  }

  :deep(.ant-select-selection-placeholder),
  :deep(.ant-select-selection-item),
  :deep(.ant-select-selection-item-content) {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.custom-field-control__phone {
  width: 100%;
}

// Same card as message.warning. Field tables clip overflow, so this is portaled.
.phone-field-tip {
  position: fixed;
  z-index: 2000;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  color: rgba(0, 0, 0, 0.88);
  font-size: 13px;
  line-height: 20px;
  white-space: nowrap;
  pointer-events: none;
  background: #fff;
  border-radius: 8px;
  box-shadow:
    0 6px 16px 0 rgba(0, 0, 0, 0.08),
    0 3px 6px -4px rgba(0, 0, 0, 0.12),
    0 9px 28px 8px rgba(0, 0, 0, 0.05);
  transform: translate(-50%, calc(-100% - 8px));

  &::after {
    content: "";
    position: absolute;
    bottom: -4px;
    left: 50%;
    width: 8px;
    height: 8px;
    margin-left: -4px;
    background: #fff;
    border-radius: 1px;
    transform: rotate(45deg);
  }
}

.phone-field-tip__icon {
  color: #faad14;
  font-size: 16px;
}

.phone-field-tip--below {
  transform: translate(-50%, 8px);

  &::after {
    top: -4px;
    bottom: auto;
  }
}

.custom-field-control__buttons {
  display: flex;
  max-width: 100%;
  overflow-x: auto;
  white-space: nowrap;

  :deep(.ant-radio-button-wrapper) {
    flex: 0 0 auto;
  }
}
</style>
