// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// dashboardResponse is the public shared-dashboard projection. Visibility member, role, and
// editor IDs deliberately stay out of this projection; only a manager may read them through
// the dedicated permission endpoint.
type dashboardResponse struct {
	Id             int                     `json:"id"`
	Name           string                  `json:"name"`
	SortOrder      int                     `json:"sort_order"`
	VisibilityType string                  `json:"visibility_type"`
	Revision       int64                   `json:"revision"`
	CreatedAt      int64                   `json:"created_at"`
	UpdatedAt      int64                   `json:"updated_at"`
	CanEdit        bool                    `json:"can_edit"`
	CanManage      bool                    `json:"can_manage"`
	CanReorder     bool                    `json:"can_reorder"`
	Cards          []dashboardCardResponse `json:"cards"`
}

// dashboardCardResponse is the public projection of one persisted dashboard card.
type dashboardCardResponse struct {
	Id          int             `json:"id"`
	DashboardId int             `json:"dashboard_id"`
	CardType    string          `json:"card_type"`
	Title       string          `json:"title"`
	Config      json.RawMessage `json:"config"`
	Content     string          `json:"content"`
	ContentMode string          `json:"content_mode"`
	X           int             `json:"x"`
	Y           int             `json:"y"`
	W           int             `json:"w"`
	H           int             `json:"h"`
	Revision    int64           `json:"revision"`
	CreatedAt   int64           `json:"created_at"`
	UpdatedAt   int64           `json:"updated_at"`
}

// dashboardPermissionResponse is available only to a dashboard manager.
type dashboardPermissionResponse struct {
	DashboardId    int    `json:"dashboard_id"`
	VisibilityType string `json:"visibility_type"`
	RoleIds        []int  `json:"role_ids"`
	MemberIds      []int  `json:"member_ids"`
	EditorIds      []int  `json:"editor_ids"`
	Revision       int64  `json:"revision"`
}

// dashboardAnnouncementResponse is the project-wide announcement read through a visible dashboard.
type dashboardAnnouncementResponse struct {
	Id          int    `json:"id"`
	Content     string `json:"content"`
	ContentMode string `json:"content_mode"`
	Revision    int64  `json:"revision"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// dashboardUserStateResponse is the requesting user's private dashboard filter state.
type dashboardUserStateResponse struct {
	Filters          json.RawMessage `json:"filters"`
	FiltersCollapsed bool            `json:"filters_collapsed"`
	CreatedAt        int64           `json:"created_at"`
	UpdatedAt        int64           `json:"updated_at"`
}

// dashboardMetricResponse is one on-demand metric value.
type dashboardMetricResponse struct {
	SLA        *dashboardSLAStatsResponse `json:"sla,omitempty"`
	MetricType string                     `json:"metric_type"`
	DataScope  string                     `json:"data_scope"`
	Value      *float64                   `json:"value"`
}

// dashboardAttachmentResponse is a newly uploaded, still-unbound rich-text asset.
type dashboardAttachmentResponse struct {
	FileId       int    `json:"file_id"`
	Url          string `json:"url"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
}

// createDashboardRequest is the HTTP payload for a new shared dashboard.
type createDashboardRequest struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

// updateDashboardRequest is the HTTP payload for a name update protected by dashboard revision.
type updateDashboardRequest struct {
	Name     *string `json:"name"`
	Revision int64   `json:"revision"`
}

// deleteDashboardRequest is the HTTP payload for a revision-guarded dashboard delete.
type deleteDashboardRequest struct {
	Revision int64 `json:"revision"`
}

// reorderDashboardsRequest contains the complete resulting shared dashboard order.
type reorderDashboardsRequest struct {
	Ids []int `json:"ids"`
}

// replaceDashboardPermissionRequest is the manager-only visibility and editor configuration.
type replaceDashboardPermissionRequest struct {
	VisibilityType string `json:"visibility_type"`
	RoleIds        []int  `json:"role_ids"`
	MemberIds      []int  `json:"member_ids"`
	EditorIds      []int  `json:"editor_ids"`
	Revision       int64  `json:"revision"`
}

// saveDashboardAsRequest is the revision-guarded source copy request.
type saveDashboardAsRequest struct {
	Name     string `json:"name"`
	Revision int64  `json:"revision"`
}

// dashboardLayoutCardRequest is one complete card snapshot in an atomic layout save. Id is
// omitted for a new card and Revision is required for a persisted card.
type dashboardLayoutCardRequest struct {
	Id          *int            `json:"id"`
	CardType    string          `json:"card_type"`
	Title       string          `json:"title"`
	Config      json.RawMessage `json:"config"`
	Content     string          `json:"content"`
	ContentMode string          `json:"content_mode"`
	FileIds     []int           `json:"file_ids"`
	X           int             `json:"x"`
	Y           int             `json:"y"`
	W           int             `json:"w"`
	H           int             `json:"h"`
	Revision    *int64          `json:"revision"`
}

// dashboardDeletedCardRequest identifies one card removed by an atomic layout save.
type dashboardDeletedCardRequest struct {
	Id       int   `json:"id"`
	Revision int64 `json:"revision"`
}

// dashboardAnnouncementMutationRequest is the optional project announcement change bundled
// with an atomic layout save.
type dashboardAnnouncementMutationRequest struct {
	Content     string `json:"content"`
	ContentMode string `json:"content_mode"`
	FileIds     []int  `json:"file_ids"`
	Revision    int64  `json:"revision"`
}

