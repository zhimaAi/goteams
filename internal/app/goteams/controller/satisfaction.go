package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

type satisfactionTagDTO struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}
type satisfactionOptionDTO struct {
	Key             string               `json:"key"`
	Label           string               `json:"label"`
	Sentiment       string               `json:"sentiment"`
	Value           int                  `json:"value"`
	Tags            []satisfactionTagDTO `json:"tags"`
	TagsRequired    bool                 `json:"tags_required"`
	CommentRequired bool                 `json:"comment_required"`
}

// 配置读写刻意共用公开字段契约，版本检查单独属于保存请求。
type satisfactionConfigResponse struct {
	Enabled          bool                    `json:"enabled"`
	ManualEnabled    bool                    `json:"manual_enabled"`
	ValiditySeconds  int                     `json:"validity_seconds"`
	AutoCreatorKinds []int                   `json:"auto_creator_kinds"`
	AutoStatusIds    []int                   `json:"auto_status_ids"`
	AutoTemplateIds  []int                   `json:"auto_template_ids"`
	RatingMode       int                     `json:"rating_mode"`
	Options          []satisfactionOptionDTO `json:"options"`
	ResolvedEnabled  bool                    `json:"resolved_enabled"`
	ResolvedRequired bool                    `json:"resolved_required"`
	InvitationText   string                  `json:"invitation_text"`
	CompletionText   string                  `json:"completion_text"`
	Version          int                     `json:"version"`
}
type saveSatisfactionRequest struct {
	Enabled          bool                    `json:"enabled"`
	ManualEnabled    bool                    `json:"manual_enabled"`
	ValiditySeconds  int                     `json:"validity_seconds"`
	AutoCreatorKinds []int                   `json:"auto_creator_kinds"`
	AutoStatusIds    []int                   `json:"auto_status_ids"`
	AutoTemplateIds  []int                   `json:"auto_template_ids"`
	RatingMode       int                     `json:"rating_mode"`
	Options          []satisfactionOptionDTO `json:"options"`
	ResolvedEnabled  bool                    `json:"resolved_enabled"`
	ResolvedRequired bool                    `json:"resolved_required"`
	InvitationText   string                  `json:"invitation_text"`
	CompletionText   string                  `json:"completion_text"`
	ExpectedVersion  int                     `json:"expected_version"`
}
type satisfactionRuleResponse struct {
	RatingMode       int                     `json:"rating_mode"`
	Options          []satisfactionOptionDTO `json:"options"`
	ResolvedEnabled  bool                    `json:"resolved_enabled"`
	ResolvedRequired bool                    `json:"resolved_required"`
	InvitationText   string                  `json:"invitation_text"`
	CompletionText   string                  `json:"completion_text"`
}
type satisfactionResultResponse struct {
	RequirementId  int                      `json:"requirement_id"`
	ConfigVersion  int                      `json:"config_version"`
	State          string                   `json:"state"`
	EligibleAt     int64                    `json:"eligible_at"`
	ExpiresAt      int64                    `json:"expires_at"`
	SubmittedAt    int64                    `json:"submitted_at"`
	Rule           satisfactionRuleResponse `json:"rule"`
	OptionKey      string                   `json:"option_key"`
	Label          string                   `json:"label"`
	Sentiment      string                   `json:"sentiment"`
	Value          int                      `json:"value"`
	Tags           []satisfactionTagDTO     `json:"tags"`
	Content        string                   `json:"content"`
	Resolved       *bool                    `json:"resolved"`
	ReviewerName   string                   `json:"reviewer_name"`
	ReviewerAvatar string                   `json:"reviewer_avatar"`
}
type submitSatisfactionRequest struct {
	ExpectedConfigVersion int      `json:"expected_config_version"`
	OptionKey             string   `json:"option_key"`
	TagKeys               []string `json:"tag_keys"`
	Content               string   `json:"content"`
	Resolved              *bool    `json:"resolved"`
}

