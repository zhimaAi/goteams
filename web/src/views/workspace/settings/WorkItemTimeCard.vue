<template>
  <a-card
    v-if="fields.length"
    size="small"
    class="settings-card work-item-time-card"
  >
    <template #title>
      <span class="time-card-title">
        {{ $t("workitem_time_settings") }}
        <a-tooltip :title="$t('workitem_time_hint')">
          <QuestionCircleOutlined
            class="time-help"
            tabindex="0"
            :aria-label="$t('workitem_time_hint')"
          />
        </a-tooltip>
      </span>
    </template>
    <template #extra>
      <a-switch
        :checked="form.enabled"
        :loading="saving"
        :disabled="!canManage || !ready || loading || saving"
        :aria-label="$t('workitem_time_record')"
        @change="(enabled) => save({ ...form, enabled })"
      />
    </template>
    <a-spin :spinning="loading">
      <div class="time-setting-rows">
        <div v-for="field in fields" :key="field.key" class="time-setting-row">
          <span class="time-field-label">{{ field.label }}</span>
          <div class="time-setting-rule">
            <span>{{
              $t(
                field.key === "started_at"
                  ? "workitem_time_first_change"
                  : "workitem_time_last_change",
              )
            }}</span>
            <a-select
              mode="multiple"
              max-tag-count="responsive"
              allow-clear
              :value="form[field.key]"
              :options="statusOptions"
              :disabled="!canManage || !ready || loading || saving"
              :aria-label="field.label"
              :placeholder="$t('workitem_time_select_status')"
              class="time-status-select"
              @change="(value) => save({ ...form, [field.key]: value })"
            />
            <span>{{ $t("workitem_time_of_change") }}</span>
          </div>
        </div>
      </div>
    </a-spin>
  </a-card>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { QuestionCircleOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  getWorkspaceSettings,
  listFieldConfigs,
  updateWorkspaceSettings,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  statuses: { type: Array, default: () => [] },
  canManage: Boolean,
  reloadKey: Number,
});
const { t: $t } = useI18n();
const auth = useAuthStore();
const loading = ref(false);
const saving = ref(false);
const ready = ref(false);
const fields = ref([]);
const form = ref({
  enabled: false,
  started_at: [],
  ended_at: [],
});
const statusOptions = computed(() =>
  props.statuses.map((status) => ({
    value: Number(status.id),
    label: status.name,
  })),
);
let revision = 0;
watch(
  () => [props.wsId, props.reloadKey],
  async () => {
    const current = ++revision;
    fields.value = [];
    ready.value = false;
    saving.value = false;
    form.value = { enabled: false, started_at: [], ended_at: [] };
    loading.value = true;
    try {
      const [settings, configs] = await Promise.all([
        getWorkspaceSettings(props.wsId),
        listFieldConfigs(props.wsId, "requirement"),
      ]);
      if (current !== revision) return;
      fields.value = (configs || []).filter(
        (field) =>
          ["started_at", "ended_at"].includes(field.key) &&
          Number(field.enabled) === 1,
      );
      if (
        !Array.isArray(settings.requirement_start_status_ids) ||
        !Array.isArray(settings.requirement_end_status_ids)
      ) {
        message.warning($t("workitem_time_upgrade_required"));
        return;
      }
      ready.value = true;
      form.value = {
        enabled: !!settings.requirement_time_enabled,
        started_at: (settings.requirement_start_status_ids || []).map(Number),
        ended_at: (settings.requirement_end_status_ids || []).map(Number),
      };
    } catch {
      // Request errors are handled by the shared interceptor.
    } finally {
      if (current === revision) loading.value = false;
    }
  },
  { immediate: true },
);

async function save(next) {
  if (!props.canManage || !ready.value || loading.value || saving.value) return;
  if (
    next.enabled &&
    fields.value.some(
      (field) =>
        !next[field.key].length ||
        next[field.key].some(
          (id) => !statusOptions.value.some((status) => status.value === id),
        ),
    )
  ) {
    message.warning($t("workitem_time_select_status"));
    return;
  }
  const wsId = props.wsId;
  const current = revision;
  const previous = { ...form.value };
  form.value = next;
  saving.value = true;
  try {
    await updateWorkspaceSettings(wsId, {
      requirement_time_enabled: next.enabled,
      requirement_start_status_ids: next.started_at,
      requirement_end_status_ids: next.ended_at,
    });
    if (current === revision) {
      auth.clearProjectMetadata();
      message.success($t("common_saveSuccess"));
    }
  } catch {
    // Restore the last persisted values if this workspace is still active.
    if (current === revision) form.value = previous;
  } finally {
    if (current === revision) saving.value = false;
  }
}
</script>

<style scoped lang="less">
.time-card-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.time-help {
  color: inherit;
  opacity: 0.55;
  font-size: 14px;
  cursor: help;
}
.work-item-time-card.ant-card.ant-card-small :deep(.ant-card-head) {
  background: inherit;
  border-bottom: 0;
  padding: 0 22px;
}
.work-item-time-card.ant-card.ant-card-small :deep(.ant-card-body) {
  padding: 8px 22px 22px;
}
.time-setting-rows {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.time-setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  min-height: 38px;
}
.time-field-label {
  flex-shrink: 0;
}
.time-setting-rule {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
}
.time-setting-rule > span {
  white-space: nowrap;
}
.time-status-select {
  width: 214px;
}
@media (max-width: 680px) {
  .time-setting-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 8px;
  }
  .time-setting-rule {
    width: 100%;
    flex-wrap: wrap;
    justify-content: flex-start;
    gap: 8px;
  }
  .time-status-select {
    flex: 1;
    min-width: 140px;
    max-width: 214px;
  }
}
</style>
