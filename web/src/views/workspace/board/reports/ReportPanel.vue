<template>
  <section class="report-panel">
    <aside class="report-nav" :aria-label="$t('report_views')">
      <strong>{{ $t("report_views") }}</strong>
      <a-button
        v-for="type in availableTypes"
        :key="type"
        :type="type === kind ? 'primary' : 'text'"
        @click="kind = type"
      >
        {{ $t(viewLabels[type]) }}
      </a-button>
    </aside>
    <main v-if="kind" class="report-content">
      <template v-if="!historyOpen">
        <h3 style="margin-bottom: 16px">{{ $t(viewLabels[kind]) }}</h3>
        <section class="report-filters">
          <div class="filter-grid">
            <label>
              {{ $t("report_dimension") }}
              <a-select
                v-if="kind === 'work-hours'"
                :value="filters.dimensions[0]"
                :options="dimensionOptions"
                @change="(value) => (filters.dimensions = [value])"
              />
              <a-select
                v-else
                v-model:value="filters.dimensions"
                mode="multiple"
                :max-tag-count="1"
                :options="dimensionOptions"
              />
            </label>
            <label>
              {{ $t("report_period") }}
              <a-segmented
                v-model:value="filters.preset"
                :options="periodOptions"
              />
            </label>
            <div
              v-if="filters.preset === 'custom'"
              role="group"
              :aria-label="$t('report_dates')"
            >
              <a-range-picker
                v-model:value="filters.dates"
                value-format="YYYY-MM-DD"
              />
            </div>
            <label>
              {{ $t("report_target_status") }}
              <a-select
                v-model:value="filters.statuses"
                mode="multiple"
                :max-tag-count="1"
                allow-clear
                :placeholder="$t('report_all_statuses')"
                :options="selectOptions(options.statuses)"
              />
            </label>
            <label>
              {{ $t("report_roles") }}
              <a-select
                v-model:value="filters.roles"
                mode="multiple"
                :max-tag-count="1"
                allow-clear
                :placeholder="$t('report_all_roles')"
                :options="selectOptions(options.roles)"
                @change="changeRoles"
              />
            </label>
            <label>
              {{ $t("report_members") }}
              <a-select
                v-model:value="filters.members"
                mode="multiple"
                :max-tag-count="1"
                allow-clear
                :filter-option="false"
                :loading="loadingMembers"
                :placeholder="$t('report_all_members')"
                :options="memberOptions"
                @search="searchMembers"
                @popupScroll="loadMoreMembers"
              />
            </label>
          </div>
          <div class="report-actions">
            <a-button
              v-if="canGenerate"
              type="primary"
              :loading="generating"
              :disabled="!canSubmit"
              @click="generate"
            >
              {{ $t("report_generate") }}
            </a-button>
            <a-space class="report-secondary-actions">
              <a-button
                v-if="canExport"
                :disabled="!result"
                :loading="exporting"
                @click="download(result)"
              >
                {{ $t("report_export") }}
              </a-button>
              <a-button @click="showHistory">{{
                $t("report_history")
              }}</a-button>
            </a-space>
          </div>
        </section>
        <a-alert
          v-if="optionError"
          type="error"
          :message="$t('common_requestFailed')"
          show-icon
        >
          <template #action
            ><a-button size="small" @click="loadOptions">{{
              $t("report_retry")
            }}</a-button></template
          >
        </a-alert>
        <a-alert
          v-if="generateError"
          type="error"
          :message="generateError"
          show-icon
        />
        <template v-if="result">
          <p class="report-period">
            {{
              $t(result.report.id ? "report_date_label" : "report_live_period")
            }}
            {{ period(result.report) }}
          </p>
          <a-table
            :columns="summaryColumns(result)"
            :data-source="result.rows"
            :pagination="summaryPagination"
            row-key="key"
            size="middle"
          >
            <template #bodyCell="{ column, record, text }">
              <a-button
                v-if="column.key === 'action'"
                type="link"
                class="report-table-link"
                @click="showDetails(result, record)"
              >
                {{ $t("report_details") }}
              </a-button>
              <template v-else-if="column.key === 'dimension'">{{
                dimensionLabel(result, text)
              }}</template>
              <template v-else>{{ text ?? "—" }}</template>
            </template>
          </a-table>
        </template>
        <div v-else class="report-empty report-empty-prompt" role="status">
          <a-spin v-if="generating" :tip="$t('common_loading')" />
          <span v-else>{{ $t("report_select_conditions") }}</span>
        </div>
      </template>
      <template v-else>
        <div class="history-heading">
          <a-breadcrumb :aria-label="$t('report_history')">
            <a-breadcrumb-item>
              <a href="#" @click.prevent="historyOpen = false">{{
                $t(viewLabels[kind])
              }}</a>
            </a-breadcrumb-item>
            <a-breadcrumb-item>
              <span aria-current="page">{{ $t("report_history") }}</span>
            </a-breadcrumb-item>
          </a-breadcrumb>
        </div>
        <a-alert
          v-if="historyError"
          type="error"
          :message="$t('common_requestFailed')"
          show-icon
        >
          <template #action
            ><a-button size="small" @click="loadHistory">{{
              $t("report_retry")
            }}</a-button></template
          >
        </a-alert>
        <a-table
          :columns="historyColumns"
          :data-source="history"
          :loading="loadingHistory"
          :pagination="historyPage"
          row-key="id"
          size="middle"
          @change="changeHistoryPage"
        >
          <template #bodyCell="{ column, record, text }">
            <template v-if="column.key === 'period'">{{
              period(record)
            }}</template>
            <template v-else-if="column.key === 'created_at'">{{
              formatLocalDate(text, true)
            }}</template>
            <a-space v-else-if="column.key === 'action'">
              <a-button
                type="link"
                class="report-table-link"
                @click="openHistory(record.id)"
              >
                {{ $t("report_view") }}
              </a-button>
              <a-button
                v-if="canExport"
                type="link"
                class="report-table-link"
                :loading="exporting && exportingRecord === record.id"
                :disabled="exporting && exportingRecord !== record.id"
                @click="download({ report: record })"
              >
                {{ $t("report_export") }}
              </a-button>
            </a-space>
            <template v-else-if="column.key === 'dimensions'">{{
              optionNames(text)
            }}</template>
            <template
              v-else-if="['statuses', 'roles', 'members'].includes(column.key)"
            >
              {{ optionNames(text) || $t(`report_all_${column.key}`) }}
            </template>
          </template>
        </a-table>
      </template>
    </main>
    <a-drawer
      :open="historyDrawer"
      :width="'min(1400px, 94vw)'"
      :title="$t('report_history')"
      @close="closeHistory"
    >
      <template #extra>
        <a-button
          v-if="canExport && historical"
          :loading="exporting"
          @click="download(historical)"
        >
          {{ $t("report_export") }}
        </a-button>
      </template>
      <a-alert
        v-if="historicalError"
        type="error"
        :message="$t('common_requestFailed')"
        show-icon
      />
      <a-table
        :columns="summaryColumns(historical)"
        :data-source="historical?.rows || []"
        :loading="loadingHistorical"
        :pagination="summaryPagination"
        row-key="key"
        size="middle"
      >
        <template #bodyCell="{ column, record, text }">
          <a-button
            v-if="column.key === 'action'"
            type="link"
            class="report-table-link"
            @click="showDetails(historical, record)"
          >
            {{ $t("report_details") }}
          </a-button>
          <template v-else-if="column.key === 'dimension'">{{
            dimensionLabel(historical, text)
          }}</template>
          <template v-else>{{ text ?? "—" }}</template>
        </template>
      </a-table>
    </a-drawer>
    <!-- Mount the detail portal after the drawer that opened it. -->
    <ReportDetails
      v-if="kind && details"
      :open="!!details"
      :ws-id="wsId"
      :type="kind"
      :run-id="details?.id || ''"
      :preview-timezone="details?.previewTimezone"
      :snapshot-version="details?.snapshotVersion"
      :group="details?.group"
      :statuses="options.statuses"
      :priorities="options.priorities"
      :metadata="metadata"
      @close="details = null"
    />
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { LeftOutlined } from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import {
  exportReport,
  generateReport,
  getReportOptions,
  getReportRun,
  listReportRuns,
} from "@/api/workspaceReport";
import { formatLocalDate } from "@/utils/date";
import ReportDetails from "./ReportDetails.vue";

