package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/pressly/goose/v3"
	"strconv"
	"strings"
)

func init() {
	goose.AddNamedMigrationContext("20260907181805_backfill_dashboard_completion.go", upDashboardCompletion, downDashboardCompletion)
}

type completionScope struct {
	admin, workspace    int
	requirement, defect string
	updated             int64
}
type completionStatus struct {
	id      int
	name    string
	updated int64
}
type completionLegacyItem struct{ id, status int }
type completionHistory struct {
	field, old, next string
	at               int64
}

func upDashboardCompletion(ctx context.Context, tx *sql.Tx) error {
	return backfillDashboardCompletion(ctx, tx, true)
}

// backfillDashboardCompletion restores the first completion date from status history.
// restrictBySettingsUpdate keeps the original, overly conservative lower bound:
// saving any workspace setting refreshes workspaces_settings.updated_at, which then
// discards every earlier status change and leaves the completion trend empty.
func backfillDashboardCompletion(ctx context.Context, tx *sql.Tx, restrictBySettingsUpdate bool) error {
	rows, err := tx.QueryContext(ctx, `SELECT admin_id,workspace_id,requirement_final_status_ids,defect_final_status_ids,updated_at FROM workspaces_settings ORDER BY admin_id,workspace_id`)
	if err != nil {
		return err
	}
	scopes := []completionScope{}
	for rows.Next() {
		var s completionScope
		if err = rows.Scan(&s.admin, &s.workspace, &s.requirement, &s.defect, &s.updated); err != nil {
			rows.Close()
			return err
		}
		scopes = append(scopes, s)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, scope := range scopes {
		for _, typ := range []string{"requirement", "defect"} {
			raw := scope.requirement
			if typ == "defect" {
				raw = scope.defect
			}
			finals := map[int]bool{}
			for _, part := range strings.Split(raw, ",") {
				if part == "" {
					continue
				}
				id, parseErr := strconv.Atoi(part)
				if parseErr != nil {
					return parseErr
				}
				finals[id] = true
			}
			if len(finals) == 0 {
				continue
			}
			statusRows, err := tx.QueryContext(ctx, `SELECT id,name,updated_at FROM workspace_statuses WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3`, scope.admin, scope.workspace, typ)
			if err != nil {
				return err
			}
			byId := map[int]completionStatus{}
			byName := map[string][]completionStatus{}
			for statusRows.Next() {
				var s completionStatus
				if err = statusRows.Scan(&s.id, &s.name, &s.updated); err != nil {
					statusRows.Close()
					return err
				}
				byId[s.id] = s
				byName[s.name] = append(byName[s.name], s)
			}
			if err = statusRows.Err(); err != nil {
				statusRows.Close()
				return err
			}
			if err = statusRows.Close(); err != nil {
				return err
			}
			cursor := 0
			for {
				itemRows, err := tx.QueryContext(ctx, `SELECT id,status_id FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND type=$3 AND id>$4 AND completed_at=0 AND deleted_at=0 ORDER BY id LIMIT 200`, scope.admin, scope.workspace, typ, cursor)
				if err != nil {
					return err
				}
				items := []completionLegacyItem{}
				for itemRows.Next() {
					var i completionLegacyItem
					if err = itemRows.Scan(&i.id, &i.status); err != nil {
						itemRows.Close()
						return err
					}
					items = append(items, i)
				}
				if err = itemRows.Err(); err != nil {
					itemRows.Close()
					return err
				}
				if err = itemRows.Close(); err != nil {
					return err
				}
				if len(items) == 0 {
					break
				}
				cursor = items[len(items)-1].id
				for _, item := range items {
					if !finals[item.status] {
						continue
					}
					current, exists := byId[item.status]
					if !exists || len(byName[current.name]) != 1 {
						continue
					}
					historyRows, err := tx.QueryContext(ctx, `SELECT field_name,old_value,new_value,created_at FROM requirement_histories WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND field_name IN ('status','workspace_id','type') ORDER BY id DESC`, scope.admin, scope.workspace, item.id)
					if err != nil {
						return err
					}
					completed := int64(0)
					expected := current.name
					for historyRows.Next() {
						var h completionHistory
						if err = historyRows.Scan(&h.field, &h.old, &h.next, &h.at); err != nil {
							historyRows.Close()
							return err
						}
						// Renames, transfers, missing links and ambiguous names remain undated.
						if h.field != "status" || h.next != expected || len(byName[h.old]) != 1 || len(byName[h.next]) != 1 {
							break
						}
						if restrictBySettingsUpdate && h.at < scope.updated {
							break
						}
						old, next := byName[h.old][0], byName[h.next][0]
						if old.updated > h.at || next.updated > h.at {
							break
						}
						if !finals[old.id] && finals[next.id] {
							completed = h.at
							break
						}
						if !finals[old.id] || !finals[next.id] {
							break
						}
						expected = h.old
					}
					if err = historyRows.Err(); err != nil {
						historyRows.Close()
						return err
					}
					if err = historyRows.Close(); err != nil {
						return err
					}
					if completed > 0 {
						if _, err = tx.ExecContext(ctx, `UPDATE requirements SET completed_at=$1 WHERE admin_id=$2 AND workspace_id=$3 AND id=$4 AND status_id=$5 AND completed_at=0`, completed, scope.admin, scope.workspace, item.id, item.status); err != nil {
							return fmt.Errorf("backfill completion: %w", err)
						}
					}
				}
			}
		}
	}
	return nil
}

func downDashboardCompletion(context.Context, *sql.Tx) error { return nil }