// replaceDashboardLayoutRequest saves all resulting cards, removals, and an optional announcement atomically.
type replaceDashboardLayoutRequest struct {
	Revision     int64                                 `json:"revision"`
	Cards        []dashboardLayoutCardRequest          `json:"cards"`
	DeletedCards []dashboardDeletedCardRequest         `json:"deleted_cards"`
	Announcement *dashboardAnnouncementMutationRequest `json:"announcement"`
}

// createDashboardCardRequest remains for callers of the pre-batch card endpoint.
type createDashboardCardRequest struct {
	CardType    string          `json:"card_type"`
	Title       string          `json:"title"`
	Config      json.RawMessage `json:"config"`
	Content     string          `json:"content"`
	ContentMode string          `json:"content_mode"`
	FileIds     []int           `json:"file_ids"`
	X           int             `json:"x"`
	Y           int             `json:"y"`
	W           int             `json:"w"`
	H           int             `json:"h"`
}

// updateDashboardCardRequest remains for callers of the pre-batch card endpoint.
type updateDashboardCardRequest struct {
	Title       *string          `json:"title"`
	Config      *json.RawMessage `json:"config"`
	Content     *string          `json:"content"`
	ContentMode *string          `json:"content_mode"`
	FileIds     []int            `json:"file_ids"`
	X           *int             `json:"x"`
	Y           *int             `json:"y"`
	W           *int             `json:"w"`
	H           *int             `json:"h"`
	Revision    int64            `json:"revision"`
}

// deleteDashboardCardRequest remains for callers of the pre-batch card endpoint.
type deleteDashboardCardRequest struct {
	Revision int64 `json:"revision"`
}

// copyDashboardCardRequest identifies the target dashboard for a metric-card copy.
type copyDashboardCardRequest struct {
	TargetDashboardId int `json:"target_dashboard_id"`
}

// dashboardMetricQueryRequest contains the metric card and an optional filter tree that can
// only narrow the requesting user's already-authorized result set.
type dashboardMetricQueryRequest struct {
	CardId   int             `json:"card_id"`
	Filters  json.RawMessage `json:"filters"`
	Config   json.RawMessage `json:"config"`
	Timezone string          `json:"timezone"`
}

// updateDashboardAnnouncementRequest remains for callers of the pre-batch announcement endpoint.
type updateDashboardAnnouncementRequest struct {
	DashboardId int    `json:"dashboard_id"`
	Content     string `json:"content"`
	ContentMode string `json:"content_mode"`
	FileIds     []int  `json:"file_ids"`
	Revision    int64  `json:"revision"`
}

// updateDashboardUserStateRequest saves the requesting user's private filter state.
type updateDashboardUserStateRequest struct {
	Filters          json.RawMessage `json:"filters"`
	FiltersCollapsed *bool           `json:"filters_collapsed"`
}

// dashboardMetricConfigRequest is the fixed JSON shape for a metric card's config object.
type dashboardMetricConfigRequest struct {
	SLARecentDays     *int            `json:"sla_recent_days,omitempty"`
	SLAShowComparison bool            `json:"sla_show_comparison,omitempty"`
	SLALineColor      string          `json:"sla_line_color,omitempty"`
	SLAStageIds       []int           `json:"sla_stage_ids"`
	MetricType        string          `json:"metric_type"`
	DataScope         string          `json:"data_scope"`
	Filters           json.RawMessage `json:"filters"`
	RecentDays        int             `json:"recent_days"`
	Unit              string          `json:"unit"`
	Color             string          `json:"color"`
}

type dashboardRequestScope struct {
	AdminId     int
	WorkspaceId int
	UserId      int
}

// dashboardScope loads trusted request identity and the workspace path ID once per handler.
func dashboardScope(c *gin.Context, lang, stage string) (dashboardRequestScope, bool) {
	workspaceId, err := positiveParam(c, `wsId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.workspace_id`, err)
		return dashboardRequestScope{}, false
	}
	return dashboardRequestScope{
		AdminId: c.GetInt(`adminId`), WorkspaceId: workspaceId, UserId: c.GetInt(`userId`),
	}, true
}

// dashboardPathID validates the dashboard path ID for a handler.
func dashboardPathID(c *gin.Context, lang, stage string) (int, bool) {
	dashboardId, err := positiveParam(c, `dashboardId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.dashboard_id`, err)
		return 0, false
	}
	return dashboardId, true
}

// dashboardCardPathID validates the dashboard-card path ID for a handler.
func dashboardCardPathID(c *gin.Context, lang, stage string) (int, bool) {
	cardId, err := positiveParam(c, `cardId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.card_id`, err)
		return 0, false
	}
	return cardId, true
}

func dashboardObjectJSON(value string) (json.RawMessage, error) {
	value = strings.TrimSpace(value)
	if value == `` {
		return json.RawMessage(`{}`), nil
	}
	raw := json.RawMessage(value)
	if !json.Valid(raw) || raw[0] != '{' {
		return nil, errors.New(`dashboard object JSON is invalid`)
	}
	return raw, nil
}

func dashboardFiltersJSON(value string) (json.RawMessage, error) {
	value = strings.TrimSpace(value)
	if value == `` || value == `null` {
		return nil, nil
	}
	raw := json.RawMessage(value)
	if !json.Valid(raw) || raw[0] != '{' {
		return nil, errors.New(`dashboard filters JSON is invalid`)
	}
	return raw, nil
}