const props = defineProps({
  wsId: { type: [Number, String], required: true },
  defectEnabled: Boolean,
  metadata: { type: Object, default: () => ({}) },
});
const { t } = useI18n();
const auth = useAuthStore();
const viewLabels = {
  "work-hours": "report_work_hours",
  defects: "report_defects",
};
const defaultFilters = (type) => ({
  dimensions:
    type === "work-hours"
      ? [auth.hasPermission("requirement:view") ? "requirement" : "defect"]
      : ["responsible", "severity", "priority", "category"],
  preset: "this-month",
  dates: null,
  statuses: [],
  roles: [],
  members: [],
});
function newReportView(type) {
  return {
    filters: defaultFilters(type),
    result: null,
  };
}
const views = reactive({
  "work-hours": newReportView("work-hours"),
  defects: newReportView("defects"),
});
const reportTimezone =
  Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
const availableTypes = computed(() => {
  if (!auth.hasPermission("report:view")) return [];
  return [
    ...(auth.hasPermission("requirement:view") ||
    (props.defectEnabled && auth.hasPermission("defect:view"))
      ? ["work-hours"]
      : []),
    ...(props.defectEnabled && auth.hasPermission("defect:view")
      ? ["defects"]
      : []),
  ];
});
const kind = ref("");
const currentView = computed(() => views[kind.value]);
const filters = computed(() => currentView.value?.filters);
const result = computed(() => currentView.value?.result);
const options = reactive({
  dimensions: [],
  statuses: [],
  priorities: [],
  roles: [],
});
const optionError = ref(false);
const optionsReady = ref(false);
const memberChoices = ref([]);
const currentMemberChoice = new Map();
const memberOptions = computed(() => selectOptions(memberChoices.value));
const memberSearch = ref("");
const memberPage = ref(1);
const memberTotal = ref(0);
const loadingMembers = ref(false);
let memberTimer;
let optionSequence = 0;
let kindSequence = 0;
let historySequence = 0;
let historicalSequence = 0;
let disposed = false;
const dimensionOptions = computed(() =>
  options.dimensions.map(({ key, name }) => ({ label: name, value: key })),
);
const periodOptions = computed(() => [
  { label: t("report_this_month"), value: "this-month" },
  { label: t("report_last_month"), value: "last-month" },
  { label: t("report_custom"), value: "custom" },
]);
const canGenerate = computed(() => auth.hasPermission("report:generate"));
const canExport = computed(() => auth.hasPermission("report:export"));
const canSubmit = computed(() => {
  const value = filters.value;
  return (
    optionsReady.value &&
    value.dimensions.length &&
    (value.preset !== "custom" ||
      (value.dates?.length === 2 && value.dates[0] <= value.dates[1]))
  );
});
const generating = ref(false);
const generateError = ref("");
const exporting = ref(false);
const exportingRecord = ref("");
const pending = new Map();

