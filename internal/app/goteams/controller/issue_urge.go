package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

type listIssueUrgeRecordsQuery struct {
	Page string `form:"page"`
	Size string `form:"size"`
}

type issueUrgeRecordResponse struct {
	Id        int   `json:"id"`
	CreatedAt int64 `json:"created_at"`
	Legacy    bool  `json:"legacy"`
}

type listIssueUrgeRecordsResponse struct {
	List              []issueUrgeRecordResponse `json:"list"`
	Total             int                       `json:"total"`
	Page              int                       `json:"page"`
	Size              int                       `json:"size"`
	IncompleteHistory bool                      `json:"incomplete_history"`
}

// ListIssueUrgeRecords 读取当前工单的分页催单记录。
func ListIssueUrgeRecords(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	wsId, wsErr := positiveParam(c, "wsId")
	reqId, reqErr := positiveParam(c, "reqId")
	var query listIssueUrgeRecordsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "issue_urge.bind", err)
		return
	}
	if wsErr != nil || reqErr != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "issue_urge.path", business.ErrIssueInviteInvalid)
		return
	}
	if query.Page == "" {
		query.Page = "1"
	}
	if query.Size == "" {
		query.Size = "20"
	}
	page, pageErr := strconv.Atoi(query.Page)
	size, sizeErr := strconv.Atoi(query.Size)
	if pageErr != nil || sizeErr != nil || page < 1 || size < 1 || size > 100 || page > int(^uint(0)>>1)/size {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "issue_urge.pagination", business.ErrIssueInviteInvalid)
		return
	}
	model, err := business.ListIssueUrgeRecords(business.ListIssueUrgeRecordsModel{
		AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), WorkspaceId: wsId, RequirementId: reqId, Page: page, Size: size,
	})
	if err != nil {
		switch {
		case errors.Is(err, business.ErrRequirementNotFound):
			abortNotFound(c, lib_i18n.Show(lang, define.MsgRequirementNotFound))
		case errors.Is(err, business.ErrIssueInviteInvalid):
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), "issue_urge.list", err)
		default:
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), "issue_urge.list", err)
		}
		return
	}
	response := listIssueUrgeRecordsResponse{List: make([]issueUrgeRecordResponse, 0, len(model.Items)), Total: model.Total, Page: model.Page, Size: model.Size, IncompleteHistory: model.IncompleteHistory}
	for _, item := range model.Items {
		response.List = append(response.List, issueUrgeRecordResponse{Id: item.Id, CreatedAt: item.CreatedAt, Legacy: item.Legacy})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response})
}
