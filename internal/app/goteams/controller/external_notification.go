package controller

import (
	"errors"
	"net/http"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

type externalNotificationStrategyRequest struct {
	Name       string   `json:"name"`
	Platform   string   `json:"platform"`
	WebhookURL string   `json:"webhook_url"`
	Secret     string   `json:"secret"`
	Events     []string `json:"events"`
	MemberIds  []int    `json:"member_ids"`
	AtMode     string   `json:"at_mode"`
	AtUserIds  []int    `json:"at_user_ids"`
	AtRoles    []string `json:"at_roles"`
	AtAll      bool     `json:"at_all"`
}

func externalNotificationStrategyResponse(item business.NotificationStrategyModel) gin.H {
	return gin.H{"id": item.Id, "name": item.Name, "platform": item.Platform, "webhook_url": item.WebhookURL, "secret": item.Secret, "events": item.Events, "at_mode": item.AtMode, "at_user_ids": item.AtUserIds, "at_roles": item.AtRoles, "at_all": item.AtAll, "member_ids": item.MemberIds, "members": item.Members, "enabled": item.Enabled, "today_trigger_count": item.TodayTriggerCount, "total_trigger_count": item.TotalTriggerCount, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt}
}

// UpdateExternalNotificationStrategyStatus toggles a strategy on or off.
func UpdateExternalNotificationStrategyStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "external_notification.status.bind", err)
		return
	}
	enabled := 0
	if req.Enabled {
		enabled = 1
	}
	if err := business.SetNotificationStrategyEnabled(c.GetInt("adminId"), cast.ToInt(c.Param("strategyId")), enabled); err != nil {
		abortExternalNotificationError(c, lang, "external_notification.status", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// TestExternalNotificationStrategy sends a real test message through the saved strategy.
func TestExternalNotificationStrategy(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	if err := business.TestNotificationStrategy(business.TestNotificationStrategyModel{
		AdminId: c.GetInt("adminId"), StrategyId: cast.ToInt(c.Param("strategyId")),
	}); err != nil {
		abortExternalNotificationError(c, lang, "external_notification.test", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// ListExternalNotificationStrategies lists strategies of the enterprise.
func ListExternalNotificationStrategies(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	items, err := business.ListNotificationStrategies(business.ListNotificationStrategiesModel{AdminId: c.GetInt("adminId")})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "external_notification.list", err)
		return
	}
	data := make([]gin.H, 0, len(items))
	for _, item := range items {
		data = append(data, externalNotificationStrategyResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: data})
}

func bindExternalNotificationStrategy(c *gin.Context, lang string) (externalNotificationStrategyRequest, bool) {
	var req externalNotificationStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "external_notification.bind", err)
		return req, false
	}
	return req, true
}

// CreateExternalNotificationStrategy creates an enterprise strategy.
func CreateExternalNotificationStrategy(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	req, ok := bindExternalNotificationStrategy(c, lang)
	if !ok {
		return
	}
	item, err := business.CreateNotificationStrategy(business.SaveNotificationStrategyModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), Name: req.Name, Platform: req.Platform, WebhookURL: req.WebhookURL, Secret: req.Secret, Events: req.Events, MemberIds: req.MemberIds, AtMode: req.AtMode, AtUserIds: req.AtUserIds, AtRoles: req.AtRoles, AtAll: req.AtAll})
	if err != nil {
		abortExternalNotificationError(c, lang, "external_notification.create", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: externalNotificationStrategyResponse(item)})
}

