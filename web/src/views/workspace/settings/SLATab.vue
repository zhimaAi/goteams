<template>
  <a-spin :spinning="loading">
    <a-alert
      v-if="loadFailed"
      type="error"
      :message="$t('sla_loadFailed')"
      show-icon
      ><template #action
        ><a-button size="small" @click="load">{{
          $t("issue_retry")
        }}</a-button></template
      ></a-alert
    >
    <div v-else class="sla-settings">
      <div class="sla-header">
        <div class="sla-heading">
          <span class="sla-title">{{ $t("sla_title") }}</span>
          <span class="sla-introduction">{{ $t("sla_introduction") }}</span>
        </div>
        <div class="sla-master-switch">
          <span id="sla-enabled-label">{{ $t("sla_enableFeature") }}</span>
          <a-switch
            :checked="settings.enabled"
            :checked-children="$t('sla_on')"
            :un-checked-children="$t('sla_off')"
            :disabled="!canManage || savingSettings"
            :loading="savingSettings"
            aria-labelledby="sla-enabled-label"
            @change="toggleEnabled"
          />
        </div>
      </div>
      <div class="sla-content">
        <a-tabs v-model:active-key="tab" class="sla-tabs" size="small">
          <a-tab-pane key="policies" :tab="$t('sla_settings')" />
          <a-tab-pane key="stages" :tab="$t('sla_stages')" />
        </a-tabs>
        <a-alert
          v-if="!settings.enabled"
          class="sla-disabled"
          type="warning"
          :message="$t('sla_disabledHint')"
          show-icon
        />
        <template v-if="tab === 'policies'">
          <div class="sla-toolbar">
            <a-button v-if="canManage" type="primary" @click="createPolicy">
              <template #icon><PlusOutlined /></template
              >{{ $t("sla_newPolicy") }}
            </a-button>
          </div>
          <a-table
            class="sla-table"
            size="middle"
            :data-source="policies"
            :columns="policyColumns"
            :pagination="false"
            row-key="id"
          >
            <template #bodyCell="{ column, record, index }">
              <div v-if="column.key === 'name'" class="sla-policy-name">
                <FileTextOutlined /><span>{{ record.name }}</span>
              </div>
              <template v-else-if="column.key === 'conditions'">
                <div v-if="record.priority_ids.length">
                  <span class="sla-condition-label"
                    >{{ $t("sla_priority") }}：</span
                  >{{
                    conditionNames(record.priority_ids, metadata.priorities)
                  }}
                </div>
                <div v-if="record.category_ids.length">
                  <span class="sla-condition-label"
                    >{{ $t("sla_category") }}：</span
                  >{{
                    conditionNames(record.category_ids, metadata.categories)
                  }}
                </div>
                <span
                  v-if="
                    !record.priority_ids.length && !record.category_ids.length
                  "
                  >{{ $t("sla_any") }}</span
                >
                <a-tag v-if="policyIncomplete(record)" color="orange">{{
                  $t("sla_configurationIncomplete")
                }}</a-tag>
              </template>
              <a-switch
                v-else-if="column.key === 'enabled'"
                :checked="record.enabled"
                :checked-children="$t('sla_on')"
                :un-checked-children="$t('sla_off')"
                :aria-label="$t('sla_policyEnabled', { name: record.name })"
                :disabled="!canManage || saving || sorting || !!policySaving"
                :loading="policySaving === record.id"
                @change="togglePolicy(record, $event).catch(() => {})"
              />
              <a-space v-else-if="column.key === 'actions'" :size="16">
                <a-button
                  type="link"
                  class="sla-text-action"
                  @click="editPolicy(record)"
                  >{{ $t(canManage ? "common_edit" : "sla_view") }}</a-button
                >
                <a-popconfirm
                  v-if="canManage"
                  :title="$t('sla_archiveConfirm')"
                  @confirm="archive('policies', record)"
                  ><a-button
                    type="link"
                    danger
                    class="sla-text-action"
                    :disabled="saving || !!policySaving"
                    >{{ $t("common_delete") }}</a-button
                  ></a-popconfirm
                >
                <a-dropdown v-if="canManage" :trigger="['click']">
                  <a-button
                    type="text"
                    class="sla-order-action"
                    :aria-label="$t('sla_matchOrder')"
                    :disabled="sorting || !!policySaving"
                    ><MoreOutlined
                  /></a-button>
                  <template #overlay
                    ><a-menu>
                      <a-menu-item
                        :disabled="index === 0 || sorting"
                        @click="movePolicy(index, -1)"
                        >{{ $t("sla_moveUp") }}</a-menu-item
                      >
                      <a-menu-item
                        :disabled="index === policies.length - 1 || sorting"
                        @click="movePolicy(index, 1)"
                        >{{ $t("sla_moveDown") }}</a-menu-item
                      >
                    </a-menu></template
                  >
                </a-dropdown>
              </a-space>
            </template>
          </a-table>
        </template>
        <template v-else>
          <div class="sla-toolbar">
            <a-button v-if="canManage" type="primary" @click="editStage()"
              ><template #icon><PlusOutlined /></template
              >{{ $t("sla_createStage") }}</a-button
            >
          </div>
          <a-table
            class="sla-table"
            size="middle"
            :data-source="stages"
            :columns="stageColumns"
            :pagination="false"
            :scroll="{ x: 680 }"
            row-key="id"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'transition'">{{
                transitionText(record.config)
              }}</template>
              <a-space v-else-if="column.key === 'actions'" :size="16">
                <a-button
                  type="link"
                  class="sla-text-action"
                  @click="editStage(record)"
                  >{{ $t(canManage ? "common_edit" : "sla_view") }}</a-button
                >
                <a-popconfirm
                  v-if="canManage"
                  :title="$t('sla_archiveConfirm')"
                  @confirm="archive('stages', record)"
                  ><a-button
                    type="link"
                    danger
                    class="sla-text-action"
                    :disabled="saving"
                    >{{ $t("common_delete") }}</a-button
                  ></a-popconfirm
                >
              </a-space>
            </template>
          </a-table>
        </template>
      </div>

      <a-modal
        v-model:open="stageOpen"
        :title="$t(stageDraft.id ? 'sla_editStage' : 'sla_addStage')"
        :width="520"
        wrap-class-name="sla-stage-modal"
        centered
        destroy-on-close
        :mask-closable="!saving"
        :keyboard="!saving"
        :closable="!saving"
        :confirm-loading="saving"
        :cancel-button-props="{ disabled: saving }"
        :ok-text="$t('common_confirm')"
        :cancel-text="$t('common_cancel')"
        :footer="canManage ? undefined : null"
        @ok="submitStage"
      >
        <a-form
          class="sla-stage-form"
          layout="vertical"
          :disabled="!canManage || saving"
        >
          <a-form-item
            :label="$t('sla_name')"
            :validate-status="
              stageSubmitted && !stageDraft.name.trim() ? 'error' : undefined
            "
            :help="
              stageSubmitted && !stageDraft.name.trim()
                ? $t('sla_stageNameRequired')
                : undefined
            "
            required
          >
            <a-input
              v-model:value="stageDraft.name"
              :maxlength="100"
              :placeholder="$t('sla_stageNamePlaceholder')"
            />
          </a-form-item>
          <div class="sla-stage-boundary">
            <div class="sla-stage-label">{{ $t("sla_statusChange") }}</div>
            <div class="sla-transition">
              <span class="sla-transition-word">{{ $t("sla_from") }}</span>
              <template v-for="side in ['from', 'to']" :key="side">
                <span v-if="side === 'to'" class="sla-transition-word">{{
                  $t("sla_transitionTo")
                }}</span>
                <a-form-item
                  :validate-status="
                    stageSubmitted &&
                    (stageSelectorMissing(stageDraft.config[side]) ||
                      stageStatusesOverlap)
                      ? 'error'
                      : undefined
                  "
                  :help="
                    stageSubmitted &&
                    stageSelectorMissing(stageDraft.config[side])
                      ? $t('sla_stageStatusRequired')
                      : stageSubmitted && stageStatusesOverlap
                        ? $t('sla_stageStatusesOverlap')
                        : undefined
                  "
                >
                  <a-select
                    :value="
                      stageDraft.config[side].mode === 'any'
                        ? ['any']
                        : stageDraft.config[side].status_ids
                    "
                    mode="multiple"
                    :options="stageStatusOptions"
                    max-tag-count="responsive"
                    :placeholder="$t('sla_stageStatusPlaceholder')"
                    :aria-label="$t(side === 'from' ? 'sla_from' : 'sla_to')"
                    option-filter-prop="label"
                    show-arrow
                    @change="updateStageStatuses(side, $event)"
                  />
                </a-form-item>
              </template>
            </div>
          </div>
        </a-form>
      </a-modal>

      <a-modal
        v-model:open="webhookOpen"
        :title="$t('sla_notificationMethod')"
        :width="720"
        class="sla-webhook-modal"
        :footer="canManage ? undefined : null"
        @ok="saveWebhooks"
      >
        <a-form
          layout="vertical"
          :disabled="!canManage"
          class="sla-webhook-form"
        >
          <div class="sla-webhook-platforms">
            <a-button
              v-for="hook in webhookDrafts"
              :key="hook.platform"
              :class="{ 'is-selected': hook.enabled }"
              :aria-pressed="hook.enabled"
              @click="hook.enabled = !hook.enabled"
              >{{ $t(`external_notification_${hook.platform}`) }}</a-button
            >
          </div>
          <section
            v-for="hook in webhookDrafts.filter((item) => item.enabled)"
            :key="hook.platform"
            class="sla-webhook-config"
          >
            <h3>
              {{ $t(`external_notification_${hook.platform}`) }}
              {{ $t("external_notification_webhook_title") }}
            </h3>
            <a-form-item
              :label="$t('external_notification_webhook_url')"
              required
            >
              <a-input
                v-model:value="hook.url"
                :placeholder="webhookPlaceholders[hook.platform]"
              />
            </a-form-item>
            <a-form-item
              v-if="hook.platform !== 'wecom'"
              :label="$t('external_notification_' + hook.platform + '_secret')"
            >
              <a-input-password
                v-model:value="hook.secret"
                :placeholder="$t('external_notification_secret_placeholder')"
              />
            </a-form-item>
            <a-form-item :label="$t('automation_atMembers')">
              <a-button @click="openWebhookAt(hook)">{{
                $t("automation_selectAtMembers")
              }}</a-button>
              <div class="sla-webhook-at-tags">
                <a-tag v-if="hook.at_all">{{ $t("automation_atAll") }}</a-tag>
                <a-tag v-for="id in hook.at_user_ids" :key="id">{{
                  webhookMembers.find((member) => member.value === id)?.label ||
                  id
                }}</a-tag>
                <a-tag v-for="role in hook.at_roles" :key="role">{{
                  $t(
                    role === "created_by"
                      ? "automation_atRoleCreatedBy"
                      : "automation_atRoleAssignee",
                  )
                }}</a-tag>
              </div>
            </a-form-item>
            <a-form-item :label="$t('automation_atMode')">
              <a-radio-group v-model:value="hook.at_mode" button-style="solid">
                <a-radio-button
                  v-for="mode in webhookModes[hook.platform]"
                  :key="mode"
                  :value="mode"
                  >{{
                    mode === "mobile"
                      ? $t("automation_atModeMobile")
                      : robotAtModeLabels[mode]
                  }}</a-radio-button
                >
              </a-radio-group>
              <p class="sla-webhook-tip">
                {{ $t("automation_paramMappingTip") }}
                <a-button
                  v-if="canManage && auth.hasPermission('automation:manage')"
                  type="link"
                  size="small"
                  :disabled="!webhookMemberRows.length"
                  @click="mappingModalOpen = true"
                  >{{ $t("automation_paramMapping") }}</a-button
                >
              </p>
            </a-form-item>
          </section>
        </a-form>
        <RobotMentionModal
          v-model:open="webhookAtOpen"
          :value="webhookAtTarget || {}"
          :members="webhookMembers"
          @confirm="confirmWebhookAt"
        />
      </a-modal>
      <RobotUserMappingModal
        v-model:open="mappingModalOpen"
        :ws-id="wsId"
        :members="webhookMemberRows"
      />
      <a-drawer
        v-model:open="policyOpen"
        :title="$t('sla_serviceGoals')"
        root-class-name="sla-policy-drawer"
        :width="'min(100vw, 1280px)'"
        :mask-closable="!saving"
        :closable="!saving"
        :keyboard="!saving"
      >
        <a-form
          class="sla-policy-form"
          layout="horizontal"
          :colon="false"
          :disabled="!canManage || saving"
        >
          <div class="sla-policy-basics">
            <a-form-item :label="$t('sla_name')" required>
              <a-input
                v-model:value="policyDraft.name"
                :maxlength="100"
                :placeholder="$t('sla_policyNamePlaceholder')"
                class="sla-name-input"
              />
            </a-form-item>
            <a-form-item :label="$t('sla_description')">
              <a-textarea
                v-model:value="policyDraft.description"
                :maxlength="2000"
                :rows="3"
                :placeholder="$t('sla_descriptionPlaceholder')"
              />
            </a-form-item>
          </div>

          <section
            class="sla-policy-section"
            aria-labelledby="sla-scope-heading"
          >
            <h3 id="sla-scope-heading">
              {{ $t("sla_scope") }}
              <a-tooltip :title="$t('sla_conditionHint')"
                ><InfoCircleOutlined
                  tabindex="0"
                  :aria-label="$t('sla_conditionHint')"
              /></a-tooltip>
            </h3>
            <a-form-item :label="$t('sla_priority')">
              <a-select
                v-model:value="policyDraft.priority_ids"
                class="sla-priority-select"
                mode="multiple"
                :options="priorityOptions"
                :placeholder="$t('sla_any')"
                option-filter-prop="label"
                allow-clear
              />
            </a-form-item>
            <a-form-item :label="$t('sla_category')">
              <a-select
                v-model:value="policyDraft.category_ids"
                class="sla-category-select"
                mode="multiple"
                :options="categoryOptions"
                :placeholder="$t('sla_any')"
                option-filter-prop="label"
              />
            </a-form-item>
          </section>

          <section
            class="sla-policy-section"
            aria-labelledby="sla-clock-heading"
          >
            <h3 id="sla-clock-heading">{{ $t("sla_clock") }}</h3>
            <a-radio-group
              v-model:value="policyDraft.config.calendar.mode"
              class="sla-clock-options"
            >
              <a-radio value="weekly">{{ $t("sla_weekly") }}</a-radio>
              <div
                v-if="policyDraft.config.calendar.mode === 'weekly'"
                class="sla-slots"
              >
                <div
                  v-for="(slot, index) in slots"
                  :key="index"
                  class="sla-slot"
                >
                  <a-checkbox-group
                    v-model:value="slot.weekdays"
                    :options="weekdays"
                    :aria-label="$t('sla_weekly')"
                  />
                  <div class="sla-slot-time">
                    <a-time-range-picker
                      :value="[
                        slot.start,
                        slot.end === '24:00' ? null : slot.end,
                      ]"
                      class="sla-time-range-picker"
                      format="HH:mm"
                      value-format="HH:mm"
                      :allow-clear="false"
                      :order="false"
                      :allow-empty="[false, slot.end === '24:00']"
                      :disabled="[
                        !canManage || saving,
                        !canManage || saving || slot.end === '24:00',
                      ]"
                      :placeholder="[
                        $t('sla_startTime'),
                        slot.end === '24:00' ? '24:00' : $t('sla_endTime'),
                      ]"
                      @update:value="updateSlotRange(slot, $event)"
                    />
                    <a-checkbox
                      :checked="slot.end === '24:00'"
                      @change="
                        slot.end = $event.target.checked ? '24:00' : '18:00'
                      "
                      >{{ $t("sla_endOfDay") }}</a-checkbox
                    >
                    <a-button
                      v-if="canManage"
                      class="sla-remove-button"
                      danger
                      type="text"
                      :disabled="slots.length === 1"
                      :aria-label="$t('sla_removeSlot')"
                      @click="slots.splice(index, 1)"
                      ><MinusOutlined
                    /></a-button>
                  </div>
                </div>
                <a-button
                  v-if="canManage"
                  class="sla-add-button"
                  type="text"
                  :aria-label="$t('sla_addSlot')"
                  :title="$t('sla_addSlot')"
                  @click="slots.push(defaultSlot())"
                  ><PlusOutlined
                /></a-button>
              </div>
              <a-radio value="all_day">{{ $t("sla_allDay") }}</a-radio>
            </a-radio-group>
          </section>

          <section
            class="sla-policy-section"
            aria-labelledby="sla-pause-heading"
          >
            <h3 id="sla-pause-heading">{{ $t("sla_pauseTime") }}</h3>
            <a-checkbox
              v-model:checked="policyDraft.config.calendar.pause_on_holidays"
              >{{ $t("sla_pauseHolidays") }}</a-checkbox
            >
            <a-alert
              v-if="
                policyDraft.config.calendar.pause_on_holidays &&
                !settings.calendar_available
              "
              class="sla-hint"
              type="warning"
              :message="$t('sla_calendarMissing')"
              show-icon
            />
          </section>

          <section
            class="sla-policy-section"
            aria-labelledby="sla-targets-heading"
          >
            <h3 id="sla-targets-heading">{{ $t("sla_targets") }}</h3>
            <a-form-item :label="$t('sla_selectStages')" required>
              <a-select
                v-model:value="selectedStages"
                mode="multiple"
                :options="stageOptions"
              />
            </a-form-item>
            <div class="sla-target-scroll">
              <table class="sla-targets">
                <thead>
                  <tr>
                    <th scope="col">{{ $t("sla_priority") }}</th>
                    <th
                      v-for="stage in targetStages"
                      :key="stage.id"
                      scope="col"
                    >
                      {{ stage.name }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="priority in targetPriorities" :key="priority.id">
                    <th scope="row">
                      <span
                        class="sla-priority-tag"
                        :style="{
                          '--priority-color': priority.color || undefined,
                        }"
                        >{{ priority.name }}</span
                      >
                    </th>
                    <td v-for="stage in targetStages" :key="stage.id">
                      <div class="sla-duration">
                        <a-input-number
                          v-model:value="
                            targetCell(priority.id, stage.id).value
                          "
                          :min="1"
                          :precision="0"
                          :aria-label="`${priority.name} ${stage.name}`"
                        />
                        <a-select
                          v-model:value="targetCell(priority.id, stage.id).unit"
                          :options="unitOptions"
                          :aria-label="`${priority.name} ${stage.name} ${$t('sla_durationUnit')}`"
                        />
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section
            class="sla-policy-section"
            aria-labelledby="sla-actions-heading"
          >
            <h3 id="sla-actions-heading">
              {{ $t("sla_executionActions") }}
              <span class="sla-optional">{{ $t("sla_optional") }}</span>
              <a-tooltip
                :title="$t('sla_notificationHint')"
                :overlay-style="{ maxWidth: '560px' }"
                :overlay-inner-style="{ whiteSpace: 'pre-line' }"
                ><InfoCircleOutlined
                  tabindex="0"
                  :aria-label="$t('sla_notificationHint')"
              /></a-tooltip>
            </h3>
            <div
              v-for="(action, index) in actionDrafts"
              :key="action.key"
              class="sla-action"
            >
              <a-select
                v-model:value="action.stage_id"
                :options="
                  targetStages.map((item) => ({
                    value: item.id,
                    label: item.name,
                  }))
                "
                :placeholder="$t('sla_selectStage')"
                :aria-label="$t('sla_selectStage')"
              />
              <a-select
                v-model:value="action.position"
                :options="positionOptions"
                :aria-label="$t('sla_actionTiming')"
              />
              <a-input-number
                v-model:value="action.value"
                :min="0"
                :precision="0"
                :aria-label="$t('sla_offset')"
              />
              <a-select
                v-model:value="action.unit"
                :options="unitOptions"
                :aria-label="$t('sla_durationUnit')"
              />
              <a-select
                v-model:value="action.channel"
                :aria-label="$t('sla_notificationMethod')"
                :options="[
                  {
                    value: 'notification',
                    label: $t('sla_notificationAction'),
                  },
                  { value: 'webhook', label: $t('sla_webhookAction') },
                ]"
              />
              <div
                v-if="action.channel === 'webhook'"
                class="sla-webhook-summary"
              >
                <a-button type="link" @click="openWebhooks(action)">{{
                  $t("sla_webhookSettings")
                }}</a-button>
                <a-tag v-for="hook in action.webhooks" :key="hook.platform">
                  {{ $t(`external_notification_${hook.platform}`) }}
                </a-tag>
              </div>
              <a-select
                v-if="action.channel !== 'webhook'"
                v-model:value="action.recipients"
                mode="multiple"
                :max-tag-count="'responsive'"
                :options="recipientOptions"
                :placeholder="$t('sla_recipient')"
                :aria-label="$t('sla_recipient')"
              />
              <a-button
                v-if="canManage"
                class="sla-remove-button"
                danger
                type="text"
                :aria-label="$t('sla_removeAction')"
                @click="actionDrafts.splice(index, 1)"
                ><MinusOutlined
              /></a-button>
            </div>
            <a-button
              v-if="canManage"
              class="sla-add-button"
              type="text"
              :aria-label="$t('sla_addAction')"
              :title="$t('sla_addAction')"
              @click="addAction"
              ><PlusOutlined
            /></a-button>
          </section>
        </a-form>
        <template #footer>
          <div class="sla-policy-footer">
            <a-button :disabled="saving" @click="policyOpen = false">{{
              $t("common_cancel")
            }}</a-button>
            <a-button
              v-if="canManage"
              type="primary"
              :loading="saving"
              @click="submitPolicy"
              >{{ $t("common_save") }}</a-button
            >
          </div>
        </template>
      </a-drawer>
    </div>
  </a-spin>
