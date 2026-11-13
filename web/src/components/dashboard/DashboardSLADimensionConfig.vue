<template>
  <a-form-item :label="$t('dashboard_category_dimension')">
    <a-radio-group
      :value="config.sla_dimension"
      @update:value="emit('dimension', $event)"
    >
      <a-radio-button value="category">{{
        $t("dashboard_sla_by_category")
      }}</a-radio-button>
      <a-radio-button value="priority">{{
        $t("dashboard_sla_by_priority")
      }}</a-radio-button>
    </a-radio-group>
  </a-form-item>
  <a-form-item
    v-if="byPriority"
    :label="$t('dashboard_category_all_priorities')"
    :extra="$t('dashboard_sla_dimension_all')"
  >
    <a-select
      v-model:value="config.priority_ids"
      :options="priorityOptions"
      mode="multiple"
      option-filter-prop="label"
      :placeholder="$t('dashboard_category_all_priorities')"
      :aria-label="$t('dashboard_category_all_priorities')"
      allow-clear
    />
  </a-form-item>
  <a-form-item
    v-else
    :label="$t('dashboard_workload_all_categories')"
    :extra="$t('dashboard_sla_category_rollup')"
  >
    <a-spin :spinning="loading">
      <a-alert
        v-if="failed"
        type="error"
        :message="$t('dashboard_chart_failed')"
      >
        <template #action>
          <a-button size="small" @click="loadCategories">{{
            $t("issue_retry")
          }}</a-button>
        </template>
      </a-alert>
      <template v-else-if="!loading">
        <a-alert
          v-if="staleCategoryIds.length"
          type="warning"
          :message="$t('dashboard_sla_category_stale')"
        >
          <template #action>
            <a-button size="small" @click="config.sla_category_ids = []">{{
              $t("dashboard_scope_all")
            }}</a-button>
          </template>
        </a-alert>
        <a-tree
          class="sla-dimension-config__tree"
          checkable
          check-strictly
          show-line
          default-expand-all
          :tree-data="tree"
          :checked-keys="{
            checked: config.sla_category_ids || [],
            halfChecked: [],
          }"
          @check="
            ({ checked }) =>
              (config.sla_category_ids = checked.filter((id) =>
                parents.has(id),
              ))
          "
        />
      </template>
    </a-spin>
  </a-form-item>
  <a-form-item
    v-for="choice in colorOptions"
    :key="choice.key"
    :label="choice.label"
  >
    <DashboardColorPicker
      :value="choice.value"
      :label="choice.label"
      @update:value="setColor(choice.key, $event)"
    />
  </a-form-item>
</template>

<script setup>
import { computed, ref, watch, onBeforeUnmount } from "vue";
import { listRequirementCategories } from "@/api/workspace";
import { useDashboardI18n } from "@/composables/useDashboardI18n";
import DashboardColorPicker from "./DashboardColorPicker.vue";
import { slaCategoryColor } from "./dashboardCharts";

const props = defineProps({
  config: { type: Object, required: true },
  wsId: { type: [Number, String], required: true },
  priorities: { type: Array, default: () => [] },
});
const emit = defineEmits(["dimension"]);
const { t } = useDashboardI18n();
const byPriority = computed(() => props.config.sla_dimension === "priority");
const categories = ref([]);
const loading = ref(false);
const failed = ref(false);
let sequence = 0;

const priorityOptions = computed(() => [
  ...props.priorities.map((item) => ({
    value: Number(item.id),
    label: item.label || item.name,
  })),
  { value: 0, label: t("dashboard_chart_unset") },
]);
const parents = computed(
  () =>
    new Map([
      [0, 0],
      ...categories.value.map((item) => [
        Number(item.id),
        Number(item.parent_id),
      ]),
    ]),
);
const staleCategoryIds = computed(() =>
  (props.config.sla_category_ids || []).filter((id) => !parents.value.has(id)),
);
const tree = computed(() => {
  const nodes = new Map(
    categories.value.map((item) => [
      Number(item.id),
      { key: Number(item.id), title: item.name, children: [] },
    ]),
  );
  const roots = [];
  for (const [id, node] of nodes) {
    const parent = nodes.get(parents.value.get(id));
    (parent?.children || roots).push(node);
  }
  roots.push({ key: 0, title: t("sla_uncategorized") });
  return roots;
});
const colorOptions = computed(() => {
  if (byPriority.value || failed.value || loading.value) return [];
  const selected = new Set(props.config.sla_category_ids || []);
  const choices = [
    ...categories.value.map((item) => ({
      id: Number(item.id),
      name: item.name,
    })),
    { id: 0, name: t("sla_uncategorized") },
  ];
  return choices
    .filter(({ id }) => {
      let parent = parents.value.get(id);
      if (!selected.size) return !parent;
      if (!selected.has(id)) return false;
      // Match the server's three-level category rollup.
      for (let depth = 0; parent > 0 && depth < 2; depth++) {
        if (selected.has(parent)) return false;
        parent = parents.value.get(parent);
      }
      return true;
    })
    .map(({ id, name }) => ({
      key: id,
      label: name,
      value: slaCategoryColor(props.config, id),
    }));
});

function setColor(key, color) {
  props.config.sla_category_colors = {
    ...props.config.sla_category_colors,
    [key]: color,
  };
}

async function loadCategories() {
  const request = ++sequence;
  loading.value = true;
  failed.value = false;
  try {
    const result = await listRequirementCategories(props.wsId);
    if (request === sequence) categories.value = result?.list || [];
  } catch {
    if (request === sequence) failed.value = true;
  } finally {
    if (request === sequence) loading.value = false;
  }
}
watch(
  [() => props.wsId, byPriority],
  ([, priority]) => {
    sequence++;
    categories.value = [];
    if (!priority) void loadCategories();
  },
  { immediate: true },
);
onBeforeUnmount(() => sequence++);
</script>

<style scoped lang="less">
.sla-dimension-config__tree {
  max-height: 210px;
  overflow: auto;
  padding: 8px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
}
</style>
