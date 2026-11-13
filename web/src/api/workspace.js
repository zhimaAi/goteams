import { request, axios } from "./request";

// ======================== Workspace API ========================

export function listWorkspaces() {
  return request({
    url: "/api/workspaces",
    method: "GET",
  });
}

export function listMyWork() {
  return request({
    url: "/api/my-work",
    method: "GET",
  });
}

// Work items the current user opened within the last week, most recent first.
export function listRecentVisits() {
  return request({
    url: "/api/my-work/recent-visits",
    method: "GET",
  });
}

// Stamps the current user's latest visit time on a work item.
// Silent: a failed visit record must not interrupt the detail drawer with a toast.
export function recordRequirementVisit(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/visit`,
    method: "POST",
    silent: true,
  });
}

export function listTeamWork(params) {
  return request({
    url: "/api/team-work",
    method: "GET",
    params,
  });
}

export function listTeamWorkMembers() {
  return request({
    url: "/api/team-work/members",
    method: "GET",
  });
}

// Picker source data for the team-work display list: organization tree plus the members the
// current role may see, each flagged with whether it is already shown.
export function listTeamWorkMemberCandidates() {
  return request({
    url: "/api/team-work/member-candidates",
    method: "GET",
  });
}

export function updateTeamWorkMembers(data) {
  return request({
    url: "/api/team-work/members",
    method: "PUT",
    data,
  });
}

export function createWorkspace(data) {
  return request({
    url: "/api/workspaces",
    method: "POST",
    data,
  });
}

/** List template workspaces from the configured template admin account. */
export function listWorkspaceTemplates() {
  return request({
    url: "/api/workspace-templates",
    method: "GET",
  });
}

export function listWorkspaceTypes() {
  return request({ url: "/api/workspace-types", method: "GET" });
}

export function createWorkspaceType(name) {
  return request({
    url: "/api/workspace-types",
    method: "POST",
    data: { name },
  });
}

export function getWorkspace(id) {
  return request({
    url: `/api/workspaces/${id}`,
    method: "GET",
  });
}

export function getWorkspaceAccess(id) {
  return request({
    url: `/api/workspaces/${id}/access`,
    method: "GET",
    silent: true,
  });
}

export function requestWorkspaceAccess(id, reason) {
  return request({
    url: `/api/workspaces/${id}/access-requests`,
    method: "POST",
    data: { reason },
  });
}

export function updateWorkspace(id, data) {
  return request({
    url: `/api/workspaces/${id}`,
    method: "PUT",
    data,
  });
}

export function deleteWorkspace(id) {
  return request({
    url: `/api/workspaces/${id}`,
    method: "DELETE",
  });
}

/**
 * 结束或重新开启项目
 * @param {number|string} id - 项目 ID
 * @param {{ended: boolean}} data - 结束状态
 */
export function updateWorkspaceEnded(id, data) {
  return request({
    url: `/api/workspaces/${id}/ended`,
    method: "PUT",
    data,
  });
}

/**
 * 开启/关闭甘特图应用（应用市场）
 * @param {number|string} id - 项目 ID
 * @param {{enabled: boolean}} data - 开关状态
 */
export function updateWorkspaceGanttEnabled(id, data) {
  return request({
    url: `/api/workspaces/${id}/apps/gantt`,
    method: "PUT",
    data,
  });
}

/**
 * 获取企业下全部项目（项目管理页）
 * @param {{keyword?: string}} params - 查询参数
 * @returns {Promise<Array>}
 */
export function listEnterpriseProjects(params) {
  return request({
    url: "/api/enterprise/projects",
    method: "GET",
    params,
  });
}

/**
 * 保存项目展示顺序（同时作用于左侧项目列表）
 * @param {{ids: number[], apply_to_all?: boolean}} data - 按目标顺序排列的项目 ID
 */
export function sortEnterpriseProjects(data) {
  return request({
    url: "/api/enterprise/projects/sort",
    method: "PUT",
    data,
  });
}

export function uploadWorkspaceIcon(id, file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: `/api/workspaces/${id}/icon`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

export function getWorkspaceIconBlob(id) {
  return axios
    .get(`/api/workspaces/${id}/icon`, {
      responseType: "blob",
    })
    .then((response) => response.data);
}

export function deleteWorkspaceIcon(id) {
  return request({
    url: `/api/workspaces/${id}/icon`,
    method: "DELETE",
  });
}

// ======================== Requirement Category API ========================

export function listRequirementCategories(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/requirement-categories`,
    method: "GET",
  });
}