func toDashboardCardResponse(model business.DashboardCardModel) (dashboardCardResponse, error) {
	config, err := dashboardObjectJSON(model.Config)
	if err != nil {
		return dashboardCardResponse{}, err
	}
	return dashboardCardResponse{
		Id: model.Id, DashboardId: model.DashboardId, CardType: model.CardType, Title: model.Title,
		Config: config, Content: model.Content, ContentMode: model.ContentMode,
		X: model.X, Y: model.Y, W: model.W, H: model.H, Revision: model.Revision,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}, nil
}

func toDashboardResponse(model business.DashboardModel) (dashboardResponse, error) {
	cards := make([]dashboardCardResponse, 0, len(model.Cards))
	for _, card := range model.Cards {
		response, err := toDashboardCardResponse(card)
		if err != nil {
			return dashboardResponse{}, err
		}
		cards = append(cards, response)
	}
	return dashboardResponse{
		Id: model.Id, Name: model.Name, SortOrder: model.SortOrder, VisibilityType: model.VisibilityType,
		Revision: model.Revision, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
		CanEdit: model.CanEdit, CanManage: model.CanManage, CanReorder: model.CanReorder, Cards: cards,
	}, nil
}

func toDashboardAnnouncementResponse(model business.DashboardAnnouncementModel) dashboardAnnouncementResponse {
	return dashboardAnnouncementResponse{
		Id: model.Id, Content: model.Content, ContentMode: model.ContentMode, Revision: model.Revision,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}
}

// dashboardMetricTopFilters preserves the metric-request's three filter states: an omitted field
// reuses saved personal filters; explicit null or an empty filter object disables extra filters;
// a non-empty object is validated and used for this request only.
func dashboardMetricTopFilters(scope dashboardRequestScope, raw json.RawMessage) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	filters, err := dashboardFiltersString(raw, define.DashboardMetricDataScopeAll)
	if err != nil {
		return nil, err
	}
	if filters != `` {
		var sections define.DashboardFilters
		if err = json.Unmarshal([]byte(filters), &sections); err != nil {
			return nil, validate.ErrDashboardFiltersInvalid
		}
		if dashboardFilterSectionsEmpty(sections) {
			filters = ``
		}
	}
	if err = business.ValidateDashboardFilters(scope.AdminId, scope.WorkspaceId, filters, define.DashboardMetricDataScopeAll); err != nil {
		return nil, err
	}
	return &filters, nil
}

func dashboardFilterSectionsEmpty(sections define.DashboardFilters) bool {
	return len(sections.WorkItemTypes) == 0 && dashboardFilterNodeEmpty(sections.Common) && dashboardFilterNodeEmpty(sections.Requirement) && dashboardFilterNodeEmpty(sections.Defect)
}

func dashboardFilterNodeEmpty(node *define.ViewFilterNode) bool {
	if node == nil {
		return true
	}
	if !node.IsGroup() {
		return false
	}
	for index := range node.Conditions {
		if !dashboardFilterNodeEmpty(&node.Conditions[index]) {
			return false
		}
	}
	return true
}

func toDashboardPermissionResponse(model business.DashboardPermissionModel) dashboardPermissionResponse {
	return dashboardPermissionResponse{
		DashboardId: model.DashboardId, VisibilityType: model.VisibilityType,
		RoleIds: model.RoleIds, MemberIds: model.MemberIds, EditorIds: model.EditorIds, Revision: model.Revision,
	}
}

func toDashboardUserStateResponse(model business.DashboardUserStateModel) (dashboardUserStateResponse, error) {
	filters, err := dashboardFiltersJSON(model.Filters)
	if err != nil {
		return dashboardUserStateResponse{}, err
	}
	return dashboardUserStateResponse{
		Filters: filters, FiltersCollapsed: model.FiltersCollapsed, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt,
	}, nil
}

func dashboardConfigString(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if err := validate.DashboardConfig(raw); err != nil {
		return ``, err
	}
	if len(raw) == 0 {
		return ``, nil
	}
	return string(raw), nil
}

func dashboardFiltersString(raw json.RawMessage, dataScope string) (string, error) {
	raw = bytes.TrimSpace(raw)
	if err := validate.DashboardFilters(raw, dataScope); err != nil {
		return ``, err
	}
	if len(raw) == 0 || bytes.Equal(raw, []byte(`null`)) {
		return ``, nil
	}
	return string(raw), nil
}

