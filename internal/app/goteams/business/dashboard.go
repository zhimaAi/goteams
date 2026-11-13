// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// Dashboard errors are deliberately separate from generic permission errors so controllers can
// preserve the dashboard API's 404/403/409 response contract without exposing database details.
var (
	ErrDashboardNotFound           = errors.New(`dashboard not found`)
	ErrDashboardForbidden          = errors.New(`dashboard forbidden`)
	ErrDashboardRevisionConflict   = errors.New(`dashboard revision conflict`)
	ErrDashboardFeatureDisabled    = errors.New(`dashboard feature disabled`)
	ErrDashboardAnnouncementExists = errors.New(`dashboard announcement card already exists`)
	ErrDashboardInvalidLayout      = errors.New(`dashboard layout is invalid`)
	ErrDashboardInvalid            = errors.New(`dashboard is invalid`)
	ErrDashboardInvalidMetric      = errors.New(`dashboard metric is invalid`)
	ErrDashboardInvalidFilters     = errors.New(`dashboard filters are invalid`)
	ErrDashboardAttachmentInvalid  = errors.New(`dashboard attachment is invalid`)
	ErrDashboardAttachmentTooLarge = errors.New(`dashboard attachment is too large`)
	ErrDashboardAttachmentNotFound = errors.New(`dashboard attachment not found`)
)

const (
	dashboardDefaultName                = `项目概览`
	dashboardDefaultAnnouncementContent = `<p><span style="color: rgb(124, 133, 151);">欢迎使用GoTeams</span></p><p>你可以在项目仪表盘添加卡片，关注重点事项及相关统计卡片，预知项目风险，关注项目质量和进度把控。</p><p>每项目仅支持一份项目公告，即项目中不同项目仪表盘的项目公告内容是一样的。</p><p>更多使用秘籍请查看帮助文档：<a target="_blank" rel="noopener noreferrer" href="https://www.yuque.com/zhimaxiaoshiwangluo/vgyo1p?#">帮助文档</a></p>`
	dashboardProjectAdminRoleName       = `perm_role_project_admin`
)

// DashboardRevisionConflictError carries the latest server revision for optimistic-lock callers.
type DashboardRevisionConflictError struct {
	CurrentRevision int64
}

// Error implements error.
func (e *DashboardRevisionConflictError) Error() string {
	return ErrDashboardRevisionConflict.Error()
}

// Unwrap lets controllers test the stable revision-conflict sentinel.
func (e *DashboardRevisionConflictError) Unwrap() error {
	return ErrDashboardRevisionConflict
}

// DashboardModel describes one shared workspace dashboard.
type DashboardModel struct {
	Id             int
	Name           string
	SortOrder      int
	VisibilityType string
	RoleIds        []int
	MemberIds      []int
	EditorIds      []int
	CreatorId      int
	Revision       int64
	CreatedAt      int64
	UpdatedAt      int64
	CanEdit        bool
	CanManage      bool
	CanReorder     bool
	Cards          []DashboardCardModel
}

// DashboardCardModel describes a persisted dashboard card.
type DashboardCardModel struct {
	Id          int
	DashboardId int
	CardType    string
	Title       string
	Config      string
	Content     string
	ContentMode string
	X           int
	Y           int
	W           int
	H           int
	Revision    int64
	CreatedBy   int
	CreatedAt   int64
	UpdatedAt   int64
}

// DashboardListModel is the authenticated context of a dashboard-list request.
type DashboardListModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
}

// GetDashboardModel is the authenticated context of a single-dashboard read.
type GetDashboardModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
}

// CreateDashboardModel describes a new shared dashboard.
type CreateDashboardModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	Name         string
	Template     string
}

// UpdateDashboardModel describes a dashboard-name update guarded by a revision.
type UpdateDashboardModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	Name         *string
	Revision     int64
}

// DeleteDashboardModel describes a dashboard soft-delete request.
type DeleteDashboardModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	Revision     int64
}

// DashboardOrderItemModel is a dashboard's new shared position.
type DashboardOrderItemModel struct {
	DashboardId int
	SortOrder   int
}

// ReorderDashboardsModel describes a complete shared dashboard order. Non-administrators may move
// their editable dashboards only; the relative order of every other dashboard must remain intact.
type ReorderDashboardsModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	Items        []DashboardOrderItemModel
}

// DashboardPermissionModel describes one dashboard's visibility and extra editors.
type DashboardPermissionModel struct {
	AdminId        int
	WorkspaceId    int
	UserId         int
	ActorIsAdmin   bool
	DashboardId    int
	VisibilityType string
	RoleIds        []int
	MemberIds      []int
	EditorIds      []int
	Revision       int64
}

// SaveDashboardAsModel describes a dashboard copy within one workspace.
type SaveDashboardAsModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	Name         string
	Revision     int64
}

// DashboardLayoutItemModel is one card rectangle in the 12-column layout.
type DashboardLayoutItemModel struct {
	CardId int
	X      int
	Y      int
	W      int
	H      int
}

// DashboardLayoutCardModel is one complete card snapshot in an atomic dashboard layout save.
// Id is zero for a new card; Revision is required for an existing card.
type DashboardLayoutCardModel struct {
	Id          int
	CardType    string
	Title       string
	Config      string
	Content     string
	ContentMode string
	FileIds     []int
	X           int
	Y           int
	W           int
	H           int
	Revision    int64
}

// DashboardDeletedCardModel identifies one persisted card removed by an atomic layout save.
type DashboardDeletedCardModel struct {
	CardId   int
	Revision int64
}

// DashboardAnnouncementMutationModel is the optional shared project announcement update in an
// atomic layout save. A nil pointer means that announcement content is unchanged.
type DashboardAnnouncementMutationModel struct {
	Content     string
	ContentMode string
	FileIds     []int
	Revision    int64
}

// ReplaceDashboardLayoutModel atomically saves the complete editable card state under a
// dashboard revision. Cards and DeletedCards must together account for every active card.
type ReplaceDashboardLayoutModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	Revision     int64
	Cards        []DashboardLayoutCardModel
	DeletedCards []DashboardDeletedCardModel
	Announcement *DashboardAnnouncementMutationModel
}

// CreateDashboardCardModel describes a new dashboard card. FileIds are newly uploaded, unbound
// dashboard editor assets that become owned by the card after a successful save.
type CreateDashboardCardModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	CardType     string
	Title        string
	Config       string
	Content      string
	ContentMode  string
	X            int
	Y            int
	W            int
	H            int
	FileIds      []int
}

// UpdateDashboardCardModel describes a partial card update guarded by the card revision.
type UpdateDashboardCardModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	CardId       int
	Title        *string
	Config       *string
	Content      *string
	ContentMode  *string
	X            *int
	Y            *int
	W            *int
	H            *int
	FileIds      []int
	Revision     int64
}

// DeleteDashboardCardModel describes a card soft-delete request.
type DeleteDashboardCardModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	CardId       int
	Revision     int64
}

// CopyDashboardCardModel copies a numeric metric card to an editable target dashboard.
type CopyDashboardCardModel struct {
	AdminId           int
	WorkspaceId       int
	UserId            int
	ActorIsAdmin      bool
	DashboardId       int
	CardId            int
	TargetDashboardId int
}

// DashboardMetricConfig is the JSON shape kept in a metric card's Config column.
// Filters is a GanttViewFilters JSON object, or an empty string when no condition is configured.
type DashboardMetricConfig struct {
	SLARecentDays     *int            `json:"sla_recent_days,omitempty"`
	SLAShowComparison bool            `json:"sla_show_comparison,omitempty"`
	SLALineColor      string          `json:"sla_line_color,omitempty"`
	SLAStageIds       []int           `json:"sla_stage_ids,omitempty"`
	SLAStageName      string          `json:"-"`
	MetricType        string          `json:"metric_type"`
	DataScope         string          `json:"data_scope"`
	Filters           json.RawMessage `json:"filters"`
	// RecentDays is an optional rolling "recent N days" window on created_at;
	// 0 (absent) means no time limit. The window is resolved at query time.
	RecentDays int    `json:"recent_days,omitempty"`
	Unit       string `json:"unit,omitempty"`
	Color      string `json:"color,omitempty"`
}

// DashboardMetricQueryModel describes an on-demand metric calculation. A nil TopFilters uses the
// persisted personal state; a non-nil value is the current request state (including explicit empty).
// It is strictly validated and can only narrow the query; data-view scope remains server-side.
type DashboardMetricQueryModel struct {
	AdminId       int
	WorkspaceId   int
	UserId        int
	ActorIsAdmin  bool
	DashboardId   int
	CardId        int
	TopFilters    *string
	PreviewConfig *string
	Timezone      string
}

// DashboardMetricResultModel is one rendered metric value.
type DashboardMetricResultModel struct {
	SLA        *DashboardSLAStatsModel
	MetricType string
	DataScope  string
	Value      float64
}

// DashboardAnnouncementModel is the project-wide announcement shown by all announcement cards.
type DashboardAnnouncementModel struct {
	Id          int
	AdminId     int
	WorkspaceId int
	Content     string
	ContentMode string
	Revision    int64
	CreatedBy   int
	UpdatedBy   int
	CreatedAt   int64
	UpdatedAt   int64
}

// GetDashboardAnnouncementModel reads the project announcement through a particular visible dashboard.
type GetDashboardAnnouncementModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
}

// UpdateDashboardAnnouncementModel updates the project announcement through one editable dashboard.
type UpdateDashboardAnnouncementModel struct {
	AdminId      int
	WorkspaceId  int
	UserId       int
	ActorIsAdmin bool
	DashboardId  int
	Content      string
	ContentMode  string
	FileIds      []int
	Revision     int64
}

// DashboardUserStateModel is one user's personal filter and collapse state for one dashboard.
type DashboardUserStateModel struct {
	AdminId          int
	WorkspaceId      int
	UserId           int
	ActorIsAdmin     bool
	DashboardId      int
	Filters          string
	FiltersCollapsed bool
	CreatedAt        int64
	UpdatedAt        int64
}

// DashboardAttachmentModel is a pending dashboard rich-text asset returned after upload.
type DashboardAttachmentModel struct {
	FileId       int
	Url          string
	OriginalName string
	MimeType     string
	FileSize     int64
}

type dashboardRelations struct {
	RoleIds   []int
	MemberIds []int
	EditorIds []int
}

// EnsureDefaultDashboardForWorkspace creates a workspace's first dashboard and announcement in
// one transaction. UpdateWorkspaceApplication uses the Tx variant so app enable and defaults are atomic.
func EnsureDefaultDashboardForWorkspace(adminId, workspaceId, userId int) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin ensure dashboard defaults transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = EnsureDefaultDashboardForWorkspaceTx(tx, adminId, workspaceId, userId); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit ensure dashboard defaults transaction failed, err:%s", err.Error())
	}
	return err
}

