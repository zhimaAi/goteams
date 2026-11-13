package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// notificationResponseItem is the JSON representation of one notification.
type notificationResponseItem struct {
	PriorityName             string `json:"priority_name"`
	PriorityColor            string `json:"priority_color"`
	CreatorName              string `json:"creator_name"`
	AssigneeNames            string `json:"assignee_names"`
	ID                       int    `json:"id"`
	EventType                string `json:"event_type"`
	Content                  string `json:"content"`
	ActorUserID              int    `json:"actor_user_id"`
	ActorName                string `json:"actor_name"`
	ActorAvatar              string `json:"actor_avatar"`
	WorkspaceID              int    `json:"workspace_id"`
	WorkspaceName            string `json:"workspace_name"`
	WorkspaceIcon            string `json:"workspace_icon"`
	RequirementID            int    `json:"requirement_id"`
	RequirementType          string `json:"requirement_type"`
	RequirementTypeIconText  string `json:"requirement_type_icon_text"`
	RequirementTypeIconColor string `json:"requirement_type_icon_color"`
	RequirementTitle         string `json:"requirement_title"`
	FieldName                string `json:"field_name"`
	Read                     bool   `json:"read"`
	CreatedAt                int64  `json:"created_at"`
}

type notificationListResponse struct {
	List        []notificationResponseItem `json:"list"`
	Total       int                        `json:"total"`
	UnreadTotal int                        `json:"unread_total"`
	Page        int                        `json:"page"`
	Size        int                        `json:"size"`
}

func toNotificationResponse(model business.NotificationListModel, lang string) notificationListResponse {
	items := make([]notificationResponseItem, 0, len(model.Items))
	for _, item := range model.Items {
		content := item.Content
		if item.EventType == define.NotificationEventSlaOverdue || item.EventType == define.NotificationEventSlaWarning {
			content = translateSLAContent(content, lang)
		}
		items = append(items, notificationResponseItem{PriorityName: lib_i18n.ShowStored(lang, item.PriorityName), PriorityColor: item.PriorityColor, CreatorName: item.CreatorName, AssigneeNames: item.AssigneeNames, ID: item.ID, EventType: item.EventType, Content: content, ActorUserID: item.ActorUserID, ActorName: item.ActorName, ActorAvatar: item.ActorAvatar, WorkspaceID: item.WorkspaceID, WorkspaceName: item.WorkspaceName, WorkspaceIcon: item.WorkspaceIcon, RequirementID: item.RequirementID, RequirementType: item.RequirementType, RequirementTypeIconText: item.RequirementTypeIconText, RequirementTypeIconColor: item.RequirementTypeIconColor, RequirementTitle: item.RequirementTitle, FieldName: item.FieldName, Read: item.Read, CreatedAt: item.CreatedAt})
	}
	return notificationListResponse{List: items, Total: model.Total, UnreadTotal: model.UnreadTotal, Page: model.Page, Size: model.Size}
}

// translateSLAContent renders the status i18n key stored by the SLA worker (e.g.
// ${workitem_status_pending_plan}) in the request language; plain custom status names
// pass through unchanged. The worker has no request language, so translation must
// happen at read time.
func translateSLAContent(content, lang string) string {
	var payload struct {
		Status  string `json:"status"`
		Minutes int    `json:"minutes,omitempty"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil || payload.Status == `` {
		return content
	}
	payload.Status = lib_i18n.ShowStored(lang, payload.Status)
	translated, err := json.Marshal(payload)
	if err != nil {
		return content
	}
	return string(translated)
}

// ListNotifications returns the current user's cross-workspace notifications.
func ListNotifications(c *gin.Context) {
	page, size, unreadOnly, err := parseNotificationQuery(c)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgParamInvalid), `notification.list.query`, err)
		return
	}
	model, err := business.ListNotifications(c.GetInt(`adminId`), c.GetInt(`userId`), unreadOnly, page, size)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgError), `notification.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgSuccess), Data: toNotificationResponse(model, lib_i18n.GetLang(c))})
}

// UnreadNotificationCount returns the current user's unread count.
func UnreadNotificationCount(c *gin.Context) {
	count, err := business.CountUnreadNotifications(c.GetInt(`adminId`), c.GetInt(`userId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgError), `notification.unread_count`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgSuccess), Data: map[string]int{`count`: count}})
}

// MarkNotificationsRead marks selected notifications read.
func MarkNotificationsRead(c *gin.Context) {
	var body struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		abortBadRequest(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgParamInvalid), `notification.read.bind`, err)
		return
	}
	if err := business.MarkNotificationsRead(c.GetInt(`adminId`), c.GetInt(`userId`), body.IDs); err != nil {
		abortInternal(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgError), `notification.read`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgSuccess)})
}

// MarkAllNotificationsRead marks all notifications read for the current user.
func MarkAllNotificationsRead(c *gin.Context) {
	if err := business.MarkAllNotificationsRead(c.GetInt(`adminId`), c.GetInt(`userId`)); err != nil {
		abortInternal(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgError), `notification.read_all`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgSuccess)})
}

func parseNotificationQuery(c *gin.Context) (int, int, bool, error) {
	page, size := 1, 20
	var err error
	if value := c.Query(`page`); value != `` {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			return 0, 0, false, errors.New(`invalid page`)
		}
	}
	if value := c.Query(`size`); value != `` {
		size, err = strconv.Atoi(value)
		if err != nil || size < 1 || size > 100 {
			return 0, 0, false, errors.New(`invalid size`)
		}
	}
	unread := c.Query(`unread_only`)
	if unread != `` && unread != `0` && unread != `1` {
		return 0, 0, false, errors.New(`invalid unread_only`)
	}
	return page, size, unread == `1`, nil
}
