// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// Issue invitation table names.
const (
	TableWorkspaceIssueInvites       = `workspace_issue_invites`
	TableWorkspaceIssueItemTemplates = `workspace_issue_item_templates`
	TableWorkspaceIssueRequirements  = `workspace_issue_requirements`
)

// Issue invitation domain constants.
const (
	AppKeyIssue                 = `issue`
	IssueInviteKeyLength        = 16
	IssueInviteGeneratedOpenID  = 32
	IssueInviteOpenIDMaxLength  = 128
	IssueInvitePublicPageSize   = 10
	IssueInviteCacheTTLSeconds  = int64(24 * 60 * 60)
	IssueInviteKeyRetryMaxCount = 5
	// IssueInviteSubmitRateLimit is the maximum public submissions per submitter per minute.
	IssueInviteSubmitRateLimit = 5
	// IssueInviteGuideTitleMaxLength is the maximum guide-title length in characters.
	IssueInviteGuideTitleMaxLength = 20
	// IssueInviteGuideDescriptionMaxLength is the maximum guide-description length in characters.
	IssueInviteGuideDescriptionMaxLength = 300
	// IssueInviteThemeImageMaxBytes is the maximum stored theme-image URL length in bytes.
	IssueInviteThemeImageMaxBytes = 255
	// IssueInviteCommentMaxLength is the maximum public comment content length in characters.
	IssueInviteCommentMaxLength = 2000
	// IssueInviteUrgeIntervalSeconds 是催单的最短间隔：提交后要等这么久，两次催单之间也要隔这么久。
	IssueInviteUrgeIntervalSeconds = int64(30 * 60)
)

// Issue invitation comment source and handle-record type constants.
const (
	// IssueInviteCommentSource marks requirement_comments rows posted from the public issue detail page.
	IssueInviteCommentSource = `issue_invite`
	// IssueInviteRecordTypeComment is a public submitter comment in the handle-records feed.
	IssueInviteRecordTypeComment = `comment`
	// IssueInviteRecordTypeReply is an internal reply to one of the submitter's comments.
	IssueInviteRecordTypeReply = `reply`
	// IssueInviteRecordTypeAt is an internal comment that mentions the submitter.
	IssueInviteRecordTypeAt = `at`
	// IssueInviteRecordTypeCreate is the submitter's issue-creation entry in the handle-records feed.
	IssueInviteRecordTypeCreate = `create`
	// IssueInviteRecordTypeStatusChange is a status-change history entry in the handle-records feed.
	IssueInviteRecordTypeStatusChange = `status_change`
	// IssueInviteRecordTypeRecall 是用户撤回在办理记录里的一条。
	IssueInviteRecordTypeRecall = `recall`
)

// Issue invitation permission keys.
const (
	PermissionTicketCopyLink = `ticket:copy_link`
	PermissionTicketManage   = `ticket:manage`
)

