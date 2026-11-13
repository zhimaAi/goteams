// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

var errTeamWorkPeriodInvalid = errors.New("invalid team work period")

type teamWorkResponseItem struct {
	Id                string   `json:"id"`
	WorkspaceId       string   `json:"workspace_id"`
	StatusId          string   `json:"status_id"`
	AssigneeId        string   `json:"assignee_id"`
	CreatedBy         string   `json:"created_by"`
	MemberSortOrder   string   `json:"member_sort_order"`
	PlannedStartDate  string   `json:"planned_start_date"`
	PlannedEndDate    string   `json:"planned_end_date"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
	Priority          string   `json:"priority"`
	StatusSortOrder   string   `json:"status_sort_order"`
	PrioritySortOrder string   `json:"priority_sort_order"`
	Title             string   `json:"title"`
	Type              string   `json:"type"`
	WorkspaceName     string   `json:"workspace_name"`
	StatusName        string   `json:"status_name"`
	StatusColor       string   `json:"status_color"`
	PriorityName      string   `json:"priority_name"`
	PriorityColor     string   `json:"priority_color"`
	AssigneeName      string   `json:"assignee_name"`
	AssigneeUsername  string   `json:"assignee_username"`
	AssigneeAvatar    string   `json:"assignee_avatar"`
	IsDone            string   `json:"is_done"`
	IsCompleted       bool     `json:"is_completed"`
	CanView           string   `json:"can_view"`
	CanEdit           string   `json:"can_edit"`
	AssigneeIds       []int    `json:"assignee_ids"`
	AssigneeNames     []string `json:"assignee_names"`
	AssigneeAvatars   []string `json:"assignee_avatars"`
	AssigneeUsernames []string `json:"assignee_usernames"`
	CanOpenDetail     bool     `json:"can_open_detail"`
	requirementWorkHoursSummaryResponse
}

type teamWorkResponse struct {
	List []teamWorkResponseItem `json:"list"`
	Date string                 `json:"date,omitempty"`
}

func toTeamWorkResponseItem(item business.TeamWorkItemModel, lang string) teamWorkResponseItem {
	optionalInt := func(value *int) string {
		if value == nil {
			return ""
		}
		return strconv.Itoa(*value)
	}
	return teamWorkResponseItem{
		Id:                strconv.Itoa(item.Id),
		WorkspaceId:       strconv.Itoa(item.WorkspaceId),
		StatusId:          strconv.Itoa(item.StatusId),
		AssigneeId:        strconv.Itoa(item.AssigneeId),
		CreatedBy:         strconv.Itoa(item.CreatedBy),
		MemberSortOrder:   strconv.Itoa(item.MemberSortOrder),
		PlannedStartDate:  strconv.FormatInt(item.PlannedStartDate, 10),
		PlannedEndDate:    strconv.FormatInt(item.PlannedEndDate, 10),
		CreatedAt:         strconv.FormatInt(item.CreatedAt, 10),
		UpdatedAt:         strconv.FormatInt(item.UpdatedAt, 10),
		Priority:          optionalInt(item.Priority),
		StatusSortOrder:   optionalInt(item.StatusSortOrder),
		PrioritySortOrder: optionalInt(item.PrioritySortOrder),
		Title:             item.Title,
		Type:              item.Type,
		WorkspaceName:     item.WorkspaceName,
		StatusName:        lib_i18n.ShowStored(lang, item.StatusName),
		StatusColor:       item.StatusColor,
		PriorityName:      lib_i18n.ShowStored(lang, item.PriorityName),
		PriorityColor:     item.PriorityColor,
		AssigneeName:      item.AssigneeName,
		AssigneeUsername:  item.AssigneeUsername,
		AssigneeAvatar:    item.AssigneeAvatar,
		AssigneeIds:       item.AssigneeIds,
		AssigneeNames:     item.AssigneeNames,
		AssigneeAvatars:   item.AssigneeAvatars,
		AssigneeUsernames: item.AssigneeUsernames,
		IsDone:            strconv.FormatBool(item.IsDone),
		IsCompleted:       item.IsCompleted,
		CanView:           strconv.Itoa(cast.ToInt(item.CanView)),
		CanEdit:           strconv.Itoa(cast.ToInt(item.CanEdit)),
		CanOpenDetail:     item.CanOpenDetail,
		// 团队工作原查询未取工时，保留原响应中的零值汇总。
		requirementWorkHoursSummaryResponse: toRequirementWorkHoursSummaryResponse(business.CalculateRequirementWorkHours(0, 0)),
	}
}

// ListTeamWork Return the team work items within the specified date and role viewing range.
func ListTeamWork(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := cast.ToInt(c.GetInt(`adminId`))
	userId := cast.ToInt(c.GetInt(`userId`))
	day, permissionKey, mode, err := resolveTeamWorkPeriod(c.Query(`period`), c.Query(`date`))
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `team_work.list.period`, err)
		return
	}
	if !business.HasPermission(adminId, userId, ``, permissionKey) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}

	list, err := business.ListTeamWork(business.ListTeamWorkModel{AdminId: adminId, UserId: userId, Day: day, Mode: mode})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `team_work.list.query`, err)
		return
	}
	responseList := make([]teamWorkResponseItem, 0, len(list))
	for _, item := range list {
		responseList = append(responseList, toTeamWorkResponseItem(item, lang))
	}
	data := teamWorkResponse{List: responseList}
	if mode == business.TeamWorkModeDay {
		data.Date = day.Format(`2006-01-02`)
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    data,
	})
}

// ListTeamWorkMembers Return the member visibility settings within the current user's permission scope.
func ListTeamWorkMembers(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	list, err := business.ListTeamWorkMembers(c.GetInt(`adminId`), c.GetInt(`userId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `team_work.members.list`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    list,
	})
}

