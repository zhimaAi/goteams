import dayjs from "dayjs";
import { getRequirement } from "@/api/workspace";
import { parseCustomFieldValue } from "@/utils/customField";

export function useCopyWorkItemSource(ctx) {
  function fieldPrefillable(key) {
    return ctx.fieldEnabled(key) && ctx.fieldEditable(key);
  }

  function copiedOptionId(options, value) {
    const id = Number(value);
    return options.some((item) => Number(item.id) === id) ? id : undefined;
  }

  function copiedDate(value) {
    const timestamp = Number(value);
    return timestamp > 0 ? dayjs.unix(timestamp) : null;
  }

  function copiedCustomValue(field, value) {
    return parseCustomFieldValue(field, value);
  }

  async function applyCopySource() {
    let source;
    try {
      source = await getRequirement(ctx.routeWsId, ctx.copyFromId);
    } catch {
      return;
    }
    ctx.title.value = `${source.title} ${ctx.copySuffix()}`.slice(0, 200);
    ctx.description.value = source.description || "";
    ctx.descriptionMode.value =
      source.description_mode === "markdown" ? "markdown" : "rich-text";
    ctx.parentId.value = Number(source.parent_id) || undefined;
    if (ctx.type.value === "requirement" && fieldPrefillable("category"))
      ctx.categoryId.value = Number(source.category_id) || 0;
    if (fieldPrefillable("status"))
      ctx.status.value =
        copiedOptionId(ctx.statuses.value, source.status_id) ??
        ctx.status.value;
    if (fieldPrefillable("priority"))
      ctx.priority.value =
        copiedOptionId(ctx.priorities.value, source.priority) ??
        ctx.priority.value;
    if (fieldPrefillable("version"))
      ctx.versionId.value = copiedOptionId(
        ctx.versions.value,
        source.version_id,
      );
    if (fieldPrefillable("assignee"))
      ctx.assigneeIds.value = (source.assignee_ids || [])
        .map(Number)
        .filter((id) =>
          ctx.members.value.some((member) => Number(member.user_id) === id),
        );
    if (fieldPrefillable("planned_start_date"))
      ctx.plannedStartDate.value = copiedDate(source.planned_start_date);
    if (fieldPrefillable("planned_end_date"))
      ctx.plannedEndDate.value = copiedDate(source.planned_end_date);
    if (
      ctx.type.value === "requirement" &&
      fieldPrefillable("estimated_work_hours")
    )
      ctx.estimatedWorkHours.value = Number(source.estimated_work_hours) || 0;
    ctx.customValues.value = Object.fromEntries(
      ctx.customFields.value.map((field) => [
        field.id,
        copiedCustomValue(field, source.custom_fields?.[field.id] ?? ""),
      ]),
    );
  }

  return {
    applyCopySource,
    fieldPrefillable,
    copiedOptionId,
    copiedDate,
    copiedCustomValue,
  };
}