const historyOpen = ref(false);
const history = ref([]);
const loadingHistory = ref(false);
const historyError = ref(false);
const paginationOptions = {
  pageSizeOptions: ["20", "40", "80", "100"],
  showSizeChanger: true,
  showTotal: (count) => `${t("common_all")} ${count}`,
};
const historyPage = reactive({
  ...paginationOptions,
  current: 1,
  pageSize: 40,
  total: 0,
});
const historyDrawer = ref(false);
const historical = ref(null);
const historicalError = ref(false);
const loadingHistorical = ref(false);
const details = ref(null);

const summaryPagination = {
  ...paginationOptions,
  defaultPageSize: 40,
};
const countsOnly = () => kind.value === "defects";
const summaryColumns = (run) => {
  const groups =
    countsOnly() && run?.report.snapshot_version >= 3
      ? run.report.dimensions.map(({ key, name }, index) => ({
          key,
          dataIndex: ["values", index],
          title: name,
        }))
      : [
          ...(kind.value === "defects" || run?.report.snapshot_version >= 3
            ? [
                {
                  key: "dimension",
                  dataIndex: "dimension",
                  title: t("report_dimension"),
                },
              ]
            : []),
          {
            key: "name",
            dataIndex: "name",
            title: t(
              kind.value === "defects" ? "report_group" : "report_people",
            ),
          },
        ];
  return [
    ...groups,
    {
      key: "count",
      dataIndex: "count",
      title: t(
        kind.value === "defects"
          ? "report_defect_count"
          : run?.report.snapshot_version >= 3
            ? "report_work_item_count"
            : "report_requirement_count",
      ),
    },
    ...(!countsOnly()
      ? [
          {
            key: "hours",
            dataIndex: "hours",
            title: t(
              run?.report.snapshot_version >= 2
                ? "report_estimated_total"
                : kind.value === "work-hours"
                  ? "report_logged_hours"
                  : "report_total_hours",
            ),
          },
        ]
      : []),
    { key: "action", title: t("report_action") },
  ];
};
const historyColumns = computed(() =>
  [
    ["period", "report_period"],
    ["dimensions", "report_dimension"],
    ["statuses", "requirement_filterStatus"],
    ["roles", "report_roles"],
    ["members", "report_members"],
    ["creator", "report_creator"],
    ["created_at", "report_created_at"],
    ["action", "report_action"],
  ].map(([key, label]) => ({ key, dataIndex: key, title: t(label) })),
);

