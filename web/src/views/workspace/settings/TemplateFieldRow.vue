<template>
  <div
    class="field-row-wrap"
    :class="{ dragging }"
    @dragover.prevent
    @drop="$emit('drop')"
  >
    <div class="field-row">
      <span
        class="drag-handle"
        draggable="true"
        @dragstart="$emit('dragstart', $event)"
        @dragend="$emit('dragend')"
      >
        <HolderOutlined />
      </span>
      <span class="field-name">{{ meta.label }}</span>
      <a-tag class="type-tag" :title="$t(typeLabelKey)">{{
        $t(typeLabelKey)
      }}</a-tag>
      <div class="default-value">
        <div v-if="isDate" class="date-default">
          <a-select
            :value="dateMode"
            size="small"
            :options="dateModeOptions"
            @update:value="onDateMode"
          />
          <PlannedDatePicker
            v-if="dateMode === 'fixed' && meta.widget === 'unix-date'"
            :value="unixDate"
            size="small"
            :placeholder="$t('requirement_selectDate')"
            @update:value="onUnixDate"
          />
          <a-date-picker
            v-else-if="dateMode === 'fixed'"
            :value="plainDate"
            size="small"
            @update:value="onPlainDate"
          />
          <template v-else-if="dateMode === 'after' || dateMode === 'before'">
            <a-input-number
              :value="dateOffset"
              :min="1"
              :max="36500"
              :precision="0"
              size="small"
              @update:value="onDateOffset"
            />
            <span>{{ $t("settings_template_date_days") }}</span>
          </template>
        </div>
        <a-select
          v-else-if="meta.widget === 'select'"
          :value="field.default_value ?? undefined"
          allow-clear
          size="small"
          :placeholder="$t('view_value_placeholder')"
          :options="meta.options"
          @update:value="patch({ default_value: $event ?? null })"
        />
        <a-select
          v-else-if="meta.widget === 'multi'"
          :value="arrayValue"
          mode="multiple"
          size="small"
          :placeholder="$t('view_value_placeholder')"
          :options="meta.options"
          @update:value="patch({ default_value: $event })"
        />
        <CustomFieldCascader
          v-else-if="
            meta.widget === 'cascader' || meta.widget === 'cascader-multi'
          "
          :nodes="meta.cascader || []"
          :model-value="arrayValue"
          :multiple="meta.widget === 'cascader-multi'"
          size="small"
          :placeholder="$t('view_value_placeholder')"
          @update:model-value="
            patch({ default_value: $event?.length ? $event : null })
          "
        />
        <a-date-picker
          v-else-if="meta.widget === 'datetime'"
          :value="datetimeValue"
          show-time
          size="small"
          :placeholder="$t('requirement_selectDate')"
          @update:value="onDatetime"
        />
        <a-input-number
          v-else-if="meta.widget === 'number'"
          :value="field.default_value"
          size="small"
          :min="meta.fieldType === 'estimated_work_hours' ? 0 : undefined"
          :max="
            meta.fieldType === 'estimated_work_hours' ? 99999999.99 : undefined
          "
          :precision="meta.fieldType === 'estimated_work_hours' ? 2 : undefined"
          @update:value="patch({ default_value: $event ?? null })"
        >
          <template
            v-if="meta.fieldType === 'estimated_work_hours'"
            #addonAfter
          >
            {{ $t("requirement_workHoursSuffix") }}
          </template>
        </a-input-number>
        <a-textarea
          v-else-if="meta.widget === 'textarea'"
          :value="field.default_value || ''"
          :rows="2"
          @update:value="patch({ default_value: $event })"
        />
        <CustomFieldControl
          v-else-if="meta.widget === 'phone'"
          :field="{ field_type: 'phone', name: meta.label }"
          :model-value="field.default_value || ''"
          size="small"
          @update:model-value="onPhone"
        />
        <a-input
          v-else
          :value="field.default_value || ''"
          size="small"
          @update:value="patch({ default_value: $event })"
        />
      </div>
      <a-button
        v-if="linkageEnabled"
        type="link"
        size="small"
        @click="addLinkage"
      >
        <ApartmentOutlined />
        {{ $t("settings_template_add_linkage") }}
      </a-button>
      <span v-else></span>
      <a-checkbox
        :checked="!!(field.required || meta.workspaceRequired)"
        :disabled="!!meta.workspaceRequired"
        @change="patch({ required: $event.target.checked })"
      >
        {{ $t("settings_template_required") }}
      </a-checkbox>
      <a-button type="text" danger size="small" @click="$emit('remove')">
        <DeleteOutlined />
      </a-button>
    </div>

    <div v-if="field.linkages?.length" class="linkage-list">
      <div
        v-for="(rule, index) in field.linkages"
        :key="index"
        class="linkage-rule"
      >
        <ApartmentOutlined />
        <span>{{
          $t("settings_template_linkage_when", { name: meta.label })
        }}</span>
        <a-select
          :value="rule.value ?? undefined"
          size="small"
          :options="linkageValueOptions(index)"
          :placeholder="$t('settings_template_linkage_value')"
          @update:value="updateLinkage(index, { value: $event })"
        />
        <span>{{ $t("settings_template_linkage_show") }}</span>
        <a-select
          :value="rule.field_keys || []"
          mode="multiple"
          size="small"
          :options="linkageTargets"
          :placeholder="$t('settings_template_linkage_fields')"
          @update:value="updateLinkage(index, { field_keys: $event })"
        />
        <a-button type="text" danger size="small" @click="removeLinkage(index)">
          <DeleteOutlined />
        </a-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue";