export function assignRequirementCategory(wsId, reqId, categoryId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/category`,
    method: "PUT",
    data: { category_id: categoryId },
  });
}

export function createRequirementCategory(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirement-categories`,
    method: "POST",
    data,
  });
}

export function updateRequirementCategory(wsId, categoryId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirement-categories/${categoryId}`,
    method: "PUT",
    data,
  });
}

export function deleteRequirementCategory(wsId, categoryId) {
  return request({
    url: `/api/workspaces/${wsId}/requirement-categories/${categoryId}`,
    method: "DELETE",
  });
}

export function moveRequirementCategory(wsId, categoryId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirement-categories/${categoryId}/move`,
    method: "PUT",
    data,
  });
}

// ======================== Requirement API ========================

export function listRequirements(wsId, params) {
  return request({
    url: `/api/workspaces/${wsId}/requirements`,
    method: "GET",
    params,
  });
}

export function listGanttRequirements(wsId, params) {
  return request({
    url: `/api/workspaces/${wsId}/gantt/requirements`,
    method: "GET",
    params,
  });
}

export function createRequirement(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements`,
    method: "POST",
    data: {
      ...data,
      timezone:
        data.timezone ||
        Intl.DateTimeFormat().resolvedOptions().timeZone ||
        "UTC",
    },
  });
}

export function getRequirement(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}`,
    method: "GET",
  });
}

export function updateRequirement(wsId, reqId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}`,
    method: "PUT",
    data,
  });
}

export function listRequirementWorkLogs(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/work-logs`,
    method: "GET",
  });
}

export function createRequirementWorkLog(wsId, reqId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/work-logs`,
    method: "POST",
    data,
  });
}

/**
 * Apply an advanced status transition with extra fields and an optional comment.
 * @param {number|string} wsId
 * @param {number|string} reqId
 * @param {Object} data
 */
export function transitionWorkItemStatus(wsId, reqId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/status-transitions`,
    method: "POST",
    data,
  });
}

export function updateRequirementWorkLog(wsId, reqId, logId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/work-logs/${logId}`,
    method: "PUT",
    data,
  });
}

export function deleteRequirementWorkLog(wsId, reqId, logId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/work-logs/${logId}`,
    method: "DELETE",
  });
}

export function batchUpdateRequirements(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/batch`,
    method: "PUT",
    data,
  });
}

export function batchDeleteRequirements(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/batch`,
    method: "DELETE",
    data,
  });
}

export function deleteRequirement(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}`,
    method: "DELETE",
  });
}

export function listWorkItemActionTargets(params) {
  return request({
    url: "/api/work-item-action-targets",
    method: "GET",
    params,
  });
}

export function moveWorkItem(wsId, reqId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/move`,
    method: "POST",
    data,
  });
}

export function convertWorkItem(wsId, reqId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/convert`,
    method: "POST",
    data,
  });
}

export function listWorkItemMergeTargets(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/merge-targets`,
    method: "GET",
  });
}

export function mergeWorkItem(wsId, reqId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/merge`,
    method: "POST",
    data,
  });
}

export function downloadWorkItemImportTemplate(wsId, params) {
  return axios.get(`/api/workspaces/${wsId}/work-items/import-template`, {
    params,
    responseType: "blob",
  });
}

export function exportWorkItems(wsId, params = {}) {
  return axios.get(`/api/workspaces/${wsId}/work-items/export`, {
    params: {
      ...params,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    },
    responseType: "blob",
  });
}

export function importWorkItems(wsId, type, file, timezone) {
  const data = new FormData();
  data.append("file", file);
  data.append("timezone", timezone);
  return request({
    url: `/api/workspaces/${wsId}/work-items/import`,
    method: "POST",
    params: { type },
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}

export function previewWorkItemImport(wsId, type, file, timezone) {
  const data = new FormData();
  data.append("file", file);
  data.append("timezone", timezone);
  return request({
    url: `/api/workspaces/${wsId}/work-items/import-preview`,
    method: "POST",
    params: { type },
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}

// ======================== Comment API ========================

export function listComments(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/comments`,
    method: "GET",
  });
}