// EnsureDefaultDashboardForWorkspaceTx creates missing defaults in the caller's transaction.
// The workspace row lock serializes concurrent first-enable requests.
func EnsureDefaultDashboardForWorkspaceTx(tx *sql.Tx, adminId, workspaceId, userId int) error {
	if tx == nil || adminId <= 0 || workspaceId <= 0 || userId <= 0 {
		return ErrDashboardInvalid
	}
	if err := lockDashboardWorkspace(tx, adminId, workspaceId); err != nil {
		return err
	}
	now := int64(tool.Time2Int())
	if err := ensureDashboardAnnouncementTx(tx, adminId, workspaceId, userId, now); err != nil {
		return err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkspaceDashboards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0 LIMIT 1 FOR UPDATE`, tx, adminId, workspaceId)
	if err != nil {
		logs.Error("load default dashboard failed, err:%s", err.Error())
		return err
	}
	if len(rows) > 0 {
		return nil
	}
	rows, err = msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkspaceDashboards+`
		(admin_id, workspace_id, name, visibility_type, sort_order, creator_id, revision, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, 1, $6, $6) RETURNING id`, tx,
		adminId, workspaceId, dashboardDefaultName, define.DashboardVisibilityPublic, userId, now)
	if err != nil {
		logs.Error("create default dashboard failed, err:%s", err.Error())
		return err
	}
	// Seed the default dashboard with the "project overview" starter cards. They are
	// ordinary cards: editors can move, edit and delete them like any other card.
	dashboardId := cast.ToInt(rows[0][`id`])
	cards, err := dashboardDefaultCardsForWorkspaceTx(tx, adminId, workspaceId)
	if err != nil {
		return err
	}
	for _, card := range cards {
		if _, err = insertDashboardCardTx(tx, adminId, workspaceId, dashboardId, userId, card, now); err != nil {
			logs.Error("seed default dashboard card failed, err:%s", err.Error())
			return err
		}
	}
	return nil
}

// dashboardDefaultCards returns the starter cards seeded into every new default dashboard.
// Titles are stored as card data (renameable), matching the existing default announcement content.
func dashboardDefaultCards() []DashboardCardModel {
	mode := define.DashboardContentModeRichText
	return []DashboardCardModel{
		{CardType: define.DashboardCardTypeProjectInfo, Title: `项目信息`, ContentMode: mode, X: 0, Y: 0, W: 6, H: 3},
		{CardType: define.DashboardCardTypeAnnouncement, Title: `项目公告`, ContentMode: mode, X: 6, Y: 0, W: 6, H: 3},
		{CardType: define.DashboardCardTypeMetric, Title: `近30天新增需求`, ContentMode: mode, X: 0, Y: 3, W: 3, H: 2,
			Config: `{"metric_type":"created_requirement_count","data_scope":"requirement","recent_days":30}`},
		{CardType: define.DashboardCardTypeMetric, Title: `已延期需求数`, ContentMode: mode, X: 3, Y: 3, W: 3, H: 2,
			Config: `{"metric_type":"overdue_requirement_count","data_scope":"requirement"}`},
		{CardType: define.DashboardCardTypeMetric, Title: `近30天新增缺陷`, ContentMode: mode, X: 6, Y: 3, W: 3, H: 2,
			Config: `{"metric_type":"created_defect_count","data_scope":"defect","recent_days":30}`},
		{CardType: define.DashboardCardTypeMetric, Title: `未解决缺陷数`, ContentMode: mode, X: 9, Y: 3, W: 3, H: 2,
			Config: `{"metric_type":"unresolved_defect_count","data_scope":"defect"}`},
		{CardType: define.DashboardCardTypeMetric, Title: `已完成需求`, ContentMode: mode, X: 0, Y: 5, W: 3, H: 2,
			Config: `{"metric_type":"completed_requirement_count","data_scope":"requirement"}`},
		{CardType: define.DashboardCardTypeMetric, Title: `已完成缺陷`, ContentMode: mode, X: 3, Y: 5, W: 3, H: 2,
			Config: `{"metric_type":"completed_defect_count","data_scope":"defect"}`},
		{CardType: define.DashboardCardTypeChart, Title: `需求状态分布`, ContentMode: mode, X: 0, Y: 7, W: 6, H: 4,
			Config: `{"chart_type":"status_bar","data_scope":"requirement","sort_order":"default","palette":"status","requirement_color":"#1677ff","defect_color":"#52c41a"}`},
		{CardType: define.DashboardCardTypeChart, Title: `缺陷状态分布`, ContentMode: mode, X: 6, Y: 7, W: 6, H: 4,
			Config: `{"chart_type":"status_pie","data_scope":"defect","sort_order":"default","palette":"status","requirement_color":"#1677ff","defect_color":"#52c41a"}`},
		{CardType: define.DashboardCardTypeChart, Title: `近2周需求完成数量趋势`, ContentMode: mode, X: 0, Y: 11, W: 6, H: 4,
			Config: `{"chart_type":"completion_line","data_scope":"requirement","days_from":14,"days_to":0,"sort_order":"default","palette":"status","requirement_color":"#1677ff","defect_color":"#52c41a"}`},
		{CardType: define.DashboardCardTypeChart, Title: `近2周缺陷完成数量趋势`, ContentMode: mode, X: 6, Y: 11, W: 6, H: 4,
			Config: `{"chart_type":"completion_line","data_scope":"defect","days_from":14,"days_to":0,"sort_order":"default","palette":"status","requirement_color":"#1677ff","defect_color":"#52c41a"}`},
	}
}

// dashboardDefaultCardsForWorkspaceTx 使用当前事务识别新建中的空间，避免读取不到尚未提交的空间类型。
func dashboardDefaultCardsForWorkspaceTx(tx *sql.Tx, adminId, workspaceId int) ([]DashboardCardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT workspace_type FROM `+define.TableWorkspaces+`
		WHERE admin_id = $1 AND id = $2`, tx, adminId, workspaceId)
	if err != nil {
		logs.Error("load dashboard workspace type failed, err:%s", err.Error())
		return nil, err
	}
	if len(rows) != 1 {
		return nil, ErrDashboardNotFound
	}
	return dashboardStarterCards(rows[0][`workspace_type`] == define.WorkspaceTypeIssue), nil
}

func dashboardStarterCards(issueWorkspace bool) []DashboardCardModel {
	if !issueWorkspace {
		return dashboardDefaultCards()
	}
	mode := define.DashboardContentModeRichText
	return []DashboardCardModel{
		{CardType: define.DashboardCardTypeMetric, Title: `近30天新增工单`, ContentMode: mode, X: 0, Y: 0, W: 4, H: 2,
			Config: `{"metric_type":"created_requirement_count","data_scope":"requirement","recent_days":30}`},
		{CardType: define.DashboardCardTypeMetric, Title: `已延期工单数`, ContentMode: mode, X: 4, Y: 0, W: 4, H: 2,
			Config: `{"metric_type":"overdue_requirement_count","data_scope":"requirement"}`},
		{CardType: define.DashboardCardTypeMetric, Title: `已完成工单`, ContentMode: mode, X: 8, Y: 0, W: 4, H: 2,
			Config: `{"metric_type":"completed_requirement_count","data_scope":"requirement"}`},
		{CardType: define.DashboardCardTypeChart, Title: `工单状态分布`, ContentMode: mode, X: 0, Y: 2, W: 6, H: 4,
			Config: `{"chart_type":"status_bar","data_scope":"requirement","sort_order":"default","palette":"status","requirement_color":"#1677ff","defect_color":"#52c41a"}`},
		{CardType: define.DashboardCardTypeChart, Title: `近2周工单完成数量趋势`, ContentMode: mode, X: 6, Y: 2, W: 6, H: 4,
			Config: `{"chart_type":"completion_line","data_scope":"requirement","days_from":14,"days_to":0,"sort_order":"default","palette":"status","requirement_color":"#1677ff","defect_color":"#52c41a"}`},
	}
}

// dashboardDataScope 将历史卡片的统计范围与工作空间支持的类型取交集；空范围不参与统计。
func dashboardDataScope(scope string, issueWorkspace bool) string {
	if issueWorkspace {
		switch scope {
		case define.DashboardMetricDataScopeAll:
			return define.DashboardMetricDataScopeRequirement
		case define.DashboardMetricDataScopeDefect:
			return ""
		}
	}
	return scope
}

// ListDashboards returns dashboards the current actor may see, ordered by shared project order.
func ListDashboards(model DashboardListModel) ([]DashboardModel, error) {
	if err := requireDashboardFeature(model.AdminId, model.WorkspaceId); err != nil {
		return nil, err
	}
	dashboards, err := loadWorkspaceDashboards(model.AdminId, model.WorkspaceId)
	if err != nil {
		return nil, err
	}
	if len(dashboards) == 0 {
		return []DashboardModel{}, nil
	}
	if err = loadDashboardRelations(model.AdminId, model.WorkspaceId, dashboards); err != nil {
		return nil, err
	}
	isAdmin, err := dashboardActorIsProjectAdmin(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin)
	if err != nil {
		return nil, err
	}
	roleId, err := dashboardActorRoleId(model.AdminId, model.WorkspaceId, model.UserId)
	if err != nil {
		return nil, err
	}
	hasViewPermission := HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), define.PermissionDashboardView)
	items := make([]DashboardModel, 0, len(dashboards))
	for _, dashboard := range dashboards {
		if !dashboardCanView(dashboard, model.UserId, isAdmin, hasViewPermission, roleId) {
			continue
		}
		setDashboardCapabilities(&dashboard, model.UserId, isAdmin)
		dashboard.Cards = []DashboardCardModel{}
		items = append(items, dashboard)
	}
	return items, nil
}

// GetDashboard loads one visible dashboard and all of its active cards.
func GetDashboard(model GetDashboardModel) (DashboardModel, error) {
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	dashboard.Cards, err = loadDashboardCards(model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	return dashboard, nil
}

// CanEditDashboard reports whether the actor can edit the visible dashboard.
func CanEditDashboard(adminId, workspaceId, userId, dashboardId int) (bool, error) {
	dashboard, err := getDashboardForActor(adminId, workspaceId, userId, false, dashboardId)
	if err != nil {
		if errors.Is(err, ErrDashboardNotFound) {
			return false, nil
		}
		return false, err
	}
	return dashboard.CanEdit, nil
}

// CreateDashboard creates a public shared dashboard for an actor holding dashboard:create.
func CreateDashboard(model CreateDashboardModel) (DashboardModel, error) {
	if err := requireDashboardFeature(model.AdminId, model.WorkspaceId); err != nil {
		return DashboardModel{}, err
	}
	if !HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), define.PermissionDashboardCreate) ||
		!HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), define.PermissionDashboardView) {
		return DashboardModel{}, ErrDashboardForbidden
	}
	name, err := validateDashboardName(model.Name)
	if err != nil {
		return DashboardModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create dashboard transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardModel{}, err
	}
	sortOrder, err := nextDashboardSortOrderTx(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return DashboardModel{}, err
	}
	now := int64(tool.Time2Int())
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkspaceDashboards+`
		(admin_id, workspace_id, name, visibility_type, sort_order, creator_id, revision, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7) RETURNING id`, tx,
		model.AdminId, model.WorkspaceId, name, define.DashboardVisibilityPublic, sortOrder, model.UserId, now)
	if err != nil {
		logs.Error("create dashboard failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	if len(rows) != 1 {
		return DashboardModel{}, ErrDashboardInvalid
	}
	dashboardId := cast.ToInt(rows[0][`id`])
	// Manually created dashboards start with the same starter cards as the seeded default,
	// so users see a working overview instead of an empty grid. Cards stay fully editable.
	var cards []DashboardCardModel
	switch model.Template {
	case "":
		cards, err = dashboardDefaultCardsForWorkspaceTx(tx, model.AdminId, model.WorkspaceId)
	case "workload", "category", "sla_stats", "sla_risk":
		issueWorkspace, workspaceErr := IsIssueWorkspace(model.AdminId, model.WorkspaceId)
		if workspaceErr != nil {
			return DashboardModel{}, workspaceErr
		}
		if !issueWorkspace {
			return DashboardModel{}, ErrDashboardInvalid
		}
		switch model.Template {
		case "workload":
			cards, err = dashboardWorkloadTemplateCards()
		case "category":
			cards = []DashboardCardModel{{
				CardType: define.DashboardCardTypeChart, ContentMode: define.DashboardContentModeRichText,
				W: define.DashboardGridColumns, H: 5,
				Config: `{"chart_type":"category_stats","data_scope":"requirement","sort_order":"default","palette":"status","requirement_color":"#1664ff","defect_color":"#f5222d","days_from":29,"days_to":0,"assignee_scope":"all"}`,
			}}
		case "sla_stats":
			var stages []SLADefinitionModel
			stages, err = listSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, true)
			if err == nil {
				cards, err = dashboardSLAStatsTemplateCards(stages)
			}
		case "sla_risk":
			cards = []DashboardCardModel{{
				CardType: define.DashboardCardTypeChart, ContentMode: define.DashboardContentModeRichText,
				W: define.DashboardGridColumns, H: 7,
				Config: `{"chart_type":"sla_risk","data_scope":"requirement","sla_stage_ids":[]}`,
			}}
		}
	default:
		return DashboardModel{}, ErrDashboardInvalid
	}
	if err != nil {
		return DashboardModel{}, err
	}
	for _, card := range cards {
		if _, err = insertDashboardCardTx(tx, model.AdminId, model.WorkspaceId, dashboardId, model.UserId, card, now); err != nil {
			logs.Error("seed dashboard starter cards failed, err:%s", err.Error())
			return DashboardModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create dashboard transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	return GetDashboard(GetDashboardModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
		ActorIsAdmin: model.ActorIsAdmin, DashboardId: dashboardId,
	})
}

// UpdateDashboard changes a visible dashboard's name under its optimistic revision lock.
func UpdateDashboard(model UpdateDashboardModel) (DashboardModel, error) {
	if model.Name == nil || validate.DashboardRevision(model.Revision) != nil {
		return DashboardModel{}, ErrDashboardInvalid
	}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	if !dashboard.CanEdit {
		return DashboardModel{}, ErrDashboardForbidden
	}
	name, err := validateDashboardName(*model.Name)
	if err != nil {
		return DashboardModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin update dashboard transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardModel{}, err
	}
	current, err := loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	if current.Revision != model.Revision {
		return DashboardModel{}, dashboardRevisionConflict(current.Revision)
	}
	now := int64(tool.Time2Int())
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceDashboards+`
		SET name = $1, revision = revision + 1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND id = $5 AND deleted_at = 0`, tx,
		name, now, model.AdminId, model.WorkspaceId, model.DashboardId); err != nil {
		logs.Error("update dashboard failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit update dashboard transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	return GetDashboard(GetDashboardModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
		ActorIsAdmin: model.ActorIsAdmin, DashboardId: model.DashboardId,
	})
}

// DeleteDashboard soft-deletes one managed dashboard while preserving the workspace's last one.
func DeleteDashboard(model DeleteDashboardModel) error {
	if validate.DashboardRevision(model.Revision) != nil {
		return ErrDashboardInvalid
	}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return err
	}
	if !dashboard.CanManage {
		return ErrDashboardForbidden
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin delete dashboard transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id, revision FROM `+define.TableWorkspaceDashboards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0 ORDER BY id ASC FOR UPDATE`, tx,
		model.AdminId, model.WorkspaceId)
	if err != nil {
		logs.Error("lock workspace dashboards for delete failed, err:%s", err.Error())
		return err
	}
	currentRevision := int64(0)
	found := false
	for _, row := range rows {
		if cast.ToInt(row[`id`]) != model.DashboardId {
			continue
		}
		found = true
		currentRevision = cast.ToInt64(row[`revision`])
		break
	}
	if !found {
		return ErrDashboardNotFound
	}
	if currentRevision != model.Revision {
		return dashboardRevisionConflict(currentRevision)
	}
	now := int64(tool.Time2Int())
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceDashboards+`
		SET deleted_at = $1, updated_at = $1, revision = revision + 1
		WHERE admin_id = $2 AND workspace_id = $3 AND id = $4 AND deleted_at = 0`, tx,
		now, model.AdminId, model.WorkspaceId, model.DashboardId); err != nil {
		logs.Error("delete dashboard failed, err:%s", err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit delete dashboard transaction failed, err:%s", err.Error())
	}
	return err
}