import dayjs from "dayjs";
import {
  ApartmentOutlined,
  DeleteOutlined,
  HolderOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import CustomFieldCascader from "@/components/common/CustomFieldCascader.vue";
import CustomFieldControl from "@/components/common/CustomFieldControl.vue";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";

const props = defineProps({
  field: { type: Object, required: true },
  meta: { type: Object, required: true },
  dragging: { type: Boolean, default: false },
  linkageEnabled: { type: Boolean, default: false },
  linkageTargets: { type: Array, default: () => [] },
});
const emit = defineEmits(["update", "remove", "dragstart", "dragend", "drop"]);
const { t } = useI18n();

const fieldTypeLabelKeys = {
  text: "settings_fieldTypeText",
  textarea: "settings_fieldTypeTextarea",
  number: "settings_fieldTypeNumber",
  numeric: "settings_fieldTypeNumeric",
  phone: "settings_fieldTypePhone",
  date: "settings_fieldTypeDate",
  datetime: "settings_fieldTypeDateTime",
  select_single: "settings_fieldTypeSelect",
  select_multi: "settings_fieldTypeMultiSelect",
  user: "settings_fieldTypeSingleMember",
  user_multi: "settings_fieldTypeMultiMember",
  cascader_single: "settings_fieldTypeCascaderRadio",
  cascader_multi: "settings_fieldTypeCascaderCheckbox",
};
const typeLabelKey = computed(() => {
  const widget = props.meta.widget;
  if (widget === "unix-date" || widget === "date")
    return "settings_fieldTypeDate";
  if (props.meta.fieldType === "assignee") return "settings_fieldTypeMember";
  if (fieldTypeLabelKeys[props.meta.fieldType])
    return fieldTypeLabelKeys[props.meta.fieldType];
  if (widget === "select" || widget === "multi")
    return "settings_fieldTypeSelect";
  if (widget === "number") return "settings_fieldTypeNumber";
  if (widget === "textarea") return "settings_fieldTypeTextarea";
  return "settings_fieldTypeText";
});
const isDate = computed(
  () => props.meta.widget === "unix-date" || props.meta.widget === "date",
);
const dynamicDate = computed(() => {
  const value = props.field.default_value;
  return value && typeof value === "object" && !Array.isArray(value)
    ? value
    : null;
});
const dateMode = computed(() => dynamicDate.value?.date_type || "fixed");
const dateOffset = computed(() => Number(dynamicDate.value?.offset_days) || 1);
const dateModeOptions = computed(() => [
  { value: "fixed", label: t("settings_template_date_fixed") },
  { value: "current", label: t("settings_template_date_current") },
  { value: "after", label: t("settings_template_date_after") },
  { value: "before", label: t("settings_template_date_before") },
]);
const unixDate = computed(() => {
  if (dateMode.value !== "fixed") return null;
  const value = Number(props.field.default_value);
  return value > 0 ? dayjs.unix(value) : null;
});
const plainDate = computed(() => {
  if (dateMode.value !== "fixed") return null;
  return props.field.default_value ? dayjs(props.field.default_value) : null;
});
const arrayValue = computed(() =>
  Array.isArray(props.field.default_value) ? props.field.default_value : [],
);
const datetimeValue = computed(() =>
  props.field.default_value ? dayjs(props.field.default_value) : null,
);

function patch(partial) {
  emit("update", { ...props.field, ...partial });
}
function onUnixDate(value) {
  patch({ default_value: value ? value.startOf("day").unix() : null });
}
function onPlainDate(value) {
  patch({ default_value: value ? value.format("YYYY-MM-DD") : null });
}
function onDatetime(value) {
  patch({
    default_value: value ? value.format("YYYY-MM-DD HH:mm:ss") : null,
  });
}
function onPhone(value) {
  patch({ default_value: value || null });
}
function onDateMode(mode) {
  if (mode === "fixed") {
    patch({ default_value: null });
    return;
  }
  patch({
    default_value: {
      date_type: mode,
      ...(mode === "current" ? {} : { offset_days: 1 }),
    },
  });
}
function onDateOffset(days) {
  patch({
    default_value: {
      date_type: dateMode.value,
      offset_days: Math.min(36500, Math.max(1, Number(days) || 1)),
    },
  });
}
function addLinkage() {
  patch({
    linkages: [
      ...(props.field.linkages || []),
      { value: null, field_keys: [] },
    ],
  });
}
function linkageValueOptions(index) {
  const used = new Set(
    (props.field.linkages || [])
      .filter((_, ruleIndex) => ruleIndex !== index)
      .map((rule) => String(rule.value)),
  );
  return (props.meta.options || []).filter(
    (option) => !used.has(String(option.value)),
  );
}
function updateLinkage(index, partial) {
  const linkages = (props.field.linkages || []).map((rule, ruleIndex) =>
    ruleIndex === index ? { ...rule, ...partial } : rule,
  );
  patch({ linkages });
}
function removeLinkage(index) {
  patch({
    linkages: (props.field.linkages || []).filter(
      (_, ruleIndex) => ruleIndex !== index,
    ),
  });
}
</script>

<style scoped lang="less">
.field-row-wrap {
  border-bottom: 1px solid #f0f2f5;
}
.field-row-wrap.dragging {
  opacity: 0.5;
}
.field-row {
  display: grid;
  grid-template-columns:
    24px minmax(80px, 140px) 72px minmax(220px, 1fr)
    auto auto 32px;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
}
.drag-handle {
  color: #98a2b3;
  cursor: grab;
}
.field-name {
  overflow: hidden;
  color: #30394a;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.type-tag {
  margin: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.default-value {
  min-width: 0;
  :deep(.ant-picker),
  :deep(.ant-select),
  :deep(.ant-input),
  :deep(.ant-input-number) {
    width: 100%;
  }
}
.date-default {
  display: grid;
  grid-template-columns: minmax(92px, 120px) minmax(96px, 1fr) auto;
  align-items: center;
  gap: 8px;
}
.linkage-list {
  padding: 4px 12px 12px 44px;
  background: #f7f8fa;
}
.linkage-rule {
  display: grid;
  grid-template-columns:
    auto auto minmax(120px, 180px) auto minmax(200px, 1fr)
    32px;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  color: #344054;
}
</style>