func validateDashboardMetricConfig(scope dashboardRequestScope, raw json.RawMessage) (string, error) {
	config, err := dashboardConfigString(raw)
	if err != nil {
		return ``, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var metric dashboardMetricConfigRequest
	if err = decoder.Decode(&metric); err != nil {
		return ``, validate.ErrDashboardConfigInvalid
	}
	if err = validate.DashboardMetric(metric.MetricType, metric.DataScope); err != nil {
		return ``, err
	}
	if err = validate.DashboardMetricRecentDays(metric.RecentDays); err != nil {
		return ``, err
	}
	if err = validate.DashboardMetricStyle(metric.Unit, metric.Color); err != nil {
		return ``, err
	}
	if err = validate.DashboardSLAMetricOptions(metric.MetricType, metric.SLAStageIds, metric.SLAShowComparison, metric.SLALineColor, metric.SLARecentDays); err != nil {
		return ``, err
	}
	filters, err := dashboardFiltersString(metric.Filters, metric.DataScope)
	if err != nil {
		return ``, err
	}
	if err = business.ValidateDashboardFilters(scope.AdminId, scope.WorkspaceId, filters, metric.DataScope); err != nil {
		return ``, err
	}
	return config, nil
}

func validateDashboardCardInput(scope dashboardRequestScope, cardType, title string, config json.RawMessage, content, contentMode string, fileIds []int) (string, error) {
	if err := validate.DashboardCardType(cardType); err != nil {
		return ``, err
	}
	if err := validate.DashboardCardTitle(title); err != nil {
		return ``, err
	}
	if err := validate.DashboardContent(content); err != nil {
		return ``, err
	}
	if err := validate.DashboardContentMode(contentMode); err != nil {
		return ``, err
	}
	if err := validate.DashboardFileIDs(fileIds); err != nil {
		return ``, err
	}
	if cardType == define.DashboardCardTypeMetric {
		return validateDashboardMetricConfig(scope, config)
	}
	return dashboardConfigString(config)
}

func toReplaceDashboardLayoutModel(scope dashboardRequestScope, dashboardId int, req replaceDashboardLayoutRequest) (business.ReplaceDashboardLayoutModel, error) {
	if err := validate.DashboardRevision(req.Revision); err != nil {
		return business.ReplaceDashboardLayoutModel{}, err
	}
	items := make([]business.DashboardLayoutCardModel, 0, len(req.Cards))
	layout := make([]validate.DashboardLayoutItem, 0, len(req.Cards))
	persistedIDs := make(map[int]struct{}, len(req.Cards))
	for index, card := range req.Cards {
		if card.Id == nil && card.Revision != nil {
			return business.ReplaceDashboardLayoutModel{}, validate.ErrDashboardRevisionInvalid
		}
		cardId := 0
		revision := int64(0)
		if card.Id != nil {
			if *card.Id <= 0 {
				return business.ReplaceDashboardLayoutModel{}, validate.ErrDashboardLayoutInvalid
			}
			if card.Revision == nil || validate.DashboardRevision(*card.Revision) != nil {
				return business.ReplaceDashboardLayoutModel{}, validate.ErrDashboardRevisionInvalid
			}
			cardId = *card.Id
			revision = *card.Revision
			if _, exists := persistedIDs[cardId]; exists {
				return business.ReplaceDashboardLayoutModel{}, validate.ErrDashboardLayoutInvalid
			}
			persistedIDs[cardId] = struct{}{}
		}
		config, err := validateDashboardCardInput(scope, card.CardType, card.Title, card.Config, card.Content, card.ContentMode, card.FileIds)
		if err != nil {
			return business.ReplaceDashboardLayoutModel{}, err
		}
		items = append(items, business.DashboardLayoutCardModel{
			Id: cardId, CardType: card.CardType, Title: card.Title, Config: config,
			Content: card.Content, ContentMode: card.ContentMode, FileIds: card.FileIds,
			X: card.X, Y: card.Y, W: card.W, H: card.H, Revision: revision,
		})
		// validate.DashboardLayout requires positive unique IDs; the index gives new cards a
		// request-local identity without leaking that transport detail into the business model.
		layout = append(layout, validate.DashboardLayoutItem{Id: index + 1, X: card.X, Y: card.Y, W: card.W, H: card.H})
	}
	if err := validate.DashboardLayout(layout); err != nil {
		return business.ReplaceDashboardLayoutModel{}, err
	}
	deleted := make([]business.DashboardDeletedCardModel, 0, len(req.DeletedCards))
	deletedIDs := make(map[int]struct{}, len(req.DeletedCards))
	for _, card := range req.DeletedCards {
		if card.Id <= 0 || validate.DashboardRevision(card.Revision) != nil {
			return business.ReplaceDashboardLayoutModel{}, validate.ErrDashboardRevisionInvalid
		}
		if _, exists := persistedIDs[card.Id]; exists {
			return business.ReplaceDashboardLayoutModel{}, validate.ErrDashboardLayoutInvalid
		}
		if _, exists := deletedIDs[card.Id]; exists {
			return business.ReplaceDashboardLayoutModel{}, validate.ErrDashboardLayoutInvalid
		}
		deletedIDs[card.Id] = struct{}{}
		deleted = append(deleted, business.DashboardDeletedCardModel{CardId: card.Id, Revision: card.Revision})
	}
	var announcement *business.DashboardAnnouncementMutationModel
	if req.Announcement != nil {
		if err := validate.DashboardContent(req.Announcement.Content); err != nil {
			return business.ReplaceDashboardLayoutModel{}, err
		}
		if err := validate.DashboardContentMode(req.Announcement.ContentMode); err != nil {
			return business.ReplaceDashboardLayoutModel{}, err
		}
		if err := validate.DashboardFileIDs(req.Announcement.FileIds); err != nil {
			return business.ReplaceDashboardLayoutModel{}, err
		}
		if err := validate.DashboardRevision(req.Announcement.Revision); err != nil {
			return business.ReplaceDashboardLayoutModel{}, err
		}
		announcement = &business.DashboardAnnouncementMutationModel{
			Content: req.Announcement.Content, ContentMode: req.Announcement.ContentMode,
			FileIds: req.Announcement.FileIds, Revision: req.Announcement.Revision,
		}
	}
	return business.ReplaceDashboardLayoutModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, Revision: req.Revision, Cards: items, DeletedCards: deleted, Announcement: announcement,
	}, nil
}

