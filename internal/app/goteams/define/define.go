// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

import (
	"fmt"
	"net/http"

	"github.com/go-redis/redis/v8"
)

// AppRoot is the project root directory (set at runtime).
var AppRoot = `./`

// IsDev indicates whether the current environment is development.
var IsDev = true

// IsLoc indicates whether running on a local development machine (Windows/Mac).
var IsLoc = false

// Config configuration mapping struct.
type ConfigStruct struct {
	WebService map[string]string
	Session    map[string]string
	Redis      map[string]string
	Postgres   map[string]string
	OssS3      map[string]string
	// SessionEncryption holds the [session-encryption] section (master_key etc.) for
	// envelope encryption of client-reported session logs (需求 1896 优化点3).
	SessionEncryption map[string]string
	OpenApi           map[string]string
	Email             map[string]string
	Client            map[string]string
	Template          map[string]string
	// Oauth holds the shared config of third-party scan-code login (site URL, state/ticket TTL).
	Oauth map[string]string
	// Dingtalk holds the DingTalk self-built app credentials used by scan-code login.
	Dingtalk map[string]string
	// Feishu holds the Feishu self-built app credentials used by scan-code login.
	Feishu map[string]string
	// Scheduler 定时任务基础组件的运行参数（并发上限）。
	Scheduler map[string]string
	// Sso 企业SSO的部署级参数（入口加密 key、SSRF 放行网段）。
	Sso map[string]string
}

var Config ConfigStruct

// Redis global Redis client.
var Redis *redis.Client

// WebService HTTP server instance.
var WebService *http.Server

// Database connection identifier (used for msql registration).
const Postgres = `postgres`