func satisfactionText(lang, value string) string {
	if strings.HasPrefix(value, "satisfaction_") {
		return lib_i18n.Show(lang, value)
	}
	return value
}
func toSatisfactionConfig(lang string, m business.SatisfactionConfigModel) satisfactionConfigResponse {
	options := []satisfactionOptionDTO{}
	for _, o := range m.Options {
		tags := []satisfactionTagDTO{}
		for _, t := range o.Tags {
			tags = append(tags, satisfactionTagDTO{Key: t.Key, Label: t.Label})
		}
		options = append(options, satisfactionOptionDTO{Key: o.Key, Label: satisfactionText(lang, o.Label), Sentiment: o.Sentiment, Value: o.Value, Tags: tags, TagsRequired: o.TagsRequired, CommentRequired: o.CommentRequired})
	}
	return satisfactionConfigResponse{Enabled: m.Enabled, ManualEnabled: m.ManualEnabled, ValiditySeconds: m.ValiditySeconds, AutoCreatorKinds: m.AutoCreatorKinds, AutoStatusIds: m.AutoStatusIds, AutoTemplateIds: m.AutoTemplateIds, RatingMode: m.RatingMode, Options: options, ResolvedEnabled: m.ResolvedEnabled, ResolvedRequired: m.ResolvedRequired, InvitationText: m.InvitationText, CompletionText: m.CompletionText, Version: m.Version}
}
func toSatisfactionResult(lang string, m business.SatisfactionResultModel) satisfactionResultResponse {
	tags := []satisfactionTagDTO{}
	for _, t := range m.Tags {
		tags = append(tags, satisfactionTagDTO{Key: t.Key, Label: t.Label})
	}
	return satisfactionResultResponse{RequirementId: m.RequirementId, ConfigVersion: m.Rule.Version, State: m.State, EligibleAt: m.EligibleAt, ExpiresAt: m.ExpiresAt, SubmittedAt: m.SubmittedAt, Rule: satisfactionRuleResponse{RatingMode: m.Rule.RatingMode, Options: toSatisfactionConfig(lang, m.Rule).Options, ResolvedEnabled: m.Rule.ResolvedEnabled, ResolvedRequired: m.Rule.ResolvedRequired, InvitationText: m.Rule.InvitationText, CompletionText: m.Rule.CompletionText}, OptionKey: m.OptionKey, Label: satisfactionText(lang, m.Label), Sentiment: m.Sentiment, Value: m.Value, Tags: tags, Content: m.Content, Resolved: m.Resolved, ReviewerName: m.ReviewerName, ReviewerAvatar: m.ReviewerAvatar}
}
func satisfactionError(c *gin.Context, e error) {
	lang := lib_i18n.GetLang(c)
	switch {
	case errors.Is(e, business.ErrSatisfactionInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, "satisfaction_invalid"), "satisfaction", e)
	case errors.Is(e, business.ErrSatisfactionConflict):
		abortConflict(c, lib_i18n.Show(lang, "satisfaction_conflict"))
	case errors.Is(e, business.ErrSatisfactionForbidden):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
	case errors.Is(e, business.ErrRequirementNotFound), errors.Is(e, business.ErrIssueInviteInvalid), errors.Is(e, business.ErrIssueInviteAppDisabled):
		abortNotFound(c, lib_i18n.Show(lang, "satisfaction_unavailable"))
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "satisfaction", e)
	}
}
func satisfactionRespond(c *gin.Context, data any) {
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgSuccess), Data: data})
}

// GetSatisfactionConfig 返回当前空间的满意度设置。
func GetSatisfactionConfig(c *gin.Context) {
	scope, ok := dashboardScope(c, lib_i18n.GetLang(c), "satisfaction.config")
	if !ok {
		return
	}
	m, e := business.GetSatisfactionConfig(business.SatisfactionConfigModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId})
	if e != nil {
		satisfactionError(c, e)
		return
	}
	satisfactionRespond(c, toSatisfactionConfig(lib_i18n.GetLang(c), m))
}