func dashboardError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrDashboardNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgDashboardNotFound))
	case errors.Is(err, business.ErrDashboardForbidden):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgDashboardForbidden))
	case errors.Is(err, business.ErrDashboardFeatureDisabled):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgDashboardFeatureDisabled))
	case errors.Is(err, business.ErrDashboardRevisionConflict):
		abortConflict(c, lib_i18n.Show(lang, define.MsgDashboardRevisionConflict))
	case errors.Is(err, business.ErrDashboardAnnouncementExists):
		abortConflict(c, lib_i18n.Show(lang, define.MsgDashboardAnnouncementExists))
	case errors.Is(err, business.ErrDashboardInvalidLayout), errors.Is(err, validate.ErrDashboardLayoutInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgDashboardInvalidLayout), stage, err)
	case errors.Is(err, business.ErrDashboardInvalid), errors.Is(err, business.ErrDashboardInvalidMetric),
		errors.Is(err, business.ErrDashboardInvalidFilters), errors.Is(err, business.ErrDashboardAttachmentInvalid),
		errors.Is(err, business.ErrDashboardAttachmentTooLarge), errors.Is(err, business.ErrDashboardAttachmentNotFound),
		errors.Is(err, validate.ErrDashboardNameInvalid), errors.Is(err, validate.ErrDashboardCardTitleInvalid),
		errors.Is(err, validate.ErrDashboardCardTypeInvalid), errors.Is(err, validate.ErrDashboardContentModeInvalid),
		errors.Is(err, validate.ErrDashboardContentInvalid), errors.Is(err, validate.ErrDashboardConfigInvalid),
		errors.Is(err, validate.ErrDashboardFiltersInvalid), errors.Is(err, validate.ErrDashboardVisibilityInvalid),
		errors.Is(err, validate.ErrDashboardOrderInvalid), errors.Is(err, validate.ErrDashboardRevisionInvalid),
		errors.Is(err, validate.ErrDashboardMetricInvalid), errors.Is(err, validate.ErrDashboardFileIDsInvalid):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

func dashboardAnnouncementID(c *gin.Context, lang, stage string) (int, bool) {
	dashboardId, err := strconv.Atoi(c.Query(`dashboard_id`))
	if err != nil || dashboardId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), stage+`.dashboard_id`, errInvalidID)
		return 0, false
	}
	return dashboardId, true
}

func dashboardUploadRequiredMessage(isImage, isVideo bool, lang string) string {
	if isImage {
		return lib_i18n.Show(lang, define.MsgImageRequired)
	}
	if isVideo {
		return lib_i18n.Show(lang, define.MsgVideoRequired)
	}
	return lib_i18n.Show(lang, define.MsgFileRequired)
}

func dashboardUploadValidationMessage(fileName string, fileSize int64, isImage, isVideo bool, lang string) string {
	if fileSize <= 0 {
		return dashboardUploadRequiredMessage(isImage, isVideo, lang)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), `.`)
	if isImage {
		if fileSize > business.MaxImageSize {
			return lib_i18n.Show(lang, define.MsgImageTooLarge)
		}
		if !business.IsAllowedImageExt(ext) {
			return lib_i18n.Show(lang, define.MsgImageTypeInvalid)
		}
		return ``
	}
	if isVideo {
		if fileSize > business.MaxEditorVideoSize {
			return lib_i18n.Show(lang, define.MsgVideoTooLarge)
		}
		if !business.IsAllowedEditorVideoExt(ext) {
			return lib_i18n.Show(lang, define.MsgVideoTypeInvalid)
		}
		return ``
	}
	if fileSize > business.MaxEditorFileSize {
		return lib_i18n.Show(lang, define.MsgFileTooLarge)
	}
	if !business.IsAllowedEditorFileExt(ext) {
		return lib_i18n.Show(lang, define.MsgFileTypeInvalid)
	}
	return ``
}

// ListDashboards returns the dashboards that the authenticated workspace member may view.
func ListDashboards(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.list`)
	if !ok {
		return
	}
	models, err := business.ListDashboards(business.DashboardListModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.list`, err)
		return
	}
	items := make([]dashboardResponse, 0, len(models))
	for _, model := range models {
		item, convertErr := toDashboardResponse(model)
		if convertErr != nil {
			abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.list.response`, convertErr)
			return
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: items,
	})
}

// GetDashboard returns one visible dashboard and its active cards.
func GetDashboard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.get`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.get`)
	if !ok {
		return
	}
	model, err := business.GetDashboard(business.GetDashboardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.get`, err)
		return
	}
	response, err := toDashboardResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.get.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// CreateDashboard creates a shared dashboard for an actor with dashboard:create.
func CreateDashboard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.create`)
	if !ok {
		return
	}
	var req createDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.create.bind`, err)
		return
	}
	if err := validate.DashboardName(req.Name); err != nil {
		dashboardError(c, lang, `dashboard.create.name`, err)
		return
	}
	model, err := business.CreateDashboard(business.CreateDashboardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, Name: req.Name, Template: req.Template,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.create`, err)
		return
	}
	response, err := toDashboardResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.create.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// UpdateDashboard renames an editable dashboard under optimistic concurrency control.
func UpdateDashboard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.update`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.update`)
	if !ok {
		return
	}
	var req updateDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.update.bind`, err)
		return
	}
	if req.Name == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.update.name`, validate.ErrDashboardNameInvalid)
		return
	}
	if err := validate.DashboardName(*req.Name); err != nil {
		dashboardError(c, lang, `dashboard.update.name`, err)
		return
	}
	if err := validate.DashboardRevision(req.Revision); err != nil {
		dashboardError(c, lang, `dashboard.update.revision`, err)
		return
	}
	model, err := business.UpdateDashboard(business.UpdateDashboardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, Name: req.Name, Revision: req.Revision,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.update`, err)
		return
	}
	response, err := toDashboardResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.update.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// DeleteDashboard soft-deletes one manageable dashboard while preserving the final dashboard.
