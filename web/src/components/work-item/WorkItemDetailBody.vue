<template>
  <div
    v-if="mode === 'drawer'"
    class="drawer-resize-handle"
    role="separator"
    tabindex="0"
    aria-orientation="vertical"
    :aria-label="$t('requirement_resize_detail')"
    :aria-valuemin="Math.min(drawerMinWidth, viewportWidth)"
    :aria-valuemax="Math.min(drawerMaxWidth, viewportWidth)"
    :aria-valuenow="drawerWidth"
    @keydown="resizeDrawerByKeyboard"
    @mousedown="startDrawerResize"
  />
  <a-spin v-if="loading" class="drawer-loading" />
  <div
    v-else-if="detail"
    class="detail-wrap"
    :class="{
      narrow: isNarrowLayout,
      'issue-detail': isIssueDetail,
      modal: mode === 'modal',
      page: mode === 'page',
      'description-editing': descriptionEditing,
    }"
    :style="descriptionEditing ? descriptionEditingStyle : undefined"
  >
    <div
      ref="detailLeftRef"
      class="detail-left"
      :class="{ expanded: propertiesCollapsed }"
    >
      <div class="detail-tabs-row">
        <RichTextPreviewActions
          v-if="isIssueDetail"
          v-show="detailTab === 'desc'"
          class="issue-description-actions"
          :read-only="!canEdit"
          @edit="descriptionEditorRef?.enterEdit()"
          @preview="descriptionEditorRef?.enterFullscreenPreview()"
        />
        <nav
          v-if="isIssueDetail"
          class="issue-tabs-nav"
          :aria-label="$t('issue_detail')"
        >
          <button
            v-for="item in primaryTabs"
            :key="item.key"
            type="button"
            :class="{ active: detailTab === item.key }"
            :aria-pressed="detailTab === item.key"
            :disabled="descriptionEditing"
            @click="selectMoreTab(item.key)"
          >
            {{ item.label }}
          </button>
          <a-dropdown
            v-if="isIssueDetail"
            :trigger="['click']"
            :disabled="descriptionEditing"
          >
            <a-button
              type="text"
              size="small"
              :disabled="descriptionEditing"
              class="issue-more-button"
              :class="{
                'issue-more-active': moreTabs.some(
                  (item) => item.key === detailTab,
                ),
              }"
            >
              {{ $t("common_more") }} <DownOutlined />
            </a-button>
            <template #overlay>
              <a-menu
                :selected-keys="[detailTab]"
                @click="({ key }) => selectMoreTab(key)"
              >
                <a-menu-item v-for="item in moreTabs" :key="item.key">{{
                  item.label
                }}</a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
          <a-button
            v-if="propertiesCollapsed"
            type="text"
            size="small"
            class="properties-toggle expand"
            @click="propertiesCollapsed = false"
            >{{ $t("requirement_expand_properties") }}</a-button
          >
        </nav>
        <a-tabs
          v-model:activeKey="detailTab"
          :tab-bar-style="isIssueDetail ? { display: 'none' } : undefined"
          size="small"
          :animated="false"
          @change="handleDetailTabChange"
          class="detail-tabs"
        >
          <template #rightExtra>
            <div class="detail-tab-actions">
              <RichTextPreviewActions
                v-if="!isIssueDetail"
                v-show="detailTab === 'desc'"
                :read-only="!canEdit"
                @edit="descriptionEditorRef?.enterEdit()"
                @preview="descriptionEditorRef?.enterFullscreenPreview()"
              />
              <a-button
                v-if="propertiesCollapsed"
                type="link"
                size="small"
                class="properties-toggle expand"
                @click="propertiesCollapsed = false"
              >
                {{ $t("requirement_expand_properties") }}
              </a-button>
            </div>
          </template>
          <a-tab-pane key="desc" :tab="$t('requirement_description')">
            <RichTextEditor
              ref="descriptionEditorRef"
              v-model="descriptionDraft"
              v-model:image-file-ids="imageFileIds"
              v-model:content-mode="descriptionModeDraft"
              :content-revision="draftRestoreRevision"
              :ws-id="wsId"
              :read-only="!canEdit"
              :min-height="160"
              compact-actions
              external-compact-actions
              @save="saveDescription"
              @cancel="handleDescriptionCancel"
              @editing-change="descriptionEditing = $event"
              @maximize="propertiesCollapsed = true"
            >
              <template #editing-extra>
                <LocalDraftTips :draft="draft" @open="openDraftModal" />
              </template>
            </RichTextEditor>
          </a-tab-pane>
          <a-tab-pane
            v-if="detail.type === 'requirement'"
            key="children"
            :disabled="descriptionEditing"
          >
            <template #tab
              ><span :class="{ 'detail-secondary-tab': isIssueDetail }">{{
                `${$t("requirement_children")} (${childRequirements.length})`
              }}</span></template
            >
            <a-empty
              v-if="!childRequirements.length"
              :image="simpleImage"
              :description="$t('requirement_noChildren')"
            />
            <a-button
              v-if="canCreateType('requirement')"
              type="link"
              class="quick-add"
              @click="goCreate('requirement')"
            >
              <PlusOutlined /> {{ $t("requirement_quick_add_child") }}
            </a-button>
            <div v-if="childRequirements.length" class="relation-table">
              <div class="relation-row relation-head">
                <span>{{ $t("workspace_workItemTitle") }}</span>
                <span>{{ $t("workspace_status") }}</span>
                <span>{{ $t("workspace_priority") }}</span>
                <span>{{ $t("workspace_assignee") }}</span>
              </div>
              <button
                v-for="item in childRequirements"
                :key="item.id"
                type="button"
                class="relation-row"
                :class="{ 'final-status-row': isFinalStatus(item) }"
                @click="$emit('navigate', item.id, wsId)"
              >
                <strong>{{ item.title }}</strong>
                <StatusPill
                  :text="item.status_name"
                  :color="item.status_color"
                />
                <PriorityPill
                  :priority="item.priority"
                  :label="item.priority_name"
                  :color="item.priority_color"
                />
                <AssigneeCell
                  :ids="item.assignee_ids"
                  :names="item.assignee_names"
                  :avatars="item.assignee_avatars"
                  :usernames="item.assignee_usernames"
                />
              </button>
            </div>
          </a-tab-pane>
          <a-tab-pane
            v-if="defectAppEnabled && detail.type === 'requirement'"
            key="defects"
            :disabled="descriptionEditing"
          >
            <template #tab
              ><span :class="{ 'detail-secondary-tab': isIssueDetail }">{{
                `${$t("requirement_relatedDefects")} (${relatedDefects.length})`
              }}</span></template
            >
            <a-empty
              v-if="!relatedDefects.length"
              :image="simpleImage"
              :description="$t('requirement_noRelatedDefects')"
            />
            <a-button
              v-if="defectAppEnabled && canCreateType('defect')"
              type="link"
              class="quick-add"
              @click="goCreate('defect')"
            >
              <PlusOutlined /> {{ $t("requirement_quick_add_defect") }}
            </a-button>
            <div v-if="relatedDefects.length" class="relation-table">
              <div class="relation-row relation-head">
                <span>{{ $t("workspace_workItemTitle") }}</span>
                <span>{{ $t("workspace_status") }}</span>
                <span>{{ $t("workspace_priority") }}</span>
                <span>{{ $t("workspace_assignee") }}</span>
              </div>
              <button
                v-for="item in relatedDefects"
                :key="item.id"
                type="button"
                class="relation-row"
                :class="{ 'final-status-row': isFinalStatus(item) }"
                @click="$emit('navigate', item.id, wsId)"
              >
                <strong>{{ item.title }}</strong>
                <StatusPill
                  :text="item.status_name"
                  :color="item.status_color"
                />
                <PriorityPill
                  :priority="item.priority"
                  :label="item.priority_name"
                  :color="item.priority_color"
                />
                <AssigneeCell
                  :ids="item.assignee_ids"
                  :names="item.assignee_names"
                  :avatars="item.assignee_avatars"
                  :usernames="item.assignee_usernames"
                />
              </button>
            </div>
          </a-tab-pane>
          <a-tab-pane
            key="history"
            :disabled="descriptionEditing"
            :tab="`${$t('requirement_changeHistory')} (${
              historyLoaded ? histories.length : detail.history_count || 0
            })`"
          >
            <a-spin v-if="historyLoading" />
            <a-empty
              v-else-if="!histories.length"
              :image="simpleImage"
              :description="$t('requirement_noHistory')"
            />
            <div v-else class="history-table">
              <div class="history-row history-head">
                <span>#</span>
                <span>{{ $t("requirement_change_time") }}</span>
                <span>{{ $t("requirement_changed_by") }}</span>
                <span>{{ $t("requirement_change_type") }}</span>
                <span>{{ $t("requirement_changed_field") }}</span>
                <span>{{ $t("requirement_before_change") }}</span>
                <span>{{ $t("requirement_after_change") }}</span>
              </div>
              <div
                v-for="(item, index) in histories"
                :key="item.id"
                class="history-row"
              >
                <span>{{ histories.length - index }}</span>
                <span>{{ formatLocalDate(item.created_at, true) }}</span>
                <span>{{ item.user_name || "-" }}</span>
                <span class="history-action" :class="item.action">{{
                  historyActionText(item)
                }}</span>
                <span>{{ historyFieldText(item) }}</span>
                <span class="history-before">
                  <a
                    v-if="isDescriptionHistory(item)"
                    class="history-detail-link"
                    href="#"
                    @click.prevent="openHistoryDetail(item)"
                    >{{ $t("requirement_history_viewDetail") }}</a
                  >
                  <template v-else>{{
                    historyValueText(item, item.old_value)
                  }}</template>
                </span>
                <strong>
                  <a
                    v-if="isDescriptionHistory(item)"
                    class="history-detail-link"
                    href="#"
                    @click.prevent="openHistoryDetail(item)"
                    >{{ $t("requirement_history_viewDetail") }}</a
                  >
                  <template v-else>{{
                    historyValueText(item, item.new_value)
                  }}</template>
                </strong>
              </div>
            </div>
          </a-tab-pane>
          <a-tab-pane
            v-if="
              detail.type === 'requirement' &&
              fieldEnabled(detail.type, 'completed_work_hours')
            "
            key="expense"
            :disabled="descriptionEditing"
          >
            <template #tab
              ><span :class="{ 'detail-secondary-tab': isIssueDetail }">{{
                `${$t("requirement_expense")} (${detail.work_log_count || 0})`
              }}</span></template
            >
            <RequirementWorkLogs
              ref="workLogsRef"
              :ws-id="wsId"
              :requirement-id="requirementId"
              :can-edit="canEdit"
              @summary-change="handleWorkLogSummary"
            />
          </a-tab-pane>
          <a-tab-pane
            v-if="
              detail.type === 'requirement' &&
              !workspaceMetadata?.settings?.sla_activated_at
            "
            key="aging"
            :disabled="descriptionEditing"
            :tab="$t('aging_column')"
          >
            <RequirementAging
              v-if="detailTab === 'aging'"
              :ws-id="wsId"
              :requirement-id="requirementId"
              :reload-key="detailReloadKey"
            />
          </a-tab-pane>
          <a-tab-pane
            v-if="
              detail.type === 'requirement' &&
              workspaceMetadata?.workspace?.workspace_type === 'issue' &&
              workspaceMetadata?.settings?.sla_enabled
            "
            key="sla"
            :disabled="descriptionEditing"
            :tab="$t('sla_title')"
          >
            <WorkItemSLA
              v-if="detailTab === 'sla'"
              :ws-id="wsId"
              :requirement-id="requirementId"
              :reload-key="detailReloadKey"
              @disabled="
                workspaceMetadata.settings.sla_enabled = false;
                detailTab = 'desc';
              "
            />
          </a-tab-pane>
          <!-- Agent tab: client execution records synced from TeamsBoard,
                 available on every work-item type and entry point. -->
          <a-tab-pane key="agent" :disabled="descriptionEditing">
            <template #tab
              ><span :class="{ 'detail-secondary-tab': isIssueDetail }">{{
                $t("agent_tab_title")
              }}</span></template
            >
            <RequirementAgentTasks
              v-if="detailTab === 'agent'"
              :ws-id="wsId"
              :requirement-id="requirementId"
              :work-item-type="detail.type || 'requirement'"
            />
          </a-tab-pane>
        </a-tabs>
      </div>

      <div class="comments-container">
        <RequirementComments
          :open="open"
          :ws-id="wsId"
          :requirement-id="requirementId"
          :can-edit="canEdit"
          :description-editing="descriptionEditing"
          :input-style="commentInputStyle"
          :attachments="attachments"
          :mention-members="members"
          :invite-filler="Boolean(detail.invite_openid)"
          @refresh-attachments="reloadAttachments"
        />
      </div>
    </div>

    <aside v-if="!propertiesCollapsed" class="detail-right">
      <a-tabs
        v-if="isIssueDetail"
        v-model:activeKey="sidebarTab"
        size="small"
        :animated="false"
        class="issue-sidebar-tabs"
      >
        <template #rightExtra
          ><a-button
            type="text"
            size="small"
            class="properties-toggle"
            @click="propertiesCollapsed = true"
            >{{ $t("common_collapse") }}</a-button
          ></template
        >
        <a-tab-pane key="processing" :tab="$t('issue_processing_records')" />
        <a-tab-pane key="properties" :tab="$t('requirement_properties')" />
      </a-tabs>
      <IssueProcessingRecords
        v-if="isIssueDetail && sidebarTab === 'processing'"
        :records="processingHistories"
        :loading="historyLoading"
        :loaded="historyLoaded"
        :horizontal="isNarrowLayout"
        :value-text="historyValueText"
        :status-color="historyStatusColor"
        @retry="handleDetailTabChange('processing')"
      />
      <template v-if="!isIssueDetail || sidebarTab === 'properties'">
        <div v-if="!isIssueDetail" class="properties-header">
          <h5>{{ $t("requirement_properties") }}</h5>
          <a-button
            type="link"
            size="small"
            class="properties-toggle"
            @click="propertiesCollapsed = true"
          >
            {{ $t("requirement_collapse_properties") }}
          </a-button>
        </div>
        <div class="properties-card">
          <div class="prop-item parent-prop-item">
            <span class="prop-label">{{
              $t(
                detail.type === "defect"
                  ? "requirement_relatedRequirement"
                  : "requirement_parentReq",
              )
            }}</span>
            <div class="parent-property-control">
              <RequirementRelationPicker
                v-if="canEdit && (hasParent || editingParent)"
                :model-value="hasParent ? detail.parent_id : undefined"
                :ws-id="wsId"
                :selected-title="detail.parent_title || ''"
                :exclude-id="detail.id"
                :mode="detail.type === 'defect' ? 'related' : 'parent'"
                compact
                @change="updateParent"
              />
              <span
                v-else
                class="prop-value"
                :class="{ editable: canEdit }"
                @click="canEdit && (editingParent = true)"
                >{{ detail.parent_title || "-" }}</span
              >
              <a-tooltip v-if="hasParent" :title="$t('planning_openInNewTab')">
                <a-button
                  type="text"
                  size="small"
                  class="parent-open-button"
                  :aria-label="$t('planning_openInNewTab')"
                  @click.stop="openParentInNewWindow"
                >
                  <ExportOutlined />
                </a-button>
              </a-tooltip>
            </div>
          </div>
          <div
            v-if="fieldEnabled(detail.type, 'status')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'status')"
          >
            <span class="prop-label">{{ $t("workspace_status") }}</span>
            <a-select
              v-if="
                editingProperty === 'status' &&
                fieldEditable(detail.type, 'status') &&
                !statusTransitionOn
              "
              :value="Number(detail.status_id)"
              size="small"
              default-open
              autofocus
              style="width: 140px"
              @change="(value) => saveProperty('status_id', value)"
              @dropdownVisibleChange="(open) => !open && (editingProperty = '')"
            >
              <a-select-option
                v-for="item in statusOptions"
                :key="item.id"
                :value="item.id"
                >{{ displayName(item.name) }}</a-select-option
              >
            </a-select>
            <span
              v-else
              class="property-pill-trigger"
              :class="{ editable: fieldEditable(detail.type, 'status') }"
              @click="openStatusEditor()"
            >
              <StatusPill
                :text="detail.status_name"
                :color="detail.status_color"
              />
            </span>
          </div>
          <div
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'status')"
          >
            <span class="prop-label">{{ $t("settings_template") }}</span>
            <span class="prop-value" :title="detail.template_name || undefined">
              {{ detail.template_name || "—" }}
            </span>
          </div>
          <div
            v-if="
              detail.type === 'requirement' &&
              fieldEnabled(detail.type, 'category')
            "
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'category')"
          >
            <span class="prop-label">{{ $t("workspace_field_category") }}</span>
            <a-select
              v-if="canAssignCategory"
              :value="Number(detail.category_id) || 0"
              :options="categoryOptions"
              :aria-label="$t('workspace_field_category')"
              :loading="categorySaving"
              :disabled="categorySaving"
              size="small"
              @change="saveCategory"
            />
            <span v-else class="prop-value">
              {{ requirementCategoryName(detail.category_id) }}
            </span>
          </div>
          <div
            v-if="fieldEnabled(detail.type, 'priority')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'priority')"
          >
            <span class="prop-label">{{ $t("workspace_priority") }}</span>
            <a-select
              v-if="
                editingProperty === 'priority' &&
                fieldEditable(detail.type, 'priority')
              "
              :value="Number(detail.priority) || undefined"
              size="small"
              default-open
              allowClear
              autofocus
              style="width: 110px"
              @change="(value) => saveProperty('priority', value ?? null)"
              @dropdownVisibleChange="(open) => !open && (editingProperty = '')"
            >
              <a-select-option
                v-for="item in priorities[detail.type] || []"
                :key="item.id"
                :value="item.id"
                >{{ displayName(item.name || item.label) }}</a-select-option
              >
            </a-select>
            <span
              v-else
              class="property-pill-trigger"
              :class="{ editable: fieldEditable(detail.type, 'priority') }"
              @click="
                fieldEditable(detail.type, 'priority') &&
                (editingProperty = 'priority')
              "
            >
              <PriorityPill
                :priority="detail.priority"
                :label="detail.priority_name"
                :color="detail.priority_color"
              />
            </span>
          </div>
          <div
            v-if="fieldEnabled(detail.type, 'version')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'version')"
          >
            <span class="prop-label">{{ $t("workspace_version") }}</span>
            <a-select
              v-if="
                editingProperty === 'version' &&
                fieldEditable(detail.type, 'version')
              "
              :value="Number(detail.version_id) || undefined"
              allow-clear
              size="small"
              default-open
              autofocus
              style="width: 140px"
              @change="(value) => saveProperty('version_id', value || 0)"
              @dropdownVisibleChange="(open) => !open && (editingProperty = '')"
            >
              <a-select-option
                v-for="item in selectableVersions"
                :key="item.id"
                :value="item.id"
              >
                <IterationLabel
                  :name="item.name"
                  :cross-workspace="item.iteration_type === 'cross'"
                />
              </a-select-option>
            </a-select>
            <span
              v-else
              class="prop-value"
              :class="{ editable: fieldEditable(detail.type, 'version') }"
              @click="
                fieldEditable(detail.type, 'version') &&
                (editingProperty = 'version')
              "
            >
              <IterationLabel
                v-if="detail.version_name"
                :name="detail.version_name"
                :cross-workspace="selectedIteration?.iteration_type === 'cross'"
              />
              <template v-else>-</template>
            </span>
          </div>

          <div
            v-if="fieldEnabled(detail.type, 'assignee')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'assignee')"
          >
            <span class="prop-label">{{ $t("workspace_assignee") }}</span>
            <AssigneeSelect
              v-if="
                editingProperty === 'assignee' &&
                fieldEditable(detail.type, 'assignee')
              "
              :value="assigneeIdsValue"
              :members="members"
              style="width: 220px"
              @save="saveAssignees"
              @cancel="editingProperty = ''"
            />
            <span
              v-else
              class="prop-value"
              :class="{ editable: fieldEditable(detail.type, 'assignee') }"
              @click="
                fieldEditable(detail.type, 'assignee') &&
                (editingProperty = 'assignee')
              "
              ><AssigneeCell
                :ids="detail.assignee_ids"
                :names="detail.assignee_names"
                :avatars="detail.assignee_avatars"
                :usernames="detail.assignee_usernames"
              ></AssigneeCell
            ></span>
          </div>
          <div
            v-if="fieldEnabled(detail.type, 'planned_start_date')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'planned_start_date')"
          >
            <span class="prop-label">{{ $t("requirement_planStart") }}</span>
            <PlannedDatePicker
              v-if="
                editingProperty === 'planned_start_date' &&
                fieldEditable(detail.type, 'planned_start_date')
              "
              :value="plannedDateValue(detail.planned_start_date)"
              :default-picker-value="dayjs()"
              size="small"
              open
              @change="
                (value) =>
                  saveProperty(
                    'planned_start_date',
                    value ? value.startOf('day').unix() : 0,
                  )
              "
              @openChange="(o) => !o && (editingProperty = '')"
            />
            <span
              v-else
              class="prop-value"
              :class="{
                editable: fieldEditable(detail.type, 'planned_start_date'),
              }"
              @click="
                fieldEditable(detail.type, 'planned_start_date') &&
                (editingProperty = 'planned_start_date')
              "
              >{{ formatLocalDate(detail.planned_start_date) }}</span
            >
          </div>
          <div
            v-if="fieldEnabled(detail.type, 'planned_end_date')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'planned_end_date')"
          >
            <span class="prop-label">{{ $t("requirement_planEnd") }}</span>
            <PlannedDatePicker
              v-if="
                editingProperty === 'planned_end_date' &&
                fieldEditable(detail.type, 'planned_end_date')
              "
              :value="plannedDateValue(detail.planned_end_date)"
              :default-picker-value="dayjs()"
              size="small"
              open
              @change="
                (value) =>
                  saveProperty(
                    'planned_end_date',
                    value ? value.startOf('day').unix() : 0,
                  )
              "
              @openChange="(o) => !o && (editingProperty = '')"
            />
            <span
              v-else
              class="prop-value"
              :class="{
                editable: fieldEditable(detail.type, 'planned_end_date'),
              }"
              @click="
                fieldEditable(detail.type, 'planned_end_date') &&
                (editingProperty = 'planned_end_date')
              "
              >{{ formatLocalDate(detail.planned_end_date) }}</span
            >
          </div>
          <div
            v-for="field in (customFields[detail.type] || []).filter((item) =>
              fieldEnabled(detail.type, `custom:${item.id}`),
            )"
            :key="field.id"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, `custom:${field.id}`)"
          >
            <span class="prop-label" :title="field.name">{{ field.name }}</span>
            <CustomFieldControl
              v-if="canEdit && editingProperty === `custom_${field.id}`"
              v-model="customValues[field.id]"
              :field="field"
              :members="customFieldMembers"
              force-open
              @blur="saveAndCloseCustomField(field)"
              @change="saveChangedCustomField(field)"
              @openChange="(open) => !open && closeCustomFieldEditor(field)"
              @pressEnter="saveAndCloseCustomField(field)"
            />
            <span
              v-else
              class="prop-value"
              :class="{ editable: canEdit }"
              @click="canEdit && (editingProperty = `custom_${field.id}`)"
              >{{ customDisplay(field) }}</span
            >
          </div>
          <div
            v-if="fieldEnabled(detail.type, 'created_at')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'created_at')"
          >
            <span class="prop-label">{{ $t("workspace_createdTime") }}</span
            ><span class="prop-value">{{
              formatLocalDate(detail.created_at, true)
            }}</span>
          </div>
          <div
            v-if="fieldEnabled(detail.type, 'created_by')"
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'created_by')"
          >
            <span class="prop-label">{{ $t("workspace_createdBy") }}</span>
            <span class="prop-value">{{ detail.creator_name || "-" }}</span>
          </div>
          <div
            v-if="
              detail.type === 'requirement' &&
              fieldEnabled(detail.type, 'estimated_work_hours')
            "
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'estimated_work_hours')"
          >
            <span class="prop-label">{{
              $t("requirement_estimatedWorkHours")
            }}</span>
            <a-input-number
              v-if="editingProperty === 'estimated_work_hours'"
              v-model:value="estimatedWorkHoursDraft"
              :min="0"
              :max="99999999.99"
              :precision="2"
              size="small"
              autofocus
              @press-enter="saveEstimatedWorkHours"
              @blur="saveEstimatedWorkHours"
            >
              <template #addonAfter>{{
                $t("requirement_workHoursSuffix")
              }}</template>
            </a-input-number>
            <span
              v-else
              class="prop-value"
              :class="{
                editable: fieldEditable(detail.type, 'estimated_work_hours'),
              }"
              @click="startEstimatedWorkHoursEdit"
              >{{ formatWorkHours(detail.estimated_work_hours) }}</span
            >
          </div>
          <div
            v-if="
              detail.type === 'requirement' &&
              fieldEnabled(detail.type, 'work_progress')
            "
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'work_progress')"
          >
            <span class="prop-label">{{ $t("requirement_workProgress") }}</span>
            <span class="prop-value progress-property-value">
              <a-progress
                :percent="Number(detail.work_progress) || 0"
                :show-info="false"
                size="small"
              />
              <span class="progress-property-percent"
                >{{ Number(detail.work_progress) || 0 }}%</span
              >
            </span>
          </div>
          <div
            v-if="
              detail.type === 'requirement' &&
              fieldEnabled(detail.type, 'completed_work_hours')
            "
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'completed_work_hours')"
          >
            <span class="prop-label">{{
              $t("requirement_completedWorkHours")
            }}</span>
            <span
              class="prop-value"
              :class="{
                editable: fieldEditable(detail.type, 'completed_work_hours'),
              }"
              @click="openWorkLogCreate"
              >{{ formatWorkHoursValue(detail.completed_work_hours) }}</span
            >
          </div>
          <div
            v-if="
              detail.type === 'requirement' &&
              fieldEnabled(detail.type, 'remaining_work_hours')
            "
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'remaining_work_hours')"
          >
            <span class="prop-label">{{
              $t("requirement_remainingWorkHours")
            }}</span>
            <span class="prop-value">{{
              formatWorkHoursValue(detail.remaining_work_hours)
            }}</span>
          </div>
          <div
            v-if="
              detail.type === 'requirement' &&
              fieldEnabled(detail.type, 'overtime_work_hours')
            "
            class="prop-item"
            :style="fieldOrderStyle(detail.type, 'overtime_work_hours')"
          >
            <span class="prop-label">{{
              $t("requirement_overtimeWorkHours")
            }}</span>
            <span class="prop-value">{{
              formatWorkHoursValue(detail.overtime_work_hours)
            }}</span>
          </div>
        </div>
      </template>
    </aside>
  </div>
  <WorkItemActionModal
    :open="actionModalOpen"
    :action="actionModalType"
    :ws-id="wsId"
    :work-item-id="requirementId"
    :source-type="detail?.type || 'requirement'"
    :target-type="actionTargetType"
    @close="closeActionModal"
    @success="handleActionSuccess"
  />
  <LocalDraftModal
    :open="draftModalOpen"
    :records="draftRecords"
    @close="draftModalOpen = false"
    @restore="applyDraftRestore"
  />
  <StatusTransitionModal
    v-model:open="transitionOpen"
    :ws-id="wsId"
    :work-item="detail"
    :metadata="workspaceMetadata"
    @success="handleStatusTransitionSuccess"
  />
</template>
<script setup>
import { computed, ref, watch } from "vue";
import dayjs from "dayjs";
import {
  DownOutlined,
  ExportOutlined,
  PlusOutlined,
} from "@ant-design/icons-vue";
import IssueProcessingRecords from "./IssueProcessingRecords.vue";
import RequirementComments from "@/components/common/RequirementComments.vue";
import RequirementWorkLogs from "@/components/requirement/RequirementWorkLogs.vue";
import WorkItemSLA from "@/components/work-item/WorkItemSLA.vue";
import RequirementAging from "@/components/requirement/RequirementAging.vue";
import RequirementAgentTasks from "@/components/requirement/RequirementAgentTasks.vue";
import RichTextEditor from "@/components/common/RichTextEditor.vue";
import RichTextPreviewActions from "@/components/common/rich-text-editor/RichTextPreviewActions.vue";
import LocalDraftTips from "@/components/common/LocalDraftTips.vue";
import LocalDraftModal from "@/components/common/LocalDraftModal.vue";
import WorkItemActionModal from "@/components/common/WorkItemActionModal.vue";
import RequirementRelationPicker from "@/components/common/RequirementRelationPicker.vue";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import CustomFieldControl from "@/components/common/CustomFieldControl.vue";
import IterationLabel from "@/components/iteration/IterationLabel.vue";
import { formatLocalDate } from "@/utils/date";
import StatusTransitionModal from "@/components/common/StatusTransitionModal.vue";
const props = defineProps({ model: { type: Object, required: true } });
const {
  t,
  isIssueDetail,
  processingHistories,
  defectAppEnabled,
  draft,
  draftModalOpen,
  draftRecords,
  draftRestoreRevision,
  simpleImage,
  drawerMinWidth,
  drawerMaxWidth,
  viewportWidth,
  drawerWidth,
  isNarrowLayout,
  loading,
  detailLeftRef,
  startDrawerResize,
  resizeDrawerByKeyboard,
  detail,
  detailTab,
  detailReloadKey,
  propertiesCollapsed,
  editingParent,
  editingProperty,
  workLogsRef,
  estimatedWorkHoursDraft,
  descriptionEditing,
  descriptionEditorRef,
  descriptionDraft,
  imageFileIds,
  descriptionModeDraft,
  histories,
  historyLoading,
  historyLoaded,
  childRequirements,
  relatedDefects,
  attachments,
  members,
  customFieldMembers,
  priorities,
  customFields,
  workspaceMetadata,
  transitionOpen,
  statusTransitionOn,
  customValues,
  commentInputStyle,
  descriptionEditingStyle,
  canEdit,
  selectedIteration,
  selectableVersions,
  actionModalOpen,
  actionModalType,
  actionTargetType,
  hasParent,
  statusOptions,
  displayName,
  canCreateType,
  fieldEnabled,
  fieldOrderStyle,
  fieldEditable,
  canAssignCategory,
  categoryOptions,
  categorySaving,
  requirementCategoryName,
  saveCategory,
  customDisplay,
  assigneeIdsValue,
  historyActionText,
  isDescriptionHistory,
  openHistoryDetail,
  historyFieldText,
  historyValueText,
  historyStatusColor,
  reloadAttachments,
  goCreate,
  openParentInNewWindow,
  closeActionModal,
  handleActionSuccess,
  isFinalStatus,
  handleDetailTabChange,
  plannedDateValue,
  openStatusEditor,
  handleStatusTransitionSuccess,
  saveProperty,
  formatWorkHours,
  formatWorkHoursValue,
  startEstimatedWorkHoursEdit,
  saveEstimatedWorkHours,
  openWorkLogCreate,
  handleWorkLogSummary,
  saveAssignees,
  updateParent,
  openDraftModal,
  applyDraftRestore,
  handleDescriptionCancel,
  saveDescription,
  saveChangedCustomField,
  saveAndCloseCustomField,
  closeCustomFieldEditor,
  wsId,
  requirementId,
  open,
  mode,
} = props.model;
const sidebarTab = ref("processing");
const primaryTabs = computed(() => [
  { key: "desc", label: t("requirement_description") },
  {
    key: "history",
    label: `${t("requirement_changeHistory")} (${historyLoaded.value ? histories.value.length : detail.value?.history_count || 0})`,
  },
  ...(detail.value?.type === "requirement"
    ? [{ key: "aging", label: t("aging_column") }]
    : []),
]);
const moreTabs = computed(() => [
  ...(detail.value?.type === "requirement"
    ? [
        {
          key: "children",
          label: `${t("requirement_children")} (${childRequirements.value.length})`,
        },
        ...(defectAppEnabled.value
          ? [
              {
                key: "defects",
                label: `${t("requirement_relatedDefects")} (${relatedDefects.value.length})`,
              },
            ]
          : []),
        ...(fieldEnabled(detail.value.type, "completed_work_hours")
          ? [
              {
                key: "expense",
                label: `${t("requirement_expense")} (${detail.value.work_log_count || 0})`,
              },
            ]
          : []),
      ]
    : []),
  { key: "agent", label: t("agent_tab_title") },
]);
function selectMoreTab(key) {
  detailTab.value = key;
  handleDetailTabChange(key);
}
watch(
  [
    () => open.value,
    () => loading.value,
    () => historyLoaded.value,
    sidebarTab,
    detailTab,
  ],
  () => {
    if (
      isIssueDetail &&
      open.value &&
      !loading.value &&
      !historyLoaded.value &&
      (sidebarTab.value === "processing" || detailTab.value === "history")
    ) {
      // The API interceptor reports failures; keep the panel available for retry.
      handleDetailTabChange("processing").catch(() => {});
    }
  },
  { immediate: true },
);
</script>
<style scoped lang="less">
@import "./workItemDetail.less";
</style>