// Database table name constants.
const (
	TableUsers                  = `users`
	TableWorkspaces             = `workspaces`
	TableWorkspaceTypes         = `workspace_types`
	TableWorkspaceMembers       = `workspace_members`
	TableRequirements           = `requirements`
	TableRequirementCategories  = `requirement_categories`
	TableRequirementComments    = `requirement_comments`
	TableRequirementWorkLogs    = `requirement_work_logs`
	TableVersions               = `versions`
	TableVersionWorkspaces      = `version_workspaces`
	TableRoles                  = `roles`
	TableRolePermissions        = `role_permissions`
	TableRoleTeamWorkMembers    = `role_team_work_members`
	TableRoleDataViewMembers    = `role_data_view_members`
	TableUserTeamWorkSettings   = `user_team_work_member_settings`
	TableUserContactPreferences = `user_contact_preferences`
	TableWorkItemStatuses       = `workspace_statuses`
	TableWorkItemPriorities     = `workspace_priorities`
	TableWorkItemFieldConfigs   = `workspace_field_configs`
	TableCustomFields           = `workspace_custom_fields`
	TableCustomFieldValues      = `requirement_custom_field_values`
	TableTenantPlans            = `tenant_plans`
	TableApiTokens              = `api_tokens`
	// TableApiAdminDailyQuotas counts a tenant's API calls per day; it enforces the daily plan quota.
	TableApiAdminDailyQuotas = `api_admin_daily_quotas`
	// TableApiTokenDailyStats counts one token's API calls per day; it backs the today/yesterday counters.
	TableApiTokenDailyStats   = `api_token_daily_stats`
	TableMemberInvitations    = `member_invitations`
	TableDepartments          = `departments`
	TableUserDepartments      = `user_departments`
	TableRequirementHistories = `requirement_histories`
	// TableEnterpriseFilePrefix is the prefix of the enterprise file ledger sharding tables (enterprise_files_0 ~ 9).
	TableEnterpriseFilePrefix                = `enterprise_files_`
	TableAutomationRules                     = `automation_rules`
	TableAutomationExecutions                = `automation_executions`
	TableExternalNotificationStrategies      = `external_notification_strategies`
	TableExternalNotificationStrategyMembers = `external_notification_strategy_members`
	// TableRequirementVisits keeps one row per user per work item holding the latest visit time.
	TableRequirementVisits = `requirement_visits`
	// TableRemovedUsers archives removed members' display info so historical records still show their name.
	TableRemovedUsers = `removed_users`
	// TableMemberLogs stores enterprise member operation logs (create/remove/enable/disable).
	TableMemberLogs = `member_logs`
	// TableWorkspaceSettings stores per-workspace initial/final status config for requirements and defects.
	TableWorkspaceSettings = `workspaces_settings`
	// TableWorkItemTemplates stores tenant-level work-item templates reused across workspaces.
	TableWorkItemTemplates = `work_item_templates`
	// TableWorkspaceWorkItemTemplates stores workspace associations to pool templates.
	TableWorkspaceWorkItemTemplates = `workspace_work_item_templates`
	// TableWorkItemDisplaySettings stores per-workspace edit/view field display rules.
	TableWorkItemDisplaySettings = `workspace_work_item_display_settings`
	// TableWorkItemAttributes stores per-workspace work item display attributes (name/icon/description) by type.
	TableWorkItemAttributes = `workspace_work_item_attributes`

	// TableWorkspaceViews stores requirement/defect list view definitions.
	TableWorkspaceViews = `workspace_views`
	// TableUserViewSettings stores per-user view preferences (default view, enabled set, order, remember switch).
	TableUserViewSettings = `user_view_settings`
	// TableNotifications stores per-user in-app notifications.
	TableNotifications           = `notifications`
	TableWorkspaceAccessRequests = `workspace_access_requests`
	// TableWorkspaceNavigationSettings stores the project-wide top navigation policy.
	TableWorkspaceNavigationSettings = `workspace_navigation_settings`
	// TableUserWorkspaceNavigationSettings stores one member's navigation override.
	TableUserWorkspaceNavigationSettings = `user_workspace_navigation_settings`
	// TableUserWorkspaceOrderSettings stores per-account project sidebar order overrides.
	TableUserWorkspaceOrderSettings = `user_workspace_order_settings`

	// TablePlanningGroups stores planning group definitions (top tabs on the board planning page).
	TablePlanningGroups = `planning_groups`
	// TablePlanningRelations 规划与需求/缺陷的关联表。
	TablePlanningRelations = `planning_relations`
	// TableUserOauthBindings stores the binding between a GoTeams user and a DingTalk/Feishu account.
	TableUserOauthBindings = `user_oauth_bindings`
	// TableSsoConfigs 企业SSO登录配置，一个租户一行，同一时刻只有一个平台生效。
	TableSsoConfigs = `sso_configs`
	// TableSsoSyncConfigs 成员账号同步配置，与登录配置分开存储。
	TableSsoSyncConfigs = `sso_sync_configs`
	// TableUserSsoBindings 企业SSO身份绑定，把IdP身份映射到GoTeams成员。
	TableUserSsoBindings = `user_sso_bindings`
	// TableDepartmentSsoBindings 企业SSO部门身份关联，把IdP部门映射到本地部门。
	TableDepartmentSsoBindings = `department_sso_bindings`
	// TableScheduledTasks 通用定时任务表；租约列用来保证同一任务不被并发执行。
	TableScheduledTasks = `scheduled_tasks`
	// TenantRequirementTotalsTaskKey 租户需求、缺陷总数的全局汇总任务类型。
	TenantRequirementTotalsTaskKey = `tenant_requirement_totals`
)

// Pipeline-related database table name constants.
const (
	TableClientDevice        = `client_devices`
	TablePipeline            = `pipelines`
	TablePipelineMember      = `pipeline_members`
	TablePipelineDepartment  = `pipeline_departments`
	TablePipelineStep        = `pipeline_steps`
	TablePipelineTask        = `pipeline_tasks`
	TablePipelineTaskStep    = `pipeline_task_steps`
	TablePipelineTaskSession = `pipeline_task_sessions`
	// TablePipelineTaskDocument 存产出文档元数据，文件本体在对象存储。
	TablePipelineTaskDocument = `pipeline_task_documents`
	// TablePipelineDocumentUploadJob 是产出文档的异步上传队列。
	TablePipelineDocumentUploadJob = `pipeline_document_upload_jobs`
)

// Expert-group-related database table name constants.
const (
	TableExpertGroup           = `expert_groups`
	TableExpertGroupAgent      = `expert_group_agents`
	TableExpertGroupUser       = `expert_group_users`
	TableExpertGroupDepartment = `expert_group_departments`
)

