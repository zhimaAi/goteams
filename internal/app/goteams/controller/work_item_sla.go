package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

type slaSettingsRequest struct {
	Enabled  bool   `json:"enabled"`
	Timezone string `json:"timezone"`
	Revision int64  `json:"revision"`
}
type slaSettingsResponse struct {
	Enabled           bool   `json:"enabled"`
	Timezone          string `json:"timezone"`
	Revision          int64  `json:"revision"`
	ActivatedAt       int64  `json:"activated_at"`
	ClockRevision     int64  `json:"clock_revision"`
	CalendarAvailable bool   `json:"calendar_available"`
	CalendarRegion    string `json:"calendar_region"`
}
type slaStageRequest struct {
	Name      string          `json:"name"`
	SortOrder int             `json:"sort_order"`
	Revision  int64           `json:"revision"`
	Config    json.RawMessage `json:"config"`
}
type slaPolicyRequest struct {
	Enabled     *bool           `json:"enabled"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	SortOrder   int             `json:"sort_order"`
	Revision    int64           `json:"revision"`
	PriorityIds []int           `json:"priority_ids"`
	CategoryIds []int           `json:"category_ids"`
	Config      json.RawMessage `json:"config"`
}
type slaStageResponse struct {
	Id        int             `json:"id"`
	Name      string          `json:"name"`
	SortOrder int             `json:"sort_order"`
	Revision  int64           `json:"revision"`
	Config    json.RawMessage `json:"config"`
}
type slaPolicyResponse struct {
	Enabled     bool            `json:"enabled"`
	Id          int             `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	SortOrder   int             `json:"sort_order"`
	Revision    int64           `json:"revision"`
	PriorityIds []int           `json:"priority_ids"`
	CategoryIds []int           `json:"category_ids"`
	Config      json.RawMessage `json:"config"`
}
type slaStageListResponse struct {
	List  []slaStageResponse `json:"list"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}
type slaPolicyListResponse struct {
	List  []slaPolicyResponse `json:"list"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Size  int                 `json:"size"`
}
type slaOrderRequest struct {
	Ids      []int `json:"ids"`
	Revision int64 `json:"revision"`
}
type slaArchiveRequest struct {
	Revision int64 `json:"revision"`
}

func slaWorkspaceParam(c *gin.Context) (int, bool) {
	id, err := positiveParam(c, "wsId")
	if err != nil {
		abortSLAError(c, business.ErrSLAInvalid)
		return 0, false
	}
	return id, true
}
func abortSLAError(c *gin.Context, err error) {
	lang := lib_i18n.GetLang(c)
	switch {
	case errors.Is(err, business.ErrSLAInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgSLAInvalid), "sla", err)
	case errors.Is(err, business.ErrSLAWorkspace):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgSLAWorkspace), "sla", err)
	case errors.Is(err, business.ErrSLAConflict):
		abortConflict(c, lib_i18n.Show(lang, define.MsgSLAConflict))
	case errors.Is(err, business.ErrSLANotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgSLANotFound))
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "sla", err)
	}
}
func bindSLARequest(c *gin.Context, request any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	if err := c.ShouldBindJSON(request); err != nil {
		abortBadRequest(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgParamInvalid), "sla.bind", err)
		return false
	}
	return true
}
func toSLASettingsResponse(model business.SLASettingsModel) slaSettingsResponse {
	return slaSettingsResponse{Enabled: model.Enabled, Timezone: model.Timezone, Revision: model.Revision, ActivatedAt: model.ActivatedAt, ClockRevision: model.ClockRevision, CalendarAvailable: model.CalendarAvailable, CalendarRegion: model.CalendarRegion}
}
func toSLAStageResponse(model business.SLADefinitionModel) slaStageResponse {
	return slaStageResponse{Id: model.Id, Name: model.Name, SortOrder: model.SortOrder, Revision: model.Revision, Config: model.Config}
}
func toSLAPolicyResponse(model business.SLADefinitionModel) slaPolicyResponse {
	return slaPolicyResponse{Enabled: !model.Disabled, Id: model.Id, Name: model.Name, Description: model.Description, SortOrder: model.SortOrder, Revision: model.Revision, PriorityIds: model.PriorityIds, CategoryIds: model.CategoryIds, Config: model.Config}
}

// GetSLASettings 读取工作空间 SLA 设置。
func GetSLASettings(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	model, err := business.GetSLASettings(business.SLASettingsModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toSLASettingsResponse(model)})
}

// SaveSLASettings 修改总开关和固定计时时区。
func SaveSLASettings(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	var request slaSettingsRequest
	if !bindSLARequest(c, &request) {
		return
	}
	model, err := business.SaveSLASettings(business.SLASettingsModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), WorkspaceId: ws, Enabled: request.Enabled, Timezone: request.Timezone, Revision: request.Revision})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toSLASettingsResponse(model)})
}
func slaListModel(c *gin.Context) (business.ListSLADefinitionsModel, bool) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return business.ListSLADefinitionsModel{}, false
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		abortSLAError(c, business.ErrSLAInvalid)
		return business.ListSLADefinitionsModel{}, false
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", "100"))
	if err != nil {
		abortSLAError(c, business.ErrSLAInvalid)
		return business.ListSLADefinitionsModel{}, false
	}
	return business.ListSLADefinitionsModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws, UserId: c.GetInt("userId"), Page: page, Size: size}, true
}

