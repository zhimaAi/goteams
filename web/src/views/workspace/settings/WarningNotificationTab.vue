<template>
  <a-spin :spinning="loading">
    <h3 class="warning-title">{{ t("warning_title") }}</h3>
    <a-alert
      class="warning-notice"
      type="info"
      :message="t('warning_ruleHint')"
      show-icon
    />
    <a-alert
      v-if="loadFailed"
      type="error"
      :message="t('warning_loadFailed')"
      show-icon
    />
    <div class="warning-toolbar">
      <a-button v-if="canManage" type="primary" @click="edit()">{{
        t("warning_new")
      }}</a-button>
      <a-input-search
        class="warning-search"
        v-model:value="query.name"
        :placeholder="t('warning_searchName')"
        @search="search"
      />
      <a-input-search
        class="warning-search warning-search-condition"
        v-model:value="query.keyword"
        :placeholder="t('warning_searchCondition')"
        @search="search"
      />
      <a-button @click="load">{{ t("warning_refresh") }}</a-button>
    </div>
    <a-table
      :data-source="items"
      :columns="columns"
      :pagination="{
        current: query.page,
        pageSize: query.size,
        total,
        showSizeChanger: true,
      }"
      row-key="id"
      @change="pageChanged"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'template'">{{
          record.config.template_id === 0
            ? t("warning_defaultTemplate")
            : record.config.template_name
        }}</template>
        <template v-else-if="column.key === 'conditions'">{{
          conditionSummary(record.config)
        }}</template>
        <template v-else-if="column.key === 'threshold'">{{
          t("warning_thresholdSummary", {
            minutes: record.config.window_minutes,
            count: record.config.threshold,
          })
        }}</template>
        <template v-else-if="column.key === 'enabled'"
          ><a-switch
            :checked="record.enabled"
            :disabled="!canManage || busy"
            :aria-label="record.name"
            @change="toggle(record, $event)"
        /></template>
        <a-space
          v-else-if="column.key === 'actions' && canManage"
          :size="4"
          class="warning-actions"
        >
          <a-button type="link" :disabled="busy" @click="edit(record)">{{
            t("warning_edit")
          }}</a-button>
          <a-popconfirm
            :title="t('warning_deleteConfirm')"
            @confirm="remove(record)"
            ><a-button type="link" danger :disabled="busy">{{
              t("warning_delete")
            }}</a-button></a-popconfirm
          >
        </a-space>
      </template>
    </a-table>
    <a-drawer
      v-model:open="open"
      :title="t(form.id ? 'warning_edit' : 'warning_new')"
      width="min(100vw, 1000px)"
      class="warning-drawer"
      :closable="!busy"
      :keyboard="!busy"
      :mask-closable="false"
      :destroy-on-close="true"
    >
      <a-spin :spinning="metadataLoading">
        <a-form
          ref="formElement"
          class="warning-form"
          layout="horizontal"
          label-align="right"
          :label-col="{ class: 'warning-form-label' }"
          :wrapper-col="{ flex: 1 }"
          @finish="save"
        >
          <a-form-item
            :label="t('warning_name')"
            v-bind="validationProps('name')"
            required
            ><a-input
              class="warning-name"
              v-model:value="form.name"
              :maxlength="20"
              :placeholder="t('warning_nameHint')"
              show-count
          /></a-form-item>
          <a-form-item
            :label="t('warning_template')"
            v-bind="validationProps('template')"
            required
            ><a-select
              :value="form.config.template_id"
              class="warning-select"
              :placeholder="t('warning_template')"
              :options="
                templates.map((x) => ({
                  value: x.id,
                  label: x.name,
                  isInvite: x.isInvite,
                }))
              "
              @change="templateChanged"
            >
              <template #option="option">
                <a-tag :color="option.isInvite ? 'purple' : 'blue'">{{
                  t(
                    option.isInvite
                      ? "work_item_template_source_invite"
                      : "work_item_template_source_internal",
                  )
                }}</a-tag
                >{{ option.label }}
              </template>
            </a-select></a-form-item
          >
          <a-form-item :label="t('warning_relation')" required
            ><a-radio-group v-model:value="form.config.relation"
              ><a-radio value="AND">{{ t("warning_all") }}</a-radio
              ><a-radio value="OR">{{
                t("warning_any")
              }}</a-radio></a-radio-group
            ></a-form-item
          >
          <a-form-item
            :label="t('warning_conditions')"
            v-bind="validationProps('conditions')"
            required
          >
            <div
              v-for="(condition, index) in form.config.conditions"
              :key="index"
              class="warning-condition"
            >
              <a-select
                v-model:value="condition.field_key"
                :aria-label="t('warning_field')"
                :placeholder="t('warning_field')"
                :options="fields.map((f) => ({ value: f.key, label: f.label }))"
                @change="resetCondition(condition)"
              />
              <a-select
                v-model:value="condition.operator"
                :aria-label="t('warning_operator')"
                :options="operators(field(condition))"
                @change="condition.value = undefined"
              />
              <template
                v-if="!['empty', 'not_empty'].includes(condition.operator)"
              >
                <a-select
                  v-if="isChoice(field(condition))"
                  v-model:value="condition.value"
                  :aria-label="t('warning_value')"
                  :mode="isMulti(condition) ? 'multiple' : undefined"
                  :options="
                    field(condition).options.map((o) => ({
                      value: o.key,
                      label: o.name,
                    }))
                  "
                />
                <a-input-number
                  v-else-if="
                    ['number', 'numeric'].includes(field(condition).type)
                  "
                  v-model:value="condition.value"
                  :aria-label="t('warning_value')"
                />
                <a-date-picker
                  v-else-if="
                    ['date', 'datetime'].includes(field(condition).type)
                  "
                  :value="dateValue(condition)"
                  :show-time="field(condition).type === 'datetime'"
                  :aria-label="t('warning_value')"
                  @change="setDate(condition, $event)"
                />
                <a-input
                  v-else
                  v-model:value="condition.value"
                  :aria-label="t('warning_value')"
                />
              </template>
              <a-button
                danger
                :aria-label="t('warning_removeCondition')"
                @click="form.config.conditions.splice(index, 1)"
                >−</a-button
              >
            </div>
            <a-button
              :disabled="
                form.config.template_id == null ||
                metadataLoading ||
                form.config.conditions.length >= 30
              "
              @click="
                form.config.conditions.push({
                  field_key: undefined,
                  operator: undefined,
                  value: undefined,
                })
              "
              >{{ t("warning_addCondition") }}</a-button
            >
          </a-form-item>
          <a-form-item
            :label="t('warning_trigger')"
            v-bind="validationProps('trigger')"
            required
          >
            <div class="warning-sentence">
              <span>{{ t("warning_within") }}</span>
              <a-input-number
                v-model:value="form.config.window_minutes"
                :min="1"
                :max="43200"
                :precision="0"
                :aria-label="t('warning_window')"
              />
              <span>{{ t("warning_countHint") }}</span>
              <a-input-number
                v-model:value="form.config.threshold"
                :min="1"
                :max="1000000"
                :precision="0"
                :aria-label="t('warning_threshold')"
              />
              <span>{{ t("warning_triggerSuffix") }}</span>
            </div>
          </a-form-item>
          <a-form-item
            :label="t('warning_intervalLabel')"
            v-bind="validationProps('interval')"
            required
          >
            <div class="warning-sentence">
              <span>{{ t("warning_every") }}</span>
              <a-input-number
                v-model:value="form.config.interval_minutes"
                :min="1"
                :max="43200"
                :precision="0"
                :aria-label="t('warning_interval')"
              />
              <span>{{ t("warning_intervalHint") }}</span>
            </div>
          </a-form-item>
          <a-form-item
            :label="t('warning_channel')"
            v-bind="validationProps('channels')"
            required
          >
            <a-checkbox-group
              v-model:value="selectedChannels"
              class="warning-channels"
              :aria-label="t('warning_channel')"
            >
              <a-checkbox
                v-for="channel in channels"
                :key="channel.value"
                :value="channel.value"
                >{{ channel.label }}</a-checkbox
              >
            </a-checkbox-group>
            <div class="warning-hint">{{ t("warning_multiHint") }}</div>
          </a-form-item>
          <a-form-item class="warning-configs" :wrapper-col="{ span: 24 }">
            <section
              v-for="channel in activeChannels"
              :key="channel.channel"
              class="warning-channel-panel"
            >
              <h4>
                {{
                  t("warning_channelConfig", {
                    channel: t("warning_channel_" + channel.channel),
                  })
                }}
              </h4>
              <template v-if="channel.channel !== 'internal'">
                <a-form-item
                  :label="t('warning_webhook')"
                  v-bind="validationProps(channel.channel + '_url')"
                  required
                  :label-col="{ span: 24 }"
                  :wrapper-col="{ span: 24 }"
                >
                  <a-input
                    v-model:value="channel.url"
                    :placeholder="t('warning_webhookHint')"
                    autocomplete="off"
                  />
                </a-form-item>
                <a-form-item
                  v-if="channel.channel !== 'wecom'"
                  :label="t('warning_secret')"
                  :label-col="{ span: 24 }"
                  :wrapper-col="{ span: 24 }"
                >
                  <a-input-password
                    v-model:value="channel.secret"
                    autocomplete="new-password"
                  />
                </a-form-item>
              </template>
              <a-alert
                v-else
                :message="t('warning_recipientHint')"
                type="info"
                show-icon
                class="warning-recipient-hint"
              />
              <a-form-item
                :label="t('warning_content')"
                v-bind="validationProps(channel.channel + '_content')"
                required
                :label-col="{ span: 24 }"
                :wrapper-col="{ span: 24 }"
              >
                <a-textarea
                  v-model:value="channel.content"
                  :placeholder="t('warning_contentHint')"
                  :maxlength="1000"
                  :rows="5"
                  show-count
                />
              </a-form-item>
            </section>
          </a-form-item>
        </a-form>
      </a-spin>
      <template #footer>
        <a-space>
          <a-button
            type="primary"
            :loading="busy"
            :disabled="metadataLoading"
            @click="save"
            >{{ t("warning_save") }}</a-button
          >
          <a-button :disabled="busy" @click="open = false">{{
            t("warning_cancel")
          }}</a-button>
        </a-space>
      </template>
    </a-drawer>
  </a-spin>
