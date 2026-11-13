<template>
  <div class="workitem-attribute-tab">
    <a-spin :spinning="loading">
      <div class="attr-form">
        <div class="attr-row">
          <label class="attr-label">
            <span class="attr-required">*</span>{{ $t("workitem_attribute_name") }}
          </label>
          <div class="attr-control">
            <div class="inline-field">
              <a-input
                v-model:value="form.name"
                :placeholder="$t('workitem_attribute_namePlaceholder')"
                :maxlength="20"
                class="attr-input"
                @input="handleNameInput"
              />
              <span class="attr-counter">{{ charCount(form.name) }} / 20</span>
            </div>
          </div>
        </div>

        <div class="attr-row">
          <label class="attr-label">
            <span class="attr-required">*</span>{{ $t("workitem_attribute_iconSetting") }}
          </label>
          <div class="attr-control">
            <div class="icon-text-line">
              <a-input
                :value="form.iconText"
                class="attr-input attr-input--icon-text"
                :maxlength="4"
                :placeholder="$t('workitem_attribute_iconTextPlaceholder')"
                @input="handleIconTextInput"
              >
                <template #prefix>
                  <a-popover
                    v-model:open="colorPopoverOpen"
                    trigger="click"
                    placement="bottomLeft"
                    :overlay-inner-style="{ padding: '8px' }"
                  >
                    <button
                      type="button"
                      class="icon-color-trigger"
                      :style="{ backgroundColor: form.iconColor }"
                      :aria-label="$t('workitem_attribute_iconColor')"
                      @mousedown.prevent
                    />
                    <template #content>
                      <div class="icon-swatches">
                        <button
                          v-for="color in presetColors"
                          :key="color"
                          type="button"
                          class="icon-swatch"
                          :class="{ 'icon-swatch--active': form.iconColor === color }"
                          :style="{ backgroundColor: color }"
                          :aria-label="color"
                          @click="selectColor(color)"
                        />
                      </div>
                    </template>
                  </a-popover>
                </template>
              </a-input>
              <span class="attr-counter">{{ charCount(form.iconText) }} / 2</span>
            </div>
          </div>
        </div>

        <div class="attr-row attr-row--center">
          <label class="attr-label">{{ $t("workitem_attribute_preview") }}</label>
          <div class="attr-control">
            <div class="preview-line">
              <span
                class="preview-badge"
                :style="{ backgroundColor: form.iconColor }"
              >
                {{ previewText }}
              </span>
              <span class="preview-text">{{ previewText }}</span>
            </div>
          </div>
        </div>

        <div class="attr-row">
          <label class="attr-label">{{ $t("workitem_attribute_description") }}</label>
          <div class="attr-control">
            <a-textarea
              v-model:value="form.description"
              :placeholder="$t('workitem_attribute_descriptionPlaceholder')"
              :rows="3"
              :maxlength="200"
              class="attr-textarea"
            />
            <div class="attr-counter-right">
              {{ charCount(form.description) }} / 200
            </div>
          </div>
        </div>

        <div class="attr-row">
          <label class="attr-label" />
          <div class="attr-control">
            <a-button type="primary" :loading="saving" :disabled="!changed" @click="handleSave">
              {{ $t("common_update") }}
            </a-button>
          </div>
        </div>
      </div>
    </a-spin>
  </div>
</template>

