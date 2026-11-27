<template>
  <a-drawer
    :open="open"
    :width="'min(1400px, 94vw)'"
    :title="title"
    @close="$emit('close')"
  >
    <template #extra>
      <a-space>
        <a-button :aria-expanded="searchOpen" @click="searchOpen = !searchOpen">
          {{ $t("common_search") }}
        </a-button>
        <a-button @click="settings = true">{{
          $t("common_settings")
        }}</a-button>
      </a-space>
    </template>
    <WorkItemSearchBar
      v-if="searchOpen"
      :filters="searchFilters"
      type-key="report"
      :fields="searchFields"
      :members="metadata?.members || []"
      :statuses="filterOptions?.statuses || statuses"
      :priorities="searchPriorities"
      :versions="metadata?.versions || []"
      :can-save-view="false"
      class="detail-search"
      @change="changeSearch"
      @search="applySearch"
    />
    <a-alert
      v-if="failed"
      type="error"
      :message="$t('common_requestFailed')"
      show-icon
    >
      <template #action>
        <a-button size="small" @click="load">{{ $t("report_retry") }}</a-button>
      </template>
    </a-alert>
    <a-table
      :columns="columns"
      :data-source="items"
      :loading="loading"
      :pagination="pagination"
      :scroll="{ x: 'max-content' }"
      row-key="id"
      size="middle"
      @change="changePage"
    >
      <template #bodyCell="{ column, text, record }">
        <a-button
          v-if="column.key === 'title'"
          type="link"
          class="report-table-link"
          @click="openItem(record.id)"
        >
          {{ text }}
        </a-button>
        <template
          v-else-if="['created_at', 'status_changed_at'].includes(column.key)"
        >
          {{ formatLocalDate(text, true) }}
        </template>
        <template v-else-if="column.key === 'responsible_names'">
          {{
            record.responsible_names == null
              ? $t("report_unknown_responsible")
              : record.responsible_names.join(", ") ||
                $t("report_no_responsible")
          }}
        </template>
        <template v-else-if="column.key === 'counted_assignees'">
          {{ text?.join(", ") || "—" }}
        </template>
        <template v-else>{{ text ?? "—" }}</template>
      </template>
    </a-table>
  </a-drawer>
  <ListSettingsModal
    v-model:open="settings"
    :user-id="auth.userInfo?.id"
    :ws-id="wsId"
    :type-key="`report-${type}`"
    :fields="fields"
    hide-view-mode
    scoped-preview-mode
    @change="preferences = $event"
  />
  <RequirementDetailDrawer
    v-if="itemId"
    :open="true"
    :ws-id="wsId"
    :requirement-id="itemId"
    :mode="preferences.previewMode"
    @close="itemId = null"
    @navigate="openItem"
  />
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { getReportOptions, listReportItems } from "@/api/workspaceReport";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import ListSettingsModal from "@/components/work-item/ListSettingsModal.vue";
import WorkItemSearchBar from "@/components/work-item/WorkItemSearchBar.vue";
import { normalizeSearchFilterTree } from "@/constants/workItemView";
import { useAuthStore } from "@/stores/auth";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  open: Boolean,
  wsId: { type: [String, Number], required: true },
  type: { type: String, required: true },
  runId: { type: String, default: "" },
  previewTimezone: { type: String, default: "" },
  snapshotVersion: { type: Number, default: 1 },
  group: { type: Object, default: null },
  statuses: { type: Array, default: () => [] },
  priorities: { type: Array, default: () => [] },
  metadata: { type: Object, default: () => ({}) },
});
defineEmits(["close"]);
const { t } = useI18n();
const auth = useAuthStore();
const router = useRouter();
const searchOpen = ref(false);
const searchFilters = ref(defaultSearch());
const settings = ref(false);
const itemId = ref(null);
const items = ref([]);
const filterOptions = ref(null);
const loading = ref(false);
const failed = ref(false);
const pagination = ref({
  current: 1,
  pageSize: 40,
  pageSizeOptions: ["20", "40", "80", "100"],
  total: 0,
  showSizeChanger: true,
  showTotal: (count) => `${t("common_all")} ${count}`,
});
const preferences = ref({ previewMode: "drawer", fields: [] });
const searchFields = computed(() => [
  { field: "title", label: t("report_item_title"), control: "text" },
  {
    field: "status",
    label: t("requirement_filterStatus"),
    control: "multi",
  },
  {
    field: "priority",
    label: t("requirement_priority"),
    control: "multi",
  },
  { field: "assignee", label: t("report_assignees"), control: "person" },
  {
    field: "version",
    label: t("requirement_filterVersion"),
    control: "multi",
  },
]);
const searchPriorities = computed(() =>
  (filterOptions.value?.priorities || props.priorities).map((priority) => ({
    ...priority,
    label: priority.name,
  })),
);
const countsOnly = computed(() => props.type === "defects");
const title = computed(() => {
  if (!props.group) return "";
  return t(
    countsOnly.value
      ? "report_defect_details_title"
      : props.snapshotVersion >= 2
        ? "report_estimated_details_title"
        : "report_details_title",
    {
      name: props.group.name,
      count: props.group.count,
      hours: props.group.hours ?? "—",
    },
  );
});