function selectOptions(list) {
  return list.map(({ id, name }) => ({ label: name || String(id), value: id }));
}
function optionNames(list) {
  return list?.map(({ name }) => name).join(", ") || "";
}
function dimensionLabel(run, dimension) {
  return (
    run?.report.dimensions.find(({ key }) => key === dimension)?.name ||
    dimension
  );
}
function period(report) {
  const dateInReportZone = new Intl.DateTimeFormat("sv-SE", {
    timeZone: report.timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  });
  return [report.period_start_at, report.period_end_at - 1]
    .map((seconds) => dateInReportZone.format(new Date(seconds * 1000)))
    .join(" – ");
}

async function loadOptions(page = 1) {
  if (!kind.value) return;
  const request = ++optionSequence;
  loadingMembers.value = true;
  optionError.value = false;
  try {
    const result = await getReportOptions(props.wsId, kind.value, {
      page,
      search: memberSearch.value,
      role_ids: filters.value.roles.join(","),
      item_type:
        kind.value === "work-hours" ? filters.value.dimensions[0] : undefined,
    });
    if (request !== optionSequence) return;
    options.dimensions = result.dimensions;
    const allowed = new Set(result.dimensions.map(({ key }) => key));
    filters.value.dimensions = filters.value.dimensions.filter((key) =>
      allowed.has(key),
    );
    if (kind.value === "work-hours" && !filters.value.dimensions.length) {
      filters.value.dimensions = result.dimensions
        .slice(0, 1)
        .map(({ key }) => key);
    }
    options.statuses = result.statuses;
    options.priorities = result.priorities;
    options.roles = result.roles;
    optionsReady.value = true;
    memberPage.value = page;
    memberTotal.value = result.members.total;
    if (page === 1) {
      for (const id of currentMemberChoice.keys()) {
        if (!filters.value.members.includes(id)) currentMemberChoice.delete(id);
      }
    }
    for (const option of result.members.list)
      currentMemberChoice.set(option.id, option);
    memberChoices.value = [...currentMemberChoice.values()];
  } catch {
    if (request === optionSequence) optionError.value = true;
  } finally {
    if (request === optionSequence) loadingMembers.value = false;
  }
}