func DeleteDashboard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.delete`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.delete`)
	if !ok {
		return
	}
	var req deleteDashboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.delete.bind`, err)
		return
	}
	if err := validate.DashboardRevision(req.Revision); err != nil {
		dashboardError(c, lang, `dashboard.delete.revision`, err)
		return
	}
	if err := business.DeleteDashboard(business.DeleteDashboardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, Revision: req.Revision,
	}); err != nil {
		dashboardError(c, lang, `dashboard.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// ReorderDashboards saves the complete shared order. Business verifies that a non-manager did
// not move any dashboard it cannot edit relative to another such dashboard.
func ReorderDashboards(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.reorder`)
	if !ok {
		return
	}
	var req reorderDashboardsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.reorder.bind`, err)
		return
	}
	if err := validate.DashboardOrder(req.Ids); err != nil {
		dashboardError(c, lang, `dashboard.reorder.ids`, err)
		return
	}
	items := make([]business.DashboardOrderItemModel, 0, len(req.Ids))
	for index, dashboardId := range req.Ids {
		items = append(items, business.DashboardOrderItemModel{DashboardId: dashboardId, SortOrder: index + 1})
	}
	if err := business.ReorderDashboards(business.ReorderDashboardsModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, Items: items,
	}); err != nil {
		dashboardError(c, lang, `dashboard.reorder`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// SaveDashboardAs copies a visible dashboard into a new shared dashboard under a source revision.
func SaveDashboardAs(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.save_as`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.save_as`)
	if !ok {
		return
	}
	var req saveDashboardAsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.save_as.bind`, err)
		return
	}
	if err := validate.DashboardName(req.Name); err != nil {
		dashboardError(c, lang, `dashboard.save_as.name`, err)
		return
	}
	if err := validate.DashboardRevision(req.Revision); err != nil {
		dashboardError(c, lang, `dashboard.save_as.revision`, err)
		return
	}
	model, err := business.SaveDashboardAs(business.SaveDashboardAsModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, Name: req.Name, Revision: req.Revision,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.save_as`, err)
		return
	}
	response, err := toDashboardResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.save_as.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// GetDashboardPermission returns visibility and editor IDs only to a dashboard manager.
func GetDashboardPermission(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.permission.get`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.permission.get`)
	if !ok {
		return
	}
	model, err := business.GetDashboardPermission(business.DashboardPermissionModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.permission.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toDashboardPermissionResponse(model),
	})
}

// ReplaceDashboardPermission replaces the manager-only visibility and additional-editor configuration.
func ReplaceDashboardPermission(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.permission.replace`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.permission.replace`)
	if !ok {
		return
	}
	var req replaceDashboardPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.permission.replace.bind`, err)
		return
	}
	if err := validate.DashboardVisibility(req.VisibilityType, req.RoleIds, req.MemberIds, req.EditorIds); err != nil {
		dashboardError(c, lang, `dashboard.permission.replace.visibility`, err)
		return
	}
	if err := validate.DashboardRevision(req.Revision); err != nil {
		dashboardError(c, lang, `dashboard.permission.replace.revision`, err)
		return
	}
	model, err := business.ReplaceDashboardPermission(business.DashboardPermissionModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
		VisibilityType: req.VisibilityType, RoleIds: req.RoleIds, MemberIds: req.MemberIds,
		EditorIds: req.EditorIds, Revision: req.Revision,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.permission.replace`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toDashboardPermissionResponse(model),
	})
}