// GetDashboardPermission returns managed visibility and editor configuration for one dashboard.
func GetDashboardPermission(model DashboardPermissionModel) (DashboardPermissionModel, error) {
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardPermissionModel{}, err
	}
	if !dashboard.CanManage {
		return DashboardPermissionModel{}, ErrDashboardForbidden
	}
	return DashboardPermissionModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId, ActorIsAdmin: model.ActorIsAdmin,
		DashboardId: dashboard.Id, VisibilityType: dashboard.VisibilityType, RoleIds: dashboard.RoleIds,
		MemberIds: dashboard.MemberIds, EditorIds: dashboard.EditorIds, Revision: dashboard.Revision,
	}, nil
}

// ReplaceDashboardPermission atomically replaces a managed dashboard's visibility and extra editors.
func ReplaceDashboardPermission(model DashboardPermissionModel) (DashboardPermissionModel, error) {
	if validate.DashboardRevision(model.Revision) != nil ||
		validate.DashboardVisibility(model.VisibilityType, model.RoleIds, model.MemberIds, model.EditorIds) != nil {
		return DashboardPermissionModel{}, ErrDashboardInvalid
	}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardPermissionModel{}, err
	}
	if !dashboard.CanManage {
		return DashboardPermissionModel{}, ErrDashboardForbidden
	}
	roleIds, memberIds, editorIds, err := validateDashboardRelationTargets(
		model.AdminId, model.WorkspaceId, dashboard.CreatorId, model.RoleIds, model.MemberIds, model.EditorIds,
	)
	if err != nil {
		return DashboardPermissionModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin replace dashboard permission transaction failed, err:%s", err.Error())
		return DashboardPermissionModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardPermissionModel{}, err
	}
	current, err := loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardPermissionModel{}, err
	}
	if current.Revision != model.Revision {
		return DashboardPermissionModel{}, dashboardRevisionConflict(current.Revision)
	}
	now := int64(tool.Time2Int())
	if err = replaceDashboardRelationsTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, roleIds, memberIds, editorIds, now); err != nil {
		return DashboardPermissionModel{}, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceDashboards+`
		SET visibility_type = $1, revision = revision + 1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND id = $5 AND deleted_at = 0`, tx,
		model.VisibilityType, now, model.AdminId, model.WorkspaceId, model.DashboardId); err != nil {
		logs.Error("update dashboard permission failed, err:%s", err.Error())
		return DashboardPermissionModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit replace dashboard permission transaction failed, err:%s", err.Error())
		return DashboardPermissionModel{}, err
	}
	return GetDashboardPermission(DashboardPermissionModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
		ActorIsAdmin: model.ActorIsAdmin, DashboardId: model.DashboardId,
	})
}

