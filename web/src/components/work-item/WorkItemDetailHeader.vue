<template>
  <div
    v-if="detail"
    class="detail-title"
    :class="{ 'issue-detail-title': isIssueDetail }"
  >
    <div class="detail-heading-main">
      <WorkItemTypePill :type="detail.type" :ws-id="wsId" />
      <template v-if="isIssueDetail">
        <a-select
          v-if="
            editingProperty === 'header_status' &&
            fieldEditable(detail.type, 'status') &&
            !statusTransitionOn
          "
          :value="Number(detail.status_id)"
          size="small"
          default-open
          autofocus
          style="width: 140px"
          :aria-label="$t('workspace_status')"
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
        <button
          v-else
          type="button"
          class="detail-pill-trigger"
          :disabled="!fieldEditable(detail.type, 'status')"
          :aria-label="$t('workspace_status')"
          @click="openStatusEditor('header_status')"
        >
          <StatusPill :text="detail.status_name" :color="detail.status_color" />
        </button>
        <template v-if="Number(detail.priority) > 0">
          <a-select
            v-if="
              editingProperty === 'header_priority' &&
              fieldEditable(detail.type, 'priority')
            "
            :value="Number(detail.priority) || undefined"
            size="small"
            default-open
            allowClear
            autofocus
            style="width: 110px"
            :aria-label="$t('workspace_priority')"
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
          <button
            v-else
            type="button"
            class="detail-pill-trigger"
            :disabled="!fieldEditable(detail.type, 'priority')"
            :aria-label="$t('workspace_priority')"
            @click="editingProperty = 'header_priority'"
          >
            <PriorityPill
              :priority="detail.priority"
              :label="detail.priority_name"
              :color="detail.priority_color"
            />
          </button>
        </template>
      </template>
      <a-input
        v-if="editingTitle"
        v-model:value="titleDraft"
        size="large"
        class="detail-title-input"
        autofocus
        @press-enter="saveTitle"
        @blur="saveTitle"
      />
      <span
        v-else
        class="detail-title-text"
        :class="{ editable: canEdit }"
        @click="canEdit && (editingTitle = true)"
      >
        {{ titleDisplay }}
      </span>
      <a-dropdown
        :trigger="['hover']"
        placement="bottom"
        overlay-class-name="detail-copy-dropdown"
      >
        <a-button
          type="text"
          class="share-button"
          :aria-label="$t('requirement_copyTitleLink')"
        >
          <img v-if="isIssueDetail" :src="copyIcon" alt="" />
          <CopyOutlined v-else />
        </a-button>
        <template #overlay>
          <a-menu @click="({ key }) => handleCopyAction(key)">
            <a-menu-item key="titleLink">
              <template #icon><ShareAltOutlined /></template>
              {{ $t("requirement_copyTitleLink") }}
            </a-menu-item>
            <a-menu-item key="title">
              <template #icon><FontSizeOutlined /></template>
              {{ $t("requirement_copyTitle") }}
            </a-menu-item>
            <a-menu-item key="link">
              <template #icon><LinkOutlined /></template>
              {{ $t("requirement_copyLink") }}
            </a-menu-item>
            <a-menu-item key="description">
              <template #icon><FileTextOutlined /></template>
              {{ $t("requirement_copyDescription") }}
            </a-menu-item>
            <a-menu-item key="titleLinkDescription">
              <template #icon><SnippetsOutlined /></template>
              {{ $t("requirement_copyTitleLinkDescription") }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>
    </div>
    <div v-if="isIssueDetail" class="issue-detail-meta">
      <span>{{
        $t("issue_created_at", {
          time: formatLocalDate(detail.created_at, true),
        })
      }}</span>
      <span class="issue-meta-divider" aria-hidden="true" />
      <span>{{ $t("workspace_assignee") }} {{ assigneeDisplay }}</span>
    </div>
    <div class="detail-header-actions">
      <a-tooltip :title="$t('requirement_previousItem')">
        <a-button
          type="text"
          class="detail-action-button"
          :disabled="!previousNavigationItem"
          :aria-label="$t('requirement_previousItem')"
          @click="navigateRelative(-1)"
        >
          <UpOutlined />
        </a-button>
      </a-tooltip>
      <a-tooltip :title="$t('requirement_nextItem')">
        <a-button
          type="text"
          class="detail-action-button"
          :disabled="!nextNavigationItem"
          :aria-label="$t('requirement_nextItem')"
          @click="navigateRelative(1)"
        >
          <DownOutlined />
        </a-button>
      </a-tooltip>
      <a-tooltip v-if="canEdit" :title="$t('requirement_fullscreenEdit')">
        <a-button
          type="text"
          class="detail-action-button"
          :aria-label="$t('requirement_fullscreenEdit')"
          @click="openFullscreenEditor"
        >
          <EditOutlined />
        </a-button>
      </a-tooltip>
      <a-dropdown
        v-if="hasMoreActions"
        :disabled="descriptionEditing"
        :trigger="['click']"
        placement="bottomRight"
      >
        <a-button
          type="text"
          class="detail-action-button"
          :disabled="descriptionEditing"
          :title="$t('common_more')"
          :aria-label="$t('common_more')"
        >
          <MoreOutlined />
        </a-button>
        <template #overlay>
          <a-menu @click="({ key }) => handleDetailAction(key)">
            <a-menu-item v-if="canCreateChild" key="child">
              <ApartmentOutlined /> {{ $t("requirement_createChild") }}
            </a-menu-item>
            <a-menu-item v-if="canCreateDefect" key="createDefect">
              <BugOutlined /> {{ $t("requirement_createBug") }}
            </a-menu-item>
            <a-menu-item v-if="canCopyDetail" key="copy">
              <CopyOutlined /> {{ copyMenuText }}
            </a-menu-item>
            <a-menu-item v-if="canMoveDetail" key="move">
              <FolderOpenOutlined /> {{ moveMenuText }}
            </a-menu-item>
            <a-menu-item v-if="canConvertDetail" key="convert">
              <SwapOutlined /> {{ convertMenuText }}
            </a-menu-item>
            <a-menu-item v-if="canMergeDetail" key="merge">
              <MergeCellsOutlined /> {{ $t("workItemAction_mergeDefect") }}
            </a-menu-item>
            <a-menu-divider v-if="canDeleteDetail && hasNonDeleteAction" />
            <a-menu-item v-if="canDeleteDetail" key="delete" danger>
              <DeleteOutlined /> {{ $t("common_delete") }}
            </a-menu-item>
          </a-menu>
        </template>
      </a-dropdown>
      <a-tooltip :title="$t('requirement_openNewWindow')" v-if="mode != 'page'">
        <a-button
          type="text"
          class="detail-action-button"
          :disabled="descriptionEditing"
          :aria-label="$t('requirement_openNewWindow')"
          @click="openInNewWindow"
        >
          <ExportOutlined />
        </a-button>
      </a-tooltip>
      <a-tooltip :title="$t('common_close')">
        <a-button
          type="text"
          class="detail-action-button detail-close"
          :aria-label="$t('common_close')"
          @click="requestClose"
        >
          <img v-if="isIssueDetail" :src="closeIcon" alt="" />
          <CloseOutlined v-else />
        </a-button>
      </a-tooltip>
    </div>
  </div>
  <span v-else>{{ $t("requirement_detail") }}</span>
</template>
<script setup>
import { computed } from "vue";
import {
  ApartmentOutlined,
  BugOutlined,
  CloseOutlined,
  CopyOutlined,
  DeleteOutlined,
  DownOutlined,
  EditOutlined,
  ExportOutlined,
  FileTextOutlined,
  FolderOpenOutlined,
  FontSizeOutlined,
  LinkOutlined,
  MergeCellsOutlined,
  MoreOutlined,
  ShareAltOutlined,
  SnippetsOutlined,
  SwapOutlined,
  UpOutlined,
} from "@ant-design/icons-vue";
import StatusPill from "@/components/work-item/StatusPill.vue";
import PriorityPill from "@/components/work-item/PriorityPill.vue";
import copyIcon from "@/image/issue-detail/copy.svg";
import closeIcon from "@/image/issue-detail/close.svg";
import { formatLocalDate } from "@/utils/date";
import WorkItemTypePill from "@/components/common/WorkItemTypePill.vue";
const props = defineProps({ model: { type: Object, required: true } });
const {
  t,
  isIssueDetail,
  requestClose,
  detail,
  editingProperty,
  fieldEditable,
  statusOptions,
  statusTransitionOn,
  openStatusEditor,
  priorities,
  displayName,
  saveProperty,
  editingTitle,
  descriptionEditing,
  titleDraft,
  titleDisplay,
  canEdit,
  previousNavigationItem,
  nextNavigationItem,
  canCreateChild,
  canCreateDefect,
  canCopyDetail,
  canMoveDetail,
  canConvertDetail,
  canMergeDetail,
  canDeleteDetail,
  hasNonDeleteAction,
  hasMoreActions,
  moveMenuText,
  copyMenuText,
  convertMenuText,
  navigateRelative,
  openFullscreenEditor,
  openInNewWindow,
  handleDetailAction,
  handleCopyAction,
  saveTitle,
  wsId,
  mode,
} = props.model;
const assigneeDisplay = computed(() => {
  const names = detail.value?.assignee_names;
  return (
    (Array.isArray(names) ? names.filter(Boolean).join(";") : names) || "—"
  );
});
</script>
<style scoped lang="less">
@import "./workItemDetail.less";
</style>
