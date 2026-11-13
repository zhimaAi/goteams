<template>
  <div class="type-settings">
    <div v-if="!templateEditing" class="type-segmented-sticky">
      <a-segmented
        v-model:value="sub"
        :options="subOptions"
        class="type-segmented"
      />
    </div>
    <div class="type-panel">
      <WorkItemAttributesTab
        v-if="sub === 'attribute' && canAttribute"
        :ws-id="wsId"
        :type-key="typeKey"
      />
      <ListFieldsTab
        v-else-if="sub === 'fields' && canField"
        :ws-id="wsId"
        :type-key="typeKey"
      />
      <StatusTab
        v-else-if="sub === 'status' && canStatus"
        :ws-id="wsId"
        :type-key="typeKey"
      />
      <PriorityTab
        v-else-if="sub === 'priority' && canPriority"
        :ws-id="wsId"
        :type-key="typeKey"
      />
      <TemplateTab
        v-else-if="sub === 'template' && canTemplate"
        :ws-id="wsId"
        :type-key="typeKey"
        @editing-change="templateEditing = $event"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watchEffect } from "vue";
import { useI18n } from "vue-i18n";
import WorkItemAttributesTab from "./WorkItemAttributesTab.vue";
import ListFieldsTab from "./ListFieldsTab.vue";
import StatusTab from "./StatusTab.vue";
import PriorityTab from "./PriorityTab.vue";
import TemplateTab from "./TemplateTab.vue";
import { useRoute } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { typeSettingPermission } from "@/utils/typePermission";

const route = useRoute();
const authStore = useAuthStore();
const { t: $t } = useI18n();
const props = defineProps({
  typeKey: { type: String, default: "requirement" },
  wsId: { type: [String, Number], default: "" },
});
const wsId = props.wsId || route.params.wsId || "";
const sub = ref("attribute");
const templateEditing = ref(false);
const canAttribute = computed(() => {
  // Planning has no attribute tab; the work item attribute config covers requirement/defect only.
  if (props.typeKey === "planning") return false;
  return authStore.hasPermission(typeSettingPermission(props.typeKey, "attribute"));
});
const canField = computed(() =>
  authStore.hasPermission(typeSettingPermission(props.typeKey, "field")),
);
const canStatus = computed(() =>
  authStore.hasPermission(typeSettingPermission(props.typeKey, "status")),
);
const canPriority = computed(() =>
  authStore.hasPermission(typeSettingPermission(props.typeKey, "priority")),
);
const canTemplate = computed(
  () => props.typeKey !== "planning" && canField.value,
);
const visibleTabs = computed(() =>
  [
    canAttribute.value && "attribute",
    canField.value && "fields",
    canStatus.value && "status",
    canPriority.value && "priority",
    canTemplate.value && "template",
  ].filter(Boolean),
);
const subOptions = computed(() =>
  [
    canAttribute.value && {
      value: "attribute",
      label: $t("settings_workItemAttributes"),
    },
    canField.value && {
      value: "fields",
      label: $t("settings_attributeFields"),
    },
    canStatus.value && { value: "status", label: $t("settings_status") },
    canPriority.value && { value: "priority", label: $t("settings_priority") },
    canTemplate.value && { value: "template", label: $t("settings_template") },
  ].filter(Boolean),
);

// `?tab=` deep links select a sub tab (e.g. the invite settings' "添加模板"
// jump); appliedTab guards so manual tab switches are not overwritten while the
// query param still points at the previous tab.
let appliedTab = '';
watchEffect(() => {
  const queryTab = typeof route.query.tab === "string" ? route.query.tab : "";
  if (queryTab && queryTab !== appliedTab) {
    appliedTab = queryTab;
    if (visibleTabs.value.includes(queryTab)) {
      sub.value = queryTab;
    }
  }
  if (sub.value !== "template") templateEditing.value = false;
  if (!visibleTabs.value.includes(sub.value)) {
    sub.value = visibleTabs.value[0] || "fields";
  }
});
</script>

<style scoped lang="less">
.type-settings {
  min-height: 100%;
}
.type-segmented-sticky {
  position: sticky;
  top: 0;
  z-index: 5;
  background: #fff;
}
.type-segmented {
  height: 36px;
  padding: 3px;
  border-radius: 8px;
  background: #f0f2f5;

  :deep(.ant-segmented-item) {
    min-width: 72px;
    color: #6b7280;
    font-size: 13px;
    line-height: 28px;
    border-radius: 6px;
  }
  :deep(.ant-segmented-item-selected) {
    color: #3157e2;
    font-weight: 600;
    box-shadow: 0 1px 3px rgba(24, 43, 80, 0.12);
  }
}
.type-panel {
  padding: 8px 0 24px;
}
</style>
