<template>
  <template v-if="!inviteKey">
    <div v-if="loading" class="satisfaction-panel satisfaction-panel-loading">
      <a-spin />
    </div>
    <div v-else-if="failed" class="satisfaction-panel">
      <a-alert type="error" :message="t('common_requestFailed')">
        <template #action
          ><a-button @click="load()">{{ t("issue_retry") }}</a-button></template
        >
      </a-alert>
    </div>
    <section
      v-else-if="
        data && (data.state !== 'unavailable' || presentation === 'detail')
      "
      class="satisfaction-panel"
      :class="{ 'satisfaction-panel-detail': presentation === 'detail' }"
    >
      <h3>
        {{
          t(
            presentation === "detail"
              ? "satisfaction_customer_evaluation"
              : "satisfaction_title",
          )
        }}
      </h3>
      <ResultCard
        v-if="data.state === 'submitted'"
        :data="data"
        :detail="presentation === 'detail'"
      />
      <a-empty v-else :description="t(`satisfaction_${data.state}`)" />
    </section>
  </template>
  <template v-else-if="presentation === 'action'">
    <button
      v-if="failed"
      type="button"
      class="satisfaction-action"
      @click="load()"
    >
      <ReloadOutlined aria-hidden="true" /><span>{{ t("issue_retry") }}</span>
    </button>
    <button
      v-else-if="data && data.state !== 'unavailable'"
      type="button"
      class="satisfaction-action"
      @click="emit('open')"
    >
      <StarOutlined aria-hidden="true" />
      <span>{{ t("satisfaction_evaluation_title") }}</span>
    </button>
  </template>
  <section v-else class="satisfaction-evaluation">
    <header class="satisfaction-evaluation-header">
      <button
        type="button"
        :aria-label="t('common_back')"
        :disabled="saving"
        @click="emit('close')"
      >
        <LeftOutlined aria-hidden="true" />
      </button>
      <h1>{{ t("satisfaction_evaluation_title") }}</h1>
    </header>
    <div v-if="loading" class="satisfaction-evaluation-loading"><a-spin /></div>
    <div v-else-if="failed" class="satisfaction-evaluation-status">
      <a-alert type="error" :message="t('common_requestFailed')" show-icon>
        <template #action
          ><a-button
            :aria-label="t('issue_retry')"
            @click="load(configurationChanged)"
            >{{ t("issue_retry") }}</a-button
          ></template
        >
      </a-alert>
    </div>
    <div
      v-else-if="data?.state === 'submitted'"
      class="satisfaction-evaluation-result"
    >
      <a-alert
        v-if="completed"
        type="success"
        :message="data.rule.completion_text || t('satisfaction_completed')"
        show-icon
      />
      <section class="satisfaction-form-card">
        <h2>{{ t("satisfaction_rating") }}</h2>
        <a-rate
          v-if="data.rule.rating_mode !== 2"
          class="satisfaction-stars"
          :count="data.rule.rating_mode"
          :value="data.value"
          :aria-label="t('satisfaction_rating')"
          disabled
        >
          <template #character="{ index, value }"
            ><StarFilled v-if="index < value" /><StarOutlined v-else
          /></template>
        </a-rate>
        <a-radio-group
          v-else
          class="satisfaction-options satisfaction-readonly-options"
          :value="data.option_key"
          :aria-label="t('satisfaction_rating')"
          disabled
        >
          <a-radio-button
            v-for="option in data.rule.options"
            :key="option.key"
            :value="option.key"
            >{{ option.label }}</a-radio-button
          >
        </a-radio-group>
      </section>
      <section v-if="data.tags.length" class="satisfaction-form-card">
        <h2>{{ t("satisfaction_tags") }}</h2>
        <div class="satisfaction-tags satisfaction-readonly-tags">
          <button
            v-for="tag in data.tags"
            :key="tag.key"
            type="button"
            class="selected"
            aria-pressed="true"
            disabled
          >
            {{ tag.label }}
          </button>
        </div>
      </section>
      <section class="satisfaction-form-card">
        <h2>{{ t("satisfaction_comment") }}</h2>
        <div class="satisfaction-comment">
          <p class="satisfaction-readonly-comment">{{ data.content }}</p>
        </div>
      </section>
      <section
        v-if="data.rule.resolved_enabled"
        class="satisfaction-form-card satisfaction-resolution"
      >
        <h2>{{ t("satisfaction_resolved_question") }}</h2>
        <a-radio-group
          class="satisfaction-readonly-options"
          :value="data.resolved"
          :aria-label="t('satisfaction_resolved_question')"
          size="small"
          disabled
        >
          <a-radio-button :value="true">{{
            t("satisfaction_resolved")
          }}</a-radio-button>
          <a-radio-button :value="false">{{
            t("satisfaction_unresolved")
          }}</a-radio-button>
        </a-radio-group>
      </section>
      <p class="satisfaction-form-hint satisfaction-result-reviewer">
        <a-avatar
          v-if="data.reviewer_avatar"
          :src="data.reviewer_avatar"
          :size="24"
        />
        {{ data.reviewer_name || t("satisfaction_customer") }} ·
        {{ formatLocalDate(data.submitted_at, true) }}
      </p>
      <p class="satisfaction-form-hint">{{ t("satisfaction_readonly") }}</p>
    </div>
    <form
      v-else-if="data?.state === 'available'"
      class="satisfaction-form"
      @submit.prevent="submit"
    >
      <a-alert
        v-if="configurationChanged"
        type="warning"
        :message="t('satisfaction_config_changed')"
        show-icon
      />
      <p v-if="data.rule.invitation_text" class="satisfaction-form-hint">
        {{ data.rule.invitation_text }}
      </p>
      <section class="satisfaction-form-card">
        <h2>
          {{ t("satisfaction_rating")
          }}<span class="satisfaction-required" aria-hidden="true"> *</span>
        </h2>
        <a-rate
          v-if="data.rule.rating_mode !== 2"
          class="satisfaction-stars"
          :count="data.rule.rating_mode"
          :value="selected?.value || 0"
          :tooltips="ratingOptions.map((option) => option.label)"
          :allow-clear="false"
          :allow-half="false"
          :aria-label="t('satisfaction_rating')"
          :disabled="saving"
          @change="selectRating"
        >
          <template #character="{ index, value }"
            ><StarFilled v-if="index < value" /><StarOutlined v-else
          /></template>
        </a-rate>
        <a-radio-group
          v-else
          class="satisfaction-options"
          :value="optionKey"
          :aria-label="t('satisfaction_rating')"
          :disabled="saving"
          @change="selectOption($event.target.value)"
        >
          <a-radio-button
            v-for="option in data.rule.options"
            :key="option.key"
            :value="option.key"
            >{{ option.label }}</a-radio-button
          >
        </a-radio-group>
      </section>
      <section v-if="showTags" class="satisfaction-form-card">
        <h2>
          {{ t("satisfaction_tags")
          }}<span
            v-if="selected?.tags_required"
            class="satisfaction-required"
            aria-hidden="true"
          >
            *</span
          >
        </h2>
        <div v-if="selected" class="satisfaction-tags">
          <button
            v-for="tag in selected.tags"
            :key="tag.key"
            type="button"
            :class="{ selected: tagKeys.includes(tag.key) }"
            :aria-pressed="tagKeys.includes(tag.key)"
            :disabled="saving"
            @click="toggleTag(tag.key)"
          >
            {{ tag.label }}
          </button>
        </div>
        <p v-else class="satisfaction-form-hint">
          {{ t("satisfaction_choose_rating") }}
        </p>
      </section>
      <section class="satisfaction-form-card">
        <h2>
          {{ t("satisfaction_comment")
          }}<span
            v-if="selected?.comment_required"
            class="satisfaction-required"
            aria-hidden="true"
          >
            *</span
          >
        </h2>
        <div class="satisfaction-comment">
          <a-textarea
            v-model:value="content"
            :aria-label="t('satisfaction_comment')"
            :placeholder="t('satisfaction_comment_placeholder')"
            :maxlength="200"
            :bordered="false"
            :disabled="saving"
          />
          <span class="satisfaction-comment-count"
            >{{ content.length }}/200</span
          >
        </div>
      </section>
      <section
        v-if="data.rule.resolved_enabled"
        class="satisfaction-form-card satisfaction-resolution"
      >
        <h2>
          {{ t("satisfaction_resolved_question")
          }}<span
            v-if="data.rule.resolved_required"
            class="satisfaction-required"
            aria-hidden="true"
          >
            *</span
          >
        </h2>
        <a-radio-group
          v-model:value="resolved"
          :aria-label="t('satisfaction_resolved_question')"
          :disabled="saving"
          size="small"
        >
          <a-radio-button :value="true">{{
            t("satisfaction_resolved")
          }}</a-radio-button>
          <a-radio-button :value="false">{{
            t("satisfaction_unresolved")
          }}</a-radio-button>
        </a-radio-group>
      </section>
      <p class="satisfaction-form-hint">
        {{
          t("satisfaction_deadline", {
            time: formatLocalDate(data.expires_at, true),
          })
        }}
      </p>
      <a-alert
        v-if="validationError || submitFailed"
        :type="submitFailed ? 'error' : 'warning'"
        :message="submitFailed ? t('common_requestFailed') : validationError"
        show-icon
      />
      <footer class="satisfaction-form-footer">
        <a-button
          type="primary"
          html-type="submit"
          block
          :loading="saving"
          :aria-label="t('satisfaction_submit')"
          >{{ t("satisfaction_submit") }}</a-button
        >
      </footer>
    </form>
    <div v-else-if="data" class="satisfaction-evaluation-status">
      <a-empty :description="t(`satisfaction_${data.state}`)" />
    </div>
  </section>