// ReplaceDashboardLayout atomically saves every card mutation, card removal, layout rectangle,
// and optional project-announcement update for one editable dashboard.
func ReplaceDashboardLayout(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.layout.replace`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.layout.replace`)
	if !ok {
		return
	}
	var req replaceDashboardLayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.layout.replace.bind`, err)
		return
	}
	model, err := toReplaceDashboardLayoutModel(scope, dashboardId, req)
	if err != nil {
		dashboardError(c, lang, `dashboard.layout.replace.validate`, err)
		return
	}
	result, err := business.ReplaceDashboardLayout(model)
	if err != nil {
		dashboardError(c, lang, `dashboard.layout.replace`, err)
		return
	}
	response, err := toDashboardResponse(result)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.layout.replace.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// CreateDashboardCard remains for callers that have not yet switched to atomic layout saves.
func CreateDashboardCard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.card.create`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.card.create`)
	if !ok {
		return
	}
	var req createDashboardCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.card.create.bind`, err)
		return
	}
	config, err := validateDashboardCardInput(scope, req.CardType, req.Title, req.Config, req.Content, req.ContentMode, req.FileIds)
	if err != nil {
		dashboardError(c, lang, `dashboard.card.create.validate`, err)
		return
	}
	if err = validate.DashboardLayout([]validate.DashboardLayoutItem{{Id: 1, X: req.X, Y: req.Y, W: req.W, H: req.H}}); err != nil {
		dashboardError(c, lang, `dashboard.card.create.layout`, err)
		return
	}
	model, err := business.CreateDashboardCard(business.CreateDashboardCardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
		CardType: req.CardType, Title: req.Title, Config: config, Content: req.Content, ContentMode: req.ContentMode,
		FileIds: req.FileIds, X: req.X, Y: req.Y, W: req.W, H: req.H,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.card.create`, err)
		return
	}
	response, err := toDashboardCardResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.card.create.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// UpdateDashboardCard remains for callers that have not yet switched to atomic layout saves.
func UpdateDashboardCard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.card.update`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.card.update`)
	if !ok {
		return
	}
	cardId, ok := dashboardCardPathID(c, lang, `dashboard.card.update`)
	if !ok {
		return
	}
	var req updateDashboardCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.card.update.bind`, err)
		return
	}
	if err := validate.DashboardRevision(req.Revision); err != nil {
		dashboardError(c, lang, `dashboard.card.update.revision`, err)
		return
	}
	if req.Title != nil {
		if err := validate.DashboardCardTitle(*req.Title); err != nil {
			dashboardError(c, lang, `dashboard.card.update.title`, err)
			return
		}
	}
	if req.Content != nil {
		if err := validate.DashboardContent(*req.Content); err != nil {
			dashboardError(c, lang, `dashboard.card.update.content`, err)
			return
		}
	}
	if req.ContentMode != nil {
		if err := validate.DashboardContentMode(*req.ContentMode); err != nil {
			dashboardError(c, lang, `dashboard.card.update.content_mode`, err)
			return
		}
	}
	if err := validate.DashboardFileIDs(req.FileIds); err != nil {
		dashboardError(c, lang, `dashboard.card.update.file_ids`, err)
		return
	}
	var config *string
	if req.Config != nil {
		encoded, err := dashboardConfigString(*req.Config)
		if err != nil {
			dashboardError(c, lang, `dashboard.card.update.config`, err)
			return
		}
		config = &encoded
	}
	if (req.X != nil && *req.X < 0) || (req.Y != nil && *req.Y < 0) ||
		(req.W != nil && *req.W < 1) || (req.H != nil && *req.H < 1) {
		dashboardError(c, lang, `dashboard.card.update.layout`, validate.ErrDashboardLayoutInvalid)
		return
	}
	model, err := business.UpdateDashboardCard(business.UpdateDashboardCardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, CardId: cardId, Title: req.Title, Config: config, Content: req.Content,
		ContentMode: req.ContentMode, FileIds: req.FileIds, X: req.X, Y: req.Y, W: req.W, H: req.H, Revision: req.Revision,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.card.update`, err)
		return
	}
	response, err := toDashboardCardResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.card.update.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// DeleteDashboardCard soft-deletes a card under its own revision guard.
func DeleteDashboardCard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.card.delete`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.card.delete`)
	if !ok {
		return
	}
	cardId, ok := dashboardCardPathID(c, lang, `dashboard.card.delete`)
	if !ok {
		return
	}
	var req deleteDashboardCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.card.delete.bind`, err)
		return
	}
	if err := validate.DashboardRevision(req.Revision); err != nil {
		dashboardError(c, lang, `dashboard.card.delete.revision`, err)
		return
	}
	if err := business.DeleteDashboardCard(business.DeleteDashboardCardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, CardId: cardId, Revision: req.Revision,
	}); err != nil {
		dashboardError(c, lang, `dashboard.card.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// CopyDashboardCard copies a numeric metric card to another editable dashboard in the workspace.
func CopyDashboardCard(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.card.copy`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.card.copy`)
	if !ok {
		return
	}
	cardId, ok := dashboardCardPathID(c, lang, `dashboard.card.copy`)
	if !ok {
		return
	}
	var req copyDashboardCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.card.copy.bind`, err)
		return
	}
	if req.TargetDashboardId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.card.copy.target_dashboard_id`, errInvalidID)
		return
	}
	model, err := business.CopyDashboardCard(business.CopyDashboardCardModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, CardId: cardId, TargetDashboardId: req.TargetDashboardId,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.card.copy`, err)
		return
	}
	response, err := toDashboardCardResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.card.copy.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// GetDashboardAnnouncement returns the project-wide announcement through one visible dashboard.
func GetDashboardAnnouncement(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.announcement.get`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardAnnouncementID(c, lang, `dashboard.announcement.get`)
	if !ok {
		return
	}
	model, err := business.GetDashboardAnnouncement(business.GetDashboardAnnouncementModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.announcement.get`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toDashboardAnnouncementResponse(model),
	})
}

// UpdateDashboardAnnouncement remains for callers that have not yet switched to atomic layout saves.
func UpdateDashboardAnnouncement(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.announcement.update`)
	if !ok {
		return
	}
	var req updateDashboardAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.announcement.update.bind`, err)
		return
	}
	if req.DashboardId <= 0 {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.announcement.update.dashboard_id`, errInvalidID)
		return
	}
	if err := validate.DashboardContent(req.Content); err != nil {
		dashboardError(c, lang, `dashboard.announcement.update.content`, err)
		return
	}
	if err := validate.DashboardContentMode(req.ContentMode); err != nil {
		dashboardError(c, lang, `dashboard.announcement.update.content_mode`, err)
		return
	}
	if err := validate.DashboardFileIDs(req.FileIds); err != nil {
		dashboardError(c, lang, `dashboard.announcement.update.file_ids`, err)
		return
	}
	if err := validate.DashboardRevision(req.Revision); err != nil {
		dashboardError(c, lang, `dashboard.announcement.update.revision`, err)
		return
	}
	model, err := business.UpdateDashboardAnnouncement(business.UpdateDashboardAnnouncementModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: req.DashboardId,
		Content: req.Content, ContentMode: req.ContentMode, FileIds: req.FileIds, Revision: req.Revision,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.announcement.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: toDashboardAnnouncementResponse(model),
	})
}

// GetDashboardUserState returns only the requesting user's private filters and collapse state.
func GetDashboardUserState(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.user_state.get`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.user_state.get`)
	if !ok {
		return
	}
	model, err := business.GetDashboardUserState(business.DashboardUserStateModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.user_state.get`, err)
		return
	}
	response, err := toDashboardUserStateResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.user_state.get.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// UpdateDashboardUserState saves only the requesting user's private filters and collapse state.