// ListTeamWorkMemberCandidates returns the member picker source data for the team-work display list.
func ListTeamWorkMemberCandidates(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.ListTeamWorkMemberCandidates(c.GetInt(`adminId`), c.GetInt(`userId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `team_work.members.candidates`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toListMemberCandidatesResponse(model, lang),
	})
}

// UpdateTeamWorkMembers saves the current user's member display order and visibility switches.
func UpdateTeamWorkMembers(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req struct {
		Members []business.TeamWorkMemberSetting `json:"members" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `team_work.members.bind`, err)
		return
	}
	if err := business.UpdateTeamWorkMemberSettings(c.GetInt(`adminId`), c.GetInt(`userId`), req.Members); err != nil {
		if errors.Is(err, business.ErrTeamWorkMemberSettingsInvalid) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `team_work.members.validate`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `team_work.members.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
	})
}

func resolveTeamWorkPeriod(period string, customDate string) (time.Time, string, string, error) {
	now := time.Now()
	switch period {
	case `today`:
		return now, `team_work:view_today`, business.TeamWorkModeDay, nil
	case `yesterday`:
		return now.AddDate(0, 0, -1), `team_work:view_yesterday`, business.TeamWorkModeDay, nil
	case `tomorrow`:
		return now.AddDate(0, 0, 1), `team_work:view_tomorrow`, business.TeamWorkModeDay, nil
	case `unscheduled`:
		return now, `team_work:view_unscheduled`, business.TeamWorkModeUnscheduled, nil
	case `scheduled`:
		return now, `team_work:view_scheduled`, business.TeamWorkModeScheduled, nil
	case `custom`:
		day, err := time.ParseInLocation(`2006-01-02`, customDate, time.Local)
		if err != nil {
			return time.Time{}, ``, ``, errTeamWorkPeriodInvalid
		}
		return day, `team_work:view_custom_date`, business.TeamWorkModeDay, nil
	default:
		return time.Time{}, ``, ``, errTeamWorkPeriodInvalid
	}
}
