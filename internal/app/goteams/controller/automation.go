package controller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

const maxAutomationRequestBody = 1 << 20

type automationRuleRequest struct {
	Name   string          `json:"name"`
	Locale string          `json:"locale"`
	Config json.RawMessage `json:"config"`
}

type automationRuleIdResponse struct {
	Id int `json:"id"`
}

type automationCopyTargetsResponseItem struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type automationCopyResponse struct {
	Id    int   `json:"id,omitempty"`
	Ids   []int `json:"ids"`
	Count int   `json:"count"`
}

type automationCopyRequest struct {
	WorkspaceIds *[]int `json:"workspace_ids"`
}

// saveAutomationUserMappingPushItem carries the mention parameters named after each robot API field.
type saveAutomationUserMappingPushItem struct {
	DingtalkAtMobiles        *string `json:"dingtalk_at_mobiles"`
	DingtalkAtUserIds        *string `json:"dingtalk_at_user_ids"`
	FeishuUserId             *string `json:"feishu_user_id"`
	WecomMentionedList       *string `json:"wecom_mentioned_list"`
	WecomMentionedMobileList *string `json:"wecom_mentioned_mobile_list"`
}

type saveAutomationUserMappingItem struct {
	UserId int                               `json:"user_id"`
	Push   saveAutomationUserMappingPushItem `json:"push"`
}

type saveAutomationUserMappingsRequest struct {
	Items []saveAutomationUserMappingItem `json:"items"`
}

type automationRuleResponse struct {
	Id                    string          `json:"id"`
	AdminId               string          `json:"admin_id"`
	WorkspaceId           string          `json:"workspace_id"`
	Name                  string          `json:"name"`
	Enabled               string          `json:"enabled"`
	CreatedBy             string          `json:"created_by"`
	CreatorNameSnapshot   string          `json:"creator_name_snapshot"`
	CreatorAvatarSnapshot string          `json:"creator_avatar_snapshot"`
	CreatorName           string          `json:"creator_name"`
	CreatorAvatar         string          `json:"creator_avatar"`
	Locale                string          `json:"locale"`
	TotalTriggerCount     string          `json:"total_trigger_count"`
	CreatedAt             string          `json:"created_at"`
	UpdatedAt             string          `json:"updated_at"`
	Config                json.RawMessage `json:"config"`
}

// toAutomationRuleResponse keeps the historical string-typed rule detail contract.
func toAutomationRuleResponse(model business.AutomationRuleModel) automationRuleResponse {
	return automationRuleResponse{
		Id: strconv.Itoa(model.Id), AdminId: strconv.Itoa(model.AdminId), WorkspaceId: strconv.Itoa(model.WorkspaceId),
		Name: model.Name, Enabled: strconv.Itoa(model.Enabled), CreatedBy: strconv.Itoa(model.CreatedBy),
		CreatorNameSnapshot: model.CreatorNameSnapshot, CreatorAvatarSnapshot: model.CreatorAvatarSnapshot,
		CreatorName: model.CreatorName, CreatorAvatar: model.CreatorAvatar, Locale: model.Locale,
		TotalTriggerCount: strconv.FormatInt(model.TotalTriggerCount, 10),
		CreatedAt:         strconv.FormatInt(model.CreatedAt, 10), UpdatedAt: strconv.FormatInt(model.UpdatedAt, 10),
		Config: model.Config,
	}
}