// ReorderDashboards changes the shared dashboard sequence without letting a non-administrator
// move dashboards that they cannot edit.
func ReorderDashboards(model ReorderDashboardsModel) error {
	if err := requireDashboardFeature(model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	if !HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), define.PermissionDashboardView) {
		return ErrDashboardForbidden
	}
	dashboards, err := loadWorkspaceDashboards(model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	if len(dashboards) == 0 {
		return ErrDashboardInvalid
	}
	if err = loadDashboardRelations(model.AdminId, model.WorkspaceId, dashboards); err != nil {
		return err
	}
	isAdmin, err := dashboardActorIsProjectAdmin(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin)
	if err != nil {
		return err
	}
	roleId, err := dashboardActorRoleId(model.AdminId, model.WorkspaceId, model.UserId)
	if err != nil {
		return err
	}
	currentByID := make(map[int]DashboardModel, len(dashboards))
	currentIDs := make([]int, 0, len(dashboards))
	visibleIDs := make([]int, 0, len(dashboards))
	for _, item := range dashboards {
		if dashboardCanView(item, model.UserId, isAdmin, true, roleId) {
			setDashboardCapabilities(&item, model.UserId, isAdmin)
			visibleIDs = append(visibleIDs, item.Id)
		}
		currentByID[item.Id] = item
		currentIDs = append(currentIDs, item.Id)
	}
	nextIDs, err := dashboardOrderIDs(model.Items)
	if err != nil {
		return err
	}
	if !sameDashboardIDSet(visibleIDs, nextIDs) {
		return ErrDashboardInvalid
	}
	if !isAdmin {
		if !sameRelativeNonEditableOrder(visibleIDs, nextIDs, currentByID) {
			return ErrDashboardForbidden
		}
	}
	visibleSet := make(map[int]struct{}, len(visibleIDs))
	for _, id := range visibleIDs {
		visibleSet[id] = struct{}{}
	}
	finalIDs := make([]int, 0, len(currentIDs))
	nextVisibleIndex := 0
	for _, id := range currentIDs {
		if _, visible := visibleSet[id]; !visible {
			finalIDs = append(finalIDs, id)
			continue
		}
		finalIDs = append(finalIDs, nextIDs[nextVisibleIndex])
		nextVisibleIndex++
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin reorder dashboards transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	lockedRows, err := msql.RawValues(define.Postgres, `SELECT id, sort_order FROM `+define.TableWorkspaceDashboards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0 ORDER BY sort_order ASC, id ASC FOR UPDATE`, tx,
		model.AdminId, model.WorkspaceId)
	if err != nil {
		logs.Error("lock dashboards for reorder failed, err:%s", err.Error())
		return err
	}
	if len(lockedRows) != len(finalIDs) {
		return ErrDashboardRevisionConflict
	}
	for index, row := range lockedRows {
		if cast.ToInt(row[`id`]) != currentIDs[index] {
			return ErrDashboardRevisionConflict
		}
	}
	now := int64(tool.Time2Int())
	for index, id := range finalIDs {
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceDashboards+`
			SET sort_order = $1, revision = CASE WHEN sort_order = $1 THEN revision ELSE revision + 1 END,
				updated_at = CASE WHEN sort_order = $1 THEN updated_at ELSE $2 END
			WHERE admin_id = $3 AND workspace_id = $4 AND id = $5 AND deleted_at = 0`, tx,
			index+1, now, model.AdminId, model.WorkspaceId, id); err != nil {
			logs.Error("update dashboard order failed, err:%s", err.Error())
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit reorder dashboards transaction failed, err:%s", err.Error())
	}
	return err
}

// SaveDashboardAs creates an independent dashboard copy. Visibility settings and cards are copied,
// while extra editors and personal user state intentionally remain local to their original dashboard.
func SaveDashboardAs(model SaveDashboardAsModel) (DashboardModel, error) {
	if validate.DashboardRevision(model.Revision) != nil {
		return DashboardModel{}, ErrDashboardInvalid
	}
	if err := requireDashboardFeature(model.AdminId, model.WorkspaceId); err != nil {
		return DashboardModel{}, err
	}
	if !HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), define.PermissionDashboardCreate) ||
		!HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), define.PermissionDashboardView) {
		return DashboardModel{}, ErrDashboardForbidden
	}
	name, err := validateDashboardName(model.Name)
	if err != nil {
		return DashboardModel{}, err
	}
	if _, err = getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId); err != nil {
		return DashboardModel{}, err
	}
	createdPaths := make([]string, 0)
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, path := range createdPaths {
			if deleteErr := lib_storage.M.Delete(path); deleteErr != nil {
				logs.Error("cleanup copied dashboard attachment failed, err:%s", deleteErr.Error())
			}
		}
	}()
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin save dashboard as transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardModel{}, err
	}
	source, err := loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	if source.Revision != model.Revision {
		return DashboardModel{}, dashboardRevisionConflict(source.Revision)
	}
	sourceRelations, err := dashboardRelationsForIdTx(tx, model.AdminId, model.WorkspaceId, source.Id)
	if err != nil {
		return DashboardModel{}, err
	}
	sourceCards, err := loadDashboardCardsForUpdateTx(tx, model.AdminId, model.WorkspaceId, source.Id)
	if err != nil {
		return DashboardModel{}, err
	}
	if len(sourceCards) > define.DashboardMaxCardsPerDashboard {
		return DashboardModel{}, ErrDashboardInvalidLayout
	}
	sortOrder, err := nextDashboardSortOrderTx(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return DashboardModel{}, err
	}
	now := int64(tool.Time2Int())
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkspaceDashboards+`
		(admin_id, workspace_id, name, visibility_type, sort_order, creator_id, revision, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7) RETURNING id`, tx,
		model.AdminId, model.WorkspaceId, name, source.VisibilityType, sortOrder, model.UserId, now)
	if err != nil || len(rows) != 1 {
		if err == nil {
			err = ErrDashboardInvalid
		}
		if err != nil {
			logs.Error("insert dashboard copy failed, err:%s", err.Error())
		}
		return DashboardModel{}, err
	}
	targetID := cast.ToInt(rows[0][`id`])
	if err = replaceDashboardRelationsTx(tx, model.AdminId, model.WorkspaceId, targetID,
		sourceRelations.RoleIds, sourceRelations.MemberIds, nil, now); err != nil {
		return DashboardModel{}, err
	}
	for _, sourceCard := range sourceCards {
		targetCard, insertErr := insertDashboardCardTx(tx, model.AdminId, model.WorkspaceId, targetID, model.UserId, sourceCard, now)
		if insertErr != nil {
			return DashboardModel{}, insertErr
		}
		if sourceCard.CardType != define.DashboardCardTypeText {
			continue
		}
		content, copyErr := copyDashboardRichTextAttachmentsTx(tx, model.AdminId, model.WorkspaceId, model.WorkspaceId,
			define.SourceTypeDashboardCard, sourceCard.Id, define.SourceTypeDashboardCard, targetCard.Id,
			model.UserId, targetCard.Title, sourceCard.Content, &createdPaths)
		if copyErr != nil {
			return DashboardModel{}, copyErr
		}
		if content == sourceCard.Content {
			continue
		}
		if _, copyErr = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardCards+`
			SET content = $1, updated_at = $2 WHERE admin_id = $3 AND workspace_id = $4 AND id = $5 AND dashboard_id = $6`, tx,
			content, now, model.AdminId, model.WorkspaceId, targetCard.Id, targetID); copyErr != nil {
			logs.Error("write copied dashboard card content failed, err:%s", copyErr.Error())
			return DashboardModel{}, copyErr
		}
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit save dashboard as transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	committed = true
	return GetDashboard(GetDashboardModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
		ActorIsAdmin: model.ActorIsAdmin, DashboardId: targetID,
	})
}

// CreateDashboardCard adds one card to an editable dashboard. The batch layout endpoint is used
// by the editor UI; this focused endpoint remains available for integrations that create one card.
func CreateDashboardCard(model CreateDashboardCardModel) (DashboardCardModel, error) {
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardCardModel{}, err
	}
	if !dashboard.CanEdit {
		return DashboardCardModel{}, ErrDashboardForbidden
	}
	contentMode, err := dashboardContentMode(model.ContentMode)
	if err != nil {
		return DashboardCardModel{}, err
	}
	card := DashboardCardModel{
		DashboardId: model.DashboardId, CardType: model.CardType, Title: model.Title, Config: model.Config,
		Content: model.Content, ContentMode: contentMode, X: model.X, Y: model.Y, W: model.W, H: model.H,
	}
	if err = validateDashboardCardPayload(model.AdminId, model.WorkspaceId, card, model.FileIds); err != nil {
		return DashboardCardModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin create dashboard card transaction failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardCardModel{}, err
	}
	if _, err = loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId); err != nil {
		return DashboardCardModel{}, err
	}
	cards, err := loadDashboardCardsForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardCardModel{}, err
	}
	if card.CardType == define.DashboardCardTypeAnnouncement && dashboardCardsContainType(cards, define.DashboardCardTypeAnnouncement) {
		return DashboardCardModel{}, ErrDashboardAnnouncementExists
	}
	if card.W <= 0 || card.H <= 0 {
		card.W, card.H = 4, 3
		card.X, card.Y = firstOpenDashboardPosition(cards, card.W, card.H)
	}
	if err = validateDashboardCardLayout(cards, card, 0); err != nil {
		return DashboardCardModel{}, err
	}
	now := int64(tool.Time2Int())
	card, err = insertDashboardCardTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, model.UserId, card, now)
	if err != nil {
		return DashboardCardModel{}, err
	}
	if err = bindDashboardPendingFilesTx(tx, model.AdminId, model.WorkspaceId, model.UserId, card.Id,
		define.SourceTypeDashboardCard, card.Title, model.FileIds); err != nil {
		return DashboardCardModel{}, err
	}
	if err = bumpDashboardRevisionTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, now); err != nil {
		return DashboardCardModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit create dashboard card transaction failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	return card, nil
}

// UpdateDashboardCard updates one editable card under its card revision lock.
func UpdateDashboardCard(model UpdateDashboardCardModel) (DashboardCardModel, error) {
	if validate.DashboardRevision(model.Revision) != nil {
		return DashboardCardModel{}, ErrDashboardInvalid
	}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardCardModel{}, err
	}
	if !dashboard.CanEdit {
		return DashboardCardModel{}, ErrDashboardForbidden
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin update dashboard card transaction failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardCardModel{}, err
	}
	if _, err = loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId); err != nil {
		return DashboardCardModel{}, err
	}
	current, err := loadDashboardCardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, model.CardId)
	if err != nil {
		return DashboardCardModel{}, err
	}
	if current.Revision != model.Revision {
		return DashboardCardModel{}, dashboardRevisionConflict(current.Revision)
	}
	next := current
	if model.Title != nil {
		next.Title = *model.Title
	}
	if model.Config != nil {
		next.Config = *model.Config
	}
	if model.Content != nil {
		next.Content = *model.Content
	}
	if model.ContentMode != nil {
		next.ContentMode = *model.ContentMode
	}
	if model.X != nil {
		next.X = *model.X
	}
	if model.Y != nil {
		next.Y = *model.Y
	}
	if model.W != nil {
		next.W = *model.W
	}
	if model.H != nil {
		next.H = *model.H
	}
	if next.ContentMode, err = dashboardContentMode(next.ContentMode); err != nil {
		return DashboardCardModel{}, err
	}
	if err = validateDashboardCardPayload(model.AdminId, model.WorkspaceId, next, model.FileIds); err != nil {
		return DashboardCardModel{}, err
	}
	cards, err := loadDashboardCardsForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardCardModel{}, err
	}
	if err = validateDashboardCardLayout(cards, next, next.Id); err != nil {
		return DashboardCardModel{}, err
	}
	now := int64(tool.Time2Int())
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardCards+`
		SET title = $1, config = $2, content = $3, content_mode = $4, grid_x = $5, grid_y = $6,
			grid_width = $7, grid_height = $8, revision = revision + 1, updated_at = $9
		WHERE admin_id = $10 AND workspace_id = $11 AND dashboard_id = $12 AND id = $13 AND deleted_at = 0`, tx,
		next.Title, next.Config, next.Content, next.ContentMode, next.X, next.Y, next.W, next.H, now,
		model.AdminId, model.WorkspaceId, model.DashboardId, model.CardId); err != nil {
		logs.Error("update dashboard card failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	if err = bindDashboardPendingFilesTx(tx, model.AdminId, model.WorkspaceId, model.UserId, current.Id,
		define.SourceTypeDashboardCard, next.Title, model.FileIds); err != nil {
		return DashboardCardModel{}, err
	}
	if err = bumpDashboardRevisionTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, now); err != nil {
		return DashboardCardModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit update dashboard card transaction failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	next.Revision++
	next.UpdatedAt = now
	return next, nil
}

// DeleteDashboardCard soft-deletes one card from an editable dashboard under its revision lock.
func DeleteDashboardCard(model DeleteDashboardCardModel) error {
	if validate.DashboardRevision(model.Revision) != nil {
		return ErrDashboardInvalid
	}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return err
	}
	if !dashboard.CanEdit {
		return ErrDashboardForbidden
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin delete dashboard card transaction failed, err:%s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	if _, err = loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId); err != nil {
		return err
	}
	card, err := loadDashboardCardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, model.CardId)
	if err != nil {
		return err
	}
	if card.Revision != model.Revision {
		return dashboardRevisionConflict(card.Revision)
	}
	now := int64(tool.Time2Int())
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardCards+`
		SET deleted_at = $1, updated_at = $1, revision = revision + 1
		WHERE admin_id = $2 AND workspace_id = $3 AND dashboard_id = $4 AND id = $5 AND deleted_at = 0`, tx,
		now, model.AdminId, model.WorkspaceId, model.DashboardId, model.CardId); err != nil {
		logs.Error("delete dashboard card failed, err:%s", err.Error())
		return err
	}
	if err = bumpDashboardRevisionTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, now); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit delete dashboard card transaction failed, err:%s", err.Error())
	}
	return err
}

// CopyDashboardCard copies one metric card into an editable target dashboard.
func CopyDashboardCard(model CopyDashboardCardModel) (DashboardCardModel, error) {
	cards, err := CopyDashboardCards(CopyDashboardCardsModel{AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId, DashboardId: model.DashboardId, CardId: model.CardId, TargetDashboardIds: []int{model.TargetDashboardId}})
	if err != nil {
		return DashboardCardModel{}, err
	}
	return cards[0], nil
}

// CopyDashboardCardsModel identifies a saved card and its bounded workspace targets.
type CopyDashboardCardsModel struct {
	AdminId, WorkspaceId, UserId, DashboardId, CardId int
	TargetDashboardIds                                []int
}

// CopyDashboardCards saves all copies in one workspace transaction.
func CopyDashboardCards(model CopyDashboardCardsModel) ([]DashboardCardModel, error) {
	if len(model.TargetDashboardIds) > 100 || validate.DashboardOrder(model.TargetDashboardIds) != nil {
		return nil, ErrDashboardInvalid
	}
	source, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, false, model.DashboardId)
	if err != nil {
		return nil, err
	}
	if !source.CanEdit {
		return nil, ErrDashboardForbidden
	}
	for _, id := range model.TargetDashboardIds {
		if id == model.DashboardId {
			return nil, ErrDashboardInvalid
		}
		target, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, false, id)
		if err != nil {
			return nil, err
		}
		if !target.CanEdit {
			return nil, ErrDashboardForbidden
		}
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logs.Error("rollback chart copies: %s", rollbackErr.Error())
		}
	}()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return nil, err
	}
	sourceCard, err := loadDashboardCardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, model.CardId)
	if err != nil {
		return nil, err
	}
	if sourceCard.CardType != define.DashboardCardTypeMetric && sourceCard.CardType != define.DashboardCardTypeChart {
		return nil, ErrDashboardInvalid
	}
	result := make([]DashboardCardModel, 0, len(model.TargetDashboardIds))
	now := int64(tool.Time2Int())
	for _, targetId := range model.TargetDashboardIds {
		if _, err = loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, targetId); err != nil {
			return nil, err
		}
		targetCards, err := loadDashboardCardsForUpdateTx(tx, model.AdminId, model.WorkspaceId, targetId)
		if err != nil {
			return nil, err
		}
		candidate := sourceCard
		candidate.Id = 0
		candidate.DashboardId = targetId
		candidate.X, candidate.Y = firstOpenDashboardPosition(targetCards, candidate.W, candidate.H)
		if err = validateDashboardCardLayout(targetCards, candidate, 0); err != nil {
			return nil, err
		}
		card, err := insertDashboardCardTx(tx, model.AdminId, model.WorkspaceId, targetId, model.UserId, candidate, now)
		if err != nil {
			return nil, err
		}
		if err = bumpDashboardRevisionTx(tx, model.AdminId, model.WorkspaceId, targetId, now); err != nil {
			return nil, err
		}
		result = append(result, card)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// ReplaceDashboardLayout atomically saves the complete editable card state, removals, geometry,
// and an optional project-announcement edit. Any invalid card or attachment rolls back all writes.
func ReplaceDashboardLayout(model ReplaceDashboardLayoutModel) (DashboardModel, error) {
	if validate.DashboardRevision(model.Revision) != nil {
		return DashboardModel{}, ErrDashboardInvalid
	}
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	if !dashboard.CanEdit {
		return DashboardModel{}, ErrDashboardForbidden
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin replace dashboard layout transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardModel{}, err
	}
	currentDashboard, err := loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	if currentDashboard.Revision != model.Revision {
		return DashboardModel{}, dashboardRevisionConflict(currentDashboard.Revision)
	}
	currentCards, err := loadDashboardCardsForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	currentByID := make(map[int]DashboardCardModel, len(currentCards))
	for _, card := range currentCards {
		currentByID[card.Id] = card
	}
	deletedByID := make(map[int]DashboardDeletedCardModel, len(model.DeletedCards))
	for _, deleted := range model.DeletedCards {
		if deleted.CardId <= 0 || validate.DashboardRevision(deleted.Revision) != nil {
			return DashboardModel{}, ErrDashboardInvalid
		}
		if _, duplicate := deletedByID[deleted.CardId]; duplicate {
			return DashboardModel{}, ErrDashboardInvalid
		}
		current, exists := currentByID[deleted.CardId]
		if !exists {
			return DashboardModel{}, ErrDashboardNotFound
		}
		if current.Revision != deleted.Revision {
			return DashboardModel{}, dashboardRevisionConflict(current.Revision)
		}
		deletedByID[deleted.CardId] = deleted
	}
	seenExisting := make(map[int]struct{}, len(model.Cards))
	layoutItems := make([]validate.DashboardLayoutItem, 0, len(model.Cards))
	announcementCount := 0
	syntheticID := 1
	for _, candidate := range model.Cards {
		if candidate.Id > 0 {
			if _, duplicate := seenExisting[candidate.Id]; duplicate {
				return DashboardModel{}, ErrDashboardInvalid
			}
			current, exists := currentByID[candidate.Id]
			if !exists || candidate.CardType != current.CardType || validate.DashboardRevision(candidate.Revision) != nil {
				return DashboardModel{}, ErrDashboardInvalid
			}
			if current.Revision != candidate.Revision {
				return DashboardModel{}, dashboardRevisionConflict(current.Revision)
			}
			seenExisting[candidate.Id] = struct{}{}
			layoutItems = append(layoutItems, validate.DashboardLayoutItem{Id: candidate.Id, X: candidate.X, Y: candidate.Y, W: candidate.W, H: candidate.H})
		} else {
			if candidate.Id < 0 {
				return DashboardModel{}, ErrDashboardInvalid
			}
			for {
				if _, used := currentByID[syntheticID]; !used {
					break
				}
				syntheticID++
			}
			layoutItems = append(layoutItems, validate.DashboardLayoutItem{Id: syntheticID, X: candidate.X, Y: candidate.Y, W: candidate.W, H: candidate.H})
			syntheticID++
		}
		card := DashboardCardModel{
			CardType: candidate.CardType, Title: candidate.Title, Config: candidate.Config,
			Content: candidate.Content, ContentMode: candidate.ContentMode,
		}
		if card.ContentMode, err = dashboardContentMode(card.ContentMode); err != nil {
			return DashboardModel{}, err
		}
		if err = validateDashboardCardPayload(model.AdminId, model.WorkspaceId, card, candidate.FileIds); err != nil {
			return DashboardModel{}, err
		}
		if candidate.CardType == define.DashboardCardTypeAnnouncement {
			announcementCount++
		}
	}
	if announcementCount > 1 || validate.DashboardLayout(layoutItems) != nil {
		return DashboardModel{}, ErrDashboardInvalidLayout
	}
	if len(seenExisting)+len(deletedByID) != len(currentByID) {
		return DashboardModel{}, ErrDashboardInvalidLayout
	}
	for id := range seenExisting {
		if _, deleted := deletedByID[id]; deleted {
			return DashboardModel{}, ErrDashboardInvalid
		}
	}
	if model.Announcement != nil {
		if err = validateDashboardAnnouncementMutation(*model.Announcement); err != nil {
			return DashboardModel{}, err
		}
	}
	now := int64(tool.Time2Int())
	for _, deleted := range model.DeletedCards {
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardCards+`
			SET deleted_at = $1, updated_at = $1, revision = revision + 1
			WHERE admin_id = $2 AND workspace_id = $3 AND dashboard_id = $4 AND id = $5 AND revision = $6 AND deleted_at = 0`, tx,
			now, model.AdminId, model.WorkspaceId, model.DashboardId, deleted.CardId, deleted.Revision); err != nil {
			logs.Error("delete dashboard card in layout batch failed, err:%s", err.Error())
			return DashboardModel{}, err
		}
	}
	for _, candidate := range model.Cards {
		contentMode, contentErr := dashboardContentMode(candidate.ContentMode)
		if contentErr != nil {
			return DashboardModel{}, contentErr
		}
		if candidate.Id == 0 {
			card := DashboardCardModel{
				DashboardId: model.DashboardId, CardType: candidate.CardType, Title: candidate.Title,
				Config: candidate.Config, Content: candidate.Content, ContentMode: contentMode,
				X: candidate.X, Y: candidate.Y, W: candidate.W, H: candidate.H,
			}
			card, err = insertDashboardCardTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, model.UserId, card, now)
			if err != nil {
				return DashboardModel{}, err
			}
			if err = bindDashboardPendingFilesTx(tx, model.AdminId, model.WorkspaceId, model.UserId, card.Id,
				define.SourceTypeDashboardCard, card.Title, candidate.FileIds); err != nil {
				return DashboardModel{}, err
			}
			continue
		}
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardCards+`
			SET title = $1, config = $2, content = $3, content_mode = $4, grid_x = $5, grid_y = $6,
				grid_width = $7, grid_height = $8, revision = revision + 1, updated_at = $9
			WHERE admin_id = $10 AND workspace_id = $11 AND dashboard_id = $12 AND id = $13
				AND revision = $14 AND deleted_at = 0`, tx,
			candidate.Title, candidate.Config, candidate.Content, contentMode, candidate.X, candidate.Y, candidate.W, candidate.H, now,
			model.AdminId, model.WorkspaceId, model.DashboardId, candidate.Id, candidate.Revision); err != nil {
			logs.Error("update dashboard card in layout batch failed, err:%s", err.Error())
			return DashboardModel{}, err
		}
		if err = bindDashboardPendingFilesTx(tx, model.AdminId, model.WorkspaceId, model.UserId, candidate.Id,
			define.SourceTypeDashboardCard, candidate.Title, candidate.FileIds); err != nil {
			return DashboardModel{}, err
		}
	}
	if model.Announcement != nil {
		if err = updateDashboardAnnouncementTx(tx, model.AdminId, model.WorkspaceId, model.UserId, currentDashboard.Name, *model.Announcement, now); err != nil {
			return DashboardModel{}, err
		}
	}
	if err = bumpDashboardRevisionTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, now); err != nil {
		return DashboardModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit replace dashboard layout transaction failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	return GetDashboard(GetDashboardModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
		ActorIsAdmin: model.ActorIsAdmin, DashboardId: model.DashboardId,
	})
}

// GetDashboardAnnouncement reads the project-wide announcement through a dashboard the actor may view.
func GetDashboardAnnouncement(model GetDashboardAnnouncementModel) (DashboardAnnouncementModel, error) {
	if _, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId); err != nil {
		return DashboardAnnouncementModel{}, err
	}
	return loadDashboardAnnouncement(model.AdminId, model.WorkspaceId)
}

// UpdateDashboardAnnouncement updates the shared project announcement through one editable dashboard.
func UpdateDashboardAnnouncement(model UpdateDashboardAnnouncementModel) (DashboardAnnouncementModel, error) {
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardAnnouncementModel{}, err
	}
	if !dashboard.CanEdit {
		return DashboardAnnouncementModel{}, ErrDashboardForbidden
	}
	mutation := DashboardAnnouncementMutationModel{
		Content: model.Content, ContentMode: model.ContentMode, FileIds: model.FileIds, Revision: model.Revision,
	}
	if err = validateDashboardAnnouncementMutation(mutation); err != nil {
		return DashboardAnnouncementModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin update dashboard announcement transaction failed, err:%s", err.Error())
		return DashboardAnnouncementModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockDashboardWorkspace(tx, model.AdminId, model.WorkspaceId); err != nil {
		return DashboardAnnouncementModel{}, err
	}
	currentDashboard, err := loadDashboardForUpdateTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId)
	if err != nil {
		return DashboardAnnouncementModel{}, err
	}
	now := int64(tool.Time2Int())
	if err = updateDashboardAnnouncementTx(tx, model.AdminId, model.WorkspaceId, model.UserId, currentDashboard.Name, mutation, now); err != nil {
		return DashboardAnnouncementModel{}, err
	}
	if err = bumpDashboardRevisionTx(tx, model.AdminId, model.WorkspaceId, model.DashboardId, now); err != nil {
		return DashboardAnnouncementModel{}, err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit update dashboard announcement transaction failed, err:%s", err.Error())
		return DashboardAnnouncementModel{}, err
	}
	return loadDashboardAnnouncement(model.AdminId, model.WorkspaceId)
}

// GetDashboardUserState returns the current actor's private filter and collapse state for a visible dashboard.
func GetDashboardUserState(model DashboardUserStateModel) (DashboardUserStateModel, error) {
	if _, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId); err != nil {
		return DashboardUserStateModel{}, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT filters, filters_collapsed, created_at, updated_at
		FROM `+define.TableDashboardUserStates+`
		WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3 AND user_id = $4 LIMIT 1`, nil,
		model.AdminId, model.WorkspaceId, model.DashboardId, model.UserId)
	if err != nil {
		logs.Error("load dashboard user state failed, err:%s", err.Error())
		return DashboardUserStateModel{}, err
	}
	result := DashboardUserStateModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
		ActorIsAdmin: model.ActorIsAdmin, DashboardId: model.DashboardId, Filters: ``, FiltersCollapsed: true,
	}
	if len(rows) == 0 {
		return result, nil
	}
	result.Filters = rows[0][`filters`]
	result.FiltersCollapsed = cast.ToInt(rows[0][`filters_collapsed`]) == 1
	result.CreatedAt = cast.ToInt64(rows[0][`created_at`])
	result.UpdatedAt = cast.ToInt64(rows[0][`updated_at`])
	return result, nil
}

