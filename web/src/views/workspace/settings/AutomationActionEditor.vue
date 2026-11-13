<template>
  <div class="action-editor">
    <a-form layout="vertical">
      <div class="action-picker">
        <div class="section-label">{{ $t("automation_selectAction") }}</div>
        <a-select
          v-if="props.compact"
          class="action-type-select"
          :value="action.type"
          @change="selectAction"
        >
          <a-select-option
            v-for="item in actionTypes"
            :key="item.value"
            :value="item.value"
          >
            <span class="action-type-option">
              <component
                :is="item.icon"
                class="action-type-icon"
                :class="{ danger: item.danger }"
              />
              <strong>{{ item.label }}</strong>
              <small>{{ item.description }}</small>
            </span>
          </a-select-option>
        </a-select>
        <div v-else class="action-type-grid">
          <button
            v-for="item in actionTypes"
            :key="item.value"
            type="button"
            class="action-type-card"
            :class="{ active: action.type === item.value }"
            @click="selectAction(item.value)"
          >
            <component
              :is="item.icon"
              class="action-type-icon"
              :class="{ danger: item.danger }"
            />
            <span class="action-type-copy">
              <strong>{{ item.label }}</strong>
              <small>{{ item.description }}</small>
            </span>
          </button>
        </div>
      </div>

      <template v-if="action.type === 'status_transition'">
        <div
          v-for="(item, index) in action.transitions"
          :key="index"
          class="action-row"
        >
          <a-select v-model:value="item.scope" :options="scopeOptions" />
          <a-select
            v-model:value="item.target_status_id"
            :options="
              withMissingOption(
                statusOptions(item.scope),
                item.target_status_id,
              )
            "
            :status="
              referenceMissing(
                statusOptions(item.scope),
                item.target_status_id,
              ) ||
              (props.showErrors && requiredMissing(item.target_status_id))
                ? 'error'
                : undefined
            "
            :placeholder="$t('automation_targetStatus')"
          />
          <a-button
            type="text"
            danger
            @click="action.transitions.splice(index, 1)"
            ><DeleteOutlined
          /></a-button>
        </div>
        <a-button
          type="dashed"
          block
          @click="action.transitions.push(newTransition())"
          ><PlusOutlined />{{ $t("automation_addTransition") }}</a-button
        >
      </template>

      <template v-else-if="action.type === 'field_update'">
        <a-form-item
          :label="$t('automation_scope')"
          required
          :validate-status="
            props.showErrors && requiredMissing(action.scope)
              ? 'error'
              : undefined
          "
        >
          <a-select v-model:value="action.scope" :options="scopeOptions" />
        </a-form-item>
        <div
          v-for="(item, index) in action.updates"
          :key="index"
          class="field-update"
        >
          <a-select
            v-model:value="item.field_key"
            :options="
              withMissingOption(
                editableTargetFields(action.scope),
                item.field_key,
              )
            "
            :status="
              referenceMissing(
                editableTargetFields(action.scope),
                item.field_key,
              ) ||
              (props.showErrors && requiredMissing(item.field_key))
                ? 'error'
                : undefined
            "
            :placeholder="$t('automation_field')"
            @change="resetFieldUpdate(item)"
          />
          <a-select
            v-model:value="item.value_source"
            :options="valueSources"
            @change="resetUpdateValue(item)"
          />
          <template v-if="item.value_source === 'fixed'">
            <a-select
              v-if="fieldUsesOptions(item.field_key)"
              v-model:value="item.value"
              :mode="multiValueField(item.field_key) ? 'multiple' : undefined"
              :options="
                withMissingOption(fieldOptions(item.field_key), item.value)
              "
              :status="
                referenceMissing(fieldOptions(item.field_key), item.value) ||
                (props.showErrors && requiredMissing(item.value))
                  ? 'error'
                  : undefined
              "
              :placeholder="$t('automation_value')"
            />
            <a-input-number
              v-else-if="
                ['number', 'numeric'].includes(fieldType(item.field_key))
              "
              v-model:value="item.value"
              :status="
                props.showErrors && requiredMissing(item.value)
                  ? 'error'
                  : undefined
              "
            />
            <a-select
              v-else-if="fieldType(item.field_key) === 'date'"
              v-model:value="item.value"
              :options="withMissingOption(dateValues, item.value)"
              :status="
                referenceMissing(dateValues, item.value) ||
                (props.showErrors && requiredMissing(item.value))
                  ? 'error'
                  : undefined
              "
              :placeholder="$t('automation_value')"
            />
            <a-date-picker
              v-else-if="fieldType(item.field_key) === 'datetime'"
              :value="datetimePickerValue(item.value)"
              :show-time="{ format: 'HH:mm:ss' }"
              format="YYYY-MM-DD HH:mm:ss"
              :status="
                (props.showErrors || !requiredMissing(item.value)) &&
                !datetimePickerValue(item.value)
                  ? 'error'
                  : undefined
              "
              @change="
                (value) =>
                  (item.value = value
                    ? value.format('YYYY-MM-DD HH:mm:ss')
                    : '')
              "
            />
            <a-input
              v-else-if="fieldType(item.field_key) === 'phone'"
              v-model:value="item.value"
              :status="
                props.showErrors && requiredMissing(item.value)
                  ? 'error'
                  : undefined
              "
            />
            <AutomationTemplateInput
              v-else
              v-model="item.value"
              :variables="variables"
              :status="
                props.showErrors && requiredMissing(item.value)
                  ? 'error'
                  : undefined
              "
              :placeholder="$t('automation_value')"
            />
          </template>
          <a-select
            v-else
            v-model:value="item.source_field_key"
            :options="
              withMissingOption(
                compatibleSourceFields(item),
                item.source_field_key,
              )
            "
            :status="
              referenceMissing(
                compatibleSourceFields(item),
                item.source_field_key,
              ) ||
              (props.showErrors && requiredMissing(item.source_field_key))
                ? 'error'
                : undefined
            "
            :placeholder="$t('automation_sourceField')"
          />
          <a-button type="text" danger @click="action.updates.splice(index, 1)"
            ><DeleteOutlined
          /></a-button>
        </div>
        <a-button type="dashed" block @click="action.updates.push(newUpdate())"
          ><PlusOutlined />{{ $t("automation_addField") }}</a-button
        >
      </template>

      <template v-else-if="action.type === 'add_comment'">
        <a-form-item
          :label="$t('automation_scope')"
          required
          :validate-status="
            props.showErrors && requiredMissing(action.scope)
              ? 'error'
              : undefined
          "
          ><a-select v-model:value="action.scope" :options="scopeOptions"
        /></a-form-item>
        <a-form-item
          :label="$t('automation_commentTemplate')"
          required
          :validate-status="
            props.showErrors && !templateValid(action.template)
              ? 'error'
              : undefined
          "
          :help="templateHelp(action.template)"
        >
          <AutomationTemplateInput
            v-model="action.template"
            multiline
            :variables="variables"
            :placeholder="$t('automation_templatePlaceholder')"
          />
        </a-form-item>
      </template>

      <template v-else-if="action.type === 'create_requirement'">
        <a-form-item
          :label="$t('automation_parentRequirement')"
          required
          :validate-status="
            props.showErrors && requiredMissing(action.parent)
              ? 'error'
              : undefined
          "
        >
          <a-select
            v-model:value="action.parent"
            :options="createParentOptions"
          />
        </a-form-item>
        <a-form-item
          :label="$t('automation_titleTemplate')"
          required
          :validate-status="
            props.showErrors && !templateValid(action.template)
              ? 'error'
              : undefined
          "
          :help="templateHelp(action.template)"
        >
          <AutomationTemplateInput
            v-model="action.template"
            :variables="variables"
            :placeholder="$t('automation_templatePlaceholder')"
          />
        </a-form-item>
        <a-form-item
          :label="$t('automation_createDescription')"
          :validate-status="
            props.showErrors &&
            action.description?.trim() &&
            !templateValid(action.description)
              ? 'error'
              : undefined
          "
          :help="templateHelp(action.description)"
        >
          <AutomationTemplateInput
            v-model="action.description"
            multiline
            :variables="variables"
            :placeholder="$t('automation_templatePlaceholder')"
          />
        </a-form-item>
        <a-form-item
          v-for="field in createOptionFields"
          :key="field.key"
          :label="field.label"
        >
          <div class="create-field-row">
            <a-select
              :value="action[field.key]?.source || 'none'"
              :options="createSourceOptions(field)"
              @change="setCreateFieldSource(field.key, $event)"
            />
            <a-select
              v-if="action[field.key]?.source === 'fixed'"
              :value="createFieldValue(field)"
              :mode="field.multiple ? 'multiple' : undefined"
              :options="
                withMissingOption(field.options, createFieldValue(field))
              "
              :status="
                referenceMissing(field.options, createFieldValue(field)) ||
                (props.showErrors && requiredMissing(createFieldValue(field)))
                  ? 'error'
                  : undefined
              "
              :placeholder="$t('automation_value')"
              @change="setCreateFieldValue(field, $event)"
            />
          </div>
        </a-form-item>
        <a-form-item
          v-for="field in createDateFields"
          :key="field.key"
          :label="field.label"
        >
          <a-select
            :value="action[field.key] || 'none'"
            :options="createDateOptions(field)"
            @change="action[field.key] = $event === 'none' ? undefined : $event"
          />
        </a-form-item>
      </template>

      <template v-else-if="action.type === 'copy_work_item'">
        <a-form-item
          :label="$t('automation_targetProject')"
          required
          :validate-status="
            referenceMissing(copyTargets, action.target_workspace_id) ||
            (props.showErrors && requiredMissing(action.target_workspace_id))
              ? 'error'
              : undefined
          "
          :help="
            referenceMissing(copyTargets, action.target_workspace_id)
              ? $t('automation_invalidOption')
              : undefined
          "
        >
          <a-select
            v-model:value="action.target_workspace_id"
            :options="
              withMissingOption(copyTargets, action.target_workspace_id)
            "
            :status="
              referenceMissing(copyTargets, action.target_workspace_id) ||
              (props.showErrors && requiredMissing(action.target_workspace_id))
                ? 'error'
                : undefined
            "
            @change="action.parent = 'empty'"
          />
        </a-form-item>
        <a-form-item
          :label="$t('automation_parentRequirement')"
          required
          :validate-status="
            props.showErrors && requiredMissing(action.parent)
              ? 'error'
              : undefined
          "
        >
          <a-select
            v-model:value="action.parent"
            :options="copyParentOptions"
          />
        </a-form-item>
      </template>

      <template v-else-if="action.type === 'webhook'">
        <a-form-item :label="$t('automation_pushSystem')">
          <a-radio-group
            :value="action.push_system || 'custom'"
            button-style="solid"
            @change="selectPushSystem($event.target.value)"
          >
            <a-radio-button
              v-for="item in pushSystems"
              :key="item.value"
              :value="item.value"
              >{{ item.label }}</a-radio-button
            >
          </a-radio-group>
        </a-form-item>
        <a-form-item
          :label="$t('automation_webhookUrl')"
          required
          :validate-status="
            props.showErrors && !templateValid(action.url) ? 'error' : undefined
          "
          :help="templateHelp(action.url)"
        >
          <AutomationTemplateInput
            v-model="action.url"
            :variables="variables"
            :placeholder="pushUrlPlaceholder"
          />
        </a-form-item>

        <template v-if="(action.push_system || 'custom') === 'custom'">
          <a-form-item
            :label="$t('automation_webhookBody')"
            required
            :validate-status="
              props.showErrors && !templateValid(action.body)
                ? 'error'
                : undefined
            "
            :help="templateHelp(action.body)"
          >
            <AutomationTemplateInput
              v-model="action.body"
              multiline
              :auto-size="{ minRows: 3, maxRows: 12 }"
              :variables="variables"
              :placeholder="$t('automation_webhookBodyPlaceholder')"
            />
          </a-form-item>
        </template>

        <template v-else>
          <a-form-item
            v-if="action.push_system !== 'wecom'"
            :label="$t('automation_webhookSecret')"
          >
            <AutomationTemplateInput
              v-model="action.secret"
              :variables="variables"
              :placeholder="$t('automation_webhookSecretPlaceholder')"
            />
          </a-form-item>
          <a-form-item
            :label="$t('automation_webhookText')"
            required
            :validate-status="
              props.showErrors && !templateValid(action.template)
                ? 'error'
                : undefined
            "
            :help="templateHelp(action.template)"
          >
            <AutomationTemplateInput
              v-model="action.template"
              multiline
              :auto-size="{ minRows: 3, maxRows: 12 }"
              :variables="variables"
              :placeholder="$t('automation_webhookTextPlaceholder')"
            />
          </a-form-item>
          <a-form-item :label="$t('automation_atMembers')">
            <a-button @click="openAtModal">{{
              $t("automation_selectAtMembers")
            }}</a-button>
            <div v-if="atTags.length" class="at-tags">
              <a-tag
                v-for="tag in atTags"
                :key="tag.key"
                closable
                @close="removeAtTag(tag)"
                >{{ tag.label }}</a-tag
              >
            </div>
          </a-form-item>
          <a-form-item :label="$t('automation_atMode')">
            <a-radio-group
              :value="atModeValue"
              button-style="solid"
              @change="action.at_mode = $event.target.value"
            >
              <a-radio-button
                v-for="item in atModes"
                :key="item.value"
                :value="item.value"
                >{{ item.label }}</a-radio-button
              >
            </a-radio-group>
            <div class="at-mode-tip">
              {{ $t("automation_paramMappingTip") }}
              <a @click="mappingModalOpen = true">{{
                $t("automation_paramMapping")
              }}</a>
            </div>
          </a-form-item>
        </template>
      </template>
    </a-form>

    <RobotMentionModal
      v-model:open="atModalOpen"
      :value="action"
      :members="memberOptions"
      @confirm="confirmAtModal"
    />

    <RobotUserMappingModal
      v-model:open="mappingModalOpen"
      :ws-id="metadata.workspace_metadata?.workspace?.id"
      :members="members"
    />
  </div>