export function createComment(wsId, reqId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/comments`,
    method: "POST",
    data,
  });
}

export function updateComment(wsId, reqId, commentId, data) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/comments/${commentId}`,
    method: "PUT",
    data,
  });
}

export function deleteComment(wsId, reqId, commentId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/comments/${commentId}`,
    method: "DELETE",
  });
}

// ======================== History API ========================

export function listRequirementHistories(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/histories`,
    method: "GET",
  });
}

export function getRequirementAging(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/aging`,
    method: "GET",
  });
}

export function getRequirementHistory(wsId, reqId, historyId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/histories/${historyId}`,
    method: "GET",
  });
}

// side is "old" or "new": which side of the recorded change the description is restored to.
export function restoreRequirementHistory(wsId, reqId, historyId, side) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/histories/${historyId}/restore`,
    method: "POST",
    data: { side },
  });
}

export function listRequirementAttachments(wsId, reqId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/attachments`,
    method: "GET",
  });
}

export function uploadRequirementAttachments(
  wsId,
  reqId,
  files,
  commentId = 0,
) {
  const data = new FormData();
  files.forEach((file) => data.append("files", file));
  if (commentId) data.append("comment_id", String(commentId));
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/attachments`,
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    // Attachments may be up to 200MB; the 15s global default would abort long before that.
    timeout: 600000,
  });
}

export function downloadRequirementAttachment(wsId, reqId, attachmentId) {
  return axios.get(
    `/api/workspaces/${wsId}/requirements/${reqId}/attachments/${attachmentId}/download`,
    { responseType: "blob" },
  );
}

export function deleteRequirementAttachment(wsId, reqId, attachmentId) {
  return request({
    url: `/api/workspaces/${wsId}/requirements/${reqId}/attachments/${attachmentId}`,
    method: "DELETE",
  });
}

// ======================== Version API ========================

export function listVersions(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/versions`,
    method: "GET",
  });
}

/** List workspaces where the current user may create iterations. */
export function listVersionWorkspaceOptions(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/versions/workspace-options`,
    method: "GET",
  });
}

/** List visible work items in an iteration across associated workspaces. */
export function listVersionWorkItems(wsId, versionId, params) {
  return request({
    url: `/api/workspaces/${wsId}/versions/${versionId}/work-items`,
    method: "GET",
    params,
  });
}

/** Load iteration search option sources visible to the current user. */
export function getIterationFilterOptions(wsId, versionId) {
  return request({
    url: `/api/workspaces/${wsId}/versions/${versionId}/filter-options`,
    method: "GET",
  });
}

export function createVersion(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/versions`,
    method: "POST",
    data,
  });
}

export function updateVersion(wsId, versionId, data) {
  return request({
    url: `/api/workspaces/${wsId}/versions/${versionId}`,
    method: "PUT",
    data,
  });
}

export function updateVersionEnded(wsId, versionId, data) {
  return request({
    url: `/api/workspaces/${wsId}/versions/${versionId}/ended`,
    method: "PUT",
    data,
  });
}

export function deleteVersion(wsId, versionId) {
  return request({
    url: `/api/workspaces/${wsId}/versions/${versionId}`,
    method: "DELETE",
  });
}

// ======================== Member API ========================