// UpdateDashboardUserState upserts the current actor's private dashboard filters.
func UpdateDashboardUserState(model DashboardUserStateModel) (DashboardUserStateModel, error) {
	if _, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId); err != nil {
		return DashboardUserStateModel{}, err
	}
	if err := ValidateDashboardFilters(model.AdminId, model.WorkspaceId, model.Filters, define.DashboardMetricDataScopeAll); err != nil {
		return DashboardUserStateModel{}, err
	}
	now := int64(tool.Time2Int())
	collapsed := 0
	if model.FiltersCollapsed {
		collapsed = 1
	}
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableDashboardUserStates+`
		(admin_id, workspace_id, dashboard_id, user_id, filters, filters_collapsed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (admin_id, workspace_id, dashboard_id, user_id)
		DO UPDATE SET filters = EXCLUDED.filters, filters_collapsed = EXCLUDED.filters_collapsed, updated_at = EXCLUDED.updated_at
		RETURNING created_at, updated_at`, nil,
		model.AdminId, model.WorkspaceId, model.DashboardId, model.UserId, strings.TrimSpace(model.Filters), collapsed, now)
	if err != nil {
		logs.Error("upsert dashboard user state failed, err:%s", err.Error())
		return DashboardUserStateModel{}, err
	}
	if len(rows) != 1 {
		return DashboardUserStateModel{}, ErrDashboardInvalid
	}
	model.Filters = strings.TrimSpace(model.Filters)
	model.CreatedAt = cast.ToInt64(rows[0][`created_at`])
	model.UpdatedAt = cast.ToInt64(rows[0][`updated_at`])
	return model, nil
}

// QueryDashboardMetric calculates one saved metric card through the actor's dashboard, work-item,
// and data-view permissions. Client filters may narrow the result but cannot expand data access.
func QueryDashboardMetric(model DashboardMetricQueryModel) (DashboardMetricResultModel, error) {
	dashboard, err := getDashboardForActor(model.AdminId, model.WorkspaceId, model.UserId, model.ActorIsAdmin, model.DashboardId)
	if err != nil {
		return DashboardMetricResultModel{}, err
	}
	var rawConfig string
	if model.PreviewConfig != nil {
		// Unsaved configuration is available only to an editor; normal viewers query saved cards.
		if !dashboard.CanEdit {
			return DashboardMetricResultModel{}, ErrDashboardForbidden
		}
		rawConfig = *model.PreviewConfig
	} else {
		card, err := loadDashboardCard(model.AdminId, model.WorkspaceId, model.DashboardId, model.CardId)
		if err != nil {
			return DashboardMetricResultModel{}, err
		}
		if card.CardType != define.DashboardCardTypeMetric {
			return DashboardMetricResultModel{}, ErrDashboardInvalidMetric
		}
		rawConfig = card.Config
	}
	config, cardFilters, err := parseDashboardMetricConfig(model.AdminId, model.WorkspaceId, rawConfig)
	if err != nil {
		return DashboardMetricResultModel{}, err
	}

	issueWorkspace, err := IsIssueWorkspace(model.AdminId, model.WorkspaceId)
	if err != nil {
		return DashboardMetricResultModel{}, err
	}
	config.DataScope = dashboardDataScope(config.DataScope, issueWorkspace)
	if config.DataScope == "" {
		return DashboardMetricResultModel{MetricType: config.MetricType, DataScope: config.DataScope}, nil
	}

	where, args, err := dashboardWorkItemConditions(model, config.DataScope, cardFilters, issueWorkspace)
	if err != nil {
		return DashboardMetricResultModel{}, err
	}
	if strings.HasPrefix(config.MetricType, "sla_") {
		return queryDashboardSLAMetric(model, config, where, args, false, nil)
	}
	// Rolling window on created_at is resolved at query time so "recent N days" stays current.
	if config.RecentDays > 0 {
		args = append(args, int64(tool.Time2Int())-int64(config.RecentDays)*86400)
		where = append(where, fmt.Sprintf(`r.created_at >= $%d`, len(args)))
	}
	if strings.HasPrefix(config.MetricType, "issue_") && !issueWorkspace {
		return DashboardMetricResultModel{}, ErrDashboardInvalidMetric
	}
	switch config.MetricType {
	case define.DashboardMetricIssueSLADueSoon:
		value, err := queryDashboardSLADueSoon(model.AdminId, model.WorkspaceId, where, args)
		return DashboardMetricResultModel{MetricType: config.MetricType, DataScope: config.DataScope, Value: float64(value)}, err
	case define.DashboardMetricClosedCount, define.DashboardMetricInProgressCount, define.DashboardMetricIssuePendingCount,
		define.DashboardMetricIssueCreatedToday, define.DashboardMetricIssueCompletedToday:
		where, args, err = dashboardStatusMetricConditions(model, config.MetricType, where, args)
		if err != nil {
			return DashboardMetricResultModel{}, err
		}
	}
	expression, err := dashboardMetricExpression(model.AdminId, model.WorkspaceId, config.MetricType, &args, &where)
	if err != nil {
		return DashboardMetricResultModel{}, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT `+expression+` AS value FROM `+define.TableRequirements+` r
		WHERE `+strings.Join(where, ` AND `), nil, args...)
	if err != nil {
		logs.Error("query dashboard metric failed, err:%s", err.Error())
		return DashboardMetricResultModel{}, err
	}
	value := float64(0)
	if len(rows) > 0 {
		parsed, parseErr := strconv.ParseFloat(rows[0][`value`], 64)
		if parseErr != nil {
			return DashboardMetricResultModel{}, fmt.Errorf("parse dashboard metric value: %w", parseErr)
		}
		value = parsed
	}
	return DashboardMetricResultModel{MetricType: config.MetricType, DataScope: config.DataScope, Value: value}, nil
}

func requireDashboardFeature(adminId, workspaceId int) error {
	if adminId <= 0 || workspaceId <= 0 {
		return ErrDashboardInvalid
	}
	enabled, err := IsWorkspaceApplicationEnabled(adminId, workspaceId, define.AppKeyDashboard)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrDashboardFeatureDisabled
	}
	return nil
}

func lockDashboardWorkspace(tx *sql.Tx, adminId, workspaceId int) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkspaces+`
		WHERE admin_id = $1 AND id = $2 FOR UPDATE`, tx, adminId, workspaceId)
	if err != nil {
		logs.Error("lock dashboard workspace failed, err:%s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrWorkspaceNotFound
	}
	return nil
}

func nextDashboardSortOrderTx(tx *sql.Tx, adminId, workspaceId int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(MAX(sort_order), 0) AS sort_order
		FROM `+define.TableWorkspaceDashboards+` WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0`, tx,
		adminId, workspaceId)
	if err != nil {
		logs.Error("load dashboard max sort order failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 1, nil
	}
	return cast.ToInt(rows[0][`sort_order`]) + 1, nil
}

func ensureDashboardAnnouncementTx(tx *sql.Tx, adminId, workspaceId, userId int, now int64) error {
	_, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableDashboardAnnouncements+`
		(admin_id, workspace_id, content, content_mode, revision, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, $5, $6, $6)
		ON CONFLICT (admin_id, workspace_id) DO NOTHING`, tx,
		adminId, workspaceId, dashboardDefaultAnnouncementContent, define.DashboardContentModeRichText, userId, now)
	if err != nil {
		logs.Error("ensure dashboard announcement failed, err:%s", err.Error())
	}
	return err
}

func validateDashboardName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == `` || utf8.RuneCountInString(value) > define.DashboardNameMaxLength {
		return ``, ErrDashboardInvalid
	}
	return value, nil
}

func dashboardActorIsProjectAdmin(adminId, workspaceId, userId int, hinted bool) (bool, error) {
	if hinted || IsTenantOwner(adminId, userId) {
		return true, nil
	}
	if userId <= 0 {
		return false, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT 1 FROM `+define.TableWorkspaceMembers+` wm
		JOIN `+define.TableRoles+` r ON r.id = wm.role_id AND r.admin_id = wm.admin_id
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2 AND wm.user_id = $3
		AND r.workspace_id = $2 AND r.scope = $4 AND r.is_system = 1
		AND r.name IN ($5, $6) LIMIT 1`, nil,
		adminId, workspaceId, userId, permission.ScopeProject,
		dashboardProjectAdminRoleName, lib_i18n.WrapKey(dashboardProjectAdminRoleName))
	if err != nil {
		logs.Error("check dashboard project admin failed, err:%s", err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

func dashboardActorRoleId(adminId, workspaceId, userId int) (int, error) {
	if userId <= 0 {
		return 0, nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT role_id FROM `+define.TableWorkspaceMembers+`
		WHERE admin_id = $1 AND workspace_id = $2 AND user_id = $3 LIMIT 1`, nil, adminId, workspaceId, userId)
	if err != nil {
		logs.Error("load dashboard actor role failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt(rows[0][`role_id`]), nil
}

func loadWorkspaceDashboards(adminId, workspaceId int) ([]DashboardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name, visibility_type, sort_order, creator_id,
		revision, created_at, updated_at FROM `+define.TableWorkspaceDashboards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0 ORDER BY sort_order ASC, id ASC`, nil,
		adminId, workspaceId)
	if err != nil {
		logs.Error("list dashboards failed, err:%s", err.Error())
		return nil, err
	}
	items := make([]DashboardModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := dashboardModelFromRow(row)
		if convErr != nil {
			return nil, convErr
		}
		items = append(items, item)
	}
	return items, nil
}

func loadDashboard(adminId, workspaceId, dashboardId int) (DashboardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name, visibility_type, sort_order, creator_id,
		revision, created_at, updated_at FROM `+define.TableWorkspaceDashboards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND deleted_at = 0 LIMIT 1`, nil,
		adminId, workspaceId, dashboardId)
	if err != nil {
		logs.Error("load dashboard failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	if len(rows) == 0 {
		return DashboardModel{}, ErrDashboardNotFound
	}
	return dashboardModelFromRow(rows[0])
}

func dashboardModelFromRow(row msql.Params) (DashboardModel, error) {
	parse := func(key string) (int, error) {
		value, err := strconv.Atoi(row[key])
		if err != nil {
			return 0, fmt.Errorf("parse dashboard %s: %w", key, err)
		}
		return value, nil
	}
	parse64 := func(key string) (int64, error) {
		value, err := strconv.ParseInt(row[key], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse dashboard %s: %w", key, err)
		}
		return value, nil
	}
	id, err := parse(`id`)
	if err != nil {
		return DashboardModel{}, err
	}
	sortOrder, err := parse(`sort_order`)
	if err != nil {
		return DashboardModel{}, err
	}
	creatorId, err := parse(`creator_id`)
	if err != nil {
		return DashboardModel{}, err
	}
	revision, err := parse64(`revision`)
	if err != nil {
		return DashboardModel{}, err
	}
	createdAt, err := parse64(`created_at`)
	if err != nil {
		return DashboardModel{}, err
	}
	updatedAt, err := parse64(`updated_at`)
	if err != nil {
		return DashboardModel{}, err
	}
	return DashboardModel{
		Id: id, Name: row[`name`], SortOrder: sortOrder, VisibilityType: row[`visibility_type`],
		CreatorId: creatorId, Revision: revision, CreatedAt: createdAt, UpdatedAt: updatedAt,
		RoleIds: []int{}, MemberIds: []int{}, EditorIds: []int{}, Cards: []DashboardCardModel{},
	}, nil
}

func loadDashboardRelations(adminId, workspaceId int, dashboards []DashboardModel) error {
	if len(dashboards) == 0 {
		return nil
	}
	ids := make([]int, 0, len(dashboards))
	byID := make(map[int]*DashboardModel, len(dashboards))
	for index := range dashboards {
		ids = append(ids, dashboards[index].Id)
		byID[dashboards[index].Id] = &dashboards[index]
	}
	queries := []struct {
		table  string
		column string
		apply  func(*DashboardModel, int)
	}{
		{define.TableDashboardVisibilityRoles, `role_id`, func(item *DashboardModel, id int) { item.RoleIds = append(item.RoleIds, id) }},
		{define.TableDashboardVisibilityMembers, `user_id`, func(item *DashboardModel, id int) { item.MemberIds = append(item.MemberIds, id) }},
		{define.TableDashboardEditors, `user_id`, func(item *DashboardModel, id int) { item.EditorIds = append(item.EditorIds, id) }},
	}
	for _, query := range queries {
		rows, err := msql.RawValues(define.Postgres, `SELECT dashboard_id, `+query.column+` FROM `+query.table+`
			WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = ANY($3) ORDER BY dashboard_id ASC, `+query.column+` ASC`, nil,
			adminId, workspaceId, pq.Array(ids))
		if err != nil {
			logs.Error("load dashboard relation failed, table:%s, err:%s", query.table, err.Error())
			return err
		}
		for _, row := range rows {
			if item := byID[cast.ToInt(row[`dashboard_id`])]; item != nil {
				query.apply(item, cast.ToInt(row[query.column]))
			}
		}
	}
	return nil
}

func loadDashboardCards(adminId, workspaceId, dashboardId int) ([]DashboardCardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, dashboard_id, card_type, title, config, content,
		content_mode, grid_x, grid_y, grid_width, grid_height, revision, created_by, created_at, updated_at
		FROM `+define.TableDashboardCards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3 AND deleted_at = 0
		ORDER BY grid_y ASC, grid_x ASC, id ASC`, nil, adminId, workspaceId, dashboardId)
	if err != nil {
		logs.Error("load dashboard cards failed, err:%s", err.Error())
		return nil, err
	}
	items := make([]DashboardCardModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := dashboardCardModelFromRow(row)
		if convErr != nil {
			return nil, convErr
		}
		items = append(items, item)
	}
	return items, nil
}

