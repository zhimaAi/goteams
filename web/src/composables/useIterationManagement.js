// Iteration page state and mutation workflow.
import { computed, onMounted, reactive, ref, watch } from "vue";
import { message, Modal } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import {
  createVersion,
  deleteVersion,
  getIterationFilterOptions,
  listVersions,
  listVersionWorkspaceOptions,
  updateVersion,
  updateVersionEnded,
} from "@/api/workspace";
import { useIterationDetail } from "@/composables/useIterationDetail";
import {
  emptyIterationFilterOptions,
  iterationEditForm,
  iterationPayload,
  newIterationForm,
  normalizeIteration,
  resetIterationForm,
} from "@/components/iteration/iterationModel";

export function useIterationManagement(props) {
  const { t } = useI18n();
  const detail = useIterationDetail();
  const iterations = ref([]);
  const activeIterationId = ref(null);
  const filterOptions = ref(emptyIterationFilterOptions());
  const workspaceOptions = ref([]);
  const createOpen = ref(false);
  const editOpen = ref(false);
  const saving = ref(false);
  const editingIteration = ref(null);
  const listRefreshKey = ref(0);
  const createForm = reactive(newIterationForm(props.wsId));
  const editForm = reactive(newIterationForm(props.wsId));
  const canCreate = computed(() =>
    detail.authStore.hasPermission("version:create"),
  );
  const canEdit = computed(() =>
    detail.authStore.hasPermission("version:edit"),
  );
  const canDelete = computed(() =>
    detail.authStore.hasPermission("version:delete"),
  );

  async function loadIterations(preferredId = activeIterationId.value) {
    try {
      iterations.value = (await listVersions(props.wsId)).map(
        normalizeIteration,
      );
      const preferred = iterations.value.find(
        (item) => Number(item.id) === Number(preferredId),
      );
      activeIterationId.value =
        preferred?.id || iterations.value[0]?.id || null;
      await loadFilterOptions();
    } catch {
      iterations.value = [];
      activeIterationId.value = null;
    }
  }

  async function loadFilterOptions() {
    if (!activeIterationId.value) {
      filterOptions.value = emptyIterationFilterOptions();
      return;
    }
    try {
      filterOptions.value =
        (await getIterationFilterOptions(
          props.wsId,
          activeIterationId.value,
        )) || emptyIterationFilterOptions();
    } catch {
      filterOptions.value = emptyIterationFilterOptions();
    }
  }

  async function loadWorkspaceOptions(selected = []) {
    let available = [];
    try {
      available = await listVersionWorkspaceOptions(props.wsId);
    } catch {
      /* handled by interceptor */
    }
    const merged = new Map();
    [...selected, ...available].forEach((workspace) =>
      merged.set(Number(workspace.id), {
        id: Number(workspace.id),
        name: workspace.name,
      }),
    );
    workspaceOptions.value = [...merged.values()];
  }

  async function selectIteration(id) {
    if (Number(activeIterationId.value) === Number(id)) return;
    activeIterationId.value = Number(id);
    detail.navigationItems.value = [];
    await loadFilterOptions();
  }

  async function openCreate() {
    resetIterationForm(createForm, props.wsId);
    await loadWorkspaceOptions();
    createOpen.value = true;
  }

  async function openEdit(iteration) {
    editingIteration.value = iteration;
    resetIterationForm(editForm, props.wsId, iterationEditForm(iteration));
    await loadWorkspaceOptions(iteration.workspaces);
    editOpen.value = true;
  }

  function validatedPayload(form) {
    const result = iterationPayload(form, props.wsId);
    if (result.error) message.warning(t(result.error));
    return result.data;
  }

  async function createIteration() {
    const payload = validatedPayload(createForm);
    if (!payload) return;
    saving.value = true;
    try {
      const result = await createVersion(props.wsId, payload);
      createOpen.value = false;
      message.success(t("workspace_versionCreated"));
      await loadIterations(result?.id);
    } catch {
      /* handled by interceptor */
    } finally {
      saving.value = false;
    }
  }

  async function updateIteration() {
    if (!editingIteration.value) return;
    const payload = validatedPayload(editForm);
    if (!payload) return;
    saving.value = true;
    try {
      await updateVersion(props.wsId, editingIteration.value.id, payload);
      editOpen.value = false;
      message.success(t("workspace_versionUpdated"));
      await loadIterations(editingIteration.value.id);
    } catch {
      /* handled by interceptor */
    } finally {
      saving.value = false;
    }
  }

  function confirmEnded(iteration, ended) {
    Modal.confirm({
      title: t(ended ? "iteration_endTitle" : "iteration_reopenTitle"),
      content: t(ended ? "iteration_endConfirm" : "iteration_reopenConfirm", {
        name: iteration.name,
      }),
      okText: t(ended ? "iteration_confirmEnd" : "iteration_confirmReopen"),
      cancelText: t("common_cancel"),
      async onOk() {
        await updateVersionEnded(props.wsId, iteration.id, { ended });
        message.success(
          t(ended ? "iteration_endedSuccess" : "iteration_reopenedSuccess"),
        );
        await loadIterations(iteration.id);
      },
    });
  }

  function confirmDelete(iteration) {
    Modal.confirm({
      title: t("workspace_deleteVersion"),
      content: t("workspace_deleteVersionConfirm", { name: iteration.name }),
      okText: t("common_delete"),
      okType: "danger",
      cancelText: t("common_cancel"),
      async onOk() {
        await deleteVersion(props.wsId, iteration.id);
        message.success(t("workspace_versionDeleted"));
        await loadIterations();
      },
    });
  }

  function handleAction(key, iteration) {
    if (key === "edit") openEdit(iteration);
    else if (key === "end") confirmEnded(iteration, true);
    else if (key === "reopen") confirmEnded(iteration, false);
    else if (key === "delete") confirmDelete(iteration);
  }

  async function refreshAfterWorkItemChange() {
    listRefreshKey.value += 1;
    await loadIterations(activeIterationId.value);
  }

  onMounted(loadIterations);
  watch(
    () => props.wsId,
    () => {
      detail.closeDetail();
      activeIterationId.value = null;
      loadIterations();
    },
  );

  return {
    ...detail,
    iterations,
    activeIterationId,
    filterOptions,
    workspaceOptions,
    createOpen,
    editOpen,
    saving,
    editingIteration,
    listRefreshKey,
    createForm,
    editForm,
    canCreate,
    canEdit,
    canDelete,
    selectIteration,
    openCreate,
    createIteration,
    updateIteration,
    handleAction,
    refreshAfterWorkItemChange,
  };
}