</template>
<script setup>
import { computed, nextTick, ref, watch } from "vue";
import dayjs from "dayjs";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import { listWorkItemTemplates } from "@/api/workspace";
import { listIssueInviteTemplates } from "@/api/issueInvite";
import { useAuthStore } from "@/stores/auth";
import {
  listWarningRules,
  getWarningMetadata,
  saveWarningRule,
  deleteWarningRule,
  setWarningRuleEnabled,
} from "@/api/warningNotification";
const props = defineProps({ wsId: { type: [String, Number], required: true } });
const { t } = useI18n();
const auth = useAuthStore();
const canManage = computed(() =>
  auth.hasPermission("warning_notification:manage"),
);
const loadFailed = ref(false);
const loading = ref(false),
  busy = ref(false),
  open = ref(false),
  metadataLoading = ref(false),
  items = ref([]),
  listFields = ref({}),
  total = ref(0),
  templates = ref([]),
  fields = ref([]);
const query = ref({ page: 1, size: 20, name: "", keyword: "" });
const form = ref(blank());
const formElement = ref(null);
const submitted = ref(false);
const selectedChannels = ref(["wecom"]);
const channelDrafts = ref({});
const activeChannels = computed(() =>
  channels.value
    .filter(({ value }) => selectedChannels.value.includes(value))
    .map(({ value }) => channelDrafts.value[value]),
);
let loadSeq = 0,
  metaSeq = 0;