function searchMembers(text) {
  clearTimeout(memberTimer);
  optionSequence++;
  memberSearch.value = text;
  memberTimer = setTimeout(() => loadOptions(), 200);
}

function changeRoles() {
  clearTimeout(memberTimer);
  filters.value.members = [];
  memberSearch.value = "";
  currentMemberChoice.clear();
  memberChoices.value = [];
  memberPage.value = 1;
  memberTotal.value = 0;
  optionsReady.value = false;
  loadOptions();
}

function loadMoreMembers({ target }) {
  if (
    !loadingMembers.value &&
    memberPage.value * 50 < memberTotal.value &&
    target.scrollTop + target.clientHeight >= target.scrollHeight - 12
  ) {
    loadOptions(memberPage.value + 1);
  }
}

function requestId() {
  // getRandomValues also works in local HTTP development environments.
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  const hex = Array.from(bytes, (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

async function generate() {
  if (!canSubmit.value || generating.value) return;
  const type = kind.value;
  const view = views[type];
  const seq = kindSequence;
  const body = {
    dimensions: view.filters.dimensions,
    preset: view.filters.preset,
    start_date: view.filters.dates?.[0],
    end_date: view.filters.dates?.[1],
    timezone: reportTimezone,
    status_ids: view.filters.statuses,
    role_ids: view.filters.roles,
    member_ids: view.filters.members,
  };
  const signature = JSON.stringify(body);
  const previous = pending.get(type);
  const id = previous?.signature === signature ? previous.id : requestId();
  pending.set(type, { id, signature });
  generating.value = true;
  generateError.value = "";
  try {
    const run = await generateReport(props.wsId, type, {
      ...body,
      request_id: id,
    });
    const snapshot = await getReportRun(props.wsId, type, run.id);
    if (
      disposed ||
      !availableTypes.value.includes(type) ||
      pending.get(type)?.id !== id
    )
      return;
    pending.delete(type);
    view.result = snapshot;
  } catch (error) {
    if (seq === kindSequence) {
      generateError.value =
        error.response?.data?.message || t("common_requestFailed");
    }
  } finally {
    if (seq === kindSequence) generating.value = false;
  }
}

async function loadHistory() {
  const request = ++historySequence;
  loadingHistory.value = true;
  historyError.value = false;
  try {
    const records = await listReportRuns(props.wsId, kind.value, {
      page: historyPage.current,
      size: historyPage.pageSize,
    });
    if (request !== historySequence) return;
    history.value = records.list;
    historyPage.total = records.total;
  } catch {
    if (request === historySequence) {
      historyError.value = true;
      history.value = [];
      historyPage.total = 0;
    }
  } finally {
    if (request === historySequence) loadingHistory.value = false;
  }
}
function showHistory() {
  historyOpen.value = true;
  historyPage.current = 1;
  loadHistory();
}
function changeHistoryPage({ current, pageSize }) {
  historyPage.current = pageSize === historyPage.pageSize ? current : 1;
  historyPage.pageSize = pageSize;
  loadHistory();
}
function closeHistory() {
  historicalSequence++;
  historyDrawer.value = false;
  historical.value = null;
}
async function openHistory(id) {
  const request = ++historicalSequence;
  historyDrawer.value = true;
  historical.value = null;
  historicalError.value = false;
  loadingHistorical.value = true;
  try {
    const data = await getReportRun(props.wsId, kind.value, id);
    if (request === historicalSequence) historical.value = data;
  } catch {
    if (request === historicalSequence) historicalError.value = true;
  } finally {
    if (request === historicalSequence) loadingHistorical.value = false;
  }
}
function showDetails(run, group) {
  details.value = {
    id: run.report.id,
    snapshotVersion: run.report.snapshot_version,
    previewTimezone: run.report.id ? undefined : run.report.timezone,
    group,
  };
}
async function download(run) {
  if (!run || exporting.value) return;
  const seq = kindSequence;
  exporting.value = true;
  exportingRecord.value = run.report.id;
  try {
    const { data, headers } = await exportReport(
      props.wsId,
      kind.value,
      run.report.id,
      run.report.timezone,
    );
    if (seq !== kindSequence) return;
    const encoded = headers["content-disposition"]?.match(
      /filename\*=UTF-8''([^;]+)/i,
    )?.[1];
    const url = URL.createObjectURL(data);
    const link = document.createElement("a");
    link.href = url;
    link.download = encoded
      ? decodeURIComponent(encoded)
      : `${t(viewLabels[kind.value])}.xlsx`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  } catch {
    // The shared request interceptor displays download errors.
  } finally {
    if (seq === kindSequence) exporting.value = false;
  }
}

watch(
  availableTypes,
  (types) => {
    for (const type of Object.keys(views)) {
      if (!types.includes(type)) {
        views[type] = newReportView(type);
        pending.delete(type);
      }
    }
    if (!types.includes(kind.value)) kind.value = types[0] || "";
  },
  { immediate: true },
);

watch(
  () => (kind.value === "work-hours" ? filters.value?.dimensions[0] : null),
  (type, previous) => {
    if (!type || !previous || type === previous) return;
    filters.value.statuses = [];
    options.statuses = [];
    options.priorities = [];
    optionsReady.value = false;
    loadOptions();
  },
);

watch(
  kind,
  () => {
    kindSequence++;
    optionSequence++;
    historySequence++;
    clearTimeout(memberTimer);
    closeHistory();
    details.value = null;
    historyOpen.value = false;
    history.value = [];
    generating.value = false;
    exporting.value = false;
    generateError.value = "";
    optionsReady.value = false;
    memberSearch.value = "";
    currentMemberChoice.clear();
    memberChoices.value = [];
    options.dimensions = [];
    options.statuses = [];
    options.priorities = [];
    options.roles = [];
    loadOptions();
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  disposed = true;
  kindSequence++;
  optionSequence++;
  historySequence++;
  historicalSequence++;
  clearTimeout(memberTimer);
});
</script>

<style scoped lang="less">
.report-table-link {
  padding-inline: 0;
}
.report-empty-prompt {
  display: flex;
  min-height: 126px;
  align-items: center;
  justify-content: center;
  color: var(--app-text-secondary, #8c8c8c);
}
.history-back {
  display: inline-flex;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  padding: 0;
  border-radius: 8px;
}
:deep(.ant-table-pagination.ant-pagination) {
  margin: 12px 0 0;
}
.report-panel {
  display: grid;
  grid-template-columns: clamp(145px, 14vw, 245px) minmax(0, 1fr);
  min-height: 0;
  flex: 1;
  overflow: auto;
}
.report-nav {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  padding: 20px 12px;
  border-inline-end: 1px solid var(--border-color, #e5e7eb);
  .ant-btn {
    text-align: start;
  }
}
.report-content {
  min-width: 0;
  padding: 22px 24px;
}
.report-filters,
.report-empty {
  margin-bottom: 18px;
  padding: 16px;
  border: 1px solid var(--border-color, #e5e7eb);
  border-radius: 10px;
}
.filter-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
  label {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .ant-select {
    min-width: 145px;
  }
}
.report-actions,
.history-heading {
  display: flex;
  align-items: center;
  gap: 12px;
}
.report-actions {
  margin-top: 12px;
}
.report-secondary-actions {
  margin-inline-start: auto;
}
.report-period {
  margin: 12px 0;
}
.history-heading {
  margin-bottom: 18px;
}
@media (max-width: 800px) {
  .report-panel {
    display: block;
  }
  .report-nav {
    flex-direction: row;
    align-items: center;
    border-inline-end: 0;
  }
  .report-content {
    padding: 12px;
  }
}
</style>