// Work item and list context type key constants.
const (
	TypeKeyRequirement = "requirement"
	TypeKeyDefect      = "defect"
	TypeKeyPlanning    = "planning"
	// TypeKeyGantt is a view-module key only (gantt views mix both work item types).
	TypeKeyGantt     = "gantt"
	TypeKeyIteration = "iteration"

	SourceUser       = "user"
	SourceAutomation = "automation"
)

// 工作项类型在通知文案里的显示名。通知文案直接拼中文、不走 i18n，类型名集中在此，避免各处硬编码。
const (
	// TypeNameRequirement 是需求在通知文案里的显示名。
	TypeNameRequirement = `需求`
	// TypeNameDefect 是缺陷在通知文案里的显示名。
	TypeNameDefect = `缺陷`
)

// Third-party login provider keys; they are also the user_oauth_bindings.provider values.
const (
	OauthProviderDingtalk = "dingtalk"
	OauthProviderFeishu   = "feishu"
)

// Third-party login scenes; login comes from the sign-in page, bind comes from personal settings.
const (
	OauthSceneLogin = "login"
	OauthSceneBind  = "bind"
)

// OauthCallbackPath returns the third-party callback path of one provider.
// It must stay identical to the redirect URI registered in the provider's console.
func OauthCallbackPath(provider string) string {
	return `/api/auth/` + provider + `/callback`
}

// Error reasons carried back to the frontend when a third-party callback fails.
const (
	OauthErrorProviderDisabled = "provider_disabled"
	OauthErrorStateInvalid     = "state_invalid"
	OauthErrorCodeInvalid      = "code_invalid"
	OauthErrorTicketInvalid    = "ticket_invalid"
	OauthErrorBoundByOther     = "bound_by_other"
	OauthErrorServer           = "server_error"
)

// Iteration type constants.
const (
	IterationTypeSingle = "single"
	IterationTypeCross  = "cross"
)

// Work item description editor mode constants.
const (
	DescriptionModeRichText = "rich-text"
	DescriptionModeMarkdown = "markdown"
)

// Work item status status key constants.
const (
	StatusPendingPlan   = "pending_plan"
	StatusPendingAssign = "pending_assign"
	StatusInProgress    = "in_progress"
	StatusDone          = "done"
	StatusClosed        = "closed"
	StatusRejected      = "rejected"
)

// Planning-specific status keys. planning_closed intentionally does not reuse StatusClosed,
// to avoid hitting the seed-logic branch that treats StatusClosed as a terminal state.
const (
	StatusPlanned        = "planned"
	StatusDesigned       = "designed"
	StatusImplemented    = "implemented"
	StatusAccepted       = "accepted"
	StatusPlanningClosed = "planning_closed"
)

// Planning sort gap: sparse integer ordering; re-sort the whole group when the gap is exhausted.
const PlanningSortGap = 65536

// API unified response struct.
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Common success/error codes.
const (
	CodeSuccess = 200
	CodeError   = 500
)

// History action key constants.
const (
	HistoryActionStatusChanged = "status_change"
	HistoryActionFieldUpdated  = "field_update"
	HistoryActionAssignChanged = "assignee_change"
	HistoryActionCreated       = "created"
	// HistoryActionRestored marks a change produced by restoring the description to a historical version.
	HistoryActionRestored = "restore"
	// HistoryActionIssueRecall marks the status change written when the public submitter recalls an issue.
	HistoryActionIssueRecall = "issue_recall"
)

// Description restore target side constants.
const (
	// HistorySideOld restores the value stored before the recorded change.
	HistorySideOld = "old"
	// HistorySideNew restores the value stored after the recorded change.
	HistorySideNew = "new"
)

// Enterprise member account status constants (users.status).
const (
	MemberStatusEnabled  = 1
	MemberStatusDisabled = 0
)

// Enterprise member log action constants (member_logs.action).
const (
	MemberLogActionCreate  = "create"
	MemberLogActionRemove  = "remove"
	MemberLogActionEnable  = "enable"
	MemberLogActionDisable = "disable"
)

// MsgStatusTransitionFieldRequired is the status-transition required-field message key.
const MsgStatusTransitionFieldRequired = "workitem_status_transition_field_required"

