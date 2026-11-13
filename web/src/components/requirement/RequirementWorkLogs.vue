<template>
  <section class="work-logs">
    <div v-if="showList" class="work-log-toolbar">
      <a-button
        style="padding-left: 0"
        v-if="canEdit"
        type="link"
        @click="openCreate"
      >
        <PlusOutlined /> {{ t("requirement_addExpense") }}
      </a-button>
    </div>

    <a-spin v-if="showList" :spinning="loading">
      <a-empty
        v-if="!loading && !items.length"
        :image="simpleImage"
        :description="t('requirement_noExpenses')"
      />
      <div v-else-if="items.length" class="work-log-table">
        <div class="work-log-row work-log-head">
          <span>{{ t("requirement_expenseDate") }}</span>
          <span>{{ t("requirement_expenseHours") }}</span>
          <span>{{ t("requirement_expenseCreator") }}</span>
          <span>{{ t("workspace_action") }}</span>
        </div>
        <div v-for="item in items" :key="item.id" class="work-log-row">
          <span>{{ formatLocalDate(item.work_date) }}</span>
          <span>{{ formatHours(item.hours) }}</span>
          <span>{{ item.creator_name || "-" }}</span>
          <a-space v-if="canEdit" :size="2">
            <a-button type="link" size="small" @click="openEdit(item)">
              <EditOutlined />
            </a-button>
            <a-popconfirm
              :title="t('requirement_deleteExpenseConfirm')"
              @confirm="remove(item)"
            >
              <a-button type="link" size="small" danger>
                <DeleteOutlined />
              </a-button>
            </a-popconfirm>
          </a-space>
          <span v-else>-</span>
        </div>
      </div>
    </a-spin>

    <a-modal
      v-model:open="editorOpen"
      :title="
        editingId ? t('requirement_editExpense') : t('requirement_addExpense')
      "
      :ok-text="t('common_save')"
      :cancel-text="t('common_cancel')"
      :confirm-loading="saving"
      width="480px"
      :destroy-on-close="true"
      @ok="save"
    >
      <div class="work-hours-summary">
        <div class="work-hours-summary-item">
          <span>{{ t("requirement_estimatedWorkHours") }}</span>
          <strong>{{ formatHours(workSummary.estimated_work_hours) }}</strong>
        </div>
        <div class="work-hours-summary-item">
          <span>{{ t("requirement_completedWorkHours") }}</span>
          <strong>{{
            formatHoursValue(workSummary.completed_work_hours)
          }}</strong>
        </div>
        <div class="work-hours-summary-item">
          <span>{{ t("requirement_remainingWorkHours") }}</span>
          <strong>{{
            formatHoursValue(workSummary.remaining_work_hours)
          }}</strong>
        </div>
        <div class="work-hours-summary-item">
          <span>{{ t("requirement_overtimeWorkHours") }}</span>
          <strong>{{
            formatHoursValue(workSummary.overtime_work_hours)
          }}</strong>
        </div>
        <div class="work-hours-summary-item work-progress-summary" v-if="false">
          <span>{{ t("requirement_workProgress") }}</span>
          <div class="work-progress-value">
            <a-progress
              :percent="Number(workSummary.work_progress) || 0"
              :show-info="false"
              size="small"
            />
            <strong>{{ Number(workSummary.work_progress) || 0 }}%</strong>
          </div>
        </div>
      </div>
      <a-form layout="vertical" class="work-log-form">
        <a-form-item :label="t('requirement_expenseDate')" required>
          <a-date-picker
            v-model:value="workDate"
            style="width: 100%"
            :allow-clear="true"
            @change="handleDateChange"
          />
          <a-alert
            v-if="selectedDateItem"
            class="duplicate-date-tip"
            type="info"
            show-icon
            :message="t('requirement_expenseDateExists')"
          />
        </a-form-item>
        <a-form-item :label="t('requirement_expenseHours')" required>
          <a-input-number
            v-model:value="hours"
            :min="0"
            :max="99999999.99"
            :precision="2"
            style="width: 100%"
          >
            <template #addonAfter>{{
              t("requirement_workHoursSuffix")
            }}</template>
          </a-input-number>
        </a-form-item>
        <a-form-item :label="t('requirement_expenseDescription')">
          <a-textarea
            v-model:value="description"
            :rows="3"
            :placeholder="t('requirement_expenseDescriptionPlaceholder')"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </section>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import dayjs from "dayjs";