</template>

<script setup>
import RobotMentionModal from "@/components/notification/RobotMentionModal.vue";
import {
  robotAtModes as webhookModes,
  robotAtModeLabels,
  robotUrlPlaceholders as webhookPlaceholders,
} from "@/constants/robot";
import RobotUserMappingModal from "@/components/notification/RobotUserMappingModal.vue";
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  FileTextOutlined,
  InfoCircleOutlined,
  MinusOutlined,
  PlusOutlined,
  SettingOutlined,
  MoreOutlined,
} from "@ant-design/icons-vue";
import { useAuthStore } from "@/stores/auth";
import {
  getSLASettings,
  saveSLASettings,
  getSLAMetadata,
  listSLAStages,
  listSLAPolicies,
  saveSLAStage,
  saveSLAPolicy,
  saveSLAPolicyOrder,
  archiveSLADefinition,
} from "@/api/sla";

const webhookOpen = ref(false),
  webhookDrafts = ref([]),
  webhookMembers = ref([]);
const mappingModalOpen = ref(false);
const webhookMemberRows = ref([]);
let webhookAction;
const webhookAtOpen = ref(false);
const webhookAtTarget = ref(null);
function openWebhookAt(hook) {
  webhookAtTarget.value = hook;
  webhookAtOpen.value = true;
}
function confirmWebhookAt(value) {
  if (!canManage.value) return;
  Object.assign(webhookAtTarget.value, value);
}
async function openWebhooks(action) {
  webhookAtOpen.value = false;
  webhookAction = action;
  webhookMembers.value = [];
  webhookMemberRows.value = [];
  webhookDrafts.value = ["feishu", "wecom", "dingtalk"].map((platform) => ({
    platform,
    enabled: action.webhooks.some((h) => h.platform === platform),
    url: "",
    secret: "",
    at_mode: webhookModes[platform][0],
    at_user_ids: [],
    at_roles: [],
    at_all: false,
    ...clone(action.webhooks.find((h) => h.platform === platform) || {}),
  }));
  webhookOpen.value = true;
  const ws = props.wsId;
  try {
    const { listMembers } = await import("@/api/workspace");
    const rows = await listMembers(ws);
    if (ws === props.wsId) {
      webhookMemberRows.value = rows;
      webhookMembers.value = rows.map((row) => ({
        value: Number(row.user_id),
        label: row.user_name || row.user_username,
      }));
    }
  } catch {}
}
function saveWebhooks() {
  if (!canManage.value) return;
  const hooks = webhookDrafts.value.filter((h) => h.enabled);
  if (
    !hooks.length ||
    hooks.some((h) => {
      try {
        return !["http:", "https:"].includes(new URL(h.url).protocol);
      } catch {
        return true;
      }
    })
  ) {
    message.warning(t("sla_webhookRequired"));
    return;
  }
  webhookAction.webhooks = hooks.map(({ enabled, ...hook }) => hook);
  webhookOpen.value = false;
}
const clone = (value) => JSON.parse(JSON.stringify(value));
const props = defineProps({ wsId: { type: [String, Number], required: true } });
const { t } = useI18n();
const auth = useAuthStore();
const canManage = computed(() => auth.hasPermission("sla:manage"));
const loadFailed = ref(false);
const loading = ref(false),
  saving = ref(false),
  savingSettings = ref(false),
  sorting = ref(false),
  policySaving = ref(0);
