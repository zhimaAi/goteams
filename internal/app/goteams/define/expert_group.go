// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// ======================== Expert group request/response structs ========================

// Expert group scope values reuse the pipeline scope semantics (all / members / departments),
// so no new scope constants are introduced; see PipelineScopeAll / Members / Departments.

// Expert group member role values within one group's orchestration.
const (
	// ExpertGroupRoleLeader is the orchestrating lead expert (at most one per group).
	ExpertGroupRoleLeader = `leader`
	// ExpertGroupRoleMember is a normal expert member.
	ExpertGroupRoleMember = `member`
)

// ExpertGroupListItem is one expert group card in the management list.
type ExpertGroupListItem struct {
	ID              int                    `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Avatar          string                 `json:"avatar"`
	Status          int                    `json:"status"`
	Scope           string                 `json:"scope"`
	MemberCount     int                    `json:"member_count"`
	DepartmentCount int                    `json:"department_count"`
	AgentCount      int                    `json:"agent_count"`
	Agents          []ExpertGroupAgentInfo `json:"agents"`
	ScopeMembers    []PipelineScopeMember  `json:"scope_members"`
	ScopeTotal      int                    `json:"scope_total"`
	CanManage       bool                   `json:"can_manage"`
	CanUse          bool                   `json:"can_use"`
	UpdatedAt       int                    `json:"updated_at"`
}

// ExpertGroupAgentInfo is one orchestrated expert (agent) within a group.
type ExpertGroupAgentInfo struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Avatar    string `json:"avatar"`
	Prompt    string `json:"prompt"`
	SortOrder int    `json:"sort_order"`
}

// ExpertGroupDetail is the detailed expert group information.
type ExpertGroupDetail struct {
	ID            int                      `json:"id"`
	AdminId       int                      `json:"-"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	Avatar        string                   `json:"avatar"`
	Status        int                      `json:"status"`
	Scope         string                   `json:"scope"`
	DepartmentIds []int                    `json:"department_ids"`
	Departments   []PipelineDepartmentInfo `json:"departments"`
	Members       []PipelineMemberInfo     `json:"members"`
	Agents        []ExpertGroupAgentInfo   `json:"agents"`
	CreatedBy     int                      `json:"-"`
	UpdatedAt     int                      `json:"updated_at"`
}

// CreateExpertGroupAgentReq is the request payload for one orchestrated expert.
type CreateExpertGroupAgentReq struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	Avatar    string `json:"avatar"`
	Prompt    string `json:"prompt"`
	SortOrder int    `json:"sort_order"`
}

// CreateExpertGroupReq is the request payload for creating an expert group.
type CreateExpertGroupReq struct {
	Name          string                      `json:"name"`
	Description   string                      `json:"description"`
	Avatar        string                      `json:"avatar"`
	Status        int                         `json:"status"`
	Scope         string                      `json:"scope"`
	MemberIds     []int                       `json:"member_ids"`
	DepartmentIds []int                       `json:"department_ids"`
	Agents        []CreateExpertGroupAgentReq `json:"agents"`
}

// UpdateExpertGroupReq is the request payload for updating an expert group.
type UpdateExpertGroupReq struct {
	Name          string                      `json:"name"`
	Description   string                      `json:"description"`
	Avatar        string                      `json:"avatar"`
	Status        int                         `json:"status"`
	Scope         string                      `json:"scope"`
	MemberIds     []int                       `json:"member_ids"`
	DepartmentIds []int                       `json:"department_ids"`
	Agents        []CreateExpertGroupAgentReq `json:"agents"`
}

// ExpertGroupListResp is the expert group list response.
type ExpertGroupListResp struct {
	List  []ExpertGroupListItem `json:"list"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}

// ClientExpertGroupItem is the expert group snapshot pulled by the client.
type ClientExpertGroupItem struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Avatar      string                 `json:"avatar"`
	Scope       string                 `json:"scope"`
	Agents      []ExpertGroupAgentInfo `json:"agents"`
	UpdatedAt   int                    `json:"updated_at"`
}