// Notification event type constants.
const (
	NotificationEventItemUpdated       = `item_updated`
	NotificationEventItemAssigned      = `item_assigned`
	NotificationEventCommentReplied    = `comment_replied`
	NotificationEventAssigneeCommented = `assignee_commented`
	NotificationEventMention           = `mention`
	NotificationEventSlaOverdue        = `sla_overdue`
	NotificationEventSlaWarning        = `sla_warning`
	// NotificationEventIssueInviteUrged is a public submitter urging the assignees of their issue.
	NotificationEventIssueInviteUrged         = `issue_invite_urged`
	NotificationEventWorkspaceAccessRequested = `workspace_access_requested`
	NotificationEventWorkspaceMemberAdded     = `workspace_member_added`
)

// Common error message keys (corresponding to i18n language packs).
const (
	MsgRequirementWorkLogNotFound   = "requirement_work_log_not_found"
	MsgRequirementWorkLogDateExists = "requirement_work_log_date_exists"
	MsgRequirementWorkHoursInvalid  = "requirement_work_hours_invalid"
	MsgSuccess                      = "common_success"
	MsgError                        = "common_error"
	MsgUnauthorized                 = "auth_unauthorized"
	MsgForbidden                    = "auth_forbidden"
	MsgTokenExpired                 = "auth_token_expired"
	MsgTokenInvalid                 = "auth_token_invalid"
	MsgClientLoginSessionNotFound   = "client_login_session_not_found"
	MsgClientLoginTicketInvalid     = "client_login_ticket_invalid"
	MsgClientLoginRedirectInvalid   = "client_login_redirect_invalid"
	MsgUserNotFound                 = "user_not_found"
	MsgUserExist                    = "user_exist"
	MsgEmailExists                  = "email_exists"
	MsgPasswordWrong                = "user_password_wrong"
	MsgContactFavoriteLimit         = "contact_favorite_limit"
	MsgContactInvalid               = "contact_invalid"
	MsgLoginWrong                   = "user_login_wrong"
	MsgAccountDisabled              = "user_account_disabled"
	MsgAvatarInvalid                = "user_avatar_invalid"
	MsgAvatarTooLarge               = "user_avatar_too_large"
	MsgAvatarNotFound               = "user_avatar_not_found"
	MsgWorkspaceNotFound            = "workspace_not_found"
	MsgWorkspaceAccessPending       = "workspace_access_pending"
	MsgWorkspaceAccessAlreadyMember = "workspace_access_already_member"
	MsgWorkspaceSlugExist           = "workspace_slug_exist"
	MsgWorkspaceIconInvalid         = "workspace_icon_invalid"
	MsgWorkspaceIconTooLarge        = "workspace_icon_too_large"
	MsgWorkspaceIconNotFound        = "workspace_icon_not_found"
	MsgProjectRoleNameExist         = "project_role_name_exist"
	MsgVersionNotFound              = "version_not_found"
	MsgVersionEnded                 = "version_ended"
	MsgPlanningNotFound             = "planning_not_found"
	MsgPlanningGroupNotFound        = "planning_group_not_found"
	MsgPlanningGroupLastOne         = "planning_group_last_one"
	MsgPlanningRelationInvalid      = "planning_relation_invalid"
	MsgVersionNameExist             = "version_name_exist"
	MsgWorkspaceMemberExist         = "member_already_exist"
	MsgWorkspaceRoleNotFound        = "member_role_not_found"
	MsgRequirementNotFound          = "requirement_not_found"
	MsgParamInvalid                 = "common_param_invalid"
	MsgOwnerTransferTargetInvalid   = "owner_transfer_target_invalid"
	MsgProjectOwnerProtected        = "project_owner_protected"
	MsgFeatureDisabled              = "common_feature_disabled"
	MsgStatusNotFound               = "workitem_status_not_found"
	MsgPriorityNotFound             = "workitem_priority_not_found"
	MsgStatusTransitionInvalid      = "workitem_status_transition_invalid"
	MsgCustomFieldRequired          = "workitem_custom_field_required"
	MsgCustomFieldUnknown           = "workitem_custom_field_unknown"
	MsgCustomFieldOptionInvalid     = "workitem_custom_field_option_invalid"
	MsgRequirementHistoryCreated    = "requirement_history_created"
	MsgRequirementHistoryNotFound   = "requirement_history_not_found"
	MsgRequirementHistoryInvalid    = "requirement_history_not_restorable"
	MsgRequirementHistoryRestored   = "requirement_history_restored"
	MsgRequirementRelationInvalid   = "requirement_relation_invalid"
	MsgPlannedDateRangeInvalid      = "workitem_planned_date_range_invalid"
	MsgStorageExtendSuccess         = "storage_extend_success"
	MsgStorageExtendOnlyOnce        = "storage_extend_only_once"
	MsgStorageExtendNotOverLimit    = "storage_extend_not_over_limit"
	MsgWorkItemTypeImmutable        = "workitem_type_immutable"
	MsgCommentPublicUnavailable     = "comment_public_unavailable"
	MsgCommentNotFound              = "comment_not_found"
	MsgCommentOwnerRequired         = "comment_owner_required"
	MsgCommentParentInvalid         = "comment_parent_invalid"
	MsgAutomationRuleNotFound       = "automation_rule_not_found"
	MsgAutomationExecutionNotFound  = "automation_execution_not_found"
	MsgAutomationRuleNameExists     = "automation_rule_name_exists"
	MsgAutomationConfigInvalid      = "automation_config_invalid"
	MsgAttachmentNotFound           = "attachment_not_found"
	MsgAttachmentTooLarge           = "attachment_too_large"
	MsgRouteNotFound                = "common_route_not_found"
	MsgMethodNotAllowed             = "common_method_not_allowed"
	MsgNotFound                     = "common_not_found"
	MsgServerError                  = "common_server_error"
	MsgImageRequired                = "image_required"
	MsgImageTypeInvalid             = "image_type_invalid"
	MsgImageTooLarge                = "image_too_large"
	MsgQuotaExceeded                = "quota_exceeded"
	MsgOpenApiUnauthorized          = "open_api_unauthorized"
	MsgEmailSendFailed              = "email_send_failed"
	MsgVerifyCodeInvalid            = "verify_code_invalid"
	MsgVerifyCodeSent               = "verify_code_sent"
	MsgFileRequired                 = "file_required"
	MsgFileTypeInvalid              = "file_type_invalid"
	MsgFileTooLarge                 = "file_too_large"
	MsgVideoRequired                = "video_required"
	MsgVideoTypeInvalid             = "video_type_invalid"
	MsgVideoTooLarge                = "video_too_large"
	MsgInvitationInvalid            = "invitation_invalid"

	// API token related message keys.
	MsgApiTokenInvalid       = "api_token_invalid"
	MsgApiTokenDisabled      = "api_token_disabled"
	MsgApiTokenOwnerInactive = "api_token_owner_inactive"
	MsgApiTokenNotFound      = "api_token_not_found"
	MsgApiTokenNameInvalid   = "api_token_name_invalid"
	MsgApiTokenLimit         = "api_token_limit"
	MsgApiTokenSkillLinkGone = "api_token_skill_link_gone"
	MsgApiQuotaExceeded      = "api_quota_exceeded"
	MsgApiPermissionDenied   = "api_permission_denied"
	MsgApiNotInWorkspace     = "api_not_in_workspace"
	MsgApiIdInvalid          = "api_id_invalid"
	MsgApiCustomFieldInvalid = "api_custom_field_invalid"

	// Department related message keys.
	MsgDepartmentNotFound      = "department_not_found"
	MsgDepartmentNameExists    = "department_name_exists"
	MsgDepartmentDepthExceeded = "department_depth_exceeded"
	MsgDepartmentInvalid       = "department_invalid"

	// Pipeline related message keys.
	MsgPipelineNameRequired            = "pipeline_name_required"
	MsgPipelineWorkflowInvalid         = "pipeline_workflow_invalid"
	MsgPipelineNotFound                = "pipeline_not_found"
	MsgPipelineManageForbidden         = "pipeline_manage_forbidden"
	MsgPipelineUseForbidden            = "pipeline_use_forbidden"
	MsgPipelineWorkflowVersionConflict = "pipeline_workflow_version_conflict"
	MsgPipelineTaskNotFound            = "pipeline_task_not_found"
	MsgPipelineIconInvalid             = "pipeline_icon_invalid"
	MsgPipelineIconTooLarge            = "pipeline_icon_too_large"
	MsgPipelineIconNotFound            = "pipeline_icon_not_found"
	MsgPipelineMemberInvalid           = "pipeline_member_invalid"
	MsgPipelineScopeInvalid            = "pipeline_scope_invalid"
	MsgPipelineDepartmentInvalid       = "pipeline_department_invalid"
	MsgPipelineStepPromptRequired      = "pipeline_step_prompt_required"
	MsgPipelineDocumentNotFound        = "pipeline_document_not_found"
	MsgPipelineDocumentTooLarge        = "pipeline_document_too_large"
	MsgPipelineDocumentTypeInvalid     = "pipeline_document_type_invalid"
	MsgPipelineDocumentStorageFailed   = "pipeline_document_storage_failed"
	MsgPipelineDocumentBusy            = "pipeline_document_busy"

	// Expert group related message keys.
	MsgExpertGroupNameRequired        = "expert_group_name_required"
	MsgExpertGroupNotFound            = "expert_group_not_found"
	MsgExpertGroupScopeInvalid        = "expert_group_scope_invalid"
	MsgExpertGroupMemberInvalid       = "expert_group_member_invalid"
	MsgExpertGroupDepartmentInvalid   = "expert_group_department_invalid"
	MsgExpertGroupNoLeader            = "expert_group_no_leader"
	MsgExpertGroupNoAgent             = "expert_group_no_agent"
	MsgExpertGroupManageForbidden     = "expert_group_manage_forbidden"
	MsgExpertGroupUseForbidden        = "expert_group_use_forbidden"
	MsgExpertGroupAvatarInvalid       = "expert_group_avatar_invalid"
	MsgExpertGroupAvatarTooLarge      = "expert_group_avatar_too_large"
	MsgExpertGroupAgentPromptRequired = "expert_group_agent_prompt_required"

	// Client related message keys.
	MsgClientDeviceCredentialInvalid = "client_device_credential_invalid"
	MsgClientDeviceRevoked           = "client_device_revoked"
	MsgClientTenantMismatch          = "client_tenant_mismatch"
	MsgClientProjectionHashInvalid   = "client_projection_hash_invalid"
	MsgClientProtocolMismatch        = "client_protocol_mismatch"

	MsgWorkItemTemplateNotFound     = "workitem_template_not_found"
	MsgWorkItemTemplateInUse        = "workitem_template_in_use"
	MsgWorkItemTemplateNameInvalid  = "workitem_template_name_invalid"
	MsgWorkItemTemplateFieldInvalid = "workitem_template_field_invalid"

	// Third-party (OAuth) scan-code login message keys.
	// Callback failures are redirected to the frontend with a reason code instead of a JSON
	// message, so only the states returned by the JSON endpoints need keys here.
	MsgOauthProviderDisabled = "oauth_provider_disabled"
	MsgOauthProviderUnknown  = "oauth_provider_unknown"
	MsgOauthTicketInvalid    = "oauth_ticket_invalid"
	MsgOauthBoundByOther     = "oauth_bound_by_other"
	MsgOauthAlreadyBound     = "oauth_already_bound"
	MsgOauthNotBound         = "oauth_not_bound"
)

