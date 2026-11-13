import { buildIssueProcessingRecords } from "@/utils/issueProcessingRecords";
import {
  computed,
  nextTick,
  provide,
  toRefs,
  ref,
  watch,
  onMounted,
  onBeforeUnmount,
} from "vue";
import { Drawer, Empty, message, Modal } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import {
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
  useRouter,
  useRoute,
} from "vue-router";
import dayjs from "dayjs";

import { useAuthStore } from "@/stores/auth";
import { useLocalDraft } from "@/composables/useLocalDraft";
import { registerSaveHotkey } from "@/utils/saveHotkey";
import { useWorkItemTypeAttributes } from "@/composables/useWorkItemTypeAttributes";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";
import { buildCategoryOptions } from "@/utils/requirementCategory";
import { formatLocalDate } from "@/utils/date";
import {
  CUSTOM_FIELD_TYPE,
  isMultiValueFieldType,
} from "@/constants/customField";
import {
  formatCustomFieldValue,
  isValidPhoneFieldValue,
  parseCustomFieldValue,
  serializeCustomFieldValue,
} from "@/utils/customField";
import { normalizeWorkItemNavigationItems } from "@/utils/workItemNavigation";
import {
  buildWorkItemPageUrl,
  combineDescriptionContent,
  descriptionContent,
  writeClipboardRichContent,
  writeClipboardText,
} from "@/utils/workItemClipboard";
import {
  assignRequirementCategory,
  deleteRequirement,
  getRequirement,
  listRequirementAttachments,
  listRequirementCategories,
  listRequirementHistories,
  listRequirements,
  getWorkItemDisplaySettings,
  getWorkspaceMetadata,
  recordRequirementVisit,
  updateRequirement,
} from "@/api/workspace";
import { isStatusTransitionEnabled } from "@/utils/statusTransition";
import {
  showWorkItemActionSuccess,
  showWorkItemUpdateSuccess,
} from "@/utils/workItemActionToast";
import { workItemRoute } from "@/utils/workItemRoutes";
export function useWorkItemDetail(props, emit) {
  const { t } = useI18n();
  const router = useRouter();
  const surface = useRoute().meta.workItemSurface;
  const isIssueDetail = surface === "issue";
  const authStore = useAuthStore();
  // 产出文档入口/抽屉据此决定落点：整页详情下开新标签，抽屉详情下就地跳转；
  // 文档抽屉看到有值就只收起自己，回到下面已经打开的详情。
  provide(
    "workItemDetailMode",
    computed(() => (props.open ? props.mode : "")),
  );
  const workItemTypeAttributes = useWorkItemTypeAttributes(
    computed(() => props.wsId),
  );
  // Every defect entry in the drawer (related tab, quick add, create/convert/merge) hangs
  // off this single switch.
  const { defectAppEnabled } = useWorkspaceApplications(
    computed(() => props.wsId),
  );
  // 当前工作项的本地草稿；type 在 loadDetail 后通过 setTarget 补充
  const draft = useLocalDraft({
    kind: "work-item",
    userId: authStore.userInfo?.id || "",
    wsId: props.wsId,
    type: "",
    targetId: props.requirementId,
    onStorageError: () => {
      message.warning(t("draft_storage_failed"));
    },
  });
  const draftModalOpen = ref(false);
  const draftRecords = ref([]);
  const draftRestoreRevision = ref(0);
  const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE;
  const drawerMinWidth = 684;
  // 属性面板的位置由实际可用宽度决定：够宽就在右侧做独立列，不够就落到正文上方。
  const drawerLayoutBreakpoint = 860;
  const drawerMaxWidth = 1200;
  const drawerDefaultWidth = isIssueDetail ? 980 : 860;
  const detailContainer = computed(() =>
    props.mode === "modal" ? Modal : Drawer,
  );
  const drawerWidthStorageKey = "goteams-requirement-detail-drawer-width";
  function storedDrawerWidth() {
    try {
      const width = Number(window.localStorage.getItem(drawerWidthStorageKey));
      if (width >= drawerMinWidth && width <= drawerMaxWidth) return width;
    } catch {
      // Local storage may be unavailable under restricted browser settings.
    }
    return drawerDefaultWidth;
  }
  function saveDrawerWidth(width) {
    const normalizedWidth = Math.round(width);
    if (normalizedWidth < drawerMinWidth || normalizedWidth > drawerMaxWidth)
      return;
    try {
      window.localStorage.setItem(
        drawerWidthStorageKey,
        String(normalizedWidth),
      );
    } catch {
      // Keep resizing available even when the preference cannot be persisted.
    }
  }
  const drawerWidthValue = ref(storedDrawerWidth());
  const viewportWidth = ref(window.innerWidth);
  const drawerWidth = computed(() =>
    Math.min(drawerWidthValue.value, viewportWidth.value),
  );
  const detailContainerProps = computed(() => {
    if (props.mode === "modal") {
      return {
        rootClassName: `work-item-detail-modal${isIssueDetail ? " issue-detail-surface" : ""}`,
        open: props.open,
        width: "min(1100px, calc(100vw - 48px))",
        mask: true,
        maskClosable: true,
        keyboard: !descriptionEditing.value,
        closable: false,
        footer: null,
        centered: true,
      };
    }
    if (props.mode === "page") {
      return {
        rootClassName: `work-item-detail-page${isIssueDetail ? " issue-detail-surface" : ""}`,
        open: props.open,
        width: "100%",
        // 同上：关闭态宽度样式会被摘掉，整页详情收起时也会先被内容撑开再滑出。
        contentWrapperStyle: { width: "100%" },
        mask: false,
        keyboard: false,
        closable: false,
        placement: "right",
        getContainer: false,
        rootStyle: { position: "absolute" },
      };
    }
    return {
      rootClassName: `work-item-detail-drawer${isIssueDetail ? " issue-detail-surface" : ""}`,
      open: props.open,
      width: drawerWidth.value,
      // antd 在 mask=false 的抽屉关闭时会清掉 width 内联样式（drawer/index.js 的 offsetStyle
      // 在 !visible && !mask 时返回空对象），content-wrapper 于是退化成按内容收缩；收起动画里
      // 宽度先被内容撑开再随 transform 滑出，视觉上就是"弹一下"。这里在 contentWrapperStyle
      // 里再钉一份宽度，关闭态也保持固定宽度。
      contentWrapperStyle: { width: `${drawerWidth.value}px` },
      mask: false,
      keyboard: !descriptionEditing.value,
      closable: false,
      placement: "right",
    };
  });
  const isNarrowLayout = computed(() =>
    props.mode === "drawer"
      ? drawerWidth.value < drawerLayoutBreakpoint
      : viewportWidth.value - (props.mode === "page" ? 268 : 48) <
        drawerLayoutBreakpoint,
  );
  let detailLoadSequence = 0;
  let detailDisposed = false;
  let loadedDetailKey = "";
  function detailLoadContext() {
    const sequence = detailLoadSequence;
    const wsId = props.wsId;
    const id = props.requirementId;
    return {
      wsId,
      id,
      isCurrent: () =>
        !detailDisposed &&
        sequence === detailLoadSequence &&
        props.open &&
        !!wsId &&
        !!id &&
        String(props.wsId) === String(wsId) &&
        String(props.requirementId) === String(id),
    };
  }
  const loading = ref(false);
  const detailLeftRef = ref(null);
  let drawerResizeCleanup;
  let detailLeftResizeObserver;

  function syncDescriptionActionsPosition() {
    if (!descriptionEditing.value || !detailLeftRef.value) return;
    const rect = detailLeftRef.value.getBoundingClientRect();
    descriptionEditingStyle.value = {
      "--description-actions-right": `${Math.max(
        0,
        window.innerWidth - rect.right,
      )}px`,
      "--description-actions-bottom": `${Math.max(
        0,
        window.innerHeight - rect.bottom,
      )}px`,
      "--description-actions-width": `${rect.width}px`,
    };
  }

  function confirmLeaveEditing() {
    if (!descriptionEditing.value) return true;
    if (
      isDescriptionDirty.value &&
      !window.confirm(t("editor_unsaved_leave"))
    ) {
      return false;
    }
    descriptionEditorRef.value?.cancelEdit?.();
    return true;
  }
  function requestClose() {
    if (!confirmLeaveEditing()) return;
    emit("close");
  }
  function onBeforeUnload(event) {
    if (!isDescriptionDirty.value) return;
    event.preventDefault();
    event.returnValue = "";
  }
  // Auto-collapse the drawer when clicking anywhere outside the detail panel.
  function onDocMouseDown(e) {
    if (!props.open || props.mode === "page") return;
    // The Ant overlay is mounted on body but still belongs to the interaction of controls inside the drawer.
    // The produced-documents drawer stacks on top of the detail; without it listed here every
    // click inside that drawer closes the detail and takes the documents drawer down with it.
    if (
      e.target?.closest?.(
        ".work-item-detail-drawer, .work-item-detail-modal, .task-documents-drawer, .ant-select-dropdown, .ant-picker-dropdown, .ant-dropdown, .ant-tabs-dropdown, .ant-modal-root, .viewer-container",
      )
    )
      return;
    if (descriptionEditing.value) {
      e.preventDefault();
      e.stopPropagation();
      if (!confirmLeaveEditing()) return;
    }
    emit("close");
  }
  function onWindowResize() {
    viewportWidth.value = window.innerWidth;
    requestAnimationFrame(syncDescriptionActionsPosition);
  }
  onMounted(function () {
    document.addEventListener("mousedown", onDocMouseDown, true);
    window.addEventListener("beforeunload", onBeforeUnload);
    window.addEventListener("resize", onWindowResize);
    if (typeof ResizeObserver !== "undefined") {
      detailLeftResizeObserver = new ResizeObserver(
        syncDescriptionActionsPosition,
      );
      if (detailLeftRef.value)
        detailLeftResizeObserver.observe(detailLeftRef.value);
    }
  });
  onBeforeUnmount(function () {
    detailDisposed = true;
    detailLoadSequence += 1;
    // SPA 离开前尽力落盘，不标记已提交。
    void draft.flush();
    unregisterSaveHotkey?.();
    unregisterSaveHotkey = null;
    document.removeEventListener("mousedown", onDocMouseDown, true);
    window.removeEventListener("beforeunload", onBeforeUnload);
    window.removeEventListener("resize", onWindowResize);
    detailLeftResizeObserver?.disconnect();
    drawerResizeCleanup?.();
  });
  onBeforeRouteLeave(() => confirmLeaveEditing());
  onBeforeRouteUpdate(() => confirmLeaveEditing());
  function startDrawerResize(event) {
    if (event.button !== 0 || viewportWidth.value <= 720) return;
    event.preventDefault();
    const wrapper = event.currentTarget.closest(".ant-drawer-content-wrapper");
    if (!wrapper) return;
    const startX = event.clientX;
    const startWidth = wrapper.getBoundingClientRect().width;
    const previousCursor = document.body.style.cursor;
    const previousUserSelect = document.body.style.userSelect;
    const previousTransition = wrapper.style.transition;
    let nextWidth = startWidth;
    let resizeFrame;
    const applyWidth = () => {
      wrapper.style.width = `${nextWidth}px`;
      drawerWidthValue.value = nextWidth;
      resizeFrame = undefined;
    };
    const onMouseMove = (moveEvent) => {
      const maxWidth = Math.min(drawerMaxWidth, viewportWidth.value);
      const minWidth = Math.min(drawerMinWidth, maxWidth);
      nextWidth = Math.min(
        maxWidth,
        Math.max(minWidth, startWidth + startX - moveEvent.clientX),
      );
      if (!resizeFrame) resizeFrame = requestAnimationFrame(applyWidth);
    };
    const stopResize = () => {
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", stopResize);
      if (resizeFrame) cancelAnimationFrame(resizeFrame);
      applyWidth();
      saveDrawerWidth(nextWidth);
      document.body.style.cursor = previousCursor;
      document.body.style.userSelect = previousUserSelect;
      drawerResizeCleanup = undefined;
      requestAnimationFrame(() => {
        wrapper.style.transition = previousTransition;
      });
    };
    drawerResizeCleanup = stopResize;
    wrapper.style.transition = "none";
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", stopResize);
  }
  function resizeDrawerByKeyboard(event) {
    const step = 24;
    const maxWidth = Math.min(drawerMaxWidth, viewportWidth.value);
    const minWidth = Math.min(drawerMinWidth, maxWidth);
    let width = drawerWidthValue.value;
    if (event.key === "ArrowLeft") width += step;
    else if (event.key === "ArrowRight") width -= step;
    else if (event.key === "Home") width = minWidth;
    else if (event.key === "End") width = maxWidth;
    else return;
    drawerWidthValue.value = Math.min(maxWidth, Math.max(minWidth, width));
    saveDrawerWidth(drawerWidthValue.value);
    event.preventDefault();
  }
  const detail = ref(null);
  const requirementCategories = ref(null);
  const categorySaving = ref(false);
  const categoryOptions = computed(() =>
    buildCategoryOptions(
      requirementCategories.value,
      t("requirement_categoryUncategorized"),
    ),
  );

  function requirementCategoryName(categoryId) {
    const id = Number(categoryId) || 0;
    if (!id) return t("requirement_categoryUncategorized");
    return (
      requirementCategories.value?.find((item) => Number(item.id) === id)
        ?.name || `#${id}`
    );
  }

  const detailTab = ref("desc");
  // Bumped on every detail reload so panes showing derived data (时效) re-fetch after an inline
  // field edit instead of keeping the snapshot they loaded when they were opened.
  const detailReloadKey = ref(0);
  // The related-defects pane disappears with the app; an active key without a matching pane
  // would render an empty tab body, so fall back to the description.
  watch(defectAppEnabled, (enabled) => {
    if (!enabled && detailTab.value === "defects") detailTab.value = "desc";
  });
  const propertiesCollapsed = ref(false);
  const editingTitle = ref(false);
  const editingParent = ref(false);
  const editingProperty = ref("");
  const workLogsRef = ref(null);
  const estimatedWorkHoursDraft = ref(0);
  const estimatedWorkHoursSaving = ref(false);
  const handledWorkLogEditorToken = ref(0);
  const descriptionEditing = ref(false);
  const descriptionEditorRef = ref(null);
  const titleDraft = ref("");
  const descriptionDraft = ref("");
  // 收集编辑期间上传的富文本图片/文件/视频台账 ID，保存时随 image_file_ids 提交并回绑到当前需求。
  const imageFileIds = ref([]);
  // 正文编辑会话内展示草稿中的标题，保证恢复后立即可见
  const titleDisplay = computed(() =>
    descriptionEditing.value ? titleDraft.value : detail.value?.title || "",
  );
  const descriptionModeDraft = ref("rich-text");
  const workspacePermissions = ref([]);
  const workspacePermissionsLoaded = ref(false);
  const histories = ref([]);
  const processingHistories = computed(() =>
    historyLoaded.value
      ? buildIssueProcessingRecords(histories.value, detail.value)
      : [],
  );
  const historyLoading = ref(false);
  const historyLoaded = ref(false);
  let historyRequestSequence = 0;
  const childRequirements = ref([]);
  const relatedDefects = ref([]);
  const finalStatusIds = ref({ requirement: [], defect: [] });
  const attachments = ref([]);
  const members = ref([]);
  const customFieldMembers = ref([]);
  // Disabled and removed members cannot be picked; existing assignments still render from `members`.
  const assignableMembers = computed(() =>
    members.value.filter((m) => String(m.user_status) !== "0" && !m.removed),
  );
  const selectableMembers = computed(() => assignableMembers.value);
  const versions = ref([]);
  const statuses = ref({ requirement: [], defect: [] });
  const priorities = ref({ requirement: [], defect: [] });
  const fieldConfigs = ref({ requirement: [], defect: [] });
  const customFields = ref({ requirement: [], defect: [] });
  const displaySettings = ref({ requirement: null, defect: null });
  const workspaceMetadata = ref(null);
  const transitionOpen = ref(false);
  let statusTransitionContext;
  const statusTransitionOn = computed(() =>
    isStatusTransitionEnabled(
      workspaceMetadata.value?.settings,
      detail.value?.type,
    ),
  );
  const customValues = ref({});
  // 窄布局下属性在正文上方，评论输入框无需给右侧列让位
  const commentInputStyle = computed(() => ({
    right:
      propertiesCollapsed.value || isNarrowLayout.value
        ? "0px"
        : isIssueDetail
          ? "min(42.04%, 412px)"
          : "292px",
  }));
  const descriptionEditingStyle = ref({});
  let unregisterSaveHotkey = null;
  const savedEditSnapshot = ref({ title: "", html: "", mode: "" });
  const savingDescription = ref(false);
  const isDescriptionDirty = computed(() => {
    if (!descriptionEditing.value) return false;
    return (
      titleDraft.value !== savedEditSnapshot.value.title ||
      descriptionDraft.value !== savedEditSnapshot.value.html ||
      descriptionModeDraft.value !== savedEditSnapshot.value.mode
    );
  });
  function captureEditSnapshot() {
    savedEditSnapshot.value = {
      title: titleDraft.value,
      html: descriptionDraft.value,
      mode: descriptionModeDraft.value,
    };
  }
  watch(descriptionEditing, (editing) => {
    unregisterSaveHotkey?.();
    unregisterSaveHotkey = null;
    if (!editing) {
      descriptionEditingStyle.value = {};
      return;
    }
    // 只在正文编辑会话内接管 Ctrl/Cmd+S，其余场景保留浏览器默认行为
    unregisterSaveHotkey = registerSaveHotkey(() => {
      descriptionEditorRef.value?.flushAndSaveStay?.();
    });
    nextTick(() => {
      syncDescriptionActionsPosition();
      captureEditSnapshot();
    });
  });
  watch(detailLeftRef, (element, previousElement) => {
    if (previousElement) detailLeftResizeObserver?.unobserve(previousElement);
    if (!element) return;
    detailLeftResizeObserver?.observe(element);
    if (descriptionEditing.value) nextTick(syncDescriptionActionsPosition);
  });
  const canEdit = computed(() => {
    if (detail.value && workspacePermissionsLoaded.value) {
      return hasWorkspacePermission(detail.value.type, "edit");
    }
    if (!props.canEditByType || !detail.value) return props.canEdit;
    return Boolean(props.canEditByType[detail.value.type]);
  });
  const selectedIteration = computed(() =>
    versions.value.find(
      (item) => Number(item.id) === Number(detail.value?.version_id),
    ),
  );
  const selectableVersions = computed(() => {
    const selectedId = Number(detail.value?.version_id) || 0;
    return versions.value.filter(
      (item) => !item.is_ended || Number(item.id) === selectedId,
    );
  });
  const normalizedNavigationItems = computed(() =>
    normalizeWorkItemNavigationItems(props.navigationItems),
  );
  const currentNavigationIndex = computed(() =>
    normalizedNavigationItems.value.findIndex(
      (item) =>
        Number(item.id) === Number(props.requirementId) &&
        Number(item.workspaceId) === Number(props.wsId),
    ),
  );
  const previousNavigationItem = computed(() => {
    const index = currentNavigationIndex.value;
    return index > 0 ? normalizedNavigationItems.value[index - 1] : null;
  });
  const nextNavigationItem = computed(() => {
    const index = currentNavigationIndex.value;
    return index >= 0 && index < normalizedNavigationItems.value.length - 1
      ? normalizedNavigationItems.value[index + 1]
      : null;
  });
  const canCreateChild = computed(
    () => detail.value?.type === "requirement" && canCreateType("requirement"),
  );
  const canCreateDefect = computed(
    () =>
      defectAppEnabled.value &&
      detail.value?.type === "requirement" &&
      canCreateType("defect"),
  );
  const canCopyDetail = computed(
    () =>
      hasWorkspacePermission(detail.value?.type, "copy") &&
      canCreateType(detail.value?.type),
  );
  const canMoveDetail = computed(() =>
    hasWorkspacePermission(detail.value?.type, "edit"),
  );
  // Conversion always has a defect on one side of it, so it is an app-gated action.
  const canConvertDetail = computed(
    () =>
      defectAppEnabled.value &&
      hasWorkspacePermission(detail.value?.type, "edit"),
  );
  const canMergeDetail = computed(
    () =>
      defectAppEnabled.value &&
      detail.value?.type === "defect" &&
      hasWorkspacePermission("defect", "delete"),
  );
  const canDeleteDetail = computed(() =>
    hasWorkspacePermission(detail.value?.type, "delete"),
  );
  const hasNonDeleteAction = computed(
    () =>
      canCreateChild.value ||
      canCreateDefect.value ||
      canCopyDetail.value ||
      canMoveDetail.value ||
      canConvertDetail.value ||
      canMergeDetail.value,
  );
  const hasMoreActions = computed(
    () => hasNonDeleteAction.value || canDeleteDetail.value,
  );
  const moveMenuText = computed(() =>
    t(
      detail.value?.type === "defect"
        ? "workItemAction_moveDefect"
        : "workItemAction_moveRequirement",
    ),
  );
  const copyMenuText = computed(() =>
    t(
      detail.value?.type === "defect"
        ? "workItemAction_copyDefect"
        : "workItemAction_copyRequirement",
    ),
  );
  const convertMenuText = computed(() =>
    t(
      detail.value?.type === "defect"
        ? "workItemAction_convertRequirement"
        : "workItemAction_convertDefect",
    ),
  );
  const actionModalOpen = ref(false);
  const actionModalType = ref("");
  const actionTargetType = computed(() => {
    if (actionModalType.value === "convert") {
      return detail.value?.type === "defect" ? "requirement" : "defect";
    }
    return detail.value?.type || "requirement";
  });
  const hasParent = computed(() => Number(detail.value?.parent_id) > 0);
  const statusOptions = computed(() => {
    if (!detail.value) return [];
    const all = statuses.value[detail.value.type] || [];
    if (!Array.isArray(detail.value.available_status_ids)) return all;
    const allowed = new Set(detail.value.available_status_ids.map(Number));
    allowed.add(Number(detail.value.status_id));
    return all.filter((item) => allowed.has(Number(item.id)));
  });

  function displayName(name) {
    return name?.startsWith("workitem.")
      ? t(name.replaceAll(".", "_"))
      : name || "-";
  }
  function fieldConfig(type, key) {
    return (fieldConfigs.value[type] || []).find((item) => item.key === key);
  }
  const workHourFieldKeys = new Set([
    "estimated_work_hours",
    "work_progress",
    "completed_work_hours",
    "remaining_work_hours",
    "overtime_work_hours",
  ]);
  function hasWorkspacePermission(type, action) {
    if (!type) return false;
    return workspacePermissions.value.includes(`${type}:${action}`);
  }
  function canCreateType(type) {
    if (workspacePermissionsLoaded.value)
      return hasWorkspacePermission(type, "create");
    return Boolean(props.canCreateByType?.[type]);
  }
  function fieldEnabled(type, key) {
    const config = fieldConfig(type, key);
    if (workHourFieldKeys.has(key) && !config) return false;
    if (config && Number(config.enabled) !== 1) return false;
    const setting = displaySettings.value[type];
    if (!setting?.template_enabled) return true;
    return resolvedDetailFieldKeys(type).includes(
      normalizedDisplayFieldKey(key),
    );
  }

  function resolvedViewKeys(setting) {
    if (setting?.view_display_mode === "status") {
      const match = (setting.status_fields || []).find(
        (item) => Number(item.status_id) === Number(detail.value?.status_id),
      );
      if (match) return match.field_keys || [];
    }
    return setting?.default_fields || [];
  }

  function resolvedDetailFieldKeys(type) {
    const setting = displaySettings.value[type];
    if (!setting) return [];
    if (!setting.template_enabled) return setting.default_fields || [];
    if (descriptionEditing.value) {
      return setting.edit_display_mode === "view"
        ? resolvedViewKeys(setting)
        : setting.default_fields || [];
    }
    return resolvedViewKeys(setting);
  }

  function normalizedDisplayFieldKey(key) {
    return key.replace(/^custom_/, "custom:");
  }

  function fieldOrderStyle(type, key) {
    const normalizedKey = normalizedDisplayFieldKey(key);
    const displayIndex = resolvedDetailFieldKeys(type).indexOf(normalizedKey);
    if (displayIndex >= 0) return { order: displayIndex };

    const config = fieldConfig(type, normalizedKey);
    if (config) return { order: 10000 + Number(config.sort_order || 0) };
    const customId = Number(normalizedKey.replace(/^custom:/, ""));
    const customField = (customFields.value[type] || []).find(
      (item) => Number(item.id) === customId,
    );
    return { order: 10000 + Number(customField?.sort_order || 0) };
  }

  function fieldEditable(type, key) {
    const config = fieldConfig(type, key);
    return (
      canEdit.value &&
      (key === "status" || !config || Number(config.editable) === 1)
    );
  }
  // Plain-text display of custom fields in the detail panel (shown when not in edit mode).
  function customDisplay(field) {
    return formatCustomFieldValue(
      field,
      {
        [field.id]: serializeCustomFieldValue(
          field,
          customValues.value[field.id],
        ),
      },
      customFieldMembers.value,
    );
  }
  const assigneeIdsValue = computed(() =>
    Array.isArray(detail.value?.assignee_ids)
      ? detail.value.assignee_ids.map(Number)
      : [],
  );

  function historyActionText(item) {
    const keys = {
      created: "requirement_history_created",
      status_change: "requirement_historyStatusChanged",
      // 邀填工单被提交人撤回时的状态落库，变更类型单独显示「用户撤回」
      issue_recall: "requirement_history_issue_recall",
      field_update: "requirement_historyFieldUpdated",
      assignee_change: "requirement_historyAssigneeChanged",
      restore: "requirement_history_restored",
    };
    return keys[item.action] ? t(keys[item.action]) : item.action;
  }

  // Only a description change keeps a comparable before/after version; every other field keeps its
  // plain-text rendering in the list.
  function isDescriptionHistory(item) {
    return item.field_name === "description";
  }

  function openHistoryDetail(item) {
    const target = router.resolve({
      name:
        surface === "issue" ? "IssueHistoryDetail" : "WorkItemHistoryDetail",
      params: {
        wsId: props.wsId,
        id: props.requirementId,
        historyId: item.id,
      },
    });
    window.open(target.href, "_blank", "noopener,noreferrer");
  }

  function historyFieldText(item) {
    const keys = {
      title: "workspace_workItemTitle",
      description: "requirement_description",
      status: "workspace_status",
      status_id: "workspace_status",
      priority: "workspace_priority",
      category_id: "workspace_field_category",
      assignee: "workspace_assignee",
      version_id: "workspace_version",
      planned_start_date: "requirement_planStart",
      planned_end_date: "requirement_planEnd",
      parent_id: "requirement_parentReq",
      work_hours: "requirement_estimatedWorkHours",
    };
    if (item.action === "created")
      return workItemTypeAttributes.displayName(
        detail.value?.type === "defect" ? "defect" : "requirement",
      );
    return keys[item.field_name]
      ? t(keys[item.field_name])
      : item.field_name || "-";
  }

  function historyStatusColor(item) {
    const label = historyValueText(item, item.new_value);
    const option = (statuses.value[detail.value?.type] || []).find(
      (entry) => displayName(entry.name) === label,
    );
    return option?.color || "#9ba4af";
  }

  function historyValueText(item, value) {
    if (value === undefined || value === null || value === "") return "-";
    const wrappedKey = /^\$\{(.+)\}$/.exec(String(value))?.[1];
    const storedStatusKeys = {
      workitem_status_pending_plan: "requirement_backlog",
      workitem_status_pending_assign: "requirement_assigned",
      workitem_status_in_progress: "requirement_inProgress",
      workitem_status_done: "requirement_done",
      workitem_status_resolved: "requirement_resolved",
      workitem_status_closed: "requirement_closed",
      workitem_status_rejected: "requirement_rejected",
    };
    if (wrappedKey && storedStatusKeys[wrappedKey])
      return t(storedStatusKeys[wrappedKey]);
    const field = item.field_name;
    if (field === "category_id") return requirementCategoryName(value);
    if (field === "parent_id")
      return Number(value) > 0
        ? t("requirement_parent_linked")
        : t("requirement_parent_removed");
    // Date fields: the backend stores a Unix timestamp; the frontend must convert it to a readable date (precise to the second).
    // Date fields: selection is precise only to the day; history records are also shown only to the day (0 value returns "-").
    if (field === "planned_start_date" || field === "planned_end_date")
      return formatLocalDate(Number(value));
    if (field === "work_hours") return formatWorkHours(value);
    const id = Number(value);
    if (field === "status" || field === "status_id") {
      const option = (statuses.value[detail.value?.type] || []).find(
        (entry) => Number(entry.id) === id,
      );
      return option ? displayName(option.name) : value;
    }
    if (field === "priority") {
      if (id === 0) return "-";
      const option = (priorities.value[detail.value?.type] || []).find(
        (entry) => Number(entry.id) === id,
      );
      return option ? displayName(option.name || option.label) : value;
    }
    if (field === "assignee")
      return String(value)
        .split(",")
        .map((s) => s.trim())
        .filter((s) => s !== "")
        .map(
          (id) =>
            members.value.find((entry) => Number(entry.user_id) === Number(id))
              ?.user_name || id,
        )
        .join("、");
    if (field === "version_id")
      return (
        versions.value.find((entry) => Number(entry.id) === id)?.name || value
      );
    return value;
  }

  function setAttachments(files) {
    attachments.value = files || [];
  }
  async function reloadAttachments() {
    const context = detailLoadContext();
    if (!context.isCurrent()) return;
    const files = await listRequirementAttachments(context.wsId, context.id);
    if (context.isCurrent()) setAttachments(files);
  }

  function goCreate(type, action = "child") {
    emit("close");
    router.push({
      ...workItemRoute(surface, "create", props.wsId),
      query: {
        type,
        parentId: props.requirementId,
        detailAction: action,
      },
    });
  }

  function goCopy() {
    emit("close");
    router.push({
      ...workItemRoute(surface, "create", props.wsId),
      query: {
        type: detail.value.type,
        copyFrom: props.requirementId,
        detailAction: "copy",
      },
    });
  }

  function navigateRelative(offset) {
    if (!confirmLeaveEditing()) return;
    const target =
      offset < 0 ? previousNavigationItem.value : nextNavigationItem.value;
    if (!target) return;
    emit("navigate", target.id, target.workspaceId);
  }

  async function openFullscreenEditor() {
    detailTab.value = "desc";
    await nextTick();
    descriptionEditorRef.value?.enterFullscreenEdit?.();
  }

  // 与「复制链接」保持一致：新窗口打开同一个可分享地址，不携带列表导航上下文和来源页签。
  function openInNewWindow() {
    if (descriptionEditing.value) return;
    window.open(detailPageUrl(), "_blank", "noopener,noreferrer");
  }

  function openParentInNewWindow() {
    if (!hasParent.value) return;
    const target = router.resolve({
      name: surface === "issue" ? "IssueDetail" : "WorkItemDetail",
      params: { wsId: props.wsId, id: detail.value.parent_id },
    });
    window.open(target.href, "_blank", "noopener,noreferrer");
  }

  function deleteDetailItem() {
    if (!canDeleteDetail.value || !detail.value) return;
    Modal.confirm({
      title: t("workspace_confirmDelete"),
      content: t("workspace_confirmDeleteContent", {
        title: detail.value.title,
      }),
      okText: t("common_delete"),
      okType: "danger",
      cancelText: t("common_cancel"),
      onOk: async () => {
        await deleteRequirement(props.wsId, props.requirementId);
        message.success(t("workspace_deleteSuccess"));
        emit("updated", { action: "refresh" });
        // 工作项已删除，清空快照：keepAlive 的宿主不能再用它把抽屉重新打开。
        detail.value = null;
        emit("close");
      },
    });
  }

  function handleDetailAction(key) {
    if (descriptionEditing.value) return;
    if (key === "child") return goCreate("requirement", "child");
    if (key === "createDefect") return goCreate("defect", "createDefect");
    if (key === "copy") return goCopy();
    if (["move", "convert", "merge"].includes(key)) {
      actionModalType.value = key;
      actionModalOpen.value = true;
      return;
    }
    if (key === "delete") deleteDetailItem();
  }

  function closeActionModal() {
    actionModalOpen.value = false;
    actionModalType.value = "";
  }

  function handleActionSuccess(result) {
    const successKey = `workItemAction_${actionModalType.value}Success`;
    showWorkItemActionSuccess(
      router,
      t(successKey),
      result,
      t("workItemAction_viewDetail"),
    );
    closeActionModal();
    emit("updated", { action: "refresh" });
    // 移动/转换/合并后当前工作项已不是原来那条，快照作废，避免 keepAlive 复用。
    detail.value = null;
    emit("close");
  }

  function detailBoardUrl() {
    return buildBoardLinkUrl(props.wsId, props.requirementId);
  }

  function detailPageUrl() {
    return buildWorkItemPageUrl(props.wsId, props.requirementId, surface);
  }

  async function copyContent(text, successKey = "requirement_copySuccess") {
    if (!text) return;
    try {
      await writeClipboardText(text);
      message.success(t(successKey));
    } catch {
      message.error(t("requirement_copyTitleLinkFailed"));
    }
  }

  async function copyRichContent(content) {
    if (!content?.html && !content?.text) return;
    try {
      await writeClipboardRichContent(content);
      message.success(t("requirement_copySuccess"));
    } catch {
      message.error(t("requirement_copyTitleLinkFailed"));
    }
  }

  async function shareDetail() {
    const title = detail.value?.title || "";
    await copyContent(
      `${title}\n${detailPageUrl()}`,
      "requirement_titleLinkCopied",
    );
  }

  async function handleCopyAction(key) {
    const title = detail.value?.title || "";
    const link = detailPageUrl();
    const description = descriptionContent(
      detail.value?.description || "",
      detail.value?.description_mode,
    );
    if (key === "titleLink") return shareDetail();
    if (key === "title") return copyContent(title);
    if (key === "link") return copyContent(link);
    if (key === "description") return copyRichContent(description);
    if (key === "titleLinkDescription")
      return copyRichContent(
        combineDescriptionContent(title, link, description),
      );
  }

  async function loadOptions(context) {
    if (!context.isCurrent()) return;
    workspacePermissionsLoaded.value = false;
    const types = ["requirement", "defect"];
    const optional = (promise) => promise.catch(() => []);
    const configs =
      authStore.consumeProjectMetadata(context.wsId) ||
      (await optional(getWorkspaceMetadata(context.wsId)));
    if (!context.isCurrent()) return;
    const display = await Promise.all(
      types.map((type) =>
        getWorkItemDisplaySettings(context.wsId, type).catch(() => null),
      ),
    );
    if (!context.isCurrent()) return;
    workspacePermissions.value = Array.isArray(configs?.permissions)
      ? configs.permissions
      : [];
    workspacePermissionsLoaded.value = true;
    members.value = (configs?.members || []).map((item) => ({
      ...item,
      user_id: Number(item.user_id),
    }));
    customFieldMembers.value = (configs?.custom_field_members || []).map(
      (item) => ({
        ...item,
        user_id: Number(item.user_id),
      }),
    );
    // The metadata endpoint returns version rows as strings; the update API
    // expects a numeric id, so normalize it like members and BoardPage do
    versions.value = (configs?.versions || []).map((item) => ({
      ...item,
      id: Number(item.id),
      is_ended: item.is_ended === true || Number(item.ended_at) > 0,
    }));
    statuses.value = Object.fromEntries(
      types.map((type) => [type, configs?.[type]?.statuses || []]),
    );
    priorities.value = Object.fromEntries(
      types.map((type) => [type, configs?.[type]?.priorities || []]),
    );
    finalStatusIds.value = {
      requirement: (configs?.settings?.requirement_final_status_ids || []).map(
        Number,
      ),
      defect: (configs?.settings?.defect_final_status_ids || []).map(Number),
    };
    fieldConfigs.value = Object.fromEntries(
      types.map((type) => [type, configs?.[type]?.field_configs || []]),
    );
    workspaceMetadata.value = configs;
    customFields.value = Object.fromEntries(
      types.map((type) => [
        type,
        (configs?.[type]?.custom_fields || []).filter(
          (item) => Number(item.is_enabled) === 1,
        ),
      ]),
    );
    displaySettings.value = Object.fromEntries(
      types.map((type, index) => [type, display[index]]),
    );
  }

  function isFinalStatus(item) {
    return finalStatusIds.value[item.type]?.includes(Number(item.status_id));
  }

  function detailUpdateEvent(changedFields) {
    const item = detail.value;
    // The detail endpoint does not return the workflow sort orders, but the
    // cross-project lists sort by them in memory, so patch them from metadata.
    const status = (statuses.value[item.type] || []).find(
      (option) => Number(option.id) === Number(item.status_id),
    );
    const priority = (priorities.value[item.type] || []).find(
      (option) => Number(option.id) === Number(item.priority),
    );
    return {
      action: "update",
      id: item.id,
      workspaceId: Number(item.workspace_id || props.wsId),
      changedFields,
      item: {
        ...item,
        is_done: isFinalStatus(item),
        is_completed: (
          workspaceMetadata.value?.settings?.[
            `${item.type}_completed_status_ids`
          ] || []
        )
          .map(Number)
          .includes(Number(item.status_id)),
        status_sort_order: Number(status?.sort_order) || 0,
        priority_sort_order: Number(priority?.sort_order) || 0,
      },
    };
  }

  async function loadCurrentDetail(context = detailLoadContext()) {
    if (!context.isCurrent()) return null;
    const item = await getRequirement(context.wsId, context.id);
    if (!context.isCurrent()) return null;
    detail.value = item;
    emit("loaded", detail.value);
    // Opening the detail is what counts as a visit. This drawer is the only detail view in the
    // app, so tracking here covers the board, my work, team work, the agent page, the standalone
    // detail page and the parent/child/relation jumps inside the drawer. Fire and forget: a
    // failed record must never delay or break rendering.
    recordRequirementVisit(context.wsId, context.id).catch(() => {});
    // Keep local drafts while the user is editing the title or description
    if (!editingTitle.value && !descriptionEditing.value) {
      titleDraft.value = detail.value.title || "";
      descriptionDraft.value = detail.value.description || "";
      descriptionModeDraft.value =
        detail.value.description_mode === "markdown" ? "markdown" : "rich-text";
    }
    // The active tab is owned by the open watcher and the deep link, never by a data refresh:
    // editing a property reloads the detail, and resetting the tab here threw the user from the
    // pane they were reading (e.g. 时效) back to 描述.
    detailReloadKey.value += 1;
    customValues.value = Object.fromEntries(
      (customFields.value[detail.value.type] || []).map((field) => [
        field.id,
        parseCustomFieldValue(
          field,
          detail.value.custom_fields?.[field.id] ?? "",
        ),
      ]),
    );
    return item;
  }

  async function loadDetail(context) {
    if (!(await loadCurrentDetail(context)) || !context.isCurrent()) return;
    // 深链入口（列表时效单元格 / SLA 通知）要求定位到指定页签；常规入口回落「描述」。
    detailTab.value = props.initialTab || "desc";
    const [relations, fileList, categories] = await Promise.all([
      listRequirements(context.wsId, {
        page: 1,
        size: 100,
        parent_id: context.id,
      }).then((result) => result?.list || []),
      listRequirementAttachments(context.wsId, context.id),
      detail.value.type === "requirement"
        ? listRequirementCategories(context.wsId)
            .then((result) => result?.list || [])
            .catch(() => null)
        : null,
    ]);
    if (!context.isCurrent()) return;
    childRequirements.value = relations.filter(
      (item) => item.type === "requirement",
    );
    relatedDefects.value = relations.filter((item) => item.type === "defect");
    setAttachments(fileList);
    requirementCategories.value = categories;
  }

  function resetHistories() {
    historyRequestSequence += 1;
    histories.value = [];
    historyLoading.value = false;
    historyLoaded.value = false;
  }

  async function handleDetailTabChange(key) {
    if (
      !["history", "processing"].includes(key) ||
      historyLoaded.value ||
      historyLoading.value
    )
      return;
    const context = detailLoadContext();
    if (!context.isCurrent()) return;
    const requestSequence = ++historyRequestSequence;
    historyLoading.value = true;
    try {
      const result = await listRequirementHistories(context.wsId, context.id);
      if (!context.isCurrent() || requestSequence !== historyRequestSequence)
        return;
      histories.value = result || [];
      historyLoaded.value = true;
    } finally {
      if (context.isCurrent() && requestSequence === historyRequestSequence) {
        historyLoading.value = false;
      }
    }
  }

  watch(
    () => [props.open, props.wsId, props.requirementId],
    async ([open, wsId, id], _, onCleanup) => {
      detailLoadSequence += 1;
      const context = detailLoadContext();
      onCleanup(() => {
        detailLoadSequence += 1;
      });
      // keepAlive：同一个工作项的收起/展开只切换可见性。收起时保留 detail 与当前页签，
      // 展开时也不再重载——否则每次「任务详情」都重新拉一遍并跳回默认页签，像又进了一次详情。
      if (
        props.keepAlive &&
        detail.value &&
        loadedDetailKey === `${wsId}:${id}`
      ) {
        loading.value = false;
        if (!open) return;
        editingTitle.value = false;
        editingParent.value = false;
        editingProperty.value = "";
        descriptionEditing.value = false;
        return;
      }
      editingTitle.value = false;
      editingParent.value = false;
      editingProperty.value = "";
      descriptionEditing.value = false;
      transitionOpen.value = false;
      workspacePermissions.value = [];
      requirementCategories.value = null;
      detailTab.value = props.initialTab || "desc";
      resetHistories();
      imageFileIds.value = [];
      if (!open || !wsId || !id) {
        detail.value = null;
        loading.value = false;
        propertiesCollapsed.value = false;
        return;
      }
      // 切换到当前工作项的草稿会话前先落盘旧会话，避免最后 350ms 输入丢失或串写。
      loadedDetailKey = "";
      loading.value = true;
      try {
        await draft.setTarget(
          String(id),
          { wsId: String(wsId) },
          context.isCurrent,
        );
        if (!context.isCurrent()) return;
        await loadOptions(context);
        if (!context.isCurrent()) return;
        await loadDetail(context);
        if (!context.isCurrent()) return;
        await consumeWorkLogEditorRequest();
        if (!context.isCurrent()) return;
        await draft.setTarget(
          String(id),
          { wsId: String(wsId), type: detail.value?.type || "" },
          context.isCurrent,
        );
        if (!context.isCurrent()) return;
        await draft.list();
        if (context.isCurrent()) loadedDetailKey = `${wsId}:${id}`;
      } catch {
        if (context.isCurrent()) detail.value = null;
      } finally {
        if (context.isCurrent()) loading.value = false;
      }
    },
    { immediate: true },
  );

  watch(
    () => props.workLogEditorToken,
    () => void consumeWorkLogEditorRequest(),
  );

  function plannedDateValue(value) {
    const timestamp = Number(value);
    return timestamp > 0 ? dayjs.unix(timestamp) : null;
  }

  async function refreshAfterUpdate(context, changedFields) {
    const item = await loadCurrentDetail(context);
    if (!item || !context.isCurrent()) {
      emit("updated", {
        action: "refresh",
        id: context.id,
        workspaceId: Number(context.wsId),
      });
      return false;
    }
    resetHistories();
    emit("updated", detailUpdateEvent(changedFields));
    return true;
  }

  async function updateField(field, value, context = detailLoadContext()) {
    if (!context.isCurrent()) return false;
    const updates = { [field]: value };
    await updateRequirement(context.wsId, context.id, updates);
    return refreshAfterUpdate(context, Object.keys(updates));
  }
  function openStatusEditor(editingKey = "status") {
    if (!fieldEditable(detail.value?.type, "status")) return;
    if (statusTransitionOn.value) {
      statusTransitionContext = detailLoadContext();
      transitionOpen.value = true;
      return;
    }
    editingProperty.value = editingKey;
  }

  async function handleStatusTransitionSuccess(patch, changedFields) {
    const context = statusTransitionContext || detailLoadContext();
    await refreshAfterUpdate(context, changedFields || Object.keys(patch));
  }

  const canAssignCategory = computed(
    () =>
      requirementCategories.value !== null &&
      detail.value?.type === "requirement" &&
      Number(detail.value.parent_id) === 0 &&
      fieldEnabled(detail.value.type, "category") &&
      fieldEditable(detail.value.type, "category"),
  );

  async function saveCategory(categoryId) {
    const context = detailLoadContext();
    if (
      !context.isCurrent() ||
      !canAssignCategory.value ||
      categorySaving.value
    )
      return;
    if (categoryId === Number(detail.value.category_id)) return;
    categorySaving.value = true;
    try {
      await assignRequirementCategory(context.wsId, context.id, categoryId);
      await refreshAfterUpdate(context, ["category_id"]);
    } catch {
      // The request interceptor shows the error and the current category stays visible.
    } finally {
      categorySaving.value = false;
    }
  }

  async function saveProperty(field, value) {
    editingProperty.value = "";
    await updateField(field, value);
  }

  function formatWorkHours(value) {
    const numeric = Number(value) || 0;
    const text = numeric
      .toFixed(2)
      .replace(/\.00$/, "")
      .replace(/(\.\d)0$/, "$1");
    return `${text}${t("requirement_workHoursUnit")}`;
  }

  function formatWorkHoursValue(value) {
    const numeric = Number(value) || 0;
    return numeric
      .toFixed(2)
      .replace(/\.00$/, "")
      .replace(/(\.\d)0$/, "$1");
  }

  function startEstimatedWorkHoursEdit() {
    if (!fieldEditable(detail.value?.type, "estimated_work_hours")) return;
    estimatedWorkHoursDraft.value =
      Number(detail.value?.estimated_work_hours) || 0;
    editingProperty.value = "estimated_work_hours";
  }

  async function saveEstimatedWorkHours() {
    if (
      editingProperty.value !== "estimated_work_hours" ||
      estimatedWorkHoursSaving.value
    )
      return;
    const value = Number(estimatedWorkHoursDraft.value);
    if (
      !Number.isFinite(value) ||
      value < 0 ||
      value > 99999999.99 ||
      Math.abs(value * 100 - Math.round(value * 100)) > 0.000001
    ) {
      message.warning(t("requirement_workHoursInvalid"));
      return;
    }
    estimatedWorkHoursSaving.value = true;
    editingProperty.value = "";
    try {
      await updateField("estimated_work_hours", value);
    } finally {
      estimatedWorkHoursSaving.value = false;
    }
  }

  async function openWorkLogCreate() {
    if (detail.value?.type !== "requirement") return;
    if (!canEdit.value) return;
    detailTab.value = "expense";
    await nextTick();
    workLogsRef.value?.openCreate?.();
  }

  async function consumeWorkLogEditorRequest() {
    const token = Number(props.workLogEditorToken) || 0;
    const targetId = String(props.workLogEditorRequirementId || "");
    if (
      !token ||
      token === handledWorkLogEditorToken.value ||
      !detail.value ||
      targetId !== String(props.requirementId) ||
      targetId !== String(detail.value.id)
    )
      return;
    handledWorkLogEditorToken.value = token;
    await openWorkLogCreate();
  }

  function handleWorkLogSummary(summary) {
    if (!detail.value) return;
    const changedFields = [
      "completed_work_hours",
      "remaining_work_hours",
      "overtime_work_hours",
      "work_progress",
      "work_log_count",
    ];
    changedFields.forEach((field) => {
      if (summary[field] !== undefined) detail.value[field] = summary[field];
    });
    emit("updated", detailUpdateEvent(changedFields));
  }
  async function saveAssignees(value) {
    // Save on every change triggered by AssigneeSelect. Do NOT clear
    // editingProperty here, otherwise the component unmounts and the
    // dropdown disappears mid-save. The dropdown stays open until the
    // user clicks outside (handled by @cancel). Recent-contact ordering
    // is deferred to dropdown close inside AssigneeSelect, so the list
    // order does not shift during the current open session.
    await updateField("assignee_ids", value);
  }
  async function updateParent(item) {
    const context = detailLoadContext();
    if (
      (await updateField("parent_id", item?.id || 0, context)) &&
      context.isCurrent()
    ) {
      editingParent.value = false;
    }
  }
  async function saveTitle() {
    editingTitle.value = false;
    // 正文编辑会话内，标题由底部「保存」统一提交
    if (descriptionEditing.value) return;
    const title = titleDraft.value.trim();
    if (title && title !== detail.value?.title) {
      // 先落盘未保存内容，避免 350ms 定时器在 markPosted 之后又把记录写回未提交
      const context = detailLoadContext();
      await draft.flush();
      if (await updateField("title", title, context)) {
        if (context.isCurrent()) await draft.markPosted();
      }
    }
  }

  // 仅在标题或正文编辑态中自动保存；进入编辑态本身不算用户输入，
  // 否则取消后再次编辑会用服务端原文覆盖已缓存的草稿。
  watch([titleDraft, descriptionDraft], ([t, d]) => {
    if (!editingTitle.value && !descriptionEditing.value) return;
    draft.schedule({ title: t, html: d, mode: descriptionModeDraft.value });
  });

  async function openDraftModal() {
    draftRecords.value = (await draft.list()) || [];
    draftModalOpen.value = true;
  }

  function applyDraftRestore(payload) {
    // 只覆盖标题、正文和编辑器模式，不请求后端；随后由 watch 自动保存到当前草稿
    titleDraft.value = payload.title;
    descriptionDraft.value = payload.html;
    if (payload.mode) descriptionModeDraft.value = payload.mode;
    draftRestoreRevision.value += 1;
  }

  function handleDescriptionCancel() {
    // 先退出编辑态再恢复服务端值；本地草稿保持未提交，可再次找回
    descriptionEditing.value = false;
    editingTitle.value = false;
    titleDraft.value = detail.value?.title || "";
    descriptionDraft.value = detail.value?.description || "";
  }
  async function saveDescription() {
    if (savingDescription.value) return;
    const nextTitle = titleDraft.value.trim();
    if (!nextTitle) {
      message.warning(t("requirement_titleRequired"));
      return;
    }
    const context = detailLoadContext();
    if (!context.isCurrent()) return;
    savingDescription.value = true;
    try {
      // 一次请求提交发生变化的标题、正文和描述模式
      const updates = {};
      if (nextTitle !== detail.value?.title) updates.title = nextTitle;
      if (descriptionDraft.value !== detail.value?.description)
        updates.description = descriptionDraft.value;
      if (descriptionModeDraft.value !== detail.value?.description_mode)
        updates.description_mode = descriptionModeDraft.value;
      // 有先行上传的图片/文件/视频时，即使其他字段未变化也要提交，供后端回绑台账记录
      if (imageFileIds.value.length)
        updates.image_file_ids = imageFileIds.value;
      if (!Object.keys(updates).length) {
        captureEditSnapshot();
        descriptionEditorRef.value?.markSaved?.();
        if (descriptionEditing.value)
          showWorkItemUpdateSuccess(t("common_saved"));
        return;
      }
      // 先落盘未保存内容，避免 350ms 定时器在 markPosted 之后又把记录写回未提交
      await draft.flush();
      if (!context.isCurrent()) return;
      await updateRequirement(context.wsId, context.id, updates);
      if (!context.isCurrent()) {
        await refreshAfterUpdate(context, Object.keys(updates));
        return;
      }
      imageFileIds.value = [];
      showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
      await draft.markPosted();
      const changedFields = Object.keys(updates).filter(
        (field) => field !== "image_file_ids",
      );
      if (!(await refreshAfterUpdate(context, changedFields))) return;
      if (!context.isCurrent()) return;
      captureEditSnapshot();
      descriptionEditorRef.value?.markSaved?.();
    } catch {
      /* handled by interceptor */
    } finally {
      savingDescription.value = false;
    }
  }
  async function saveChangedCustomField(field) {
    const context = detailLoadContext();
    const saved = await saveCustomField(field);
    if (
      context.isCurrent() &&
      saved &&
      !isMultiValueFieldType(field.field_type)
    ) {
      editingProperty.value = "";
    }
  }

  async function saveAndCloseCustomField(field) {
    if (editingProperty.value !== `custom_${field.id}`) return;
    const context = detailLoadContext();
    const saved = await saveCustomField(field);
    if (context.isCurrent() && saved) editingProperty.value = "";
  }

  async function closeCustomFieldEditor(field) {
    const context = detailLoadContext();
    if (editingProperty.value !== `custom_${field.id}`) return;
    if (
      isMultiValueFieldType(field.field_type) ||
      field.field_type === CUSTOM_FIELD_TYPE.CASCADER_SINGLE
    ) {
      const saved = await saveCustomField(field);
      if (!saved) return;
    }
    if (context.isCurrent()) editingProperty.value = "";
  }

  async function saveCustomField(field) {
    if (!canEdit.value) return false;
    if (!isValidPhoneFieldValue(field, customValues.value[field.id])) {
      message.warning(t("settings_fieldPhoneInvalid"));
      return false;
    }
    const context = detailLoadContext();
    if (!context.isCurrent()) return false;
    try {
      await updateRequirement(context.wsId, context.id, {
        custom_fields: {
          [field.id]: serializeCustomFieldValue(
            field,
            customValues.value[field.id],
          ),
        },
      });
      if (!(await refreshAfterUpdate(context, ["custom_fields"]))) return false;
      showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
      return true;
    } catch {
      return false;
    }
  }

  return {
    ...toRefs(props),
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
    detailContainer,
    viewportWidth,
    drawerWidth,
    detailContainerProps,
    isNarrowLayout,
    loading,
    detailLeftRef,
    confirmLeaveEditing,
    requestClose,
    startDrawerResize,
    resizeDrawerByKeyboard,
    detail,
    detailTab,
    detailReloadKey,
    propertiesCollapsed,
    editingTitle,
    editingParent,
    editingProperty,
    workLogsRef,
    estimatedWorkHoursDraft,
    descriptionEditing,
    descriptionEditorRef,
    titleDraft,
    descriptionDraft,
    imageFileIds,
    titleDisplay,
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
    navigateRelative,
    openFullscreenEditor,
    openInNewWindow,
    openParentInNewWindow,
    handleDetailAction,
    closeActionModal,
    handleActionSuccess,
    handleCopyAction,
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
    saveTitle,
    openDraftModal,
    applyDraftRestore,
    handleDescriptionCancel,
    saveDescription,
    saveChangedCustomField,
    saveAndCloseCustomField,
    closeCustomFieldEditor,
  };
}
