// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// UpdateRoleBasicReq carries the parameters for updating a role's basic information (name and description).
type UpdateRoleBasicReq struct {
	AdminID     int    // tenant the role belongs to
	WorkspaceID string // workspace of the role; empty for enterprise-scoped roles
	RoleID      int    // role to update
	Name        string // new role name
	Description string // new role description
}