import { Empty, message } from "ant-design-vue";
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { formatLocalDate } from "@/utils/date";
import {
  createRequirementWorkLog,
  deleteRequirementWorkLog,
  listRequirementWorkLogs,
  updateRequirementWorkLog,
} from "@/api/workspace";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  requirementId: { type: [String, Number], required: true },
  canEdit: { type: Boolean, default: false },
  showList: { type: Boolean, default: true },
});
const emit = defineEmits(["summary-change"]);
const { t } = useI18n();
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
const items = ref([]);
const loading = ref(false);
const saving = ref(false);
const editorOpen = ref(false);
const editingId = ref(0);
const workDate = ref(null);
const hours = ref(null);
const description = ref("");
const editingOriginalDateKey = ref("");
const workSummary = ref({
  estimated_work_hours: 0,
  completed_work_hours: 0,
  remaining_work_hours: 0,
  overtime_work_hours: 0,
  work_progress: 0,
});

const selectedDateKey = computed(() =>
  workDate.value ? workDate.value.format("YYYY-MM-DD") : "",
);
const selectedDateItem = computed(() =>
  items.value.find(
    (item) => formatLocalDate(item.work_date) === selectedDateKey.value,
  ),
);
const duplicateDateItem = computed(() =>
  items.value.find(
    (item) =>
      Number(item.id) !== Number(editingId.value) &&
      formatLocalDate(item.work_date) === selectedDateKey.value,
  ),
);

function formatHours(value) {
  const numeric = Number(value) || 0;
  const text = numeric
    .toFixed(2)
    .replace(/\.00$/, "")
    .replace(/(\.\d)0$/, "$1");
  return `${text}${t("requirement_workHoursUnit")}`;
}

function formatHoursValue(value) {
  const numeric = Number(value) || 0;
  return numeric
    .toFixed(2)
    .replace(/\.00$/, "")
    .replace(/(\.\d)0$/, "$1");
}

function notifySummary(summary) {
  workSummary.value = { ...workSummary.value, ...(summary || {}) };
  emit("summary-change", { ...summary, work_log_count: items.value.length });
}

async function load() {
  if (!props.wsId || !props.requirementId) return;
  loading.value = true;
  try {
    const result = await listRequirementWorkLogs(
      props.wsId,
      props.requirementId,
    );
    items.value = result?.list || [];
    notifySummary(result?.summary || {});
  } finally {
    loading.value = false;
  }
}

async function openCreate() {
  if (!props.canEdit) return;
  if (loading.value) await load();
  const today = dayjs().startOf("day");
  const todayKey = today.format("YYYY-MM-DD");
  const todayItem = items.value.find(
    (item) => formatLocalDate(item.work_date) === todayKey,
  );
  if (todayItem) {
    openEdit(todayItem);
    return;
  }
  editingId.value = 0;
  editingOriginalDateKey.value = "";
  workDate.value = today;
  hours.value = null;
  description.value = "";
  editorOpen.value = true;
}

function clearDateForm() {
  editingId.value = 0;
  editingOriginalDateKey.value = "";
  hours.value = null;
  description.value = "";
}

function handleDateChange(value) {
  workDate.value = value ? value.startOf("day") : null;
  if (!workDate.value) {
    clearDateForm();
    return;
  }
  const item = selectedDateItem.value;
  if (item) {
    openEdit(item);
    return;
  }
  clearDateForm();
}

function openEdit(item) {
  if (!props.canEdit) return;
  editingId.value = Number(item.id);
  workDate.value = dayjs.unix(Number(item.work_date)).startOf("day");
  editingOriginalDateKey.value = formatLocalDate(item.work_date);
  hours.value = Number(item.hours) || 0;
  description.value = item.description || "";
  editorOpen.value = true;
}