// Issue invitation message keys.
const (
	MsgIssueInviteAppDisabled      = "issue_invite_app_disabled"
	MsgIssueInviteInvalid          = "issue_invite_invalid"
	MsgIssueInviteKeyInvalid       = "issue_invite_key_invalid"
	MsgIssueInviteOpenIDInvalid    = "issue_invite_openid_invalid"
	MsgIssueInviteTemplateNotFound = "issue_invite_template_not_found"
	MsgIssueInviteTemplateDisabled = "issue_invite_template_disabled"
	MsgIssueInviteFieldInvalid     = "issue_invite_field_invalid"
	// 公开表单的字段校验提示：站内文案带「自定义字段」这类平台术语，外部填写人不该看到，
	// 所以按「错误原因（字段名）」另开一套。
	MsgIssueInviteFieldRequired           = "issue_invite_field_required"
	MsgIssueInviteFieldValueInvalid       = "issue_invite_field_value_invalid"
	MsgIssueInviteFieldOptionInvalid      = "issue_invite_field_option_invalid"
	MsgIssueInviteFieldPhoneInvalid       = "issue_invite_field_phone_invalid"
	MsgIssueInviteFieldNumberInvalid      = "issue_invite_field_number_invalid"
	MsgIssueInviteFieldIntegerRequired    = "issue_invite_field_integer_required"
	MsgIssueInviteFieldDateInvalid        = "issue_invite_field_date_invalid"
	MsgIssueInviteFieldDateTimeInvalid    = "issue_invite_field_datetime_invalid"
	MsgIssueInviteFieldMemberInvalid      = "issue_invite_field_member_invalid"
	MsgIssueInviteOrderInvalid            = "issue_invite_order_invalid"
	MsgIssueInviteRateLimited             = "issue_invite_rate_limited"
	MsgIssueInviteGuideTitleInvalid       = "issue_invite_guide_title_invalid"
	MsgIssueInviteGuideDescriptionInvalid = "issue_invite_guide_description_invalid"
	MsgIssueInviteThemeImageTooLarge      = "issue_invite_theme_image_too_large"
	MsgIssueInviteThemeImageTypeInvalid   = "issue_invite_theme_image_type_invalid"
	MsgIssueInviteThemeImageInvalid       = "issue_invite_theme_image_invalid"
	MsgIssueInviteRecallDisabled          = "issue_invite_recall_disabled"
	MsgIssueInviteRecallStatusInvalid     = "issue_invite_recall_status_invalid"
	MsgIssueInviteRecallNotAllowed        = "issue_invite_recall_not_allowed"
	MsgIssueInviteResubmitNotAllowed      = "issue_invite_resubmit_not_allowed"
	MsgIssueInviteUrgeTooSoon             = "issue_invite_urge_too_soon"
	MsgIssueInviteUrgeNotAllowed          = "issue_invite_urge_not_allowed"
	MsgIssueInviteLinkParamInvalid        = "issue_invite_link_param_invalid"
)

// Issue invitation cache key prefixes.
const (
	CacheKeyIssueInviteConfig         = `goteams:issue:config`
	CacheKeyIssueInviteTemplate       = `goteams:issue:template`
	CacheKeyIssueInviteHistoryVersion = `goteams:issue:history-version`
	CacheKeyIssueInviteRenderVersion  = `goteams:issue:render-version`
	CacheKeyIssueInviteHistory        = `goteams:issue:history`
	CacheKeyIssueInviteDetail         = `goteams:issue:detail`
	CacheKeyIssueInviteSubmitLimit    = `goteams:issue:submit-limit`
	CacheKeyIssueInviteAttachment     = `goteams:issue:attachment`
)

// IssueInviteConfigModel is the project invitation configuration business model.
type IssueInviteConfigModel struct {
	AdminID          int
	WorkspaceID      int
	InviteKey        string
	Enabled          int
	GuideTitle       string
	GuideDescription string
	ThemeImage       string
	CreatedBy        int
	CreatedAt        int64
	UpdatedAt        int64
}

// IssueInviteConfigUpdateModel carries editable invitation guide configuration.
type IssueInviteConfigUpdateModel struct {
	AdminID          int
	WorkspaceID      int
	GuideTitle       string
	GuideDescription string
	ThemeImage       string
	Enabled          int
	OperatorID       int
}

// IssueInviteTemplateModel is an independent invitation template.
type IssueInviteTemplateModel struct {
	ID               int
	AdminID          int
	WorkspaceID      int
	InviteKey        string
	TypeKey          string
	Name             string
	DescriptionMode  string
	FieldsJSON       string
	ThemeImage       string
	GuideTitle       string
	GuideDescription string
	// GuideEnabled 是模板自己的邀请文案开关，与项目级配置的 Enabled 互不同步。
	GuideEnabled int
	Enabled      int
	SortOrder    int
	// RecallEnabled 是「用户撤回」开关，1 表示用户端可以撤回。
	RecallEnabled int
	// RecallStatusID 是撤回后落到的状态，0 表示还没配。
	RecallStatusID int
	CreatedBy      int
	CreatedAt      int64
	UpdatedAt      int64
}

// IssueInviteTemplateUpdateModel carries editable copied-template data.
type IssueInviteTemplateUpdateModel struct {
	AdminID          int
	WorkspaceID      int
	TemplateID       int
	Name             string
	DescriptionMode  string
	FieldsJSON       string
	ThemeImage       string
	GuideTitle       string
	GuideDescription string
	GuideEnabled     int
	Enabled          int
	RecallEnabled    int
	RecallStatusID   int
	OperatorID       int
}

// IssueInviteTemplateOrderModel carries one complete template order update.
type IssueInviteTemplateOrderModel struct {
	AdminID           int
	WorkspaceID       int
	OrderedTemplateID []int
	OperatorID        int
}

