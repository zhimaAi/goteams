import {
  computed,
  ref,
  watch,
  watchEffect,
  onMounted,
  nextTick,
  onBeforeUnmount,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { useLocalDraft } from "@/composables/useLocalDraft";
import { useCopyWorkItemSource } from "@/composables/useCopyWorkItemSource";
import { useCreateWorkItemOptions } from "@/composables/useCreateWorkItemOptions";
import { useCreateWorkItemSubmit } from "@/composables/useCreateWorkItemSubmit";
import {
  listWorkspaces,
  listWorkItemActionTargets,
  getRequirement,
} from "@/api/workspace";
import { useAuthStore } from "@/stores/auth";
import { createDraftSessionId } from "@/utils/localDraftRecord";
import { isDraftAutoRestoreEnabled } from "@/utils/localDraftPreference";
import { customTemplateFieldKey } from "@/utils/workItemTemplate";
import { serializeCustomFieldValue } from "@/utils/customField";
import { useCreateWorkItemTemplate } from "@/composables/useCreateWorkItemTemplate";
import { useWorkspaceApplications } from "@/composables/useWorkspaceApplications";
import { workItemRoute } from "@/utils/workItemRoutes";
export function useWorkItemCreate() {
  const { t: $t } = useI18n();
  const route = useRoute();
  const router = useRouter();
  const surface = route.meta.workItemSurface;
  const authStore = useAuthStore();
  // The entry is fixed at /workspace/:wsId/create (from the board or My Work); wsId is taken from the route.
  const routeWsId = route.params.wsId;

  // Form state (selectedWs is the currently selected workspace id, a string).
  const selectedWs = ref(routeWsId ? String(routeWsId) : "");
  // The copy entry passes the source type in the query so `type` already holds the right value here:
  // assigning it later would trigger the workspace/type watcher and wipe the prefilled properties.
  const type = ref(route.query.type === "defect" ? "defect" : "requirement");
  // Copy context: copyFrom is the id of the work item being copied; its title, description and
  // properties prefill the form, and the user may still change the type and workspace.
  const copyFromId = Number(route.query.copyFrom) || 0;
  const detailAction = String(route.query.detailAction || "");

  // 每次进入创建页生成独立会话，对应一条独立创建草稿
  const sessionId = createDraftSessionId();
  // 初始化回填（如规划页带入标题）不视为用户输入，不触发草稿保存
  let draftSuppress = false;
  const draftModalOpen = ref(false);
  const draftRecords = ref([]);
  const draftRestoreRevision = ref(0);
  const draft = useLocalDraft({
    kind: "create",
    userId: authStore.userInfo?.id || "",
    wsId: selectedWs.value,
    type: type.value,
    targetId: sessionId,
    onStorageError: () => {
      message.warning($t("draft_storage_failed"));
    },
  });
  const requestedStatusId = ref(Number(route.query.statusId) || 0);
  // 记录上次创建工作项时选择的工作空间，供“我的工作”页默认跳转使用。
  const LAST_CREATE_WS_KEY = "goteams-last-create-ws";
  watch(selectedWs, (val) => {
    if (val) localStorage.setItem(LAST_CREATE_WS_KEY, val);
  });
  // Planning context: when planningId is provided, the created work item auto-relates to that planning;
  // "Save and continue" keeps the context so multiple items can be entered in a row.
  const planningId = ref(Number(route.query.planningId) || 0);
  function normalizedCategoryQuery(value) {
    if (String(value) === "uncategorized" || String(value) === "0") {
      return "uncategorized";
    }
    const id = Number(value);
    return Number.isInteger(id) && id > 0 ? String(id) : "";
  }
  const returnCategoryQuery = ref(
    type.value === "requirement"
      ? normalizedCategoryQuery(route.query.categoryId)
      : "",
  );
  const categoryId = ref(Number(returnCategoryQuery.value) || 0);
  const title = ref("");
  const titleTouched = ref(false);
  const titleInvalid = computed(
    () => titleTouched.value && !title.value.trim(),
  );
  const description = ref("");
  // Keep the legacy image_file_ids API field for all rich-text asset ledger IDs.
  const imageFileIds = ref([]);
  const descriptionMode = ref("");
  const status = ref(undefined);
  const priority = ref(undefined);
  const versionId = ref(undefined);
  const assigneeIds = ref([]);
  const plannedStartDate = ref(null);
  const plannedEndDate = ref(null);
  const estimatedWorkHours = ref(0);
  const parentId = ref(undefined);
  const creating = ref(false);

  watch([title, description], ([t, d]) => {
    if (draftSuppress) return;
    draft.schedule({ title: t, html: d, mode: descriptionMode.value });
  });

  async function openDraftModal() {
    draftRecords.value = (await draft.list()) || [];
    draftModalOpen.value = true;
  }

  async function applyDraftRestore(payload) {
    // TAPD 的空白新建会话不会保存首次恢复回填；已有当前草稿时仍按正常编辑保存。
    const suppressRestoreSave = !draft.lastSavedAt.value;
    if (suppressRestoreSave) draftSuppress = true;
    title.value = payload.title;
    description.value = payload.html;
    if (payload.mode) descriptionMode.value = payload.mode;
    draftRestoreRevision.value += 1;
    await nextTick();
    if (suppressRestoreSave) draftSuppress = false;
  }
  const permissionScopeReady = computed(
    () => String(authStore.permissionWorkspaceId) === String(selectedWs.value),
  );
  const canCreateRequirement = computed(
    () =>
      permissionScopeReady.value &&
      authStore.hasPermission("requirement:create"),
  );
  // The defect type is an app-market module of the selected project: with the app off the
  // type switch must not offer it. The watchEffect below moves an active defect selection
  // back to requirement.
  const { defectAppEnabled, applications: targetApplications } =
    useWorkspaceApplications(selectedWs);
  const canCreateDefect = computed(
    () =>
      permissionScopeReady.value &&
      defectAppEnabled.value &&
      authStore.hasPermission("defect:create"),
  );
  const canCreateCurrentType = computed(
    () =>
      permissionScopeReady.value &&
      authStore.hasPermission(
        `${type.value === "defect" ? "defect" : "requirement"}:create`,
      ),
  );

  const workspaces = ref([]);
  const verOptions = ref([]);
  const selectableVersionOptions = computed(() =>
    verOptions.value.filter((item) => !item.is_ended),
  );
  const memberOpts = ref([]);
  const statuses = ref([]);
  const priorityOptions = ref([]);
  const fieldConfigs = ref([]);
  const categoryOptions = ref([]);
  const customFields = ref([]);
  const customFieldMembers = ref([]);
  const customValues = ref({});

  const builtinPropertyKeys = [
    "status",
    "version",
    "priority",
    "category",
    "planned_start_date",
    "planned_end_date",
    "assignee",
    "estimated_work_hours",
    "work_progress",
    "completed_work_hours",
    "remaining_work_hours",
    "overtime_work_hours",
  ];

  const requirementWorkHourKeys = new Set([
    "estimated_work_hours",
    "work_progress",
    "completed_work_hours",
    "remaining_work_hours",
    "overtime_work_hours",
  ]);

  function builtinPropertyLabel(key) {
    const labels = {
      status: $t("workspace_status"),
      priority: $t("workspace_priority"),
      version: $t("requirement_filterVersion"),
      category: $t("workspace_field_category"),
      assignee: $t("workspace_assignee"),
      planned_start_date: $t("requirement_planStartShort"),
      planned_end_date: $t("requirement_planEndShort"),
      estimated_work_hours: $t("requirement_estimatedWorkHours"),
      work_progress: $t("requirement_workProgress"),
      completed_work_hours: $t("requirement_completedWorkHours"),
      remaining_work_hours: $t("requirement_remainingWorkHours"),
      overtime_work_hours: $t("requirement_overtimeWorkHours"),
    };
    return labels[key] || key;
  }

  function fieldEnabled(key) {
    const config = fieldConfigs.value.find((item) => item.key === key);
    if (requirementWorkHourKeys.has(key) && !config) return false;
    return !config || Number(config.enabled) === 1;
  }

  function fieldEditable(key) {
    const config = fieldConfigs.value.find((item) => item.key === key);
    return !config || Number(config.editable) === 1;
  }

  const { applyCopySource, copiedOptionId, copiedDate, copiedCustomValue } =
    useCopyWorkItemSource({
      routeWsId,
      copyFromId,
      copySuffix: () => $t("requirement_copySuffix"),
      type,
      title,
      description,
      descriptionMode,
      parentId,
      categoryId,
      status,
      priority,
      versionId,
      assigneeIds,
      plannedStartDate,
      plannedEndDate,
      estimatedWorkHours,
      statuses,
      priorities: priorityOptions,
      versions: selectableVersionOptions,
      members: memberOpts,
      customFields,
      customValues,
      fieldEnabled,
      fieldEditable,
    });

  const {
    templates,
    templateId,
    showTemplates,
    selectedTemplate,
    isTemplateRequired,
    isTemplateFieldVisible,
    isTemplateFieldFilled,
    applyTemplate,
    loadTemplates,
    onTemplateIdChange,
  } = useCreateWorkItemTemplate({
    copyFromId,
    type,
    selectedWs,
    title,
    description,
    descriptionMode,
    status,
    priority,
    versionId,
    categoryId,
    categoryOptions,
    assigneeIds,
    plannedStartDate,
    plannedEndDate,
    estimatedWorkHours,
    statuses,
    priorityOptions,
    verOptions: selectableVersionOptions,
    memberOpts,
    customFields,
    customValues,
    fieldEnabled,
    fieldEditable,
    copiedOptionId,
    copiedDate,
    copiedCustomValue,
    setDraftSuppress: (value) => {
      draftSuppress = value;
    },
    bumpEditorRevision: () => {
      draftRestoreRevision.value += 1;
    },
    keepTitleOnApply: () => !!route.query.title,
  });

  const propertyItems = computed(() => {
    const builtins = builtinPropertyKeys
      .filter(
        (key) =>
          (key !== "category" || type.value === "requirement") &&
          (!requirementWorkHourKeys.has(key) || type.value === "requirement"),
      )
      .map((key, index) => {
        const config = fieldConfigs.value.find((item) => item.key === key);
        return {
          key,
          kind: "builtin",
          label: config?.label || builtinPropertyLabel(key),
          editable: fieldEditable(key),
          sortOrder: Number(config?.sort_order ?? index),
        };
      });
    const customs = customFields.value.map((field) => ({
      key: customTemplateFieldKey(field.id),
      kind: "custom",
      label: field.name,
      editable: true,
      sortOrder: Number(field.sort_order ?? 100),
      field,
    }));
    const templateOrder = new Map(
      (selectedTemplate.value?.fields || []).map((field, index) => [
        field.field_key,
        index,
      ]),
    );
    return [...builtins, ...customs]
      .filter((item) => isTemplateFieldVisible(item.key))
      .sort((left, right) => {
        const leftRank = templateOrder.has(left.key)
          ? templateOrder.get(left.key)
          : 10000 + left.sortOrder;
        const rightRank = templateOrder.has(right.key)
          ? templateOrder.get(right.key)
          : 10000 + right.sortOrder;
        return leftRank - rightRank;
      })
      .map((item) => ({
        ...item,
        required: isTemplateRequired(item.key),
      }));
  });

  function customFieldPayload() {
    return Object.fromEntries(
      customFields.value
        .filter((field) =>
          isTemplateFieldVisible(customTemplateFieldKey(field.id)),
        )
        .map((field) => [
          field.id,
          serializeCustomFieldValue(field, customValues.value[field.id]),
        ]),
    );
  }

  const { loadOptions } = useCreateWorkItemOptions({
    t: $t,
    authStore,
    type,
    selectedWs,
    returnCategoryQuery,
    categoryId,
    fieldEnabled,
    requestedStatusId: requestedStatusId.value,
    status,
    priority,
    versions: verOptions,
    members: memberOpts,
    statuses,
    priorities: priorityOptions,
    fieldConfigs,
    categories: categoryOptions,
    customFields,
    customFieldMembers,
    customValues,
    loadTemplates,
  });

  onMounted(async () => {
    await loadWorkspaceOptions();
    await loadOptions();
    // 初始化提示条状态：当前工作空间/类型下是否有未提交创建草稿
    void draft.list();
    // 上次异常关闭（未点取消）遗留的草稿自动回填一次；复制新建和规划页带入标题以来源内容为准
    if (isDraftAutoRestoreEnabled() && !route.query.title && !copyFromId) {
      const restored = await draft.takeAutoRestorable();
      if (restored) {
        await applyDraftRestore(restored);
        message.info($t("draft_auto_restored"));
      }
    }
    if (route.query.parentId) {
      parentId.value = Number(route.query.parentId);
      if (type.value === "requirement") {
        try {
          const parent = await getRequirement(selectedWs.value, parentId.value);
          categoryId.value = Number(parent?.category_id) || 0;
        } catch {
          /* handled by interceptor */
        }
      }
    }
    // The title brought from the planning page and the copy prefill are initialization backfill,
    // not user input, so they do not trigger draft save.
    if (route.query.title || copyFromId) {
      draftSuppress = true;
      if (route.query.title) title.value = String(route.query.title);
      if (copyFromId) await applyCopySource();
      await nextTick();
      draftSuppress = false;
    }
  });

  async function loadWorkspaceOptions() {
    try {
      const items = detailAction
        ? await listWorkItemActionTargets({
            sourceWsId: routeWsId,
            type: type.value,
            action: "convert",
          })
        : await listWorkspaces();
      workspaces.value = (items || []).map((workspace) => ({
        id: String(workspace.id),
        name: workspace.name,
        color: workspace.color || "#0d68ff",
        icon: workspace.icon || "",
      }));
      if (
        detailAction &&
        !workspaces.value.some((item) => item.id === String(selectedWs.value))
      ) {
        selectedWs.value = workspaces.value[0]?.id || "";
      }
    } catch {
      workspaces.value = [];
    }
  }

  watchEffect(() => {
    if (
      type.value === "requirement" &&
      !canCreateRequirement.value &&
      canCreateDefect.value
    ) {
      type.value = "defect";
    }
    if (
      type.value === "defect" &&
      !canCreateDefect.value &&
      canCreateRequirement.value
    ) {
      type.value = "requirement";
    }
  });

  // Refresh related data when switching workspace or type, to avoid reusing statuses and priority IDs of other types.
  watch([selectedWs, type], async ([value]) => {
    if (!value) return;
    // retarget 内部先落盘，再把当前草稿迁移到新范围（同 key 原地更新）
    await draft.retarget({ wsId: selectedWs.value, type: type.value });
    // 新范围下重新计算提示条状态
    await draft.list();
    parentId.value = undefined;
    returnCategoryQuery.value = "";
    categoryId.value = 0;
    versionId.value = undefined;
    assigneeIds.value = [];
    status.value = undefined;
    priority.value = undefined;
    estimatedWorkHours.value = 0;
    await authStore.fetchProjectPermissions(value);
    await loadOptions();
  });

  watch(type, () => {
    if (detailAction) void loadWorkspaceOptions();
  });

  watch(templateId, onTemplateIdChange);

  const { submitCreate } = useCreateWorkItemSubmit({
    t: $t,
    router,
    detailAction,
    canCreateCurrentType,
    selectedWs,
    type,
    title,
    titleTouched,
    description,
    imageFileIds,
    descriptionMode,
    status,
    priority,
    versionId,
    assigneeIds,
    plannedStartDate,
    plannedEndDate,
    estimatedWorkHours,
    parentId,
    categoryId,
    planningId,
    customFields,
    customValues,
    propertyItems,
    returnCategoryQuery,
    selectedTemplate,
    isTemplateFieldFilled,
    isTemplateFieldVisible,
    customFieldPayload,
    applyTemplate,
    creating,
    draft,
    goBoard,
  });

  function goBoard() {
    // 用户主动离开视为正常关闭：落盘当前内容，但下次新建不再自动恢复
    void draft.skipAutoRestore();
    router.push({
      ...workItemRoute(
        surface === "issue" &&
          (String(selectedWs.value) === String(route.params.wsId) ||
            targetApplications.value.includes("issue"))
          ? surface
          : undefined,
        "list",
        selectedWs.value,
      ),
      // Entered from the planning page, so return to the planning tab after completion.
      query: {
        tab: planningId.value ? "planning" : type.value,
        ...(returnCategoryQuery.value && !planningId.value
          ? { categoryId: returnCategoryQuery.value }
          : {}),
      },
    });
  }

  onBeforeUnmount(() => {
    void draft.flush();
  });

  return {
    selectedWs,
    type,
    draftModalOpen,
    draftRecords,
    draftRestoreRevision,
    draft,
    categoryId,
    title,
    titleTouched,
    titleInvalid,
    description,
    imageFileIds,
    descriptionMode,
    status,
    priority,
    versionId,
    assigneeIds,
    plannedStartDate,
    plannedEndDate,
    estimatedWorkHours,
    parentId,
    creating,
    openDraftModal,
    applyDraftRestore,
    canCreateRequirement,
    canCreateDefect,
    canCreateCurrentType,
    workspaces,
    selectableVersionOptions,
    memberOpts,
    statuses,
    priorityOptions,
    categoryOptions,
    customFieldMembers,
    customValues,
    templates,
    templateId,
    showTemplates,
    propertyItems,
    submitCreate,
    goBoard,
  };
}