func dashboardCardModelFromRow(row msql.Params) (DashboardCardModel, error) {
	parse := func(key string) (int, error) {
		value, err := strconv.Atoi(row[key])
		if err != nil {
			return 0, fmt.Errorf("parse dashboard card %s: %w", key, err)
		}
		return value, nil
	}
	parse64 := func(key string) (int64, error) {
		value, err := strconv.ParseInt(row[key], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse dashboard card %s: %w", key, err)
		}
		return value, nil
	}
	item := DashboardCardModel{
		CardType: row[`card_type`], Title: row[`title`], Config: row[`config`], Content: row[`content`], ContentMode: row[`content_mode`],
	}
	var err error
	for _, field := range []struct {
		key    string
		target *int
	}{
		{`id`, &item.Id}, {`dashboard_id`, &item.DashboardId}, {`grid_x`, &item.X}, {`grid_y`, &item.Y},
		{`grid_width`, &item.W}, {`grid_height`, &item.H}, {`created_by`, &item.CreatedBy},
	} {
		if *field.target, err = parse(field.key); err != nil {
			return DashboardCardModel{}, err
		}
	}
	for _, field := range []struct {
		key    string
		target *int64
	}{
		{`revision`, &item.Revision}, {`created_at`, &item.CreatedAt}, {`updated_at`, &item.UpdatedAt},
	} {
		if *field.target, err = parse64(field.key); err != nil {
			return DashboardCardModel{}, err
		}
	}
	return item, nil
}

func getDashboardForActor(adminId, workspaceId, userId int, actorIsAdmin bool, dashboardId int) (DashboardModel, error) {
	if err := requireDashboardFeature(adminId, workspaceId); err != nil {
		return DashboardModel{}, err
	}
	dashboard, err := loadDashboard(adminId, workspaceId, dashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	relations, err := dashboardRelationsForId(adminId, workspaceId, dashboardId)
	if err != nil {
		return DashboardModel{}, err
	}
	dashboard.RoleIds, dashboard.MemberIds, dashboard.EditorIds = relations.RoleIds, relations.MemberIds, relations.EditorIds
	isAdmin, err := dashboardActorIsProjectAdmin(adminId, workspaceId, userId, actorIsAdmin)
	if err != nil {
		return DashboardModel{}, err
	}
	roleId, err := dashboardActorRoleId(adminId, workspaceId, userId)
	if err != nil {
		return DashboardModel{}, err
	}
	hasViewPermission := HasPermission(adminId, userId, cast.ToString(workspaceId), define.PermissionDashboardView)
	if !dashboardCanView(dashboard, userId, isAdmin, hasViewPermission, roleId) {
		return DashboardModel{}, ErrDashboardNotFound
	}
	setDashboardCapabilities(&dashboard, userId, isAdmin)
	return dashboard, nil
}

func dashboardRelationsForId(adminId, workspaceId, dashboardId int) (dashboardRelations, error) {
	result := dashboardRelations{RoleIds: []int{}, MemberIds: []int{}, EditorIds: []int{}}
	queries := []struct {
		table  string
		column string
		target *[]int
	}{
		{define.TableDashboardVisibilityRoles, `role_id`, &result.RoleIds},
		{define.TableDashboardVisibilityMembers, `user_id`, &result.MemberIds},
		{define.TableDashboardEditors, `user_id`, &result.EditorIds},
	}
	for _, query := range queries {
		rows, err := msql.RawValues(define.Postgres, `SELECT `+query.column+` FROM `+query.table+`
			WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3 ORDER BY `+query.column+` ASC`, nil,
			adminId, workspaceId, dashboardId)
		if err != nil {
			logs.Error("load dashboard relations failed, table:%s, err:%s", query.table, err.Error())
			return dashboardRelations{}, err
		}
		for _, row := range rows {
			*query.target = append(*query.target, cast.ToInt(row[query.column]))
		}
	}
	return result, nil
}

func dashboardCanView(dashboard DashboardModel, userId int, isAdmin, hasViewPermission bool, roleId int) bool {
	if !hasViewPermission {
		return false
	}
	if isAdmin || dashboard.CreatorId == userId || intSliceContains(dashboard.EditorIds, userId) {
		return true
	}
	switch dashboard.VisibilityType {
	case define.DashboardVisibilitySelf:
		return false
	case define.DashboardVisibilityPublic:
		return true
	case define.DashboardVisibilityRoles:
		return intSliceContains(dashboard.RoleIds, roleId)
	case define.DashboardVisibilityMembers:
		return intSliceContains(dashboard.MemberIds, userId)
	default:
		return false
	}
}

func dashboardCanEdit(dashboard DashboardModel, userId int, isAdmin bool) bool {
	return isAdmin || dashboard.CreatorId == userId || intSliceContains(dashboard.EditorIds, userId)
}

func setDashboardCapabilities(dashboard *DashboardModel, userId int, isAdmin bool) {
	dashboard.CanEdit = dashboardCanEdit(*dashboard, userId, isAdmin)
	dashboard.CanManage = isAdmin || dashboard.CreatorId == userId
	// Editors can reorder only their own editable dashboards; project administrators can reorder all.
	dashboard.CanReorder = dashboard.CanEdit
}

func normalizeDashboardIDs(values []int) ([]int, error) {
	set := make(map[int]bool, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || set[value] {
			return nil, ErrDashboardInvalid
		}
		set[value] = true
		result = append(result, value)
	}
	sort.Ints(result)
	return result, nil
}

func dashboardContentMode(value string) (string, error) {
	if value == `` {
		return define.DashboardContentModeRichText, nil
	}
	if value != define.DashboardContentModeRichText {
		return ``, ErrDashboardInvalid
	}
	return value, nil
}

func validateDashboardPayloadSize(value string, max int) error {
	if len([]byte(value)) > max {
		return ErrDashboardInvalid
	}
	return nil
}

func validateDashboardFiltersJSON(filters string) (define.DashboardFilters, error) {
	if err := validateDashboardPayloadSize(filters, define.DashboardFiltersMaxBytes); err != nil {
		return define.DashboardFilters{}, err
	}
	if strings.TrimSpace(filters) == `` || strings.TrimSpace(filters) == `null` {
		return define.DashboardFilters{}, nil
	}
	var sections define.DashboardFilters
	if err := json.Unmarshal([]byte(filters), &sections); err != nil {
		return define.DashboardFilters{}, ErrDashboardInvalidFilters
	}
	return sections, nil
}

// ValidateDashboardFilters validates a dashboard's combined requirement/defect filter tree.
func ValidateDashboardFilters(adminId, workspaceId int, filters string, dataScope string) error {
	if err := validate.DashboardFilters([]byte(filters), dataScope); err != nil {
		return ErrDashboardInvalidFilters
	}
	sections, err := validateDashboardFiltersJSON(filters)
	if err != nil {
		return err
	}
	if strings.TrimSpace(filters) == `` || strings.TrimSpace(filters) == `null` {
		return nil
	}
	if dataScope != define.DashboardMetricDataScopeAll && dataScope != define.DashboardMetricDataScopeRequirement && dataScope != define.DashboardMetricDataScopeDefect {
		return ErrDashboardInvalidMetric
	}
	if sections.Common != nil {
		if err = validate.ValidateGanttCommonFilterFields(*sections.Common); err != nil {
			return ErrDashboardInvalidFilters
		}
		if err = ValidateWorkItemFilterFields(adminId, cast.ToString(workspaceId), define.TypeKeyRequirement, *sections.Common); err != nil {
			return err
		}
	}
	if sections.Requirement != nil {
		if err = validate.ValidateViewFiltersForType(*sections.Requirement, define.TypeKeyRequirement); err != nil {
			return ErrDashboardInvalidFilters
		}
		if err = ValidateWorkItemFilterFields(adminId, cast.ToString(workspaceId), define.TypeKeyRequirement, *sections.Requirement); err != nil {
			return err
		}
	}
	if sections.Defect != nil {
		if err = validate.ValidateViewFiltersForType(*sections.Defect, define.TypeKeyDefect); err != nil {
			return ErrDashboardInvalidFilters
		}
		if err = ValidateWorkItemFilterFields(adminId, cast.ToString(workspaceId), define.TypeKeyDefect, *sections.Defect); err != nil {
			return err
		}
	}
	return nil
}

func dashboardRevisionConflict(current int64) error {
	return &DashboardRevisionConflictError{CurrentRevision: current}
}

func loadDashboardForUpdateTx(tx *sql.Tx, adminId, workspaceId, dashboardId int) (DashboardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name, visibility_type, sort_order, creator_id,
		revision, created_at, updated_at FROM `+define.TableWorkspaceDashboards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND deleted_at = 0 FOR UPDATE`, tx,
		adminId, workspaceId, dashboardId)
	if err != nil {
		logs.Error("lock dashboard failed, err:%s", err.Error())
		return DashboardModel{}, err
	}
	if len(rows) == 0 {
		return DashboardModel{}, ErrDashboardNotFound
	}
	return dashboardModelFromRow(rows[0])
}

func loadDashboardCardsForUpdateTx(tx *sql.Tx, adminId, workspaceId, dashboardId int) ([]DashboardCardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, dashboard_id, card_type, title, config, content,
		content_mode, grid_x, grid_y, grid_width, grid_height, revision, created_by, created_at, updated_at
		FROM `+define.TableDashboardCards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3 AND deleted_at = 0
		ORDER BY grid_y ASC, grid_x ASC, id ASC FOR UPDATE`, tx, adminId, workspaceId, dashboardId)
	if err != nil {
		logs.Error("lock dashboard cards failed, err:%s", err.Error())
		return nil, err
	}
	items := make([]DashboardCardModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := dashboardCardModelFromRow(row)
		if convErr != nil {
			return nil, convErr
		}
		items = append(items, item)
	}
	return items, nil
}

func loadDashboardCard(adminId, workspaceId, dashboardId, cardId int) (DashboardCardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, dashboard_id, card_type, title, config, content,
		content_mode, grid_x, grid_y, grid_width, grid_height, revision, created_by, created_at, updated_at
		FROM `+define.TableDashboardCards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3 AND id = $4 AND deleted_at = 0 LIMIT 1`, nil,
		adminId, workspaceId, dashboardId, cardId)
	if err != nil {
		logs.Error("load dashboard card failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	if len(rows) == 0 {
		return DashboardCardModel{}, ErrDashboardNotFound
	}
	return dashboardCardModelFromRow(rows[0])
}

func loadDashboardCardForUpdateTx(tx *sql.Tx, adminId, workspaceId, dashboardId, cardId int) (DashboardCardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, dashboard_id, card_type, title, config, content,
		content_mode, grid_x, grid_y, grid_width, grid_height, revision, created_by, created_at, updated_at
		FROM `+define.TableDashboardCards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3 AND id = $4 AND deleted_at = 0 FOR UPDATE`, tx,
		adminId, workspaceId, dashboardId, cardId)
	if err != nil {
		logs.Error("lock dashboard card failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	if len(rows) == 0 {
		return DashboardCardModel{}, ErrDashboardNotFound
	}
	return dashboardCardModelFromRow(rows[0])
}

