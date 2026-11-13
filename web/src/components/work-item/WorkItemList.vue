<template>
  <RequirementCategorySidebar
    v-if="activeTab === 'requirement'"
    ref="categorySidebarRef"
    :ws-id="wsId"
    :selected-key="selectedCategoryKey"
    :collapsed="categorySidebarCollapsed"
    :initial-width="categorySidebarWidth"
    :can-manage="canManageRequirementCategories"
    :can-create="canCreateType('requirement')"
    @resize="setCategorySidebarWidth"
    @resize-end="persistCategorySidebarWidth"
    @select="selectRequirementCategory"
    @update:collapsed="setCategorySidebarCollapsed"
    @categories-loaded="requirementCategories = $event"
    @create-work-item="goCreateInCategory"
    @changed="handleRequirementCategoryChanged"
  >
    <template #views>
      <div class="sidebar-view-list">
        <template v-for="group in sidebarViewGroups" :key="group.type">
          <div v-if="group.views.length" class="sidebar-view-group">
            <button
              class="sidebar-view-heading"
              type="button"
              :aria-expanded="!collapsedViewGroups[group.type]"
              @click="
                collapsedViewGroups[group.type] =
                  !collapsedViewGroups[group.type]
              "
            >
              <CaretRightOutlined
                v-if="collapsedViewGroups[group.type]"
                aria-hidden="true"
              />
              <CaretDownOutlined v-else aria-hidden="true" />
              <span>{{ $t(group.label) }}</span>
              <span>({{ group.views.length }})</span>
            </button>
            <template v-if="!collapsedViewGroups[group.type]">
              <button
                v-for="view in group.views"
                :key="view.id"
                type="button"
                :aria-pressed="String(currentViewId) === String(view.id)"
                @click="handleViewSelect(view.id)"
              >
                {{ view.name }}
              </button>
            </template>
          </div>
        </template>
      </div>
      <div class="sidebar-view-actions">
        <a-button type="text" @click="openCreateView">
          <PlusOutlined /> {{ $t("view_create") }}
        </a-button>
        <a-button type="text" @click="viewConfigOpen = true">
          <EditOutlined /> {{ $t("view_config_title") }}
        </a-button>
      </div>
    </template>
  </RequirementCategorySidebar>
  <div
    class="board-main"
    :style="
      activeTab === 'requirement' && !categorySidebarCollapsed
        ? { marginLeft: `${categorySidebarWidth}px` }
        : undefined
    "
    :class="{
      'with-category-sidebar':
        activeTab === 'requirement' && !categorySidebarCollapsed,
    }"
  >
    <!-- Toolbar -->
    <div
      class="board-toolbar"
      :class="{
        'board-toolbar--search-open': currentSearchExpanded,
      }"
    >
      <!-- Batch operations (merged into one dropdown button when selected) -->
      <a-dropdown v-if="selectedRowKeys.length && canUseBatchActions">
        <a-button type="primary"
          ><EditOutlined />
          {{ $t("workspace_batchAction", { n: selectedRowKeys.length }) }}
          <DownOutlined
        /></a-button>
        <template #overlay>
          <a-menu @click="({ key }) => handleBatchMenu(key)">
            <a-menu-item
              v-if="canExportActiveWorkItem"
              key="export"
              class="toolbar-menu-item"
              ><ExportOutlined />{{ $t("requirement_exportCSV") }}</a-menu-item
            >
            <a-menu-divider
              v-if="canExportActiveWorkItem && canEditActiveWorkItem"
            />
            <a-sub-menu
              v-if="canEditActiveWorkItem"
              key="status"
              :title="$t('requirement_batchStatus')"
            >
              <a-menu-item
                v-for="s in statusOptions"
                :key="`status_id:${s.id}`"
                >{{ s.name }}</a-menu-item
              >
            </a-sub-menu>
            <a-sub-menu
              v-if="canEditActiveWorkItem"
              key="priority"
              :title="$t('requirement_batchPriority')"
            >
              <a-menu-item key="priority:0">{{
                $t("requirement_clearPriority")
              }}</a-menu-item>
              <a-menu-divider />
              <a-menu-item
                v-for="p in priorityOptions"
                :key="`priority:${p.id}`"
                >{{ p.label }}</a-menu-item
              >
            </a-sub-menu>
            <a-menu-item v-if="canEditActiveWorkItem" key="assignee_ids">
              {{ $t("requirement_batchAssignee") }}
            </a-menu-item>
            <a-menu-divider
              v-if="
                (canExportActiveWorkItem || canEditActiveWorkItem) &&
                (canDeleteActiveWorkItem || canCopyActiveWorkItem)
              "
            />
            <a-menu-item v-if="canDeleteActiveWorkItem" key="delete" danger>
              <DeleteOutlined /> {{ $t("common_delete") }}
            </a-menu-item>
            <a-menu-item v-if="canCopyActiveWorkItem" key="copyTitleLink">
              <ShareAltOutlined />
              {{ $t("requirement_copyTitleLink") }}
            </a-menu-item>
            <a-menu-item v-if="canCopyActiveWorkItem" key="copyTitle">
              <FontSizeOutlined /> {{ $t("requirement_copyTitle") }}
            </a-menu-item>
            <a-menu-item v-if="canCopyActiveWorkItem" key="copyLink">
              <LinkOutlined /> {{ $t("requirement_copyLink") }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>

      <a-button
        v-if="canCreateActiveWorkItem"
        type="primary"
        @click="goCreateActiveWorkItem"
      >
        <PlusOutlined /> {{ $t("common_create") }}
      </a-button>

      <a-dropdown
        v-if="canImportActiveWorkItem || canExportActiveWorkItem"
        trigger="click"
      >
        <a-button><MoreOutlined /> {{ $t("common_more") }}</a-button>
        <template #overlay>
          <a-menu @click="({ key }) => handleMoreMenu(key)">
            <a-menu-item
              v-if="canImportActiveWorkItem"
              key="import"
              class="toolbar-menu-item"
              ><ImportOutlined />{{ $t("requirement_import") }}</a-menu-item
            >
            <a-menu-item
              v-if="canExportActiveWorkItem"
              key="export"
              class="toolbar-menu-item"
              ><ExportOutlined />{{ $t("requirement_exportAll") }}</a-menu-item
            >
          </a-menu>
        </template>
      </a-dropdown>

      <WorkItemSearchBar
        v-if="currentSearchExpanded"
        :filters="currentSearchFilters"
        :type-key="activeTab"
        :fields="searchFieldDescriptors"
        :statuses="statusOptions"
        :priorities="priorityOptions"
        :members="members"
        :custom-field-members="customFieldMembers"
        :versions="versions"
        :categories="requirementCategories"
        class="toolbar-search-bar"
        @change="handleSearchFiltersChange"
        @search="handleSearch"
        @save-as="openSaveCurrentSearch"
      />
    </div>

    <!-- Requirement table -->
    <a-table
      v-if="listPreferences.viewMode === 'list'"
      class="requirement-table"
      :columns="tableCols"
      :data-source="treeList"
      :loading="loading"
      :row-selection="rowSelection"
      row-key="id"
      :pagination="false"
      size="small"
      table-layout="fixed"
      :scroll="{ x: tableScrollWidth, y: 'calc(100vh - 220px)' }"
      :row-class-name="getRowClassName"
      :locale="{
        emptyText:
          activeTab === 'defect'
            ? $t('requirement_noDefects')
            : $t('requirement_noRequirements'),
      }"
      @resizeColumn="handleResizeColumn"
    >
      <template #headerCell="{ title, column }">
        <div
          v-if="column.preferenceKey"
          class="work-item-column-header"
          :class="{
            'is-menu-open': openColumnMenuKey === column.preferenceKey,
            'is-dragging': columnDragState.sourceKey === column.preferenceKey,
            'is-drop-before':
              columnDragState.targetKey === column.preferenceKey &&
              columnDragState.side === 'before',
            'is-drop-after':
              columnDragState.targetKey === column.preferenceKey &&
              columnDragState.side === 'after',
            'is-sorted': currentListSort.field === column.sortField,
          }"
          draggable="true"
          @dragstart="startColumnDrag($event, column)"
          @dragover.prevent="updateColumnDropTarget($event, column)"
          @drop.prevent="finishColumnDrop($event, column)"
          @dragend="clearColumnDrag"
        >
          <span class="work-item-column-main">
            <span v-if="column.key === 'title'" class="title-header-content">
              <button
                v-if="activeTab === 'requirement'"
                type="button"
                class="tree-header-toggle"
                :title="
                  allTreeExpanded ? $t('common_collapse') : $t('common_expand')
                "
                :aria-label="
                  allTreeExpanded ? $t('common_collapse') : $t('common_expand')
                "
                draggable="false"
                @mousedown.stop
                @click.stop="toggleAllTreeRows"
              >
                <CaretDownOutlined v-if="allTreeExpanded" />
                <CaretRightOutlined v-else />
              </button>
              <span class="work-item-column-label">{{ title }}</span>
              <span class="title-header-count">({{ total }})</span>
            </span>
            <span v-else class="work-item-column-label">{{ title }}</span>
            <a-tooltip
              v-if="column.sortField"
              :title="$t('workspace_cycleColumnSort')"
            >
              <button
                type="button"
                class="column-header-action column-sort-action"
                :class="{
                  active: currentListSort.field === column.sortField,
                }"
                :aria-label="$t('workspace_cycleColumnSort')"
                draggable="false"
                @mousedown.stop
                @click.stop="cycleColumnSort(column)"
              >
                <img
                  class="column-sort-icon"
                  :src="columnSortIcon(column)"
                  alt=""
                  aria-hidden="true"
                />
              </button>
            </a-tooltip>
          </span>
          <span class="work-item-column-actions">
            <a-dropdown
              v-if="
                canFreezeColumn(column) ||
                hasFrozenColumns ||
                canHideColumn(column)
              "
              trigger="click"
              placement="bottomRight"
              @openChange="(open) => handleColumnMenuOpenChange(open, column)"
            >
              <button
                type="button"
                class="column-header-action column-more-action"
                :aria-label="$t('common_more')"
                draggable="false"
                @mousedown.stop
                @click.stop
              >
                <MoreOutlined />
              </button>
              <template #overlay>
                <a-menu @click="({ key }) => handleColumnMenu(key, column)">
                  <a-menu-item v-if="canFreezeColumn(column)" key="freeze">
                    <LockOutlined />
                    {{ $t("workspace_freezeToColumn") }}
                  </a-menu-item>
                  <a-menu-item v-if="hasFrozenColumns" key="unfreeze">
                    <UnlockOutlined />
                    {{ $t("workspace_cancelColumnFreeze") }}
                  </a-menu-item>
                  <a-menu-divider
                    v-if="
                      (canFreezeColumn(column) || hasFrozenColumns) &&
                      canHideColumn(column)
                    "
                  />
                  <a-menu-item v-if="canHideColumn(column)" key="hide">
                    <EyeInvisibleOutlined />
                    {{ $t("workspace_hideColumn") }}
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </span>
        </div>
        <template v-else>{{ title }}</template>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'type'">
          <WorkItemTypePill :type="record.type" :ws-id="wsId" />
        </template>
        <template v-if="column.key === 'title'">
          <button
            v-if="record.__loadMore"
            type="button"
            class="load-more-button"
            :disabled="isChildLoading(record.__parentId)"
            @click.stop="loadMoreChildren(record.__parentId)"
          >
            <LoadingOutlined v-if="isChildLoading(record.__parentId)" />
            {{ $t("common_loadMore") }}
          </button>
          <div v-else class="title-cell">
            <span
              class="tree-indent"
              :style="{ width: `${14 + (record.__depth || 0) * 16}px` }"
            >
              <button
                v-if="hasChildren(record)"
                type="button"
                class="tree-toggle"
                :title="
                  expandedRowKeys.includes(record.id)
                    ? $t('common_collapse')
                    : $t('common_expand')
                "
                @click.stop="toggleExpand(record)"
              >
                <LoadingOutlined v-if="isChildLoading(record.id)" />
                <DownOutlined v-else-if="expandedRowKeys.includes(record.id)" />
                <RightOutlined v-else />
              </button>
            </span>
            <WorkItemTypePill
              :type="record.type"
              :ws-id="wsId"
              class="title-type-pill"
            />
            <input
              v-if="isEditing(record, 'title')"
              :ref="(element) => setTitleInputRef(record.id, element)"
              v-model="titleDraft"
              class="inline-title-input"
              @keydown.enter.prevent="$event.currentTarget.blur()"
              @blur="saveInlineTitle(record)"
              @click.stop
            />
            <span
              v-else
              class="title-text"
              :title="record.title"
              @click="openDetail(record.id)"
            >
              {{ record.title }}
            </span>
            <a-tooltip
              v-if="
                fieldEditable(record.type, 'title') &&
                !isEditing(record, 'title')
              "
              :title="$t('common_edit')"
            >
              <EditOutlined
                class="title-edit-icon"
                @click.stop="startEditTitle(record)"
              />
            </a-tooltip>
          </div>
        </template>
        <template v-if="column.key === 'status'">
          <a-select
            v-if="isEditing(record, 'status_id')"
            size="small"
            :value="Number(record.status_id)"
            style="width: 110px"
            default-open
            @change="(v) => saveInline(record, 'status_id', v)"
            @blur="clearEditing"
            @keydown.esc.stop.prevent="cancelEditing"
            @click.stop
          >
            <a-select-option
              v-for="s in statusesForRecord(record)"
              :key="s.id"
              :value="s.id"
              >{{ s.name }}</a-select-option
            >
          </a-select>
          <span v-else
            ><StatusPill
              :text="record.status_name"
              :color="record.status_color"
          /></span>
        </template>
        <template v-if="column.key === 'template'">
          {{ record.template_name || "—" }}
        </template>
        <template v-if="column.key === 'priority'">
          <a-select
            v-if="isEditing(record, 'priority')"
            size="small"
            :value="Number(record.priority) || undefined"
            style="width: 90px"
            allowClear
            default-open
            @change="(v) => saveInline(record, 'priority', v ?? null)"
            @blur="clearEditing"
            @keydown.esc.stop.prevent="cancelEditing"
            @click.stop
          >
            <a-select-option
              v-for="p in priorityOptionsForType(record.type)"
              :key="p.id"
              :value="p.id"
              >{{ p.label }}</a-select-option
            >
          </a-select>
          <span v-else
            ><PriorityPill
              :priority="record.priority"
              :label="record.priority_name"
              :color="record.priority_color"
          /></span>
        </template>
        <template v-if="column.key === 'assignee'">
          <AssigneeSelect
            v-if="isEditing(record, 'assignee_ids')"
            :value="record.assignee_ids"
            :members="members"
            style="width: 180px"
            @save="(value) => saveAssigneesInline(record, value)"
            @cancel="cancelEditing"
            @click.stop
          />
          <span v-else
            ><AssigneeCell
              :ids="record.assignee_ids"
              :names="record.assignee_names"
              :avatars="record.assignee_avatars"
              :usernames="record.assignee_usernames"
          /></span>
        </template>
        <template v-if="column.key === 'version'">
          <a-select
            v-if="isEditing(record, 'version_id')"
            size="small"
            :value="Number(record.version_id) || undefined"
            allow-clear
            style="width: 110px"
            default-open
            @change="(v) => saveInline(record, 'version_id', v || 0)"
            @blur="clearEditing"
            @keydown.esc.stop.prevent="cancelEditing"
            @click.stop
          >
            <a-select-option
              v-for="v in versionsForRecord(record)"
              :key="v.id"
              :value="v.id"
            >
              <IterationLabel
                :name="v.name"
                :cross-workspace="v.iteration_type === 'cross'"
              />
            </a-select-option>
          </a-select>
          <IterationLabel
            v-else-if="record.version_name"
            :name="record.version_name"
            :cross-workspace="isCrossVersion(record.version_id)"
          />
          <span v-else>-</span>
        </template>
        <template v-if="column.key === 'category'">
          <button
            v-if="canAssignCategory(record)"
            type="button"
            class="category-cell"
            :title="requirementCategoryName(record.category_id)"
            @click.stop="openCategoryEditor(record)"
          >
            {{ requirementCategoryName(record.category_id) }}
          </button>
          <span v-else>{{ requirementCategoryName(record.category_id) }}</span>
        </template>
        <template v-if="column.key === 'parent'">
          <a
            v-if="Number(record.parent_id) > 0"
            @click.stop="openDetail(record.parent_id)"
            >{{ record.parent_title || `#${record.parent_id}` }}</a
          >
          <span v-else>-</span>
        </template>
        <template v-if="column.key === 'dueDate'">
          <PlannedDatePicker
            v-if="isEditing(record, 'planned_start_date')"
            size="small"
            :value="dateValue(record.planned_start_date)"
            :default-picker-value="dayjs()"
            style="width: 105px"
            open
            @change="
              (_, dateString) =>
                saveInline(
                  record,
                  'planned_start_date',
                  toPlannedDateTimestamp(dateString),
                )
            "
            @openChange="(o) => !o && clearEditing()"
            @keydown.esc.stop.prevent="cancelEditing"
            @click.stop
          />
          <span v-else>{{ fmtDate(record.planned_start_date) }}</span>
        </template>
        <template v-if="column.key === 'endDate'">
          <PlannedDatePicker
            v-if="isEditing(record, 'planned_end_date')"
            size="small"
            :value="dateValue(record.planned_end_date)"
            :default-picker-value="dayjs()"
            style="width: 105px"
            open
            @change="
              (_, dateString) =>
                saveInline(
                  record,
                  'planned_end_date',
                  toPlannedDateTimestamp(dateString),
                )
            "
            @openChange="(o) => !o && clearEditing()"
            @keydown.esc.stop.prevent="cancelEditing"
            @click.stop
          />
          <span v-else>{{ fmtDate(record.planned_end_date) }}</span>
        </template>
        <template v-if="column.key === 'estimatedHours'">
          <a-input-number
            v-if="isEditing(record, 'estimated_work_hours')"
            v-model:value="editingCell.draft"
            size="small"
            :min="0"
            :max="99999999.99"
            :precision="2"
            autofocus
            @keydown.enter.prevent="
              saveInline(
                record,
                'estimated_work_hours',
                Number(editingCell.draft) || 0,
              )
            "
            @blur="
              saveInline(
                record,
                'estimated_work_hours',
                Number(editingCell.draft) || 0,
              )
            "
            @keydown.esc.stop.prevent="cancelEditing"
            @click.stop
          >
            <template #addonAfter>{{
              $t("requirement_workHoursSuffix")
            }}</template>
          </a-input-number>
          <span v-else>{{ formatWorkHours(record.estimated_work_hours) }}</span>
        </template>
        <template v-if="column.key === 'workProgress'">
          {{ Number(record.work_progress) || 0 }}%
        </template>
        <template v-if="column.key === 'completedHours'">
          <button
            type="button"
            class="work-hours-link"
            @click.stop="openWorkLogEditor(record)"
          >
            {{ formatWorkHoursValue(record.completed_work_hours) }}
          </button>
        </template>
        <template v-if="column.key === 'remainingHours'">
          {{ formatWorkHoursValue(record.remaining_work_hours) }}
        </template>
        <template v-if="column.key === 'overtimeHours'">
          {{ formatWorkHoursValue(record.overtime_work_hours) }}
        </template>
        <template v-if="column.key === 'createdBy'">{{
          record.creator_name || "-"
        }}</template>
        <template v-if="column.key === 'createdAt'">{{
          formatLocalDate(record.created_at, true)
        }}</template>
        <template v-if="column.customField">
          <CustomFieldControl
            v-if="isEditing(record, column.key)"
            v-model="editingCell.draft"
            :field="column.customField"
            :members="customFieldMembers"
            force-open
            @pressEnter="saveInlineCustomField(record, column.customField)"
            @blur="saveInlineCustomField(record, column.customField)"
            @change="saveInlineCustomField(record, column.customField)"
            @openChange="
              (open) =>
                !open && saveInlineCustomField(record, column.customField)
            "
            @keydown.esc.stop.prevent="cancelEditing"
            @click.stop
          />
          <span v-else class="editable-cell-value">
            {{
              formatCustomFieldValue(
                column.customField,
                record.custom_fields,
                customFieldMembers,
              )
            }}
          </span>
        </template>
        <template v-if="column.key === 'sla'">
          <button class="sla-cell" @click.stop="openDetail(record.id, 'sla')">
            <template
              v-if="slaSummaries[record.id]?.configuration_incomplete"
              >{{ $t("sla_configurationIncomplete") }}</template
            >
            <template v-else-if="slaFailed">{{
              $t("sla_loadFailed")
            }}</template>
            <template v-else-if="visibleSLAStages[record.id]?.length"
              ><SLAStageResult
                v-for="stage in visibleSLAStages[record.id]"
                :key="stage.stage_id"
                :stage="stage"
                compact
            /></template>
            <span v-else class="sla-empty">—</span>
          </button>
        </template>
        <template v-if="column.key === 'aging'">
          <span
            class="aging-tag"
            :class="agingTagClass(record.aging_status)"
            @click.stop="openDetail(record.id, 'aging')"
            >{{ agingTagText(record.aging_status) }}</span
          >
        </template>
      </template>
    </a-table>
    <WorkItemKanban
      v-else-if="boardContextReady"
      ref="kanbanRef"
      :ws-id="wsId"
      :type-key="activeTab"
      :statuses="statusOptions"
      :status-preferences="listPreferences.boardStatuses"
      :board-sort="listPreferences.boardSort"
      :board-colors="listPreferences.boardColors"
      :priorities="priorityOptions"
      :members="members"
      :filters="kanbanFilters"
      :can-create="canCreateActiveWorkItem"
      :can-edit="canEditActiveWorkItem"
      :editable="kanbanEditableFields"
      :field-editable="kanbanFieldEditability"
      :selected-item-id="detailReqId"
      @open="openDetail"
      @create="createInStatus"
      @updated="handleKanbanUpdated"
      @status-visibility-change="handleKanbanStatusVisibility"
      @status-color-change="handleKanbanStatusColor"
      @navigation-change="kanbanNavigationItems = $event"
    />
    <div v-else class="board-loading">
      <a-spin />
    </div>
    <Teleport to="body">
      <div
        v-if="tableResizeGuide.visible"
        class="table-resize-guide"
        :style="{
          left: `${tableResizeGuide.left}px`,
          top: `${tableResizeGuide.top}px`,
          height: `${tableResizeGuide.height}px`,
          '--header-height': `${tableResizeGuide.headerHeight}px`,
        }"
      >
        <span class="table-resize-guide-header" />
        <span class="table-resize-guide-body" />
      </div>
    </Teleport>
    <a-pagination
      v-if="listPreferences.viewMode === 'list'"
      class="requirement-pagination"
      :current="page"
      :page-size="pageSize"
      :page-size-options="['20', '40', '80', '100']"
      :total="total"
      :show-size-changer="true"
      :show-total="(count) => `${$t('common_all')} ${count}`"
      @change="handleListPageChange"
    />
  </div>

  <BatchAssigneeModal
    v-model:open="batchAssigneeOpen"
    v-model:assignee-ids="batchAssigneeIds"
    :members="assignableMembers"
    :confirm-loading="batchAssigneeSaving"
    @ok="submitBatchAssignees"
    @cancel="batchAssigneeIds = []"
  />
  <DeleteConfirmModal
    :open="deleteConfirmOpen"
    :title="deleteConfirmTitle"
    :content="deleteConfirmContent"
    :ok-text="deleteConfirmOkText"
    :confirm-loading="deletingTarget"
    @update:open="handleDeleteConfirmOpen"
    @ok="confirmDeleteTarget"
  />
  <a-modal
    :open="!!categoryEditTarget"
    :title="$t('requirement_set_category')"
    :ok-text="$t('common_confirm')"
    :cancel-text="$t('common_cancel')"
    :confirm-loading="categorySaving"
    :closable="!categorySaving"
    :mask-closable="!categorySaving"
    :keyboard="!categorySaving"
    :cancel-button-props="{ disabled: categorySaving }"
    @cancel="closeCategoryEditor"
    @ok="confirmCategoryAssignment"
  >
    <a-form layout="vertical">
      <a-form-item :label="$t('workspace_field_category')">
        <a-select
          v-model:value="categoryDraft"
          :aria-label="$t('workspace_field_category')"
          :options="categoryOptions"
          :disabled="categorySaving"
        />
      </a-form-item>
    </a-form>
  </a-modal>
  <StatusTransitionModal
    v-model:open="transitionOpen"
    :ws-id="wsId"
    :work-item="transitionItem"
    :metadata="workspaceMetadata"
    @success="handleStatusTransitionSuccess"
  />
  <RequirementDetailDrawer
    :open="!!detailReqId"
    :ws-id="wsId"
    :requirement-id="detailReqId"
    :mode="listPreferences.previewMode"
    :work-log-editor-token="workLogEditorToken"
    :work-log-editor-requirement-id="workLogEditorRequirementId"
    :initial-tab="route.query.detailTab || ''"
    :navigation-items="boardNavigationItems"
    :can-edit-by-type="{
      requirement: authStore.hasPermission('requirement:edit'),
      defect: authStore.hasPermission('defect:edit'),
    }"
    :can-create-by-type="{
      requirement: authStore.hasPermission('requirement:create'),
      defect: authStore.hasPermission('defect:create'),
    }"
    @close="closeDetail"
    @navigate="handleDrawerNavigate"
    @updated="handleDetailUpdated"
  />
  <RequirementWorkLogs
    v-if="directWorkLogRequirementId"
    ref="directWorkLogsRef"
    :ws-id="wsId"
    :requirement-id="directWorkLogRequirementId"
    :can-edit="authStore.hasPermission('requirement:edit')"
    :show-list="false"
    @summary-change="handleDirectWorkLogSummaryChange"
  />
  <ListSettingsModal
    v-if="isWorkItemTab"
    v-model:open="listSettingsOpen"
    :user-id="authStore.userInfo?.id"
    :ws-id="wsId"
    :type-key="activeTab"
    :fields="systemListFields"
    :statuses="statusOptions"
    :current-preferences="listPreferences"
    @change="applyListPreferences"
  />
  <ImportWorkItemsModal
    v-model:open="showImportModal"
    :ws-id="wsId"
    :type-key="activeTab"
    @imported="handleWorkItemsImported"
  />
  <ViewEditModal
    v-if="isWorkItemTab"
    v-model:open="viewEditOpen"
    :ws-id="wsId"
    :type-key="activeTab"
    :mode="viewEditMode"
    :view="viewEditTarget"
    :can-manage-system-view="canManageSystemView"
    :status-options="statusOptions"
    :priority-options="priorityOptions"
    :members="members"
    :custom-field-members="customFieldMembers"
    :versions="versions"
    :categories="requirementCategories"
    :field-descriptors="searchFieldDescriptors"
    :final-status-ids="finalStatusIds[activeTab] || []"
    @saved="handleViewSaved"
  />
  <ViewConfigModal
    v-if="isWorkItemTab"
    v-model:open="viewConfigOpen"
    :ws-id="wsId"
    :type-key="activeTab"
    :views="currentTabViews"
    :remember-last="currentTabRememberLast"
    :lock-all-view="activeTab === 'requirement'"
    :can-manage-system-view="canManageSystemView"
    @edit="openEditView"
    @save-as="openSaveAsView"
    @deleted="handleViewDeleted"
    @saved="handleViewSettingsSaved"
  />