export function listMembers(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/members`,
    method: "GET",
  });
}

// Enterprise level external system notification strategies: one configuration pushes for every project.
const externalNotificationBase =
  "/api/enterprise/external-notification/strategies";

export function listExternalNotificationStrategies() {
  return request({
    url: externalNotificationBase,
    method: "GET",
  });
}

export function createExternalNotificationStrategy(data) {
  return request({
    url: externalNotificationBase,
    method: "POST",
    data,
  });
}

export function updateExternalNotificationStrategy(strategyId, data) {
  return request({
    url: `${externalNotificationBase}/${strategyId}`,
    method: "PUT",
    data,
  });
}

export function updateExternalNotificationStrategyStatus(strategyId, enabled) {
  return request({
    url: `${externalNotificationBase}/${strategyId}/status`,
    method: "PUT",
    data: { enabled: !!enabled },
  });
}

export function testExternalNotificationStrategy(strategyId) {
  return request({
    url: `${externalNotificationBase}/${strategyId}/test`,
    method: "POST",
  });
}

export function deleteExternalNotificationStrategy(strategyId) {
  return request({
    url: `${externalNotificationBase}/${strategyId}`,
    method: "DELETE",
  });
}

export function addMember(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/members`,
    method: "POST",
    data,
  });
}

// Source data for the project member picker: enterprise name, departments, enterprise roles and
// every tenant member flagged with whether it already joined this workspace.
export function listMemberCandidates(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/members/candidates`,
    method: "GET",
  });
}

// Copy the selected roles and their members from another project into this one.
export function copyProjectMembers(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/members/copy`,
    method: "POST",
    data,
  });
}

export function updateMemberRole(wsId, userId, data) {
  return request({
    url: `/api/workspaces/${wsId}/members/${userId}/role`,
    method: "PUT",
    data,
  });
}

// Hand-over targets ride along as query parameters because the route is a DELETE.
export function removeMember(wsId, userId, params) {
  return request({
    url: `/api/workspaces/${wsId}/members/${userId}`,
    method: "DELETE",
    params,
  });
}

// Reports how many unfinished work items the member holds in this project.
export function previewWorkspaceMemberRemoval(wsId, userId) {
  return request({
    url: `/api/workspaces/${wsId}/members/${userId}/removal-preview`,
    method: "GET",
  });
}

// Hands the default project admin role to another project member.
export function transferWorkspaceOwner(wsId, targetUserId) {
  return request({
    url: `/api/workspaces/${wsId}/owner/transfer`,
    method: "POST",
    data: { target_user_id: Number(targetUserId) },
  });
}

// ======================== Enterprise Member API ========================

export function listEnterpriseMembers() {
  return request({
    url: "/api/enterprise/members",
    method: "GET",
  });
}

export function getEnterprisePlanUsage() {
  return request({
    url: "/api/enterprise/plan/usage",
    method: "GET",
  });
}

export function updateEnterpriseName(data) {
  return request({
    url: "/api/enterprise/name",
    method: "PUT",
    data,
  });
}

