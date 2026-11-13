<!-- StatusTransitionModal edits a status and its configured transition fields. -->
<template>
  <a-modal
    :open="open"
    :title="null"
    :footer="null"
    :width="760"
    :closable="false"
    centered
    :destroy-on-close="true"
    class="status-transition-modal"
    @cancel="close"
  >
    <div
      v-if="loading"
      class="transition-loading"
      :style="{ color: token.colorTextTertiary }"
    >
      {{ $t("common_loading") }}
    </div>
    <div
      v-else
      class="transition-layout"
      :style="{
        '--transition-bg': token.colorBgContainer,
        '--transition-pane-bg': token.colorFillQuaternary,
        '--transition-fill-hover': token.colorFillTertiary,
        '--transition-text': token.colorText,
        '--transition-text-secondary': token.colorTextSecondary,
        '--transition-error': token.colorError,
        '--transition-border': token.colorBorder,
        '--transition-primary': token.colorPrimary,
        '--transition-radius': `${token.borderRadius}px`,
      }"
    >
      <aside class="status-pane">
        <button
          v-for="item in statusOptions"
          :key="item.id"
          type="button"
          class="status-option"
          :class="{ active: Number(selectedStatusId) === Number(item.id) }"
          @click="selectedStatusId = Number(item.id)"
        >
          <span class="status-name">{{ displayStatusName(item.name) }}</span>
          <span
            v-if="Number(item.id) === Number(workItem?.status_id)"
            class="current-mark"
            >{{ $t("status_transition_current") }}</span
          >
        </button>
      </aside>
      <section class="field-pane">
        <div class="field-scroll">
          <div
            v-for="field in form.fields"
            :key="field.key"
            class="field-row"
            :class="{
              required: field.required === 1,
              'field-row--comment': field.key === 'comment',
            }"
          >
            <label>
              {{ fieldLabel(field.key) }}
              <span v-if="field.required === 1" class="req">*</span>
            </label>
            <div class="field-control">
              <AssigneeSelect
                v-if="field.key === 'assignee'"
                :value="draft.assignee_ids"
                :members="members"
                size="default"
                :default-open="false"
                @save="(value) => (draft.assignee_ids = value)"
              />
              <a-select
                v-else-if="field.key === 'version'"
                v-model:value="draft.version_id"
                allow-clear
                :placeholder="$t('workspace_field_iteration')"
                :options="versionOptions"
              />
              <a-select
                v-else-if="field.key === 'priority'"
                v-model:value="draft.priority"
                allow-clear
                :placeholder="$t('workspace_field_priority')"
                :options="priorityOptions"
              />
              <PlannedDatePicker
                v-else-if="field.key === 'planned_start_date'"
                v-model:value="draft.planned_start_date"
              />
              <PlannedDatePicker
                v-else-if="field.key === 'planned_end_date'"
                v-model:value="draft.planned_end_date"
              />
              <CommentRichTextInput
                v-else-if="field.key === 'comment'"
                v-model="draft.comment"
                v-model:image-file-ids="draft.image_file_ids"
                class="comment-rich-text-input--modal"
                :ws-id="wsId"
                :min-height="168"
                always-expanded
                full-border
                :placeholder="$t('requirement_addCommentPlaceholder')"
                @submit="confirm"
              />
              <template v-else-if="customFieldOf(field.key)">
                <a-select
                  v-if="
                    customFieldOf(field.key).field_type === 'select_single' ||
                    customFieldOf(field.key).field_type === 'select_multi'
                  "
                  v-model:value="
                    draft.custom_fields[customFieldOf(field.key).id]
                  "
                  :mode="
                    customFieldOf(field.key).field_type === 'select_multi'
                      ? 'multiple'
                      : undefined
                  "
                  allow-clear
                  :options="
                    (customFieldOf(field.key).options || []).map((option) => ({
                      value: option.key,
                      label: option.name,
                    }))
                  "
                />
                <a-input-number
                  v-else-if="customFieldOf(field.key).field_type === 'number'"
                  v-model:value="
                    draft.custom_fields[customFieldOf(field.key).id]
                  "
                />
                <a-date-picker
                  v-else-if="customFieldOf(field.key).field_type === 'date'"
                  v-model:value="
                    draft.custom_fields[customFieldOf(field.key).id]
                  "
                />
                <a-select
                  v-else-if="customFieldOf(field.key).field_type === 'user'"
                  v-model:value="
                    draft.custom_fields[customFieldOf(field.key).id]
                  "
                  allow-clear
                  :options="memberOptions"
                />
                <a-textarea
                  v-else-if="customFieldOf(field.key).field_type === 'textarea'"
                  v-model:value="
                    draft.custom_fields[customFieldOf(field.key).id]
                  "
                  :rows="2"
                />
                <a-input
                  v-else
                  v-model:value="
                    draft.custom_fields[customFieldOf(field.key).id]
                  "
                />
              </template>
            </div>
          </div>
        </div>
        <div class="footer">
          <a-button type="primary" :loading="submitting" @click="confirm">{{
            $t("common_confirm")
          }}</a-button>
          <a-button @click="close">{{ $t("common_cancel") }}</a-button>
        </div>
      </section>
    </div>
  </a-modal>
</template>

<script setup>
import { theme } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import CommentRichTextInput from "@/components/common/CommentRichTextInput.vue";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";
import { useStatusTransitionModal } from "@/composables/useStatusTransitionModal";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
  workItem: { type: Object, default: () => null },
  metadata: { type: Object, default: () => null },
});
const emit = defineEmits({
  "update:open": (value) => typeof value === "boolean",
  success: (payload) => !!payload,
});
const { t: $t } = useI18n();
const { token } = theme.useToken();
const {
  loading,
  submitting,
  selectedStatusId,
  form,
  draft,
  members,
  memberOptions,
  versionOptions,
  priorityOptions,
  statusOptions,
  displayStatusName,
  fieldLabel,
  customFieldOf,
  close,
  confirm,
} = useStatusTransitionModal(props, emit, $t);
</script>

<style scoped lang="less" src="./StatusTransitionModal.less"></style>
