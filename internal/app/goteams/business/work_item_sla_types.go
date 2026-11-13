package business

import (
	"encoding/json"
	"errors"
)

var (
	// ErrSLAInvalid 表示配置或操作参数无效。
	ErrSLAInvalid = errors.New("invalid SLA configuration")
	// ErrSLAConflict 表示配置版本或引用状态冲突。
	ErrSLAConflict = errors.New("SLA configuration conflict")
	// ErrSLANotFound 表示当前空间没有对应的资源。
	ErrSLANotFound = errors.New("SLA resource not found")
	// ErrSLAWorkspace 表示当前空间不是工单工作空间。
	ErrSLAWorkspace = errors.New("SLA requires an issue workspace")
	// ErrSLACalendar 表示计时所需年度数据不可用。
	ErrSLACalendar = errors.New("SLA calendar unavailable")
)

// SLASettingsModel 保存工作空间 SLA 设置及读取上下文。
type SLASettingsModel struct {
	AdminId, WorkspaceId, UserId int
	Enabled                      bool
	Timezone                     string
	ActivatedAt, Revision        int64
	ClockRevision                int64
	CalendarAvailable            bool
	CalendarRegion               string
}

// SLADefinitionModel 表示阶段或服务目标的完整配置用例。
type SLADefinitionModel struct {
	AdminId, WorkspaceId, UserId int
	Id, SortOrder                int
	Disabled                     bool
	Name, Description            string
	Revision                     int64
	PriorityIds, CategoryIds     []int
	Config                       json.RawMessage
}

// SLAOrderModel 表示原子调整服务目标顺序。
type SLAOrderModel struct {
	AdminId, WorkspaceId int
	Revision             int64
	Ids                  []int
}

// ListSLADefinitionsModel 保存有界配置列表及其分页上下文。
type ListSLADefinitionsModel struct {
	AdminId, WorkspaceId, UserId, Page, Size int
	Items                                    []SLADefinitionModel
	Total                                    int
}

// SLAWorkItemStateModel 是事务内读取的工单状态。
type SLAWorkItemStateModel struct {
	WorkspaceId, StatusId, PriorityId, CategoryId int
	CreatedAt                                     int64
}

// SLAChangeModel 仅用于创建、真实状态或优先级变化，不通过 HTTP 回调。
type SLAChangeModel struct {
	AdminId, WorkspaceId, RequirementId int
	MutationKey, Kind, Source           string
	OccurredAt                          int64
	Before, After                       SLAWorkItemStateModel
}

// SLAReadModel 表示当前工单 SLA 的读取条件。
type SLAReadModel struct {
	AdminId, WorkspaceId, RequirementId, UserId int
	Page, Size                                  int
}

// SLAStageResultModel 是面向界面的阶段计时投影。
type SLAStageResultModel struct {
	CycleId, StageId                 int64
	StageName, State, Reason         string
	CalculationState                 string
	StartedAt, EndedAt, LimitSeconds int64
	ElapsedSeconds, RemainingSeconds *int64
	OverdueSeconds, DeadlineAt       *int64
}

// SLAResultModel 保存单一计算时刻的当前阶段与周期历史。
type SLAResultModel struct {
	Enabled, Paused         bool
	ConfigurationIncomplete bool
	EvaluatedAt             int64
	Current                 []SLAStageResultModel
	History                 []SLAStageResultModel
	Total                   int
}

// SLABatchReadModel 表示列表页中已知工单 ID 的有界批量读取。
type SLABatchReadModel struct {
	AdminId, WorkspaceId, UserId int
	Ids                          []int
	Items                        []SLAItemResultModel
}

// SLAItemResultModel 将工单 ID 与其阶段结果关联。
type SLAItemResultModel struct {
	RequirementId int
	Result        SLAResultModel
}

// SLAOptionModel 表示同空间的配置选项，名称在响应边界翻译。
type SLAOptionModel struct {
	Id, ParentId int
	Name, Color  string
}

// SLAMetadataModel 保存 SLA 编辑所需选项，不额外要求角色管理权限。
type SLAMetadataModel struct {
	AdminId, WorkspaceId                    int
	Statuses, Priorities, Categories, Roles []SLAOptionModel
}