// SaveSatisfactionConfig 按客户端读取版本保存整份配置。
func SaveSatisfactionConfig(c *gin.Context) {
	scope, ok := dashboardScope(c, lib_i18n.GetLang(c), "satisfaction.config")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	var r saveSatisfactionRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		satisfactionError(c, business.ErrSatisfactionInvalid)
		return
	}
	options := []business.SatisfactionOptionModel{}
	for _, o := range r.Options {
		tags := []business.SatisfactionTagModel{}
		for _, t := range o.Tags {
			tags = append(tags, business.SatisfactionTagModel{Key: t.Key, Label: t.Label})
		}
		options = append(options, business.SatisfactionOptionModel{Key: o.Key, Label: o.Label, Value: o.Value, Sentiment: o.Sentiment, Tags: tags, TagsRequired: o.TagsRequired, CommentRequired: o.CommentRequired})
	}
	m, e := business.SaveSatisfactionConfig(business.SatisfactionConfigModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, Enabled: r.Enabled, ManualEnabled: r.ManualEnabled, ValiditySeconds: r.ValiditySeconds, AutoCreatorKinds: r.AutoCreatorKinds, AutoStatusIds: r.AutoStatusIds, AutoTemplateIds: r.AutoTemplateIds, RatingMode: r.RatingMode, Options: options, ResolvedEnabled: r.ResolvedEnabled, ResolvedRequired: r.ResolvedRequired, InvitationText: r.InvitationText, CompletionText: r.CompletionText, Version: r.ExpectedVersion})
	if e != nil {
		satisfactionError(c, e)
		return
	}
	satisfactionRespond(c, toSatisfactionConfig(lib_i18n.GetLang(c), m))
}
func satisfactionAccess(c *gin.Context) (business.SatisfactionAccessModel, bool) {
	m := business.SatisfactionAccessModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), InviteKey: c.Param("inviteKey"), OpenID: c.Query("openid")}
	var e error
	if m.InviteKey == "" {
		m.WorkspaceId, e = positiveParam(c, "wsId")
		if e != nil {
			satisfactionError(c, business.ErrSatisfactionInvalid)
			return m, false
		}
	}
	param := "requirementId"
	if m.InviteKey == "" {
		param = "reqId"
	}
	m.RequirementId, e = positiveParam(c, param)
	if e != nil {
		satisfactionError(c, business.ErrSatisfactionInvalid)
		return m, false
	}
	return m, true
}

// GetSatisfaction 返回内部或当前邀填客户可查看的评价。
func GetSatisfaction(c *gin.Context) {
	m, ok := satisfactionAccess(c)
	if !ok {
		return
	}
	r, e := business.GetSatisfaction(m)
	if e != nil {
		satisfactionError(c, e)
		return
	}
	satisfactionRespond(c, toSatisfactionResult(lib_i18n.GetLang(c), r))
}

// SubmitSatisfaction 只接受公开邀填客户首次提交，不存在改评入口。
func SubmitSatisfaction(c *gin.Context) {
	m, ok := satisfactionAccess(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var r submitSatisfactionRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		satisfactionError(c, business.ErrSatisfactionInvalid)
		return
	}
	result, e := business.SubmitSatisfaction(business.SatisfactionSubmitModel{Access: m, ExpectedConfigVersion: r.ExpectedConfigVersion, OptionKey: r.OptionKey, TagKeys: r.TagKeys, Content: r.Content, Resolved: r.Resolved})
	if e != nil {
		satisfactionError(c, e)
		return
	}
	satisfactionRespond(c, toSatisfactionResult(lib_i18n.GetLang(c), result))
}

type satisfactionStateResponse struct {
	RequirementId int    `json:"requirement_id"`
	State         string `json:"state"`
}

// ListPublicSatisfactionStates 返回当前列表页的评价能力。
func ListPublicSatisfactionStates(c *gin.Context) {
	ids := []int{}
	raw := c.Query("requirement_ids")
	if len(raw) > 600 {
		satisfactionError(c, business.ErrSatisfactionInvalid)
		return
	}
	if raw != "" {
		for _, s := range strings.Split(raw, ",") {
			id, e := strconv.Atoi(s)
			if e != nil {
				satisfactionError(c, business.ErrSatisfactionInvalid)
				return
			}
			ids = append(ids, id)
		}
	}
	m, e := business.ListPublicSatisfactionStates(business.SatisfactionAccessModel{InviteKey: c.Param("inviteKey"), OpenID: c.Query("openid")}, ids)
	if e != nil {
		satisfactionError(c, e)
		return
	}
	out := []satisfactionStateResponse{}
	for _, v := range m {
		out = append(out, satisfactionStateResponse{RequirementId: v.RequirementId, State: v.State})
	}
	satisfactionRespond(c, out)
}

