<template>
  <div class="create-top">
    <div class="create-header-main">
      <a-button
        type="text"
        class="back-button"
        :aria-label="$t('common_back')"
        @click="$emit('back')"
      >
        <ArrowLeftOutlined />
      </a-button>
      <h4 class="create-title">{{ $t("requirement_create") }}</h4>
      <a-select
        :value="selectedWs"
        class="workspace-select"
        :placeholder="$t('requirement_selectProject')"
        @update:value="$emit('update:selectedWs', $event)"
      >
        <a-select-option v-for="ws in workspaces" :key="ws.id" :value="ws.id">
          <span class="ws-option">
            <img
              v-if="ws.icon || workspaceStore.iconUrls[ws.id]"
              :src="ws.icon || workspaceStore.iconUrls[ws.id]"
              alt=""
              class="ws-icon-image"
            />
            <span
              v-else
              class="ws-icon"
              :style="{ backgroundColor: ws.color || '#0d68ff' }"
            >
              {{ (ws.name || "").trim().charAt(0) }}
            </span>
            <span class="ws-name">{{ ws.name }}</span>
          </span>
        </a-select-option>
      </a-select>
      <a-radio-group
        :value="type"
        class="type-switch"
        @update:value="$emit('update:type', $event)"
      >
        <a-radio-button
          v-if="canCreateRequirement"
          value="requirement"
          class="requirement-option"
        >
          <WorkItemTypePill
            type="requirement"
            :ws-id="selectedWs"
            class="title-type-pill"
          />
          <span class="ml4">{{
            workItemAttributes.displayName("requirement")
          }}</span>
        </a-radio-button>
        <a-radio-button
          v-if="canCreateDefect"
          value="defect"
          class="defect-option"
        >
          <WorkItemTypePill
            type="defect"
            :ws-id="selectedWs"
            class="title-type-pill"
          />
          <span class="ml4">{{
            workItemAttributes.displayName("defect")
          }}</span>
        </a-radio-button>
      </a-radio-group>
      <a-select
        v-if="showTemplates"
        :value="templateId"
        class="template-select"
        :options="templateOptions"
        @update:value="$emit('update:templateId', $event)"
      />
    </div>
  </div>
</template>

<script setup>
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { ArrowLeftOutlined } from "@ant-design/icons-vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import { useWorkItemTypeAttributes } from "@/composables/useWorkItemTypeAttributes";
import { useWorkspaceStore } from "@/stores/workspace";

const props = defineProps({
  type: { type: String, default: "requirement" },
  selectedWs: { type: String, default: "" },
  workspaces: { type: Array, default: () => [] },
  canCreateRequirement: { type: Boolean, default: false },
  canCreateDefect: { type: Boolean, default: false },
  showTemplates: { type: Boolean, default: false },
  templateId: { type: [Number, String], default: undefined },
  templates: { type: Array, default: () => [] },
});
defineEmits(["back", "update:type", "update:selectedWs", "update:templateId"]);

const { t } = useI18n();
const workspaceStore = useWorkspaceStore();
// The type names follow the currently selected workspace's attribute settings;
// before a workspace is chosen they fall back to the built-in labels.
const wsIdRef = computed(() => props.selectedWs);
const workItemAttributes = useWorkItemTypeAttributes(wsIdRef);
const templateOptions = computed(() =>
  (props.templates || []).map((item) => ({
    value: Number(item.id),
    label: t("settings_template_select_label", { name: item.name }),
  })),
);
</script>

<style scoped lang="less">
.create-top {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 16px;
  height: 68px;
  padding: 0 24px;
  border-bottom: 1px solid #e6eaf0;
}
.create-header-main {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.back-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  border-radius: 8px;
  color: #657188;
}
.create-title {
  margin: 0;
  color: #30394a;
  font-size: 17px;
  font-weight: 600;
  line-height: 22px;
  white-space: nowrap;
}
.ml4 {
  margin-left: 4px;
}
.workspace-select,
.template-select {
  width: 220px;
  flex-shrink: 0;
  :deep(.ant-select-selector) {
    height: 34px !important;
    padding: 0 12px !important;
    border-color: #dce2ea !important;
    border-radius: 5px !important;
  }
  :deep(.ant-select-selection-item),
  :deep(.ant-select-selection-placeholder) {
    font-size: 12px;
    line-height: 32px !important;
  }
}
.ws-option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.ws-icon,
.ws-icon-image {
  width: 20px;
  height: 20px;
  border-radius: 6px;
  object-fit: cover;
}
.ws-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 12px;
}
.type-switch {
  display: flex;
  align-items: center;
  gap: 3px;
}
.type-switch :deep(.ant-radio-button-wrapper) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 72px;
  height: 32px;
  padding: 0 12px;
  border: 0;
  border-radius: 7px;
  color: #7a8598;
  font-size: 13px;
  font-weight: 500;
  line-height: 32px;
  box-shadow: none;
}
.type-switch :deep(.ant-radio-button-wrapper > .title-type-pill) {
  margin-right: 8px;
}
.type-switch :deep(.ant-radio-button-wrapper::before) {
  display: none;
}
.type-switch :deep(.requirement-option.ant-radio-button-wrapper-checked) {
  color: #3157e2;
  background: #eef2ff;
  box-shadow: none;
}
.type-switch :deep(.defect-option.ant-radio-button-wrapper-checked) {
  color: #e5484d;
  background: #fff1f0;
  box-shadow: none;
}
@media (max-width: 900px) {
  .create-top {
    height: auto;
    min-height: 68px;
    padding: 10px 16px;
  }
}
</style>