export function uploadEnterpriseMemberAvatar(file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: "/api/upload/avatar",
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

export function createEnterpriseMember(data) {
  return request({
    url: "/api/enterprise/members",
    method: "POST",
    data,
  });
}

export function updateEnterpriseMember(userId, data) {
  return request({
    url: `/api/enterprise/members/${userId}`,
    method: "PUT",
    data,
  });
}

// Hand-over targets ride along as query parameters because the route is a DELETE.
export function removeEnterpriseMember(userId, params) {
  return request({
    url: `/api/enterprise/members/${userId}`,
    method: "DELETE",
    params,
  });
}

// Reports the projects the member owns and how many unfinished work items they hold.
export function previewEnterpriseMemberRemoval(userId) {
  return request({
    url: `/api/enterprise/members/${userId}/removal-preview`,
    method: "GET",
  });
}

// Hands the default system administrator role to another enterprise member.
export function transferTenantOwner(targetUserId) {
  return request({
    url: "/api/enterprise/owner/transfer",
    method: "POST",
    data: { target_user_id: Number(targetUserId) },
  });
}

export function resetEnterpriseMemberPassword(userId, data) {
  return request({
    url: `/api/enterprise/members/${userId}/password`,
    method: "PUT",
    data,
  });
}

// Enable (status=1) or disable (status=0) an enterprise member.
export function updateEnterpriseMemberStatus(userId, status) {
  return request({
    url: `/api/enterprise/members/${userId}/status`,
    method: "PUT",
    data: { status },
  });
}

// Member operation logs (create/remove/enable/disable), newest first.
export function listMemberLogs(params) {
  return request({
    url: "/api/enterprise/member-logs",
    method: "GET",
    params,
  });
}

// ======================== Enterprise Department API ========================

export function listEnterpriseDepartments() {
  return request({
    url: "/api/enterprise/departments",
    method: "GET",
  });
}

export function createEnterpriseDepartment(data) {
  return request({
    url: "/api/enterprise/departments",
    method: "POST",
    data,
  });
}

export function renameEnterpriseDepartment(departmentId, data) {
  return request({
    url: `/api/enterprise/departments/${departmentId}`,
    method: "PUT",
    data,
  });
}

export function deleteEnterpriseDepartment(departmentId) {
  return request({
    url: `/api/enterprise/departments/${departmentId}`,
    method: "DELETE",
  });
}

// ======================== Enterprise Member Excel Import/Export ========================

function memberTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

export function downloadEnterpriseMemberImportTemplate() {
  return axios.get("/api/enterprise/members/import-template", {
    params: { timezone: memberTimezone() },
    responseType: "blob",
  });
}

export function exportEnterpriseMembers() {
  return axios.get("/api/enterprise/members/export", {
    params: { timezone: memberTimezone() },
    responseType: "blob",
  });
}

export function previewEnterpriseMemberImport(file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: "/api/enterprise/members/import-preview",
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}

export function importEnterpriseMembers(file) {
  const data = new FormData();
  data.append("file", file);
  return request({
    url: "/api/enterprise/members/import",
    method: "POST",
    data,
    headers: { "Content-Type": "multipart/form-data" },
    timeout: 120000,
  });
}

export function listEnterpriseRoles() {
  return request({
    url: "/api/enterprise/roles",
    method: "GET",
  });
}

export function createEnterpriseRole(data) {
  return request({
    url: "/api/enterprise/roles",
    method: "POST",
    data,
  });
}

export function updateEnterpriseRolePermissions(roleId, data) {
  return request({
    url: `/api/enterprise/roles/${roleId}/permissions`,
    method: "PUT",
    data,
  });
}

// Save a role's team work view scope; data = { viewScope, targets: [{ type, id }] }.
export function updateEnterpriseRoleTeamWorkScope(roleId, data) {
  return request({
    url: `/api/enterprise/roles/${roleId}/team-work-scope`,
    method: "PUT",
    data,
  });
}

export function deleteEnterpriseRole(roleId) {
  return request({
    url: `/api/enterprise/roles/${roleId}`,
    method: "DELETE",
  });
}

// Delete the project role (custom roles only, and not used by any member).
export function deleteProjectRole(wsId, roleId) {
  return request({
    url: `/api/workspaces/${wsId}/roles/${roleId}`,
    method: "DELETE",
  });
}

export function listEnterpriseFiles(params) {
  return request({
    url: "/api/enterprise/files",
    method: "GET",
    params,
  });
}

export function downloadEnterpriseFile(fileId) {
  return axios.get(`/api/enterprise/files/${fileId}/download`, {
    responseType: "blob",
  });
}

export function deleteEnterpriseFile(fileId) {
  return request({
    url: `/api/enterprise/files/${fileId}`,
    method: "DELETE",
  });
}

export function batchDeleteEnterpriseFiles(data) {
  return request({
    url: "/api/enterprise/files/batch-delete",
    method: "POST",
    data,
  });
}

// Update basic info of a custom enterprise role; data = { name, description }.
export function updateEnterpriseRole(roleId, data) {
  return request({
    url: `/api/enterprise/roles/${roleId}`,
    method: "PUT",
    data,
  });
}

// Update basic info of a project role; data = { name, description }.
export function updateProjectRole(wsId, roleId, data) {
  return request({
    url: `/api/workspaces/${wsId}/roles/${roleId}`,
    method: "PUT",
    data,
  });
}

// ======================== Permission List API ========================

export function listPermissions(scope, wsId = undefined) {
  return request({
    url: "/api/permissions",
    method: "GET",
    params: { scope, wsId },
  });
}

// ======================== Project Role API ========================

export function listProjectRoles(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/roles`,
    method: "GET",
  });
}

// Create a project role (custom role, is_system=0).
export function createProjectRole(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/roles`,
    method: "POST",
    data,
  });
}

