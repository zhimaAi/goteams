import { message } from "ant-design-vue";
import { createRequirement } from "@/api/workspace";
import { createDraftSessionId } from "@/utils/localDraftRecord";
import { showWorkItemActionSuccess } from "@/utils/workItemActionToast";
import { isValidPhoneFieldValue } from "@/utils/customField";

export function useCreateWorkItemSubmit(ctx) {
  async function submitCreate(continueAfter) {
    if (!ctx.canCreateCurrentType.value) return;
    if (!ctx.title.value.trim()) {
      ctx.titleTouched.value = true;
      message.warning(ctx.t("requirement_titleRequired"));
      return;
    }
    const missing = ctx.propertyItems.value.find(
      (field) => field.required && !ctx.isTemplateFieldFilled(field.key),
    );
    if (missing) {
      message.warning(
        ctx.t("settings_template_field_required", {
          name: missing.label,
        }),
      );
      return;
    }
    const invalidPhoneField = ctx.customFields.value.find(
      (field) =>
        ctx.isTemplateFieldVisible(`custom:${field.id}`) &&
        !isValidPhoneFieldValue(field, ctx.customValues.value[field.id]),
    );
    if (invalidPhoneField) {
      message.warning(ctx.t("settings_fieldPhoneInvalid"));
      return;
    }
    const fieldValue = (key, value, empty) =>
      ctx.isTemplateFieldVisible(key) ? value : empty;
    const plannedStartDate = fieldValue(
      "planned_start_date",
      ctx.plannedStartDate.value,
      null,
    );
    const plannedEndDate = fieldValue(
      "planned_end_date",
      ctx.plannedEndDate.value,
      null,
    );
    if (
      plannedStartDate &&
      plannedEndDate &&
      plannedStartDate.isAfter(plannedEndDate, "day")
    ) {
      message.warning(ctx.t("requirement_planDateInvalid"));
      return;
    }
    const estimatedWorkHours = Number(
      fieldValue("estimated_work_hours", ctx.estimatedWorkHours.value, 0),
    );
    if (
      ctx.type.value === "requirement" &&
      (!Number.isFinite(estimatedWorkHours) ||
        estimatedWorkHours < 0 ||
        estimatedWorkHours > 99999999.99 ||
        Math.abs(
          estimatedWorkHours * 100 - Math.round(estimatedWorkHours * 100),
        ) > 0.000001)
    ) {
      message.warning(ctx.t("requirement_workHoursInvalid"));
      return;
    }

    ctx.creating.value = true;
    try {
      const created = await createRequirement(ctx.selectedWs.value, {
        title: ctx.title.value,
        description: ctx.description.value,
        description_mode: ctx.descriptionMode.value || "rich-text",
        template_id: Number(ctx.selectedTemplate.value?.id) || 0,
        image_file_ids: ctx.imageFileIds.value,
        type: ctx.type.value,
        priority: fieldValue("priority", ctx.priority.value, null) ?? null,
        status_id: fieldValue("status", ctx.status.value, 0) || 0,
        parent_id: Number(ctx.parentId.value) || 0,
        category_id:
          ctx.type.value === "requirement"
            ? Number(
                fieldValue(
                  "category",
                  ctx.categoryId.value,
                  Number(ctx.returnCategoryQuery.value) || 0,
                ),
              ) || 0
            : 0,
        assignee_ids: fieldValue("assignee", ctx.assigneeIds.value, []),
        version_id: fieldValue("version", ctx.versionId.value, 0) || 0,
        planned_start_date: plannedStartDate
          ? plannedStartDate.startOf("day").unix()
          : 0,
        planned_end_date: plannedEndDate
          ? plannedEndDate.startOf("day").unix()
          : 0,
        estimated_work_hours:
          ctx.type.value === "requirement" ? estimatedWorkHours : 0,
        custom_fields: ctx.customFieldPayload(),
        planning_id: ctx.planningId.value,
      });
      if (ctx.detailAction) {
        const textKey =
          ctx.detailAction === "copy"
            ? "workItemAction_copySuccess"
            : ctx.detailAction === "child"
              ? "workItemAction_childSuccess"
              : "workItemAction_createDefectSuccess";
        showWorkItemActionSuccess(
          ctx.router,
          ctx.t(textKey),
          {
            id: created?.id,
            workspace_id: Number(ctx.selectedWs.value),
            type: ctx.type.value,
          },
          ctx.t("workItemAction_viewDetail"),
        );
      } else {
        message.success(ctx.t("requirement_createSuccess"));
      }
      await ctx.draft.flush();
      await ctx.draft.markPosted();

      if (!continueAfter) {
        ctx.goBoard();
        return;
      }
      await ctx.draft.startNew(createDraftSessionId());
      ctx.title.value = "";
      ctx.titleTouched.value = false;
      ctx.description.value = "";
      ctx.imageFileIds.value = [];
      ctx.parentId.value = undefined;
      if (ctx.selectedTemplate.value)
        ctx.applyTemplate(ctx.selectedTemplate.value);
      await ctx.draft.list();
    } catch {
      // The request interceptor already reports the failure.
    } finally {
      ctx.creating.value = false;
    }
  }

  return { submitCreate };
}