// ListSLAStages 返回阶段定义列表。
func ListSLAStages(c *gin.Context) {
	model, ok := slaListModel(c)
	if !ok {
		return
	}
	model, err := business.ListSLAStages(model)
	if err != nil {
		abortSLAError(c, err)
		return
	}
	items := make([]slaStageResponse, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, toSLAStageResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: slaStageListResponse{List: items, Total: model.Total, Page: model.Page, Size: model.Size}})
}

// ListSLAPolicies 返回服务目标列表。
func ListSLAPolicies(c *gin.Context) {
	model, ok := slaListModel(c)
	if !ok {
		return
	}
	model, err := business.ListSLAPolicies(model)
	if err != nil {
		abortSLAError(c, err)
		return
	}
	items := make([]slaPolicyResponse, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, toSLAPolicyResponse(item))
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: slaPolicyListResponse{List: items, Total: model.Total, Page: model.Page, Size: model.Size}})
}
func slaDefinitionId(c *gin.Context, name string) (int, bool) {
	if c.Param(name) == "" {
		return 0, true
	}
	id, err := positiveParam(c, name)
	if err != nil {
		abortSLAError(c, business.ErrSLAInvalid)
		return 0, false
	}
	return id, true
}

// SaveSLAStage 创建或更新阶段定义。
func SaveSLAStage(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	id, ok := slaDefinitionId(c, "stageId")
	if !ok {
		return
	}
	var request slaStageRequest
	if !bindSLARequest(c, &request) {
		return
	}
	model, err := business.SaveSLAStage(business.SLADefinitionModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), WorkspaceId: ws, Id: id, Name: request.Name, SortOrder: request.SortOrder, Revision: request.Revision, Config: request.Config})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toSLAStageResponse(model)})
}

// SaveSLAPolicy 创建或更新服务目标。
func SaveSLAPolicy(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	id, ok := slaDefinitionId(c, "policyId")
	if !ok {
		return
	}
	var request slaPolicyRequest
	if !bindSLARequest(c, &request) {
		return
	}
	model, err := business.SaveSLAPolicy(business.SLADefinitionModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), WorkspaceId: ws, Id: id, Name: request.Name, Description: request.Description, Disabled: request.Enabled != nil && !*request.Enabled, SortOrder: request.SortOrder, Revision: request.Revision, Config: request.Config, PriorityIds: request.PriorityIds, CategoryIds: request.CategoryIds})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toSLAPolicyResponse(model)})
}

// GetSLAPolicy 读取单条服务目标。
func GetSLAPolicy(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	id, ok := slaDefinitionId(c, "policyId")
	if !ok {
		return
	}
	model, err := business.GetSLAPolicy(business.SLADefinitionModel{AdminId: c.GetInt("adminId"), UserId: c.GetInt("userId"), WorkspaceId: ws, Id: id})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toSLAPolicyResponse(model)})
}

// SaveSLAPolicyOrder 原子保存目标顺序。
func SaveSLAPolicyOrder(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	var request slaOrderRequest
	if !bindSLARequest(c, &request) {
		return
	}
	if err := business.SaveSLAPolicyOrder(business.SLAOrderModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws, Ids: request.Ids, Revision: request.Revision}); err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess})
}

// ArchiveSLADefinition 按引用约束归档阶段或服务目标。
func ArchiveSLADefinition(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	stage := c.Param("stageId") != ""
	name := "policyId"
	if stage {
		name = "stageId"
	}
	id, ok := slaDefinitionId(c, name)
	if !ok {
		return
	}
	var request slaArchiveRequest
	if !bindSLARequest(c, &request) {
		return
	}
	if err := business.ArchiveSLADefinition(business.SLADefinitionModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws, Id: id, Revision: request.Revision}, stage); err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess})
}

type slaStageResultResponse struct {
	CycleId          int64  `json:"cycle_id"`
	StageId          int64  `json:"stage_id"`
	StageName        string `json:"stage_name"`
	State            string `json:"state"`
	Reason           string `json:"reason"`
	CalculationState string `json:"calculation_state"`
	StartedAt        int64  `json:"started_at"`
	EndedAt          int64  `json:"ended_at"`
	LimitSeconds     int64  `json:"limit_seconds"`
	ElapsedSeconds   *int64 `json:"elapsed_seconds"`
	RemainingSeconds *int64 `json:"remaining_seconds"`
	OverdueSeconds   *int64 `json:"overdue_seconds"`
	DeadlineAt       *int64 `json:"deadline_at"`
}
type slaResultResponse struct {
	Enabled                 bool                     `json:"enabled"`
	ConfigurationIncomplete bool                     `json:"configuration_incomplete"`
	Paused                  bool                     `json:"paused"`
	EvaluatedAt             int64                    `json:"evaluated_at"`
	Current                 []slaStageResultResponse `json:"current"`
	History                 []slaStageResultResponse `json:"history"`
	Total                   int                      `json:"total"`
}
type slaItemSummaryResponse struct {
	RequirementId int               `json:"requirement_id"`
	SLA           slaResultResponse `json:"sla"`
}
type slaSummaryListResponse struct {
	List []slaItemSummaryResponse `json:"list"`
}

