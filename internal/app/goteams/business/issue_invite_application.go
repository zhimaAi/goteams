// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// Issue invitation domain errors.
var (
	// ErrIssueInviteNotImplemented marks an issue-invitation function whose TODO body is pending.
	ErrIssueInviteNotImplemented = errors.New(`issue invite not implemented`)
	// ErrIssueInviteAppDisabled indicates the issue application is not enabled in the workspace.
	ErrIssueInviteAppDisabled = errors.New("issue invite application disabled")
	// ErrIssueInviteInvalid indicates the public invitation is missing, stopped or unknown.
	ErrIssueInviteInvalid = errors.New("issue invite invalid")
	// ErrIssueInviteTemplateNotFound indicates an invitation template is missing in the project scope.
	ErrIssueInviteTemplateNotFound = errors.New("issue invite template not found")
	// ErrIssueInviteTemplateDisabled indicates a copied template is disabled.
	ErrIssueInviteTemplateDisabled = errors.New("issue invite template disabled")
	// ErrIssueInviteFieldInvalid indicates a submitted public field failed validation.
	ErrIssueInviteFieldInvalid = errors.New("issue invite field invalid")
	// ErrIssueInviteOrderInvalid indicates the copied-template order set is incomplete.
	ErrIssueInviteOrderInvalid = errors.New("issue invite template order invalid")
	// ErrIssueInviteKeyGenerate indicates a unique public key could not be generated.
	ErrIssueInviteKeyGenerate = errors.New("issue invite key generation failed")
	// ErrIssueInviteRateLimited indicates the submitter exceeded the per-minute submission limit.
	ErrIssueInviteRateLimited = errors.New("issue invite rate limited")
	// ErrIssueInviteRecallStatusInvalid 表示开了用户撤回但撤回状态没落到项目的真实状态上。
	ErrIssueInviteRecallStatusInvalid = errors.New("issue invite recall status invalid")
	// ErrIssueInviteRecallDisabled 表示该模板没开用户撤回，用户端不能撤回。
	ErrIssueInviteRecallDisabled = errors.New("issue invite recall disabled")
	// ErrIssueInviteRecallNotAllowed 表示当前状态不允许撤回。
	ErrIssueInviteRecallNotAllowed = errors.New("issue invite recall not allowed")
	// ErrIssueInviteResubmitNotAllowed 表示当前状态不允许修改并重新提交。
	ErrIssueInviteResubmitNotAllowed = errors.New("issue invite resubmit not allowed")
	// ErrIssueInviteUrgeTooSoon 表示还没到可催单的时间。
	ErrIssueInviteUrgeTooSoon = errors.New("issue invite urge too soon")
	// ErrIssueInviteUrgeNotAllowed 表示工单已撤回或已结束，不再接受催单。
	ErrIssueInviteUrgeNotAllowed = errors.New("issue invite urge not allowed")
	// ErrIssueInviteLinkParamInvalid 表示链接参数名不合法，或者同一模板里两个字段用了同一个参数名。
	ErrIssueInviteLinkParamInvalid = errors.New("issue invite link param invalid")
)

// generateIssueInviteKey produces a 16-character URL-safe key (letters, digits, '-' and '_').
// Twelve random bytes encode to exactly sixteen base64url characters, matching the column limit.
func generateIssueInviteKey() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		logs.Error("generate issue invite key failed, err:%s", err.Error())
		return ``, err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// isIssueInviteKeyConflict reports whether the error is a unique violation on the invite key.
func isIssueInviteKeyConflict(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == `uk_workspace_issue_invites_key`
}