</template>

<script setup>
import RobotMentionModal from "@/components/notification/RobotMentionModal.vue";
import {
  robotAtModes as pushAtModes,
  robotAtModeLabels as atModeLabels,
  robotUrlPlaceholders,
} from "@/constants/robot";
import { computed, markRaw, ref } from "vue";
import { message } from "ant-design-vue";
import dayjs from "dayjs";
import { filterMembersForField } from "@/utils/customField";
import {
  CommentOutlined,
  CopyOutlined,
  DeleteOutlined,
  FileAddOutlined,
  FileSyncOutlined,
  FontColorsOutlined,
  LinkOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import RobotUserMappingModal from "@/components/notification/RobotUserMappingModal.vue";
import AutomationTemplateInput from "./AutomationTemplateInput.vue";

// 动作编辑器只消费后端返回的字段、状态和复制目标元数据。
const props = defineProps({
  metadata: { type: Object, default: () => ({}) },
  triggerType: { type: String, default: "requirement" },
  variables: { type: Array, default: () => [] },
  showErrors: { type: Boolean, default: false },
  // Condition branches stack many editors, so the action type picker collapses into a select.
  compact: { type: Boolean, default: false },
});
const action = defineModel({ type: Object, required: true });
const { t } = useI18n();
const actionTypes = computed(() => [
  actionOption(
    "status_transition",
    "automation_actionStatus",
    "automation_actionStatusDesc",
    FileSyncOutlined,
  ),
  actionOption(
    "field_update",
    "automation_actionField",
    "automation_actionFieldDesc",
    FontColorsOutlined,
  ),
  actionOption(
    "add_comment",
    "automation_actionComment",
    "automation_actionCommentDesc",
    CommentOutlined,
  ),
  actionOption(
    "create_requirement",
    "automation_actionCreate",
    "automation_actionCreateDesc",
    FileAddOutlined,
  ),
  actionOption(
    "copy_work_item",
    "automation_actionCopy",
    "automation_actionCopyDesc",
    CopyOutlined,
  ),
  actionOption(
    "webhook",
    "automation_actionWebhook",
    "automation_actionWebhookDesc",
    LinkOutlined,
    true,
  ),
]);
const scopeOptions = computed(() => [
  option("trigger", "automation_scopeTrigger"),
  option("parent", "automation_scopeParent"),
  // Defects can never own child work items, so that scope would silently match nothing.
  ...(props.triggerType === "defect"
    ? []
    : [option("descendants", "automation_scopeDescendants")]),
]);
const valueSources = computed(() => [
  option("fixed", "automation_valueFixed"),
  option("trigger", "automation_valueTrigger"),
  option("parent", "automation_valueParent"),
  option("child", "automation_valueChild"),
]);
// Fixed date values are relative to the trigger date, so a rule stays valid over time.
const dateValues = computed(() => [
  option("clear", "automation_dateValueClear"),
  option("today", "automation_dateValueToday"),
  option("tomorrow", "automation_dateValueTomorrow"),
  option("day_after_tomorrow", "automation_dateValueDayAfterTomorrow"),
]);
const createParentOptions = computed(() => [
  ...(props.triggerType === "requirement"
    ? [option("trigger", "automation_parentTrigger")]
    : []),
  option("empty", "automation_parentEmpty"),
]);
const copyParentOptions = computed(() => {
  const sameWorkspace =
    Number(action.value.target_workspace_id) ===
    Number(props.metadata.workspace_metadata?.workspace?.id);
  return [
    ...(sameWorkspace && props.triggerType === "requirement"
      ? [option("trigger", "automation_parentTrigger")]
      : []),
    option("empty", "automation_parentEmpty"),
  ];
});
const copyTargets = computed(() =>
  (props.metadata.copy_targets || []).map((item) => ({
    value: Number(item.id),
    label: item.name,
  })),
);
// Optional fields of the create_requirement action; options reuse the requirement-scope helpers.
const createOptionFields = computed(() => [
  {
    key: "status",
    label: t("workspace_field_status"),
    trigger: t("automation_createStatusTrigger"),
    options: statusOptions(),
    multiple: false,
  },
  {
    key: "priority",
    label: t("workspace_field_priority"),
    trigger: t("automation_createPriorityTrigger"),
    options: fieldOptions("priority"),
    multiple: false,
  },
  {
    key: "version",
    label: t("workspace_field_iteration"),
    trigger: t("automation_createVersionTrigger"),
    options: fieldOptions("version"),
    multiple: false,
  },
  {
    key: "assignee",
    label: t("workspace_field_assignee"),
    trigger: t("automation_createAssigneeTrigger"),
    options: fieldOptions("assignee"),
    multiple: true,
  },
]);
const createDateFields = computed(() => [
  {
    key: "planned_start",
    label: t("workspace_field_plannedStartDate"),
    trigger: t("automation_createStartTrigger"),
  },
  {
    key: "planned_end",
    label: t("workspace_field_plannedEndDate"),
    trigger: t("automation_createEndTrigger"),
  },
]);
const atModalOpen = ref(false);
const mappingModalOpen = ref(false);
const members = computed(
  () => props.metadata.workspace_metadata?.members || [],
);
const pushSystems = computed(() => [
  option("custom", "automation_pushCustom"),
  option("dingtalk", "automation_pushDingtalk"),
  option("feishu", "automation_pushFeishu"),
  option("wecom", "automation_pushWecom"),
]);
const pushUrlPlaceholder = computed(
  () =>
    robotUrlPlaceholders[action.value.push_system] ||
    "https://example.com/hooks/{{trigger.id}}",
);
const atModes = computed(() =>
  (pushAtModes[action.value.push_system] || []).map((mode) => ({
    value: mode,
    label: atModeLabels[mode] || t("automation_atModeMobile"),
  })),
);
const atModeValue = computed(() => {
  const mode = action.value.at_mode;
  return atModes.value.some((item) => item.value === mode)
    ? mode
    : atModes.value[0]?.value;
});
const memberOptions = computed(() =>
  members.value.map((item) => ({
    value: Number(item.user_id),
    label: item.user_name,
  })),
);
const roleOptions = computed(() => [
  option("created_by", "automation_atRoleCreatedBy"),
  option("assignee", "automation_atRoleAssignee"),
]);
const atTags = computed(() => [
  ...(action.value.at_all
    ? [{ key: "all", type: "all", value: "all", label: t("automation_atAll") }]
    : []),
  ...(action.value.at_user_ids || []).map((id) => ({
    key: `user_${id}`,
    type: "user",
    value: id,
    label:
      memberOptions.value.find((item) => item.value === Number(id))?.label ||
      t("automation_invalidOptionValue", { value: id }),
  })),
  ...(action.value.at_roles || []).map((role) => ({
    key: `role_${role}`,
    type: "role",
    value: role,
    label: roleOptions.value.find((item) => item.value === role)?.label || role,
  })),
]);

function option(value, key) {
  return { value, label: t(key) };
}
function requiredMissing(value) {
  return (
    value === undefined ||
    value === null ||
    value === "" ||
    (Array.isArray(value) && value.length === 0)
  );
}
function templateValid(value) {
  if (!value?.trim()) return false;
  const allowed = new Set(props.variables.map((item) => item.key));
  const matches = [...value.matchAll(/\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g)];
  const remainder = value.replace(/\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g, "");
  return (
    !remainder.includes("{{") &&
    !remainder.includes("}}") &&
    matches.every((match) => allowed.has(match[1]))
  );
}
function templateHelp(value) {
  return props.showErrors && value?.trim() && !templateValid(value)
    ? t("automation_invalidVariable")
    : undefined;
}
function referenceMissing(options, value) {
  if (requiredMissing(value)) return false;
  const values = Array.isArray(value) ? value : [value];
  return values.some(
    (current) =>
      !options.some((item) => String(item.value) === String(current)),
  );
}
function withMissingOption(options, value) {
  if (!referenceMissing(options, value)) return options;
  const values = Array.isArray(value) ? value : [value];
  const missing = values
    .filter(
      (current) =>
        !options.some((item) => String(item.value) === String(current)),
    )
    .map((current) => ({
      value: current,
      label: t("automation_invalidOptionValue", { value: current }),
      class: "automation-invalid-option",
    }));
  return [...options, ...missing];
}
function actionOption(value, labelKey, descriptionKey, icon, danger = false) {
  return {
    value,
    icon: markRaw(icon),
    danger,
    label: t(labelKey),
    description: t(descriptionKey),
  };
}
function newTransition() {
  return { scope: "trigger", target_status_id: undefined };
}
function newUpdate() {
  return { field_key: undefined, value_source: "fixed", value: "" };
}
function createSourceOptions(field) {
  return [
    { value: "none", label: t("automation_parentEmpty") },
    { value: "trigger", label: field.trigger },
    { value: "fixed", label: t("automation_valueFixed") },
  ];
}
// Fixed date values stay relative to the trigger date, matching the backend keywords.
function createDateOptions(field) {
  return [
    { value: "none", label: t("automation_parentEmpty") },
    { value: "trigger", label: field.trigger },
    { value: "today", label: t("automation_dateTriggerDay") },
    { value: "tomorrow", label: t("automation_dateTriggerPlus1") },
    {
      value: "day_after_tomorrow",
      label: t("automation_dateTriggerPlus2"),
    },
  ];
}
function createFieldValue(field) {
  const config = action.value[field.key];
  return field.multiple ? config?.values : config?.value;
}
function setCreateFieldValue(field, value) {
  const config = action.value[field.key];
  if (field.multiple) config.values = value;
  else config.value = value;
}
// Selecting none removes the field config so stripUI drops it before saving.
function setCreateFieldSource(key, source) {
  action.value[key] = source === "none" ? undefined : { source };
}
function selectAction(type) {
  if (action.value.type !== type) resetAction(type);
}
// 切换动作类型时丢弃旧类型字段，避免提交无效配置。
function resetAction(type) {
  const defaults = {
    status_transition: { type, transitions: [newTransition()] },
    field_update: { type, scope: "trigger", updates: [newUpdate()] },
    add_comment: { type, scope: "trigger", template: "" },
    create_requirement: {
      type,
      parent: "empty",
      template: "",
      description: "",
    },
    copy_work_item: { type, target_workspace_id: undefined, parent: "empty" },
    webhook: { type, push_system: "custom", url: "", body: "" },
  };
  action.value = defaults[type];
}
// 推送渠道决定机器人字段与可用的 @ 参数，切换时丢弃另一形态的字段避免提交无效配置。
function selectPushSystem(system) {
  if ((action.value.push_system || "custom") === system) return;
  const base = { type: "webhook", push_system: system, url: action.value.url };
  if (system === "custom") {
    action.value = { ...base, body: "" };
    return;
  }
  action.value = {
    ...base,
    template: "",
    at_mode: pushAtModes[system][0],
    at_user_ids: [],
    at_roles: [],
    at_all: false,
    ...(system === "wecom" ? {} : { secret: "" }),
  };
}
function openAtModal() {
  atModalOpen.value = true;
}
function confirmAtModal(value) {
  Object.assign(action.value, value);
}
function removeAtTag(tag) {
  if (tag.type === "all") action.value.at_all = false;
  else if (tag.type === "user")
    action.value.at_user_ids = (action.value.at_user_ids || []).filter(
      (id) => Number(id) !== Number(tag.value),
    );
  else
    action.value.at_roles = (action.value.at_roles || []).filter(
      (role) => role !== tag.value,
    );
}

function targetType(scope) {
  return scope === "trigger" ? props.triggerType : "requirement";
}
function statusOptions(scope) {
  return (
    props.metadata.workspace_metadata?.[targetType(scope)]?.statuses || []
  ).map((item) => ({ value: Number(item.id), label: item.name }));
}
function targetFields(scope) {
  return scope === "trigger"
    ? props.metadata.fields || []
    : props.metadata.requirement_fields || [];
}
function editableTargetFields(scope) {
  return targetFields(scope)
    .filter((field) => field.editable)
    .map((field) => ({ value: field.key, label: field.label }));
}
function selectedField(key) {
  return targetFields(action.value.scope).find((field) => field.key === key);
}
function datetimePickerValue(value) {
  if (
    typeof value !== "string" ||
    !/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)
  )
    return null;
  const parsed = dayjs(value);
  return parsed.isValid() && parsed.format("YYYY-MM-DD HH:mm:ss") === value
    ? parsed
    : null;
}
function fieldType(key) {
  return selectedField(key)?.type;
}
function fieldUsesOptions(key) {
  return (
    ["select_single", "select_multi", "user", "user_multi"].includes(
      fieldType(key),
    ) || ["priority", "assignee", "version"].includes(key)
  );
}
function fieldOptions(key) {
  const field = selectedField(key);
  if (["user", "user_multi"].includes(field?.type) && key !== "assignee")
    return filterMembersForField(
      field,
      props.metadata.workspace_metadata?.custom_field_members || [],
    ).map((member) => ({
      value: Number(member.user_id),
      label: member.user_name,
    }));
  if (field?.options?.length)
    return field.options.map((item) => ({ value: item.key, label: item.name }));
  const group =
    props.metadata.workspace_metadata?.[targetType(action.value.scope)] || {};
  if (key === "priority")
    return (group.priorities || []).map((item) => ({
      value: Number(item.id),
      label: item.label,
    }));
  if (key === "assignee")
    return (props.metadata.workspace_metadata?.members || []).map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    }));
  if (key === "version")
    return (props.metadata.workspace_metadata?.versions || []).map((item) => ({
      value: Number(item.id),
      label: item.name,
    }));
  return [];
}
function resetFieldUpdate(item) {
  item.value = emptyFieldValue(item.field_key);
  item.source_field_key = undefined;
}
function resetUpdateValue(item) {
  item.value = emptyFieldValue(item.field_key);
  item.source_field_key = undefined;
}
// The assignee field keeps every selected member, so it behaves like a multi-select field.
function multiValueField(key) {
  return (
    ["select_multi", "user_multi"].includes(fieldType(key)) ||
    key === "assignee"
  );
}
function emptyFieldValue(key) {
  return multiValueField(key) ? [] : "";
}
// 复制字段值时只展示后端允许的兼容字段类型。
function compatibleSourceFields(item) {
  const source =
    item.value_source === "trigger"
      ? props.metadata.fields || []
      : props.metadata.requirement_fields || [];
  const target = selectedField(item.field_key);
  return source
    .filter(
      (field) =>
        target &&
        (field.type === target.type ||
          (["text", "textarea"].includes(field.type) &&
            ["text", "textarea"].includes(target.type))),
    )
    .map((field) => ({ value: field.key, label: field.label }));
}
</script>