async function save() {
  if (!workDate.value) {
    message.warning(t("requirement_expenseDateRequired"));
    return;
  }
  if (hours.value === null || hours.value === undefined || hours.value === "") {
    message.warning(t("requirement_expenseHoursRequired"));
    return;
  }
  const numericHours = Number(hours.value);
  if (
    !Number.isFinite(numericHours) ||
    numericHours < 0 ||
    numericHours > 99999999.99 ||
    Math.abs(numericHours * 100 - Math.round(numericHours * 100)) > 0.000001
  ) {
    message.warning(t("requirement_workHoursInvalid"));
    return;
  }
  const duplicateIsBlocked =
    duplicateDateItem.value &&
    (!editingId.value ||
      selectedDateKey.value !== editingOriginalDateKey.value);
  if (duplicateIsBlocked) {
    message.warning(t("requirement_expenseDateExists"));
    return;
  }
  saving.value = true;
  try {
    const payload = {
      work_date: workDate.value.startOf("day").unix(),
      hours: numericHours,
      description: description.value,
    };
    const result = editingId.value
      ? await updateRequirementWorkLog(
          props.wsId,
          props.requirementId,
          editingId.value,
          payload,
        )
      : await createRequirementWorkLog(
          props.wsId,
          props.requirementId,
          payload,
        );
    if (result?.summary) notifySummary(result.summary);
    message.success(
      t(
        editingId.value
          ? "requirement_expenseUpdated"
          : "requirement_expenseAdded",
      ),
    );
    editorOpen.value = false;
    await load();
  } finally {
    saving.value = false;
  }
}

async function remove(item) {
  const summary = await deleteRequirementWorkLog(
    props.wsId,
    props.requirementId,
    item.id,
  );
  items.value = items.value.filter(
    (entry) => Number(entry.id) !== Number(item.id),
  );
  notifySummary(summary || {});
  message.success(t("requirement_expenseDeleted"));
}

watch(
  () => [props.wsId, props.requirementId],
  () => void load(),
  { immediate: true },
);

defineExpose({ openCreate, reload: load });
</script>

<style scoped lang="less">
.work-logs {
  padding-left: 16px;
}
.work-log-toolbar {
  min-height: 36px;
  margin-bottom: 8px;
}
.work-log-table {
  border-top: 1px solid #edf0f5;
  overflow-x: auto;
}
.work-log-row {
  display: grid;
  grid-template-columns: 1.1fr 0.8fr 1fr 72px;
  align-items: center;
  min-width: 460px;
  min-height: 46px;
  border-bottom: 1px solid #edf0f5;
  color: var(--app-text);
  font-size: 13px;
}
.work-log-head {
  min-height: 38px;
  background: var(--app-surface-muted, #fafafa);
  font-weight: 600;
}
.work-log-description {
  grid-column: 1 / -1;
  margin: -4px 0 10px;
  color: var(--app-text-secondary);
  white-space: pre-wrap;
}
.work-log-form {
  margin-top: 12px;
}
.duplicate-date-tip {
  margin-top: 8px;
}
.work-hours-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 28px;
  padding: 16px 20px;
  margin-bottom: 16px;
  background: var(--app-surface-muted, #fafafa);
  border-radius: 8px;
}
.work-hours-summary-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  color: var(--app-text-secondary);
  font-size: 13px;
}
.work-hours-summary-item strong {
  color: var(--app-text);
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
}
.work-progress-summary {
  grid-column: 1 / -1;
}
.work-progress-value {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  gap: 12px;
}
.work-progress-value :deep(.ant-progress) {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  margin: 0;
  height: 8px;
  line-height: 1;
}
.work-progress-value :deep(.ant-progress-outer) {
  display: flex;
  align-items: center;
  width: 100%;
}
.work-progress-value :deep(.ant-progress-inner) {
  display: block;
}
</style>