type satisfactionStatisticsRequest struct {
	From       int64  `json:"from"`
	To         int64  `json:"to"`
	Timezone   string `json:"timezone"`
	MemberId   *int   `json:"member_id"`
	RoleId     *int   `json:"role_id"`
	CategoryId *int   `json:"category_id"`
	PriorityId *int   `json:"priority_id"`
	Page       int    `json:"page"`
	Size       int    `json:"size"`
}
type satisfactionCountResponse struct {
	Invited   int `json:"invited"`
	Submitted int `json:"submitted"`
	Positive  int `json:"positive"`
	Neutral   int `json:"neutral"`
	Negative  int `json:"negative"`
}
type satisfactionGroupResponse struct {
	Id    int                       `json:"id"`
	Name  string                    `json:"name"`
	Count satisfactionCountResponse `json:"count"`
}
type satisfactionChoiceResponse struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
type satisfactionStatisticsResponse struct {
	Count           satisfactionCountResponse    `json:"count"`
	Trend           []satisfactionGroupResponse  `json:"trend"`
	Members         []satisfactionGroupResponse  `json:"members"`
	Categories      []satisfactionGroupResponse  `json:"categories"`
	MemberOptions   []satisfactionChoiceResponse `json:"member_options"`
	RoleOptions     []satisfactionChoiceResponse `json:"role_options"`
	CategoryOptions []satisfactionChoiceResponse `json:"category_options"`
	PriorityOptions []satisfactionChoiceResponse `json:"priority_options"`
	MemberTotal     int                          `json:"member_total"`
	CategoryTotal   int                          `json:"category_total"`
	Page            int                          `json:"page"`
	Size            int                          `json:"size"`
	EvaluatedAt     int64                        `json:"evaluated_at"`
}

func toSatisfactionCount(v business.SatisfactionCountModel) satisfactionCountResponse {
	return satisfactionCountResponse{Invited: v.Invited, Submitted: v.Submitted, Positive: v.Positive, Neutral: v.Neutral, Negative: v.Negative}
}
func satisfactionStoredLabel(lang, value string) string {
	if value == "" {
		return lib_i18n.Show(lang, "satisfaction_unassigned")
	}
	return lib_i18n.ShowStored(lang, value)
}
func toSatisfactionGroups(lang string, items []business.SatisfactionGroupModel) []satisfactionGroupResponse {
	out := []satisfactionGroupResponse{}
	for _, v := range items {
		out = append(out, satisfactionGroupResponse{Id: v.Id, Name: satisfactionStoredLabel(lang, v.Name), Count: toSatisfactionCount(v.Count)})
	}
	return out
}
func toSatisfactionChoices(lang string, items []business.SatisfactionChoiceModel) []satisfactionChoiceResponse {
	out := []satisfactionChoiceResponse{}
	for _, v := range items {
		out = append(out, satisfactionChoiceResponse{Id: v.Id, Name: satisfactionStoredLabel(lang, v.Name)})
	}
	return out
}

// QuerySatisfactionStatistics 提供独立于SLA的工单满意度统计。
func QuerySatisfactionStatistics(c *gin.Context) {
	scope, ok := dashboardScope(c, lib_i18n.GetLang(c), "satisfaction.statistics")
	if !ok {
		return
	}
	var r satisfactionStatisticsRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		satisfactionError(c, business.ErrSatisfactionInvalid)
		return
	}
	if r.Page == 0 {
		r.Page = 1
	}
	if r.Size == 0 {
		r.Size = 20
	}
	m, e := business.QuerySatisfactionStatistics(c.Request.Context(), business.SatisfactionStatisticsQueryModel{AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, From: r.From, To: r.To, Timezone: r.Timezone, MemberId: r.MemberId, RoleId: r.RoleId, CategoryId: r.CategoryId, PriorityId: r.PriorityId, Page: r.Page, Size: r.Size})
	if e != nil {
		satisfactionError(c, e)
		return
	}
	lang := lib_i18n.GetLang(c)
	satisfactionRespond(c, satisfactionStatisticsResponse{Count: toSatisfactionCount(m.Count), Trend: toSatisfactionGroups(lang, m.Trend), Members: toSatisfactionGroups(lang, m.Members), Categories: toSatisfactionGroups(lang, m.Categories), MemberOptions: toSatisfactionChoices(lang, m.MemberOptions), RoleOptions: toSatisfactionChoices(lang, m.RoleOptions), CategoryOptions: toSatisfactionChoices(lang, m.CategoryOptions), PriorityOptions: toSatisfactionChoices(lang, m.PriorityOptions), MemberTotal: m.MemberTotal, CategoryTotal: m.CategoryTotal, Page: m.Page, Size: m.Size, EvaluatedAt: m.EvaluatedAt})
}