const tab = ref("policies"),
  stageOpen = ref(false),
  stageSubmitted = ref(false),
  policyOpen = ref(false);
const settings = reactive({
  enabled: true,
  timezone: "",
  revision: 0,
  calendar_available: false,
});
const metadata = reactive({
  statuses: [],
  priorities: [],
  categories: [],
  roles: [],
});
const stages = ref([]),
  policies = ref([]),
  selectedStages = ref([]),
  slots = ref([]),
  actionDrafts = ref([]);
const stageDraft = reactive({ config: { from: {}, to: {} } });
const policyDraft = reactive({
  config: { calendar: {} },
  priority_ids: [],
  category_ids: [],
});
const targetEdits = reactive({});
let loadSequence = 0;
const unitOptions = computed(() => [
  { value: "minutes", label: t("sla_minutes") },
  { value: "hours", label: t("sla_hours") },
]);
const stageStatusOptions = computed(() => [
  { value: "any", label: t("sla_selector_any") },
  ...metadata.statuses.map((item) => ({ value: item.id, label: item.name })),
]);
function updateStageStatuses(side, values) {
  const selector = stageDraft.config[side];
  const any = selector.mode !== "any" && values.includes("any");
  selector.mode = any ? "any" : "selected";
  selector.status_ids = any ? [] : values.filter((value) => value !== "any");
}
function stageSelectorMissing(selector) {
  return selector.mode !== "any" && !selector.status_ids.length;
}
const stageStatusesOverlap = computed(() => {
  const { from, to } = stageDraft.config;
  return (
    from.mode === "selected" &&
    to.mode === "selected" &&
    from.status_ids.some((id) => to.status_ids.includes(id))
  );
});
const priorityOptions = computed(() => [
  { value: 0, label: t("sla_unspecified") },
  ...metadata.priorities.map((item) => ({
    value: item.id,
    label: item.name,
  })),
]);
const categoryOptions = computed(() => [
  { value: 0, label: t("sla_uncategorized") },
  ...metadata.categories.map((item) => ({
    value: item.id,
    label: categoryPath(item),
  })),
]);
const stageOptions = computed(() =>
  stages.value.map((item) => ({ value: item.id, label: item.name })),
);
const weekdays = computed(() =>
  Array.from({ length: 7 }, (_, index) => ({
    value: index + 1,
    label: t(`sla_weekday_${index + 1}`),
  })),
);
const positionOptions = computed(() => [
  { value: "before_deadline", label: t("sla_beforeDeadline") },
  { value: "after_deadline", label: t("sla_afterDeadline") },
]);
const recipientOptions = computed(() => [
  { value: "assignees", label: t("sla_assignees") },
  ...metadata.roles.map((item) => ({
    value: `role:${item.id}`,
    label: t("sla_roleRecipient", { role: item.name }),
  })),
]);
const policyColumns = computed(() => [
  { title: t("sla_policyName"), key: "name", width: "45%" },
  { title: t("sla_effectiveType"), key: "conditions" },
  { title: t("sla_enableColumn"), key: "enabled", width: 110 },
  { title: t("sla_operations"), key: "actions", width: 170, align: "right" },
]);
const stageColumns = computed(() => [
  { title: t("sla_name"), dataIndex: "name" },
  { title: t("sla_statusChange"), key: "transition" },
  { title: t("sla_operations"), key: "actions" },
]);
const targetStages = computed(() =>
  stages.value.filter((item) => selectedStages.value.includes(item.id)),
);
const targetPriorities = computed(() => {
  const all = [
    ...metadata.priorities,
    { id: 0, name: t("sla_unspecified"), color: "" },
  ];
  return policyDraft.priority_ids.length
    ? all.filter((item) => policyDraft.priority_ids.includes(item.id))
    : all;
});
function categoryPath(item) {
  const names = [item.name],
    seen = new Set([item.id]);
  let parent = item.parent_id;
  while (parent && !seen.has(parent)) {
    seen.add(parent);
    const row = metadata.categories.find((entry) => entry.id === parent);
    if (!row) break;
    names.unshift(row.name);
    parent = row.parent_id;
  }
  return names.join(" / ");
}
function policyIncomplete(record) {
  const priorities = record.priority_ids.length
    ? record.priority_ids
    : [0, ...metadata.priorities.map((item) => item.id)];
  const stageIds = [
    ...new Set(record.config.targets.map((row) => row.stage_id)),
  ];
  return priorities.some((priority) =>
    stageIds.some(
      (stage) =>
        !record.config.targets.some(
          (row) => row.priority_id === priority && row.stage_id === stage,
        ),
    ),
  );
}
function conditionNames(ids, options) {
  return ids.length
    ? ids
        .map((id) =>
          id === 0
            ? t(
                options === metadata.categories
                  ? "sla_uncategorized"
                  : "sla_unspecified",
              )
            : options.find((item) => item.id === id)?.name ||
              t("sla_unavailableOption"),
        )
        .join(", ")
    : t("sla_any");
}
function selectorText(selector) {
  return selector.mode === "selected"
    ? selector.status_ids
        .map(
          (id) =>
            metadata.statuses.find((item) => item.id === id)?.name ||
            t("sla_unavailableOption"),
        )
        .join(", ")
    : t(`sla_selector_${selector.mode}`);
}
function transitionText(config) {
  return `${selectorText(config.from)} → ${selectorText(config.to)}`;
}
async function readAll(fn, ws) {
  const first = await fn(ws, { page: 1, size: 100 });
  const rows = [...first.list];
  for (let page = 2; rows.length < first.total; page++) {
    const next = (await fn(ws, { page, size: 100 })).list;
    if (!next.length) break;
    rows.push(...next);
  }
  return rows;
}
async function load() {
  const sequence = ++loadSequence,
    ws = props.wsId;
  loading.value = true;
  loadFailed.value = false;
  stages.value = [];
  policies.value = [];
  stageOpen.value = false;
  policyOpen.value = false;
  webhookOpen.value = false;
  webhookAtOpen.value = false;
  mappingModalOpen.value = false;
  try {
    const [config, options, stageRows, policyRows] = await Promise.all([
      getSLASettings(ws),
      getSLAMetadata(ws),
      readAll(listSLAStages, ws),
      readAll(listSLAPolicies, ws),
    ]);
    if (sequence !== loadSequence) return;
    Object.assign(settings, config, {
      timezone:
        config.timezone ||
        Intl.DateTimeFormat().resolvedOptions().timeZone ||
        "UTC",
    });
    Object.assign(metadata, options);
    stages.value = stageRows;
    policies.value = policyRows;
  } catch {
    if (sequence === loadSequence) loadFailed.value = true;
  } finally {
    if (sequence === loadSequence) loading.value = false;
  }
}
watch(
  () => props.wsId,
  () => {
    load().catch(() => {});
  },
  { immediate: true },
);
async function persistSettings(enabled) {
  if (!canManage.value || savingSettings.value) return;
  const sequence = loadSequence;
  savingSettings.value = true;
  try {
    const result = await saveSLASettings(props.wsId, {
      enabled,
      timezone: settings.timezone,
      revision: settings.revision,
    });
    if (sequence !== loadSequence) return;
    Object.assign(settings, result);
    auth.clearProjectMetadata();
    message.success(t("common_saveSuccess"));
  } catch {
  } finally {
    savingSettings.value = false;
  }
}
function toggleEnabled(enabled) {
  persistSettings(enabled).catch(() => {});
}
function editStage(item) {
  stageSubmitted.value = false;
  Object.assign(
    stageDraft,
    item
      ? clone(item)
      : {
          id: 0,
          name: "",
          sort_order: stages.value.length,
          revision: 0,
          config: {
            from: { mode: "selected", status_ids: [] },
            to: { mode: "selected", status_ids: [] },
          },
        },
  );
  // Unsupported virtual selectors must be explicitly replaced when editing.
  for (const side of ["from", "to"]) {
    if (stageDraft.config[side].mode !== "any")
      stageDraft.config[side].mode = "selected";
  }
  stageOpen.value = true;
}
async function submitStage() {
  if (saving.value || !canManage.value) return;
  stageSubmitted.value = true;
  if (
    !stageDraft.name.trim() ||
    [stageDraft.config.from, stageDraft.config.to].some(stageSelectorMissing) ||
    stageStatusesOverlap.value
  ) {
    return;
  }
  const sequence = loadSequence;
  saving.value = true;
  try {
    await saveSLAStage(props.wsId, clone(stageDraft));
    if (sequence !== loadSequence) return;
    stageOpen.value = false;
    await load();
  } catch {
  } finally {
    saving.value = false;
  }
}
function defaultSlot() {
  return { weekdays: [1, 2, 3, 4, 5], start: "09:00", end: "18:00" };
}
function updateSlotRange(slot, range) {
  if (!range) return;
  slot.start = range[0];
  if (slot.end !== "24:00") slot.end = range[1];
}
function minuteText(value) {
  return `${String(Math.floor(value / 60)).padStart(2, "0")}:${String(value % 60).padStart(2, "0")}`;
}
function targetCell(priority, stage) {
  const key = `${priority}:${stage}`;
  if (!targetEdits[key])
    targetEdits[key] = { value: undefined, unit: "minutes" };
  return targetEdits[key];
}
function editPolicy(item) {
  Object.keys(targetEdits).forEach((key) => delete targetEdits[key]);
  const value = item
    ? clone(item)
    : {
        id: 0,
        name: "",
        description: "",
        enabled: true,
        revision: 0,
        sort_order: policies.value.length,
        priority_ids: [],
        category_ids: [],
        config: {
          calendar: {
            mode: "weekly",
            pause_on_holidays: false,
            holiday_calendar_code: "default",
            slots: [],
          },
          targets: [],
          actions: [],
        },
      };
  Object.assign(policyDraft, value);
  selectedStages.value = item
    ? [...new Set(value.config.targets.map((row) => row.stage_id))]
    : stages.value.map((row) => row.id);
  slots.value = value.config.calendar.slots?.length
    ? value.config.calendar.slots.map((slot) => ({
        weekdays: [...slot.weekdays],
        start: minuteText(slot.start_minute),
        end: minuteText(slot.end_minute),
      }))
    : [defaultSlot()];
  for (const target of value.config.targets)
    targetEdits[`${target.priority_id}:${target.stage_id}`] = {
      value: target.limit_seconds / 60,
      unit: "minutes",
    };
  actionDrafts.value = (value.config.actions || []).map((action) => ({
    channel: action.channel || "notification",
    webhooks: clone(action.webhooks || []),
    key: action.key,
    stage_id: action.stage_id,
    position: action.position,
    value: action.offset_seconds / 60,
    unit: "minutes",
    recipients: (action.channel === "webhook" ? [] : action.recipients).map(
      (recipient) =>
        recipient.type === "workspace_role"
          ? `role:${recipient.role_id}`
          : recipient.type,
    ),
  }));
  policyOpen.value = true;
}
function addAction() {
  actionDrafts.value.push({
    channel: "notification",
    webhooks: [],
    key:
      globalThis.crypto?.randomUUID?.() ||
      `${Date.now()}-${Math.random().toString(36).slice(2)}`,
    stage_id: selectedStages.value[0],
    position: "before_deadline",
    value: 5,
    unit: "minutes",
    recipients: ["assignees"],
  });
}
function minutes(value) {
  if (!/^\d{2}:\d{2}$/.test(value)) return NaN;
  const [hour, minute] = value.split(":").map(Number);
  return hour > 24 || minute > 59 || (hour === 24 && minute !== 0)
    ? NaN
    : hour * 60 + minute;
}
async function submitPolicy() {
  if (saving.value || !canManage.value) return;
  const targets = targetPriorities.value.flatMap((priority) =>
    targetStages.value.map((stage) => {
      const edit = targetCell(priority.id, stage.id);
      return {
        priority_id: priority.id,
        stage_id: stage.id,
        limit_seconds: Number(edit.value) * (edit.unit === "hours" ? 3600 : 60),
      };
    }),
  );
  const calendar = {
    ...clone(policyDraft.config.calendar),
    slots:
      policyDraft.config.calendar.mode === "weekly"
        ? slots.value.map((slot) => ({
            weekdays: slot.weekdays,
            start_minute: minutes(slot.start),
            end_minute: minutes(slot.end),
          }))
        : [],
  };
  if (!policyDraft.name.trim()) {
    message.warning(t("sla_policyNameRequired"));
    return;
  }
  if (!targetStages.value.length) {
    message.warning(t("sla_targetStagesRequired"));
    return;
  }
  const invalidTarget = targets.find(
    (row) => !Number.isSafeInteger(row.limit_seconds) || row.limit_seconds <= 0,
  );
  if (invalidTarget) {
    message.warning(
      t("sla_targetLimitRequired", {
        priority: targetPriorities.value.find(
          (item) => item.id === invalidTarget.priority_id,
        ).name,
        stage: targetStages.value.find(
          (item) => item.id === invalidTarget.stage_id,
        ).name,
      }),
    );
    return;
  }
  if (calendar.mode === "weekly" && !calendar.slots.length) {
    message.warning(t("sla_workSlotsRequired"));
    return;
  }
  for (const [index, slot] of calendar.slots.entries()) {
    if (!slot.weekdays.length) {
      message.warning(t("sla_slotWeekdaysRequired", { index: index + 1 }));
      return;
    }
    if (
      !Number.isFinite(slot.start_minute) ||
      !Number.isFinite(slot.end_minute) ||
      slot.start_minute >= 1440 ||
      slot.end_minute <= slot.start_minute
    ) {
      message.warning(t("sla_slotTimeInvalid", { index: index + 1 }));
      return;
    }
  }
  const actions = [];
  for (const [index, action] of actionDrafts.value.entries()) {
    const params = { index: index + 1 };
    if (!targetStages.value.some((stage) => stage.id === action.stage_id)) {
      message.warning(t("sla_actionStageRequired", params));
      return;
    }
    if (!["before_deadline", "after_deadline"].includes(action.position)) {
      message.warning(t("sla_actionTimingRequired", params));
      return;
    }
    const offset = Number(action.value) * (action.unit === "hours" ? 3600 : 60);
    if (
      action.value === null ||
      action.value === undefined ||
      action.value === "" ||
      !Number.isSafeInteger(offset) ||
      offset < 0
    ) {
      message.warning(t("sla_actionOffsetInvalid", params));
      return;
    }
    if (
      action.position === "before_deadline" &&
      targets.some(
        (row) => row.stage_id === action.stage_id && row.limit_seconds < offset,
      )
    ) {
      message.warning(t("sla_actionOffsetExceedsLimit", params));
      return;
    }
    if (
      action.channel !== "webhook" &&
      (!action.recipients.length ||
        action.recipients.some(
          (recipient) =>
            !recipientOptions.value.some(
              (option) => option.value === recipient,
            ),
        ))
    ) {
      message.warning(t("sla_actionRecipientsRequired", params));
      return;
    }
    if (action.channel === "webhook" && !action.webhooks.length) {
      message.warning(t("sla_webhookRequired"));
      return;
    }
    actions.push({
      channel: action.channel,
      webhooks: action.channel === "webhook" ? clone(action.webhooks) : [],
      key: action.key,
      stage_id: action.stage_id,
      position: action.position,
      offset_seconds: offset,
      recipients: (action.channel === "webhook" ? [] : action.recipients).map(
        (recipient) =>
          recipient === "assignees"
            ? { type: "assignees" }
            : {
                type: "workspace_role",
                role_id: Number(recipient.split(":")[1]),
              },
      ),
    });
  }
  const sequence = loadSequence;
  saving.value = true;
  try {
    await saveSLAPolicy(props.wsId, {
      ...clone(policyDraft),
      config: { calendar, targets, actions },
    });
    if (sequence !== loadSequence) return;
    policyOpen.value = false;
    await load();
  } catch {
  } finally {
    saving.value = false;
  }
}
function createPolicy() {
  if (!stages.value.length) {
    message.info(t("sla_needStages"));
    tab.value = "stages";
    return;
  }
  editPolicy();
}
async function togglePolicy(record, enabled) {
  if (!canManage.value || policySaving.value || saving.value || sorting.value)
    return;
  const sequence = loadSequence;
  policySaving.value = record.id;
  try {
    const result = await saveSLAPolicy(props.wsId, {
      ...clone(record),
      enabled,
    });
    if (sequence !== loadSequence) return;
    Object.assign(record, result);
    const config = await getSLASettings(props.wsId);
    if (sequence === loadSequence) Object.assign(settings, config);
  } finally {
    policySaving.value = 0;
  }
}
async function movePolicy(index, direction) {
  if (sorting.value) return;
  const ids = policies.value.map((item) => item.id);
  [ids[index], ids[index + direction]] = [ids[index + direction], ids[index]];
  sorting.value = true;
  try {
    await saveSLAPolicyOrder(props.wsId, { ids, revision: settings.revision });
    await load();
  } catch {
  } finally {
    sorting.value = false;
  }
}
async function archive(type, item) {
  if (saving.value) return;
  const sequence = loadSequence;
  saving.value = true;
  try {
    await archiveSLADefinition(props.wsId, type, item);
    if (sequence === loadSequence) await load();
  } catch {
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped lang="less">
.sla-settings {
  --sla-border: var(--ant-color-border-secondary, #eef1f6);
  --sla-muted: var(--ant-color-text-tertiary, #98a2b3);
  color: var(--ant-color-text, #1f2329);
  background: var(--ant-color-bg-container, #fff);
  border: 1px solid var(--sla-border);
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(24, 43, 80, 0.04);
  overflow: hidden;
}
.sla-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 24px;
  min-height: 54px;
  border-bottom: 1px solid var(--sla-border);
  background: var(--ant-color-fill-quaternary, #f5f7fa);
  padding: 12px 16px;
}
.sla-heading {
  display: flex;
  align-items: baseline;
  gap: 12px;
  min-width: 0;
}
.sla-title {
  flex-shrink: 0;
  font-size: 14px;
  font-weight: 600;
}
.sla-introduction {
  color: var(--sla-muted);
  font-size: 13px;
  line-height: 1.7;
}
.sla-tabs {
  :deep(.ant-tabs-nav) {
    margin-bottom: 16px;
  }
  :deep(.ant-tabs-content-holder) {
    display: none;
  }
}
.sla-master-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ant-color-text-secondary, #667085);
  font-size: 13px;
  white-space: nowrap;
}
.sla-content {
  padding: 16px;
}
.sla-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 16px 0;
  flex-wrap: wrap;
}
.sla-disabled {
  margin-bottom: 16px;
}
.sla-table {
  :deep(.ant-table-thead > tr > th) {
    font-weight: 600;
  }
}
.sla-policy-name {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-wrap: anywhere;
  .anticon {
    color: var(--ant-color-primary, #1677ff);
  }
}
.sla-condition-label {
  color: var(--ant-color-text-secondary, #536074);
}
.sla-text-action {
  padding: 0;
  height: auto;
}
.sla-order-action {
  padding: 0;
  width: 20px;
}
.sla-hint {
  margin-top: 16px;
}
/* The modal is teleported outside this page; keep overrides scoped to its wrapper. */
:global(.sla-stage-modal .ant-modal-content) {
  padding: 0;
  overflow: hidden;
  border-radius: 8px;
}
:global(.sla-stage-modal .ant-modal-header) {
  padding: 18px 20px;
  margin: 0;
  border-bottom: 1px solid #edeff2;
}
:global(.sla-stage-modal .ant-modal-title) {
  font-size: 18px;
  line-height: 26px;
  font-weight: 600;
}
:global(.sla-stage-modal .ant-modal-close) {
  top: 16px;
  inset-inline-end: 18px;
}
:global(.sla-stage-modal .ant-modal-body) {
  padding: 20px;
}
:global(.sla-stage-modal .ant-modal-footer) {
  padding: 12px 20px;
  margin: 0;
  border-top: 1px solid #edeff2;
}
:global(.sla-stage-modal .ant-modal-footer .ant-btn) {
  min-width: 60px;
}
.sla-stage-form {
  padding: 4px 0;
  :deep(.ant-form-item-label > label) {
    font-weight: 500;
  }
}
.sla-stage-boundary {
  margin-bottom: 0;
}
.sla-stage-label {
  margin-bottom: 10px;
  font-weight: 500;
}
.sla-transition {
  display: grid;
  grid-template-columns: max-content minmax(100px, 140px) max-content minmax(
      100px,
      140px
    );
  align-items: start;
  gap: 10px;
  :deep(.ant-form-item) {
    margin-bottom: 0;
    min-width: 0;
  }
  :deep(.ant-select) {
    width: 100%;
  }
}
.sla-transition-word {
  width: max-content;
  white-space: nowrap;
  line-height: 32px;
  color: #646a73;
}
.sla-stage-help {
  color: #8590a0;
  font-size: 12px;
  line-height: 1.7;
  margin: 4px 0 0;
}
:global(.sla-policy-drawer .ant-drawer-body) {
  padding: 28px 36px 40px;
}
:global(.sla-policy-drawer .ant-drawer-title) {
  font-size: 18px;
}
.sla-policy-form {
  color: var(--ant-color-text, #1f2329);
  :deep(.ant-form-item-label) {
    flex: 0 0 112px;
    text-align: left;
  }
  :deep(.ant-form-item-control) {
    min-width: 0;
  }
  :deep(.ant-input),
  :deep(.ant-select-selector),
  :deep(.ant-input-number) {
    border-color: var(--ant-color-border-secondary, #edeff2);
    border-radius: 8px;
  }
}
.sla-policy-basics {
  max-width: 720px;
}
.sla-name-input {
  max-width: 360px;
}
.sla-policy-section {
  margin-top: 30px;
  h3 {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 15px;
    font-weight: 600;
    margin: 0 0 16px;
    .anticon {
      color: var(--ant-color-text-tertiary, #8590a0);
      font-size: 14px;
      cursor: help;
    }
  }
  :deep(.ant-form-item) {
    margin-bottom: 16px;
  }
}
.sla-priority-tag {
  --priority-color: var(--ant-color-text-secondary, #8c8c8c);
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 10px;
  border: 1px solid var(--priority-color);
  border-radius: 12px;
  color: var(--priority-color);
  background: transparent;
  font-size: 12px;
  font-weight: 600;
  line-height: 22px;
  white-space: nowrap;
}
.sla-priority-select,
.sla-category-select {
  min-width: 200px;
  max-width: 500px;
}
.sla-clock-options {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 16px;
  width: 100%;
}
.sla-slots {
  // RadioGroup sets font-size to zero; restore it for the schedule content.
  font-size: var(--ant-font-size, 14px);
  padding-left: 24px;
  display: grid;
  gap: 12px;
  max-width: 100%;
}
.sla-slot,
.sla-slot-time {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.sla-slot {
  :deep(.ant-checkbox-group) {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  :deep(.ant-checkbox-wrapper) {
    margin: 0;
    color: var(--ant-color-text-secondary, #536074);
  }
}
.sla-time-range-picker {
  width: 260px;
  max-width: 100%;
  border-color: var(--ant-color-border-secondary, #edeff2);
  border-radius: 8px;
}
.sla-remove-button {
  background: var(--ant-color-error-bg, #fff1f0);
  width: 28px;
  min-width: 28px;
  height: 28px;
  padding: 0;
}
.sla-add-button {
  color: var(--ant-color-primary, #3370ff);
  width: 32px;
  padding: 0;
  font-size: 18px;
}
.sla-target-scroll {
  overflow-x: auto;
}
.sla-targets {
  border-collapse: collapse;
  th,
  td {
    padding: 8px 24px 8px 0;
    text-align: left;
    font-weight: 400;
  }
  thead th {
    color: var(--ant-color-text-secondary, #536074);
    padding-bottom: 12px;
    min-width: 136px;
  }
}
.sla-duration {
  display: flex;
  gap: 8px;
  :deep(.ant-input-number) {
    width: 80px;
  }
  :deep(.ant-select) {
    width: 88px;
  }
}
.sla-action {
  display: grid;
  grid-template-columns:
    minmax(130px, 1fr) minmax(130px, 1fr)
    72px 86px 200px minmax(140px, 1fr) 28px;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  > * {
    min-width: 0;
  }
  :deep(.ant-input-number) {
    width: 100%;
  }
}
.sla-webhook-summary {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  :deep(.ant-btn) {
    padding-inline: 0;
  }
  :deep(.ant-tag) {
    margin-inline-end: 0;
  }
}
.sla-webhook-modal :deep(.ant-modal-body) {
  max-height: calc(100dvh - 240px);
  overflow-y: auto;
  margin: 0 -24px;
  padding: 24px;
}
.sla-webhook-platforms {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 22px;
  .is-selected {
    color: var(--ant-color-primary, #3370ff);
    border-color: var(--ant-color-primary, #3370ff);
    background: var(--ant-color-primary-bg, #f0f5ff);
  }
}
.sla-webhook-config {
  padding: 20px;
  border: 1px solid var(--ant-color-border-secondary, #edeff2);
  h3 {
    margin: 0 0 16px;
    font-size: 16px;
    font-weight: 600;
  }
  :deep(.ant-form-item:last-child) {
    margin-bottom: 0;
  }
  margin-bottom: 16px;
  background: var(--ant-color-fill-quaternary, #f5f7fa);
  border-radius: 8px;
}
.sla-webhook-at-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  &:not(:empty) {
    margin-top: 8px;
  }
}
.sla-webhook-tip {
  margin: 12px 0 0;
  color: var(--ant-color-text-tertiary, #98a2b3);
}
.sla-optional {
  color: var(--ant-color-text-tertiary, #98a2b3);
  font-size: 13px;
  font-weight: 400;
}
.sla-notification-type {
  padding: 5px 12px;
  border: 1px solid var(--ant-color-border-secondary, #edeff2);
  border-radius: 8px;
  color: var(--ant-color-text-secondary, #536074);
}
.sla-policy-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
@media (max-width: 1100px) {
  .sla-action {
    grid-template-columns: minmax(120px, 1fr) minmax(120px, 1fr) 72px 86px;
  }
}
@media (max-width: 700px) {
  :global(.sla-policy-drawer .ant-drawer-body) {
    padding: 20px 16px;
  }
  .sla-policy-form :deep(.ant-form-item-label) {
    flex-basis: 100%;
    text-align: left;
  }
  .sla-slots {
    padding-left: 0;
  }
  .sla-notification-type {
    white-space: normal;
  }
  .sla-header {
    flex-wrap: wrap;
    gap: 12px;
  }
  .sla-action {
    grid-template-columns: 1fr 1fr;
  }
  .sla-transition {
    grid-template-columns: auto minmax(0, 1fr);
  }
}
</style>