function blank() {
  return {
    name: "",
    enabled: true,
    config: {
      type: "requirement",
      template_id: 0,
      relation: "AND",
      conditions: [],
      window_minutes: 1,
      threshold: 1,
      interval_minutes: 1,
      channel: "wecom",
      url: "",
      secret: "",
      content: "",
    },
  };
}
const channels = computed(() =>
  ["feishu", "wecom", "dingtalk", "internal"].map((value) => ({
    value,
    label: t(`warning_channel_${value}`),
  })),
);
const columns = computed(() => [
  { title: t("warning_name"), dataIndex: "name" },
  { title: t("warning_template"), key: "template" },
  { title: t("warning_conditions"), key: "conditions" },
  { title: t("warning_threshold"), key: "threshold" },
  { title: t("warning_enabled"), key: "enabled", width: 88 },
  { title: t("warning_actions"), key: "actions", width: 132 },
]);
function conditionSummary(config) {
  const metadata =
    listFields.value[`${config.type}:${config.template_id}`] || [];
  return config.conditions
    .map((c) => {
      const field = metadata.find((f) => f.key === c.field_key);
      const values = Array.isArray(c.value) ? c.value : [c.value];
      const value = values
        .map(
          (v) =>
            field?.options?.find((o) => String(o.key) === String(v))?.name ??
            v ??
            "",
        )
        .join(t("warning_valueSeparator"));
      return t("warning_conditionSummary", {
        field: field?.label || c.field_label || c.field_key,
        operator: t("warning_op_" + c.operator),
        value,
      }).trim();
    })
    .join(t(config.relation === "AND" ? "warning_and" : "warning_or"));
}
async function load() {
  const seq = ++loadSeq;
  loading.value = true;
  loadFailed.value = false;
  try {
    const data = await listWarningRules(props.wsId, query.value);
    if (seq !== loadSeq) return;
    const configs = new Map(
      data.list.map(({ config }) => [
        `${config.type}:${config.template_id}`,
        config,
      ]),
    );
    const metadata = await Promise.all(
      [...configs].map(async ([key, config]) => {
        const result = await getWarningMetadata(props.wsId, {
          type: config.type,
          template_id: config.template_id,
        });
        config.template_name = result.templates.find(
          (t) => t.id === config.template_id,
        )?.name;
        return [key, result.fields];
      }),
    );
    if (seq !== loadSeq) return;
    listFields.value = Object.fromEntries(metadata);
    items.value = data.list;
    total.value = data.total;
  } catch (error) {
    if (seq === loadSeq) loadFailed.value = true;
  } finally {
    if (seq === loadSeq) loading.value = false;
  }
}
function search() {
  query.value.page = 1;
  load();
}
function pageChanged(p) {
  query.value.page = p.current;
  query.value.size = p.pageSize;
  load();
}
async function metadata() {
  const seq = ++metaSeq;
  metadataLoading.value = true;
  try {
    const data = await getWarningMetadata(props.wsId, {
      type: form.value.config.type,
      template_id: form.value.config.template_id,
    });
    if (seq !== metaSeq) return;

    fields.value = data.fields;
  } finally {
    if (seq === metaSeq) metadataLoading.value = false;
  }
}
async function edit(rule) {
  submitted.value = false;
  form.value = rule ? JSON.parse(JSON.stringify(rule)) : blank();
  const config = form.value.config;
  const existing = config.channels || [
    {
      channel: config.channel,
      url: config.url,
      secret: config.secret,
      content: config.content,
    },
  ];
  selectedChannels.value = existing.map((channel) => channel.channel);
  channelDrafts.value = Object.fromEntries(
    channels.value.map(({ value }) => [
      value,
      {
        channel: value,
        url: "",
        secret: "",
        content: "",
        ...existing.find((channel) => channel.channel === value),
      },
    ]),
  );
  fields.value = [];
  templates.value = [];
  open.value = true;
  const seq = ++metaSeq;
  const workspaceId = props.wsId;
  metadataLoading.value = true;
  try {
    const [internal, invites] = await Promise.all([
      Promise.all(
        ["requirement", "defect"].map(async (type) => {
          const data = await listWorkItemTemplates(workspaceId, type);
          return data.enabled
            ? data.items.map((template) => ({ ...template, type }))
            : [];
        }),
      ),
      listIssueInviteTemplates(workspaceId),
    ]);
    if (seq !== metaSeq) return;
    templates.value = [
      ...internal.flat(),
      ...invites
        .filter((template) => template.enabled === 1)
        .map((template) => ({
          ...template,
          type: template.type_key,
          isInvite: true,
        })),
    ];
    if (!templates.value.length) {
      templates.value = [
        { id: 0, name: t("warning_defaultTemplate"), type: "requirement" },
      ];
    }
    if (!rule) {
      const template = templates.value[0];
      config.template_id = template.id;
      config.type = template.type;
      config.template_name = template.name;
    } else if (
      config.template_id === 0 &&
      !templates.value.some((template) => template.id === 0)
    ) {
      config.template_id = undefined;
      config.conditions = [];
    }
  } finally {
    if (seq === metaSeq) metadataLoading.value = false;
  }
  if (seq !== metaSeq || config.template_id == null) return;
  await metadata();
}
function templateChanged(id) {
  form.value.config.template_id = id;
  form.value.config.type = templates.value.find((x) => x.id === id)?.type;
  form.value.config.template_name = templates.value.find(
    (x) => x.id === form.value.config.template_id,
  )?.name;
  form.value.config.conditions = [];
  fields.value = [];
  metadata();
}
function field(c) {
  return (
    fields.value.find((f) => f.key === c.field_key) || {
      type: "text",
      options: [],
    }
  );
}
function resetCondition(c) {
  c.field_label = field(c).label;
  c.operator = operators(field(c))[0]?.value;
  c.value = undefined;
}
function operators(f) {
  let ops;
  if (["number", "numeric"].includes(f.type))
    ops = ["eq", "neq", "gt", "gte", "lt", "lte"];
  else if (["date", "datetime"].includes(f.type))
    ops = ["is", "is_not", "before", "before_eq", "after", "after_eq"];
  else if (["user_multi", "select_multi"].includes(f.type))
    ops = ["contains_any", "contains_all", "contains_none"];
  else if (["user", "select_single"].includes(f.type))
    ops = ["is", "is_not", "in", "not_in"];
  else ops = ["contains", "not_contains", "is", "is_not"];
  return [...ops, "empty", "not_empty"].map((value) => ({
    value,
    label: t("warning_op_" + value),
  }));
}
function isChoice(f) {
  return ["user", "user_multi", "select_single", "select_multi"].includes(
    f.type,
  );
}
function isMulti(c) {
  return (
    ["user_multi", "select_multi"].includes(field(c).type) ||
    ["in", "not_in"].includes(c.operator)
  );
}
function dateValue(c) {
  if (
    c.value != null &&
    typeof c.value === "number" &&
    c.field_key.startsWith("cf_")
  )
    return dayjs(new Date(c.value * 1000).toISOString().slice(0, 10));
  return c.value == null
    ? null
    : typeof c.value === "number"
      ? dayjs.unix(c.value)
      : dayjs(c.value);
}
function setDate(c, v) {
  c.value = !v
    ? undefined
    : field(c).type === "datetime"
      ? v.format("YYYY-MM-DD HH:mm:ss")
      : c.field_key.startsWith("cf_")
        ? Date.parse(v.format("YYYY-MM-DD")) / 1000
        : v.startOf("day").unix();
}
const validationErrors = computed(() => {
  const errors = {};
  const { name, config } = form.value;
  const required = t("warning_required");
  if (!name.trim()) errors.name = required;
  if (config.template_id == null) errors.template = required;
  if (!config.conditions.length) errors.conditions = required;
  else if (
    config.conditions.some(
      (condition) =>
        !fields.value.some((f) => f.key === condition.field_key) ||
        !condition.operator ||
        (!["empty", "not_empty"].includes(condition.operator) &&
          missingValue(condition.value)),
    )
  )
    errors.conditions = t("warning_conditionsRequired");
  if (!config.window_minutes || !config.threshold) errors.trigger = required;
  if (!config.interval_minutes) errors.interval = required;
  if (!activeChannels.value.length) errors.channels = required;
  for (const channel of activeChannels.value) {
    if (channel.channel !== "internal" && !channel.url.trim())
      errors[channel.channel + "_url"] = required;
    if (!channel.content.trim())
      errors[channel.channel + "_content"] = required;
  }
  return errors;
});
function missingValue(value) {
  return (
    value == null ||
    (typeof value === "string" && !value.trim()) ||
    (Array.isArray(value) && !value.length)
  );
}
function validationProps(key) {
  const error = submitted.value && validationErrors.value[key];
  return {
    validateStatus: error ? "error" : undefined,
    help: error || undefined,
  };
}
async function save() {
  if (busy.value || metadataLoading.value) return;
  const m = form.value,
    c = m.config;
  submitted.value = true;
  if (Object.keys(validationErrors.value).length) {
    message.warning(t("warning_invalid"));
    await nextTick();
    const error = formElement.value?.$el.querySelector(
      ".ant-form-item-has-error",
    );
    error?.scrollIntoView({ behavior: "smooth", block: "center" });
    error?.querySelector("input, textarea, button, [tabindex]")?.focus({
      preventScroll: true,
    });
    return;
  }
  busy.value = true;
  try {
    const { channel, url, secret, content, ...config } = c;
    await saveWarningRule(props.wsId, {
      ...m,
      name: m.name.trim(),
      config: { ...config, channels: activeChannels.value },
    });
    message.success(t("common_saveSuccess"));
    open.value = false;
    await load();
  } finally {
    busy.value = false;
  }
}
async function toggle(rule, value) {
  busy.value = true;
  try {
    await setWarningRuleEnabled(props.wsId, rule.id, value);
    await load();
  } finally {
    busy.value = false;
  }
}
async function remove(rule) {
  busy.value = true;
  try {
    await deleteWarningRule(props.wsId, rule.id);
    await load();
  } finally {
    busy.value = false;
  }
}
watch(
  () => props.wsId,
  () => {
    ++metaSeq;
    open.value = false;
    items.value = [];
    listFields.value = {};
    total.value = 0;
    query.value.page = 1;
    load();
  },
  { immediate: true },
);
</script>
<style scoped lang="less">
.warning-form {
  padding: 12px 0;
}
.warning-form :deep(.warning-form-label) {
  flex: 0 0 140px;
}
.warning-configs {
  margin-left: 140px;
}
.warning-name {
  max-width: 480px;
}
.warning-select {
  width: 260px;
  max-width: 100%;
}
.warning-sentence {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}
.warning-sentence :deep(.ant-input-number) {
  width: 100px;
}
.warning-channels {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.warning-channels :deep(.ant-checkbox-wrapper) {
  margin: 0;
  padding: 12px 16px;
  border: 1px solid var(--ant-color-border, #d9d9d9);
  border-radius: 6px;
}
.warning-channels :deep(.ant-checkbox-wrapper-checked) {
  border-color: var(--ant-color-primary, #1677ff);
  background: var(--ant-color-primary-bg, #e6f4ff);
}
.warning-hint {
  margin-top: 8px;
  color: var(--ant-color-text-secondary, #8c8c8c);
  font-size: 12px;
}
.warning-channel-panel {
  padding: 24px;
  margin-bottom: 16px;
  border-radius: 8px;
  background: var(--ant-color-fill-alter, #f7f8fa);
}
.warning-channel-panel h4 {
  margin: 0 0 20px;
  font-size: 15px;
  font-weight: 600;
}
.warning-recipient-hint {
  margin-bottom: 20px;
}
@media (max-width: 575px) {
  .warning-form {
    padding: 0;
  }
  .warning-configs {
    margin-left: 0;
  }
  .warning-form :deep(.warning-form-label) {
    flex: 0 0 100%;
    text-align: left;
  }
  .warning-channel-panel {
    padding: 16px;
  }
}
.warning-notice {
  margin-bottom: 16px;
}
.warning-title {
  margin: 0 0 16px;
}
.warning-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}
.warning-toolbar > .ant-btn {
  flex-shrink: 0;
}
.warning-search {
  width: 240px;
  max-width: 100%;
}
.warning-search-condition {
  width: 280px;
}
.warning-actions {
  white-space: nowrap;
}
.warning-actions :deep(.ant-btn) {
  padding-inline: 6px;
}
@media (max-width: 575px) {
  .warning-search {
    width: 100%;
  }
}
.warning-condition {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
  align-items: center;
}
.warning-condition :deep(.ant-select),
.warning-condition :deep(.ant-input),
.warning-condition :deep(.ant-input-number),
.warning-condition :deep(.ant-picker) {
  flex: 0 0 240px;
  width: 240px;
  min-width: 0;
  max-width: 100%;
}
</style>
