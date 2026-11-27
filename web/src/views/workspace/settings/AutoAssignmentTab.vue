<template>
  <a-spin :spinning="loading">
    <div class="auto-assignment">
      <h3 class="assignment-title">{{ $t("auto_assignment_title") }}</h3>
      <a-alert
        class="assignment-notice"
        type="info"
        :message="$t('auto_assignment_hint')"
        show-icon
      />
      <div class="toolbar">
        <a-button
          v-if="canManage"
          type="primary"
          :disabled="!ready || saving"
          @click="openRule()"
          >{{ $t("automation_newRule") }}</a-button
        >
        <a-input-search
          v-model:value="searchName"
          :placeholder="$t('auto_assignment_searchName')"
          :aria-label="$t('auto_assignment_searchName')"
          allow-clear
          class="rule-search"
        />
      </div>
      <a-table
        :data-source="filteredRules"
        :columns="columns"
        row-key="id"
        :pagination="pagination"
        :scroll="{ x: 950 }"
        @change="changePage"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'enabled'"
            ><a-switch
              :checked="record.enabled"
              :disabled="!canManage || !ready || saving"
              @change="toggleRule(record.id, $event)"
          /></template>
          <template v-else-if="column.key === 'mode'">{{
            $t(`auto_assignment_${record.mode}`)
          }}</template>
          <template
            v-else-if="['created_at', 'updated_at'].includes(column.key)"
            >{{ formatLocalDate(record[column.key] / 1000, true) }}</template
          >
          <template v-else-if="column.key === 'actions'">
            <a-space v-if="canManage" :size="4" class="assignment-actions">
              <a-button
                type="link"
                :disabled="saving"
                @click="openRule(record)"
                >{{ $t("common_edit") }}</a-button
              >
              <a-popconfirm
                :title="$t('automation_deleteConfirm')"
                :disabled="saving"
                @confirm="deleteRule(record.id)"
                ><a-button type="link" danger :disabled="saving">{{
                  $t("common_delete")
                }}</a-button></a-popconfirm
              >
            </a-space>
          </template>
        </template>
      </a-table>
      <a-drawer
        v-model:open="editorOpen"
        :title="
          $t(
            config.rules.some((rule) => rule.id === draft?.id)
              ? 'automation_editRule'
              : 'automation_newRule',
          )
        "
        :width="'min(100vw, 1000px)'"
        placement="right"
        :closable="!saving"
        :mask-closable="!saving"
        :keyboard="!saving"
      >
        <a-form
          v-if="draft"
          ref="formElement"
          class="assignment-form"
          layout="horizontal"
          label-align="right"
          :label-col="{ class: 'assignment-form-label' }"
          :wrapper-col="{ flex: '1 1 0' }"
          :disabled="saving"
        >
          <a-form-item
            :label="$t('automation_ruleName')"
            v-bind="validationProps(draft.name.trim(), 'automation_ruleName')"
            required
            ><a-input
              v-model:value="draft.name"
              class="assignment-name"
              :maxlength="20"
              show-count
          /></a-form-item>
          <a-form-item :label="$t('automation_conditionsLabel')">
            <a-card
              v-for="(module, mi) in draft.modules"
              :key="mi"
              class="condition-module"
              :title="$t('automation_conditionModule', { index: mi + 1 })"
            >
              <template #extra>
                <a-radio-group v-model:value="module.relation" size="small">
                  <a-radio-button value="AND">AND</a-radio-button>
                  <a-radio-button value="OR">OR</a-radio-button>
                </a-radio-group>
              </template>
              <div
                v-for="(condition, ci) in module.conditions"
                :key="ci"
                class="condition-row"
              >
                <a-form-item
                  class="condition-control"
                  v-bind="
                    validationProps(condition.field_key, 'automation_field')
                  "
                >
                  <a-select
                    v-model:value="condition.field_key"
                    :options="fieldOptions"
                    :placeholder="$t('automation_field')"
                    @change="
                      condition.operator = undefined;
                      condition.value = undefined;
                    "
                  />
                </a-form-item>
                <a-form-item
                  class="condition-control"
                  v-bind="
                    validationProps(condition.operator, 'automation_operator')
                  "
                >
                  <a-select
                    v-model:value="condition.operator"
                    :options="
                      automationOperatorOptions(
                        field(condition.field_key),
                        condition.operator,
                        t,
                      )
                    "
                    :placeholder="$t('automation_operator')"
                    @change="condition.value = undefined"
                  />
                </a-form-item>
                <a-form-item
                  v-if="!['empty', 'not_empty'].includes(condition.operator)"
                  class="condition-control"
                  v-bind="validationProps(condition.value, 'automation_value')"
                >
                  <AutomationConditionValue
                    v-model="condition.value"
                    :show-error="submitted && missingValue(condition.value)"
                    :field="field(condition.field_key)"
                    :operator="condition.operator"
                    :metadata="metadata"
                    trigger-type="requirement"
                  />
                </a-form-item>
                <a-checkbox
                  class="condition-skip"
                  v-model:checked="condition.skip_when_empty"
                  >{{ $t("automation_skipWhenEmpty") }}</a-checkbox
                >
                <a-button
                  danger
                  :disabled="module.conditions.length === 1"
                  @click="module.conditions.splice(ci, 1)"
                  >{{ $t("common_delete") }}</a-button
                >
              </div>
              <a-button @click="module.conditions.push({})">{{
                $t("automation_addCondition")
              }}</a-button>
            </a-card>
          </a-form-item>
          <a-form-item
            :label="$t('auto_assignment_mode')"
            v-bind="validationProps(draft.mode, 'auto_assignment_mode')"
            required
            ><a-radio-group v-model:value="draft.mode"
              ><a-radio value="all">{{ $t("auto_assignment_all") }}</a-radio
              ><a-radio value="round_robin">{{
                $t("auto_assignment_round_robin")
              }}</a-radio></a-radio-group
            ></a-form-item
          >
          <a-form-item
            :label="$t('auto_assignment_target')"
            v-bind="
              validationProps(
                draft.target === 'members'
                  ? draft.member_ids
                  : draft.department_ids,
                'auto_assignment_target',
              )
            "
            required
          >
            <a-radio-group v-model:value="draft.target"
              ><a-radio value="members">{{
                $t("auto_assignment_members")
              }}</a-radio
              ><a-radio value="departments">{{
                $t("auto_assignment_departments")
              }}</a-radio></a-radio-group
            >
            <a-select
              v-if="draft.target === 'members'"
              v-model:value="draft.member_ids"
              mode="multiple"
              :options="members"
              class="target-picker"
            />
            <a-tree-select
              v-else
              v-model:value="departmentSelection"
              :tree-data="departmentTree"
              tree-checkable
              tree-check-strictly
              show-checked-strategy="SHOW_ALL"
              class="target-picker"
            />
            <a-typography-paragraph
              v-if="draft.target === 'departments'"
              type="secondary"
            >
              {{ $t("auto_assignment_departmentScope") }}
            </a-typography-paragraph>
          </a-form-item>
        </a-form>
        <template #footer>
          <a-space>
            <a-button type="primary" :loading="saving" @click="applyRule">{{
              $t("common_save")
            }}</a-button>
            <a-button :disabled="saving" @click="editorOpen = false">{{
              $t("common_cancel")
            }}</a-button>
          </a-space>
        </template>
      </a-drawer>
    </div>
  </a-spin>
