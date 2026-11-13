package controller

import (
	"errors"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type getPlanInfoResponse struct {
	AdminId                       int    `json:"admin_id"`
	PlanType                      string `json:"plan_type"`
	ExpireAt                      int    `json:"expire_at"`
	IsExpired                     bool   `json:"is_expired,omitempty"`
	MemberLimit                   int    `json:"member_limit"`
	ProjectLimit                  int    `json:"project_limit"`
	CustomFieldPerProjectLimit    int    `json:"custom_field_per_project_limit"`
	AutomationRulePerProjectLimit int    `json:"automation_rule_per_project_limit"`
	AutomationExecLimit           int    `json:"automation_exec_limit"`
	AgentLimit                    int    `json:"agent_limit"`
	StorageLimit                  int    `json:"storage_limit"`
	StorageOverDeadline           int    `json:"storage_over_deadline"`
	CapacityStatus                int    `json:"capacity_status"`
}

type getPlanUsageResponse struct {
	EnterpriseName string                     `json:"enterprise_name"`
	PlanType       string                     `json:"plan_type"`
	ExpireAt       int                        `json:"expire_at"`
	IsExpired      bool                       `json:"is_expired,omitempty"`
	Items          []getPlanUsageItemResponse `json:"items"`
}

type getPlanUsageItemResponse struct {
	Key             string `json:"key"`
	Used            int    `json:"used"`
	Limit           int    `json:"limit"`
	EnterpriseLimit int    `json:"enterprise_limit"`
	PerProjectLimit int    `json:"per_project_limit,omitempty"`
	TotalLimit      int    `json:"total_limit,omitempty"`
}

type updateEnterpriseNameRequest struct {
	EnterpriseName string `json:"enterprise_name"`
}

type updateEnterpriseNameResponse struct {
	EnterpriseName string `json:"enterprise_name"`
}

// GetPlanInfo 返回当前租户套餐信息。
func GetPlanInfo(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	loadPlan := business.GetPlan
	if !define.LicenseStateCanLogin(business.GetLicenseSnapshot().State) {
		loadPlan = business.GetPlanForLicenseManagement
	}
	plan, err := loadPlan(c.GetInt(`adminId`))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `plan.info`, err)
		return
	}
	capacityStatus, err := business.GetCapacityStatus(plan)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `plan.capacity`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toGetPlanInfoResponse(plan, capacityStatus),
	})
}

// GetPlanUsage 返回当前租户套餐和用量明细。
func GetPlanUsage(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	loadUsage := business.GetPlanUsage
	if !define.LicenseStateCanLogin(business.GetLicenseSnapshot().State) {
		loadUsage = business.GetPlanUsageForLicenseManagement
	}
	usage, err := loadUsage(business.PlanUsageModel{AdminId: c.GetInt(`adminId`)})
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `plan.usage`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: toGetPlanUsageResponse(usage),
	})
}

// UpdateEnterpriseName 修改当前租户的企业名称。
func UpdateEnterpriseName(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)
	if !business.IsTenantOwner(adminId, userId) {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	var req updateEnterpriseNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `enterprise.name.bind`, err)
		return
	}
	req.EnterpriseName = strings.TrimSpace(req.EnterpriseName)
	if err := validate.ValidateEnterpriseName(req.EnterpriseName); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `enterprise.name.validate`, err)
		return
	}
	model, err := business.UpdateEnterpriseName(business.UpdateEnterpriseNameModel{
		AdminId: adminId, UserId: userId, EnterpriseName: req.EnterpriseName,
	})
	if err != nil {
		if errors.Is(err, business.ErrEnterpriseAdminRequired) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `enterprise.name.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: updateEnterpriseNameResponse{EnterpriseName: model.EnterpriseName},
	})
}

type extendStorageDeadlineResponse struct {
	StorageOverDeadline int `json:"storage_over_deadline"`
}

// ExtendStorageDeadline 容量超限延期 3 天，仅允许系统管理员操作且每企业仅一次。
func ExtendStorageDeadline(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId := c.GetInt(`adminId`)
	userId := c.GetInt(`userId`)
	if userId != adminId {
		abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
		return
	}
	deadline, err := business.ExtendStorageDeadline(adminId, userId)
	if err != nil {
		if errors.Is(err, business.ErrEnterpriseAdminRequired) {
			abortForbidden(c, lib_i18n.Show(lang, define.MsgForbidden))
			return
		}
		if errors.Is(err, business.ErrStorageExtendOnlyOnce) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgStorageExtendOnlyOnce), `plan.storage.extend`, err)
			return
		}
		if errors.Is(err, business.ErrStorageExtendUnavailable) {
			abortBadRequest(c, lib_i18n.Show(lang, define.MsgStorageExtendNotOverLimit), `plan.storage.extend`, err)
			return
		}
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `plan.storage.extend`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgStorageExtendSuccess),
		Data: extendStorageDeadlineResponse{StorageOverDeadline: deadline},
	})
}

func toGetPlanInfoResponse(model business.PlanModel, capacityStatus int) getPlanInfoResponse {
	return getPlanInfoResponse{
		AdminId: model.AdminId, PlanType: model.PlanType, ExpireAt: model.ExpireAt,
		IsExpired: model.IsExpired, MemberLimit: model.MemberLimit, ProjectLimit: model.ProjectLimit,
		CustomFieldPerProjectLimit:    model.CustomFieldPerProjectLimit,
		AutomationRulePerProjectLimit: model.AutomationRulePerProjectLimit,
		AutomationExecLimit:           model.AutomationExecLimit, AgentLimit: model.AgentLimit,
		StorageLimit:        model.StorageLimit,
		StorageOverDeadline: model.StorageOverDeadline,
		CapacityStatus:      capacityStatus,
	}
}

func toGetPlanUsageResponse(model business.PlanUsageModel) getPlanUsageResponse {
	items := make([]getPlanUsageItemResponse, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, getPlanUsageItemResponse{
			Key: item.Key, Used: item.Used, Limit: item.Limit,
			EnterpriseLimit: item.EnterpriseLimit, PerProjectLimit: item.PerProjectLimit,
			TotalLimit: item.TotalLimit,
		})
	}
	return getPlanUsageResponse{
		EnterpriseName: model.EnterpriseName, PlanType: model.PlanType,
		ExpireAt: model.ExpireAt, IsExpired: model.IsExpired, Items: items,
	}
}