// IssueInviteFieldValue is one normalized public form value.
type IssueInviteFieldValue struct {
	FieldKey string `json:"field_key"`
	Name     string `json:"name,omitempty"`
	// Type is the custom field type (e.g. date/select_single); the public detail uses
	// it to render values correctly, the submit request leaves it empty.
	Type  string `json:"type,omitempty"`
	Value any    `json:"value"`
}

// IssueInviteSubmitModel carries one public issue submission.
type IssueInviteSubmitModel struct {
	InviteKey   string
	TemplateKey string
	OpenID      string
	Fields      []IssueInviteFieldValue
	FileIDs     []int
}

// IssueInviteRequirementModel is a public-safe issue history/detail model.
type IssueInviteRequirementModel struct {
	RequirementID int    `json:"requirement_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	// GuideDescription is the invitation template's guide text the issue was submitted
	// through; the public pages prefer it over Title as the card/detail heading.
	GuideDescription string `json:"guide_description,omitempty"`
	TemplateName     string `json:"template_name"`
	TemplateType     string `json:"template_type"`
	// TemplateKey 是这张工单当初走的邀填模板 key；编辑态要按它拉表单 schema。
	TemplateKey  string `json:"template_key,omitempty"`
	StatusName   string `json:"status_name,omitempty"`
	StatusColor  string `json:"status_color,omitempty"`
	PriorityName string `json:"priority_name,omitempty"`
	// Fields 是详情页的完整展示列表：先内置字段（按属性字段预设顺序，只给有值的），
	// 再自定义字段（按配置顺序）。内置字段没有配置名，前端按 FieldKey 取文案。
	Fields []IssueInviteFieldValue `json:"fields,omitempty"`
	// EditFields 是编辑态预填用的原始值（下拉给 id、日期给时间戳），跟提交接口收的格式一致；
	// Fields 给的是展示值，下拉已经换成名字，不能拿来回填。
	EditFields []IssueInviteFieldValue `json:"edit_fields,omitempty"`
	CreatedAt  int64                   `json:"created_at"`
	UpdatedAt  int64                   `json:"updated_at"`
	// RecallEnabled 来自模板配置，前端据此决定要不要显示撤回按钮。
	RecallEnabled bool `json:"recall_enabled,omitempty"`
	// CanRecall 表示现在能撤回：模板开了撤回，且工单既没被撤回也不是终态。
	CanRecall bool `json:"can_recall,omitempty"`
	// Recalled 表示这张工单被用户撤回且还没重新提交。
	Recalled bool `json:"recalled,omitempty"`
	// CanModify 表示当前状态允许用户改并重新提交。
	CanModify bool `json:"can_modify,omitempty"`
	// LastUrgedAt 是上一次催单时间，和 CreatedAt 一起决定催单窗口；为 0 表示还没催过。
	LastUrgedAt int64 `json:"last_urged_at,omitempty"`
	// CanUrge 表示现在可以催单；不能时前端用 UrgeAvailableAt 提示下次可催时间。
	CanUrge         bool  `json:"can_urge,omitempty"`
	UrgeAvailableAt int64 `json:"urge_available_at,omitempty"`
}

// IssueInviteHistoryModel is the fixed-size public history response model.
type IssueInviteHistoryModel struct {
	Items    []IssueInviteRequirementModel `json:"items"`
	Page     int                           `json:"page"`
	PageSize int                           `json:"page_size"`
	Total    int                           `json:"total"`
}

// IssueInviteTemplateCreateModel carries one new independent invitation template.
type IssueInviteTemplateCreateModel struct {
	AdminID          int
	WorkspaceID      int
	TypeKey          string
	Name             string
	DescriptionMode  string
	FieldsJSON       string
	ThemeImage       string
	GuideTitle       string
	GuideDescription string
	GuideEnabled     int
	Enabled          int
	RecallEnabled    int
	RecallStatusID   int
	OperatorID       int
}

// IssueInviteRequirementActionModel 是用户端对一张已提交工单做动作的入参。
type IssueInviteRequirementActionModel struct {
	InviteKey     string
	OpenID        string
	RequirementID int
	// Fields 与 FileIDs 只在重新提交时用，撤回和催单留空。
	Fields  []IssueInviteFieldValue
	FileIDs []int
}
