// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext("20260914160000_backfill_dashboard_completion_without_settings_bound.go", upDashboardCompletionWithoutSettingsBound, downDashboardCompletionWithoutSettingsBound)
}

// upDashboardCompletionWithoutSettingsBound re-runs the completion backfill without the
// workspaces_settings.updated_at lower bound. Saving any workspace setting refreshes that
// timestamp, so the previous migration discarded every earlier status change and left most
// completed items without a completion date, which emptied the daily completion trend.
func upDashboardCompletionWithoutSettingsBound(ctx context.Context, tx *sql.Tx) error {
	return backfillDashboardCompletion(ctx, tx, false)
}

// downDashboardCompletionWithoutSettingsBound keeps restored dates: the recovered values are
// derived from status history, and clearing them would only reintroduce the missing trend.
func downDashboardCompletionWithoutSettingsBound(context.Context, *sql.Tx) error { return nil }