const fields = computed(() => {
  const fieldsByName = [
    ["id", "report_item_id"],
    ["title", "report_item_title"],
    [
      "status",
      props.snapshotVersion >= 2
        ? "report_target_status"
        : "requirement_filterStatus",
    ],
    ...(props.snapshotVersion >= 2
      ? [["status_changed_at", "report_status_changed_at"]]
      : []),
    ["template", "report_template"],
    ["iteration", "requirement_filterVersion"],
    ["category", "report_category"],
    ["priority", "requirement_priority"],
    ...(props.type === "defects" || props.group?.dimension === "defect"
      ? [["severity", "report_severity"]]
      : []),
    ...(!countsOnly.value ? [["estimated", "report_estimated"]] : []),
    ["assignees", "report_assignees"],
    ...(props.type === "defects" || props.group?.dimension === "responsible"
      ? [["responsible_names", "workspace_field_responsible"]]
      : []),
    ...(!countsOnly.value && props.snapshotVersion >= 2
      ? [
          ["counted_assignees", "report_counted_assignees"],
          ["counted_hours", "report_counted_hours"],
        ]
      : []),
    ["created_at", "report_item_created_at"],
  ];
  return fieldsByName.map(([key, label]) => ({
    key,
    preferenceKey: `builtin:${key}`,
    label: t(label),
    kind: "builtin",
    locked: key === "title",
  }));
});
const columns = computed(() => {
  const choices = preferences.value.fields.length
    ? preferences.value.fields
    : fields.value;
  const labels = new Map(fields.value.map((field) => [field.key, field.label]));
  return choices
    .filter((field) => field.visible !== false && labels.has(field.key))
    .map(({ key }) => ({ key, dataIndex: key, title: labels.get(key) }));
});

let sequence = 0;
let appliedSearch = {};
function defaultSearch() {
  return normalizeSearchFilterTree(
    { conditions: [{ field: "title" }, { field: "status" }] },
    "report",
  );
}
function changeSearch(filters, detail) {
  searchFilters.value = filters;
  if (detail.immediateQuery) applySearch();
}
async function load() {
  const request = ++sequence;
  if (!props.open || (!props.runId && !props.previewTimezone) || !props.group)
    return;
  loading.value = true;
  failed.value = false;
  try {
    const [result, choices] = await Promise.all([
      listReportItems(props.wsId, props.type, props.runId, {
        group: props.group.key,
        timezone: props.previewTimezone || undefined,
        ...appliedSearch,
        page: pagination.value.current,
        size: pagination.value.pageSize,
      }),
      filterOptions.value ||
        getReportOptions(props.wsId, props.type, {
          item_type:
            props.type === "work-hours" ? props.group.dimension : undefined,
        }),
    ]);
    if (request !== sequence) return;
    filterOptions.value = choices;
    items.value = result.list;
    pagination.value.total = result.total;
  } catch {
    if (request === sequence) {
      failed.value = true;
      items.value = [];
      pagination.value.total = 0;
    }
  } finally {
    if (request === sequence) loading.value = false;
  }
}

function applySearch() {
  const conditions = Object.fromEntries(
    searchFilters.value.conditions.map(({ field, values }) => [field, values]),
  );
  appliedSearch = {
    search: conditions.title?.[0]?.trim(),
    status_ids: conditions.status?.join(","),
    priority_ids: conditions.priority?.join(","),
    assignee_ids: conditions.assignee?.join(","),
    iteration_names: JSON.stringify(
      (conditions.version || [])
        .map((id) =>
          id === "0"
            ? ""
            : (props.metadata?.versions || []).find(
                (version) => String(version.id) === id,
              )?.name,
        )
        .filter((name) => name !== undefined),
    ),
    item_type:
      props.type === "defects" || props.group?.dimension === "defect"
        ? "defect"
        : "requirement",
  };
  pagination.value.current = 1;
  load();
}

function changePage({ current, pageSize }) {
  pagination.value.current =
    pageSize === pagination.value.pageSize ? current : 1;
  pagination.value.pageSize = pageSize;
  load();
}

function openItem(id) {
  if (preferences.value.previewMode === "window") {
    const { href } = router.resolve({
      name: "WorkItemDetail",
      params: { wsId: props.wsId, id },
    });
    window.open(href, "_blank", "noopener");
    return;
  }
  itemId.value = id;
}

watch(
  () => [props.open, props.runId, props.previewTimezone, props.group?.key],
  () => {
    items.value = [];
    filterOptions.value = null;
    itemId.value = null;
    searchOpen.value = false;
    searchFilters.value = defaultSearch();
    loading.value = false;
    pagination.value.total = 0;
    applySearch();
  },
  { immediate: true },
);
</script>

<style scoped lang="less">
.report-table-link {
  padding-inline: 0;
}
:deep(.ant-table-pagination.ant-pagination) {
  margin: 12px 0 0;
}
.detail-search {
  margin-bottom: 16px;
}
</style>
