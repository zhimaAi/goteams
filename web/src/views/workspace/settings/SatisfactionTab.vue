<template>
  <section
    class="satisfaction-config-panel"
    :class="{ 'satisfaction-config-panel-loading': loading }"
  >
    <a-result
      v-if="!canView"
      status="403"
      :title="t('workspace_noPermission')"
    />
    <a-spin v-else-if="loading" />
    <a-alert
      v-else-if="failed"
      type="error"
      :message="t('common_requestFailed')"
      show-icon
    >
      <template #action
        ><a-button @click="load">{{ t("issue_retry") }}</a-button></template
      >
    </a-alert>
    <a-form
      v-else-if="config"
      :model="config"
      :disabled="!canManage || saving"
      :labelCol="{ span: 2 }"
      class="satisfaction-settings"
      @finish="save"
    >
      <div class="satisfaction-settings-content">
        <a-space class="satisfaction-title"
          ><h3>{{ t("satisfaction_title") }}</h3>
          <a-switch
            v-model:checked="config.enabled"
            :disabled="!canManage || saving"
        /></a-space>
        <a-alert
          type="info"
          show-icon
          :message="t('satisfaction_config_hint')"
        />
        <fieldset :disabled="editDisabled">
          <a-form-item :label="t('satisfaction_validity')"
            ><a-select
              :disabled="editDisabled"
              v-model:value="config.validity_seconds"
              :options="durations"
          /></a-form-item>
          <div class="satisfaction-section-heading">
            <h4>{{ t("satisfaction_manual_title") }}</h4>
            <span>{{ t("satisfaction_manual_hint") }}</span>
          </div>
          <a-form-item :label="t('satisfaction_manual')"
            ><a-switch
              :disabled="editDisabled"
              v-model:checked="config.manual_enabled"
            />
          </a-form-item>
          <div class="satisfaction-section-heading">
            <h4>{{ t("satisfaction_auto") }}</h4>
            <span>{{ t("satisfaction_auto_hint") }}</span>
          </div>
          <a-form-item :label="t('satisfaction_scope')"
            ><a-checkbox-group
              :disabled="editDisabled"
              v-model:value="config.auto_creator_kinds"
              :options="creatorKinds"
          /></a-form-item>
          <a-form-item
            :label="t('satisfaction_status')"
            :extra="t('satisfaction_status_hint')"
            required
            ><a-select
              :disabled="editDisabled"
              v-model:value="config.auto_status_ids"
              mode="multiple"
              :options="statuses"
          /></a-form-item>
          <a-form-item :label="t('satisfaction_templates')"
            ><a-select
              :disabled="editDisabled"
              v-model:value="config.auto_template_ids"
              mode="multiple"
              :options="templates"
              :placeholder="t('satisfaction_all_templates')"
          /></a-form-item>
          <a-form-item class="satisfaction-mode">
            <template #label>
              <span id="satisfaction-mode-label">{{
                t("satisfaction_mode")
              }}</span>
            </template>
            <div class="satisfaction-mode-content">
              <a-radio-group
                :disabled="editDisabled"
                class="satisfaction-mode-selector"
                :value="config.rating_mode"
                aria-labelledby="satisfaction-mode-label"
                @change="changeMode($event.target.value)"
              >
                <a-radio v-for="n in [2, 3, 4, 5]" :key="n" :value="n">{{
                  t(`satisfaction_mode_${n}`)
                }}</a-radio>
              </a-radio-group>
              <div
                class="satisfaction-mode-panel"
                :class="{ 'is-star-mode': config.rating_mode !== 2 }"
              >
                <div
                  v-for="option in displayOptions"
                  :key="option.key"
                  class="satisfaction-option"
                >
                  <div class="satisfaction-option-main">
                    <div class="satisfaction-option-rating">
                      <a-popover
                        trigger="click"
                        placement="bottomLeft"
                        :open="editDisabled ? false : undefined"
                      >
                        <template #content>
                          <div class="satisfaction-sentiment-editor">
                            <label :for="`sentiment-${option.key}`">{{
                              t("satisfaction_sentiment_label")
                            }}</label>
                            <a-select
                              :id="`sentiment-${option.key}`"
                              v-model:value="option.sentiment"
                              :options="sentiments"
                              :aria-label="t('satisfaction_sentiment_label')"
                              :disabled="editDisabled"
                            />
                          </div>
                        </template>
                        <button
                          type="button"
                          class="satisfaction-option-icon"
                          :disabled="editDisabled"
                          :class="
                            config.rating_mode === 2
                              ? option.value === 2
                                ? 'is-positive'
                                : 'is-negative'
                              : 'is-stars'
                          "
                          :aria-label="`${option.label}: ${t('satisfaction_sentiment_label')}`"
                        >
                          <span
                            v-if="config.rating_mode !== 2"
                            class="satisfaction-option-stars"
                            aria-hidden="true"
                          >
                            <StarFilled
                              v-for="star in config.rating_mode"
                              :key="star"
                              :class="{ 'is-empty': star > option.value }"
                            />
                          </span>
                          <LikeFilled
                            v-else
                            :class="{
                              'satisfaction-negative-icon': option.value === 1,
                            }"
                            aria-hidden="true"
                          />
                        </button>
                      </a-popover>
                      <a-input
                        :disabled="editDisabled"
                        v-model:value="option.label"
                        class="satisfaction-option-name"
                        :style="{
                          width: `${Math.max(44, Math.min(180, Array.from(option.label).length * 14))}px`,
                        }"
                        :maxlength="100"
                        :bordered="false"
                        :aria-label="t('satisfaction_option_label')"
                      />
                    </div>
                    <div class="satisfaction-option-tags">
                      <span
                        v-for="(tag, index) in option.tags"
                        :key="tag.key"
                        class="satisfaction-tag-editor"
                      >
                        <a-input
                          :disabled="editDisabled"
                          v-model:value="tag.label"
                          :maxlength="30"
                          :aria-label="t('satisfaction_tag_label')"
                          :bordered="false"
                        />
                        <button
                          type="button"
                          :aria-label="`${t('satisfaction_remove')} ${tag.label || t('satisfaction_tag_label')}`"
                          :disabled="editDisabled"
                          @click="removeTag(option, index)"
                        >
                          <CloseOutlined aria-hidden="true" />
                        </button>
                      </span>
                      <a-button
                        class="satisfaction-add-tag"
                        :disabled="editDisabled || option.tags.length >= 20"
                        @click="addTag(option)"
                        ><PlusOutlined aria-hidden="true" />{{
                          t("satisfaction_add_tag")
                        }}</a-button
                      >
                    </div>
                    <div class="satisfaction-option-requirements">
                      <span class="satisfaction-required-control">
                        <a-checkbox
                          v-model:checked="option.tags_required"
                          :disabled="editDisabled || !option.tags.length"
                          >{{ t("satisfaction_tags_required") }}</a-checkbox
                        >
                        <a-tooltip :title="t('satisfaction_tags_required_hint')"
                          ><button
                            type="button"
                            class="satisfaction-info"
                            :aria-label="t('satisfaction_tags_required_hint')"
                          >
                            <InfoCircleOutlined /></button
                        ></a-tooltip>
                      </span>
                      <span class="satisfaction-required-control">
                        <a-checkbox
                          :disabled="editDisabled"
                          v-model:checked="option.comment_required"
                          >{{ t("satisfaction_comment_required") }}</a-checkbox
                        >
                        <a-tooltip
                          :title="t('satisfaction_comment_required_hint')"
                          ><button
                            type="button"
                            class="satisfaction-info"
                            :aria-label="
                              t('satisfaction_comment_required_hint')
                            "
                          >
                            <InfoCircleOutlined /></button
                        ></a-tooltip>
                      </span>
                    </div>
                  </div>
                </div>
                <div class="satisfaction-resolution-config">
                  <span>{{ t("satisfaction_collect_resolved") }}</span>
                  <a-switch
                    :disabled="editDisabled"
                    v-model:checked="config.resolved_enabled"
                    size="small"
                    :aria-label="t('satisfaction_collect_resolved')"
                    @change="!$event && (config.resolved_required = false)"
                  />
                  <a-checkbox
                    v-model:checked="config.resolved_required"
                    :disabled="editDisabled || !config.resolved_enabled"
                    >{{ t("satisfaction_required") }}</a-checkbox
                  >
                </div>
              </div>
            </div>
          </a-form-item>
          <a-form-item :label="t('satisfaction_invitation_text')"
            ><a-textarea
              :disabled="editDisabled"
              v-model:value="config.invitation_text"
              :maxlength="500"
              show-count
          /></a-form-item>
          <a-form-item :label="t('satisfaction_completion_text')"
            ><a-textarea
              :disabled="editDisabled"
              v-model:value="config.completion_text"
              :maxlength="500"
              show-count
          /></a-form-item>
        </fieldset>
      </div>
      <footer v-if="canManage" class="satisfaction-save-bar">
        <a-button type="primary" html-type="submit" :loading="saving">{{
          t("satisfaction_save")
        }}</a-button>
      </footer>
    </a-form>
  </section>