func UpdateDashboardUserState(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.user_state.update`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.user_state.update`)
	if !ok {
		return
	}
	var req updateDashboardUserStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.user_state.update.bind`, err)
		return
	}
	if req.FiltersCollapsed == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.user_state.update.filters_collapsed`, errInvalidID)
		return
	}
	filters, err := dashboardFiltersString(req.Filters, define.DashboardMetricDataScopeAll)
	if err != nil {
		dashboardError(c, lang, `dashboard.user_state.update.filters`, err)
		return
	}
	if err = business.ValidateDashboardFilters(scope.AdminId, scope.WorkspaceId, filters, define.DashboardMetricDataScopeAll); err != nil {
		dashboardError(c, lang, `dashboard.user_state.update.filters`, err)
		return
	}
	model, err := business.UpdateDashboardUserState(business.DashboardUserStateModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId, DashboardId: dashboardId,
		Filters: filters, FiltersCollapsed: *req.FiltersCollapsed,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.user_state.update`, err)
		return
	}
	response, err := toDashboardUserStateResponse(model)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `dashboard.user_state.update.response`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: response,
	})
}

// QueryDashboardMetrics calculates one metric card. Omitted filters read the persisted user
// state; explicit null/empty filters deliberately mean no additional top-level condition.
func QueryDashboardMetrics(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.metric.query`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.metric.query`)
	if !ok {
		return
	}
	var req dashboardMetricQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.metric.query.bind`, err)
		return
	}
	if req.CardId <= 0 && req.Config == nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `dashboard.metric.query.card_id`, errInvalidID)
		return
	}
	var previewConfig *string
	if req.Config != nil {
		config, err := validateDashboardMetricConfig(scope, req.Config)
		if err != nil {
			dashboardError(c, lang, `dashboard.metric.query.config`, err)
			return
		}
		previewConfig = &config
	}
	topFilters, err := dashboardMetricTopFilters(scope, req.Filters)
	if err != nil {
		dashboardError(c, lang, `dashboard.metric.query.filters`, err)
		return
	}
	result, err := business.QueryDashboardMetric(business.DashboardMetricQueryModel{
		AdminId: scope.AdminId, WorkspaceId: scope.WorkspaceId, UserId: scope.UserId,
		DashboardId: dashboardId, CardId: req.CardId, TopFilters: topFilters, PreviewConfig: previewConfig, Timezone: req.Timezone,
	})
	if err != nil {
		dashboardError(c, lang, `dashboard.metric.query`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: dashboardMetricResponse{
			MetricType: result.MetricType, DataScope: result.DataScope, Value: dashboardMetricValue(result), SLA: toDashboardSLAStatsResponse(result),
		},
	})
}

func uploadDashboardAttachment(c *gin.Context, isImage, isVideo bool) {
	lang := lib_i18n.GetLang(c)
	scope, ok := dashboardScope(c, lang, `dashboard.attachment.upload`)
	if !ok {
		return
	}
	dashboardId, ok := dashboardPathID(c, lang, `dashboard.attachment.upload`)
	if !ok {
		return
	}
	file, err := c.FormFile(`file`)
	if err != nil || file == nil {
		abortBadRequest(c, dashboardUploadRequiredMessage(isImage, isVideo, lang), `dashboard.attachment.upload.file`, err)
		return
	}
	if message := dashboardUploadValidationMessage(file.Filename, file.Size, isImage, isVideo, lang); message != `` {
		abortBadRequest(c, message, `dashboard.attachment.upload.validate`, errors.New(`dashboard attachment upload validation failed`))
		return
	}
	attachment, err := business.UploadDashboardAttachment(scope.AdminId, scope.WorkspaceId, scope.UserId, dashboardId, file, isImage, isVideo)
	if err != nil {
		dashboardError(c, lang, `dashboard.attachment.upload`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess), Data: dashboardAttachmentResponse{
			FileId: attachment.FileId, Url: attachment.Url, OriginalName: attachment.OriginalName,
			MimeType: attachment.MimeType, FileSize: attachment.FileSize,
		},
	})
}

// UploadDashboardImage uploads an image usable only by the target dashboard's editor flow.
func UploadDashboardImage(c *gin.Context) {
	uploadDashboardAttachment(c, true, false)
}

// UploadDashboardFile uploads a document usable only by the target dashboard's editor flow.
func UploadDashboardFile(c *gin.Context) {
	uploadDashboardAttachment(c, false, false)
}

// UploadDashboardVideo uploads a video usable only by the target dashboard's editor flow.
func UploadDashboardVideo(c *gin.Context) {
	uploadDashboardAttachment(c, false, true)
}