// ListAutomationRules returns the list of workspace rules.
func ListAutomationRules(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	page, size := cast.ToInt(c.DefaultQuery("page", "1")), cast.ToInt(c.DefaultQuery("size", "20"))
	if err := validate.ValidateAutomationPage(page, size); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.rule.list.page", err)
		return
	}
	todayStart, todayEnd := cast.ToInt64(c.Query("today_start")), cast.ToInt64(c.Query("today_end"))
	yesterdayStart, yesterdayEnd := cast.ToInt64(c.Query("yesterday_start")), cast.ToInt64(c.Query("yesterday_end"))
	if validate.ValidateAutomationTimeRange(todayStart, todayEnd) != nil || validate.ValidateAutomationTimeRange(yesterdayStart, yesterdayEnd) != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.rule.list.time", validate.ErrAutomationInvalid)
		return
	}
	result, err := business.AutomationRuleList(c.GetInt("adminId"), workspaceID, c.Query("search"), page, size, todayStart, todayEnd, yesterdayStart, yesterdayEnd)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "automation.rule.list", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: result})
}

// GetAutomationRule returns the rule detail.
func GetAutomationRule(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ruleID, ok := automationRuleParams(c, lang)
	if !ok {
		return
	}
	model, err := business.GetAutomationRule(business.GetAutomationRuleModel{
		AdminId: c.GetInt("adminId"), WorkspaceId: workspaceID, RuleId: ruleID,
	})
	if err != nil {
		automationRuleError(c, lang, "automation.rule.get", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toAutomationRuleResponse(model)})
}

// CreateAutomationRule creates a rule.
func CreateAutomationRule(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	req, ok := bindAutomationRule(c, lang)
	if !ok {
		return
	}
	model, err := business.CreateAutomationRule(business.CreateAutomationRuleModel{
		AdminId: c.GetInt("adminId"), WorkspaceId: workspaceID, UserId: c.GetInt("userId"),
		Name: req.Name, Locale: req.Locale, Config: req.Config,
	})
	if err != nil {
		automationRuleError(c, lang, "automation.rule.create", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: automationRuleIdResponse{Id: model.Id},
	})
}

// UpdateAutomationRule edits a rule.
func UpdateAutomationRule(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ruleID, ok := automationRuleParams(c, lang)
	if !ok {
		return
	}
	req, ok := bindAutomationRule(c, lang)
	if !ok {
		return
	}
	err := business.UpdateAutomationRule(business.UpdateAutomationRuleModel{
		AdminId: c.GetInt("adminId"), WorkspaceId: workspaceID, RuleId: ruleID,
		Name: req.Name, Locale: req.Locale, Config: req.Config,
	})
	if err != nil {
		automationRuleError(c, lang, "automation.rule.update", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// CopyAutomationRule copies a rule.
func CopyAutomationRule(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ruleID, ok := automationRuleParams(c, lang)
	if !ok {
		return
	}
	var req automationCopyRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.rule.copy.bind", err)
		return
	}
	targetWorkspaceIds := []int{workspaceID}
	if req.WorkspaceIds != nil {
		targetWorkspaceIds = *req.WorkspaceIds
	}
	ids, err := business.CopyAutomationRules(business.CopyAutomationRulesModel{
		AdminId: c.GetInt("adminId"), SourceWorkspaceId: workspaceID, RuleId: ruleID,
		UserId: c.GetInt("userId"), Locale: lang, TargetWorkspaceIds: targetWorkspaceIds,
	})
	if err != nil {
		if errors.Is(err, business.ErrAutomationCopyTargetForbidden) {
			respondLoggedError(c, http.StatusForbidden, http.StatusForbidden, lib_i18n.Show(lang, define.MsgForbidden), "automation.rule.copy.target_permission", err)
			return
		}
		if errors.Is(err, business.ErrAutomationCopyTargetInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.rule.copy.validate", err)
			return
		}
		automationRuleError(c, lang, "automation.rule.copy", err)
		return
	}
	firstId := 0
	if len(ids) > 0 {
		firstId = ids[0]
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: automationCopyResponse{Id: firstId, Ids: ids, Count: len(ids)},
	})
}

// ListAutomationCopyTargets returns projects where the current user can manage automation.
func ListAutomationCopyTargets(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	model, err := business.ListAutomationCopyTargets(business.ListAutomationCopyTargetsModel{
		AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), SourceWorkspaceId: workspaceID,
	})
	if err != nil {
		if errors.Is(err, business.ErrAutomationCopyTargetInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.copy_targets.validate", err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "automation.copy_targets.list", err)
		return
	}
	items := make([]automationCopyTargetsResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, automationCopyTargetsResponseItem{Id: item.Id, Name: item.Name})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: items})
}