// loadIssueInviteConfigTx returns the invite key (empty when none) inside the given transaction.
func loadIssueInviteConfigTx(tx *sql.Tx, adminID, workspaceID int) (string, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT invite_key FROM `+define.TableWorkspaceIssueInvites+`
		WHERE admin_id = $1 AND workspace_id = $2`, tx, adminID, workspaceID)
	if err != nil {
		logs.Error("load issue invite config failed, err:%s", err.Error())
		return ``, err
	}
	if len(rows) == 0 {
		return ``, nil
	}
	return rows[0][`invite_key`], nil
}

// UpdateIssueWorkspaceApplication atomically updates the issue app and creates its first invite config.
func UpdateIssueWorkspaceApplication(model WorkspaceApplicationModel) (WorkspaceApplicationModel, error) {
	enabled := 0
	if model.Enabled == 1 {
		enabled = 1
	}
	now := int64(tool.Time2Int())
	enabledAt := model.EnabledAt
	if enabled == 1 && enabledAt == 0 {
		enabledAt = now
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin issue application transaction failed, err:%s", err.Error())
		return WorkspaceApplicationModel{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// First enable must create the single general invitation config in the same transaction.
	if enabled == 1 {
		if _, keyErr := ensureIssueInviteConfigTx(tx, model.AdminId, model.WorkspaceId, model.EnabledBy, now); keyErr != nil {
			return WorkspaceApplicationModel{}, keyErr
		}
	}

	if _, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableWorkspaceApplications+`
		(admin_id, workspace_id, app_key, enabled, enabled_by, enabled_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (admin_id, workspace_id, app_key) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			enabled_by = EXCLUDED.enabled_by,
			enabled_at = CASE WHEN EXCLUDED.enabled = 1 THEN EXCLUDED.enabled_at ELSE `+define.TableWorkspaceApplications+`.enabled_at END,
			updated_at = EXCLUDED.updated_at`, tx,
		model.AdminId, model.WorkspaceId, model.AppKey, enabled, model.EnabledBy, enabledAt, now); err != nil {
		logs.Error("upsert issue application failed, err:%s", err.Error())
		return WorkspaceApplicationModel{}, err
	}

	if err = tx.Commit(); err != nil {
		logs.Error("commit issue application transaction failed, err:%s", err.Error())
		return WorkspaceApplicationModel{}, err
	}

	if err = InvalidateIssueInviteProjectCache(model.AdminId, model.WorkspaceId); err != nil {
		logs.Error("invalidate issue invite cache after app update failed, err:%s", err.Error())
	}

	return WorkspaceApplicationModel{
		AdminId: model.AdminId, WorkspaceId: model.WorkspaceId, AppKey: model.AppKey,
		Enabled: enabled, EnabledBy: model.EnabledBy, EnabledAt: enabledAt, UpdatedAt: now,
	}, nil
}

// ensureIssueInviteConfigTx creates or reloads the general invitation key inside the transaction.
// The (admin_id, workspace_id) conflict keeps the existing key stable; the global key conflict retries.
func ensureIssueInviteConfigTx(tx *sql.Tx, adminID, workspaceID, createdBy int, now int64) (string, error) {
	existing, err := loadIssueInviteConfigTx(tx, adminID, workspaceID)
	if err != nil {
		return ``, err
	}
	if existing != `` {
		return existing, nil
	}
	for i := 0; i < define.IssueInviteKeyRetryMaxCount; i++ {
		candidate, genErr := generateIssueInviteKey()
		if genErr != nil {
			return ``, genErr
		}
		rows, insertErr := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkspaceIssueInvites+`
			(admin_id, workspace_id, invite_key, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (admin_id, workspace_id) DO UPDATE SET updated_at = EXCLUDED.updated_at
			RETURNING invite_key`, tx, adminID, workspaceID, candidate, createdBy, now)
		if insertErr != nil {
			if isIssueInviteKeyConflict(insertErr) {
				continue
			}
			logs.Error("insert issue invite config failed, err:%s", insertErr.Error())
			return ``, insertErr
		}
		if len(rows) == 0 || rows[0][`invite_key`] == `` {
			return ``, ErrIssueInviteKeyGenerate
		}
		return rows[0][`invite_key`], nil
	}
	return ``, ErrIssueInviteKeyGenerate
}

// GetIssueInviteConfig returns the uncached authenticated project invitation configuration.
func GetIssueInviteConfig(adminID, workspaceID int) (define.IssueInviteConfigModel, error) {
	m := msql.Model(define.TableWorkspaceIssueInvites, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminID)},
		{`workspace_id`, cast.ToString(workspaceID)},
	}).Field(`admin_id, workspace_id, invite_key, enabled, guide_title, guide_description, theme_image, created_by, created_at, updated_at`).Find()
	if err != nil {
		logs.Error("get issue invite config failed, sql:%s, err:%s", m.GetLastSql(), err.Error())
		return define.IssueInviteConfigModel{}, err
	}
	config := define.IssueInviteConfigModel{
		AdminID: adminID, WorkspaceID: workspaceID,
	}
	if len(row) > 0 {
		config.Enabled = cast.ToInt(row[`enabled`])
		config.InviteKey = row[`invite_key`]
		config.GuideTitle = row[`guide_title`]
		config.GuideDescription = row[`guide_description`]
		config.ThemeImage = row[`theme_image`]
		config.CreatedBy = cast.ToInt(row[`created_by`])
		config.CreatedAt = cast.ToInt64(row[`created_at`])
		config.UpdatedAt = cast.ToInt64(row[`updated_at`])
	}
	return config, nil
}

// UpdateIssueInviteConfig updates the invitation guide configuration and invalidates public caches.
func UpdateIssueInviteConfig(model define.IssueInviteConfigUpdateModel) (define.IssueInviteConfigModel, error) {
	now := int64(tool.Time2Int())
	m := msql.Model(define.TableWorkspaceIssueInvites, define.Postgres)
	if _, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminID)},
		{`workspace_id`, cast.ToString(model.WorkspaceID)},
	}).Update(msql.Datas{
		`guide_title`:       model.GuideTitle,
		`guide_description`: model.GuideDescription,
		`theme_image`:       model.ThemeImage,
		`enabled`:           model.Enabled,
		`updated_at`:        now,
	}); err != nil {
		logs.Error("update issue invite config failed, sql:%s, err:%s", m.GetLastSql(), err.Error())
		return define.IssueInviteConfigModel{}, err
	}
	if err := InvalidateIssueInviteProjectCache(model.AdminID, model.WorkspaceID); err != nil {
		logs.Error("invalidate issue invite cache after config update failed, err:%s", err.Error())
	}
	return GetIssueInviteConfig(model.AdminID, model.WorkspaceID)
}
