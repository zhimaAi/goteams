// Unified mapping from work-item type to project-setting permission key.
// Planning's attribute fields/status/priority are all controlled by planning:edit; requirements/defects are split by action.
export function typeSettingPermission(typeKey, action) {
  if (typeKey === "planning") return "planning:edit";
  const prefix =
    typeKey === "defect" ? "defect_setting" : "requirement_setting";
  return `${prefix}:${action}`;
}