// Custom field validation message keys.
const (
	// MsgCustomFieldNameRequired identifies a missing custom field name.
	MsgCustomFieldNameRequired = "custom_field_name_required"
	// MsgCustomFieldNameTooLong identifies a custom field name exceeding its limit.
	MsgCustomFieldNameTooLong = "custom_field_name_too_long"
	// MsgCustomFieldTypeInvalid identifies an unsupported custom field type.
	MsgCustomFieldTypeInvalid = "custom_field_type_invalid"
	// MsgCustomFieldOptionsRequired identifies a choice field without options.
	MsgCustomFieldOptionsRequired = "custom_field_options_required"
	// MsgCustomFieldOptionKeyInvalid identifies an unknown or duplicated option identity.
	MsgCustomFieldOptionKeyInvalid = "custom_field_option_key_invalid"
	// MsgCustomFieldMemberRolesRequired identifies a role-scoped member field without roles.
	MsgCustomFieldMemberRolesRequired = "custom_field_member_roles_required"
	// MsgCustomFieldMemberRoleInvalid identifies a missing enterprise role in member field config.
	MsgCustomFieldMemberRoleInvalid = "custom_field_member_role_invalid"
	// MsgCustomFieldMemberScopeInvalid identifies an unsupported member selection scope.
	MsgCustomFieldMemberScopeInvalid = "custom_field_member_scope_invalid"
	// MsgCustomFieldCascaderRequired identifies a cascader field without nodes.
	MsgCustomFieldCascaderRequired = "custom_field_cascader_required"
	// MsgCustomFieldCascaderDepthInvalid identifies a cascader exceeding its depth limit.
	MsgCustomFieldCascaderDepthInvalid = "custom_field_cascader_depth_invalid"
	// MsgCustomFieldCascaderNameRequired identifies a cascader node without a name.
	MsgCustomFieldCascaderNameRequired = "custom_field_cascader_name_required"
	// MsgCustomFieldCascaderNameTooLong identifies a cascader node name exceeding its limit.
	MsgCustomFieldCascaderNameTooLong = "custom_field_cascader_name_too_long"
	// MsgCustomFieldCascaderSizeInvalid identifies a cascader exceeding its node limit.
	MsgCustomFieldCascaderSizeInvalid = "custom_field_cascader_size_invalid"
	// MsgCustomFieldValueInvalid identifies a malformed custom field value.
	MsgCustomFieldValueInvalid = "workitem_custom_field_value_invalid"
	// MsgCustomFieldPhoneInvalid identifies an invalid phone custom field value.
	MsgCustomFieldPhoneInvalid = "workitem_custom_field_phone_invalid"
	// MsgCustomFieldNumberInvalid identifies an invalid numeric custom field value.
	MsgCustomFieldNumberInvalid = "workitem_custom_field_number_invalid"
	// MsgCustomFieldIntegerRequired identifies a non-integer value for an integer-only field.
	MsgCustomFieldIntegerRequired = "workitem_custom_field_integer_required"
	// MsgCustomFieldDateInvalid identifies an invalid date custom field value.
	MsgCustomFieldDateInvalid = "workitem_custom_field_date_invalid"
	// MsgCustomFieldDateTimeInvalid identifies an invalid date-time custom field value.
	MsgCustomFieldDateTimeInvalid = "workitem_custom_field_datetime_invalid"
	// MsgCustomFieldMemberInvalid identifies an invalid member custom field value.
	MsgCustomFieldMemberInvalid = "workitem_custom_field_member_invalid"
)

