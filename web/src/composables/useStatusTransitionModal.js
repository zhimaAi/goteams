// Owns status-transition modal loading, draft validation, and submission.
import { computed, reactive, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { showWorkItemUpdateSuccess } from "@/utils/workItemActionToast";
import {
  getRequirement,
  getWorkspaceMetadata,
  transitionWorkItemStatus,
} from "@/api/workspace";
import {
  buildStatusTransitionListUpdate,
  statusActionPayload,
  statusTransitionCustomFieldId,
  statusTransitionFieldLabel,
  statusTransitionForm,
} from "@/utils/statusTransition";
import {
  createStatusTransitionDraft,
  isStatusTransitionFieldEmpty,
  statusTransitionCustomPayloadValue,
  statusTransitionUnixOrZero,
} from "@/utils/statusTransitionDraft";

export function useStatusTransitionModal(props, emit, t) {
  const loading = ref(false);
  const submitting = ref(false);
  const loadedMetadata = ref(null);
  const loadedAvailableStatusIds = ref(null);
  const selectedStatusId = ref(0);
  const fields = ref([]);
  const draft = reactive(createStatusTransitionDraft(null, []));

  const resolvedMetadata = computed(
    () => props.metadata || loadedMetadata.value,
  );
  const typeKey = computed(() => props.workItem?.type || "requirement");
  const typeMeta = computed(
    () => resolvedMetadata.value?.[typeKey.value] || {},
  );
  const members = computed(() =>
    (resolvedMetadata.value?.members || [])
      .filter((item) => !item.removed && String(item.user_status) !== "0")
      .map((item) => ({ ...item, user_id: Number(item.user_id) })),
  );
  const memberOptions = computed(() =>
    members.value.map((item) => ({
      value: Number(item.user_id),
      label: item.user_name,
    })),
  );
  const versionOptions = computed(() => {
    const selectedId = Number(props.workItem?.version_id) || 0;
    return (resolvedMetadata.value?.versions || [])
      .filter((item) => !item.is_ended || Number(item.id) === selectedId)
      .map((item) => ({ value: Number(item.id), label: item.name }));
  });
  const priorityOptions = computed(() =>
    (typeMeta.value.priorities || []).map((item) => ({
      value: Number(item.id),
      label: item.label || item.name,
    })),
  );
  const customFields = computed(() =>
    (typeMeta.value.custom_fields || []).filter(
      (item) => Number(item.is_enabled) === 1,
    ),
  );
  const statusOptions = computed(() => {
    const all = typeMeta.value.statuses || [];
    const allowed = Array.isArray(props.workItem?.available_status_ids)
      ? props.workItem.available_status_ids
      : loadedAvailableStatusIds.value;
    if (!Array.isArray(allowed)) return [];
    const ids = new Set(allowed.map(Number));
    ids.add(Number(props.workItem?.status_id));
    return all.filter((item) => ids.has(Number(item.id)));
  });

  function displayStatusName(name) {
    return name || "";
  }
  function fieldLabel(key) {
    return statusTransitionFieldLabel(key, t, customFields.value);
  }
  function customFieldOf(key) {
    const id = statusTransitionCustomFieldId(key);
    return customFields.value.find((item) => Number(item.id) === id) || null;
  }
  function close() {
    emit("update:open", false);
  }

  async function confirm() {
    if (loading.value || submitting.value || !selectedStatusId.value) return;
    for (const field of fields.value) {
      if (field.required !== 1) continue;
      let value = draft[field.key];
      if (field.key === "assignee") value = draft.assignee_ids;
      if (field.key === "comment") value = draft.comment;
      if (statusTransitionCustomFieldId(field.key))
        value = draft.custom_fields[customFieldOf(field.key)?.id];
      if (
        isStatusTransitionFieldEmpty(field, value, customFieldOf(field.key))
      ) {
        message.warning(t("status_transition_required"));
        return;
      }
    }
    const payload = props.statusAction
      ? statusActionPayload(props.statusAction)
      : { status_id: Number(selectedStatusId.value) };
    const configured = new Set(fields.value.map((item) => item.key));
    if (configured.has("assignee"))
      payload.assignee_ids = draft.assignee_ids || [];
    if (configured.has("version")) payload.version_id = draft.version_id || 0;
    if (configured.has("priority")) payload.priority = draft.priority || 0;
    if (configured.has("planned_start_date"))
      payload.planned_start_date = statusTransitionUnixOrZero(
        draft.planned_start_date,
      );
    if (configured.has("planned_end_date"))
      payload.planned_end_date = statusTransitionUnixOrZero(
        draft.planned_end_date,
      );
    if (configured.has("comment")) payload.comment = draft.comment || "";
    if (configured.has("comment") && draft.image_file_ids.length)
      payload.image_file_ids = draft.image_file_ids;
    const customPayload = {};
    for (const field of fields.value) {
      const custom = customFieldOf(field.key);
      if (!custom) continue;
      customPayload[custom.id] = statusTransitionCustomPayloadValue(
        custom,
        draft.custom_fields[custom.id],
      );
    }
    if (Object.keys(customPayload).length)
      payload.custom_fields = customPayload;

    submitting.value = true;
    try {
      await transitionWorkItemStatus(props.wsId, props.workItem.id, payload);
      const patch = buildStatusTransitionListUpdate(
        props.workItem,
        payload,
        configured,
        resolvedMetadata.value || {},
      );
      const changedFields = Object.keys(payload).filter(
        (key) => key !== "comment" && key !== "status_action",
      );
      showWorkItemUpdateSuccess(t("workspace_updateSuccess"));
      emit("success", patch, changedFields);
      close();
    } catch (error) {
      if (props.statusAction && error.response?.status === 409) {
        message.warning(
          error.response.data?.message || t("status_action_stale"),
        );
        emit("stale");
        close();
      }
    } finally {
      submitting.value = false;
    }
  }

  watch(
    () => [props.open, props.wsId, props.workItem?.id, props.statusAction?.id],
    async ([open, wsId], _previous, onCleanup) => {
      if (!open || !props.workItem) return;
      const item = props.workItem;
      const action = props.statusAction;
      let current = true;
      onCleanup(() => {
        current = false;
      });
      let available = Array.isArray(item.available_status_ids)
        ? item.available_status_ids
        : null;
      loading.value = !props.metadata || available === null;
      selectedStatusId.value = 0;
      try {
        const metadata = props.metadata || (await getWorkspaceMetadata(wsId));
        if (!current) return;
        if (available === null) {
          const detail = await getRequirement(wsId, item.id);
          available = Array.isArray(detail?.available_status_ids)
            ? detail.available_status_ids
            : [];
        }
        if (!current) return;
        loadedMetadata.value = metadata;
        loadedAvailableStatusIds.value = available;
        const form = statusTransitionForm(
          metadata?.settings || {},
          typeKey.value,
          customFields.value,
          typeMeta.value.field_configs || [],
        );
        fields.value = form.enabled === 1 ? form.fields : [];
        selectedStatusId.value =
          Number(action?.target_status_id ?? item.status_id) || 0;
        if (
          action &&
          !statusOptions.value.some(
            (option) => Number(option.id) === selectedStatusId.value,
          )
        ) {
          message.warning(t("status_action_stale"));
          emit("stale");
          close();
          return;
        }
        Object.assign(
          draft,
          createStatusTransitionDraft(item, customFields.value),
        );
      } catch {
        if (!current) return;
        loadedMetadata.value = null;
        loadedAvailableStatusIds.value = [];
      } finally {
        if (current) loading.value = false;
      }
    },
  );

  return {
    loading,
    submitting,
    selectedStatusId,
    fields,
    draft,
    members,
    memberOptions,
    versionOptions,
    priorityOptions,
    statusOptions,
    displayStatusName,
    fieldLabel,
    customFieldOf,
    close,
    confirm,
  };
}
