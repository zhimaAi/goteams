package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/pressly/goose/v3"
	"github.com/spf13/cast"
)

func init() {
	goose.AddNamedMigrationContext("20261027094054_convert_issue_workspaces_to_agile.go", upConvertIssueWorkspacesToAgile, downConvertIssueWorkspacesToAgile)
}

func upConvertIssueWorkspacesToAgile(ctx context.Context, tx *sql.Tx) error {
	// 本次存量转换仅在线上执行；测试环境只登记迁移版本，不修改项目数据。
	if define.IsDev {
		return nil
	}
	// 模板账号按部署配置识别，保留其项目类型供后续创建项目继承。
	templateAdminID := cast.ToInt(define.Config.Template[`workspace_template_id`])
	const query = `UPDATE ` + define.TableWorkspaces + `
        SET workspace_type = 'agile', updated_at = $1
        WHERE workspace_type = 'issue' AND admin_id <> $2`
	if _, err := tx.ExecContext(ctx, query, time.Now().Unix(), templateAdminID); err != nil {
		return fmt.Errorf("convert issue workspaces to agile: %w", err)
	}
	return nil
}

func downConvertIssueWorkspacesToAgile(_ context.Context, _ *sql.Tx) error {
	// 无法区分原敏捷项目与本次转换的项目，回滚保留类型，避免误改原敏捷项目。
	return nil
}