// SetAutomationRuleEnabled enables or disables a rule.
func SetAutomationRuleEnabled(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ruleID, ok := automationRuleParams(c, lang)
	if !ok {
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Enabled == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.rule.enabled.bind", err)
		return
	}
	if err := business.SetAutomationRuleEnabled(c.GetInt("adminId"), workspaceID, ruleID, *input.Enabled); err != nil {
		automationRuleError(c, lang, "automation.rule.enabled", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteAutomationRule deletes a rule.
func DeleteAutomationRule(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ruleID, ok := automationRuleParams(c, lang)
	if !ok {
		return
	}
	if err := business.DeleteAutomationRule(c.GetInt("adminId"), workspaceID, ruleID); err != nil {
		automationRuleError(c, lang, "automation.rule.delete", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// ListAutomationExecutions returns the list of executions.
func ListAutomationExecutions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	page, size := cast.ToInt(c.DefaultQuery("page", "1")), cast.ToInt(c.DefaultQuery("size", "20"))
	startAt, endAt := cast.ToInt64(c.Query("start_at")), cast.ToInt64(c.Query("end_at"))
	if validate.ValidateAutomationPage(page, size) != nil || validate.ValidateAutomationTimeRange(startAt, endAt) != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.execution.list.params", validate.ErrAutomationInvalid)
		return
	}
	result, err := business.ListAutomationExecutions(c.GetInt("adminId"), workspaceID, cast.ToInt(c.Query("rule_id")), c.Query("search"), startAt, endAt, page, size)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "automation.execution.list", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: result})
}

// GetAutomationExecution returns the execution log detail.
func GetAutomationExecution(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	executionID, err := strconv.ParseInt(c.Param("executionId"), 10, 64)
	if err != nil || executionID <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.execution.id", err)
		return
	}
	result, err := business.GetAutomationExecution(c.GetInt("adminId"), workspaceID, executionID, lang)
	if errors.Is(err, business.ErrAutomationExecutionNotFound) {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAutomationExecutionNotFound))
		return
	}
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "automation.execution.get", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: result})
}

// GetAutomationMetadata returns dynamic fields, variables and copy targets.
func GetAutomationMetadata(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	var query automationMetadataQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.metadata.query", err)
		return
	}
	if query.Type == `` {
		query.Type = define.TypeKeyRequirement
	}
	if query.Type != define.TypeKeyRequirement && query.Type != define.TypeKeyDefect {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.metadata.type", validate.ErrAutomationInvalid)
		return
	}
	model, err := business.AutomationMetadata(business.AutomationMetadataModel{
		AdminId: c.GetInt("adminId"), WorkspaceId: workspaceID, UserId: c.GetInt("userId"),
		RuleId: cast.ToInt(query.RuleId), ItemType: query.Type, Lang: lang,
	})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "automation.metadata.get", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toAutomationMetadataResponse(model),
	})
}

// validateSaveAutomationUserMappingsRequest validates the length of every mention parameter.
func validateSaveAutomationUserMappingsRequest(req saveAutomationUserMappingsRequest) error {
	for _, item := range req.Items {
		for _, identifier := range []*string{
			item.Push.DingtalkAtMobiles, item.Push.DingtalkAtUserIds, item.Push.FeishuUserId,
			item.Push.WecomMentionedList, item.Push.WecomMentionedMobileList,
		} {
			if identifier == nil {
				continue
			}
			if err := validate.ValidateAutomationPushIdentifier(*identifier); err != nil {
				return err
			}
		}
	}
	return nil
}

