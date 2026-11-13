// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// PipelineProtocolVersion is the synchronization protocol version between the Pipeline client and the cloud.
const PipelineProtocolVersion = 2

const (
	// PipelineTaskPeriodAll represents all pipeline tasks.
	PipelineTaskPeriodAll = `all`
	// PipelineTaskPeriodToday represents today's pipeline tasks.
	PipelineTaskPeriodToday = `today`
	// PipelineTaskPeriodYesterday represents yesterday's pipeline tasks.
	PipelineTaskPeriodYesterday = `yesterday`
	// PipelineTaskPeriodWeek represents pipeline tasks from the last seven calendar days.
	PipelineTaskPeriodWeek = `week`
)

// ======================== Pipeline request/response structs ========================

// Pipeline scope values: who is allowed to use a pipeline.
const (
	// PipelineScopeAll makes the pipeline available to every tenant member.
	PipelineScopeAll = `all`
	// PipelineScopeAll makes the pipeline available to explicit pipeline_members rows.
	PipelineScopeMembers = `members`
	// PipelineScopeDepartments makes the pipeline available to members of the listed departments.
	PipelineScopeDepartments = `departments`
)

// PipelineListItem is a pipeline list item.
type PipelineListItem struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Color           string `json:"color"`
	Icon            string `json:"icon"`
	CliType         string `json:"cli_type"`
	WorkflowVersion int    `json:"workflow_version"`
	Status          int    `json:"status"`
	Scope           string `json:"scope"`
	MemberCount     int    `json:"member_count"`
	DepartmentCount int    `json:"department_count"`
	// ScopeMembers previews the first avatars of the scope for the card avatar stack;
	// ScopeTotal is the headcount covered by the scope (rendered as "+N").
	ScopeMembers []PipelineScopeMember `json:"scope_members"`
	ScopeTotal   int                   `json:"scope_total"`
	StepCount    int                   `json:"step_count"`
	CanManage    bool                  `json:"can_manage"`
	CanUse       bool                  `json:"can_use"`
	UpdatedAt    int                   `json:"updated_at"`
}

// PipelineScopeMember is a member avatar preview in a pipeline card avatar stack.
type PipelineScopeMember struct {
	UserId int    `json:"user_id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// PipelineDepartmentInfo is a department bound to a pipeline (scope=departments).
type PipelineDepartmentInfo struct {
	DepartmentId   int    `json:"department_id"`
	DepartmentName string `json:"department_name"`
}

// PipelineMemberInfo holds pipeline member information.
type PipelineMemberInfo struct {
	UserId   int    `json:"user_id"`
	UserName string `json:"user_name"`
}

// PipelineWorkflowStep is a workflow step.
type PipelineWorkflowStep struct {
	StepKey       string `json:"step_key"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	PromptContent string `json:"prompt_content"`
	Remark        string `json:"remark"`
	SortOrder     int    `json:"sort_order"`
}

// PipelineDetail is the detailed pipeline information.
type PipelineDetail struct {
	ID              int                      `json:"id"`
	AdminId         int                      `json:"-"`
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`
	Color           string                   `json:"color"`
	Icon            string                   `json:"icon"`
	CliType         string                   `json:"cli_type"`
	WorkflowVersion int                      `json:"workflow_version"`
	Status          int                      `json:"status"`
	Scope           string                   `json:"scope"`
	DepartmentIds   []int                    `json:"department_ids"`
	Departments     []PipelineDepartmentInfo `json:"departments"`
	CreatedBy       int                      `json:"-"`
	UpdatedAt       int                      `json:"updated_at"`
	Members         []PipelineMemberInfo     `json:"members"`
	Steps           []PipelineWorkflowStep   `json:"steps"`
}

// CreatePipelineStepReq is the request payload for creating a pipeline step.
type CreatePipelineStepReq struct {
	StepKey       string `json:"step_key"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	PromptContent string `json:"prompt_content"`
	Remark        string `json:"remark"`
	SortOrder     int    `json:"sort_order"`
}

