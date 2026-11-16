import { ref, toValue, watch } from "vue";
import { useI18n } from "vue-i18n";
import { listTemplateRoleOptions } from "@/api/workspace";

export function useTemplateVisibleRoles(wsId) {
  const { t } = useI18n();
  const roleOptions = ref([]);
  const rolesReady = ref(false);

  watch(
    () => toValue(wsId),
    async (workspaceId, _, onCleanup) => {
      let current = true;
      onCleanup(() => {
        current = false;
      });
      rolesReady.value = false;
      roleOptions.value = [];
      if (!workspaceId) return;
      try {
        const roles = await listTemplateRoleOptions(workspaceId);
        if (current) {
          roleOptions.value = (roles || []).map((role) => ({
            value: Number(role.id),
            label: role.name,
          }));
          rolesReady.value = true;
        }
      } catch {
        // 请求拦截器已提示错误，保留已保存的角色并禁用选择。
      }
    },
    { immediate: true },
  );

  function roleLabel(id) {
    return (
      roleOptions.value.find((role) => role.value === id)?.label ||
      t("settings_template_unavailable_role", { id })
    );
  }

  return { roleOptions, rolesReady, roleLabel };
}
