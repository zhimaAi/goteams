<!-- TimeControlCard configures requirement aging (SLA) thresholds per status. -->
<template>
  <a-card size="small" class="settings-card">
    <template #title>{{ $t("settings_aging_control") }}</template>
    <template #extra>
      <a-space :size="8">
        <a-popconfirm
          v-if="canManage"
          :title="$t('aging_reset_confirm')"
          :ok-text="$t('common_confirm')"
          :cancel-text="$t('common_cancel')"
          @confirm="resetForm"
        >
          <a-button type="link" size="small">{{
            $t("settings_restoreDefault")
          }}</a-button>
        </a-popconfirm>
        <a-switch
          :checked="form.master_enabled"
          :disabled="!canManage"
          @change="toggleMaster"
        />
      </a-space>
    </template>

    <template v-if="form.master_enabled">
      <div class="aging-head">
        <span class="col-setting">{{ $t("aging_setting") }}</span>
        <span class="col-switch">{{ $t("aging_switch") }}</span>
        <span class="col-num col-hint">
          {{ $t("aging_timeout_minutes") }}
          <a-tooltip :title="$t('aging_timeout_hint')">
            <QuestionCircleOutlined />
          </a-tooltip>
        </span>
        <span class="col-switch">{{ $t("aging_notify") }}</span>
        <span class="col-num col-hint">
          {{ $t("aging_warn_minutes") }}
          <a-tooltip :title="$t('aging_warn_hint')">
            <QuestionCircleOutlined />
          </a-tooltip>
        </span>
      </div>

      <!-- Total duration row -->
      <div
        class="aging-row"
        :class="{ disabled: !form.total_duration.enabled }"
      >
        <span class="col-setting setting-total">{{ $t("aging_total") }}</span>
        <span class="col-switch">
          <a-switch
            :checked="form.total_duration.enabled === 1"
            :disabled="!canManage"
            size="small"
            @change="(v) => toggleFlag(form.total_duration, 'enabled', v)"
          />
        </span>
        <span class="col-num">
          <a-input-number
            v-model:value="form.total_duration.timeout_minutes"
            :min="1"
            :max="43200"
            :disabled="!canManage || form.total_duration.enabled !== 1"
            size="small"
            @change="save"
          />
        </span>
        <span class="col-switch">
          <a-switch
            :checked="form.total_duration.notify_enabled === 1"
            :disabled="!canManage || form.total_duration.enabled !== 1"
            size="small"
            @change="
              (v) => toggleFlag(form.total_duration, 'notify_enabled', v)
            "
          />
        </span>
        <span class="col-num">
          <a-input-number
            v-model:value="form.total_duration.warn_minutes"
            :min="1"
            :max="43200"
            :placeholder="$t('aging_warn_placeholder')"
            :disabled="!canManage || form.total_duration.enabled !== 1"
            size="small"
            @change="save"
          />
        </span>
      </div>

      <!-- Per-status rows -->
      <div
        v-for="(rule, idx) in form.status_rules"
        :key="rule.status_id"
        class="aging-row"
        :class="{ disabled: rule.enabled !== 1 }"
      >
        <span class="col-setting">
          <span
            class="enum-tag"
            :style="{
              color: rule.status_color,
              borderColor: rule.status_color,
            }"
            >{{ displayStatusName(rule.status_name) }}</span
          >
        </span>
        <span class="col-switch">
          <a-switch
            :checked="rule.enabled === 1"
            :disabled="!canManage"
            size="small"
            @change="(v) => toggleFlag(rule, 'enabled', v)"
          />
        </span>
        <span class="col-num">
          <a-input-number
            v-model:value="rule.timeout_minutes"
            :min="1"
            :max="43200"
            :disabled="!canManage || rule.enabled !== 1"
            size="small"
            @change="save"
          />
        </span>
        <span class="col-switch">
          <a-switch
            :checked="rule.notify_enabled === 1"
            :disabled="!canManage || rule.enabled !== 1"
            size="small"
            @change="(v) => toggleFlag(rule, 'notify_enabled', v)"
          />
        </span>
        <span class="col-num">
          <a-input-number
            v-model:value="rule.warn_minutes"
            :min="1"
            :max="43200"
            :placeholder="$t('aging_warn_placeholder')"
            :disabled="!canManage || rule.enabled !== 1"
            size="small"
            @change="save"
          />
        </span>
      </div>
    </template>
  </a-card>
</template>

<script setup>
import { reactive, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { QuestionCircleOutlined } from "@ant-design/icons-vue";
import {
  getWorkspaceSettings,
  listStatuses,
  updateWorkspaceSettings,
} from "@/api/workspace";
import { DEFAULT_AGING_CONTROL_FORM } from "@/constants/statusTransition";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
  canManage: { type: Boolean, default: false },
});

const { t: $t } = useI18n();
const statuses = ref([]);
const editableStatuses = ref([]);
const form = reactive({
  master_enabled: false,
  total_duration: {
    enabled: 0,
    timeout_minutes: DEFAULT_AGING_CONTROL_FORM.total_duration.timeout_minutes,
    notify_enabled: 0,
    warn_minutes: null,
  },
  status_rules: [],
});
const saving = ref(false);

function displayStatusName(name) {
  return name?.startsWith("workitem.")
    ? $t(name.replaceAll(".", "_"))
    : name || "";
}

// 开关统一走这里：写成 `(x = v ? 1 : 0) || save()` 会在置为 1（真值）时短路，
// 导致「关闭 → 开启」不触发保存。
function toggleFlag(target, key, v) {
  target[key] = v ? 1 : 0;
  save();
}

