import { computed, nextTick, ref } from "vue";
import { listWorkItemTemplates } from "@/api/workspace";
import {
  isTemplateValueEmpty,
  createTemplateFieldIsVisible,
  parseCustomTemplateFieldId,
  resolveTemplateDateDefault,
  templateAssigneeIds,
} from "@/utils/workItemTemplate";

export function useCreateWorkItemTemplate(ctx) {
  const templates = ref([]);
  const templateEnabled = ref(false);
  const templateId = ref(undefined);
  const skipInitialApply = ref(!!ctx.copyFromId);
  let skipSelectionApply = false;
  let baseline = null;
  let loadSequence = 0;
  const showTemplates = computed(
    () => templateEnabled.value && templates.value.length > 0,
  );
  const selectedTemplate = computed(() => {
    if (!templateEnabled.value) return undefined;
    return templates.value.find(
      (item) => Number(item.id) === Number(templateId.value),
    );
  });

  function templateFieldValue(key) {
    if (key === "status") return ctx.status.value;
    if (key === "priority") return ctx.priority.value;
    if (key === "version") return ctx.versionId.value;
    if (key === "category") return ctx.categoryId.value;
    if (key === "assignee") return ctx.assigneeIds.value;
    const customId = parseCustomTemplateFieldId(key);
    return customId ? ctx.customValues.value[customId] : undefined;
  }

  function isWorkspaceRequired(key) {
    const customId = parseCustomTemplateFieldId(key);
    return ctx.customFields.value.some(
      (field) =>
        Number(field.id) === customId &&
        Number(field.is_enabled) === 1 &&
        Number(field.is_required) === 1,
    );
  }

  function isTemplateFieldVisible(key) {
    const customId = parseCustomTemplateFieldId(key);
    const custom = ctx.customFields.value.find(
      (field) => Number(field.id) === customId,
    );
    if (!selectedTemplate.value) {
      return customId
        ? Number(custom?.is_enabled) === 1
        : ctx.fieldEnabled(key);
    }
    return createTemplateFieldIsVisible(
      selectedTemplate.value.fields,
      key,
      templateFieldValue,
      isWorkspaceRequired,
    );
  }

  function isTemplateRequired(key) {
    if (!isTemplateFieldVisible(key)) return false;
    if (isWorkspaceRequired(key)) return true;
    return !!(selectedTemplate.value?.fields || []).find(
      (field) => field.field_key === key && field.required,
    );
  }

  function isTemplateFieldFilled(key) {
    if (key === "assignee") return ctx.assigneeIds.value.length > 0;
    if (key === "planned_start_date") return !!ctx.plannedStartDate.value;
    if (key === "planned_end_date") return !!ctx.plannedEndDate.value;
    if (key === "priority")
      return ctx.priority.value != null && ctx.priority.value !== "";
    if (key === "status") return !!ctx.status.value;
    if (key === "version") return !!ctx.versionId.value;
    if (key === "category") return ctx.categoryId.value != null;
    if (key === "estimated_work_hours")
      return Number(ctx.estimatedWorkHours.value) >= 0;
    const customId = parseCustomTemplateFieldId(key);
    if (!customId) return true;
    const value = ctx.customValues.value[customId];
    if (Array.isArray(value)) return value.length > 0;
    if (value?.format) return true;
    return value != null && value !== "";
  }

  function applyTemplateField(field) {
    const key = field.field_key;
    let value = field.default_value;
    if (key === "planned_start_date" || key === "planned_end_date") {
      value = resolveTemplateDateDefault(value, true);
    }
    const customId = parseCustomTemplateFieldId(key);
    const customField = ctx.customFields.value.find(
      (item) => Number(item.id) === customId,
    );
    if (customField?.field_type === "date") {
      value = resolveTemplateDateDefault(value, false);
    }
    if (isTemplateValueEmpty(value)) return;
    if (key === "status" && ctx.fieldEditable("status"))
      ctx.status.value =
        ctx.copiedOptionId(ctx.statuses.value, value) ?? ctx.status.value;
    if (key === "priority" && ctx.fieldEditable("priority"))
      ctx.priority.value =
        ctx.copiedOptionId(ctx.priorityOptions.value, value) ??
        ctx.priority.value;
    if (key === "version" && ctx.fieldEditable("version"))
      ctx.versionId.value = ctx.copiedOptionId(ctx.verOptions.value, value);
    if (
      key === "category" &&
      ctx.fieldEditable("category") &&
      ctx.categoryOptions.value.some(
        (option) => Number(option.value) === Number(value),
      )
    )
      ctx.categoryId.value = Number(value);
    if (key === "assignee" && ctx.fieldEditable("assignee")) {
      ctx.assigneeIds.value = templateAssigneeIds(value, ctx.memberOpts.value);
    }
    if (key === "planned_start_date" && ctx.fieldEditable("planned_start_date"))
      ctx.plannedStartDate.value = ctx.copiedDate(value);
    if (key === "planned_end_date" && ctx.fieldEditable("planned_end_date"))
      ctx.plannedEndDate.value = ctx.copiedDate(value);
    if (
      key === "estimated_work_hours" &&
      ctx.fieldEditable("estimated_work_hours")
    )
      ctx.estimatedWorkHours.value = Number(value) || 0;
    if (!customField) return;
    ctx.customValues.value[customField.id] = ctx.copiedCustomValue(
      customField,
      value,
    );
  }

  function captureBaseline() {
    baseline = {
      title: ctx.title.value,
      description: ctx.description.value,
      descriptionMode: ctx.descriptionMode.value,
      status: ctx.status.value,
      priority: ctx.priority.value,
      versionId: ctx.versionId.value,
      categoryId: ctx.categoryId.value,
      assigneeIds: [...ctx.assigneeIds.value],
      plannedStartDate: ctx.plannedStartDate.value,
      plannedEndDate: ctx.plannedEndDate.value,
      estimatedWorkHours: ctx.estimatedWorkHours.value,
      customValues: Object.fromEntries(
        Object.entries(ctx.customValues.value).map(([key, value]) => [
          key,
          Array.isArray(value) ? [...value] : value,
        ]),
      ),
    };
  }

  function restoreBaseline() {
    if (!baseline) return;
    ctx.title.value = baseline.title;
    ctx.description.value = baseline.description;
    ctx.descriptionMode.value = baseline.descriptionMode;
    ctx.status.value = baseline.status;
    ctx.priority.value = baseline.priority;
    ctx.versionId.value = baseline.versionId;
    ctx.categoryId.value = baseline.categoryId;
    ctx.assigneeIds.value = [...baseline.assigneeIds];
    ctx.plannedStartDate.value = baseline.plannedStartDate;
    ctx.plannedEndDate.value = baseline.plannedEndDate;
    ctx.estimatedWorkHours.value = baseline.estimatedWorkHours;
    ctx.customValues.value = Object.fromEntries(
      Object.entries(baseline.customValues).map(([key, value]) => [
        key,
        Array.isArray(value) ? [...value] : value,
      ]),
    );
  }

  function applyTemplate(tpl, { keepTitle = false } = {}) {
    if (!tpl || !templateEnabled.value) return;
    ctx.setDraftSuppress(true);
    restoreBaseline();
    if (keepTitle) ctx.title.value = baseline?.title || ctx.title.value;
    else ctx.title.value = tpl.title || "";
    ctx.description.value = tpl.description || "";
    ctx.descriptionMode.value = tpl.description_mode || "rich-text";
    ctx.bumpEditorRevision();
    for (const field of tpl.fields || []) applyTemplateField(field);
    nextTick(() => ctx.setDraftSuppress(false));
  }

  async function loadTemplates() {
    const sequence = ++loadSequence;
    const workspaceId = ctx.selectedWs.value;
    const typeKey = ctx.type.value;
    const current = () =>
      sequence === loadSequence &&
      workspaceId === ctx.selectedWs.value &&
      typeKey === ctx.type.value;
    if (!ctx.selectedWs.value) {
      templates.value = [];
      templateEnabled.value = false;
      templateId.value = undefined;
      baseline = null;
      return;
    }
    captureBaseline();
    try {
      const data = await listWorkItemTemplates(
        ctx.selectedWs.value,
        ctx.type.value,
      );
      if (!current()) return;
      templateEnabled.value = !!data?.enabled;
      templates.value = data?.items || [];
    } catch {
      if (!current()) return;
      templateEnabled.value = false;
      templates.value = [];
    }
    const firstId = templates.value[0]
      ? Number(templates.value[0].id)
      : undefined;
    if (Number(templateId.value) !== Number(firstId)) {
      skipSelectionApply = true;
      templateId.value = firstId;
    }
    if (skipInitialApply.value) {
      skipInitialApply.value = false;
      skipSelectionApply = false;
      templateId.value = undefined;
      return;
    }
    if (selectedTemplate.value) {
      applyTemplate(selectedTemplate.value, {
        keepTitle: !!ctx.keepTitleOnApply(),
      });
    }
  }

  function onTemplateIdChange() {
    if (skipSelectionApply) {
      skipSelectionApply = false;
      return;
    }
    if (selectedTemplate.value) applyTemplate(selectedTemplate.value);
  }

  return {
    templates,
    templateEnabled,
    templateId,
    showTemplates,
    selectedTemplate,
    isTemplateRequired,
    isTemplateFieldVisible,
    isTemplateFieldFilled,
    applyTemplate,
    loadTemplates,
    onTemplateIdChange,
  };
}
