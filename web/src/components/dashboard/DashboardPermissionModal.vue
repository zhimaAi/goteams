<template>
  <a-modal
    :open="open"
    :title="$t('dashboard_permission_title')"
    :confirm-loading="saving"
    :ok-text="$t('common_save')"
    :cancel-text="$t('common_cancel')"
    width="576px"
    @ok="save"
    @cancel="emit('update:open', false)"
  >
    <a-spin :spinning="loading">
      <div class="dashboard-permission-form">
        <div class="dashboard-permission-form__label">
          {{ $t("dashboard_visibility_label") }}
        </div>
        <a-radio-group
          v-model:value="form.visibility_type"
          class="dashboard-permission-form__visibility"
        >
          <a-radio value="public">{{
            $t("dashboard_visibility_public")
          }}</a-radio>
          <a-radio value="self">{{ $t("dashboard_visibility_self") }}</a-radio>
          <a-radio value="roles">{{
            $t("dashboard_visibility_roles")
          }}</a-radio>
          <a-select
            v-if="form.visibility_type === 'roles'"
            v-model:value="form.role_ids"
            class="dashboard-permission-form__nested-select"
            mode="multiple"
            show-search
            option-filter-prop="label"
            :placeholder="$t('dashboard_visibility_roles_placeholder')"
            :options="roleOptions"
          />
          <a-radio value="members">{{
            $t("dashboard_visibility_members")
          }}</a-radio>
          <a-select
            v-if="form.visibility_type === 'members'"
            v-model:value="form.member_ids"
            class="dashboard-permission-form__nested-select"
            mode="multiple"
            show-search
            option-filter-prop="label"
            :placeholder="$t('dashboard_visibility_members_placeholder')"
            :options="memberOptions"
          />
        </a-radio-group>
        <div
          class="dashboard-permission-form__label dashboard-permission-form__label--editors"
        >
          {{ $t("dashboard_editors_label") }}
        </div>
        <div class="dashboard-permission-form__implicit">
          <a-checkbox checked disabled>{{
            $t("dashboard_editor_admin")
          }}</a-checkbox>
          <a-checkbox checked disabled>{{
            $t("dashboard_editor_creator")
          }}</a-checkbox>
        </div>
        <a-select
          v-model:value="form.editor_ids"
          class="dashboard-permission-form__select"
          mode="multiple"
          show-search
          option-filter-prop="label"
          :placeholder="$t('dashboard_editors_placeholder')"
          :options="memberOptions"
        />
      </div>
    </a-spin>
  </a-modal>
</template>

<script setup>
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  getDashboardPermission,
  updateDashboardPermission,
} from "@/api/dashboard";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], required: true },
  dashboard: { type: Object, default: null },
  members: { type: Array, default: () => [] },
  roles: { type: Array, default: () => [] },
});

const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  saved: (value) => Boolean(value),
});

const { t } = useI18n();
const loading = ref(false);
const saving = ref(false);
const form = reactive({
  visibility_type: "public",
  role_ids: [],
  member_ids: [],
  editor_ids: [],
  revision: 0,
});

const memberOptions = computed(() =>
  props.members
    .filter((member) => !member.removed)
    .map((member) => ({
      value: Number(member.user_id ?? member.id),
      label: member.user_name || member.name || member.username || "-",
    })),
);
const roleOptions = computed(() =>
  props.roles.map((role) => ({
    value: Number(role.id),
    label: role.name || "-",
  })),
);

watch(
  () => props.open,
  (open) => {
    if (open) load();
  },
);

function reset(value = {}) {
  form.visibility_type = value.visibility_type || "public";
  form.role_ids = normalizeIds(value.role_ids);
  form.member_ids = normalizeIds(value.member_ids);
  form.editor_ids = normalizeIds(value.editor_ids);
  form.revision = Number(value.revision) || 0;
}

function normalizeIds(items) {
  return (items || []).map(Number).filter(Boolean);
}

async function load() {
  if (!props.dashboard?.id) return;
  loading.value = true;
  try {
    reset(await getDashboardPermission(props.wsId, props.dashboard.id));
  } catch {
    reset();
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (form.visibility_type === "roles" && !form.role_ids.length) {
    message.warning(t("dashboard_visibility_roles_required"));
    return;
  }
  if (form.visibility_type === "members" && !form.member_ids.length) {
    message.warning(t("dashboard_visibility_members_required"));
    return;
  }
  saving.value = true;
  try {
    const payload = {
      visibility_type: form.visibility_type,
      role_ids: form.visibility_type === "roles" ? form.role_ids : [],
      member_ids: form.visibility_type === "members" ? form.member_ids : [],
      editor_ids: form.editor_ids,
      revision: form.revision,
    };
    const value = await updateDashboardPermission(
      props.wsId,
      props.dashboard.id,
      payload,
    );
    emit("saved", value || payload);
    emit("update:open", false);
    message.success(t("dashboard_permission_saved"));
  } catch {
    // The shared request interceptor displays the failure.
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped lang="less">
.dashboard-permission-form {
  padding: 10px 6px;
}
.dashboard-permission-form__visibility {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 16px;
}
.dashboard-permission-form__nested-select {
  width: calc(100% - 24px);
  margin-left: 24px;
}
.dashboard-permission-form__implicit {
  display: flex;
  flex-direction: column;
  gap: 12px;
  :deep(.ant-checkbox-wrapper) {
    margin-inline-start: 0;
  }
}

.dashboard-permission-form__label {
  margin-bottom: 10px;
  color: #262626;
  font-weight: 500;
}

.dashboard-permission-form__label--editors {
  margin-top: 24px;
}

.dashboard-permission-form__hint {
  margin: -4px 0 10px;
  color: #8c8c8c;
  font-size: 12px;
}

.dashboard-permission-form__select {
  width: 100%;
  margin-top: 12px;
}
</style>