// UpdateExternalNotificationStrategy updates an enterprise strategy.
func UpdateExternalNotificationStrategy(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	req, ok := bindExternalNotificationStrategy(c, lang)
	if !ok {
		return
	}
	err := business.UpdateNotificationStrategy(business.SaveNotificationStrategyModel{AdminId: c.GetInt("adminId"), StrategyId: cast.ToInt(c.Param("strategyId")), Name: req.Name, Platform: req.Platform, WebhookURL: req.WebhookURL, Secret: req.Secret, Events: req.Events, MemberIds: req.MemberIds, AtMode: req.AtMode, AtUserIds: req.AtUserIds, AtRoles: req.AtRoles, AtAll: req.AtAll})
	if err != nil {
		abortExternalNotificationError(c, lang, "external_notification.update", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteExternalNotificationStrategy deletes an enterprise strategy.
func DeleteExternalNotificationStrategy(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	err := business.DeleteNotificationStrategy(business.DeleteNotificationStrategyModel{AdminId: c.GetInt("adminId"), StrategyId: cast.ToInt(c.Param("strategyId"))})
	if err != nil {
		abortExternalNotificationError(c, lang, "external_notification.delete", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

func abortExternalNotificationError(c *gin.Context, lang, action string, err error) {
	if errors.Is(err, business.ErrNotificationStrategyNotFound) {
		abortNotFound(c, lib_i18n.Show(lang, "external_notification_strategy_not_found"))
		return
	}
	if errors.Is(err, business.ErrNotificationStrategyMembers) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), action, errors.New("invalid members"))
		return
	}
	var fieldErr *validate.FieldError
	if errors.As(err, &fieldErr) {
		abortBadRequest(c, lib_i18n.Show(lang, fieldErr.Key), action, err)
		return
	}
	var deliveryErr *business.NotificationDeliveryError
	if errors.As(err, &deliveryErr) {
		message := lib_i18n.Show(lang, "external_notification_test_unreachable")
		if deliveryErr.Code != 0 {
			message = lib_i18n.Show(lang, "external_notification_test_rejected", deliveryErr.Code)
		}
		abortBadRequest(c, message, action, err)
		return
	}
	abortInternal(c, lib_i18n.Show(lang, define.MsgError), action, err)
}

type notificationUserMappingResponse struct {
	UserId int                                 `json:"user_id"`
	Push   notificationUserMappingPushResponse `json:"push"`
}

type notificationUserMappingPushResponse struct {
	DingtalkAtMobiles        string `json:"dingtalk_at_mobiles"`
	DingtalkAtUserIds        string `json:"dingtalk_at_user_ids"`
	FeishuUserId             string `json:"feishu_user_id"`
	WecomMentionedList       string `json:"wecom_mentioned_list"`
	WecomMentionedMobileList string `json:"wecom_mentioned_mobile_list"`
}

// ListNotificationUserMappings 返回企业通知共用的成员映射表。
func ListNotificationUserMappings(c *gin.Context) {
	listRobotUserMappings(c, 0)
}

func listRobotUserMappings(c *gin.Context, workspaceID int) {
	lang := lib_i18n.GetLang(c)
	items, err := business.ListNotificationUserMappings(business.ListRobotUserMappingsModel{AdminId: c.GetInt("adminId"), WorkspaceId: workspaceID})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "external_notification.mapping.list", err)
		return
	}
	data := make([]notificationUserMappingResponse, 0, len(items))
	for _, item := range items {
		data = append(data, notificationUserMappingResponse{UserId: item.UserId, Push: notificationUserMappingPushResponse{
			DingtalkAtMobiles: item.Push.DingtalkAtMobiles, DingtalkAtUserIds: item.Push.DingtalkAtUserIds,
			FeishuUserId: item.Push.FeishuUserId, WecomMentionedList: item.Push.WecomMentionedList, WecomMentionedMobileList: item.Push.WecomMentionedMobileList,
		}})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: data})
}

// SaveNotificationUserMappings 保存企业级机器人映射，权限由路由统一检查。
func SaveNotificationUserMappings(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req saveAutomationUserMappingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "external_notification.mapping.bind", err)
		return
	}
	if err := validateSaveAutomationUserMappingsRequest(req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "external_notification.mapping.validate", err)
		return
	}
	if err := business.SaveNotificationUserMappings(toSaveAutomationUserMappingsModel(c, 0, req)); err != nil {
		abortExternalNotificationError(c, lang, "external_notification.mapping.save", err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}