</template>
<script setup>
import { computed, ref } from "vue";
import dayjs from "dayjs";
import {
  CaretDownOutlined,
  CaretRightOutlined,
  PlusOutlined,
  MoreOutlined,
  DownOutlined,
  RightOutlined,
  EditOutlined,
  ExportOutlined,
  ImportOutlined,
  DeleteOutlined,
  FontSizeOutlined,
  LinkOutlined,
  ShareAltOutlined,
  EyeInvisibleOutlined,
  LoadingOutlined,
  LockOutlined,
  UnlockOutlined,
} from "@ant-design/icons-vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";
import RequirementWorkLogs from "@/components/requirement/RequirementWorkLogs.vue";
import StatusTransitionModal from "@/components/common/StatusTransitionModal.vue";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
import AssigneeCell from "@/components/common/AssigneeCell.vue";
import AssigneeSelect from "@/components/common/AssigneeSelect.vue";
import CustomFieldControl from "@/components/common/CustomFieldControl.vue";
import PlannedDatePicker from "@/components/common/PlannedDatePicker.vue";
import BatchAssigneeModal from "@/components/work-item/BatchAssigneeModal.vue";
import DeleteConfirmModal from "@/components/work-item/DeleteConfirmModal.vue";
import ImportWorkItemsModal from "@/components/work-item/ImportWorkItemsModal.vue";
import ListSettingsModal from "@/components/work-item/ListSettingsModal.vue";
import RequirementCategorySidebar from "@/components/work-item/RequirementCategorySidebar.vue";
import WorkItemKanban from "@/components/work-item/WorkItemKanban.vue";
import IterationLabel from "@/components/iteration/IterationLabel.vue";
import ViewEditModal from "@/components/work-item/ViewEditModal.vue";
import ViewConfigModal from "@/components/work-item/ViewConfigModal.vue";
import WorkItemSearchBar from "@/components/work-item/WorkItemSearchBar.vue";
import { formatLocalDate } from "@/utils/date";
import { formatCustomFieldValue } from "@/utils/customField";
import SLAStageResult from "./SLAStageResult.vue";
const props = defineProps({ model: { type: Object, required: true } });
const {
  t,
  route,
  authStore,
  wsId,
  selectedCategoryKey,
  categorySidebarCollapsed,
  categorySidebarWidth,
  requirementCategories,
  categorySidebarRef,
  requirementCategoryName,
  categoryEditTarget,
  categoryDraft,
  categorySaving,
  categoryOptions,
  canAssignCategory,
  openCategoryEditor,
  closeCategoryEditor,
  confirmCategoryAssignment,
  activeTab,
  loading,
  boardContextReady,
  page,
  pageSize,
  total,
  selectedRowKeys,
  editingCell,
  expandedRowKeys,
  allTreeExpanded,
  detailReqId,
  workLogEditorToken,
  workLogEditorRequirementId,
  directWorkLogRequirementId,
  directWorkLogsRef,
  showImportModal,
  batchAssigneeOpen,
  batchAssigneeIds,
  batchAssigneeSaving,
  deleteConfirmOpen,
  deletingTarget,
  listSettingsOpen,
  listPreferences,
  columnDragState,
  openColumnMenuKey,
  currentListSort,
  kanbanRef,
  kanbanNavigationItems,
  viewEditOpen,
  viewEditMode,
  viewEditTarget,
  viewConfigOpen,
  deleteConfirmTitle,
  deleteConfirmContent,
  deleteConfirmOkText,
  tableResizeGuide,
  members,
  customFieldMembers,
  assignableMembers,
  versions,
  finalStatusIds,
  workspaceMetadata,
  transitionOpen,
  transitionItem,
  canManageRequirementCategories,
  isWorkItemTab,
  canCreateActiveWorkItem,
  canEditActiveWorkItem,
  canDeleteActiveWorkItem,
  canCopyActiveWorkItem,
  canImportActiveWorkItem,
  canExportActiveWorkItem,
  canUseBatchActions,
  statusOptions,
  priorityOptions,
  currentTabViews,
  currentViewId,
  handleViewSelect,
  openCreateView,
  currentTabRememberLast,
  currentSearchExpanded,
  currentSearchFilters,
  searchFieldDescriptors,
  canManageSystemView,
  kanbanFilters,
  kanbanEditableFields,
  kanbanFieldEditability,
  priorityOptionsForType,
  statusesForRecord,
  fieldEditable,
  canCreateType,
  selectRequirementCategory,
  setCategorySidebarCollapsed,
  setCategorySidebarWidth,
  persistCategorySidebarWidth,
  goCreateActiveWorkItem,
  goCreateInCategory,
  handleRequirementCategoryChanged,
  handleWorkItemsImported,
  rowSelection,
  systemListFields,
  hasFrozenColumns,
  startColumnDrag,
  updateColumnDropTarget,
  clearColumnDrag,
  finishColumnDrop,
  cycleColumnSort,
  columnSortIcon,
  handleColumnMenu,
  canFreezeColumn,
  canHideColumn,
  handleColumnMenuOpenChange,
  applyListPreferences,
  handleKanbanStatusVisibility,
  handleKanbanStatusColor,
  versionsForRecord,
  isCrossVersion,
  treeList,
  boardNavigationItems,
  tableCols,
  tableScrollWidth,
  handleResizeColumn,
  fmtDate,
  handleSearchFiltersChange,
  openEditView,
  openSaveAsView,
  openSaveCurrentSearch,
  handleViewSaved,
  handleViewDeleted,
  handleViewSettingsSaved,
  handleSearch,
  handleListPageChange,
  createInStatus,
  handleKanbanUpdated,
  isEditing,
  handleStatusTransitionSuccess,
  clearEditing,
  cancelEditing,
  getRowClassName,
  hasChildren,
  isChildLoading,
  toggleExpand,
  toggleAllTreeRows,
  loadMoreChildren,
  titleDraft,
  setTitleInputRef,
  startEditTitle,
  saveInlineTitle,
  saveInline,
  saveAssigneesInline,
  saveInlineCustomField,
  handleDeleteConfirmOpen,
  confirmDeleteTarget,
  handleMoreMenu,
  handleBatchMenu,
  submitBatchAssignees,
  dateValue,
  toPlannedDateTimestamp,
  handleDrawerNavigate,
  slaSummaries,
  slaFailed,
  agingTagClass,
  agingTagText,
  openDetail,
  openWorkLogEditor,
  handleDirectWorkLogSummaryChange,
  formatWorkHours,
  formatWorkHoursValue,
  closeDetail,
  handleDetailUpdated,
} = props.model;
const visibleSLAStages = computed(() =>
  Object.fromEntries(
    Object.entries(slaSummaries.value).map(([id, result]) => [
      id,
      (result.current || []).filter((stage) => stage.state !== "not_started"),
    ]),
  ),
);
const collapsedViewGroups = ref({});
const sidebarViewGroups = computed(() =>
  [
    { type: "personal", label: "view_personal_group" },
    { type: "system", label: "view_system_group" },
  ].map((group) => ({
    ...group,
    views: currentTabViews.value.filter(
      (view) => view.enabled && view.view_type === group.type,
    ),
  })),
);
</script>
<style scoped lang="less">
.category-cell {
  max-width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}
.sla-cell {
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  max-width: 100%;
  min-height: 24px;
  text-align: left;
  font: inherit;
  padding: 4px 0;
}
.sla-cell:focus-visible,
.category-cell:focus-visible {
  outline: 2px solid var(--app-primary, #3370ff);
  outline-offset: 2px;
  border-radius: 4px;
}
.sla-empty {
  color: var(--app-text-secondary, #8590a0);
}
@import "./workItemBoard.less";
.sidebar-view-list {
  flex: 1;
  overflow: auto;
  padding: 8px;
}
.sidebar-view-group {
  display: flex;
  flex-direction: column;
  gap: 2px;

  button {
    padding: 8px;
    border: 0;
    border-radius: 6px;
    background: none;
    color: inherit;
    cursor: pointer;
    text-align: left;
  }

  button:hover,
  button[aria-pressed="true"] {
    background: var(--ant-color-primary-bg, #e6f4ff);
    color: var(--ant-color-primary, #1677ff);
  }

  .sidebar-view-heading {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 0;
    font-size: 12px;
    opacity: 0.75;
  }
}
.sidebar-view-actions {
  display: flex;
  align-items: flex-start;
  padding-top: 8px;
  border-top: 1px solid var(--app-border);
}
</style>
