package business

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"goteams/internal/app/goteams/define"
)

type warningScanField struct {
	Id        int
	Key, Type string
}
type warningScanItem struct {
	Id        int
	CreatedAt int64
	Fields    map[string]any
}

// 所有 SQL 表达式来自固定白名单；条件字段名不能直接插入查询。
var warningBuiltinSQL = map[string]string{
	"title": "title", "description": "description", "status": "status_id", "priority": "priority", "version": "version_id", "assignee": "assignee_ids", "category": "NULLIF(category_id,0)",
	"planned_start_date": "NULLIF(planned_start_date,0)", "planned_end_date": "NULLIF(planned_end_date,0)", "created_by": "created_by", "created_at": "created_at", "id": "id", "updated_at": "updated_at",
	"estimated_work_hours": "COALESCE(work_hours,0)", "completed_work_hours": "COALESCE(completed_work_hours,0)",
	"remaining_work_hours": "GREATEST(COALESCE(work_hours,0)-COALESCE(completed_work_hours,0),0)", "overtime_work_hours": "GREATEST(COALESCE(completed_work_hours,0)-COALESCE(work_hours,0),0)",
	"work_progress": "CASE WHEN work_hours>0 THEN LEAST(100,ROUND(COALESCE(completed_work_hours,0)*100/work_hours)) ELSE 0 END",
}
var warningBuiltinTypes = map[string]string{"title": "text", "description": "textarea", "status": "select_single", "priority": "select_single", "version": "select_single", "assignee": "user", "category": "select_single", "planned_start_date": "date", "planned_end_date": "date", "created_by": "user", "created_at": "date", "id": "number", "updated_at": "date", "estimated_work_hours": "number", "completed_work_hours": "number", "remaining_work_hours": "number", "overtime_work_hours": "number", "work_progress": "number"}

func loadWarningScanFields(ctx context.Context, tx *sql.Tx, c warningClaim) ([]warningScanField, error) {
	var raw []byte
	var err error
	if c.Config.TemplateId != 0 {
		err = (warningSQL{tx}).QueryRowContext(ctx, "SELECT t.fields_json FROM "+define.TableWorkItemTemplates+" t JOIN "+("(SELECT admin_id,workspace_id,template_id FROM "+define.TableWorkspaceWorkItemTemplates+" UNION SELECT admin_id,workspace_id,template_id FROM "+define.TableWorkspaceIssueItemTemplates+" WHERE enabled=1)")+" a ON a.admin_id=t.admin_id AND a.template_id=t.id JOIN "+define.TableWorkspaces+" w ON w.admin_id=a.admin_id AND w.id=a.workspace_id WHERE t.admin_id=$1 AND a.workspace_id=$2 AND t.id=$3 AND t.type_key=$4 AND w.workspace_type='issue'", c.Rule.AdminId, c.Rule.WorkspaceId, c.Config.TemplateId, c.Config.Type).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWarningInvalid
		}
		if err != nil {
			return nil, err
		}
	}
	if len(raw) == 0 {
		raw = []byte("[]")
	}
	var template []templateFieldJSON
	if err = json.Unmarshal(raw, &template); err != nil {
		return nil, err
	}
	allowed := map[string]bool{"title": true, "description": true}
	customIds := []int{}
	for _, field := range template {
		if !warningTemplateFieldShown(field.Display) {
			continue
		}
		key := field.FieldKey
		if key == "status_id" {
			key = "status"
		}
		allowed[key] = true
		if id := templateCustomFieldID(key); id > 0 {
			customIds = append(customIds, id)
		}
	}
	rows, err := (warningSQL{tx}).QueryContext(ctx, "SELECT field_key FROM "+define.TableWorkItemFieldConfigs+" WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND is_enabled=1", c.Rule.AdminId, c.Rule.WorkspaceId, c.Config.Type)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{"title": true, "description": true}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return nil, err
		}
		if key == "status_id" {
			key = "status"
		}
		enabled[key] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	custom := map[string]warningScanField{}
	if c.Config.TemplateId == 0 {
		customIds = nil
	}
	if len(customIds) > 0 || c.Config.TemplateId == 0 {
		rows, err := (warningSQL{tx}).QueryContext(ctx, "SELECT id,key,field_type FROM "+define.TableCustomFields+" WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND is_enabled=1 AND ($4::int[] IS NULL OR id=ANY($4))", c.Rule.AdminId, c.Rule.WorkspaceId, c.Config.Type, pq.Array(customIds))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var f warningScanField
			if err = rows.Scan(&f.Id, &f.Key, &f.Type); err != nil {
				rows.Close()
				return nil, err
			}
			custom[f.Key] = f
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	fields := []warningScanField{}
	seen := map[string]bool{}
	for _, condition := range c.Config.Conditions {
		if seen[condition.FieldKey] {
			continue
		}
		seen[condition.FieldKey] = true
		f, ok := custom[condition.FieldKey]
		if !ok {
			typ, exists := warningBuiltinTypes[condition.FieldKey]
			if !exists || (c.Config.TemplateId != 0 && !allowed[condition.FieldKey]) || !enabled[condition.FieldKey] {
				return nil, ErrWarningInvalid
			}
			f = warningScanField{Key: condition.FieldKey, Type: typ}
		}
		if !automationOperatorAllowed(warningFieldType(f.Type), condition.Operator) {
			return nil, ErrWarningInvalid
		}
		fields = append(fields, f)
	}
	return fields, nil
}