<script setup>
import { computed, reactive, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";
import { saveWorkItemAttribute } from "@/api/workspace";
import {
  DEFAULT_TYPE_ICON_COLORS,
  DEFAULT_TYPE_LABEL_KEYS,
  cacheWorkItemAttribute,
  ensureWorkItemTypeAttributes,
} from "@/composables/useWorkItemTypeAttributes";

const props = defineProps({
  typeKey: { type: String, default: "requirement" },
  wsId: { type: [String, Number], default: "" },
});

const { t } = useI18n();
const route = useRoute();
const wsId = props.wsId || route.params.wsId || "";

const presetColors = [
  "#3582fb",
  "#e45454",
  "#0d68ff",
  "#52c41a",
  "#fa8c16",
  "#ff4d4f",
  "#722ed1",
  "#13c2c2",
  "#eb2f96",
  "#fa541c",
  "#faad14",
  "#8c8c8c",
];

const form = reactive({
  name: "",
  iconColor: DEFAULT_TYPE_ICON_COLORS[props.typeKey] || "#3582fb",
  iconText: "",
  description: "",
});
const colorPopoverOpen = ref(false);
// icon text follows the name prefix until the user edits the icon text manually.
const iconTextEditedByUser = ref(false);

function selectColor(color) {
  form.iconColor = color;
  colorPopoverOpen.value = false;
}
// Badge and side-by-side text share the icon text; fall back to the name prefix while typing.
const previewText = computed(
  () => form.iconText || firstChars(form.name, 2),
);
const loading = ref(true);
const saving = ref(false);
const initialized = ref(false);

const changed = computed(() => {
  const attr = attrSnapshot.value;
  return (
    !attr ||
    form.name.trim() !== (attr.name || "") ||
    form.iconColor !== (attr.icon_color || "") ||
    form.iconText !== (attr.icon_text || "") ||
    form.description.trim() !== (attr.description || "")
  );
});
const attrSnapshot = ref(null);

function firstChars(value, max) {
  return Array.from(value || "").slice(0, max).join("");
}
function charCount(value) {
  return Array.from(value || "").length;
}

function handleNameInput(event) {
  const next = event.target.value;
  if (!iconTextEditedByUser.value) {
    form.iconText = firstChars(next, 2);
  }
}
function handleIconTextInput(event) {
  iconTextEditedByUser.value = true;
  form.iconText = firstChars(event.target.value, 2);
}

function applyRow(row) {
  const defaultName = t(
    DEFAULT_TYPE_LABEL_KEYS[props.typeKey] || "requirement_feature",
  );
  const defaultColor =
    DEFAULT_TYPE_ICON_COLORS[props.typeKey] || "#3582fb";
  const name = row?.name || defaultName;
  const namePrefix = firstChars(name, 2);
  form.name = name;
  form.iconColor = row?.icon_color || defaultColor;
  form.iconText = row?.icon_text || namePrefix;
  form.description = row?.description || "";
  // A stored icon text differing from its name prefix is a manual customization.
  iconTextEditedByUser.value = row ? (row.icon_text || "") !== namePrefix : false;
  attrSnapshot.value = row || null;
}

async function loadRow() {
  loading.value = true;
  try {
    const data = await ensureWorkItemTypeAttributes(wsId);
    applyRow(data?.[props.typeKey] || null);
  } finally {
    loading.value = false;
    initialized.value = true;
  }
}

async function handleSave() {
  const name = form.name.trim();
  const iconText = form.iconText.trim();
  if (!name) {
    message.error(t("workitem_attribute_namePlaceholder"));
    return;
  }
  if (!iconText) {
    message.error(t("workitem_attribute_iconTextPlaceholder"));
    return;
  }
  saving.value = true;
  try {
    const row = {
      type_key: props.typeKey,
      name,
      icon_color: form.iconColor,
      icon_text: iconText,
      description: form.description.trim(),
    };
    await saveWorkItemAttribute(wsId, props.typeKey, {
      name,
      icon_color: form.iconColor,
      icon_text: iconText,
      description: form.description.trim(),
    });
    cacheWorkItemAttribute(wsId, props.typeKey, row);
    applyRow(row);
    message.success(t("workitem_attribute_updated"));
  } finally {
    saving.value = false;
  }
}

watch(
  () => props.typeKey,
  () => {
    initialized.value = false;
    form.iconColor = DEFAULT_TYPE_ICON_COLORS[props.typeKey] || "#3582fb";
    loadRow();
  },
  { immediate: true },
);
</script>

<style scoped lang="less">
.workitem-attribute-tab {
  max-width: 720px;
}
.attr-form {
  padding: 8px 0;
}
.attr-row {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 18px;
  &--center {
    align-items: center;
  }
}
.attr-label {
  flex: 0 0 96px;
  padding-top: 6px;
  color: #30394a;
  font-size: 13px;
  font-weight: 600;
  text-align: right;
  .attr-row--center & {
    padding-top: 0;
  }
}
.attr-required {
  margin-right: 2px;
  color: #ff4d4f;
}
.attr-control {
  flex: 1;
  min-width: 0;
}
.inline-field {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 10px;
}
.icon-text-line {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}
.attr-input {
  width: 320px;
}
.attr-input--icon-text {
  width: 200px;
}
.attr-textarea {
  display: block;
  width: 480px;
}
.attr-counter {
  color: #b7bcc5;
  font-size: 12px;
  line-height: 16px;
  white-space: nowrap;
}
.attr-counter-right {
  width: 480px;
  margin-top: 4px;
  color: #b7bcc5;
  font-size: 12px;
  line-height: 16px;
  text-align: right;
}
.icon-color-trigger {
  display: inline-block;
  width: 16px;
  height: 16px;
  margin: 0 2px 0 0;
  padding: 0;
  border: 1px solid rgba(0, 0, 0, 0.12);
  border-radius: 3px;
  cursor: pointer;
  outline: none;
}
.icon-swatches {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  width: 184px;
}
.icon-swatch {
  width: 24px;
  height: 24px;
  padding: 0;
  border: 2px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  outline: none;
  transition: transform 0.12s ease;
  &:hover {
    transform: scale(1.08);
  }
  &--active {
    border-color: #fff;
    box-shadow: 0 0 0 2px #30394a;
  }
}
.preview-line {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #30394a;
  font-size: 13px;
}
.preview-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 34px;
  height: 22px;
  padding: 0 6px;
  box-sizing: border-box;
  border-radius: 4px;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  line-height: 22px;
  text-align: center;
  white-space: nowrap;
}
.preview-text {
  color: #30394a;
  font-size: 13px;
  line-height: 22px;
}
</style>
