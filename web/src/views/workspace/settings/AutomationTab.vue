<template>
  <div class="automation-tab">
    <a-tabs v-model:activeKey="subTab">
      <a-tab-pane key="rules">
        <template #tab>
          <span class="tab-title"
            ><SettingOutlined />{{ $t("automation_settings") }}</span
          >
        </template>
        <div class="pane">
          <div class="toolbar">
            <a-input-search
              v-model:value="ruleQuery.search"
              :placeholder="$t('automation_searchRule')"
              allow-clear
              @search="loadRules(1)"
            />
            <a-button v-if="canManage" type="primary" @click="openRule()"
              ><PlusOutlined />{{ $t("automation_newRule") }}</a-button
            >
          </div>
          <div ref="ruleAreaRef" class="table-area">
            <a-table
              :data-source="rules"
              :columns="ruleColumns"
              :loading="ruleLoading"
              row-key="id"
              size="middle"
              bordered
              :pagination="false"
              :scroll="{ y: ruleBodyHeight }"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'name'">{{
                  record.name
                }}</template>
                <template v-else-if="column.key === 'today'">
                  <a-button
                    type="link"
                    @click="jumpToExecutions(record, 'today')"
                    >{{ record.today_trigger_count }}</a-button
                  >
                </template>
                <template v-else-if="column.key === 'yesterday'">
                  <a-button
                    type="link"
                    @click="jumpToExecutions(record, 'yesterday')"
                    >{{ record.yesterday_trigger_count }}</a-button
                  >
                </template>
                <template v-else-if="column.key === 'total'">
                  <a-button
                    type="link"
                    @click="jumpToExecutions(record, 'total')"
                    >{{ record.total_trigger_count }}</a-button
                  >
                </template>
                <template v-else-if="column.key === 'enabled'">
                  <a-switch
                    :checked="Number(record.enabled) === 1"
                    :disabled="!canManage"
                    :checked-children="$t('automation_on')"
                    :un-checked-children="$t('automation_off')"
                    @change="(value) => toggleRule(record, value)"
                  />
                </template>
                <template v-else-if="column.key === 'actions'">
                  <a-space v-if="canManage">
                    <a-button type="link" size="small" @click="openRule(record)"
                      ><EditOutlined />{{ $t("common_edit") }}</a-button
                    >
                    <a-button
                      type="link"
                      size="small"
                      :loading="copyingRuleId === record.id"
                      @click="openCopyRule(record)"
                      ><CopyOutlined />{{ $t("common_copy") }}</a-button
                    >
                    <a-popconfirm
                      :title="$t('automation_deleteConfirm')"
                      @confirm="removeRule(record)"
                    >
                      <a-button type="link" danger size="small"
                        ><DeleteOutlined />{{ $t("common_delete") }}</a-button
                      >
                    </a-popconfirm>
                  </a-space>
                </template>
              </template>
            </a-table>
            <div class="pagination-bar">
              <a-pagination
                :current="ruleQuery.page"
                :page-size="ruleQuery.size"
                :total="ruleTotal"
                show-size-changer
                :page-size-options="['20', '50', '100']"
                @change="onRulePage"
              />
            </div>
          </div>
        </div>
      </a-tab-pane>

      <a-tab-pane key="executions">
        <template #tab>
          <span class="tab-title"
            ><HistoryOutlined />{{ $t("automation_executions") }}</span
          >
        </template>
        <div class="pane">
          <div class="toolbar execution-toolbar">
            <a-input-search
              v-model:value="executionQuery.search"
              :placeholder="$t('automation_searchRule')"
              allow-clear
              @search="loadExecutions(1)"
            />
            <a-range-picker
              v-model:value="executionDates"
              @change="loadExecutions(1)"
            />
            <a-button @click="clearExecutionFilter">{{
              $t("common_reset")
            }}</a-button>
          </div>
          <div ref="executionAreaRef" class="table-area">
            <a-table
              :data-source="executions"
              :columns="executionColumns"
              :loading="executionLoading"
              row-key="id"
              size="middle"
              bordered
              :pagination="false"
              :scroll="{ y: executionBodyHeight }"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'triggered_at'">{{
                  formatLocalDate(record.triggered_at, true)
                }}</template>
                <template v-else-if="column.key === 'rule_name'">
                  <a-tooltip :title="record.rule_name">
                    <span class="execution-ellipsis">{{
                      record.rule_name
                    }}</span>
                  </a-tooltip>
                </template>
                <template v-else-if="column.key === 'subject'">
                  <a-space>
                    <a-tag
                      :color="
                        record.trigger_item_type === 'defect'
                          ? 'magenta'
                          : 'blue'
                      "
                      ><BugOutlined
                        v-if="record.trigger_item_type === 'defect'"
                      />
                      <FileTextOutlined v-else />
                      {{
                        $t(`workitem_type_${record.trigger_item_type}`)
                      }}</a-tag
                    >
                    <a-tooltip :title="record.trigger_item_title">
                      <span class="subject-title">{{
                        record.trigger_item_title
                      }}</span>
                    </a-tooltip>
                  </a-space>
                </template>
                <template v-else-if="column.key === 'status'">
                  <a-tag :color="statusColor(record.status)">{{
                    $t(`automation_status_${record.status}`)
                  }}</a-tag>
                </template>
                <template v-else-if="column.key === 'failure_reason'">
                  <a-tooltip :title="failureReason(record.failure_reason)">
                    <span
                      :class="[
                        'execution-ellipsis',
                        { failed: record.failure_reason },
                      ]"
                    >
                      {{ failureReason(record.failure_reason) }}
                    </span>
                  </a-tooltip>
                </template>
                <template v-else-if="column.key === 'detail'">
                  <a-button
                    type="link"
                    size="small"
                    @click="openExecution(record)"
                    ><FileTextOutlined />{{
                      $t("automation_viewLog")
                    }}</a-button
                  >
                </template>
              </template>
            </a-table>
            <div class="pagination-bar">
              <a-pagination
                :current="executionQuery.page"
                :page-size="executionQuery.size"
                :total="executionTotal"
                show-size-changer
                :page-size-options="['20', '50', '100']"
                @change="onExecutionPage"
              />
            </div>
          </div>
        </div>
      </a-tab-pane>
    </a-tabs>

    <a-modal
      v-model:open="ruleModalOpen"
      :width="1120"
      :footer="null"
      centered
      class="automation-rule-modal"
      :styles="{ body: { padding: 0, height: '560px', overflow: 'hidden' } }"
    >
      <template #title>
        <div class="rule-modal-title">
          <span class="required-label">{{ $t("automation_ruleName") }}</span>
          <a-input
            v-model:value="ruleName"
            :maxlength="200"
            :placeholder="$t('automation_ruleNamePlaceholder')"
            :status="
              validationAttempted && !ruleName.trim() ? 'error' : undefined
            "
            aria-required="true"
          />
        </div>
      </template>
      <div class="rule-editor">
        <aside class="flow-nav">
          <button
            :class="{ active: selectedStep === 'trigger' }"
            @click="selectedStep = 'trigger'"
          >
            <span class="flow-icon trigger-icon"><CalendarOutlined /></span>
            <span class="nav-copy">
              <strong>{{ $t("automation_trigger") }}</strong>
              <small>{{ triggerSummary }}</small>
            </span>
          </button>
          <button
            v-for="(node, index) in ruleConfig.nodes"
            :key="node.id"
            :class="{ active: selectedStep === node.id }"
            @click="selectedStep = node.id"
          >
            <span class="flow-icon action-icon"><ThunderboltOutlined /></span>
            <span class="nav-copy">
              <strong>{{
                node.name || $t("automation_executionNode", { n: index + 1 })
              }}</strong>
              <small>{{ nodeSummary(node) }}</small>
            </span>
            <CloseCircleOutlined
              v-if="ruleConfig.nodes.length > 1"
              class="remove-node"
              @click.stop="removeNode(index)"
            />
          </button>
          <a-button type="dashed" block @click="addNode"
            ><PlusOutlined />{{ $t("automation_addNode") }}</a-button
          >
        </aside>

        <section class="step-editor">
          <div class="step-editor-content">
            <template v-if="selectedStep === 'trigger'">
              <h3 class="editor-title">
                <span class="flow-icon trigger-icon"><CalendarOutlined /></span>
                {{ $t("automation_trigger") }}
              </h3>
              <div class="config-section">
                <div class="section-label">{{ $t("automation_object") }}</div>
                <div class="object-grid">
                  <button
                    type="button"
                    class="object-card"
                    :class="{
                      active: ruleConfig.trigger.object === 'requirement',
                    }"
                    @click="selectTriggerObject('requirement')"
                  >
                    <span class="object-icon requirement-icon"
                      ><FileTextOutlined
                    /></span>
                    <span>{{ $t("workitem_type_requirement") }}</span>
                  </button>
                  <button
                    v-if="defectAppEnabled"
                    type="button"
                    class="object-card defect-card"
                    :class="{ active: ruleConfig.trigger.object === 'defect' }"
                    @click="selectTriggerObject('defect')"
                  >
                    <span class="object-icon defect-icon"><BugOutlined /></span>
                    <span>{{ $t("workitem_type_defect") }}</span>
                  </button>
                </div>
              </div>
              <div class="config-section">
                <div class="section-label">{{ $t("automation_event") }}</div>
                <div class="event-grid">
                  <button
                    v-for="item in eventCards"
                    :key="item.value"
                    type="button"
                    class="event-card"
                    :class="{ active: ruleConfig.trigger.event === item.value }"
                    @click="resetTriggerDetails(item.value)"
                  >
                    <span class="event-card-title">
                      <component :is="item.icon" />{{ item.label }}
                    </span>
                    <small>{{ item.description }}</small>
                  </button>
                </div>
              </div>
              <a-form layout="vertical" class="trigger-detail-form">
                <a-form-item
                  v-if="ruleConfig.trigger.event === 'comment'"
                  :label="$t('automation_commentChange')"
                  required
                >
                  <a-select
                    v-model:value="ruleConfig.trigger.comment_change"
                    :options="commentChangeOptions"
                  />
                </a-form-item>
                <div
                  v-if="ruleConfig.trigger.event === 'status'"
                  class="two-columns"
                >
                  <a-form-item :label="$t('automation_fromStatus')">
                    <a-select
                      v-model:value="ruleConfig.trigger.from_status_id"
                      :options="fromStatusOptions"
                    />
                  </a-form-item>
                  <a-form-item :label="$t('automation_toStatus')" required>
                    <a-select
                      v-model:value="ruleConfig.trigger.to_status_id"
                      :options="
                        withMissingOption(
                          triggerStatuses,
                          ruleConfig.trigger.to_status_id,
                        )
                      "
                      :status="
                        triggerStatusInvalid ||
                        (validationAttempted &&
                          !ruleConfig.trigger.to_status_id)
                          ? 'error'
                          : undefined
                      "
                    />
                    <small
                      v-if="triggerStatusInvalid"
                      class="invalid-reference-help"
                    >
                      {{ $t("automation_invalidOption") }}
                    </small>
                  </a-form-item>
                </div>
                <template v-if="ruleConfig.trigger.event === 'field'">
                  <div class="two-columns">
                    <a-form-item :label="$t('automation_field')" required>
                      <a-select
                        v-model:value="ruleConfig.trigger.field_key"
                        :options="
                          withMissingOption(
                            editableFieldOptions,
                            ruleConfig.trigger.field_key,
                          )
                        "
                        :status="
                          triggerFieldInvalid ||
                          (validationAttempted && !ruleConfig.trigger.field_key)
                            ? 'error'
                            : undefined
                        "
                        @change="ruleConfig.trigger.field_value = undefined"
                      />
                      <small
                        v-if="triggerFieldInvalid"
                        class="invalid-reference-help"
                      >
                        {{ $t("automation_invalidOption") }}
                      </small>
                    </a-form-item>
                    <a-form-item :label="$t('automation_fieldChange')" required>
                      <a-select
                        v-model:value="ruleConfig.trigger.field_change"
                        :options="fieldChangeOptions"
                      />
                    </a-form-item>
                  </div>
                  <a-form-item
                    v-if="ruleConfig.trigger.field_change !== 'any'"
                    :label="$t('automation_value')"
                    required
                  >
                    <ConditionValue
                      v-model="ruleConfig.trigger.field_value"
                      :field="triggerField"
                      :metadata="metadata"
                      :trigger-type="ruleConfig.trigger.object"
                      :show-error="
                        validationAttempted && !triggerFieldValueValid()
                      "
                    />
                  </a-form-item>
                </template>
              </a-form>
            </template>

            <template v-else-if="activeNode">
              <div class="node-header">
                <span class="flow-icon action-icon"
                  ><ThunderboltOutlined
                /></span>
                <a-input
                  v-model:value="activeNode.name"
                  :maxlength="50"
                  :placeholder="
                    $t('automation_executionNode', { n: activeNodeIndex + 1 })
                  "
                />
                <template v-if="activeNode.mode">
                  <span class="node-type">{{ nodeModeLabel(activeNode) }}</span>
                  <a-button
                    type="link"
                    size="small"
                    @click="pickingModeNodeId = activeNode.id"
                    >{{ $t("automation_changeMode") }}</a-button
                  >
                </template>
              </div>
              <div v-if="showModePicker" class="mode-picker">
                <h3 class="mode-picker-title">
                  {{ $t("automation_selectExecutionMode") }}
                </h3>
                <div class="mode-grid">
                  <button
                    v-for="item in executionModes"
                    :key="item.value"
                    type="button"
                    class="mode-card"
                    :class="{ active: activeNode.mode === item.value }"
                    @click="selectNodeMode(item.value)"
                  >
                    <span class="mode-card-head">
                      <span class="mode-icon" :class="item.iconClass"
                        ><component :is="item.icon"
                      /></span>
                      {{ item.label }}
                    </span>
                    <small>{{ item.description }}</small>
                  </button>
                </div>
              </div>
              <div
                v-else-if="activeNode.mode === 'direct'"
                class="node-content"
              >
                <AutomationActionEditor
                  v-model="activeNode.action"
                  :metadata="metadata"
                  :trigger-type="ruleConfig.trigger.object"
                  :variables="availableVariables"
                  :show-errors="validationAttempted"
                />
              </div>
              <div v-else class="node-content conditional-editor">
                <a-collapse
                  v-model:activeKey="openBranchKeys"
                  expand-icon-position="end"
                  class="branch-collapse"
                >
                  <a-collapse-panel
                    v-for="(branch, branchIndex) in conditionalBranches(
                      activeNode,
                    )"
                    :key="branch.id"
                  >
                    <template #header>
                      <span class="branch-title">
                        <span class="module-icon"><ForkOutlined /></span>
                        {{
                          branchIndex === 0
                            ? $t("automation_ifBranch")
                            : $t("automation_elseIfBranch")
                        }}
                      </span>
                    </template>
                    <template v-if="branchIndex > 0" #extra>
                      <CloseOutlined
                        class="branch-remove"
                        :aria-label="$t('common_delete')"
                        @click.stop="
                          activeNode.else_if.splice(branchIndex - 1, 1)
                        "
                      />
                    </template>
                    <div class="branch-section-head">
                      <span class="branch-section-label">{{
                        $t("automation_conditionsLabel")
                      }}</span>
                      <span class="relation-hint">{{
                        $t("automation_moduleRelationHint")
                      }}</span>
                      <a-radio-group
                        v-model:value="branch.module_relation"
                        size="small"
                      >
                        <a-radio-button value="AND">AND</a-radio-button
                        ><a-radio-button value="OR">OR</a-radio-button>
                      </a-radio-group>
                      <a-button
                        type="primary"
                        size="small"
                        class="section-add"
                        @click="branch.modules.push(newModule())"
                        ><PlusOutlined />{{
                          $t("automation_addModule")
                        }}</a-button
                      >
                    </div>
                    <a-card
                      v-for="(module, moduleIndex) in branch.modules"
                      :key="module.id"
                      size="small"
                      class="condition-card"
                    >
                      <template #title>
                        <span class="module-title condition-title">
                          <span class="module-icon"><ForkOutlined /></span>
                          {{
                            $t("automation_conditionModule", {
                              n: moduleIndex + 1,
                            })
                          }}
                        </span>
                      </template>
                      <template #extra>
                        <span class="module-extra">
                          <span class="relation-hint">{{
                            $t("automation_conditionRelationHint")
                          }}</span>
                          <a-radio-group
                            v-model:value="module.relation"
                            size="small"
                            ><a-radio-button value="AND">AND</a-radio-button
                            ><a-radio-button value="OR"
                              >OR</a-radio-button
                            ></a-radio-group
                          >
                          <a-button
                            type="text"
                            danger
                            :disabled="branch.modules.length === 1"
                            :aria-label="$t('common_delete')"
                            @click="branch.modules.splice(moduleIndex, 1)"
                            ><DeleteOutlined
                          /></a-button>
                        </span>
                      </template>
                      <div
                        v-for="(condition, conditionIndex) in module.conditions"
                        :key="condition.id"
                        class="condition-row"
                      >
                        <a-select
                          v-model:value="condition.field_key"
                          :options="
                            withMissingOption(
                              allFieldOptions,
                              condition.field_key,
                            )
                          "
                          :status="
                            isConditionFieldInvalid(condition) ||
                            (validationAttempted && !condition.field_key)
                              ? 'error'
                              : undefined
                          "
                          :class="{
                            'invalid-reference-control':
                              isConditionFieldInvalid(condition),
                          }"
                          :placeholder="$t('automation_field')"
                          @change="resetCondition(condition)"
                        />
                        <a-select
                          v-model:value="condition.operator"
                          :options="
                            operatorOptions(
                              condition.field_key,
                              condition.operator,
                            )
                          "
                          :status="
                            validationAttempted && !condition.operator
                              ? 'error'
                              : undefined
                          "
                          :placeholder="$t('automation_operator')"
                          @change="condition.value = undefined"
                        />
                        <ConditionValue
                          v-if="
                            !['empty', 'not_empty'].includes(condition.operator)
                          "
                          v-model="condition.value"
                          :field="fieldByKey(condition.field_key)"
                          :operator="condition.operator"
                          :metadata="metadata"
                          :trigger-type="ruleConfig.trigger.object"
                          :show-error="
                            validationAttempted &&
                            !conditionValuePresent(condition.value)
                          "
                        />
                        <span v-else />
                        <a-checkbox
                          v-model:checked="condition.skip_when_empty"
                          >{{ $t("automation_skipWhenEmpty") }}</a-checkbox
                        >
                        <a-button
                          type="text"
                          danger
                          :disabled="module.conditions.length === 1"
                          @click="module.conditions.splice(conditionIndex, 1)"
                          ><DeleteOutlined
                        /></a-button>
                      </div>
                      <a-button
                        type="link"
                        @click="module.conditions.push(newCondition())"
                        ><PlusOutlined />{{
                          $t("automation_addCondition")
                        }}</a-button
                      >
                    </a-card>
                    <div class="branch-section-head">
                      <span class="branch-section-label">{{
                        $t("automation_actionsLabel")
                      }}</span>
                      <a-button
                        type="primary"
                        size="small"
                        class="section-add"
                        @click="branch.actions.push(newAction())"
                        ><PlusOutlined />{{
                          $t("automation_addAction")
                        }}</a-button
                      >
                    </div>
                    <a-card
                      v-for="(action, actionIndex) in branch.actions"
                      :key="action.id"
                      size="small"
                      class="action-card"
                    >
                      <template #title>
                        <span class="module-title action-title">
                          <span class="module-icon"
                            ><ThunderboltOutlined
                          /></span>
                          {{
                            $t("automation_actionNumber", {
                              n: actionIndex + 1,
                            })
                          }}
                        </span>
                      </template>
                      <template #extra
                        ><a-button
                          v-if="branch.actions.length > 1"
                          type="text"
                          danger
                          :aria-label="$t('common_delete')"
                          @click="branch.actions.splice(actionIndex, 1)"
                          ><DeleteOutlined /></a-button
                      ></template>
                      <AutomationActionEditor
                        v-model="branch.actions[actionIndex]"
                        compact
                        :metadata="metadata"
                        :trigger-type="ruleConfig.trigger.object"
                        :variables="availableVariables"
                        :show-errors="validationAttempted"
                      />
                    </a-card>
                  </a-collapse-panel>
                </a-collapse>
                <a-button
                  type="primary"
                  block
                  class="add-branch"
                  @click="activeNode.else_if.push(newConditionalBranch())"
                >
                  <PlusOutlined />{{ $t("automation_addBranch") }}
                </a-button>
              </div>
            </template>
          </div>
          <div class="editor-footer">
            <a-button @click="ruleModalOpen = false">{{
              $t("common_cancel")
            }}</a-button>
            <a-button
              type="primary"
              :loading="savingRule"
              @click="isTriggerStep ? goNextStep() : saveRule()"
            >
              {{
                isTriggerStep
                  ? $t("automation_nextStep")
                  : $t("automation_finish")
              }}
            </a-button>
          </div>
        </section>
      </div>
    </a-modal>

    <a-modal
      v-model:open="copyModalOpen"
      :title="$t('automation_copyTitle')"
      :ok-text="$t('common_copy')"
      :cancel-text="$t('common_cancel')"
      :confirm-loading="copySubmitting"
      :ok-button-props="{
        disabled: copyProjectsLoading || !copyTargetIds.length,
      }"
      :destroy-on-close="true"
      @ok="submitCopyRule"
    >
      <div class="copy-section-title">{{ $t("automation_copyProjects") }}</div>
      <a-spin :spinning="copyProjectsLoading">
        <a-checkbox-group
          v-model:value="copyTargetIds"
          class="copy-project-group"
        >
          <a-checkbox
            v-for="project in copyProjects"
            :key="project.id"
            :value="Number(project.id)"
            class="copy-project-item"
          >
            {{ project.name }}
          </a-checkbox>
        </a-checkbox-group>
        <a-empty
          v-if="!copyProjectsLoading && !copyProjects.length"
          :description="$t('automation_copyNoProject')"
        />
      </a-spin>
    </a-modal>

    <a-modal v-model:open="executionModalOpen" :width="640" centered>
      <template #title>
        <span class="modal-title"
          ><FileTextOutlined />{{ $t("automation_logDetail") }}</span
        >
      </template>
      <template #footer>
        <a-button @click="executionModalOpen = false">{{
          $t("common_close")
        }}</a-button>
      </template>
      <div v-if="activeExecution" class="execution-summary">
        <div>
          <b>{{ $t("automation_ruleName") }}：</b
          >{{ activeExecution.rule_name }}
        </div>
        <div>
          <b>{{ $t("automation_triggeredAt") }}：</b
          >{{ formatLocalDate(activeExecution.triggered_at, true) }}
        </div>
        <div>
          <b>{{ $t("automation_startedAt") }}：</b
          >{{ formatLocalDate(activeExecution.started_at, true) }}
        </div>
        <div>
          <b>{{ $t("automation_finishedAt") }}：</b
          >{{ formatLocalDate(activeExecution.finished_at, true) }}
        </div>
        <div>
          <b>{{ $t("automation_duration") }}：</b
          >{{ executionDuration(activeExecution) }}
        </div>
        <div>
          <b>{{ $t("automation_status") }}：</b
          >{{ $t(`automation_status_${activeExecution.status}`) }}
        </div>
        <div v-if="activeExecution.failure_reason" class="failed">
          {{ activeExecution.failure_reason }}
        </div>
        <div class="terminal">
          <div v-for="(log, index) in activeExecution.logs" :key="index">
            <span>[{{ formatLocalDate(log.time, true) }}]</span> {{ log.text }}
            <template v-if="log.attempt"> #{{ log.attempt }}</template>
            <pre v-if="log.raw">{{ log.raw }}</pre>
            <pre v-if="log.parameters">{{
              JSON.stringify(log.parameters, null, 2)
            }}</pre>
          </div>
        </div>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import {
  computed,
  defineComponent,
  h,
  markRaw,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import dayjs from "dayjs";
import {
  DatePicker,
  Input,
  InputNumber,
  Select,
  message,
} from "ant-design-vue";
import {
  BugOutlined,
  CalendarOutlined,
  CommentOutlined,
  CopyOutlined,
  CloseCircleOutlined,
  CloseOutlined,
  DeleteOutlined,
  EditOutlined,
  FileTextOutlined,
  ForkOutlined,
  HistoryOutlined,
  PlusOutlined,
  PlusSquareOutlined,
  SettingOutlined,
  SyncOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useLocaleStore } from "@/stores/locale";
import { filterMembersForField } from "@/utils/customField";
import { formatLocalDate } from "@/utils/date";
import {
  copyAutomationRule,
  createAutomationRule,
  deleteAutomationRule,
  getAutomationExecution,
  getAutomationMetadata,
  getAutomationRule,
  listAutomationCopyTargets,
  listAutomationExecutions,
  listAutomationRules,
  setAutomationRuleEnabled,
  updateAutomationRule,
} from "@/api/automation";
import AutomationActionEditor from "./AutomationActionEditor.vue";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";

// Page state serves the rule list, execution history, and rule editor.
const props = defineProps({ wsId: { type: [String, Number], required: true } });
// The defect trigger object is an app-market module: with the app off the card is gone,
// so no rule may be authored (or reopened) against it.
const { defectAppEnabled } = useWorkspaceApplications(
  computed(() => props.wsId),
);
const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const localeStore = useLocaleStore();
const authStore = useAuthStore();
const canManage = computed(() => authStore.hasPermission("automation:manage"));
const subTab = computed({
  get() {
    return route.query.automationTab === "executions" ? "executions" : "rules";
  },
  set(value) {
    if (value !== "rules" && value !== "executions") return;
    router.replace({
      query: { ...route.query, tab: "automation", automationTab: value },
    });
  },
});
const rules = ref([]);
const ruleLoading = ref(false);
const ruleQuery = ref({ page: 1, size: 20, search: "" });
const ruleTotal = ref(0);
const executions = ref([]);
const executionLoading = ref(false);
const executionQuery = ref({
  page: 1,
  size: 20,
  search: "",
  rule_id: undefined,
});
const executionTotal = ref(0);
const executionDates = ref([]);
const ruleModalOpen = ref(false);
const savingRule = ref(false);
const validationAttempted = ref(false);
const copyingRuleId = ref();
const copyModalOpen = ref(false);
const copyProjects = ref([]);
const copyProjectsLoading = ref(false);
const copySubmitting = ref(false);
const copyTargetIds = ref([]);
const copyRecord = ref();
const editingRuleId = ref();
const ruleName = ref("");
const selectedStep = ref("trigger");
// Holds the node whose execution mode is being re-picked, so switching nodes closes the picker.
const pickingModeNodeId = ref();
// The trigger step switches the footer action between Next and Finish.
const isTriggerStep = computed(() => selectedStep.value === "trigger");
const metadata = ref({});
const executionModalOpen = ref(false);
const activeExecution = ref();
const ruleConfig = ref(newConfig());
// Newly created work items have no children yet, and defects can never own children, so that context is hidden
// to avoid template variables silently rendering as empty.
const availableVariables = computed(() =>
  (metadata.value.variables || [])
    .filter(
      (item) =>
        item.group !== "child" ||
        (ruleConfig.value.trigger.event !== "create" &&
          ruleConfig.value.trigger.object !== "defect"),
    )
    .map((item) =>
      ruleConfig.value.trigger.object === "defect" && item.group === "parent"
        ? { ...item, group: "related" }
        : item,
    ),
);

// Table columns and editor options derive from locale and backend metadata.
const ruleColumns = computed(() => [
  {
    title: t("automation_ruleName"),
    key: "name",
    dataIndex: "name",
    width: 310,
  },
  { title: t("automation_today"), key: "today", width: 110 },
  { title: t("automation_yesterday"), key: "yesterday", width: 110 },
  { title: t("automation_total"), key: "total", width: 110 },
  { title: t("automation_enabled"), key: "enabled", width: 100 },
  {
    title: t("automation_actions"),
    key: "actions",
    width: 260,
    fixed: "right",
  },
]);
const executionColumns = computed(() => [
  { title: t("automation_triggeredAt"), key: "triggered_at", width: 180 },
  {
    title: t("automation_ruleName"),
    dataIndex: "rule_name",
    key: "rule_name",
    width: 220,
    ellipsis: true,
  },
  { title: t("automation_subject"), key: "subject", width: 280 },
  { title: t("automation_status"), key: "status", width: 110 },
  {
    title: t("automation_failureReason"),
    key: "failure_reason",
    width: 240,
    ellipsis: true,
  },
  {
    title: t("automation_logDetail"),
    key: "detail",
    width: 100,
    fixed: "right",
  },
]);
// Only the table body scrolls: the tab bar and toolbar stay pinned above and the standalone
// pagination is pinned below, so each body height has to follow the space its table is given.
const MIN_TABLE_BODY_HEIGHT = 120;
function useTableBodyHeight() {
  const area = ref();
  // Starting non-zero keeps the split header/body markup that later measurements rely on.
  const bodyHeight = ref(MIN_TABLE_BODY_HEIGHT);
  let observer;
  function measure() {
    const wrapper = area.value?.querySelector(".ant-table-wrapper");
    const body = wrapper?.querySelector(".ant-table-body");
    if (!area.value?.clientHeight || !wrapper || !body) return;
    // The pagination is a flex sibling of the table, so the body must leave room for it too.
    const pagination = area.value.querySelector(".pagination-bar");
    const chrome =
      wrapper.getBoundingClientRect().height -
      body.getBoundingClientRect().height +
      (pagination?.offsetHeight || 0);
    const next = Math.max(
      MIN_TABLE_BODY_HEIGHT,
      Math.floor(area.value.clientHeight - chrome),
    );
    if (next !== bodyHeight.value) bodyHeight.value = next;
  }
  watch(area, (element) => {
    observer?.disconnect();
    if (!element) return;
    observer = new ResizeObserver(measure);
    observer.observe(element);
  });
  onBeforeUnmount(() => observer?.disconnect());
  return { area, bodyHeight };
}
const { area: ruleAreaRef, bodyHeight: ruleBodyHeight } = useTableBodyHeight();
const { area: executionAreaRef, bodyHeight: executionBodyHeight } =
  useTableBodyHeight();
const activeNode = computed(() =>
  ruleConfig.value.nodes.find((item) => item.id === selectedStep.value),
);
const activeNodeIndex = computed(() =>
  ruleConfig.value.nodes.findIndex((item) => item.id === selectedStep.value),
);
// A node without a mode always opens on the picker; "change" reopens it for an already configured node.
const showModePicker = computed(
  () =>
    !activeNode.value?.mode || pickingModeNodeId.value === activeNode.value?.id,
);
const executionModes = computed(() => [
  {
    value: "direct",
    icon: markRaw(ThunderboltOutlined),
    iconClass: "direct-mode-icon",
    label: t("automation_direct"),
    description: t("automation_directDesc"),
  },
  {
    value: "conditional",
    icon: markRaw(ForkOutlined),
    iconClass: "conditional-mode-icon",
    label: t("automation_conditionalFull"),
    description: t("automation_conditionalDesc"),
  },
]);
// Branches are expanded by default, so only the explicitly collapsed ones are tracked.
const collapsedBranchIds = ref([]);
const openBranchKeys = computed({
  get() {
    return conditionalBranches(activeNode.value)
      .map((branch) => branch.id)
      .filter((id) => !collapsedBranchIds.value.includes(id));
  },
  set(keys) {
    collapsedBranchIds.value = conditionalBranches(activeNode.value)
      .map((branch) => branch.id)
      .filter((id) => !keys.includes(id));
  },
});
const eventCards = computed(() => [
  eventCard(
    "create",
    "automation_eventCreate",
    "automation_eventCreateDesc",
    PlusSquareOutlined,
  ),
  eventCard(
    "comment",
    "automation_eventComment",
    "automation_eventCommentDesc",
    CommentOutlined,
  ),
  eventCard(
    "status",
    "automation_eventStatus",
    "automation_eventStatusDesc",
    SyncOutlined,
  ),
  eventCard(
    "field",
    "automation_eventField",
    "automation_eventFieldDesc",
    EditOutlined,
  ),
]);
const triggerSummary = computed(() => {
  const object = t(`workitem_type_${ruleConfig.value.trigger.object}`);
  const event = eventCards.value.find(
    (item) => item.value === ruleConfig.value.trigger.event,
  )?.label;
  return event ? `${object} · ${event}` : t("automation_selectTrigger");
});
const commentChangeOptions = computed(() => [
  option("any", "automation_changeAny"),
  option("add", "automation_commentAdd"),
  option("edit", "automation_commentEdit"),
  option("delete", "automation_commentDelete"),
]);
const fieldChangeOptions = computed(() => [
  option("any", "automation_changeAny"),
  option("from", "automation_changeFrom"),
  option("to", "automation_changeTo"),
]);
const triggerStatuses = computed(() =>
  (
    metadata.value.workspace_metadata?.[ruleConfig.value.trigger.object]
      ?.statuses || []
  ).map((item) => ({ value: Number(item.id), label: item.name })),
);
const fromStatusOptions = computed(() => [
  { value: 0, label: t("automation_anyStatus") },
  ...triggerStatuses.value,
]);
const editableFieldOptions = computed(() =>
  (metadata.value.fields || [])
    .filter((item) => item.editable && !isCascaderJudgmentField(item))
    .map((item) => ({ value: item.key, label: item.label })),
);
const allFieldOptions = computed(() =>
  (metadata.value.fields || [])
    .filter((item) => !isCascaderJudgmentField(item))
    .map((item) => ({ value: item.key, label: item.label })),
);
const triggerField = computed(() =>
  fieldByKey(ruleConfig.value.trigger.field_key),
);
// Stale references are highlighted by their controls and fail completeness checks.
const triggerFieldInvalid = computed(() =>
  referenceMissing(
    editableFieldOptions.value,
    ruleConfig.value.trigger.field_key,
  ),
);
const triggerStatusInvalid = computed(() =>
  referenceMissing(
    triggerStatuses.value,
    ruleConfig.value.trigger.to_status_id,
  ),
);
const hasInvalidReferences = computed(() =>
  automationConfigHasInvalidReferences(ruleConfig.value, metadata.value),
);

// The condition value component reuses the matching Ant Design input by field type.
const ConditionValue = defineComponent({
  props: {
    modelValue: null,
    field: Object,
    operator: String,
    metadata: Object,
    triggerType: String,
    showError: Boolean,
  },
  emits: ["update:modelValue"],
  setup(componentProps, { emit }) {
    return () => {
      const field = componentProps.field || {};
      const options = conditionFieldOptions(
        field,
        componentProps.metadata,
        componentProps.triggerType,
      );
      const usesOptions =
        options.length > 0 ||
        ["select_single", "select_multi", "user", "user_multi"].includes(
          field.type,
        ) ||
        ["status", "priority", "assignee", "created_by", "version"].includes(
          field.key,
        );
      if (usesOptions)
        return h(Select, {
          value: componentProps.modelValue,
          options: withMissingOption(options, componentProps.modelValue),
          status:
            referenceMissing(options, componentProps.modelValue) ||
            componentProps.showError
              ? "error"
              : undefined,
          mode:
            ["select_multi", "user_multi"].includes(field.type) ||
            ["in", "not_in"].includes(componentProps.operator)
              ? "multiple"
              : undefined,
          style: "width:100%",
          "onUpdate:value": (value) => emit("update:modelValue", value),
        });
      if (["number", "numeric"].includes(field.type))
        return h(InputNumber, {
          value: componentProps.modelValue,
          status: componentProps.showError ? "error" : undefined,
          style: "width:100%",
          "onUpdate:value": (value) => emit("update:modelValue", value),
        });
      if (field.type === "datetime") {
        const valid = datetimeValueValid(componentProps.modelValue);
        return h(DatePicker, {
          value: valid ? dayjs(componentProps.modelValue) : null,
          showTime: { format: "HH:mm:ss" },
          format: "YYYY-MM-DD HH:mm:ss",
          status:
            componentProps.showError ||
            (conditionValuePresent(componentProps.modelValue) && !valid)
              ? "error"
              : undefined,
          style: "width:100%",
          onChange: (value) =>
            emit(
              "update:modelValue",
              value ? value.format("YYYY-MM-DD HH:mm:ss") : null,
            ),
        });
      }
      if (field.type === "date")
        return h(DatePicker, {
          value: componentProps.modelValue
            ? dayjs.unix(Number(componentProps.modelValue))
            : null,
          status: componentProps.showError ? "error" : undefined,
          style: "width:100%",
          onChange: (value) =>
            emit("update:modelValue", value ? value.unix() : null),
        });
      return h(Input, {
        value: componentProps.modelValue,
        status: componentProps.showError ? "error" : undefined,
        placeholder: t("automation_value"),
        "onUpdate:value": (value) => emit("update:modelValue", value),
      });
    };
  },
});

function option(value, key) {
  return { value, label: t(key) };
}
function isCascaderJudgmentField(field) {
  return ["cascader_single", "cascader_multi"].includes(field?.type);
}
function referenceMissing(options, value) {
  if (value === undefined || value === null || value === "") return false;
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
      label: isCascaderJudgmentField(fieldByKey(current))
        ? t("automation_unsupportedJudgmentField", {
            name: fieldByKey(current).label,
          })
        : t("automation_invalidOptionValue", { value: current }),
      class: "automation-invalid-option",
    }));
  return [...options, ...missing];
}
function isConditionFieldInvalid(condition) {
  return referenceMissing(allFieldOptions.value, condition.field_key);
}
function automationConfigHasInvalidReferences(config, source) {
  source = source || {};
  const fields = source.fields || [];
  const requirementFields = source.requirement_fields || [];
  const fieldKeys = new Set(fields.map((item) => item.key));
  const requirementKeys = new Set(requirementFields.map((item) => item.key));
  const statuses = {
    requirement: new Set(
      (source.workspace_metadata?.requirement?.statuses || []).map((item) =>
        Number(item.id),
      ),
    ),
    defect: new Set(
      (source.workspace_metadata?.defect?.statuses || []).map((item) =>
        Number(item.id),
      ),
    ),
  };
  const trigger = config.trigger || {};
  if (
    trigger.event === "field" &&
    trigger.field_key &&
    (!fieldKeys.has(trigger.field_key) ||
      isCascaderJudgmentField(
        fields.find((item) => item.key === trigger.field_key),
      ))
  )
    return true;
  const triggerFieldMeta = fields.find(
    (item) => item.key === trigger.field_key,
  );
  if (
    trigger.event === "field" &&
    trigger.field_change !== "any" &&
    metadataFieldValueInvalid(
      source,
      trigger.object,
      triggerFieldMeta,
      trigger.field_value,
    )
  )
    return true;
  if (
    trigger.event === "status" &&
    trigger.to_status_id &&
    !statuses[trigger.object]?.has(Number(trigger.to_status_id))
  )
    return true;
  for (const node of config.nodes || []) {
    const branches =
      node.mode === "conditional" ? conditionalBranches(node) : [];
    for (const branch of branches) {
      for (const module of branch.modules || []) {
        for (const condition of module.conditions || []) {
          if (
            condition.field_key &&
            (!fieldKeys.has(condition.field_key) ||
              isCascaderJudgmentField(
                fields.find((item) => item.key === condition.field_key),
              ))
          )
            return true;
          const field = fields.find((item) => item.key === condition.field_key);
          if (
            !["empty", "not_empty"].includes(condition.operator) &&
            metadataFieldValueInvalid(
              source,
              trigger.object,
              field,
              condition.value,
            )
          )
            return true;
        }
      }
    }
    const actions =
      node.mode === "direct"
        ? [node.action]
        : branches.flatMap((branch) => branch.actions || []);
    for (const action of actions) {
      if (
        action?.type === "status_transition" &&
        action.transitions?.some(
          (item) =>
            item.target_status_id &&
            !statuses[
              item.scope === "trigger" ? trigger.object : "requirement"
            ]?.has(Number(item.target_status_id)),
        )
      )
        return true;
      if (action?.type === "field_update") {
        const targetFields =
          action.scope === "trigger" ? fields : requirementFields;
        const targets = new Set(targetFields.map((item) => item.key));
        const targetType =
          action.scope === "trigger" ? trigger.object : "requirement";
        for (const update of action.updates || []) {
          if (update.field_key && !targets.has(update.field_key)) return true;
          if (update.value_source !== "fixed" && update.source_field_key) {
            const sourceKeys =
              update.value_source === "trigger" ? fieldKeys : requirementKeys;
            if (!sourceKeys.has(update.source_field_key)) return true;
          }
          const field = targetFields.find(
            (item) => item.key === update.field_key,
          );
          if (
            update.value_source === "fixed" &&
            metadataFieldValueInvalid(source, targetType, field, update.value)
          )
            return true;
        }
      }
      if (action?.type === "create_requirement") {
        if (
          action.status?.source === "fixed" &&
          action.status.value &&
          !statuses.requirement.has(Number(action.status.value))
        )
          return true;
        for (const key of ["priority", "version", "assignee"]) {
          const item = action[key];
          if (item?.source !== "fixed") continue;
          const field = requirementFields.find((meta) => meta.key === key);
          if (
            metadataFieldValueInvalid(
              source,
              "requirement",
              field,
              key === "assignee" ? item.values : item.value,
            )
          )
            return true;
        }
      }
      if (
        action?.type === "copy_work_item" &&
        action.target_workspace_id &&
        !(source.copy_targets || []).some(
          (item) => Number(item.id) === Number(action.target_workspace_id),
        )
      )
        return true;
    }
  }
  return false;
}
function metadataFieldValueInvalid(source, itemType, field, value) {
  if (!field || value === undefined || value === null || value === "")
    return false;
  const usesOptions =
    ["select_single", "select_multi", "user", "user_multi"].includes(
      field.type,
    ) ||
    ["status", "priority", "assignee", "created_by", "version"].includes(
      field.key,
    );
  if (!usesOptions) return false;
  if (
    ["user", "user_multi"].includes(field.type) &&
    !["assignee", "created_by"].includes(field.key)
  )
    return referenceMissing(
      conditionFieldOptions(field, source, itemType),
      value,
    );
  const group = source.workspace_metadata?.[itemType] || {};
  let options = (field.options || []).map((item) => ({ value: item.key }));
  if (field.key === "status")
    options = (group.statuses || []).map((item) => ({
      value: Number(item.id),
    }));
  if (field.key === "priority")
    options = (group.priorities || []).map((item) => ({
      value: Number(item.id),
    }));
  if (["assignee", "created_by"].includes(field.key))
    options = (source.workspace_metadata?.members || []).map((item) => ({
      value: Number(item.user_id),
    }));
  if (field.key === "version")
    options = (source.workspace_metadata?.versions || []).map((item) => ({
      value: Number(item.id),
    }));
  return referenceMissing(options, value);
}
function eventCard(value, labelKey, descriptionKey, icon) {
  return {
    value,
    icon: markRaw(icon),
    label: t(labelKey),
    description: t(descriptionKey),
  };
}
function nodeSummary(node) {
  if (!node.mode) return "";
  if (node.mode === "conditional") return t("automation_conditional");
  const key = {
    status_transition: "automation_actionStatus",
    field_update: "automation_actionField",
    add_comment: "automation_actionComment",
    create_requirement: "automation_actionCreate",
    copy_work_item: "automation_actionCopy",
    webhook: "automation_actionWebhook",
  }[node.action?.type];
  return key ? t(key) : t("automation_direct");
}
function uid() {
  return `${Date.now()}_${Math.random().toString(36).slice(2)}`;
}
// These factories only create the minimum editor state.
function newAction() {
  return { id: uid(), type: "add_comment", scope: "trigger", template: "" };
}
function newCondition() {
  return {
    id: uid(),
    field_key: undefined,
    operator: undefined,
    value: undefined,
    skip_when_empty: false,
  };
}
function newModule() {
  return { id: uid(), relation: "AND", conditions: [newCondition()] };
}
function newConditionalBranch() {
  return {
    id: uid(),
    module_relation: "AND",
    modules: [newModule()],
    actions: [newAction()],
  };
}
function conditionalBranches(node) {
  return node ? [node, ...(node.else_if || [])] : [];
}
function nodeModeLabel(node) {
  return node.mode === "conditional"
    ? t("automation_conditionalFull")
    : t("automation_direct");
}
function newNode() {
  // A new node starts without a mode so the editor opens on the execution mode picker.
  return { id: uid(), name: "", mode: "" };
}
function browserTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}
function newConfig() {
  return {
    schema_version: 1,
    timezone: browserTimezone(),
    trigger: { object: "requirement", event: "create" },
    nodes: [newNode()],
  };
}
function stripUI(value) {
  if (Array.isArray(value)) return value.map(stripUI);
  if (!value || typeof value !== "object") return value;
  return Object.fromEntries(
    Object.entries(value)
      .filter(([key, item]) => key !== "id" && item !== undefined)
      .map(([key, item]) => [key, stripUI(item)]),
  );
}
function hydrateConditionalBranch(branch) {
  branch.id = uid();
  (branch.modules || []).forEach((module) => {
    module.id = uid();
    module.conditions.forEach((condition) => (condition.id = uid()));
  });
  (branch.actions || []).forEach((action) => (action.id = uid()));
}
function hydrateConfig(config) {
  const result = structuredClone(config);
  result.timezone ||= browserTimezone();
  result.nodes.forEach((node) => {
    if (node.mode === "conditional") {
      node.else_if ||= [];
      conditionalBranches(node).forEach(hydrateConditionalBranch);
      return;
    }
    node.id = uid();
    if (node.action) node.action.id = uid();
  });
  return result;
}
function localRange(offset = 0) {
  const start = dayjs().add(offset, "day").startOf("day");
  return [start.unix(), start.add(1, "day").unix()];
}
// The server handles list search, date filtering, and pagination.
async function loadRules(page = ruleQuery.value.page) {
  ruleLoading.value = true;
  ruleQuery.value.page = page;
  const [todayStart, todayEnd] = localRange();
  const [yesterdayStart, yesterdayEnd] = localRange(-1);
  try {
    const result = await listAutomationRules(props.wsId, {
      ...ruleQuery.value,
      today_start: todayStart,
      today_end: todayEnd,
      yesterday_start: yesterdayStart,
      yesterday_end: yesterdayEnd,
    });
    rules.value = result?.list || [];
    ruleTotal.value = Number(result?.total || 0);
  } finally {
    ruleLoading.value = false;
  }
}
async function loadExecutions(page = executionQuery.value.page) {
  executionLoading.value = true;
  executionQuery.value.page = page;
  const params = { ...executionQuery.value };
  if (executionDates.value?.length === 2) {
    params.start_at = executionDates.value[0].startOf("day").unix();
    params.end_at = executionDates.value[1].add(1, "day").startOf("day").unix();
  }
  try {
    const result = await listAutomationExecutions(props.wsId, params);
    executions.value = result?.list || [];
    executionTotal.value = Number(result?.total || 0);
  } finally {
    executionLoading.value = false;
  }
}
// a-pagination reports page and size together; a size change may also move the current page.
function onRulePage(page, pageSize) {
  ruleQuery.value.size = pageSize;
  loadRules(page);
}
function onExecutionPage(page, pageSize) {
  executionQuery.value.size = pageSize;
  loadExecutions(page);
}
async function loadMetadata() {
  metadata.value = await getAutomationMetadata(
    props.wsId,
    ruleConfig.value.trigger.object,
    editingRuleId.value,
  );
}
async function openRule(record) {
  if (!canManage.value) return;
  editingRuleId.value = record?.id;
  ruleName.value = record?.name || "";
  ruleConfig.value = newConfig();
  validationAttempted.value = false;
  if (record) {
    const detail = await getAutomationRule(props.wsId, record.id);
    ruleName.value = detail.name;
    ruleConfig.value = hydrateConfig(detail.config);
  }
  // A defect rule opened while the app is off must not come back on a trigger object
  // whose card is hidden, or it could be saved without the user ever seeing it.
  if (!defectAppEnabled.value && ruleConfig.value.trigger.object === "defect")
    ruleConfig.value.trigger.object = "requirement";
  selectedStep.value = "trigger";
  await loadMetadata();
  if (triggerField.value?.type === "datetime") {
    const value = ruleConfig.value.trigger.field_value;
    const normalized =
      typeof value === "string" && value.length === 16 ? `${value}:00` : value;
    if (datetimeValueValid(normalized))
      ruleConfig.value.trigger.field_value = normalized;
  }
  ruleModalOpen.value = true;
}
async function selectTriggerObject(type) {
  if (ruleConfig.value.trigger.object === type) return;
  ruleConfig.value.trigger.object = type;
  await onTriggerTypeChanged();
}
// Turning the defect app off while a defect rule is being edited would strand the form on
// a hidden object, so move it back to requirement before the user continues.
watch(defectAppEnabled, (enabled) => {
  if (!enabled && ruleConfig.value.trigger.object === "defect")
    selectTriggerObject("requirement");
});
async function onTriggerTypeChanged() {
  resetTriggerDetails("create");
  await loadMetadata();
}
function resetTriggerDetails(event) {
  ruleConfig.value.trigger = {
    object: ruleConfig.value.trigger.object,
    event,
    ...(event === "comment" ? { comment_change: "any" } : {}),
    ...(event === "status" ? { from_status_id: 0 } : {}),
    ...(event === "field" ? { field_change: "any" } : {}),
  };
}
function addNode() {
  const node = newNode();
  ruleConfig.value.nodes.push(node);
  selectedStep.value = node.id;
}
// Next on the trigger step opens the first execution node.
function goNextStep() {
  const firstNode = ruleConfig.value.nodes[0];
  if (firstNode) selectedStep.value = firstNode.id;
}
function removeNode(index) {
  ruleConfig.value.nodes.splice(index, 1);
  selectedStep.value = "trigger";
}
// Switching the execution mode drops the other mode's fields; re-picking the same mode keeps the config.
function selectNodeMode(mode) {
  const node = activeNode.value;
  pickingModeNodeId.value = undefined;
  if (node.mode === mode) return;
  node.mode = mode;
  if (mode === "direct")
    Object.assign(node, {
      action: newAction(),
      modules: undefined,
      actions: undefined,
      module_relation: undefined,
      else_if: undefined,
    });
  else
    Object.assign(node, {
      action: undefined,
      module_relation: "AND",
      modules: [newModule()],
      actions: [newAction()],
      else_if: [],
    });
}
function fieldByKey(key) {
  return (metadata.value.fields || []).find((item) => item.key === key);
}
// Round-trip validation rejects normalized dates without changing stored wall time.
function datetimeValueValid(value) {
  return (
    typeof value === "string" &&
    /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value) &&
    dayjs(value).isValid() &&
    dayjs(value).format("YYYY-MM-DD HH:mm:ss") === value
  );
}
function triggerFieldValueValid() {
  const value = ruleConfig.value.trigger.field_value;
  return (
    conditionValuePresent(value) &&
    (triggerField.value?.type !== "datetime" || datetimeValueValid(value))
  );
}
function conditionValuePresent(value) {
  return (
    value !== undefined &&
    value !== null &&
    value !== "" &&
    (!Array.isArray(value) || value.length > 0)
  );
}
function resetCondition(condition) {
  condition.operator = undefined;
  condition.value = undefined;
}
function operatorOptions(key, currentOperator) {
  const type = fieldByKey(key)?.type;
  const byType = {
    text: ["contains", "not_contains", "is", "is_not", "empty", "not_empty"],
    textarea: [
      "contains",
      "not_contains",
      "is",
      "is_not",
      "empty",
      "not_empty",
    ],
    number: ["eq", "neq", "gt", "gte", "lt", "lte", "empty", "not_empty"],
    date: [
      "before",
      "before_eq",
      "after",
      "after_eq",
      "is",
      "is_not",
      "empty",
      "not_empty",
    ],
    user: ["is", "is_not", "empty", "not_empty"],
    select_single: ["is", "is_not", "in", "not_in", "empty", "not_empty"],
    user_multi: [
      "contains_any",
      "contains_all",
      "contains_none",
      "empty",
      "not_empty",
    ],
    select_multi: [
      "contains_any",
      "contains_all",
      "contains_none",
      "empty",
      "not_empty",
    ],
  };
  const operators =
    type === "user" &&
    (["assignee", "created_by"].includes(key) ||
      ["in", "not_in"].includes(currentOperator))
      ? ["is", "is_not", "in", "not_in", "empty", "not_empty"]
      : byType[
          type === "numeric"
            ? "number"
            : type === "phone"
              ? "text"
              : type === "datetime"
                ? "date"
                : type
        ] || [];
  return operators.map((value) => ({
    value,
    label: t(`automation_operator_${value}`),
  }));
}
function conditionFieldOptions(field, source, type) {
  if (
    ["user", "user_multi"].includes(field.type) &&
    !["assignee", "created_by"].includes(field.key)
  )
    return filterMembersForField(
      field,
      source.workspace_metadata?.custom_field_members || [],
    ).map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    }));
  if (field.options?.length)
    return field.options.map((item) => ({ value: item.key, label: item.name }));
  const group = source.workspace_metadata?.[type] || {};
  if (field.key === "status")
    return (group.statuses || []).map((item) => ({
      value: Number(item.id),
      label: item.name,
    }));
  if (field.key === "priority")
    return (group.priorities || []).map((item) => ({
      value: Number(item.id),
      label: item.label,
    }));
  if (["assignee", "created_by"].includes(field.key))
    return (source.workspace_metadata?.members || []).map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    }));
  if (field.key === "version")
    return (source.workspace_metadata?.versions || []).map((item) => ({
      value: Number(item.id),
      label: item.name,
    }));
  return [];
}
function templateValid(template) {
  if (!template?.trim()) return false;
  const allowed = new Set(availableVariables.value.map((item) => item.key));
  const matches = [...template.matchAll(/\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g)];
  const remainder = template.replace(/\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g, "");
  return (
    !remainder.includes("{{") &&
    !remainder.includes("}}") &&
    matches.every((match) => allowed.has(match[1]))
  );
}
function actionComplete(action) {
  if (!action?.type) return false;
  if (action.type === "status_transition")
    return (
      action.transitions?.length &&
      action.transitions.every((item) => item.scope && item.target_status_id)
    );
  if (action.type === "field_update")
    return (
      action.scope &&
      action.updates?.length &&
      action.updates.every(
        (item) =>
          item.field_key &&
          item.value_source &&
          (item.value_source === "fixed"
            ? conditionValuePresent(item.value)
            : item.source_field_key),
      )
    );
  if (action.type === "add_comment")
    return action.scope && templateValid(action.template);
  if (action.type === "create_requirement") {
    const optionFieldsComplete = [
      action.status,
      action.priority,
      action.version,
    ].every((item) => !item || item.source !== "fixed" || item.value);
    const assigneeComplete =
      !action.assignee ||
      action.assignee.source !== "fixed" ||
      action.assignee.values?.length;
    const descriptionValid =
      !action.description?.trim() || templateValid(action.description);
    return (
      action.parent &&
      templateValid(action.template) &&
      optionFieldsComplete &&
      assigneeComplete &&
      descriptionValid
    );
  }
  if (action.type === "copy_work_item")
    return action.target_workspace_id && action.parent;
  if (action.type === "webhook") {
    if (!templateValid(action.url)) return false;
    return (action.push_system || "custom") === "custom"
      ? templateValid(action.body)
      : templateValid(action.template);
  }
  return false;
}
function conditionalBranchComplete(branch) {
  if (
    !branch.actions?.length ||
    branch.actions.some((action) => !actionComplete(action)) ||
    !branch.modules?.length
  )
    return false;
  return branch.modules.every(
    (module) =>
      module.conditions?.length &&
      module.conditions.every(
        (condition) =>
          condition.field_key &&
          condition.operator &&
          (["empty", "not_empty"].includes(condition.operator) ||
            (conditionValuePresent(condition.value) &&
              (fieldByKey(condition.field_key)?.type !== "datetime" ||
                datetimeValueValid(condition.value)))),
      ),
  );
}
// Validate the editor immediately; the backend remains the strict authority.
function configComplete() {
  const trigger = ruleConfig.value.trigger;
  if (
    hasInvalidReferences.value ||
    !ruleName.value.trim() ||
    !trigger.object ||
    !trigger.event
  )
    return false;
  if (trigger.event === "comment" && !trigger.comment_change) return false;
  if (trigger.event === "status" && !trigger.to_status_id) return false;
  if (
    trigger.event === "field" &&
    (!trigger.field_key ||
      !trigger.field_change ||
      (trigger.field_change !== "any" && !triggerFieldValueValid()))
  )
    return false;
  return ruleConfig.value.nodes.every((node) => {
    if (node.mode === "direct") return actionComplete(node.action);
    if (node.mode === "conditional")
      return conditionalBranches(node).every(conditionalBranchComplete);
    return false;
  });
}
// Create and edit share config cleanup and submission.
async function saveRule() {
  if (!canManage.value) return;
  validationAttempted.value = true;
  if (!configComplete()) {
    message.warning(t("automation_completeConfig"));
    return;
  }
  savingRule.value = true;
  // Persist the user's IANA timezone so async execution does not depend on the server locale.
  ruleConfig.value.timezone = browserTimezone();
  const payload = {
    name: ruleName.value.trim(),
    locale: localeStore.currentLang,
    config: stripUI(ruleConfig.value),
  };
  try {
    if (editingRuleId.value)
      await updateAutomationRule(props.wsId, editingRuleId.value, payload);
    else await createAutomationRule(props.wsId, payload);
    message.success(t("common_saveSuccess"));
    ruleModalOpen.value = false;
    await loadRules(1);
  } finally {
    savingRule.value = false;
  }
}
async function toggleRule(record, enabled) {
  if (!canManage.value) return;
  await setAutomationRuleEnabled(props.wsId, record.id, enabled);
  record.enabled = enabled ? 1 : 0;
}
async function openCopyRule(record) {
  if (!canManage.value) return;
  copyRecord.value = record;
  copyProjects.value = [];
  copyTargetIds.value = [Number(props.wsId)];
  copyModalOpen.value = true;
  copyProjectsLoading.value = true;
  try {
    const projects = (await listAutomationCopyTargets(props.wsId)) || [];
    copyProjects.value = [...projects].sort((a, b) => {
      const current = Number(props.wsId);
      if (Number(a.id) === current) return -1;
      if (Number(b.id) === current) return 1;
      return 0;
    });
    if (
      !copyProjects.value.some(
        (project) => Number(project.id) === Number(props.wsId),
      )
    ) {
      copyTargetIds.value = [];
    }
  } catch {
    copyProjects.value = [];
    copyTargetIds.value = [];
  } finally {
    copyProjectsLoading.value = false;
  }
}