func toSLAStageResultResponse(model business.SLAStageResultModel) slaStageResultResponse {
	return slaStageResultResponse{CycleId: model.CycleId, StageId: model.StageId, StageName: model.StageName, State: model.State, Reason: model.Reason, CalculationState: model.CalculationState, StartedAt: model.StartedAt, EndedAt: model.EndedAt, LimitSeconds: model.LimitSeconds, ElapsedSeconds: model.ElapsedSeconds, RemainingSeconds: model.RemainingSeconds, OverdueSeconds: model.OverdueSeconds, DeadlineAt: model.DeadlineAt}
}
func toSLAResultResponse(model business.SLAResultModel) slaResultResponse {
	current := make([]slaStageResultResponse, 0, len(model.Current))
	for _, item := range model.Current {
		current = append(current, toSLAStageResultResponse(item))
	}
	history := make([]slaStageResultResponse, 0, len(model.History))
	for _, item := range model.History {
		history = append(history, toSLAStageResultResponse(item))
	}
	return slaResultResponse{Enabled: model.Enabled, ConfigurationIncomplete: model.ConfigurationIncomplete, Paused: model.Paused, EvaluatedAt: model.EvaluatedAt, Current: current, History: history, Total: model.Total}
}

// GetSLASummaries 返回有数据可见性约束的列表投影。
func GetSLASummaries(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	raw := strings.Split(c.Query("ids"), ",")
	if len(raw) > 100 {
		abortSLAError(c, business.ErrSLAInvalid)
		return
	}
	ids := make([]int, 0, len(raw))
	for _, value := range raw {
		id, err := strconv.Atoi(value)
		if err != nil || id <= 0 {
			abortSLAError(c, business.ErrSLAInvalid)
			return
		}
		ids = append(ids, id)
	}
	model, err := business.GetSLASummaries(business.SLABatchReadModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws, UserId: c.GetInt("userId"), Ids: ids})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	items := make([]slaItemSummaryResponse, 0, len(model.Items))
	for _, item := range model.Items {
		items = append(items, slaItemSummaryResponse{RequirementId: item.RequirementId, SLA: toSLAResultResponse(item.Result)})
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: slaSummaryListResponse{List: items}})
}

// GetRequirementSLA 返回工单当前阶段及历史。
func GetRequirementSLA(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	reqId, err := positiveParam(c, "reqId")
	if err != nil {
		abortSLAError(c, business.ErrSLAInvalid)
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		abortSLAError(c, business.ErrSLAInvalid)
		return
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", "20"))
	if err != nil {
		abortSLAError(c, business.ErrSLAInvalid)
		return
	}
	result, err := business.GetRequirementSLA(business.SLAReadModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws, RequirementId: reqId, UserId: c.GetInt("userId"), Page: page, Size: size})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toSLAResultResponse(result)})
}

type slaOptionResponse struct {
	Id       int    `json:"id"`
	ParentId int    `json:"parent_id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
}
type slaMetadataResponse struct {
	Statuses   []slaOptionResponse `json:"statuses"`
	Priorities []slaOptionResponse `json:"priorities"`
	Categories []slaOptionResponse `json:"categories"`
	Roles      []slaOptionResponse `json:"roles"`
}

func toSLAOptions(items []business.SLAOptionModel, lang string) []slaOptionResponse {
	options := make([]slaOptionResponse, 0, len(items))
	for _, item := range items {
		options = append(options, slaOptionResponse{Id: item.Id, ParentId: item.ParentId, Name: lib_i18n.ShowStored(lang, item.Name), Color: item.Color})
	}
	return options
}

// GetSLAMetadata 返回配置编辑所需的同空间选项。
func GetSLAMetadata(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	model, err := business.GetSLAMetadata(business.SLAMetadataModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws})
	if err != nil {
		abortSLAError(c, err)
		return
	}
	lang := lib_i18n.GetLang(c)
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: slaMetadataResponse{Statuses: toSLAOptions(model.Statuses, lang), Priorities: toSLAOptions(model.Priorities, lang), Categories: toSLAOptions(model.Categories, lang), Roles: toSLAOptions(model.Roles, lang)}})
}

// RetrySLAAction 为管理员提供失败动作的有界恢复入口。
func RetrySLAAction(c *gin.Context) {
	ws, ok := slaWorkspaceParam(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("actionId"), 10, 64)
	if err != nil || id <= 0 {
		abortSLAError(c, business.ErrSLAInvalid)
		return
	}
	if err = business.RetrySLAAction(business.RetrySLAActionModel{AdminId: c.GetInt("adminId"), WorkspaceId: ws, ActionId: id}); err != nil {
		abortSLAError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess})
}