func dashboardRelationsForIdTx(tx *sql.Tx, adminId, workspaceId, dashboardId int) (dashboardRelations, error) {
	result := dashboardRelations{RoleIds: []int{}, MemberIds: []int{}, EditorIds: []int{}}
	queries := []struct {
		table  string
		column string
		target *[]int
	}{
		{define.TableDashboardVisibilityRoles, `role_id`, &result.RoleIds},
		{define.TableDashboardVisibilityMembers, `user_id`, &result.MemberIds},
		{define.TableDashboardEditors, `user_id`, &result.EditorIds},
	}
	for _, query := range queries {
		rows, err := msql.RawValues(define.Postgres, `SELECT `+query.column+` FROM `+query.table+`
			WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3 ORDER BY `+query.column+` ASC FOR UPDATE`, tx,
			adminId, workspaceId, dashboardId)
		if err != nil {
			logs.Error("lock dashboard relations failed, table:%s, err:%s", query.table, err.Error())
			return dashboardRelations{}, err
		}
		for _, row := range rows {
			*query.target = append(*query.target, cast.ToInt(row[query.column]))
		}
	}
	return result, nil
}

func replaceDashboardRelationsTx(tx *sql.Tx, adminId, workspaceId, dashboardId int, roleIds, memberIds, editorIds []int, now int64) error {
	for _, table := range []string{define.TableDashboardVisibilityRoles, define.TableDashboardVisibilityMembers, define.TableDashboardEditors} {
		if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+table+`
			WHERE admin_id = $1 AND workspace_id = $2 AND dashboard_id = $3`, tx, adminId, workspaceId, dashboardId); err != nil {
			logs.Error("delete dashboard relations failed, table:%s, err:%s", table, err.Error())
			return err
		}
	}
	for _, roleId := range roleIds {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableDashboardVisibilityRoles+`
			(admin_id, workspace_id, dashboard_id, role_id, created_at) VALUES ($1, $2, $3, $4, $5)`, tx,
			adminId, workspaceId, dashboardId, roleId, now); err != nil {
			logs.Error("insert dashboard visible role failed, err:%s", err.Error())
			return err
		}
	}
	for _, userId := range memberIds {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableDashboardVisibilityMembers+`
			(admin_id, workspace_id, dashboard_id, user_id, created_at) VALUES ($1, $2, $3, $4, $5)`, tx,
			adminId, workspaceId, dashboardId, userId, now); err != nil {
			logs.Error("insert dashboard visible member failed, err:%s", err.Error())
			return err
		}
	}
	for _, userId := range editorIds {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableDashboardEditors+`
			(admin_id, workspace_id, dashboard_id, user_id, created_at) VALUES ($1, $2, $3, $4, $5)`, tx,
			adminId, workspaceId, dashboardId, userId, now); err != nil {
			logs.Error("insert dashboard editor failed, err:%s", err.Error())
			return err
		}
	}
	return nil
}

// deleteDashboardRoleRelationsTx removes the dashboard visibility entries for a deleted project role.
func deleteDashboardRoleRelationsTx(tx *sql.Tx, adminId, workspaceId, roleId int) error {
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableDashboardVisibilityRoles+`
		WHERE admin_id = $1 AND workspace_id = $2 AND role_id = $3`, tx, adminId, workspaceId, roleId); err != nil {
		logs.Error("delete dashboard role visibility relations failed, err:%s", err.Error())
		return err
	}
	return nil
}

// deleteDashboardMemberRelationsTx removes every dashboard-specific reference to a member leaving a project.
func deleteDashboardMemberRelationsTx(tx *sql.Tx, adminId, workspaceId, userId int) error {
	for _, table := range []string{
		define.TableDashboardVisibilityMembers,
		define.TableDashboardEditors,
		define.TableDashboardUserStates,
	} {
		if _, err := msql.RawExec(define.Postgres, `DELETE FROM `+table+`
			WHERE admin_id = $1 AND workspace_id = $2 AND user_id = $3`, tx, adminId, workspaceId, userId); err != nil {
			logs.Error("delete dashboard member relations failed, table:%s, err:%s", table, err.Error())
			return err
		}
	}
	return nil
}

func validateDashboardRelationTargets(adminId, workspaceId, creatorId int, roleIds, memberIds, editorIds []int) ([]int, []int, []int, error) {
	normalizedRoles, err := normalizeDashboardIDs(roleIds)
	if err != nil {
		return nil, nil, nil, err
	}
	normalizedMembers, err := normalizeDashboardIDs(memberIds)
	if err != nil {
		return nil, nil, nil, err
	}
	normalizedEditors, err := normalizeDashboardIDs(editorIds)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(normalizedRoles) > 0 {
		rows, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableRoles+`
			WHERE admin_id = $1 AND workspace_id = $2 AND scope = $3 AND id = ANY($4::INTEGER[])`, nil,
			adminId, workspaceId, permission.ScopeProject, pq.Array(normalizedRoles))
		if queryErr != nil {
			logs.Error("validate dashboard role targets failed, err:%s", queryErr.Error())
			return nil, nil, nil, queryErr
		}
		if len(rows) != len(normalizedRoles) {
			return nil, nil, nil, ErrDashboardInvalid
		}
	}
	filteredEditors, err := filterImplicitDashboardEditors(adminId, workspaceId, creatorId, normalizedEditors)
	if err != nil {
		return nil, nil, nil, err
	}
	membersToValidate := append(append([]int{}, normalizedMembers...), filteredEditors...)
	memberSet := make(map[int]struct{}, len(membersToValidate))
	for _, userId := range membersToValidate {
		memberSet[userId] = struct{}{}
	}
	if len(memberSet) > 0 {
		ids := make([]int, 0, len(memberSet))
		for userId := range memberSet {
			ids = append(ids, userId)
		}
		rows, queryErr := msql.RawValues(define.Postgres, `SELECT wm.user_id FROM `+define.TableWorkspaceMembers+` wm
			JOIN `+define.TableUsers+` u ON u.admin_id = wm.admin_id AND u.id = wm.user_id AND COALESCE(u.status, 1) = 1
			WHERE wm.admin_id = $1 AND wm.workspace_id = $2 AND wm.user_id = ANY($3::INTEGER[])`, nil,
			adminId, workspaceId, pq.Array(ids))
		if queryErr != nil {
			logs.Error("validate dashboard member targets failed, err:%s", queryErr.Error())
			return nil, nil, nil, queryErr
		}
		if len(rows) != len(ids) {
			return nil, nil, nil, ErrDashboardInvalid
		}
	}
	return normalizedRoles, normalizedMembers, filteredEditors, nil
}

func filterImplicitDashboardEditors(adminId, workspaceId, creatorId int, editorIds []int) ([]int, error) {
	if len(editorIds) == 0 {
		return []int{}, nil
	}
	adminRows, err := msql.RawValues(define.Postgres, `SELECT wm.user_id FROM `+define.TableWorkspaceMembers+` wm
		JOIN `+define.TableRoles+` r ON r.admin_id = wm.admin_id AND r.id = wm.role_id
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2 AND wm.user_id = ANY($3::INTEGER[])
			AND r.workspace_id = $2 AND r.scope = $4 AND r.is_system = 1 AND r.name IN ($5, $6)`, nil,
		adminId, workspaceId, pq.Array(editorIds), permission.ScopeProject,
		dashboardProjectAdminRoleName, lib_i18n.WrapKey(dashboardProjectAdminRoleName))
	if err != nil {
		logs.Error("load implicit dashboard editors failed, err:%s", err.Error())
		return nil, err
	}
	projectAdmins := make(map[int]struct{}, len(adminRows))
	for _, row := range adminRows {
		projectAdmins[cast.ToInt(row[`user_id`])] = struct{}{}
	}
	result := make([]int, 0, len(editorIds))
	for _, userId := range editorIds {
		if userId == creatorId || IsTenantOwner(adminId, userId) {
			continue
		}
		if _, implicit := projectAdmins[userId]; implicit {
			continue
		}
		result = append(result, userId)
	}
	return result, nil
}

func dashboardOrderIDs(items []DashboardOrderItemModel) ([]int, error) {
	ids := make([]int, 0, len(items))
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		if item.DashboardId <= 0 {
			return nil, ErrDashboardInvalid
		}
		if _, duplicate := seen[item.DashboardId]; duplicate {
			return nil, ErrDashboardInvalid
		}
		seen[item.DashboardId] = struct{}{}
		ids = append(ids, item.DashboardId)
	}
	if len(ids) == 0 {
		return nil, ErrDashboardInvalid
	}
	return ids, nil
}

func sameDashboardIDSet(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[int]struct{}, len(left))
	for _, id := range left {
		set[id] = struct{}{}
	}
	for _, id := range right {
		if _, exists := set[id]; !exists {
			return false
		}
		delete(set, id)
	}
	return len(set) == 0
}

func sameRelativeNonEditableOrder(currentIDs, nextIDs []int, byID map[int]DashboardModel) bool {
	current := make([]int, 0, len(currentIDs))
	for _, id := range currentIDs {
		if !byID[id].CanEdit {
			current = append(current, id)
		}
	}
	next := make([]int, 0, len(nextIDs))
	for _, id := range nextIDs {
		if !byID[id].CanEdit {
			next = append(next, id)
		}
	}
	if len(current) != len(next) {
		return false
	}
	for index := range current {
		if current[index] != next[index] {
			return false
		}
	}
	return true
}

func insertDashboardCardTx(tx *sql.Tx, adminId, workspaceId, dashboardId, userId int, card DashboardCardModel, now int64) (DashboardCardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableDashboardCards+`
		(admin_id, workspace_id, dashboard_id, card_type, title, config, content, content_mode,
		 grid_x, grid_y, grid_width, grid_height, revision, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 1, $13, $14, $14) RETURNING id`, tx,
		adminId, workspaceId, dashboardId, card.CardType, card.Title, card.Config, card.Content, card.ContentMode,
		card.X, card.Y, card.W, card.H, userId, now)
	if err != nil {
		logs.Error("insert dashboard card failed, err:%s", err.Error())
		return DashboardCardModel{}, err
	}
	if len(rows) != 1 {
		return DashboardCardModel{}, ErrDashboardInvalid
	}
	card.Id = cast.ToInt(rows[0][`id`])
	card.DashboardId = dashboardId
	card.Revision = 1
	card.CreatedBy = userId
	card.CreatedAt = now
	card.UpdatedAt = now
	return card, nil
}

func bumpDashboardRevisionTx(tx *sql.Tx, adminId, workspaceId, dashboardId int, now int64) error {
	if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkspaceDashboards+`
		SET revision = revision + 1, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND id = $4 AND deleted_at = 0`, tx,
		now, adminId, workspaceId, dashboardId); err != nil {
		logs.Error("bump dashboard revision failed, err:%s", err.Error())
		return err
	}
	return nil
}

func dashboardCardsContainType(cards []DashboardCardModel, cardType string) bool {
	for _, card := range cards {
		if card.CardType == cardType {
			return true
		}
	}
	return false
}

func firstOpenDashboardPosition(cards []DashboardCardModel, width, height int) (int, int) {
	width = maxDashboardGridSize(width, 1, define.DashboardGridColumns)
	height = maxDashboardGridSize(height, 1, define.DashboardGridMaxRows)
	maxCandidateY := define.DashboardGridMaxRows - height
	occupied := make([]DashboardCardModel, 0, len(cards))
	candidateSet := map[int]struct{}{0: {}}
	for _, card := range cards {
		if card.X < 0 || card.Y < 0 || card.W < 1 || card.H < 1 ||
			card.W > define.DashboardGridColumns || card.H > define.DashboardGridMaxRows ||
			card.X > define.DashboardGridColumns-card.W || card.Y > define.DashboardGridMaxRows-card.H {
			continue
		}
		occupied = append(occupied, card)
		if bottom := card.Y + card.H; bottom <= maxCandidateY {
			candidateSet[bottom] = struct{}{}
		}
	}
	candidateYs := make([]int, 0, len(candidateSet))
	for y := range candidateSet {
		candidateYs = append(candidateYs, y)
	}
	sort.Ints(candidateYs)
	for _, y := range candidateYs {
		for x := 0; x+width <= define.DashboardGridColumns; x++ {
			candidate := DashboardCardModel{X: x, Y: y, W: width, H: height}
			if dashboardCardFits(occupied, candidate, 0) {
				return x, y
			}
		}
	}
	return 0, maxCandidateY
}