async function submitCopyRule() {
  if (!canManage.value) return;
  const record = copyRecord.value;
  if (!record || !copyTargetIds.value.length) {
    message.warning(t("automation_copySelectRequired"));
    return;
  }
  copyingRuleId.value = record.id;
  copySubmitting.value = true;
  try {
    const detail = await getAutomationRule(props.wsId, record.id);
    const config = detail?.config || {};
    const copyMetadata = await getAutomationMetadata(
      props.wsId,
      config.trigger?.object || "requirement",
      record.id,
    );
    if (automationConfigHasInvalidReferences(config, copyMetadata)) {
      message.warning(t("automation_copyBlocked"));
      return;
    }
    const result = await copyAutomationRule(props.wsId, record.id, {
      workspace_ids: copyTargetIds.value.map((id) => Number(id)),
    });
    message.success(
      t("automation_copySuccess", { count: Number(result?.count || 0) }),
    );
    copyModalOpen.value = false;
    if (copyTargetIds.value.some((id) => Number(id) === Number(props.wsId))) {
      await loadRules(1);
    }
  } finally {
    copySubmitting.value = false;
    copyingRuleId.value = undefined;
  }
}
async function removeRule(record) {
  if (!canManage.value) return;
  await deleteAutomationRule(props.wsId, record.id);
  await loadRules(1);
}
function jumpToExecutions(record, period) {
  executionQuery.value = {
    page: 1,
    size: 20,
    search: "",
    rule_id: Number(record.id),
  };
  executionDates.value =
    period === "total"
      ? []
      : [
          dayjs().add(period === "yesterday" ? -1 : 0, "day"),
          dayjs().add(period === "yesterday" ? -1 : 0, "day"),
        ];
  subTab.value = "executions";
  loadExecutions(1);
}
function clearExecutionFilter() {
  executionQuery.value = { page: 1, size: 20, search: "", rule_id: undefined };
  executionDates.value = [];
  loadExecutions(1);
}
async function openExecution(record) {
  activeExecution.value = await getAutomationExecution(props.wsId, record.id);
  executionModalOpen.value = true;
}
function failureReason(reason) {
  if (!reason) return "-";
  return reason === "automation_execution_failed"
    ? t("automation_executionFailed")
    : reason;
}
function statusColor(status) {
  return {
    pending: "default",
    running: "processing",
    success: "success",
    skipped: "warning",
    failed: "error",
  }[status];
}
function executionDuration(item) {
  if (!Number(item.started_at) || !Number(item.finished_at)) return "-";
  return `${Number(item.finished_at) - Number(item.started_at)}s`;
}

