<!--
 * ExternalNotificationStrategyModal.vue
 * Create / edit form for one enterprise external notification strategy: recipients, events, the
 * push platform and its webhook, plus the platform specific @ settings.
 -->
<template>
  <a-modal
    :open="open"
    :title="
      strategy
        ? $t('external_notification_edit')
        : $t('external_notification_add')
    "
    :width="640"
    :destroy-on-close="true"
    :confirm-loading="saving"
    @cancel="handleOpenChange(false)"
    @ok="saveStrategy"
  >
    <a-form ref="formRef" :model="form" layout="vertical">
      <a-form-item
        :label="$t('external_notification_name')"
        :validate-status="errors.name ? 'error' : ''"
        :help="errors.name"
      >
        <a-input
          v-model:value="form.name"
          :maxlength="30"
          :placeholder="$t('external_notification_name_placeholder')"
        />
      </a-form-item>
      <a-form-item
        :label="$t('external_notification_members')"
        :validate-status="errors.member_ids ? 'error' : ''"
        :help="errors.member_ids"
      >
        <!-- Not a select: recipients are picked in the enterprise member picker, which can group
             them by department or enterprise role. -->
        <div
          class="members-field"
          role="button"
          tabindex="0"
          @click="openMemberPicker"
          @keydown.enter.prevent="openMemberPicker"
          @keydown.space.prevent="openMemberPicker"
        >
          <a-tag
            v-for="member in selectedMembers"
            :key="member.id"
            closable
            @close="removeMember(member.id)"
            >{{ member.name || member.username }}</a-tag
          >
          <span v-if="!selectedMembers.length" class="members-placeholder">{{
            $t("external_notification_members_placeholder")
          }}</span>
          <DownOutlined class="members-arrow" />
        </div>
      </a-form-item>
      <a-form-item
        :label="$t('external_notification_events')"
        :validate-status="errors.events ? 'error' : ''"
        :help="errors.events"
      >
        <a-select
          v-model:value="form.events"
          mode="multiple"
          allow-clear
          :placeholder="$t('external_notification_events_placeholder')"
          :options="eventOptions"
        />
      </a-form-item>
      <a-form-item
        :label="$t('external_notification_platform')"
        :validate-status="errors.platform ? 'error' : ''"
        :help="errors.platform"
      >
        <a-radio-group v-model:value="form.platform" @change="changePlatform">
          <a-radio-button
            v-for="item in platforms"
            :key="item.value"
            :value="item.value"
            >{{ item.label }}</a-radio-button
          >
        </a-radio-group>
      </a-form-item>
      <div v-if="form.platform" class="webhook-config">
        <div class="webhook-title">
          {{ platformMeta(form.platform).label }}
          {{ $t("external_notification_webhook_title") }}
        </div>
        <a-form-item
          :label="$t('external_notification_webhook_url')"
          :validate-status="errors.webhook_url ? 'error' : ''"
          :help="errors.webhook_url"
        >
          <a-input
            v-model:value="form.webhook_url"
            :placeholder="platformMeta(form.platform).placeholder"
          />
        </a-form-item>
        <a-form-item :label="platformMeta(form.platform).secretLabel">
          <a-input-password
            v-model:value="form.secret"
            :placeholder="$t('external_notification_secret_placeholder')"
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
          <a-radio-group v-model:value="form.at_mode" button-style="solid">
            <a-radio-button
              v-for="item in atModes"
              :key="item.value"
              :value="item.value"
              >{{ item.label }}</a-radio-button
            >
          </a-radio-group>
          <div class="at-mode-tip">
            {{ $t("automation_paramMappingTip") }}
            <a-button
              type="link"
              size="small"
              @click="mappingModalOpen = true"
              >{{ $t("automation_paramMapping") }}</a-button
            >
          </div>
        </a-form-item>
      </div>
    </a-form>

    <RobotMentionModal
      v-model:open="atModalOpen"
      :value="form"
      :members="memberOptions"
      @confirm="confirmAtModal"
    />

    <RobotUserMappingModal
      v-model:open="mappingModalOpen"
      enterprise
      :members="members"
    />
    <EnterpriseMemberPickerModal
      v-model:open="memberPickerOpen"
      :members="members"
      :selected-ids="form.member_ids"
      @confirm="confirmMemberPicker"
    />
  </a-modal>
</template>

