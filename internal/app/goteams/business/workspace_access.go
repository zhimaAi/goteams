package business

import (
	"errors"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	ErrWorkspaceAccessPending       = errors.New(`workspace access request pending`)
	ErrWorkspaceAccessAlreadyMember = errors.New(`workspace member already joined`)
)

// WorkspaceAccessModel is the limited project information visible to a tenant user.
type WorkspaceAccessModel struct {
	AdminID       int
	WorkspaceID   int
	UserID        int
	WorkspaceName string
	IsMember      bool
	RequestStatus string
}

// GetWorkspaceAccess returns the project name, membership and current request state.
func GetWorkspaceAccess(model WorkspaceAccessModel) (WorkspaceAccessModel, error) {
	query := `SELECT w.name,
		EXISTS (SELECT 1 FROM ` + define.TableWorkspaceMembers + ` wm WHERE wm.admin_id = w.admin_id AND wm.workspace_id = w.id AND wm.user_id = $3) AS is_member,
		COALESCE(ar.status, '') AS request_status
		FROM ` + define.TableWorkspaces + ` w LEFT JOIN ` + define.TableWorkspaceAccessRequests + ` ar
		ON ar.admin_id = w.admin_id AND ar.workspace_id = w.id AND ar.user_id = $3
		WHERE w.admin_id = $1 AND w.id = $2`
	rows, err := msql.RawValues(define.Postgres, query, nil, model.AdminID, model.WorkspaceID, model.UserID)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return WorkspaceAccessModel{}, err
	}
	if len(rows) == 0 {
		return WorkspaceAccessModel{}, ErrWorkspaceNotFound
	}
	model.WorkspaceName = rows[0][`name`]
	model.IsMember = cast.ToBool(rows[0][`is_member`])
	model.RequestStatus = rows[0][`request_status`]
	return model, nil
}

// RequestWorkspaceAccess stores one pending request and notifies project member managers.
func RequestWorkspaceAccess(model WorkspaceAccessModel, reason string) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const lockQuery = `SELECT id FROM ` + define.TableWorkspaces + ` WHERE admin_id = $1 AND id = $2 FOR UPDATE`
	rows, err := msql.RawValues(define.Postgres, lockQuery, tx, model.AdminID, model.WorkspaceID)
	if err != nil {
		logs.Error("sql:%s, err:%s", lockQuery, err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrWorkspaceNotFound
	}
	const memberQuery = `SELECT 1 FROM ` + define.TableWorkspaceMembers + ` WHERE admin_id = $1 AND workspace_id = $2 AND user_id = $3`
	memberRows, err := msql.RawValues(define.Postgres, memberQuery, tx, model.AdminID, model.WorkspaceID, model.UserID)
	if err != nil {
		logs.Error("sql:%s, err:%s", memberQuery, err.Error())
		return err
	}
	if len(memberRows) > 0 {
		return ErrWorkspaceAccessAlreadyMember
	}
	now := tool.Time2Int()
	const insertQuery = `INSERT INTO ` + define.TableWorkspaceAccessRequests + `
		(admin_id, workspace_id, user_id, reason, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'pending', $5, $5)
		ON CONFLICT (admin_id, workspace_id, user_id) DO UPDATE
		SET reason = EXCLUDED.reason, status = 'pending', created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at
		WHERE ` + define.TableWorkspaceAccessRequests + `.status = 'joined'`
	result, err := msql.RawExec(define.Postgres, insertQuery, tx,
		model.AdminID, model.WorkspaceID, model.UserID, reason, now)
	if err != nil {
		logs.Error("sql:%s, err:%s", insertQuery, err.Error())
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrWorkspaceAccessPending
	}
	const recipientsQuery = `SELECT DISTINCT wm.user_id
		FROM ` + define.TableWorkspaceMembers + ` wm JOIN ` + define.TableWorkspaces + ` w ON w.admin_id = wm.admin_id AND w.id = wm.workspace_id
		JOIN ` + define.TableUsers + ` u ON u.admin_id = wm.admin_id AND u.id = wm.user_id
		LEFT JOIN ` + define.TableRolePermissions + ` rp ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id AND rp.permission_key = 'project_member:add'
		WHERE wm.admin_id = $1 AND wm.workspace_id = $2 AND u.status = 1 AND u.deleted_at = 0
		AND (wm.user_id = COALESCE(NULLIF(w.owner_user_id, 0), w.created_by) OR rp.id IS NOT NULL)`
	recipients, err := msql.RawValues(define.Postgres, recipientsQuery, tx, model.AdminID, model.WorkspaceID)
	if err != nil {
		logs.Error("sql:%s, err:%s", recipientsQuery, err.Error())
		return err
	}
	for _, recipient := range recipients {
		if err = CreateWorkspaceNotificationTx(tx, notificationEvent{
			AdminID: model.AdminID, RecipientID: cast.ToInt(recipient[`user_id`]), WorkspaceID: model.WorkspaceID,
			EventType: define.NotificationEventWorkspaceAccessRequested, ActorID: model.UserID, Content: reason,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}