// CreatePipelineReq is the request payload for creating a pipeline.
type CreatePipelineReq struct {
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Color         string                  `json:"color"`
	Icon          string                  `json:"icon"`
	CliType       string                  `json:"cli_type"`
	Status        int                     `json:"status"`
	Scope         string                  `json:"scope"`
	MemberIds     []int                   `json:"member_ids"`
	DepartmentIds []int                   `json:"department_ids"`
	Steps         []CreatePipelineStepReq `json:"steps"`
}

// UpdatePipelineReq is the request payload for updating basic pipeline information.
type UpdatePipelineReq struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Color         string `json:"color"`
	Icon          string `json:"icon"`
	Status        int    `json:"status"`
	Scope         string `json:"scope"`
	MemberIds     []int  `json:"member_ids"`
	DepartmentIds []int  `json:"department_ids"`
}

// UpdatePipelineWorkflowReq is the request payload for updating the workflow.
type UpdatePipelineWorkflowReq struct {
	ExpectedWorkflowVersion int                     `json:"expected_workflow_version"`
	CliType                 string                  `json:"cli_type"`
	Steps                   []CreatePipelineStepReq `json:"steps"`
}

// RegisterDeviceReq is the request payload for device registration.
// Shared by all clients; the bootstrap key is no longer required.
type RegisterDeviceReq struct {
	DeviceUUID      string `json:"device_uuid"`
	DeviceName      string `json:"device_name"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	ClientVersion   string `json:"client_version"`
	ProtocolVersion int    `json:"protocol_version"`
}

// RequestSyncReq is the request payload for requesting a sync.
// Deprecated: WebSocket sync requests are removed; clients push results incrementally via REST.
type RequestSyncReq struct {
	UserId       int    `json:"user_id"`
	DeviceUUID   string `json:"device_uuid"`
	WorkItemType string `json:"work_item_type"`
	WorkItemId   int    `json:"work_item_id"`
}

// PipelineTaskSnapshot is the incremental pipeline task projection pushed by the client via REST.
// Steps and conversations are incremental payloads upserted by the cloud; deleted lists are
// applied before upserts so a removal always wins over a stale re-add.
type PipelineTaskSnapshot struct {
	LocalTaskUUID      string                       `json:"local_task_uuid"`
	ExecutionIndex     int                          `json:"execution_index"`
	LocalRevision      int64                        `json:"local_revision"`
	ProjectionHash     string                       `json:"projection_hash"`
	WorkItem           PipelineTaskWorkItemSnapshot `json:"work_item"`
	Pipeline           PipelineTaskPipelineSnapshot `json:"pipeline"`
	TaskStatus         string                       `json:"task_status"`
	ExecutionStatus    string                       `json:"execution_status"`
	CurrentStepKey     string                       `json:"current_step_key"`
	ExecutionSummary   string                       `json:"execution_summary"`
	StartedAt          *string                      `json:"started_at"`
	FinishedAt         *string                      `json:"finished_at"`
	LastLocalUpdatedAt string                       `json:"last_local_updated_at"`
	Usage              PipelineTaskUsageSnapshot    `json:"usage"`
	Steps              []PipelineTaskStepSnapshot   `json:"steps"`
	DeletedStepKeys    []string                     `json:"deleted_step_keys"`
	DeviceUUID         string                       `json:"-"`
}

// PipelineTaskWorkItemSnapshot is the snapshot of the work item linked to the task.
type PipelineTaskWorkItemSnapshot struct {
	Type        string `json:"type"`
	ID          int    `json:"id"`
	WorkspaceID int    `json:"workspace_id"`
	Title       string `json:"title"`
}

// PipelineTaskPipelineSnapshot is the snapshot of the pipeline used by the task.
type PipelineTaskPipelineSnapshot struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Color           string `json:"color"`
	CliType         string `json:"cli_type"`
	WorkflowVersion int    `json:"workflow_version"`
}

// PipelineTaskUsageSnapshot holds token and conversation-round statistics.
type PipelineTaskUsageSnapshot struct {
	InputTokens        int64 `json:"input_tokens"`
	OutputTokens       int64 `json:"output_tokens"`
	TotalTokens        int64 `json:"total_tokens"`
	ConversationRounds int   `json:"conversation_rounds"`
}

// PipelineTaskStepSnapshot is the client-side task step projection.
type PipelineTaskStepSnapshot struct {
	StepKey       string                    `json:"step_key"`
	StepName      string                    `json:"step_name"`
	SortOrder     int                       `json:"sort_order"`
	Status        string                    `json:"status"`
	PromptSummary string                    `json:"prompt_summary"`
	OutputSummary string                    `json:"output_summary"`
	ErrorSummary  string                    `json:"error_summary"`
	Usage         PipelineTaskUsageSnapshot `json:"usage"`
	StartedAt     *string                   `json:"started_at"`
	FinishedAt    *string                   `json:"finished_at"`
	DurationMs    int64                     `json:"duration_ms"`
}

// PipelineTaskMessageSnapshot is a single AI-visible message uploaded by the client.
type PipelineTaskMessageSnapshot struct {
	Sequence  int    `json:"sequence"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// PipelineTaskProcessEventSnapshot is one execution-process event uploaded with a round.
// Types match the client event log: thinking, message, tool_call, tool_result, permission_request.
type PipelineTaskProcessEventSnapshot struct {
	Sequence  int    `json:"sequence"`
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// PipelineTaskConversationRoundSnapshot is one user question and AI reply within a conversation.
type PipelineTaskConversationRoundSnapshot struct {
	RoundNo           int                                `json:"round_no"`
	SessionUUID       string                             `json:"session_uuid"`
	Status            string                             `json:"status"`
	UserMessage       string                             `json:"user_message"`
	AssistantMessages []PipelineTaskMessageSnapshot      `json:"assistant_messages"`
	Events            []PipelineTaskProcessEventSnapshot `json:"events,omitempty"`
	ErrorSummary      string                             `json:"error_summary"`
	Usage             PipelineTaskUsageSnapshot          `json:"usage"`
	StartedAt         *string                            `json:"started_at"`
	FinishedAt        *string                            `json:"finished_at"`
	DurationMs        int64                              `json:"duration_ms"`
}

// PipelineTaskConversationContentSnapshot holds the full conversation body stored in the cloud JSONB field.
type PipelineTaskConversationContentSnapshot struct {
	RoundCount int                                     `json:"round_count"`
	Rounds     []PipelineTaskConversationRoundSnapshot `json:"rounds"`
}

// PipelineTaskConversationSnapshot is the conversation projection aggregated by the client per conversation_uuid.
type PipelineTaskConversationSnapshot struct {
	ConversationUUID string                                  `json:"conversation_uuid"`
	StepKey          string                                  `json:"step_key"`
	Status           string                                  `json:"status"`
	CliType          string                                  `json:"cli_type"`
	Content          PipelineTaskConversationContentSnapshot `json:"conversation"`
	ErrorSummary     string                                  `json:"error_summary"`
	Usage            PipelineTaskUsageSnapshot               `json:"usage"`
	StartedAt        *string                                 `json:"started_at"`
	FinishedAt       *string                                 `json:"finished_at"`
	DurationMs       int64                                   `json:"duration_ms"`
}

// PipelineTaskStats holds pipeline task statistics.
type PipelineTaskStats struct {
	All       int `json:"all"`
	Today     int `json:"today"`
	Yesterday int `json:"yesterday"`
	Week      int `json:"week"`
}

// PipelineTaskFilterMember is a member used to filter pipeline tasks.
type PipelineTaskFilterMember struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// PipelineListResp is the pipeline list response.
type PipelineListResp struct {
	List  []PipelineListItem `json:"list"`
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}

// PipelineTaskListResp is the pipeline task list response.
type PipelineTaskListResp struct {
	List    []map[string]interface{}   `json:"list"`
	Total   int                        `json:"total"`
	Stats   PipelineTaskStats          `json:"stats"`
	Members []PipelineTaskFilterMember `json:"members"`
	Page    int                        `json:"page"`
	Size    int                        `json:"size"`
}

// RegisterDeviceResp is the device registration response.
type RegisterDeviceResp struct {
	DeviceUUID        string `json:"device_uuid"`
	Credential        string `json:"credential"`
	CredentialVersion int    `json:"credential_version"`
	ProtocolVersion   int    `json:"protocol_version"`
}

// RequestSyncResp is the sync request response.
// Deprecated: WebSocket sync requests are removed; clients push results incrementally via REST.
type RequestSyncResp struct {
	RequestID      string `json:"request_id"`
	Delivered      int    `json:"delivered"`
	OfflineTargets int    `json:"offline_targets"`
}

// PipelineTaskSyncResp is the response for an incremental task sync push.
type PipelineTaskSyncResp struct {
	CloudTaskID    int64  `json:"cloud_task_id"`
	LocalRevision  int64  `json:"local_revision"`
	ProjectionHash string `json:"projection_hash"`
	LastUploadedAt int64  `json:"last_uploaded_at"`
	Result         string `json:"result"`
}

// PipelineTaskSessionSync is the step-level session projection pushed by the client's
// asynchronous session sync service. One row per step: the client aggregates the step's
// full conversation JSON and pushes it as a whole; the cloud upserts by step_key and a
// stale out-of-order push (smaller client_updated_at) never overwrites newer data.
type PipelineTaskSessionSync struct {
	LocalTaskUUID      string                                  `json:"local_task_uuid"`
	ExecutionIndex     int                                     `json:"execution_index"`
	StepKey            string                                  `json:"step_key"`
	StepName           string                                  `json:"step_name"`
	Status             string                                  `json:"status"`
	CliType            string                                  `json:"cli_type"`
	ModelName          string                                  `json:"model_name"`
	PromptSnapshot     string                                  `json:"prompt_snapshot"`
	ResultStatus       string                                  `json:"result_status"`
	FinalResult        string                                  `json:"final_result"`
	ExitCode           int                                     `json:"exit_code"`
	ErrorCode          string                                  `json:"error_code"`
	ErrorMessage       string                                  `json:"error_message"`
	InputTokens        int64                                   `json:"input_tokens"`
	OutputTokens       int64                                   `json:"output_tokens"`
	TotalTokens        int64                                   `json:"total_tokens"`
	ConversationRounds int                                     `json:"conversation_rounds"`
	Conversation       PipelineTaskConversationContentSnapshot `json:"conversation"`
	ToolCallCount      int                                     `json:"tool_call_count"`
	FilesChanged       int                                     `json:"files_changed"`
	LinesAdded         int                                     `json:"lines_added"`
	LinesDeleted       int                                     `json:"lines_deleted"`
	StartedAt          int64                                   `json:"started_at"`
	FinishedAt         int64                                   `json:"finished_at"`
	DurationMs         int64                                   `json:"duration_ms"`
	CreatedAt          int64                                   `json:"created_at"`
	ClientUpdatedAt    int64                                   `json:"client_updated_at"`
}

// PipelineTaskSessionSyncResp is the response for a session record sync push.
type PipelineTaskSessionSyncResp struct {
	CloudTaskID    int64  `json:"cloud_task_id"`
	StepKey        string `json:"step_key"`
	LastUploadedAt int64  `json:"last_uploaded_at"`
	Result         string `json:"result"`
}

// ======================== Work-item agent tab (requirement detail) ========================

// AgentTaskSummary is one client execution record of a work item (one pipeline_tasks row).
type AgentTaskSummary struct {
	TaskID             int64  `json:"task_id"`
	TaskStatus         string `json:"task_status"`
	ExecutionStatus    string `json:"execution_status"`
	PipelineName       string `json:"pipeline_name"`
	CurrentStepName    string `json:"current_step_name"`
	ConversationRounds int    `json:"conversation_rounds"`
	TotalTokens        int64  `json:"total_tokens"`
	StartedAt          int64  `json:"started_at"`
	FinishedAt         int64  `json:"finished_at"`
	LastUploadedAt     int64  `json:"last_uploaded_at"`
}

// AgentTaskExecutor groups one user's execution records of a work item.
type AgentTaskExecutor struct {
	UserId   int                `json:"user_id"`
	UserName string             `json:"user_name"`
	Avatar   string             `json:"avatar"`
	Tasks    []AgentTaskSummary `json:"tasks"`
}