</template>
<script setup>
import { computed, ref, watch, onBeforeUnmount } from "vue";
import { useI18n } from "vue-i18n";
import { getSatisfaction, submitSatisfaction } from "@/api/satisfaction";
import { formatLocalDate } from "@/utils/date";
import ResultCard from "./SatisfactionResult.vue";
import {
  LeftOutlined,
  StarOutlined,
  StarFilled,
  ReloadOutlined,
} from "@ant-design/icons-vue";
const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  requirementId: { type: [String, Number], required: true },
  inviteKey: { type: String, default: "" },
  openid: { type: String, default: "" },
  lang: { type: String, default: "" },
  presentation: { type: String, default: "page" },
});
const emit = defineEmits(["open", "close"]);
const { t } = useI18n();
const data = ref(null),
  failed = ref(false),
  loading = ref(false),
  submitFailed = ref(false),
  configurationChanged = ref(false),
  saving = ref(false),
  completed = ref(false),
  optionKey = ref(""),
  tagKeys = ref([]),
  content = ref(""),
  resolved = ref(null),
  validationError = ref("");
let sequence = 0;
const selected = computed(() =>
  data.value?.rule.options.find((option) => option.key === optionKey.value),
);
const ratingOptions = computed(() =>
  [...(data.value?.rule.options || [])].sort((a, b) => a.value - b.value),
);
const showTags = computed(() =>
  selected.value
    ? selected.value.tags.length > 0
    : data.value?.rule.options.some((option) => option.tags.length),
);
function selectOption(key) {
  optionKey.value = key;
  tagKeys.value = [];
  validationError.value = "";
}
function selectRating(value) {
  const option = ratingOptions.value.find((item) => item.value === value);
  if (option) selectOption(option.key);
}
function toggleTag(key) {
  tagKeys.value = tagKeys.value.includes(key)
    ? tagKeys.value.filter((value) => value !== key)
    : [...tagKeys.value, key];
}
async function load(preserveDraft = false) {
  const seq = ++sequence;
  data.value = null;
  failed.value = false;
  loading.value = false;
  saving.value = false;
  submitFailed.value = false;
  if (!preserveDraft) {
    configurationChanged.value = false;
    content.value = "";
  }
  completed.value = false;
  optionKey.value = "";
  tagKeys.value = [];
  resolved.value = null;
  validationError.value = "";
  if (!props.requirementId || (props.inviteKey && !props.openid)) return;
  loading.value = true;
  try {
    const r = await getSatisfaction(
      props.wsId,
      props.requirementId,
      props.inviteKey,
      props.openid,
      props.lang,
    );
    if (seq !== sequence) return;
    data.value = r;
  } catch {
    if (seq === sequence) failed.value = true;
  } finally {
    if (seq === sequence) loading.value = false;
  }
}
async function submit() {
  if (saving.value || data.value?.state !== "available") return;
  validationError.value = "";
  submitFailed.value = false;
  const option = selected.value;
  if (
    !option ||
    (option.tags_required && !tagKeys.value.length) ||
    (option.comment_required && !content.value.trim()) ||
    (data.value.rule.resolved_required && resolved.value === null)
  ) {
    validationError.value = t("satisfaction_required_fields");
    return;
  }
  saving.value = true;
  const seq = sequence;
  try {
    const r = await submitSatisfaction(
      props.inviteKey,
      props.requirementId,
      props.openid,
      props.lang,
      {
        expected_config_version: data.value.config_version,
        option_key: optionKey.value,
        tag_keys: tagKeys.value,
        content: content.value,
        resolved: data.value.rule.resolved_enabled ? resolved.value : null,
      },
    );
    if (seq === sequence) {
      data.value = r;
      completed.value = true;
    }
  } catch (error) {
    if (seq !== sequence) return;
    if (error.response?.status === 409) {
      configurationChanged.value = true;
      await load(true);
    } else submitFailed.value = true;
  } finally {
    if (seq === sequence) saving.value = false;
  }
}
watch(
  () => [
    props.wsId,
    props.requirementId,
    props.inviteKey,
    props.openid,
    props.lang,
  ],
  () => load(),
  { immediate: true },
);
onBeforeUnmount(() => sequence++);
</script>