func maxDashboardGridSize(value, lower, upper int) int {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func validateDashboardCardLayout(cards []DashboardCardModel, candidate DashboardCardModel, excludeId int) error {
	items := make([]validate.DashboardLayoutItem, 0, len(cards)+1)
	for _, card := range cards {
		if card.Id == excludeId {
			continue
		}
		items = append(items, validate.DashboardLayoutItem{Id: card.Id, X: card.X, Y: card.Y, W: card.W, H: card.H})
	}
	items = append(items, validate.DashboardLayoutItem{Id: candidate.Id, X: candidate.X, Y: candidate.Y, W: candidate.W, H: candidate.H})
	if candidate.Id == 0 {
		items[len(items)-1].Id = dashboardSyntheticCardID(cards)
	}
	if err := validate.DashboardLayout(items); err != nil {
		return ErrDashboardInvalidLayout
	}
	return nil
}

func dashboardSyntheticCardID(cards []DashboardCardModel) int {
	maxID := 0
	for _, card := range cards {
		if card.Id > maxID {
			maxID = card.Id
		}
	}
	return maxID + 1
}

func dashboardCardFits(cards []DashboardCardModel, candidate DashboardCardModel, excludeId int) bool {
	for _, card := range cards {
		if card.Id == excludeId {
			continue
		}
		if candidate.X < card.X+card.W && card.X < candidate.X+candidate.W && candidate.Y < card.Y+card.H && card.Y < candidate.Y+candidate.H {
			return false
		}
	}
	return true
}

func validateDashboardCardPayload(adminId, workspaceId int, card DashboardCardModel, fileIds []int) error {
	if validate.DashboardCardType(card.CardType) != nil || validate.DashboardCardTitle(card.Title) != nil ||
		validate.DashboardConfig([]byte(card.Config)) != nil || validate.DashboardContent(card.Content) != nil ||
		validate.DashboardFileIDs(fileIds) != nil {
		return ErrDashboardInvalid
	}
	if card.ContentMode != define.DashboardContentModeRichText {
		return ErrDashboardInvalid
	}
	switch card.CardType {
	case define.DashboardCardTypeChart:
		if card.Content != `` || len(fileIds) > 0 {
			return ErrDashboardInvalid
		}
		_, err := parseDashboardChartConfig(adminId, workspaceId, card.Config)
		return err
	case define.DashboardCardTypeMetric:
		if card.Content != `` || len(fileIds) > 0 {
			return ErrDashboardInvalid
		}
		_, _, err := parseDashboardMetricConfig(adminId, workspaceId, card.Config)
		return err
	case define.DashboardCardTypeProjectInfo, define.DashboardCardTypeAnnouncement:
		if card.Content != `` || len(fileIds) > 0 {
			return ErrDashboardInvalid
		}
	case define.DashboardCardTypeText:
		return nil
	default:
		return ErrDashboardInvalid
	}
	return nil
}

func parseDashboardMetricConfig(adminId, workspaceId int, raw string) (DashboardMetricConfig, string, error) {
	if validate.DashboardConfig([]byte(raw)) != nil {
		return DashboardMetricConfig{}, ``, ErrDashboardInvalidMetric
	}
	var config DashboardMetricConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return DashboardMetricConfig{}, ``, ErrDashboardInvalidMetric
	}
	if validate.DashboardMetric(config.MetricType, config.DataScope) != nil || validate.DashboardMetricStyle(config.Unit, config.Color) != nil ||
		validate.DashboardMetricRecentDays(config.RecentDays) != nil {
		return DashboardMetricConfig{}, ``, ErrDashboardInvalidMetric
	}
	if err := validate.DashboardSLAMetricOptions(config.MetricType, config.SLAStageIds, config.SLAShowComparison, config.SLALineColor, config.SLARecentDays); err != nil {
		return DashboardMetricConfig{}, ``, err
	}
	if strings.HasPrefix(config.MetricType, "sla_") {
		normalizeDashboardSLAMetric(&config)
		stages, err := dashboardSLAStageNames(adminId, workspaceId, config.SLAStageIds)
		if err != nil {
			return DashboardMetricConfig{}, ``, err
		}
		if len(config.SLAStageIds) == 1 && (config.MetricType == define.DashboardMetricSLAStageAchievementRate ||
			config.MetricType == define.DashboardMetricSLAStageCriticalCount) {
			config.SLAStageName = stages[config.SLAStageIds[0]]
		}
	}
	filters := strings.TrimSpace(string(config.Filters))
	if err := ValidateDashboardFilters(adminId, workspaceId, filters, config.DataScope); err != nil {
		return DashboardMetricConfig{}, ``, err
	}
	return config, filters, nil
}

func dashboardMetricRequiredTypes(metricType, dataScope string) (bool, bool) {
	if metricType == define.DashboardMetricWorkItemCount {
		return dataScope == define.DashboardMetricDataScopeAll || dataScope == define.DashboardMetricDataScopeRequirement,
			dataScope == define.DashboardMetricDataScopeAll || dataScope == define.DashboardMetricDataScopeDefect
	}
	if dataScope == define.DashboardMetricDataScopeDefect {
		return false, true
	}
	return true, false
}

func dashboardMetricExpression(adminId, workspaceId int, metricType string, args *[]any, where *[]string) (string, error) {
	switch metricType {
	case define.DashboardMetricWorkItemCount, define.DashboardMetricCreatedRequirementCount, define.DashboardMetricCreatedDefectCount,
		define.DashboardMetricClosedCount, define.DashboardMetricInProgressCount, define.DashboardMetricIssuePendingCount,
		define.DashboardMetricIssueCreatedToday, define.DashboardMetricIssueCompletedToday:
		return `COUNT(*)`, nil
	case define.DashboardMetricEstimatedWorkHours:
		return `COALESCE(SUM(r.work_hours), 0)`, nil
	case define.DashboardMetricCompletedWorkHours:
		return `COALESCE(SUM(r.completed_work_hours), 0)`, nil
	case define.DashboardMetricRemainingWorkHours:
		return `COALESCE(SUM(GREATEST(r.work_hours - r.completed_work_hours, 0)), 0)`, nil
	case define.DashboardMetricOvertimeWorkHours:
		return `COALESCE(SUM(GREATEST(r.completed_work_hours - r.work_hours, 0)), 0)`, nil
	case define.DashboardMetricCompletedRequirementCount, define.DashboardMetricCompletedDefectCount:
		settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: adminId, WsId: cast.ToString(workspaceId)})
		if err != nil {
			return ``, err
		}
		finalStatusIds := settings.RequirementCompletedStatusIds
		if metricType == define.DashboardMetricCompletedDefectCount {
			finalStatusIds = settings.DefectCompletedStatusIds
		}
		if len(finalStatusIds) == 0 {
			*where = append(*where, `FALSE`)
			return `COUNT(*)`, nil
		}
		*args = append(*args, pq.Array(finalStatusIds))
		*where = append(*where, fmt.Sprintf(`r.status_id = ANY($%d::INTEGER[])`, len(*args)))
		return `COUNT(*)`, nil
	case define.DashboardMetricOverdueRequirementCount:
		// Overdue: has a planned end date in the past and the requirement is not in a final status.
		// Without configured final statuses there is no "done" notion, so nothing counts as overdue.
		settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: adminId, WsId: cast.ToString(workspaceId)})
		if err != nil {
			return ``, err
		}
		if len(settings.RequirementFinalStatusIds) == 0 {
			*where = append(*where, `FALSE`)
			return `COUNT(*)`, nil
		}
		*args = append(*args, pq.Array(settings.RequirementFinalStatusIds))
		*where = append(*where, fmt.Sprintf(`r.status_id != ALL($%d::INTEGER[])`, len(*args)))
		*args = append(*args, int64(tool.Time2Int()))
		*where = append(*where, fmt.Sprintf(`r.planned_end_date > 0 AND r.planned_end_date < $%d`, len(*args)))
		return `COUNT(*)`, nil
	case define.DashboardMetricUnresolvedDefectCount:
		// Unresolved: defect not in a final status; without configured final statuses every defect counts.
		settings, err := GetWorkspaceSettings(WorkspaceSettingsModel{AdminId: adminId, WsId: cast.ToString(workspaceId)})
		if err != nil {
			return ``, err
		}
		if len(settings.DefectFinalStatusIds) > 0 {
			*args = append(*args, pq.Array(settings.DefectFinalStatusIds))
			*where = append(*where, fmt.Sprintf(`r.status_id != ALL($%d::INTEGER[])`, len(*args)))
		}
		return `COUNT(*)`, nil
	default:
		return ``, ErrDashboardInvalidMetric
	}
}

func loadDashboardAnnouncement(adminId, workspaceId int) (DashboardAnnouncementModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, content, content_mode, revision, created_by, updated_by, created_at, updated_at
		FROM `+define.TableDashboardAnnouncements+`
		WHERE admin_id = $1 AND workspace_id = $2 LIMIT 1`, nil, adminId, workspaceId)
	if err != nil {
		logs.Error("load dashboard announcement failed, err:%s", err.Error())
		return DashboardAnnouncementModel{}, err
	}
	if len(rows) == 0 {
		return DashboardAnnouncementModel{}, ErrDashboardNotFound
	}
	row := rows[0]
	return DashboardAnnouncementModel{
		Id: cast.ToInt(row[`id`]), AdminId: adminId, WorkspaceId: workspaceId,
		Content: row[`content`], ContentMode: row[`content_mode`], Revision: cast.ToInt64(row[`revision`]),
		CreatedBy: cast.ToInt(row[`created_by`]), UpdatedBy: cast.ToInt(row[`updated_by`]),
		CreatedAt: cast.ToInt64(row[`created_at`]), UpdatedAt: cast.ToInt64(row[`updated_at`]),
	}, nil
}

func validateDashboardAnnouncementMutation(model DashboardAnnouncementMutationModel) error {
	if validate.DashboardRevision(model.Revision) != nil || validate.DashboardContent(model.Content) != nil ||
		validate.DashboardFileIDs(model.FileIds) != nil {
		return ErrDashboardInvalid
	}
	if _, err := dashboardContentMode(model.ContentMode); err != nil {
		return err
	}
	return nil
}

func updateDashboardAnnouncementTx(tx *sql.Tx, adminId, workspaceId, userId int, dashboardName string, model DashboardAnnouncementMutationModel, now int64) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, revision FROM `+define.TableDashboardAnnouncements+`
		WHERE admin_id = $1 AND workspace_id = $2 FOR UPDATE`, tx, adminId, workspaceId)
	if err != nil {
		logs.Error("lock dashboard announcement failed, err:%s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrDashboardNotFound
	}
	if currentRevision := cast.ToInt64(rows[0][`revision`]); currentRevision != model.Revision {
		return dashboardRevisionConflict(currentRevision)
	}
	contentMode, err := dashboardContentMode(model.ContentMode)
	if err != nil {
		return err
	}
	announcementId := cast.ToInt(rows[0][`id`])
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardAnnouncements+`
		SET content = $1, content_mode = $2, revision = revision + 1, updated_by = $3, updated_at = $4
		WHERE admin_id = $5 AND workspace_id = $6 AND id = $7`, tx,
		model.Content, contentMode, userId, now, adminId, workspaceId, announcementId); err != nil {
		logs.Error("update dashboard announcement failed, err:%s", err.Error())
		return err
	}
	return bindDashboardPendingFilesTx(tx, adminId, workspaceId, userId, announcementId,
		define.SourceTypeDashboardAnnouncement, dashboardName, model.FileIds)
}

func dashboardWorkItemConditions(model DashboardMetricQueryModel, dataScope, cardFilters string, issueWorkspace bool) ([]string, []any, error) {
	dataScope = dashboardDataScope(dataScope, issueWorkspace)
	topFilters := ``
	if model.TopFilters == nil {
		state, stateErr := GetDashboardUserState(DashboardUserStateModel{
			AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, UserId: model.UserId,
			ActorIsAdmin: model.ActorIsAdmin, DashboardId: model.DashboardId,
		})
		if stateErr != nil {
			return nil, nil, stateErr
		}
		topFilters = state.Filters
	} else {
		topFilters = strings.TrimSpace(*model.TopFilters)
	}
	if err := ValidateDashboardFilters(model.AdminId, model.WorkspaceId, topFilters, define.DashboardMetricDataScopeAll); err != nil {
		return nil, nil, err
	}
	requiresRequirement, requiresDefect := dashboardMetricRequiredTypes(define.DashboardMetricWorkItemCount, dataScope)
	if (requiresRequirement && !HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), `requirement:view`)) ||
		(requiresDefect && !HasPermission(model.AdminId, model.UserId, cast.ToString(model.WorkspaceId), `defect:view`)) {
		return nil, nil, ErrDashboardForbidden
	}
	where := []string{`r.admin_id = $1`, `r.workspace_id = $2`, `r.deleted_at = 0`}
	args := []any{model.AdminId, model.WorkspaceId}
	switch dataScope {
	case "":
		where = append(where, `FALSE`)
	case define.DashboardMetricDataScopeRequirement:
		where = append(where, `r.type = '`+define.TypeKeyRequirement+`'`)
	case define.DashboardMetricDataScopeDefect:
		where = append(where, `r.type = '`+define.TypeKeyDefect+`'`)
	default:
		where = append(where, `r.type IN ('`+define.TypeKeyRequirement+`', '`+define.TypeKeyDefect+`')`)
	}
	scopeSQL, scopeArgs, err := buildDataViewScopeCondition(model.AdminId, cast.ToString(model.WorkspaceId), model.UserId, len(args)+1)
	if err != nil {
		return nil, nil, err
	}
	if scopeSQL != `` {
		where = append(where, scopeSQL)
		args = append(args, scopeArgs...)
	}
	for _, filters := range []string{topFilters, cardFilters} {
		if strings.TrimSpace(filters) == `` || strings.TrimSpace(filters) == `null` {
			continue
		}
		sections, sectionErr := validateDashboardFiltersJSON(filters)
		if sectionErr != nil {
			return nil, nil, sectionErr
		}
		if issueWorkspace {
			// 不让已隐藏的历史缺陷条件继续影响工单统计。
			sections.Defect = nil
			sections.WorkItemTypes = nil
			encoded, encodeErr := json.Marshal(sections)
			if encodeErr != nil {
				return nil, nil, encodeErr
			}
			filters = string(encoded)
		}
		if len(sections.WorkItemTypes) > 0 {
			args = append(args, pq.Array(sections.WorkItemTypes))
			where = append(where, fmt.Sprintf(`r.type = ANY($%d::TEXT[])`, len(args)))
		}
		filterSQL, filterArgs, filterErr := buildGanttViewFilterCondition(model.AdminId, cast.ToString(model.WorkspaceId), model.UserId,
			[]int{model.WorkspaceId}, filters, model.Timezone, len(args)+1)
		if filterErr != nil {
			return nil, nil, filterErr
		}
		if filterSQL != `` {
			where = append(where, filterSQL)
			args = append(args, filterArgs...)
		}
	}
	return where, args, nil
}