// Requirement category message keys.
const (
	MsgRequirementCategoryAssignmentInvalid = "requirement_category_assignment_invalid"
	MsgRequirementCategoryNotFound          = "requirement_category_not_found"
	MsgRequirementCategoryNameExists        = "requirement_category_name_exists"
	MsgRequirementCategoryTreeInvalid       = "requirement_category_tree_invalid"
)

// Pipeline execution status constants.
const (
	ExecStatusIdle    = `idle`
	ExecStatusRunning = `running`
	ExecStatusSuccess = `success`
	ExecStatusFailed  = `failed`
	ExecStatusStopped = `stopped`
)

// 套餐类型。
const (
	PlanExperience = `experience`
	PlanEnterprise = `enterprise`
)

// 配额键。
const (
	QuotaMember         = `member`
	QuotaProject        = `project`
	QuotaCustomField    = `custom_field`
	QuotaAutomationRule = `automation_rule`
	QuotaAutomationExec = `automation_exec`
	QuotaAgent          = `agent`
	QuotaStorage        = `storage`
	QuotaApiCall        = `api_call`
)

// EnterpriseFileShardCount is the number of enterprise file ledger sharding tables.
const EnterpriseFileShardCount = 10

// GetEnterpriseFileTable returns the sharding table name for the given tenant (admin_id % 10).
func GetEnterpriseFileTable(adminID int) string {
	return TableEnterpriseFilePrefix + fmt.Sprintf(`%d`, adminID%EnterpriseFileShardCount)
}

// AllEnterpriseFileTables returns all enterprise file sharding table names.
func AllEnterpriseFileTables() []string {
	tables := make([]string, 0, EnterpriseFileShardCount)
	for i := 0; i < EnterpriseFileShardCount; i++ {
		tables = append(tables, TableEnterpriseFilePrefix+fmt.Sprintf(`%d`, i))
	}
	return tables
}