<style scoped lang="less">
.satisfaction-panel {
  margin: 16px 0;
}
.satisfaction-panel-loading {
  display: grid;
  place-items: center;
  min-height: 120px;
}
.satisfaction-panel-detail {
  margin: 16px 0 24px;
}
.satisfaction-panel-detail h3 {
  margin: 0 0 12px;
  font-size: 14px;
  line-height: 22px;
  font-weight: 600;
  color: #1d2129;
}
.satisfaction-action {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 2px 4px;
  border: 0;
  background: transparent;
  cursor: pointer;
  :deep(.anticon) {
    width: 20px;
    height: 20px;
    font-size: 20px;
    color: #86909c;
  }
  > span:last-child {
    color: #1d2129;
    font-size: 13px;
    line-height: 20px;
    text-align: center;
    overflow-wrap: anywhere;
  }
}
.satisfaction-evaluation {
  min-height: 100dvh;
  background: #f2f3f5;
  color: #1d2129;
}
.satisfaction-evaluation-header {
  position: sticky;
  top: 0;
  z-index: 25;
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr) 44px;
  align-items: center;
  height: 44px;
  border-bottom: 1px solid #eceef2;
  background: #fff;
  h1 {
    grid-column: 2;
    margin: 0;
    text-align: center;
    font-size: 16px;
    font-weight: 600;
  }
  button {
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border: 0;
    background: transparent;
    color: #1d2129;
    font-size: 17px;
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}
.satisfaction-evaluation-loading,
.satisfaction-evaluation-status {
  display: grid;
  place-content: center;
  min-height: calc(100dvh - 44px);
  padding: 16px;
}
.satisfaction-evaluation-result,
.satisfaction-form {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: calc(100dvh - 44px);
  padding: 12px 14px calc(30px + env(safe-area-inset-bottom, 0px));
}
.satisfaction-form-card {
  padding: 16px;
  border-radius: 9px;
  background: #fff;
  h2 {
    margin: 0 0 12px;
    font-size: 15px;
    line-height: 22px;
    font-weight: 600;
  }
}
.satisfaction-required {
  color: #f56c6c;
}
.satisfaction-stars {
  display: block;
  margin-top: 4px;
  font-size: 32px;
  color: #ff981a;
  :deep(.ant-rate-star:not(:last-child)) {
    margin-right: 16px;
  }
  :deep(.ant-rate-star-zero .anticon) {
    color: #d4d8e1;
  }
}
.satisfaction-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.satisfaction-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  button {
    padding: 6px 13px;
    border: 1px solid #d4dbe6;
    border-radius: 20px;
    background: #fff;
    color: #254f85;
    font-size: 12px;
    line-height: 16px;
    cursor: pointer;
  }
  button.selected {
    border-color: #4679ff;
    color: #3157e2;
    background: #edf3ff;
  }
}
.satisfaction-comment {
  position: relative;
  border-radius: 8px;
  background: #f5f6f8;
  :deep(textarea) {
    display: block;
    height: 190px;
    padding: 12px 12px 34px;
    resize: vertical;
    background: transparent;
    font-size: 14px;
    line-height: 22px;
  }
  :deep(textarea::placeholder) {
    color: #a9b1bf;
  }
}
.satisfaction-comment-count {
  position: absolute;
  right: 12px;
  bottom: 12px;
  color: #a9b1bf;
  font-size: 12px;
  pointer-events: none;
}
.satisfaction-readonly-comment {
  min-height: 190px;
  margin: 0;
  padding: 12px;
  font-size: 14px;
  line-height: 22px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.satisfaction-readonly-options {
  :deep(.ant-radio-button-wrapper-disabled) {
    color: var(--ant-color-text, rgba(0, 0, 0, 0.88));
    background: #fff;
    cursor: default;
  }
  :deep(.ant-radio-button-wrapper-disabled.ant-radio-button-wrapper-checked) {
    border-color: var(--ant-color-primary, #3157e2);
    color: var(--ant-color-primary, #3157e2);
  }
  :deep(
    .ant-radio-button-wrapper-disabled.ant-radio-button-wrapper-checked::before
  ) {
    background: var(--ant-color-primary, #3157e2);
  }
}
.satisfaction-result-reviewer {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}
.satisfaction-readonly-tags button {
  cursor: default;
}
.satisfaction-resolution {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  h2 {
    flex: 1;
    min-width: 120px;
    margin: 0;
    font-size: 13px;
    font-weight: 400;
  }
}
.satisfaction-form-hint {
  margin: 0;
  color: #8c95a8;
  font-size: 12px;
  line-height: 18px;
  white-space: pre-wrap;
}
.satisfaction-form-footer {
  margin-top: auto;
  padding-top: 16px;
  :deep(.ant-btn) {
    height: 48px;
    border-color: #4679ff;
    border-radius: 25px;
    background: #4679ff;
    box-shadow: none;
    font-size: 15px;
  }
}
.satisfaction-action:focus-visible,
.satisfaction-evaluation-header button:focus-visible,
.satisfaction-tags button:focus-visible {
  outline: 2px solid #4679ff;
  outline-offset: 3px;
}
@media (max-width: 360px) {
  .satisfaction-stars :deep(.ant-rate-star:not(:last-child)) {
    margin-right: 10px;
  }
}
</style>
