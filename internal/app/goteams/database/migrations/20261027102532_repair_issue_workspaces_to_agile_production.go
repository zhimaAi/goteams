package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/pressly/goose/v3"
	"github.com/spf13/cast"
)

func init() {
	goose.AddNamedMigrationContext("20261027102532_repair_issue_workspaces_to_agile_production.go", upRepairIssueWorkspacesToAgileProduction, downRepairIssueWorkspacesToAgileProduction)
}

func upRepairIssueWorkspacesToAgileProduction(ctx context.Context, tx *sql.Tx) error {
	// 原迁移已覆盖显式生产模式；这里只补偿 IsDev 默认开启、实际加载生产配置的容器。
	if !define.IsDev || define.IsLoc {
		return nil
	}
	// 与 initConfig 的配置选择保持一致：DEV 的第二段为 pro 时读取 config_pro.ini。
	parts := strings.Split(os.Getenv("DEV"), "-")
	if len(parts) < 2 || parts[1] != "pro" {
		return nil
	}
	templateAdminID := cast.ToInt(define.Config.Template[`workspace_template_id`])
	const query = `UPDATE ` + define.TableWorkspaces + `
        SET workspace_type = 'agile', updated_at = $1
        WHERE workspace_type = 'issue' AND admin_id <> $2`
	if _, err := tx.ExecContext(ctx, query, time.Now().Unix(), templateAdminID); err != nil {
		return fmt.Errorf("repair issue workspaces to agile production sql:%s: %w", query, err)
	}
	return nil
}

func downRepairIssueWorkspacesToAgileProduction(_ context.Context, _ *sql.Tx) error {
	// 无法区分原敏捷项目与本次转换项目，回滚保留类型，避免误改原敏捷项目。
	return nil
}
