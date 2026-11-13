<template>
  <div class="sla-detail">
    <a-spin :spinning="loading">
      <a-alert
        v-if="failed"
        type="error"
        :message="$t('sla_loadFailed')"
        show-icon
      >
        <template #action
          ><a-button size="small" @click="load">{{
            $t("issue_retry")
          }}</a-button></template
        >
      </a-alert>
      <template v-else-if="result.enabled">
        <a-typography-title :level="5">{{
          $t("sla_currentStages")
        }}</a-typography-title>
        <a-alert
          v-if="result.configuration_incomplete"
          type="warning"
          :message="$t('sla_configurationIncomplete')"
        />
        <a-empty
          v-else-if="!result.current.length"
          :description="$t('sla_noMatch')"
        />
        <a-table
          v-if="result.current.length"
          class="sla-table"
          :data-source="result.current"
          :columns="currentColumns"
          :pagination="false"
          row-key="stage_id"
          size="middle"
          :scroll="{ x: 640 }"
        >
          <template #bodyCell="{ column, record }">
            <SLAStageResult v-if="column.key === 'stage'" :stage="record" />
            <template v-else-if="column.key === 'time'">
              {{
                record.started_at
                  ? formatLocalDate(record.started_at, true)
                  : "—"
              }}
              <br />
              {{
                record.deadline_at
                  ? formatLocalDate(record.deadline_at, true)
                  : "—"
              }}
            </template>
            <template v-else-if="column.key === 'elapsed'">
              {{
                record.elapsed_seconds === null
                  ? $t("sla_calendarPending")
                  : slaDuration(record.elapsed_seconds, t)
              }}
              / {{ slaDuration(record.limit_seconds, t) }}
            </template>
          </template>
        </a-table>
        <a-divider />
        <a-typography-title :level="5">{{
          $t("sla_history")
        }}</a-typography-title>
        <a-table
          class="sla-table"
          size="middle"
          :data-source="result.history"
          :columns="columns"
          :pagination="{
            current: page,
            pageSize: 20,
            total: result.total,
            showSizeChanger: false,
          }"
          row-key="cycle_id"
          :scroll="{ x: 640 }"
          @change="changePage"
        >
          <template #bodyCell="{ column, record }">
            <SLAStageResult v-if="column.key === 'stage'" :stage="record" />
            <template v-else-if="column.key === 'time'"
              >{{ formatLocalDate(record.started_at, true) }}<br />{{
                record.ended_at ? formatLocalDate(record.ended_at, true) : "—"
              }}</template
            >
            <template v-else-if="column.key === 'elapsed'"
              >{{
                record.elapsed_seconds === null
                  ? $t("sla_calendarPending")
                  : slaDuration(record.elapsed_seconds, t)
              }}
              / {{ slaDuration(record.limit_seconds, t) }}</template
            >
          </template>
        </a-table>
      </template>
    </a-spin>
  </div>
</template>
<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { getRequirementSLA } from "@/api/sla";
import { formatLocalDate } from "@/utils/date";
import { slaDuration } from "@/utils/sla";
import SLAStageResult from "./SLAStageResult.vue";
const props = defineProps({
  wsId: { type: [Number, String], required: true },
  requirementId: { type: [Number, String], required: true },
  reloadKey: { type: Number, default: 0 },
});
const emit = defineEmits(["disabled"]);
const { t } = useI18n();
const loading = ref(false),
  failed = ref(false),
  page = ref(1);
const result = ref({ enabled: false, current: [], history: [], total: 0 });
let sequence = 0;
const columns = computed(() => [
  { title: t("sla_stageResult"), key: "stage" },
  { title: t("sla_startEndTime"), key: "time" },
  { title: t("sla_elapsedLimit"), key: "elapsed" },
]);
const currentColumns = computed(() => [
  { title: t("sla_stageResult"), key: "stage" },
  { title: t("sla_startDeadlineTime"), key: "time" },
  { title: t("sla_elapsedLimit"), key: "elapsed" },
]);
async function load() {
  const current = ++sequence;
  loading.value = true;
  failed.value = false;
  try {
    const data = await getRequirementSLA(props.wsId, props.requirementId, {
      page: page.value,
      size: 20,
    });
    if (current === sequence) {
      result.value = data;
      if (!data.enabled) emit("disabled");
    }
  } catch {
    if (current === sequence) failed.value = true;
  } finally {
    if (current === sequence) loading.value = false;
  }
}
function changePage(pagination) {
  page.value = pagination.current;
  load();
}
watch(
  () => [props.wsId, props.requirementId, props.reloadKey],
  () => {
    page.value = 1;
    result.value = { enabled: false, current: [], history: [], total: 0 };
    load();
  },
  { immediate: true },
);
const timer = setInterval(() => {
  if (!loading.value) load();
}, 60000);
onBeforeUnmount(() => {
  ++sequence;
  clearInterval(timer);
});
</script>
<style scoped lang="less">
.sla-detail {
  min-width: 0;
  padding: 4px 24px 24px;
}
.sla-table {
  :deep(.ant-tag) {
    max-width: 100%;
    margin-inline-end: 0;
    white-space: normal;
    overflow-wrap: anywhere;
  }
}
@media (max-width: 700px) {
  .sla-detail {
    padding-inline: 16px;
  }
}
</style>
