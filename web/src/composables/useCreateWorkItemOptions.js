import {
  getWorkspaceMetadata,
  listRequirementCategories,
} from "@/api/workspace";
import { emptyCustomFieldValue } from "@/utils/customField";
import { currentProjectMembers } from "@/utils/workItemTemplate";

import { buildCategoryOptions } from "@/utils/requirementCategory";

export function useCreateWorkItemOptions(ctx) {
  let loadSequence = 0;
  function clearOptions() {
    ctx.versions.value = [];
    ctx.members.value = [];
    ctx.statuses.value = [];
    ctx.priorities.value = [];
    ctx.fieldConfigs.value = [];
    ctx.categories.value = [];
    ctx.customFields.value = [];
    ctx.customFieldMembers.value = [];
    ctx.customValues.value = {};
  }

  async function loadOptions() {
    const sequence = ++loadSequence;
    const workspaceId = ctx.selectedWs.value;
    const typeKey = ctx.type.value;
    const current = () =>
      sequence === loadSequence &&
      workspaceId === ctx.selectedWs.value &&
      typeKey === ctx.type.value;
    if (!ctx.selectedWs.value) {
      clearOptions();
      await ctx.loadTemplates();
      return;
    }
    try {
      const configs =
        ctx.authStore.consumeProjectMetadata(ctx.selectedWs.value) ||
        (await getWorkspaceMetadata(ctx.selectedWs.value));
      if (!current()) return;
      const typeConfigs = configs?.[typeKey] || {};
      ctx.versions.value = (configs?.versions || []).map((item) => ({
        ...item,
        id: Number(item.id),
        is_ended: item.is_ended === true || Number(item.ended_at) > 0,
      }));
      ctx.members.value = currentProjectMembers(configs?.members);
      ctx.customFieldMembers.value = configs?.custom_field_members || [];
      ctx.statuses.value = typeConfigs.statuses || [];
      const initialStatusKey =
        typeKey === "defect"
          ? "defect_initial_status_id"
          : "requirement_initial_status_id";
      const initialStatusId = Number(configs?.settings?.[initialStatusKey]);
      ctx.status.value = (
        ctx.statuses.value.find(
          (item) => Number(item.id) === ctx.requestedStatusId,
        ) ||
        ctx.statuses.value.find(
          (item) => Number(item.id) === initialStatusId,
        ) ||
        ctx.statuses.value[0]
      )?.id;
      ctx.priorities.value = typeConfigs.priorities || [];
      if (
        !ctx.priorities.value.some(
          (item) => Number(item.id) === Number(ctx.priority.value),
        )
      ) {
        ctx.priority.value = ctx.priorities.value.find(
          (item) => Number(item.is_default) === 1,
        )?.id;
      }
      ctx.fieldConfigs.value = typeConfigs.field_configs || [];
      if (typeKey === "requirement") {
        try {
          const categoryResult = await listRequirementCategories(
            ctx.selectedWs.value,
          );
          if (!current()) return;
          ctx.categories.value = buildCategoryOptions(
            categoryResult?.list || [],
            ctx.t("requirement_categoryUncategorized"),
          );
        } catch {
          if (!current()) return;
          ctx.categories.value = buildCategoryOptions(
            [],
            ctx.t("requirement_categoryUncategorized"),
          );
        }
        if (
          !ctx.returnCategoryQuery.value &&
          !ctx.categories.value.some(
            (option) => option.value === Number(ctx.categoryId.value),
          )
        ) {
          ctx.categoryId.value = 0;
        }
      } else {
        ctx.categories.value = [];
        if (typeKey !== "requirement") ctx.categoryId.value = 0;
      }
      ctx.customFields.value = typeConfigs.custom_fields || [];
      ctx.customValues.value = Object.fromEntries(
        ctx.customFields.value.map((field) => [
          field.id,
          emptyCustomFieldValue(field),
        ]),
      );
    } catch {
      if (!current()) return;
      clearOptions();
      ctx.status.value = undefined;
      ctx.priority.value = undefined;
    }
    if (current()) await ctx.loadTemplates();
  }

  return { loadOptions };
}