function reconcileRule(saved, base) {
  const out = { ...base };
  if (saved && typeof saved === "object") {
    out.enabled = saved.enabled === 1 ? 1 : 0;
    out.notify_enabled = saved.notify_enabled === 1 ? 1 : 0;
    let minutes = Number(saved.timeout_minutes);
    if (!minutes || minutes < 1) minutes = base.timeout_minutes;
    if (minutes > 43200) minutes = 43200;
    out.timeout_minutes = minutes;
    out.warn_minutes =
      saved.warn_minutes === null || saved.warn_minutes === undefined
        ? null
        : Number(saved.warn_minutes);
  }
  return out;
}

async function load() {
  try {
    const [settings, statusList] = await Promise.all([
      getWorkspaceSettings(props.wsId),
      listStatuses(props.wsId, props.typeKey),
    ]);
    statuses.value = statusList || [];
    const finalIds = new Set(
      ((settings && settings[`${props.typeKey}_final_status_ids`]) || []).map(
        Number,
      ),
    );
    editableStatuses.value = statuses.value.filter(
      (s) => !finalIds.has(Number(s.id)),
    );
    const saved = (settings && settings.requirement_aging_control) || {};
    form.master_enabled = !!saved.master_enabled;
    form.total_duration = reconcileRule(saved.total_duration, {
      enabled: 0,
      timeout_minutes:
        DEFAULT_AGING_CONTROL_FORM.total_duration.timeout_minutes,
      notify_enabled: 0,
      warn_minutes: null,
    });
    const savedMap = new Map(
      (saved.status_rules || []).map((r) => [Number(r.status_id), r]),
    );
    form.status_rules = editableStatuses.value.map((s) => {
      const rule = reconcileRule(savedMap.get(Number(s.id)), {
        enabled: 0,
        timeout_minutes:
          DEFAULT_AGING_CONTROL_FORM.default_status_timeout_minutes,
        notify_enabled: 0,
        warn_minutes: null,
      });
      return {
        status_id: Number(s.id),
        status_name: s.name,
        status_color: s.color,
        ...rule,
      };
    });
  } catch {
    // Network errors are handled centrally by the interceptor.
  }
}

function buildPayload() {
  return {
    requirement_aging_control: {
      master_enabled: form.master_enabled,
      total_duration: {
        enabled: form.total_duration.enabled ? 1 : 0,
        timeout_minutes:
          Number(form.total_duration.timeout_minutes) ||
          DEFAULT_AGING_CONTROL_FORM.total_duration.timeout_minutes,
        notify_enabled: form.total_duration.notify_enabled ? 1 : 0,
        warn_minutes:
          form.total_duration.warn_minutes === null ||
          form.total_duration.warn_minutes === undefined
            ? null
            : Number(form.total_duration.warn_minutes),
      },
      status_rules: form.status_rules.map((r) => ({
        status_id: Number(r.status_id),
        status_name: r.status_name,
        status_color: r.status_color,
        enabled: r.enabled ? 1 : 0,
        timeout_minutes:
          Number(r.timeout_minutes) ||
          DEFAULT_AGING_CONTROL_FORM.default_status_timeout_minutes,
        notify_enabled: r.notify_enabled ? 1 : 0,
        warn_minutes:
          r.warn_minutes === null || r.warn_minutes === undefined
            ? null
            : Number(r.warn_minutes),
      })),
    },
  };
}

async function save() {
  if (!props.canManage || saving.value) return;
  saving.value = true;
  try {
    await updateWorkspaceSettings(props.wsId, buildPayload());
    message.success($t("common_saved"));
  } catch {
    // Network errors are handled centrally by the interceptor.
  } finally {
    saving.value = false;
  }
}

async function toggleMaster(checked) {
  form.master_enabled = !!checked;
  await save();
}

async function resetForm() {
  if (!props.canManage) return;
  form.master_enabled = false;
  form.total_duration = {
    enabled: 0,
    timeout_minutes: DEFAULT_AGING_CONTROL_FORM.total_duration.timeout_minutes,
    notify_enabled: 0,
    warn_minutes: null,
  };
  form.status_rules = editableStatuses.value.map((s) => ({
    status_id: Number(s.id),
    status_name: s.name,
    status_color: s.color,
    enabled: 0,
    timeout_minutes: DEFAULT_AGING_CONTROL_FORM.default_status_timeout_minutes,
    notify_enabled: 0,
    warn_minutes: null,
  }));
  await save();
}

watch(() => [props.wsId, props.typeKey], load, { immediate: true });
</script>

<style scoped>
.aging-head,
.aging-row {
  display: grid;
  /* The header carries a hint icon after each minutes column title, so those columns are
     wider than the plain number inputs they hold. */
  grid-template-columns: 1.4fr 96px 136px 96px 136px;
  align-items: center;
  min-height: 46px;
  padding: 0 4px;
  border-bottom: 1px solid #eef1f6;
  color: rgba(0, 0, 0, 0.65);
}
.aging-head {
  color: rgba(0, 0, 0, 0.45);
  font-size: 13px;
}
.col-switch,
.col-num {
  text-align: center;
}
/* Title and hint icon stay on one line; flex centering replaces the inherited text-align. */
.col-hint {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  white-space: nowrap;
}
.aging-row.disabled {
  opacity: 0.55;
}
.setting-total {
  font-weight: 600;
}
.enum-tag {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 10px;
  border-radius: 12px;
  font-size: 12px;
  line-height: 22px;
  white-space: nowrap;
  background: transparent;
  border: 1px solid;
  font-weight: 400;
}
</style>