</template>
<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message, Modal } from "ant-design-vue";
import {
  LikeFilled,
  StarFilled,
  PlusOutlined,
  CloseOutlined,
  InfoCircleOutlined,
} from "@ant-design/icons-vue";
import {
  getSatisfactionConfig,
  saveSatisfactionConfig,
} from "@/api/satisfaction";
import { getWorkspaceMetadata } from "@/api/workspace";
import { listIssueInviteTemplates } from "@/api/issueInvite";
import { useAuthStore } from "@/stores/auth";
const props = defineProps({ wsId: { type: [String, Number], required: true } });
const { t } = useI18n();
const authStore = useAuthStore();
const canManage = computed(() =>
  authStore.hasPermission("satisfaction:manage"),
);
const canView = computed(
  () => canManage.value || authStore.hasPermission("satisfaction:view"),
);
const config = ref(null),
  loading = ref(false),
  saving = ref(false),
  failed = ref(false),
  statuses = ref([]),
  templates = ref([]);
const editDisabled = computed(
  () => !canManage.value || !config.value?.enabled || saving.value,
);
let sequence = 0;
let modeConfirmation;
const durations = computed(() =>
  [1, 6, 12, 24, 48, 72, 168, 720].map((hours) => ({
    value: hours * 3600,
    label: t("satisfaction_hours", { count: hours }),
  })),
);
const creatorKinds = computed(() => [
  { value: 1, label: t("satisfaction_customer_created") },
  { value: 2, label: t("satisfaction_member_created") },
]);
const displayOptions = computed(() =>
  [...(config.value?.options || [])].sort((a, b) => b.value - a.value),
);
const sentiments = computed(() =>
  ["negative", "neutral", "positive"].map((value) => ({
    value,
    label: t(`satisfaction_${value}`),
  })),
);
async function load() {
  const seq = ++sequence;
  modeConfirmation?.destroy();
  modeConfirmation = undefined;
  config.value = null;
  statuses.value = [];
  templates.value = [];
  saving.value = false;
  failed.value = false;
  loading.value = false;
  if (!canView.value) return;
  loading.value = true;
  try {
    const [c, m, items] = await Promise.all([
      getSatisfactionConfig(props.wsId),
      getWorkspaceMetadata(props.wsId),
      listIssueInviteTemplates(props.wsId),
    ]);
    if (seq !== sequence) return;
    if (!Array.isArray(c.auto_status_ids)) {
      throw new Error("Satisfaction configuration requires auto_status_ids");
    }
    config.value = c;
    statuses.value = (m.requirement?.statuses || []).map((v) => ({
      value: Number(v.id),
      label: v.name,
    }));
    templates.value = items
      .filter((v) => Number(v.enabled) === 1)
      .map((v) => ({ value: Number(v.id), label: v.name }));
  } catch {
    if (seq === sequence) failed.value = true;
  } finally {
    if (seq === sequence) loading.value = false;
  }
}
function createConfigKey() {
  const cryptoApi = globalThis.crypto;
  if (typeof cryptoApi?.randomUUID === "function") {
    return cryptoApi.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}
function changeMode(n) {
  if (editDisabled.value || n === config.value.rating_mode) return;
  const seq = sequence;
  modeConfirmation = Modal.confirm({
    title: t("satisfaction_mode_reset"),
    onOk() {
      if (seq !== sequence || editDisabled.value) return;
      config.value.rating_mode = n;
      config.value.options = Array.from({ length: n }, (_, index) => ({
        key: createConfigKey(),
        label: t(`satisfaction_rating_${n}_${index + 1}`),
        value: index + 1,
        sentiment:
          n === 2
            ? index
              ? "positive"
              : "negative"
            : n === 3
              ? ["negative", "neutral", "positive"][index]
              : n === 4
                ? index < 2
                  ? "negative"
                  : "positive"
                : index < 2
                  ? "negative"
                  : index === 2
                    ? "neutral"
                    : "positive",
        tags: [],
        tags_required: false,
        comment_required: false,
      }));
    },
  });
}
function addTag(option) {
  if (editDisabled.value || option.tags.length >= 20) return;
  option.tags.push({ key: createConfigKey(), label: "" });
}
function removeTag(option, index) {
  if (editDisabled.value) return;
  option.tags.splice(index, 1);
  if (!option.tags.length) option.tags_required = false;
}
async function save() {
  if (!canManage.value || !config.value || saving.value) return;
  if (
    config.value.enabled &&
    (!config.value.auto_status_ids.length ||
      !config.value.auto_creator_kinds.length)
  ) {
    message.warning(t("satisfaction_invalid"));
    return;
  }
  saving.value = true;
  const ws = props.wsId;
  const seq = sequence;
  try {
    const saved = await saveSatisfactionConfig(ws, config.value);
    if (seq === sequence && ws === props.wsId && canManage.value) {
      config.value = saved;
      message.success(t("satisfaction_saved"));
    }
  } finally {
    if (seq === sequence) saving.value = false;
  }
}
watch([() => props.wsId, canView, canManage], load, {
  immediate: true,
  flush: "sync",
});
onBeforeUnmount(() => {
  sequence++;
  modeConfirmation?.destroy();
});
</script>
<style scoped lang="less">
.satisfaction-config-panel {
  height: 100%;
  min-height: 0;
}
.satisfaction-config-panel-loading {
  display: flex;
  align-items: center;
  justify-content: center;
}
.satisfaction-settings {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.satisfaction-settings-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
.satisfaction-save-bar {
  display: flex;
  justify-content: flex-start;
  position: relative;
  z-index: 1;
  flex-shrink: 0;
  margin-left: calc(0px - var(--settings-content-padding-left, 0px));
  margin-right: calc(0px - var(--settings-content-padding-right, 0px));
  padding: 16px 24px calc(16px + env(safe-area-inset-bottom, 0px));
  padding-left: calc(95px + var(--settings-content-padding-left, 0px));
  padding-right: calc(24px + var(--settings-content-padding-right, 0px));
  background: var(--app-surface, #fff);
  box-shadow: 0 -4px 12px rgb(0 0 0 / 6%);
}
.satisfaction-title {
  display: flex;
  justify-content: space-between;
}
.satisfaction-section-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px 12px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--app-border, #f0f0f0);
  h4 {
    margin: 0;
  }
  > span {
    color: var(--ant-color-text-secondary, #8c8c8c);
  }
}
fieldset {
  border: 0;
  padding: 24px 0;
  margin: 0;
  min-width: 0;
}
fieldset:disabled {
  pointer-events: none;
}
.satisfaction-mode {
  margin-bottom: 24px;
  color: #1d2129;
  font-size: 14px;
}
.satisfaction-mode-content {
  max-width: 932px;
}
.satisfaction-mode-selector {
  display: flex;
  flex-wrap: wrap;
  column-gap: 18px;
  row-gap: 8px;
  margin-bottom: 16px;
  :deep(.ant-radio-wrapper) {
    margin: 0;
    color: #1d2129;
    font-size: 14px;
  }
  :deep(.ant-radio-checked .ant-radio-inner) {
    background: #fff;
    border-color: #3b72ff;
  }
  :deep(.ant-radio-inner::after) {
    background: #3b72ff;
  }
}
.satisfaction-mode-panel {
  overflow: hidden;
  border: 1px solid #eceef2;
  border-radius: 8px;
  background: #fff;
}
.satisfaction-option {
  padding: 12px 16px;
}
.satisfaction-option + .satisfaction-option {
  border-top: 1px solid #eceef2;
}
.satisfaction-negative-icon {
  transform: rotate(180deg);
}
.satisfaction-option-rating {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 16px;
}
.satisfaction-option-stars {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: #ff8800;
  font-size: 18px;
}
.satisfaction-option-stars .is-empty {
  color: #e3e5ec;
}
.satisfaction-mode-panel.is-star-mode {
  padding: 8px;
  .satisfaction-option {
    margin-bottom: 12px;
    border: 1px solid #eceef2;
    border-radius: 8px;
  }
  .satisfaction-option-rating {
    width: 120px;
    flex-direction: column;
    gap: 6px;
  }
  .satisfaction-option-icon {
    width: 120px;
    height: 18px;
    flex-basis: auto;
    border-radius: 0;
    background: transparent;
  }
  .satisfaction-option-name {
    max-width: 100%;
    line-height: 20px;
    text-align: center;
  }
  .satisfaction-option-requirements {
    margin-right: 0;
  }
  .satisfaction-resolution-config {
    padding-top: 8px;
  }
}
.satisfaction-option-main {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 40px;
}
.satisfaction-option-icon {
  display: grid;
  place-items: center;
  flex: 0 0 40px;
  width: 40px;
  height: 40px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: #f7f8fa;
  font-size: 24px;
  cursor: pointer;
  &.is-positive {
    color: #ff8800;
  }
  &.is-neutral {
    color: #f5be40;
  }
  &.is-negative {
    color: #a6adb8;
  }
}
.satisfaction-option-name {
  flex-shrink: 0;
  padding: 0;
  border-radius: 0;
  color: #1d2129;
  font-size: 14px;
  line-height: 24px;
}
.satisfaction-option-name:focus {
  box-shadow: 0 1px #3b72ff;
}
.satisfaction-option-tags {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.satisfaction-add-tag {
  height: 26px;
  padding: 0 8px;
  border: 0;
  border-radius: 4px;
  background: #eef4ff;
  color: #3b72ff;
  box-shadow: none;
  font-size: 14px;
  :deep(.anticon) {
    font-size: 12px;
  }
  :deep(.anticon + span) {
    margin-inline-start: 4px;
  }
}
.satisfaction-tag-editor {
  display: inline-flex;
  align-items: center;
  width: 144px;
  border-radius: 4px;
  background: #eef4ff;
  :deep(input) {
    min-width: 0;
    padding: 2px 6px;
    color: #3b72ff;
  }
  button {
    padding: 3px 6px;
    border: 0;
    background: transparent;
    color: #8c95a8;
    cursor: pointer;
  }
}
.satisfaction-option-requirements {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 20px;
  margin-left: auto;
}
.satisfaction-required-control {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  white-space: nowrap;
}
.satisfaction-required-control :deep(.ant-checkbox-wrapper) {
  margin: 0;
  color: #1d2129;
}
.satisfaction-required-control :deep(.ant-checkbox-wrapper > span:last-child) {
  padding-inline-end: 2px;
}
.satisfaction-info {
  display: inline-flex;
  padding: 0;
  border: 0;
  background: transparent;
  color: #8c95a8;
  font-size: 10px;
  cursor: help;
}
.satisfaction-resolution-config {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 20px;
  min-height: 58px;
  padding: 20px 16px 16px;
}
.satisfaction-resolution-config > :deep(.ant-switch) {
  margin-left: 0;
}
.satisfaction-resolution-config > :deep(.ant-checkbox-wrapper) {
  margin-inline-start: 0;
}
.satisfaction-mode-selector :deep(.ant-radio-wrapper > span:last-child) {
  padding-inline-start: 6px;
}
.satisfaction-mode-panel :deep(.ant-checkbox-inner) {
  width: 14px;
  height: 14px;
}
.satisfaction-mode-panel :deep(.ant-checkbox-wrapper > span:last-child) {
  padding-inline-start: 6px;
}
.satisfaction-mode-panel
  :deep(.ant-checkbox-wrapper-disabled > span:last-child) {
  color: #1d2129;
}
.satisfaction-mode-panel :deep(.ant-checkbox-disabled .ant-checkbox-inner) {
  background: #fff;
  border-color: #c9cdd4;
}
.satisfaction-mode-panel :deep(.ant-checkbox-checked .ant-checkbox-inner) {
  background: #3b72ff;
  border-color: #3b72ff;
}
.satisfaction-mode-panel
  :deep(
    .ant-checkbox-disabled.ant-checkbox-checked .ant-checkbox-inner::after
  ) {
  border-color: #fff;
}
.satisfaction-resolution-config :deep(.ant-switch) {
  min-width: 30px;
}
.satisfaction-resolution-config :deep(.ant-switch:not(.ant-switch-checked)) {
  background: #c9cdd4;
}
.satisfaction-sentiment-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 160px;
}
.satisfaction-option-icon:focus-visible,
.satisfaction-info:focus-visible,
.satisfaction-tag-editor button:focus-visible {
  outline: 2px solid #3b72ff;
  outline-offset: 3px;
}
@media (max-width: 1100px) {
  .satisfaction-option-main {
    flex-wrap: wrap;
  }
  .satisfaction-option-requirements {
    flex-basis: 100%;
    margin-left: 56px;
  }
}
@media (max-width: 600px) {
  .satisfaction-option-main {
    gap: 12px;
  }
  .satisfaction-option-tags {
    flex-basis: 100%;
    margin-left: 52px;
  }
  .satisfaction-option-requirements {
    margin-left: 0;
    gap: 12px;
    flex-wrap: wrap;
  }
}
.satisfaction-settings :deep(.ant-input),
.satisfaction-settings :deep(.ant-input-textarea),
.satisfaction-settings :deep(.ant-select) {
  max-width: min(680px, 100%);
}
.satisfaction-settings :deep(.ant-select) {
  min-width: min(200px, 100%);
}
.satisfaction-settings p {
  color: var(--ant-color-text-secondary, #8c8c8c);
  margin-top: 8px;
}
</style>