func scanWarningWindow(ctx context.Context, db *sql.DB, c warningClaim) (int, []int, error) {
	// 同次计数与收件人读取同一个快照；不锁规则行，不把整段扫描放进写事务。
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return 0, nil, err
	}
	defer rollbackWarning(tx)
	fields, err := loadWarningScanFields(ctx, tx, c)
	if err != nil {
		logs.Error("warning field validation failed: admin=%d workspace=%d rule=%d template=%d: %v", c.Rule.AdminId, c.Rule.WorkspaceId, c.Rule.Id, c.Config.TemplateId, err)
		return 0, nil, err
	}
	types := map[string]string{}
	builtin := map[string]bool{}
	customIds := []int{}
	for _, f := range fields {
		types[f.Key] = warningFieldType(f.Type)
		if f.Id > 0 {
			customIds = append(customIds, f.Id)
		} else {
			builtin[f.Key] = true
		}
	}
	if c.Config.hasInternalChannel() {
		builtin["assignee"] = true
	}
	selected := []string{}
	for key := range builtin {
		selected = append(selected, key)
	}
	sort.Strings(selected)
	projections := []string{}
	for _, key := range selected {
		projections = append(projections, "'"+key+"',"+warningBuiltinSQL[key])
	}
	object := "'{}'::jsonb"
	if len(projections) > 0 {
		object = "jsonb_build_object(" + strings.Join(projections, ",") + ")"
	}
	query := "SELECT id,created_at," + object + " FROM " + define.TableRequirements + " WHERE admin_id=$1 AND workspace_id=$2 AND type=$3 AND template_id=$4 AND deleted_at=0 AND created_at>$5 AND created_at<=$6 AND (created_at,id)>($7,$8) ORDER BY created_at,id LIMIT 500"
	now := time.Now().Unix()
	afterTime, afterId := int64(0), 0
	count := 0
	recipients := map[int]bool{}
	for {
		rows, err := (warningSQL{tx}).QueryContext(ctx, query, c.Rule.AdminId, c.Rule.WorkspaceId, c.Config.Type, c.Config.TemplateId, now-int64(c.Config.WindowMinutes)*60, now, afterTime, afterId)
		if err != nil {
			return 0, nil, err
		}
		items := []warningScanItem{}
		ids := []int{}
		for rows.Next() {
			var item warningScanItem
			var raw []byte
			if err = rows.Scan(&item.Id, &item.CreatedAt, &raw); err != nil {
				rows.Close()
				return 0, nil, err
			}
			if err = json.Unmarshal(raw, &item.Fields); err != nil {
				rows.Close()
				return 0, nil, err
			}
			ids = append(ids, item.Id)
			items = append(items, item)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return 0, nil, err
		}
		if len(items) == 0 {
			break
		}
		if len(customIds) > 0 {
			if err = loadWarningCustomValues(ctx, tx, c, fields, customIds, ids, items); err != nil {
				return 0, nil, err
			}
		}
		for _, item := range items {
			if warningMatches(c.Config, types, item.Fields) {
				count++
				if c.Config.hasInternalChannel() {
					for _, id := range automationAssigneeIDs(item.Fields["assignee"]) {
						recipients[id] = true
					}
				} else if count >= c.Config.Threshold {
					return count, []int{}, tx.Commit()
				}
			}
		}
		last := items[len(items)-1]
		afterTime, afterId = last.CreatedAt, last.Id
	}
	ids := []int{}
	for id := range recipients {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return count, ids, tx.Commit()
}

func loadWarningCustomValues(ctx context.Context, tx *sql.Tx, c warningClaim, fields []warningScanField, customIds, ids []int, items []warningScanItem) error {
	byId := map[int]warningScanField{}
	byItem := map[int]map[string]any{}
	for _, f := range fields {
		if f.Id > 0 {
			byId[f.Id] = f
			for _, item := range items {
				item.Fields[f.Key] = warningStoredValue(f.Type, "")
			}
		}
	}
	for _, item := range items {
		byItem[item.Id] = item.Fields
	}
	query := "SELECT v.requirement_id,v.field_id,COALESCE(v.value_text,'') FROM " + define.TableCustomFieldValues + " v JOIN " + define.TableCustomFields + " f ON f.admin_id=v.admin_id AND f.id=v.field_id WHERE v.admin_id=$1 AND f.workspace_id=$2 AND v.requirement_id=ANY($3) AND v.field_id=ANY($4)"
	rows, err := (warningSQL{tx}).QueryContext(ctx, query, c.Rule.AdminId, c.Rule.WorkspaceId, pq.Array(ids), pq.Array(customIds))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var itemId, fieldId int
		var text string
		if err = rows.Scan(&itemId, &fieldId, &text); err != nil {
			return err
		}
		f := byId[fieldId]
		byItem[itemId][f.Key] = warningStoredValue(f.Type, text)
	}
	return rows.Err()
}

func warningStoredValue(typ, text string) any {
	if customFieldIsCascader(typ) || typ == "checkbox" {
		if text == "" {
			return []string{}
		}
		var value any
		if json.Unmarshal([]byte(text), &value) != nil {
			return map[string]any{"automation_invalid": true}
		}
		result := []string{}
		valid := true
		var flatten func(any)
		flatten = func(v any) {
			switch x := v.(type) {
			case []any:
				for _, child := range x {
					flatten(child)
				}
			case string:
				result = append(result, x)
			default:
				valid = false
			}
		}
		flatten(value)
		if !valid {
			return map[string]any{"automation_invalid": true}
		}
		return result
	}
	value, err := automationSnapshotFieldValue(typ, text)
	if err != nil {
		return map[string]any{"automation_invalid": true}
	}
	return value
}