</template>
<script setup>
import { ref, computed, nextTick, watch } from "vue";
import { formatLocalDate } from "@/utils/date";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { useAuthStore } from "@/stores/auth";
import { getAutoAssignment, saveAutoAssignment } from "@/api/autoAssignment";
import { getAutomationMetadata } from "@/api/automation";
import { automationOperatorOptions } from "@/utils/automationConditions";
import AutomationConditionValue from "./AutomationConditionValue.vue";
const props = defineProps({ wsId: { type: [String, Number], required: true } });
const { t } = useI18n();
const auth = useAuthStore();
const canManage = computed(() => auth.hasPermission("auto_assignment:manage"));
const config = ref({ rules: [] }),
  revision = ref(0),
  metadata = ref({}),
  departments = ref([]),
  loading = ref(false),
  ready = ref(false),
  saving = ref(false),
  editorOpen = ref(false),
  draft = ref(null);
const formElement = ref(null);
const submitted = ref(false);
function missingValue(value) {
  return (
    value == null ||
    (typeof value === "string" && !value.trim()) ||
    (Array.isArray(value) && !value.length)
  );
}
function validationProps(value, label) {
  const invalid = submitted.value && missingValue(value);
  return {
    validateStatus: invalid ? "error" : undefined,
    help: invalid
      ? t("auto_assignment_required", { field: t(label) })
      : undefined,
  };
}
const searchName = ref("");
const page = ref(1);
const pageSize = ref(10);
const filteredRules = computed(() => {
  const name = searchName.value.trim().toLocaleLowerCase();
  return config.value.rules.filter((rule) =>
    rule.name.toLocaleLowerCase().includes(name),
  );
});
const pagination = computed(() => ({
  current: page.value,
  pageSize: pageSize.value,
  total: filteredRules.value.length,
  showSizeChanger: true,
}));
function changePage(value) {
  page.value = value.current;
  pageSize.value = value.pageSize;
}
watch(searchName, () => {
  page.value = 1;
});
watch(
  () => filteredRules.value.length,
  (total) => {
    page.value = Math.min(
      page.value,
      Math.max(1, Math.ceil(total / pageSize.value)),
    );
  },
);
const columns = computed(() => [
  { title: t("automation_ruleName"), dataIndex: "name" },
  { title: t("auto_assignment_mode"), key: "mode" },
  { title: t("automation_on"), key: "enabled" },
  { title: t("auto_assignment_createdAt"), key: "created_at" },
  { title: t("auto_assignment_updatedAt"), key: "updated_at" },
  { title: t("auto_assignment_actions"), key: "actions", width: 132 },
]);
const fieldOptions = computed(() =>
  (metadata.value.fields || [])
    .filter((f) => !["cascader_single", "cascader_multi"].includes(f.type))
    .map((f) => ({ value: f.key, label: f.label })),
);
const activeMembers = computed(() =>
  (metadata.value.workspace_metadata?.members || [])
    .filter((m) => !m.removed && String(m.user_status ?? "1") !== "0")
    .map((m) => ({ value: Number(m.user_id), label: m.user_name })),
);
const members = computed(() => {
  const options = [...activeMembers.value];
  for (const id of draft.value?.member_ids || []) {
    if (!options.some((o) => o.value === id))
      options.push({
        value: id,
        label: t("auto_assignment_unavailableMember", { id }),
      });
  }
  return options;
});
const departmentSelection = computed({
  get: () => draft.value.department_ids.map((value) => ({ value })),
  set: (selection) => {
    draft.value.department_ids = selection.map(({ value }) => value);
  },
});
const departmentTree = computed(() => {
  const nodes = new Map(
    departments.value.map((d) => [
      d.id,
      { value: d.id, title: d.name, children: [] },
    ]),
  );
  const roots = [];
  for (const d of departments.value) {
    const parent = nodes.get(d.parent_id);
    (parent ? parent.children : roots).push(nodes.get(d.id));
  }
  return roots;
});
function field(key) {
  return (metadata.value.fields || []).find((f) => f.key === key);
}
function newModule() {
  return { relation: "AND", conditions: [{}] };
}
function openRule(rule) {
  submitted.value = false;
  draft.value = rule
    ? JSON.parse(JSON.stringify(rule))
    : {
        id:
          globalThis.crypto?.randomUUID?.() ||
          `${Date.now()}-${Math.random().toString(36).slice(2)}`,
        name: "",
        enabled: true,
        module_relation: "AND",
        modules: [newModule()],
        mode: "all",
        target: "members",
        member_ids: [],
        department_ids: [],
      };
  editorOpen.value = true;
}
async function applyRule() {
  if (saving.value || !ready.value) return;
  const r = draft.value;
  submitted.value = true;
  const invalid =
    !r.mode ||
    !r.name.trim() ||
    !(r.target === "members" ? r.member_ids : r.department_ids).length ||
    r.modules.some((m) =>
      m.conditions.some(
        (c) =>
          !c.field_key ||
          !c.operator ||
          (!["empty", "not_empty"].includes(c.operator) &&
            missingValue(c.value)),
      ),
    );
  if (invalid) {
    message.warning(t("auto_assignment_invalid"));
    await nextTick();
    const error = formElement.value?.$el.querySelector(
      ".ant-form-item-has-error",
    );
    error?.scrollIntoView({ behavior: "smooth", block: "center" });
    error
      ?.querySelector("input, textarea, button, [tabindex]")
      ?.focus({ preventScroll: true });
    return;
  }
  const nextConfig = JSON.parse(JSON.stringify(config.value));
  const index = nextConfig.rules.findIndex((item) => item.id === r.id);
  if (index < 0) nextConfig.rules.unshift(r);
  else nextConfig.rules[index] = r;
  if (await save(nextConfig)) editorOpen.value = false;
}
function toggleRule(id, enabled) {
  const nextConfig = JSON.parse(JSON.stringify(config.value));
  const rule = nextConfig.rules.find((item) => item.id === id);
  if (!rule) return false;
  rule.enabled = enabled;
  return save(nextConfig);
}
function deleteRule(id) {
  const nextConfig = JSON.parse(JSON.stringify(config.value));
  nextConfig.rules = nextConfig.rules.filter((item) => item.id !== id);
  return save(nextConfig);
}
let generation = 0;
watch(
  () => props.wsId,
  async (ws) => {
    const current = ++generation;
    searchName.value = "";
    page.value = 1;
    loading.value = true;
    ready.value = false;
    revision.value = 0;
    metadata.value = {};
    departments.value = [];
    editorOpen.value = false;
    config.value = { rules: [] };
    try {
      const [data, meta] = await Promise.all([
        getAutoAssignment(ws),
        getAutomationMetadata(ws, "requirement"),
      ]);
      if (current !== generation) return;
      if (!data.config || !Array.isArray(data.config.rules)) {
        message.error(t("auto_assignment_invalid"));
        return;
      }
      config.value = data.config;
      revision.value = data.revision;
      departments.value = data.departments || [];
      metadata.value = meta;
      ready.value = true;
    } finally {
      if (current === generation) loading.value = false;
    }
  },
  { immediate: true },
);
async function save(nextConfig = config.value) {
  if (saving.value || !ready.value) return false;
  const ws = props.wsId;
  const current = generation;
  saving.value = true;
  try {
    const data = await saveAutoAssignment(ws, {
      revision: revision.value,
      config: nextConfig,
    });
    if (current !== generation) return false;
    config.value = data.config;
    revision.value = data.revision;
    message.success(t("common_saveSuccess"));
    return true;
  } catch {
    return false;
  } finally {
    saving.value = false;
  }
}
</script>
<style scoped lang="less">
.assignment-title {
  margin: 0 0 16px;
}
.assignment-notice {
  margin-bottom: 16px;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}