<style scoped lang="less">
// a-modal 内容会被 teleport 到 body，不在 .action-editor 子树内，样式必须写在顶层。
// The compact picker renders its options in a teleported dropdown, so these rules stay at the top level too.
.action-type-select {
  width: 100%;
}
.action-type-option {
  display: flex;
  align-items: center;
  gap: 8px;
  .action-type-icon {
    flex: none;
    color: #1677ff;
    font-size: 16px;
    &.danger {
      color: #ff4d4f;
    }
  }
  strong {
    color: #262626;
    font-size: 13px;
    font-weight: 500;
  }
  small {
    color: #8c8c8c;
    font-size: 12px;
  }
}

.action-editor {
  .action-picker {
    margin-bottom: 24px;
  }
  .section-label {
    margin-bottom: 12px;
    color: #262626;
    font-size: 13px;
    font-weight: 500;
  }
  .action-type-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 16px;
  }
  .action-type-card {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    min-height: 82px;
    padding: 16px;
    text-align: left;
    background: #fff;
    border: 1px solid #e8e8e8;
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;
    &:hover,
    &.active {
      background: #f0f5ff;
      border-color: #1677ff;
      box-shadow: 0 2px 8px rgb(22 119 255 / 8%);
    }
  }
  .action-type-icon {
    flex: none;
    margin-top: 2px;
    color: #1677ff;
    font-size: 22px;
    &.danger {
      color: #ff4d4f;
    }
  }
  .action-type-copy {
    min-width: 0;
    strong,
    small {
      display: block;
    }
    strong {
      margin-bottom: 4px;
      color: #262626;
      font-size: 13px;
      font-weight: 500;
    }
    small {
      color: #8c8c8c;
      font-size: 12px;
      line-height: 1.45;
    }
  }
  .at-tags {
    margin-top: 8px;
  }
  .at-mode-tip {
    margin-top: 6px;
    color: #8c8c8c;
    font-size: 12px;
    a {
      color: #1677ff;
    }
  }
  .action-row,
  .field-update,
  .create-field-row {
    display: grid;
    grid-template-columns: 1fr 1.4fr;
    gap: 10px;
    margin-bottom: 12px;
  }
  .action-row {
    grid-template-columns: 1fr 1.4fr 36px;
  }
  .create-field-row {
    margin-bottom: 0;
  }
  .field-update {
    grid-template-columns: 1.1fr 110px 1.2fr 36px;
  }
  :deep(.ant-select-status-error .ant-select-selector) {
    border-color: #ff4d4f;
  }
  @media (max-width: 760px) {
    .action-type-grid {
      grid-template-columns: 1fr 1fr;
    }
  }
}
</style>