// toSaveAutomationUserMappingsModel converts the request and trusted context into the business model.
// 参数映射为企业共用，保留字段是否提交的语义。
func toSaveAutomationUserMappingsModel(c *gin.Context, workspaceID int, req saveAutomationUserMappingsRequest) business.SaveAutomationUserMappingsModel {
	model := business.SaveAutomationUserMappingsModel{
		AdminId: c.GetInt("adminId"), WorkspaceId: workspaceID,
		Items: make([]business.AutomationUserMappingUpdateModel, 0, len(req.Items)),
	}
	for _, item := range req.Items {
		model.Items = append(model.Items, business.AutomationUserMappingUpdateModel{
			UserId: item.UserId,
			Push: business.AutomationUserPushUpdateModel{
				DingtalkAtMobiles:        item.Push.DingtalkAtMobiles,
				DingtalkAtUserIds:        item.Push.DingtalkAtUserIds,
				FeishuUserId:             item.Push.FeishuUserId,
				WecomMentionedList:       item.Push.WecomMentionedList,
				WecomMentionedMobileList: item.Push.WecomMentionedMobileList,
			},
		})
	}
	return model
}

// SaveAutomationUserMappings stores the push identifiers used by robot mentions.
func SaveAutomationUserMappings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return
	}
	var req saveAutomationUserMappingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.user_mapping.bind", err)
		return
	}
	if err := validateSaveAutomationUserMappingsRequest(req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.user_mapping.validate", err)
		return
	}
	if err := business.SaveAutomationUserMappings(toSaveAutomationUserMappingsModel(c, workspaceID, req)); err != nil {
		if errors.Is(err, business.ErrAutomationMemberInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.user_mapping.reject", err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "automation.user_mapping.save", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// automationWorkspaceParam reads and validates the workspace path parameter.
func automationWorkspaceParam(c *gin.Context, lang string) (int, bool) {
	workspaceID, err := positiveParam(c, "wsId")
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.workspace_id", err)
		return 0, false
	}
	return workspaceID, true
}

// automationRuleParams reads and validates the workspace and rule path parameters.
func automationRuleParams(c *gin.Context, lang string) (int, int, bool) {
	workspaceID, ok := automationWorkspaceParam(c, lang)
	if !ok {
		return 0, 0, false
	}
	ruleID, err := positiveParam(c, "ruleId")
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "automation.rule_id", err)
		return 0, 0, false
	}
	return workspaceID, ruleID, true
}

// bindAutomationRule limits the request body size and strictly binds the rule input.
func bindAutomationRule(c *gin.Context, lang string) (automationRuleRequest, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAutomationRequestBody)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var req automationRuleRequest
	if err := decoder.Decode(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAutomationConfigInvalid), "automation.rule.bind", err)
		return req, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAutomationConfigInvalid), "automation.rule.trailing", err)
		return req, false
	}
	if err := validate.ValidateAutomationRule(req.Name, req.Locale, len(req.Config)); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAutomationConfigInvalid), "automation.rule.validate", err)
		return req, false
	}
	return req, true
}

// automationRuleError converts a business error into a unified HTTP response.
func automationRuleError(c *gin.Context, lang, stage string, err error) {
	if abortQuotaExceeded(c, lib_i18n.Show(lang, define.MsgQuotaExceeded), stage+".quota", err) {
		return
	}
	switch {
	case errors.Is(err, business.ErrAutomationRuleNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgAutomationRuleNotFound))
	case errors.Is(err, business.ErrAutomationRuleNameExists):
		abortConflict(c, lib_i18n.Show(lang, define.MsgAutomationRuleNameExists))
	case errors.Is(err, business.ErrAutomationConfigInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgAutomationConfigInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

// ListAutomationUserMappings 在工作空间权限边界内读取企业共用映射。
func ListAutomationUserMappings(c *gin.Context) {
	workspaceID, ok := automationWorkspaceParam(c, lib_i18n.GetLang(c))
	if !ok {
		return
	}
	listRobotUserMappings(c, workspaceID)
}