// RetrySLAActionModel 是管理员重试失败动作的可信上下文。
type RetrySLAActionModel struct {
	AdminId, WorkspaceId int
	ActionId             int64
}

// 以下类型仅用于版本化配置和周期快照存储，不作为 HTTP DTO 或业务用例 Model。
type slaSelector struct {
	Mode      string `json:"mode"`
	StatusIds []int  `json:"status_ids"`
}

type slaStageConfig struct {
	From slaSelector `json:"from"`
	To   slaSelector `json:"to"`
}

// slaSnapshotStageConfig 只为既有 schema_version=1 周期保留旧起止流转，配置接口不再接受该结构。
type slaSnapshotStageConfig struct {
	From  *slaSelector    `json:"from,omitempty"`
	To    *slaSelector    `json:"to,omitempty"`
	Start *slaStageConfig `json:"start,omitempty"`
	End   *slaStageConfig `json:"end,omitempty"`
}

type slaSlot struct {
	Weekdays    []int `json:"weekdays"`
	StartMinute int   `json:"start_minute"`
	EndMinute   int   `json:"end_minute"`
}

type slaCalendar struct {
	Mode                string           `json:"mode"`
	Timezone            string           `json:"timezone,omitempty"`
	Slots               []slaSlot        `json:"slots"`
	PauseOnHolidays     bool             `json:"pause_on_holidays"`
	HolidayCalendarCode string           `json:"holiday_calendar_code"`
	HolidayYears        []slaHolidayYear `json:"holiday_years,omitempty"`
}

type slaTarget struct {
	PriorityId   int   `json:"priority_id"`
	StageId      int   `json:"stage_id"`
	LimitSeconds int64 `json:"limit_seconds"`
}

type slaRecipient struct {
	Type   string `json:"type"`
	RoleId int    `json:"role_id,omitempty"`
}

type slaWebhook struct {
	Platform  string   `json:"platform"`
	URL       string   `json:"url"`
	Secret    string   `json:"secret,omitempty"`
	AtMode    string   `json:"at_mode"`
	AtUserIds []int    `json:"at_user_ids,omitempty"`
	AtRoles   []string `json:"at_roles,omitempty"`
	AtAll     bool     `json:"at_all,omitempty"`
}

type slaAction struct {
	Channel       string         `json:"channel,omitempty"`
	Webhooks      []slaWebhook   `json:"webhooks,omitempty"`
	Key           string         `json:"key"`
	StageId       int            `json:"stage_id"`
	Position      string         `json:"position"`
	OffsetSeconds int64          `json:"offset_seconds"`
	Recipients    []slaRecipient `json:"recipients"`
}

type slaPolicyConfig struct {
	Calendar slaCalendar `json:"calendar"`
	Targets  []slaTarget `json:"targets"`
	Actions  []slaAction `json:"actions"`
}

type slaSnapshotStage struct {
	Id       int                    `json:"id"`
	Revision int64                  `json:"revision"`
	Name     string                 `json:"name"`
	Config   slaSnapshotStageConfig `json:"config"`
}

// slaClockPeriod 保留优先级重新匹配前已发生的计时片段及原日历。
type slaClockPeriod struct {
	Start    int64       `json:"start"`
	End      int64       `json:"end"`
	Calendar slaCalendar `json:"calendar"`
}

type slaSnapshot struct {
	ClockPeriods  []slaClockPeriod   `json:"clock_periods,omitempty"`
	SchemaVersion int                `json:"schema_version"`
	PolicyId      int                `json:"policy_id"`
	Revision      int64              `json:"revision"`
	Name          string             `json:"name"`
	PriorityId    int                `json:"priority_id"`
	CategoryId    int                `json:"category_id"`
	Config        slaPolicyConfig    `json:"config"`
	Stages        []slaSnapshotStage `json:"stages"`
}

type slaPause struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// slaStoredDefinition 是设置 JSON 中的配置项，不作为 HTTP 或业务层契约。
type slaStoredDefinition struct {
	Id          int             `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	SortOrder   int             `json:"sort_order"`
	Revision    int64           `json:"revision"`
	PriorityIds []int           `json:"priority_ids"`
	CategoryIds []int           `json:"category_ids"`
	Config      json.RawMessage `json:"config"`
	ArchivedAt  int64           `json:"archived_at"`
	Disabled    bool            `json:"disabled,omitempty"`
}
