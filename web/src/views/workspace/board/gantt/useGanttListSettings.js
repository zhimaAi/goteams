// 甘特图列表字段与全局查看模式配置。
import {
  computed,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  unref,
} from "vue";
import {
  GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
  readGlobalListPreferences,
} from "@/utils/boardPreferences";
import { GANTT_LIST_FIELD_DEFINITIONS } from "@/constants/gantt";
const PREVIEW_MODES = ["drawer", "modal", "window"];

export function useGanttListSettings(userId, metadata, t) {
  const currentUserId = () => unref(userId);
  const stored = readGlobalListPreferences(currentUserId());
  const previewMode = ref(
    PREVIEW_MODES.includes(stored.previewMode) ? stored.previewMode : "drawer",
  );
  const listPreferences = reactive({
    previewMode: previewMode.value,
    viewMode: "list",
    fields: [],
  });

  function enabledBuiltinFieldKeys(type) {
    const configs = metadata.value?.[type]?.field_configs || [];
    if (!configs.length) {
      return new Set(
        GANTT_LIST_FIELD_DEFINITIONS.map((field) => field.configKey),
      );
    }
    return new Set(
      configs
        .filter((field) => Number(field.enabled) === 1)
        .map((field) => field.key),
    );
  }

  const ganttListFields = computed(() => {
    const requirementKeys = enabledBuiltinFieldKeys("requirement");
    const defectKeys = enabledBuiltinFieldKeys("defect");
    return GANTT_LIST_FIELD_DEFINITIONS.filter(
      (field) =>
        requirementKeys.has(field.configKey) && defectKeys.has(field.configKey),
    ).map((field) => ({
      ...field,
      kind: "builtin",
      preferenceKey: `builtin:${field.configKey}`,
      label: t(field.labelKey),
      enabled: true,
      visible: true,
    }));
  });
  const visibleListFields = computed(() =>
    (listPreferences.fields.length
      ? listPreferences.fields
      : ganttListFields.value
    ).filter((field) => field.visible !== false),
  );

  function setPreviewMode(mode) {
    if (!PREVIEW_MODES.includes(mode)) return;
    previewMode.value = mode;
    listPreferences.previewMode = mode;
  }

  function applyListPreferences(preferences) {
    setPreviewMode(preferences.previewMode);
    listPreferences.fields = Array.isArray(preferences.fields)
      ? preferences.fields
      : [];
  }

  function handleSharedListPreferencesChange(event) {
    if (String(event.detail?.userId) !== String(currentUserId())) return;
    setPreviewMode(event.detail?.patch?.previewMode);
  }

  onMounted(() => {
    window.addEventListener(
      GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
      handleSharedListPreferencesChange,
    );
  });
  onBeforeUnmount(() => {
    window.removeEventListener(
      GLOBAL_LIST_PREFERENCES_CHANGE_EVENT,
      handleSharedListPreferencesChange,
    );
  });

  return {
    previewMode,
    listPreferences,
    ganttListFields,
    visibleListFields,
    applyListPreferences,
  };
}
