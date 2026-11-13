package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("20261022150626_backfill_requirement_template_snapshot.go", upBackfillRequirementTemplateSnapshot, downBackfillRequirementTemplateSnapshot)
}

func upBackfillRequirementTemplateSnapshot(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE requirements r
        SET template_id = ir.template_id, template_name = ir.template_name
        FROM workspace_issue_requirements ir
        WHERE r.admin_id = ir.admin_id
            AND r.workspace_id = ir.workspace_id
            AND r.id = ir.requirement_id
            AND r.type IN ('requirement', 'defect')
            AND r.template_id = 0 AND r.template_name = ''
            AND ir.template_id > 0`)
	if err != nil {
		return fmt.Errorf("backfill requirement template snapshot: %w", err)
	}
	return nil
}

func downBackfillRequirementTemplateSnapshot(_ context.Context, _ *sql.Tx) error {
	// 回填后无法区分历史快照和新创建的快照，字段由结构迁移统一撤销。
	return nil
}