// List active workspaces where the current user can add project roles.
export function listProjectRoleCopyTargets(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/roles/copy-targets`,
    method: "GET",
  });
}

// Copy a custom project role to selected workspaces.
export function copyProjectRole(wsId, roleId, data) {
  return request({
    url: `/api/workspaces/${wsId}/roles/${roleId}/copy`,
    method: "POST",
    data,
  });
}

// Save project role permissions (custom roles only).
export function updateProjectRolePermissions(wsId, roleId, data) {
  return request({
    url: `/api/workspaces/${wsId}/roles/${roleId}/permissions`,
    method: "PUT",
    data,
  });
}

// Save project role list data view scope (custom roles only).
export function updateProjectRoleDataViewScope(wsId, roleId, data) {
  return request({
    url: `/api/workspaces/${wsId}/roles/${roleId}/data-view-scope`,
    method: "PUT",
    data,
  });
}

// ======================== Field Config (Attribute Field Switches) ========================

export function listFieldConfigs(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/field-configs`,
    method: "GET",
    params: { type },
  });
}

// Fetch the project metadata of requirements and defects.
export function getWorkspaceMetadata(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/metadata`,
    method: "GET",
  });
}

export function saveFieldConfigs(wsId, type, items) {
  return request({
    url: `/api/workspaces/${wsId}/field-configs`,
    method: "PUT",
    params: { type },
    data: items,
  });
}

// Restore the default enabled state and order of built-in fields.
export function resetFieldConfigs(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/field-configs/reset`,
    method: "PUT",
    params: { type },
  });
}

export function saveFieldOrder(wsId, type, items) {
  return request({
    url: `/api/workspaces/${wsId}/fields/order`,
    method: "PUT",
    params: { type },
    data: { items },
  });
}

// ======================== Custom Field CRUD ========================

export function listCustomFields(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/custom-fields`,
    method: "GET",
    params: { type },
  });
}

/**
 * List the minimal enterprise role options available to member custom fields.
 * @param {number|string} wsId - Workspace ID used for project-scoped permission checks.
 * @param {string} type - Work item type whose field-setting permission is required.
 * @returns {Promise<Array<{id: number, name: string}>>}
 */
export function listCustomFieldEnterpriseRoles(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/custom-fields/enterprise-roles`,
    method: "GET",
    params: { type },
  });
}

export function createCustomField(wsId, type, data) {
  return request({
    url: `/api/workspaces/${wsId}/custom-fields`,
    method: "POST",
    params: { type },
    data,
  });
}

export function updateCustomField(wsId, type, fieldId, data) {
  return request({
    url: `/api/workspaces/${wsId}/custom-fields/${fieldId}`,
    method: "PUT",
    params: { type },
    data,
  });
}

export function deleteCustomField(wsId, type, fieldId) {
  return request({
    url: `/api/workspaces/${wsId}/custom-fields/${fieldId}`,
    method: "DELETE",
    params: { type },
  });
}

// ======================== Status CRUD ========================

export function listStatuses(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/statuses`,
    method: "GET",
    params: { type },
  });
}

export function createStatus(wsId, type, data) {
  return request({
    url: `/api/workspaces/${wsId}/statuses`,
    method: "POST",
    params: { type },
    data,
  });
}

export function updateStatus(wsId, type, statusId, data) {
  return request({
    url: `/api/workspaces/${wsId}/statuses/${statusId}`,
    method: "PUT",
    params: { type },
    data,
  });
}

export function deleteStatus(wsId, type, statusId) {
  return request({
    url: `/api/workspaces/${wsId}/statuses/${statusId}`,
    method: "DELETE",
    params: { type },
  });
}

export function resetStatuses(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/statuses/reset`,
    method: "PUT",
    params: { type },
  });
}

// ======================== Start/End Status Configuration ========================