onMounted(() => {
  loadRules();
  loadExecutions();
});
</script>

<style scoped lang="less">
// The tab owns the whole settings body height so nothing but the table bodies scrolls.
.automation-tab {
  display: flex;
  min-height: 520px;
  height: 100%;
  flex-direction: column;
  :deep(.ant-tabs) {
    display: flex;
    flex: 1;
    min-height: 0;
    flex-direction: column;
  }
  :deep(.ant-tabs-nav) {
    flex: none;
  }
  :deep(.ant-tabs-content-holder) {
    display: flex;
    min-height: 0;
  }
  :deep(.ant-tabs-content) {
    height: 100%;
  }
  :deep(.ant-tabs-tabpane) {
    display: flex;
    min-height: 0;
    height: 100%;
    flex-direction: column;
  }
}
.pane {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
}
.table-area {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  :deep(.ant-table-wrapper) {
    flex: none;
  }
}
// Pinned to the bottom of the remaining space, even when the list has few rows.
.pagination-bar {
  display: flex;
  flex: none;
  justify-content: flex-end;
  margin-top: auto;
  padding-top: 16px;
  :deep(.ant-pagination) {
    flex-wrap: wrap;
    row-gap: 8px;
  }
}
.tab-title,
.modal-title,
.editor-title,
:deep(.icon-option) {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
:deep(.icon-option) {
  color: #262626;
  > .anticon {
    color: #1677ff;
  }
}
.toolbar {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  .ant-input-search {
    width: 240px;
  }
}
.execution-toolbar {
  justify-content: flex-start;
}
.copy-section-title {
  margin-bottom: 8px;
  color: #182b50;
  font-weight: 600;
}
.copy-project-group {
  display: flex;
  width: 100%;
  max-height: 260px;
  flex-direction: column;
  flex-wrap: nowrap;
  gap: 8px;
  overflow: auto;
}
.copy-project-item {
  display: flex;
  align-items: center;
  margin: 0;
  border: 1px solid #eef1f6;
  border-radius: 8px;
  padding: 9px 12px;
  color: #182b50;
}
.subject-title,
.execution-ellipsis {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.subject-title {
  display: block;
  width: 190px;
}
.execution-ellipsis {
  display: block;
  max-width: 100%;
}
.failed {
  color: #ff4d4f;
}
:global(.automation-rule-modal .ant-modal-content) {
  overflow: hidden;
  border-radius: 8px;
}
:global(.automation-rule-modal .ant-modal-header) {
  margin-bottom: 0;
  padding: 16px 24px;
  border-bottom: 1px solid #f0f0f0;
}
.rule-modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-right: 32px;
  > span {
    flex: none;
    font-size: 14px;
    font-weight: 600;
  }
  .required-label::before {
    margin-right: 4px;
    color: #ff4d4f;
    content: "*";
  }
  .ant-input {
    width: 280px;
    height: 36px;
    font-size: 14px;
  }
}
.invalid-reference-help {
  display: block;
  margin-top: -6px;
  color: #ff4d4f;
  font-size: 12px;
}
:global(.automation-invalid-option) {
  color: #ff4d4f !important;
}
.invalid-reference-control :deep(.ant-select-selector) {
  border-color: #ff4d4f;
}
.rule-editor {
  display: flex;
  height: 560px;
}
.flow-nav {
  display: flex;
  flex: none;
  flex-direction: column;
  gap: 12px;
  width: 220px;
  padding: 24px 16px;
  overflow-y: auto;
  background: #f8fafc;
  border-right: 1px solid #f0f0f0;
  > button {
    position: relative;
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    min-height: 56px;
    padding: 12px 14px;
    text-align: left;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
    &.active {
      background: #fff;
      border-color: #1677ff;
      box-shadow: 0 2px 8px rgb(22 119 255 / 8%);
    }
  }
  > .ant-btn {
    min-height: 38px;
    color: #1677ff;
    background: #fff;
    border-color: #1677ff;
  }
  .nav-copy {
    flex: 1;
    min-width: 0;
    strong,
    small {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    strong {
      color: #262626;
      font-size: 13px;
      font-weight: 500;
    }
    small {
      margin-top: 3px;
      color: #8c8c8c;
      font-size: 11px;
    }
  }
  .remove-node {
    position: absolute;
    top: 8px;
    right: 8px;
    color: #ff4d4f;
    font-size: 14px;
  }
}
.flow-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  color: #fff;
  border-radius: 4px;
}
.trigger-icon {
  background: #1677ff;
}
.action-icon {
  background: #52c41a;
}
.step-editor {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 24px;
  h3 {
    margin: 0 0 20px;
    font-size: 15px;
  }
}
.step-editor-content {
  flex: 1;
  min-height: 0;
  padding-right: 2px;
  overflow-y: auto;
}
.editor-footer {
  display: flex;
  flex: none;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid #f0f0f0;
}
.config-section {
  margin-bottom: 24px;
}
.section-label {
  margin-bottom: 12px;
  color: #262626;
  font-size: 13px;
  font-weight: 500;
}
.object-grid {
  display: flex;
  gap: 16px;
}
.object-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  width: 120px;
  padding: 16px 0;
  color: #262626;
  font-size: 13px;
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  &:hover,
  &.active {
    background: #f0f5ff;
    border-color: #1677ff;
  }
  &.defect-card.active {
    background: #fff0f6;
  }
}
.object-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  font-size: 18px;
  border-radius: 50%;
}
.requirement-icon {
  color: #1677ff;
  background: #e6f4ff;
}
.defect-icon {
  color: #eb2f96;
  background: #fff0f6;
}
.event-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}
.event-card {
  min-height: 92px;
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
  }
  small {
    display: block;
    margin-top: 6px;
    color: #8c8c8c;
    font-size: 12px;
    line-height: 1.45;
  }
}
.event-card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #262626;
  font-size: 13px;
  font-weight: 500;
  .anticon {
    color: #1677ff;
    font-size: 16px;
  }
}
.trigger-detail-form {
  margin-top: 4px;
}
.node-content {
  margin-top: 0;
}
.two-columns {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.node-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  .ant-input {
    width: 240px;
  }
}
.node-type {
  color: #262626;
  font-size: 14px;
}
.mode-picker-title {
  margin: 8px 0 24px;
  font-size: 16px;
  text-align: center;
}
.mode-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
.mode-card {
  padding: 0;
  overflow: hidden;
  text-align: left;
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  &:hover,
  &.active {
    border-color: #1677ff;
    box-shadow: 0 2px 8px rgb(22 119 255 / 8%);
  }
  small {
    display: block;
    padding: 16px;
    color: #8c8c8c;
    font-size: 12px;
    line-height: 1.6;
  }
}
.mode-card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px;
  color: #262626;
  font-size: 14px;
  font-weight: 500;
  border-bottom: 1px solid #f0f0f0;
}
.mode-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  color: #fff;
  font-size: 14px;
  border-radius: 50%;
}
.direct-mode-icon {
  background: #52c41a;
}
.conditional-mode-icon {
  background: #1677ff;
}
.branch-collapse {
  margin-bottom: 16px;
  :deep(.ant-collapse-header) {
    align-items: center;
    background: #f7f9fc;
  }
}
.branch-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #262626;
  font-size: 14px;
  font-weight: 500;
}
.branch-remove {
  color: #8c8c8c;
  font-size: 14px;
  &:hover {
    color: #ff4d4f;
  }
}
.branch-section-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
.branch-section-label {
  color: #262626;
  font-size: 13px;
  font-weight: 500;
}
.section-add {
  margin-left: auto;
}
.module-extra {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.condition-card,
.action-card {
  margin: 12px 0 16px;
  border-color: #f0f0f0;
  border-radius: 8px;
}
.relation-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.relation-hint {
  color: #8c8c8c;
  font-size: 13px;
}
.module-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}
.module-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  color: #fff;
  font-size: 12px;
  background: #1677ff;
  border-radius: 4px;
}
.action-title .module-icon {
  background: #52c41a;
}
.condition-row {
  display: grid;
  // The skip checkbox sizes to its label so neither locale wraps it onto a second line.
  grid-template-columns: 1fr 1fr 1.2fr max-content 36px;
  gap: 8px;
  align-items: center;
  margin-top: 10px;
}
.execution-summary {
  display: grid;
  gap: 12px;
  font-size: 13px;
  b {
    color: #8c8c8c;
    font-weight: 400;
  }
}
@media (max-width: 900px) {
  .event-grid {
    grid-template-columns: 1fr 1fr;
  }
}
.terminal {
  max-height: 360px;
  margin-top: 12px;
  padding: 16px;
  overflow: auto;
  color: #c9d1d9;
  font: 12px/1.7 monospace;
  background: #0d1117;
  border-radius: 8px;
  pre {
    margin: 3px 0 8px;
    color: #8b949e;
    white-space: pre-wrap;
    word-break: break-all;
  }
  span {
    color: #58a6ff;
  }
}
</style>