.toolbar > .ant-btn {
  flex-shrink: 0;
}
.rule-search {
  width: 240px;
  max-width: 100%;
}
.assignment-actions {
  white-space: nowrap;
}
.assignment-actions :deep(.ant-btn) {
  padding-inline: 6px;
}
@media (max-width: 575px) {
  .rule-search {
    width: 100%;
  }
}
.assignment-form {
  padding: 12px 0;
}
.assignment-form :deep(.assignment-form-label) {
  flex: 0 0 140px;
}
.assignment-form :deep(.ant-form-item-control) {
  min-width: 0;
}
.assignment-name,
.target-picker {
  width: 480px;
  max-width: 100%;
}
.condition-module {
  margin-bottom: 12px;
}
.condition-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: flex-start;
  margin-bottom: 12px;
}
.condition-skip {
  min-height: var(--ant-control-height, 32px);
  align-items: center;
}
.condition-control {
  flex: 1 1 120px;
  min-width: 0;
  max-width: 180px;
  margin-bottom: 0;
}
.condition-control :deep(.ant-select),
.condition-control :deep(.ant-input),
.condition-control :deep(.ant-input-number),
.condition-control :deep(.ant-picker) {
  width: 100%;
}
.target-picker {
  display: block;
  margin-top: 16px;
}
@media (max-width: 575px) {
  .assignment-form {
    padding: 0;
  }
  .assignment-form :deep(.assignment-form-label) {
    flex: 0 0 100%;
    text-align: left;
  }
}
</style>