export function getWorkspaceSettings(wsId) {
  return request({
    url: `/api/workspaces/${wsId}/settings`,
    method: "GET",
  });
}

export function updateWorkspaceSettings(wsId, data) {
  return request({
    url: `/api/workspaces/${wsId}/settings`,
    method: "PUT",
    data,
  });
}

// ======================== Priority CRUD ========================

export function listPriorities(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/priorities`,
    method: "GET",
    params: { type },
  });
}

export function createPriority(wsId, type, data) {
  return request({
    url: `/api/workspaces/${wsId}/priorities`,
    method: "POST",
    params: { type },
    data,
  });
}

export function updatePriority(wsId, type, priorityId, data) {
  return request({
    url: `/api/workspaces/${wsId}/priorities/${priorityId}`,
    method: "PUT",
    params: { type },
    data,
  });
}

export function updateDefaultPriority(wsId, type, priorityId) {
  return request({
    url: `/api/workspaces/${wsId}/priorities/default`,
    method: "PUT",
    params: { type },
    data: { priority_id: priorityId },
  });
}

export function deletePriority(wsId, type, priorityId) {
  return request({
    url: `/api/workspaces/${wsId}/priorities/${priorityId}`,
    method: "DELETE",
    params: { type },
  });
}

export function resetPriorities(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/priorities/reset`,
    method: "PUT",
    params: { type },
  });
}

/** List templates linked to the workspace, plus the feature switch. */
export function listWorkItemTemplates(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates`,
    method: "GET",
    params: { type },
  });
}

/** Persist the per-type template feature switch. */
export function updateWorkItemTemplateSettings(wsId, type, enabled) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/settings`,
    method: "PUT",
    params: { type },
    data: { enabled },
  });
}

export function getWorkItemDisplaySettings(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/display-settings`,
    method: "GET",
    params: { type },
  });
}

export function updateWorkItemDisplaySettings(wsId, type, data) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/display-settings`,
    method: "PUT",
    params: { type },
    data,
  });
}

/** Create a pool template and link it to the workspace. */
export function createWorkItemTemplate(wsId, type, data) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates`,
    method: "POST",
    params: { type },
    data,
  });
}

/** Update a linked pool template. */
export function updateWorkItemTemplate(wsId, type, templateId, data) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/${templateId}`,
    method: "PUT",
    params: { type },
    data,
  });
}

/** Delete a template from this workspace and remove its orphaned pool copy. */
export function deleteWorkItemTemplate(wsId, type, templateId) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/${templateId}`,
    method: "DELETE",
    params: { type },
  });
}

/** Unlink a template from the workspace without deleting the pool copy. */
export function unlinkWorkItemTemplate(wsId, type, templateId) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/${templateId}/link`,
    method: "DELETE",
    params: { type },
  });
}

/** List tenant pool templates not yet linked to this workspace. */
export function listWorkItemTemplatePool(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/pool`,
    method: "GET",
    params: { type },
  });
}

/** Link existing pool templates to the workspace. */
export function linkWorkItemTemplates(wsId, type, ids) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/link`,
    method: "POST",
    params: { type },
    data: { ids },
  });
}

/** Delete a pool template that is not linked to any workspace. */
export function deleteWorkItemTemplatePool(wsId, type, templateId) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-templates/pool/${templateId}`,
    method: "DELETE",
    params: { type },
  });
}

/** Get configured work item type attributes (name/icon) of a workspace; pass type to filter one type. */
export function getWorkItemAttributes(wsId, type) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-attributes`,
    method: "GET",
    params: type ? { type } : {},
  });
}

/** Save (upsert) one work item type's attributes config of a workspace. */
export function saveWorkItemAttribute(wsId, type, data) {
  return request({
    url: `/api/workspaces/${wsId}/work-item-attributes`,
    method: "PUT",
    params: { type },
    data,
  });
}

export function listNotificationUserMappings() {
  return request({
    url: "/api/enterprise/external-notification/user-mappings",
    method: "GET",
  });
}
export function saveNotificationUserMappings(items) {
  return request({
    url: "/api/enterprise/external-notification/user-mappings",
    method: "PUT",
    data: { items },
  });
}