<script setup>
import RobotMentionModal from "@/components/notification/RobotMentionModal.vue";
import {
  robotAtModes as pushAtModes,
  robotAtModeLabels as atModeLabels,
  robotUrlPlaceholders,
} from "@/constants/robot";
import { computed, reactive, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { DownOutlined } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import {
  createExternalNotificationStrategy,
  updateExternalNotificationStrategy,
} from "@/api/workspace";
import RobotUserMappingModal from "@/components/notification/RobotUserMappingModal.vue";
import EnterpriseMemberPickerModal from "./EnterpriseMemberPickerModal.vue";

const props = defineProps({
  open: { type: Boolean, default: false },
  // The edited strategy, or null when creating one.
  strategy: { type: Object, default: null },
  // Every valid enterprise member, used for the recipient labels and the @ member list.
  members: { type: Array, default: () => [] },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  // Saved successfully; the caller refreshes the list.
  saved: () => true,
});

const { t } = useI18n();
const saving = ref(false);
const mappingModalOpen = ref(false);
const formRef = ref();
const form = reactive({
  name: "",
  member_ids: [],
  events: [],
  platform: "",
  webhook_url: "",
  secret: "",
  at_mode: "",
  at_user_ids: [],
  at_roles: [],
  at_all: false,
});
const errors = reactive({
  name: "",
  member_ids: "",
  events: "",
  platform: "",
  webhook_url: "",
});
const platforms = computed(() => [
  {
    value: "feishu",
    label: t("external_notification_feishu"),
    color: "blue",
    secretLabel: t("external_notification_feishu_secret"),
    placeholder: robotUrlPlaceholders.feishu,
  },
  {
    value: "wecom",
    label: t("external_notification_wecom"),
    color: "green",
    secretLabel: t("external_notification_wecom_secret"),
    placeholder: robotUrlPlaceholders.wecom,
  },
  {
    value: "dingtalk",
    label: t("external_notification_dingtalk"),
    color: "cyan",
    secretLabel: t("external_notification_dingtalk_secret"),
    placeholder: robotUrlPlaceholders.dingtalk,
  },
]);
const eventKeys = [
  "assign_requirement",
  "assign_bug",
  "comment_mention",
  "status_change",
  "due_reminder",
  "overdue",
];
const eventOptions = computed(() =>
  eventKeys.map((value) => ({
    value,
    label: t(`external_notification_event_${value}`),
  })),
);
const platformMeta = (value) =>
  platforms.value.find((item) => item.value === value) || {
    label: "",
    color: "default",
    secretLabel: "",
    placeholder: "",
  };
const membersById = computed(() => {
  const map = {};
  for (const member of props.members) map[Number(member.id)] = member;
  return map;
});
const selectedMembers = computed(() =>
  form.member_ids.map((id) => membersById.value[Number(id)]).filter(Boolean),
);
const memberOptions = computed(() =>
  props.members.map((item) => ({
    value: Number(item.id),
    label: item.name || item.username,
  })),
);
const atModes = computed(() =>
  (pushAtModes[form.platform] || []).map((mode) => ({
    value: mode,
    label: atModeLabels[mode] || t("automation_atModeMobile"),
  })),
);
const atModalOpen = ref(false);
const roleOptions = computed(() => [
  { value: "created_by", label: t("automation_atRoleCreatedBy") },
  { value: "assignee", label: t("automation_atRoleAssignee") },
]);
const atTags = computed(() => [
  ...(form.at_all
    ? [{ key: "all", type: "all", value: "all", label: t("automation_atAll") }]
    : []),
  ...(form.at_user_ids || []).map((id) => ({
    key: `user_${id}`,
    type: "user",
    value: id,
    label:
      memberOptions.value.find((item) => item.value === Number(id))?.label ||
      id,
  })),
  ...(form.at_roles || []).map((role) => ({
    key: `role_${role}`,
    type: "role",
    value: role,
    label: roleOptions.value.find((item) => item.value === role)?.label || role,
  })),
]);
const memberPickerOpen = ref(false);
function openMemberPicker() {
  memberPickerOpen.value = true;
}
function confirmMemberPicker(ids) {
  form.member_ids = ids.map((id) => Number(id));
  if (form.member_ids.length) errors.member_ids = "";
}
function removeMember(id) {
  form.member_ids = form.member_ids.filter(
    (item) => Number(item) !== Number(id),
  );
}
function openAtModal() {
  atModalOpen.value = true;
}
function confirmAtModal(value) {
  Object.assign(form, value);
}
function removeAtTag(tag) {
  if (tag.type === "all") form.at_all = false;
  else if (tag.type === "user")
    form.at_user_ids = (form.at_user_ids || []).filter(
      (id) => Number(id) !== Number(tag.value),
    );
  else
    form.at_roles = (form.at_roles || []).filter((role) => role !== tag.value);
}
function clearErrors() {
  Object.keys(errors).forEach((key) => {
    errors[key] = "";
  });
}
function resetForm() {
  Object.assign(form, {
    name: "",
    member_ids: [],
    events: [],
    platform: "",
    webhook_url: "",
    secret: "",
    at_mode: "",
    at_user_ids: [],
    at_roles: [],
    at_all: false,
  });
  clearErrors();
}
function fillForm(record) {
  Object.assign(form, {
    name: record.name,
    member_ids: [...(record.member_ids || [])],
    events: [...(record.events || [])],
    platform: record.platform,
    webhook_url: record.webhook_url,
    secret: record.secret || "",
    at_mode: record.at_mode,
    at_user_ids: [...(record.at_user_ids || [])],
    at_roles: [...(record.at_roles || [])],
    at_all: !!record.at_all,
  });
  clearErrors();
}
function handleOpenChange(value) {
  emit("update:open", value);
}
function changePlatform() {
  form.webhook_url = "";
  form.secret = "";
  // 切换渠道后原 @ 参数可能不被新机器人支持，重置为该渠道的默认方式。
  form.at_mode = pushAtModes[form.platform]?.[0] || "";
}
function validateForm() {
  clearErrors();
  if (!form.name.trim()) errors.name = t("external_notification_name_required");
  if (!form.member_ids.length)
    errors.member_ids = t("external_notification_members_required");
  if (!form.events.length)
    errors.events = t("external_notification_events_required");
  if (!form.platform)
    errors.platform = t("external_notification_platform_required");
  if (!form.webhook_url.trim())
    errors.webhook_url = t("external_notification_webhook_required");
  return !Object.values(errors).some(Boolean);
}
async function saveStrategy() {
  if (!validateForm()) return;
  saving.value = true;
  try {
    const payload = {
      ...form,
      name: form.name.trim(),
      webhook_url: form.webhook_url.trim(),
    };
    if (props.strategy) {
      await updateExternalNotificationStrategy(props.strategy.id, payload);
      message.success(t("external_notification_updated"));
    } else {
      await createExternalNotificationStrategy(payload);
      message.success(t("external_notification_created"));
    }
    emit("saved");
    handleOpenChange(false);
    resetForm();
  } catch (e) {
    // 400 等参数错误由调用方提示，其余状态拦截器已统一弹出。
    const detail = e?.response?.data?.message;
    if (e?.response?.status === 400 && detail) message.error(detail);
  } finally {
    saving.value = false;
  }
}

// The form is rebuilt on every open so a cancelled edit never leaks into the next one.
watch(
  () => props.open,
  (open) => {
    if (!open) return;
    if (props.strategy) fillForm(props.strategy);
    else resetForm();
  },
);
</script>

<style scoped lang="less">
.webhook-config {
  padding: 16px;
  background: #f8fafc;
  border: 1px solid #e4e7ec;
  border-radius: 6px;
}

.webhook-title {
  margin-bottom: 14px;
  font-weight: 600;
  color: #344054;
}

.members-field {
  display: flex;
  min-height: 32px;
  align-items: center;
  flex-wrap: wrap;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  padding: 3px 11px;
  padding-left: 4px;
  cursor: pointer;
  gap: 4px;

  // Picked members must look exactly like the tags the notification type select renders, so the
  // values below mirror .ant-select-selection-item instead of the lighter default a-tag look.
  // Size, icon and radius already share the same tokens; only these six differ.
  :deep(.ant-tag) {
    height: 24px;
    margin: 0;
    padding: 0 4px 0 8px;
    border-color: rgba(5, 5, 5, 0.06);
    background: rgba(0, 0, 0, 0.06);
    font-size: 14px;
    line-height: 22px;
  }

  &:hover {
    border-color: #4096ff;
  }

  &:focus-visible {
    border-color: #4096ff;
    outline: none;
    box-shadow: 0 0 0 2px rgb(5 145 255 / 10%);
  }
}

.members-placeholder {
  flex: 1;
  color: rgba(0, 0, 0, 0.25);
}

.members-arrow {
  margin-left: auto;
  color: rgba(0, 0, 0, 0.25);
  font-size: 12px;
}

.at-tags {
  margin-top: 8px;
}

.at-mode-tip {
  margin-top: 6px;
  color: #8c8c8c;
  font-size: 12px;
}
</style>
