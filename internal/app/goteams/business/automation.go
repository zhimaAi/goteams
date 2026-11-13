package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrAutomationRuleNotFound means the rule does not exist or does not belong to the current tenant workspace.
	ErrAutomationRuleNotFound = errors.New("automation rule not found")
	// ErrAutomationExecutionNotFound means the execution does not exist or does not belong to the current tenant workspace.
	ErrAutomationExecutionNotFound = errors.New("automation execution not found")
	// ErrAutomationRuleNameExists means the rule name is duplicated within the workspace.
	ErrAutomationRuleNameExists = errors.New("automation rule name exists")
	// ErrAutomationCopyTargetInvalid means the copy target list is empty or malformed.
	ErrAutomationCopyTargetInvalid = errors.New("invalid automation copy target")
	// ErrAutomationCopyTargetForbidden means the user cannot manage automation in a target workspace.
	ErrAutomationCopyTargetForbidden = errors.New("automation copy target forbidden")

	automationQuotaLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
)

// AutomationRuleModel describes one automation rule.
type AutomationRuleModel struct {
	Id                    int
	AdminId               int
	WorkspaceId           int
	Name                  string
	Enabled               int
	CreatedBy             int
	CreatorNameSnapshot   string
	CreatorAvatarSnapshot string
	CreatorName           string
	CreatorAvatar         string
	Locale                string
	TotalTriggerCount     int64
	Config                json.RawMessage // versioned rule config produced by marshalling AutomationConfig
	CreatedAt             int64
	UpdatedAt             int64
}

// GetAutomationRuleModel describes the get-automation-rule use case.
type GetAutomationRuleModel struct {
	AdminId     int
	WorkspaceId int
	RuleId      int
}

// CreateAutomationRuleModel describes the create-automation-rule use case.
type CreateAutomationRuleModel struct {
	AdminId     int
	WorkspaceId int
	UserId      int
	Name        string
	Locale      string
	Config      json.RawMessage
	Id          int
}

// UpdateAutomationRuleModel describes the update-automation-rule use case.
type UpdateAutomationRuleModel struct {
	AdminId     int
	WorkspaceId int
	RuleId      int
	Name        string
	Locale      string
	Config      json.RawMessage
}

// CopyAutomationRuleModel describes the copy-automation-rule use case.
type CopyAutomationRuleModel struct {
	AdminId     int
	WorkspaceId int
	RuleId      int
	UserId      int
	Locale      string
	Id          int
}

// ListAutomationCopyTargetsModel describes the automation copy-target list use case.
type ListAutomationCopyTargetsModel struct {
	AdminId           int
	UserId            int
	SourceWorkspaceId int
	Items             []AutomationCopyTargetModel
}

// CopyAutomationRulesModel describes copying one rule to multiple workspaces.
type CopyAutomationRulesModel struct {
	AdminId            int
	SourceWorkspaceId  int
	RuleId             int
	UserId             int
	Locale             string
	TargetWorkspaceIds []int
}

// AutomationEvent is the event dispatched to the rule matcher after a user business commit.
type AutomationEvent struct {
	AdminID       int                         `json:"admin_id"`                 // tenant admin id
	WorkspaceID   int                         `json:"workspace_id"`             // workspace id
	ItemID        int                         `json:"item_id"`                  // trigger work item id
	ItemType      string                      `json:"item_type"`                // work item type
	Operation     string                      `json:"operation"`                // business operation type
	CommentChange string                      `json:"comment_change,omitempty"` // comment change type
	Comment       *AutomationCommentSnapshot  `json:"comment,omitempty"`        // comment snapshot
	Before        *AutomationWorkItemSnapshot `json:"before,omitempty"`         // before snapshot
	After         *AutomationWorkItemSnapshot `json:"after,omitempty"`          // after snapshot
	SLA           *AutomationSLASnapshot      `json:"sla,omitempty"`            // 时效管控 verdict, only set by the SLA worker
	TriggeredBy   AutomationUserSnapshot      `json:"triggered_by"`             // triggering user
	TriggeredAt   int64                       `json:"triggered_at"`             // trigger timestamp
	Timezone      string                      `json:"timezone,omitempty"`       // IANA timezone of the triggering user
	Source        string                      `json:"source"`                   // event source
}

// AutomationWorkItemSnapshot stores the work item fields at the time of the event.
type AutomationWorkItemSnapshot struct {
	ID          int            `json:"id"`                     // work item id
	WorkspaceID int            `json:"workspace_id"`           // workspace id
	Type        string         `json:"type"`                   // work item type
	Title       string         `json:"title"`                  // title
	Description string         `json:"description"`            // description
	ParentID    int            `json:"parent_id"`              // parent requirement id
	AssigneeIDs []int          `json:"assignee_ids,omitempty"` // every assignee id, ordered as stored
	Fields      map[string]any `json:"fields"`                 // built-in and custom field values
}

// AutomationCommentSnapshot stores the comment content when it is created, edited or deleted.
type AutomationCommentSnapshot struct {
	ID      int    `json:"id"`      // comment id
	Content string `json:"content"` // comment content
	UserID  int    `json:"user_id"` // comment author id
}

// AutomationSLASnapshot carries the verdict of one 时效管控 (aging control) evaluation. The SLA
// worker is the only producer: time driven notifications have no human operation behind them, so
// they are identified by this verdict instead of a business operation.
type AutomationSLASnapshot struct {
	Overdue  bool  `json:"overdue"`   // true once the configured rule is already breached
	BreachAt int64 `json:"breach_at"` // the moment the configured rule breaks
}

// AutomationUserSnapshot stores the audit info of the triggering user.
type AutomationUserSnapshot struct {
	ID      int    `json:"id"`      // user id
	Name    string `json:"name"`    // user display name
	Account string `json:"account"` // user account (username)
	Phone   string `json:"phone"`   // user phone
	Remark  string `json:"remark"`  // user remark
	Avatar  string `json:"avatar"`  // user avatar
}

// AutomationMetadataFieldOptionModel describes an option exposed by automation metadata.
type AutomationMetadataFieldOptionModel struct {
	Key   string
	Name  string
	Color string
}

// AutomationMetadataFieldModel describes a configurable automation field.
type AutomationMetadataFieldModel struct {
	MemberScope string
	RoleIds     []int
	Key         string
	Label       string
	Type        string
	Editable    bool
	Deleted     bool
	Options     []AutomationMetadataFieldOptionModel
}

// AutomationVariableModel describes one insertable template variable.
type AutomationVariableModel struct {
	Key   string
	Group string
	Label string
}

// AutomationCopyTargetModel describes a workspace available to copy actions.
type AutomationCopyTargetModel struct {
	Id   int
	Name string
}

// AutomationMetadataModel describes the automation metadata use case.
type AutomationMetadataModel struct {
	AdminId           int
	WorkspaceId       int
	UserId            int
	RuleId            int
	ItemType          string
	Lang              string
	Fields            []AutomationMetadataFieldModel
	RequirementFields []AutomationMetadataFieldModel
	Variables         []AutomationVariableModel
	CopyTargets       []AutomationCopyTargetModel
	WorkspaceMetadata WorkspaceMetadataModel
}

// AutomationTriggerSnapshot is the complete event snapshot persisted in an execution.
type AutomationTriggerSnapshot struct {
	Event  AutomationEvent             `json:"event"`            // raw business event
	Parent *AutomationWorkItemSnapshot `json:"parent,omitempty"` // related parent requirement snapshot
	Child  *AutomationWorkItemSnapshot `json:"child,omitempty"`  // related child work item snapshot
}

// AutomationRuleList returns a server-side paginated list of rules.
func AutomationRuleList(adminID, workspaceID int, search string, page, size int, todayStart, todayEnd, yesterdayStart, yesterdayEnd int64) (map[string]any, error) {
	page, size = normalizeAutomationPage(page, size)
	where := `r.admin_id = $1 AND r.workspace_id = $2`
	countWhere := where
	countArgs := []any{adminID, workspaceID}
	args := []any{adminID, workspaceID, todayStart, todayEnd, yesterdayStart, yesterdayEnd}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		where += fmt.Sprintf(` AND r.name ILIKE $%d`, len(args))
		countArgs = append(countArgs, "%"+search+"%")
		countWhere += ` AND r.name ILIKE $3`
	}
	countRows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM automation_rules r WHERE `+countWhere, nil, countArgs...)
	if err != nil {
		return nil, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := msql.RawValues(define.Postgres, `SELECT r.id, r.name, r.enabled, r.total_trigger_count,
		r.created_by, COALESCE(u.name, r.creator_name_snapshot) AS creator_name,
		COALESCE(NULLIF(u.avatar, ''), r.creator_avatar_snapshot) AS creator_avatar,
		r.locale, r.created_at, r.updated_at,
		COUNT(e.id) FILTER (WHERE e.triggered_at >= $3 AND e.triggered_at < $4) AS today_trigger_count,
		COUNT(e.id) FILTER (WHERE e.triggered_at >= $5 AND e.triggered_at < $6) AS yesterday_trigger_count
		FROM automation_rules r
		LEFT JOIN users u ON u.id = r.created_by AND u.admin_id = r.admin_id
		LEFT JOIN automation_executions e ON e.admin_id = r.admin_id AND e.workspace_id = r.workspace_id AND e.rule_id = r.id
			AND ((e.triggered_at >= $3 AND e.triggered_at < $4) OR (e.triggered_at >= $5 AND e.triggered_at < $6))
		WHERE `+where+`
		GROUP BY r.id, u.name, u.avatar
		ORDER BY r.created_at DESC, r.id DESC LIMIT $`+cast.ToString(len(args)-1)+` OFFSET $`+cast.ToString(len(args)), nil, args...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"list": rows, "total": cast.ToInt(countRows[0]["total"]), "page": page, "size": size}, nil
}

// toAutomationRuleModel converts one rule row into the business model.
func toAutomationRuleModel(row msql.Params) (AutomationRuleModel, error) {
	model := AutomationRuleModel{
		Name:                  row[`name`],
		CreatorNameSnapshot:   row[`creator_name_snapshot`],
		CreatorAvatarSnapshot: row[`creator_avatar_snapshot`],
		CreatorName:           row[`creator_name`],
		CreatorAvatar:         row[`creator_avatar`],
		Locale:                row[`locale`],
		Config:                json.RawMessage(row[`config_json`]),
	}
	numbers := []struct {
		column string
		target *int
	}{
		{`id`, &model.Id}, {`admin_id`, &model.AdminId}, {`workspace_id`, &model.WorkspaceId},
		{`enabled`, &model.Enabled}, {`created_by`, &model.CreatedBy},
	}
	for _, item := range numbers {
		value, err := strconv.Atoi(row[item.column])
		if err != nil {
			return AutomationRuleModel{}, fmt.Errorf("parse automation rule %s: %w", item.column, err)
		}
		*item.target = value
	}
	timestamps := []struct {
		column string
		target *int64
	}{
		{`total_trigger_count`, &model.TotalTriggerCount}, {`created_at`, &model.CreatedAt}, {`updated_at`, &model.UpdatedAt},
	}
	for _, item := range timestamps {
		value, err := strconv.ParseInt(row[item.column], 10, 64)
		if err != nil {
			return AutomationRuleModel{}, fmt.Errorf("parse automation rule %s: %w", item.column, err)
		}
		*item.target = value
	}
	return model, nil
}

// GetAutomationRule returns the rule detail.
func GetAutomationRule(model GetAutomationRuleModel) (AutomationRuleModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT r.id, r.admin_id, r.workspace_id, r.name, r.enabled, r.created_by,
		r.creator_name_snapshot, r.creator_avatar_snapshot, r.locale, r.total_trigger_count, r.config_json,
		r.created_at, r.updated_at, COALESCE(u.name, r.creator_name_snapshot) AS creator_name,
		COALESCE(NULLIF(u.avatar, ''), r.creator_avatar_snapshot) AS creator_avatar
		FROM automation_rules r LEFT JOIN users u ON u.id = r.created_by AND u.admin_id = r.admin_id
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.id = $3`, nil, model.AdminId, model.WorkspaceId, model.RuleId)
	if err != nil {
		logs.Error("get automation rule failed: %s", err.Error())
		return AutomationRuleModel{}, err
	}
	if len(rows) == 0 {
		return AutomationRuleModel{}, ErrAutomationRuleNotFound
	}
	return toAutomationRuleModel(rows[0])
}

// CreateAutomationRule creates a rule that is disabled by default.
func CreateAutomationRule(model CreateAutomationRuleModel) (CreateAutomationRuleModel, error) {
	name := strings.TrimSpace(model.Name)
	config, err := decodeAutomationConfig(model.Config)
	if err != nil {
		return CreateAutomationRuleModel{}, err
	}
	if err = validateAutomationReferences(model.AdminId, model.WorkspaceId, model.UserId, &config); err != nil {
		return CreateAutomationRuleModel{}, err
	}
	if model.Locale != lib_i18n.LangEnUs && model.Locale != lib_i18n.LangZhCn {
		return CreateAutomationRuleModel{}, ErrAutomationConfigInvalid
	}
	creator, err := automationUserSnapshot(model.AdminId, model.UserId)
	if err != nil {
		return CreateAutomationRuleModel{}, err
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return CreateAutomationRuleModel{}, err
	}
	if err = EnsureDefaultPlan(model.AdminId); err != nil {
		return CreateAutomationRuleModel{}, err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return CreateAutomationRuleModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// 负数 admin_id 为自动化额度锁命名空间，避免与需求层级锁冲突。
	if _, err = msql.RawValues(define.Postgres, `SELECT pg_advisory_xact_lock($1, $2)`, tx, -model.AdminId, model.WorkspaceId); err != nil {
		return CreateAutomationRuleModel{}, err
	}
	// 私有化取消规则数量配额，但后台入口仍须有可用授权。
	if _, licenseOk := licenseQuotasIfValid(); !licenseOk {
		return CreateAutomationRuleModel{}, ErrLicenseNotActivated
	}
	now := tool.Time2Int()
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO automation_rules
		(admin_id, workspace_id, name, enabled, created_by, creator_name_snapshot, creator_avatar_snapshot,
		 locale, total_trigger_count, config_json, created_at, updated_at)
		VALUES ($1,$2,$3,0,$4,$5,$6,$7,0,$8,$9,$9) RETURNING id`, tx,
		model.AdminId, model.WorkspaceId, name, model.UserId, creator.Name, creator.Avatar, model.Locale, encoded, now)
	if err != nil {
		if isAutomationNameConflict(err) {
			return CreateAutomationRuleModel{}, ErrAutomationRuleNameExists
		}
		logs.Error("create automation rule failed: %s", err.Error())
		return CreateAutomationRuleModel{}, err
	}
	if len(rows) == 0 {
		return CreateAutomationRuleModel{}, errors.New("automation rule insert returned no row")
	}
	id, err := strconv.Atoi(rows[0][`id`])
	if err != nil {
		return CreateAutomationRuleModel{}, fmt.Errorf("parse automation rule id: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return CreateAutomationRuleModel{}, err
	}
	model.Id = id
	return model, nil
}

// UpdateAutomationRule edits a rule while preserving the rule ID and trigger statistics.
func UpdateAutomationRule(model UpdateAutomationRuleModel) error {
	config, err := decodeAutomationConfig(model.Config)
	if err != nil {
		return err
	}
	creatorRows, err := msql.RawValues(define.Postgres, `SELECT created_by FROM automation_rules WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, nil, model.AdminId, model.WorkspaceId, model.RuleId)
	if err != nil {
		logs.Error("query automation rule creator failed: %s", err.Error())
		return err
	}
	if len(creatorRows) == 0 {
		return ErrAutomationRuleNotFound
	}
	creatorID, err := strconv.Atoi(creatorRows[0][`created_by`])
	if err != nil {
		return fmt.Errorf("parse automation rule created_by: %w", err)
	}
	if err = validateAutomationReferences(model.AdminId, model.WorkspaceId, creatorID, &config); err != nil {
		return err
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return err
	}
	result, err := msql.RawExec(define.Postgres, `UPDATE automation_rules SET name = $1, locale = $2,
		config_json = $3, updated_at = $4 WHERE admin_id = $5 AND workspace_id = $6 AND id = $7`, nil,
		strings.TrimSpace(model.Name), model.Locale, encoded, tool.Time2Int(), model.AdminId, model.WorkspaceId, model.RuleId)
	if err != nil {
		if isAutomationNameConflict(err) {
			return ErrAutomationRuleNameExists
		}
		logs.Error("update automation rule failed: %s", err.Error())
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAutomationRuleNotFound
	}
	return nil
}

// CopyAutomationRule copies a rule to its source workspace for backwards compatibility.
func CopyAutomationRule(model CopyAutomationRuleModel) (CopyAutomationRuleModel, error) {
	ids, err := CopyAutomationRules(CopyAutomationRulesModel{
		AdminId: model.AdminId, SourceWorkspaceId: model.WorkspaceId, RuleId: model.RuleId,
		UserId: model.UserId, Locale: model.Locale, TargetWorkspaceIds: []int{model.WorkspaceId},
	})
	if err != nil {
		return CopyAutomationRuleModel{}, err
	}
	if len(ids) > 0 {
		model.Id = ids[0]
	}
	return model, nil
}

// ListAutomationCopyTargets returns active workspaces where the user can manage automation.
func ListAutomationCopyTargets(model ListAutomationCopyTargetsModel) (ListAutomationCopyTargetsModel, error) {
	if model.AdminId <= 0 || model.UserId <= 0 || model.SourceWorkspaceId <= 0 {
		return ListAutomationCopyTargetsModel{}, ErrAutomationCopyTargetInvalid
	}

	var (
		rows []msql.Params
		err  error
	)
	if IsTenantOwner(model.AdminId, model.UserId) {
		rows, err = msql.RawValues(define.Postgres, `SELECT id, name FROM workspaces
			WHERE admin_id = $1 AND COALESCE(ended_at, 0) = 0
			ORDER BY CASE WHEN id = $2 THEN 0 ELSE 1 END, sort_order ASC, id DESC`, nil,
			model.AdminId, model.SourceWorkspaceId)
	} else {
		rows, err = msql.RawValues(define.Postgres, `SELECT w.id, w.name, w.sort_order FROM workspace_members wm
			JOIN role_permissions rp ON rp.admin_id = wm.admin_id AND rp.role_id = wm.role_id
			JOIN workspaces w ON w.admin_id = wm.admin_id AND w.id = wm.workspace_id
			WHERE wm.admin_id = $1 AND wm.user_id = $2 AND rp.permission_key = $3
				AND COALESCE(w.ended_at, 0) = 0
			GROUP BY w.id, w.name, w.sort_order
			ORDER BY CASE WHEN w.id = $4 THEN 0 ELSE 1 END, w.sort_order ASC, w.id DESC`, nil,
			model.AdminId, model.UserId, "automation:manage", model.SourceWorkspaceId)
	}
	if err != nil {
		logs.Error("list automation copy targets error: %s", err.Error())
		return ListAutomationCopyTargetsModel{}, err
	}

	model.Items = make([]AutomationCopyTargetModel, 0, len(rows))
	for _, row := range rows {
		id, convErr := strconv.Atoi(row["id"])
		if convErr != nil {
			logs.Error("convert automation copy target id error: %s", convErr.Error())
			return ListAutomationCopyTargetsModel{}, convErr
		}
		model.Items = append(model.Items, AutomationCopyTargetModel{Id: id, Name: row["name"]})
	}
	return model, nil
}

// CopyAutomationRules copies one rule to multiple workspaces atomically.
func CopyAutomationRules(model CopyAutomationRulesModel) ([]int, error) {
	targetWorkspaceIds, err := normalizeAutomationCopyTargetIds(model.SourceWorkspaceId, model.TargetWorkspaceIds)
	if err != nil {
		return nil, err
	}
	if model.AdminId <= 0 || model.UserId <= 0 || model.RuleId <= 0 || model.Locale != lib_i18n.LangEnUs && model.Locale != lib_i18n.LangZhCn {
		return nil, ErrAutomationCopyTargetInvalid
	}

	rule, err := GetAutomationRule(GetAutomationRuleModel{AdminId: model.AdminId, WorkspaceId: model.SourceWorkspaceId, RuleId: model.RuleId})
	if err != nil {
		return nil, err
	}
	config, err := decodeAutomationConfig(rule.Config)
	if err != nil {
		return nil, err
	}
	if err = validateAutomationReferences(model.AdminId, model.SourceWorkspaceId, model.UserId, &config); err != nil {
		return nil, err
	}
	targets, err := ListAutomationCopyTargets(ListAutomationCopyTargetsModel{
		AdminId: model.AdminId, UserId: model.UserId, SourceWorkspaceId: model.SourceWorkspaceId,
	})
	if err != nil {
		return nil, err
	}
	allowedTargets := make(map[int]bool, len(targets.Items))
	for _, target := range targets.Items {
		allowedTargets[target.Id] = true
	}
	for _, workspaceId := range targetWorkspaceIds {
		if !allowedTargets[workspaceId] {
			return nil, ErrAutomationCopyTargetForbidden
		}
	}
	// Target metadata can differ by project; copied rules stay disabled until stale references are repaired in the editor.
	creator, err := automationUserSnapshot(model.AdminId, model.UserId)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	if err = EnsureDefaultPlan(model.AdminId); err != nil {
		return nil, err
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	type automationCopyInsert struct {
		workspaceId int
		name        string
	}
	insertions := make([]automationCopyInsert, 0, len(targetWorkspaceIds))
	for _, workspaceId := range targetWorkspaceIds {
		if _, err = msql.RawValues(define.Postgres, `SELECT pg_advisory_xact_lock($1, $2)`, tx, -model.AdminId, workspaceId); err != nil {
			return nil, err
		}
		if _, licenseOk := licenseQuotasIfValid(); !licenseOk {
			return nil, ErrLicenseNotActivated
		}
		name, nameErr := nextAutomationCopyNameTx(tx, model.AdminId, workspaceId, rule.Name, model.Locale)
		if nameErr != nil {
			return nil, nameErr
		}
		insertions = append(insertions, automationCopyInsert{workspaceId: workspaceId, name: name})
	}

	createdIds := make([]int, 0, len(insertions))
	for _, insertion := range insertions {
		rows, insertErr := msql.RawValues(define.Postgres, `INSERT INTO automation_rules
			(admin_id, workspace_id, name, enabled, created_by, creator_name_snapshot, creator_avatar_snapshot,
			 locale, total_trigger_count, config_json, created_at, updated_at)
			VALUES ($1,$2,$3,0,$4,$5,$6,$7,0,$8,$9,$9) RETURNING id`, tx,
			model.AdminId, insertion.workspaceId, insertion.name, model.UserId, creator.Name, creator.Avatar, model.Locale, encoded, tool.Time2Int())
		if insertErr != nil {
			if isAutomationNameConflict(insertErr) {
				return nil, ErrAutomationRuleNameExists
			}
			return nil, insertErr
		}
		if len(rows) == 0 {
			return nil, errors.New("automation rule insert returned no row")
		}
		id, idErr := strconv.Atoi(rows[0]["id"])
		if idErr != nil {
			return nil, idErr
		}
		createdIds = append(createdIds, id)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return createdIds, nil
}

// SetAutomationRuleEnabled enables or disables a rule, allowing stale references to remain.
func SetAutomationRuleEnabled(adminID, workspaceID, ruleID int, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	result, err := msql.RawExec(define.Postgres, `UPDATE automation_rules SET enabled = $1, updated_at = $2
		WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`, nil, value, tool.Time2Int(), adminID, workspaceID, ruleID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAutomationRuleNotFound
	}
	return nil
}

// DeleteAutomationRule deletes a rule but permanently keeps its execution records.
func DeleteAutomationRule(adminID, workspaceID, ruleID int) error {
	result, err := msql.RawExec(define.Postgres, `DELETE FROM automation_rules WHERE admin_id = $1 AND workspace_id = $2 AND id = $3`, nil,
		adminID, workspaceID, ruleID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAutomationRuleNotFound
	}
	return nil
}

// ListAutomationExecutions returns a server-side paginated list of executions.
func ListAutomationExecutions(adminID, workspaceID int, ruleID int, search string, startAt, endAt int64, page, size int) (map[string]any, error) {
	page, size = normalizeAutomationPage(page, size)
	where := []string{"e.admin_id = $1", "e.workspace_id = $2"}
	args := []any{adminID, workspaceID}
	if ruleID > 0 {
		args = append(args, ruleID)
		where = append(where, fmt.Sprintf("e.rule_id = $%d", len(args)))
	}
	if search = strings.TrimSpace(search); search != "" {
		args = append(args, "%"+search+"%")
		where = append(where, fmt.Sprintf("COALESCE(r.name, e.rule_name_snapshot) ILIKE $%d", len(args)))
	}
	if startAt > 0 {
		args = append(args, startAt)
		where = append(where, fmt.Sprintf("e.triggered_at >= $%d", len(args)))
	}
	if endAt > 0 {
		args = append(args, endAt)
		where = append(where, fmt.Sprintf("e.triggered_at < $%d", len(args)))
	}
	whereSQL := strings.Join(where, " AND ")
	countRows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM automation_executions e
		LEFT JOIN automation_rules r ON r.admin_id=e.admin_id AND r.workspace_id=e.workspace_id AND r.id=e.rule_id WHERE `+whereSQL, nil, args...)
	if err != nil {
		return nil, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := msql.RawValues(define.Postgres, `SELECT e.id, e.rule_id, COALESCE(r.name,e.rule_name_snapshot) AS rule_name,
		e.trigger_item_id, e.trigger_item_type, e.trigger_item_title, e.event_type, e.status, e.triggered_at,
		e.started_at, e.finished_at, e.failure_reason FROM automation_executions e
		LEFT JOIN automation_rules r ON r.admin_id=e.admin_id AND r.workspace_id=e.workspace_id AND r.id=e.rule_id
		WHERE `+whereSQL+` ORDER BY e.triggered_at DESC, e.id DESC LIMIT $`+cast.ToString(len(args)-1)+` OFFSET $`+cast.ToString(len(args)), nil, args...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"list": rows, "total": cast.ToInt(countRows[0]["total"]), "page": page, "size": size}, nil
}

// GetAutomationExecution returns the execution detail and renders structured logs in the requested language.
func GetAutomationExecution(adminID, workspaceID int, executionID int64, lang string) (map[string]any, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT e.*, COALESCE(r.name,e.rule_name_snapshot) AS rule_name
		FROM automation_executions e LEFT JOIN automation_rules r
		ON r.admin_id=e.admin_id AND r.workspace_id=e.workspace_id AND r.id=e.rule_id
		WHERE e.admin_id=$1 AND e.workspace_id=$2 AND e.id=$3`, nil, adminID, workspaceID, executionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrAutomationExecutionNotFound
	}
	result := map[string]any{}
	for key, value := range rows[0] {
		if key != "structured_logs" && key != "config_snapshot" && key != "trigger_snapshot" && key != "node_progress" {
			result[key] = value
		}
	}
	logsList := []AutomationLog{}
	if err = json.Unmarshal([]byte(rows[0]["structured_logs"]), &logsList); err != nil {
		return nil, err
	}
	rendered := renderAutomationLogs(logsList, lang)
	if result["failure_reason"] == "automation_execution_failed" {
		result["failure_reason"] = lib_i18n.Show(lang, "automation_execution_failed")
	}
	result["logs"] = rendered
	return result, nil
}

func renderAutomationLogs(logsList []AutomationLog, lang string) []map[string]any {
	rendered := make([]map[string]any, 0, len(logsList))
	for _, item := range logsList {
		entry := map[string]any{"time": item.Time, "key": item.Key, "text": lib_i18n.Show(lang, item.Key), "node": item.Node, "attempt": item.Attempt, "parameters": item.Parameters}
		if item.Raw != "" {
			entry["raw"] = item.Raw
		}
		rendered = append(rendered, entry)
	}
	return rendered
}

// AutomationMetadata returns fields, template variables and copy target workspaces.
func AutomationMetadata(model AutomationMetadataModel) (AutomationMetadataModel, error) {
	if model.RuleId > 0 {
		rows, err := msql.RawValues(define.Postgres, `SELECT created_by FROM automation_rules WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, nil, model.AdminId, model.WorkspaceId, model.RuleId)
		if err != nil {
			return AutomationMetadataModel{}, err
		}
		if len(rows) == 0 {
			return AutomationMetadataModel{}, ErrAutomationRuleNotFound
		}
		model.UserId = cast.ToInt(rows[0]["created_by"])
	}
	fields, err := automationFields(model.AdminId, model.WorkspaceId, model.ItemType, model.Lang)
	if err != nil {
		return AutomationMetadataModel{}, err
	}
	requirementFields, err := automationFields(model.AdminId, model.WorkspaceId, define.TypeKeyRequirement, model.Lang)
	if err != nil {
		return AutomationMetadataModel{}, err
	}
	model.Fields = toAutomationMetadataFieldModels(fields)
	model.RequirementFields = toAutomationMetadataFieldModels(requirementFields)
	model.Variables = automationVariableMetadata(fields, requirementFields, model.Lang)
	model.CopyTargets, err = automationCopyTargets(model.AdminId, model.UserId, model.ItemType)
	if err != nil {
		return AutomationMetadataModel{}, err
	}
	model.WorkspaceMetadata, err = GetWorkspaceMetadata(WorkspaceMetadataModel{
		AdminId: model.AdminId, WorkspaceId: cast.ToString(model.WorkspaceId), Lang: model.Lang, IncludeVersions: true,
	})
	if err != nil {
		return AutomationMetadataModel{}, err
	}
	return model, nil
}

func toAutomationMetadataFieldModels(fields []AutomationFieldMeta) []AutomationMetadataFieldModel {
	models := make([]AutomationMetadataFieldModel, 0, len(fields))
	for _, field := range fields {
		options := make([]AutomationMetadataFieldOptionModel, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, AutomationMetadataFieldOptionModel{Key: option.Key, Name: option.Name, Color: option.Color})
		}
		if field.Options == nil {
			options = nil
		}
		models = append(models, AutomationMetadataFieldModel{
			Key: field.Key, Label: field.Label, Type: field.Type, Editable: field.Editable, Deleted: field.Deleted, Options: options, MemberScope: field.MemberScope, RoleIds: field.RoleIds,
		})
	}
	return models
}

// CaptureAutomationSnapshot captures the current snapshot of a work item; returns nil when it does not exist.
func CaptureAutomationSnapshot(adminID, workspaceID, itemID int) (*AutomationWorkItemSnapshot, error) {
	return captureAutomationSnapshot(nil, adminID, workspaceID, itemID)
}

// captureAutomationSnapshot reads the snapshot through the supplied transaction when present.
func captureAutomationSnapshot(tx *sql.Tx, adminID, workspaceID, itemID int) (*AutomationWorkItemSnapshot, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, workspace_id, type, title, description, parent_id, priority,
		status_id, COALESCE(assignee_ids[1], 0) AS assignee_id, assignee_ids::text AS assignee_ids, version_id,
		planned_start_date, planned_end_date, created_by, created_at, updated_at
		FROM requirements WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, tx, adminID, workspaceID, itemID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	fields := map[string]any{
		"id": cast.ToInt(row["id"]), "title": row["title"], "description": row["description"],
		"priority": cast.ToInt(row["priority"]), "status": cast.ToInt(row["status_id"]),
		"assignee": cast.ToInt(row["assignee_id"]), "version": cast.ToInt(row["version_id"]),
		"planned_start_date": cast.ToInt64(row["planned_start_date"]), "planned_end_date": cast.ToInt64(row["planned_end_date"]),
		"created_by": cast.ToInt(row["created_by"]), "created_at": cast.ToInt64(row["created_at"]), "updated_at": cast.ToInt64(row["updated_at"]),
	}
	customRows, err := msql.RawValues(define.Postgres, `SELECT f.key, f.field_type, v.value_text FROM workspace_custom_fields f
		LEFT JOIN requirement_custom_field_values v ON v.admin_id=f.admin_id AND v.field_id=f.id AND v.requirement_id=$3
		WHERE f.admin_id=$1 AND f.workspace_id=$2 AND f.type_key=$4`, tx, adminID, workspaceID, itemID, row["type"])
	if err != nil {
		return nil, err
	}
	for _, custom := range customRows {
		value, parseErr := automationSnapshotFieldValue(custom["field_type"], custom["value_text"])
		if parseErr != nil {
			logs.Error("invalid automation field: admin_id=%d workspace_id=%d item_id=%d field=%s err=%s", adminID, workspaceID, itemID, custom["key"], parseErr.Error())
			value = map[string]any{"automation_invalid": true}
		}
		fields[custom["key"]] = value
	}
	return &AutomationWorkItemSnapshot{ID: itemID, WorkspaceID: workspaceID, Type: row["type"], Title: row["title"], Description: row["description"],
		ParentID: cast.ToInt(row["parent_id"]), AssigneeIDs: automationAssigneeIDs(row["assignee_ids"]), Fields: fields}, nil
}

// automationAssigneeIDs converts a stored array text, a snapshot list or a single id into an ordered assignee id list.
func automationAssigneeIDs(value any) []int {
	items := []any{}
	switch typed := value.(type) {
	case string:
		if text := normalizeStoredAssigneeIDs(typed); text != "" {
			for _, item := range strings.Split(text, ",") {
				items = append(items, item)
			}
		}
	case []any:
		items = typed
	case []string:
		for _, item := range typed {
			items = append(items, item)
		}
	case []int:
		for _, item := range typed {
			items = append(items, item)
		}
	default:
		items = append(items, value)
	}
	result := []int{}
	for _, item := range items {
		if id := cast.ToInt(item); id > 0 {
			result = append(result, id)
		}
	}
	return result
}

// EnqueueAutomationEvent matches rules after a user business commit; failures are only returned to the caller for logging.
func EnqueueAutomationEvent(event AutomationEvent) error {
	if event.Source == define.SourceAutomation {
		return nil
	}
	// TriggeredAt must be set before dispatch: the deadline events compare the new deadline with it.
	if event.TriggeredAt == 0 {
		event.TriggeredAt = time.Now().Unix()
	}
	DispatchExternalNotifications(event)
	if event.After == nil && event.Before == nil {
		return nil
	}
	trigger := event.After
	if trigger == nil {
		trigger = event.Before
	}
	rules, err := msql.RawValues(define.Postgres, `SELECT r.*, COALESCE(u.name,r.creator_name_snapshot) AS creator_name,
		COALESCE(NULLIF(u.avatar,''),r.creator_avatar_snapshot) AS creator_avatar
		FROM automation_rules r LEFT JOIN users u ON u.id=r.created_by AND u.admin_id=r.admin_id
		WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.enabled=1 ORDER BY r.created_at ASC, r.id ASC`, nil,
		event.AdminID, event.WorkspaceID)
	if err != nil {
		return err
	}
	type matchedRule struct {
		rule   msql.Params
		config AutomationConfig
	}
	matchedRules := make([]matchedRule, 0, len(rules))
	for _, rule := range rules {
		config, decodeErr := decodeAutomationConfig([]byte(rule["config_json"]))
		if decodeErr != nil {
			logs.Error("automation rule %s config decode failed: %s", rule["id"], decodeErr.Error())
			continue
		}
		if automationTriggerMatches(config.Trigger, event) {
			matchedRules = append(matchedRules, matchedRule{rule: rule, config: config})
		}
	}
	if len(matchedRules) == 0 {
		return nil
	}
	parent, child, err := automationRelativeSnapshots(event.AdminID, event.WorkspaceID, trigger)
	if err != nil {
		return err
	}
	snapshotJSON, err := json.Marshal(AutomationTriggerSnapshot{Event: event, Parent: parent, Child: child})
	if err != nil {
		return err
	}
	if err = EnsureDefaultPlan(event.AdminID); err != nil {
		return err
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// 同一企业的自动化执行额度串行分配，确保并发触发时不超额。
	if _, err = msql.RawValues(define.Postgres, `SELECT pg_advisory_xact_lock($1, 0)`, tx, -event.AdminID); err != nil {
		return err
	}
	// 不再按月分配额度，保留同一企业事件入队的事务顺序和有效授权门禁。
	if _, licenseOk := licenseQuotasIfValid(); !licenseOk {
		return ErrLicenseNotActivated
	}
	now := tool.Time2Int()
	for _, matched := range matchedRules {
		rule, config := matched.rule, matched.config
		configJSON, marshalErr := json.Marshal(config)
		if marshalErr != nil {
			return marshalErr
		}
		progressJSON, marshalErr := json.Marshal(AutomationNodeProgress{Completed: []int{}})
		if marshalErr != nil {
			return marshalErr
		}
		rows, insertErr := msql.RawValues(define.Postgres, `INSERT INTO automation_executions
			(admin_id,workspace_id,rule_id,trigger_item_id,trigger_item_type,trigger_item_title,event_type,status,
			 triggered_at,rule_name_snapshot,creator_id_snapshot,creator_name_snapshot,creator_avatar_snapshot,locale,
			 config_snapshot,trigger_snapshot,node_progress,structured_logs,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,'pending',$8,$9,$10,$11,$12,$13,$14,$15,$16,'[]',$17,$17) RETURNING id`, tx,
			event.AdminID, event.WorkspaceID, cast.ToInt(rule["id"]), event.ItemID, event.ItemType, trigger.Title,
			config.Trigger.Event, event.TriggeredAt, rule["name"], cast.ToInt(rule["created_by"]), rule["creator_name"],
			rule["creator_avatar"], rule["locale"], configJSON, snapshotJSON, progressJSON, now)
		if insertErr != nil {
			return insertErr
		}
		if len(rows) == 0 {
			return errors.New("automation execution insert returned no row")
		}
		if _, updateErr := msql.RawExec(define.Postgres, `UPDATE automation_rules SET total_trigger_count=total_trigger_count+1,
			updated_at=$1 WHERE admin_id=$2 AND workspace_id=$3 AND id=$4`, tx, now, event.AdminID, event.WorkspaceID, cast.ToInt(rule["id"])); updateErr != nil {
			return updateErr
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func automationMonthRange(now time.Time) (int64, int64) {
	local := now.In(automationQuotaLocation)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, automationQuotaLocation)
	return start.Unix(), start.AddDate(0, 1, 0).Unix()
}

func automationAllowedCount(used, limit, requested int) int {
	remaining := limit - used
	if remaining <= 0 || requested <= 0 {
		return 0
	}
	if requested < remaining {
		return requested
	}
	return remaining
}

// enqueueWorkItemAutomation captures a fresh snapshot after a user operation and tries to enqueue events.
func enqueueWorkItemAutomation(adminID, workspaceID, itemID, userID int, operation, timezone string, before *AutomationWorkItemSnapshot) {
	after, err := CaptureAutomationSnapshot(adminID, workspaceID, itemID)
	if err != nil || after == nil {
		if err != nil {
			logs.Error("capture work item automation snapshot failed: %s", err.Error())
		}
		return
	}
	user, err := automationUserSnapshot(adminID, userID)
	if err != nil {
		logs.Error("capture work item automation user failed: %s", err.Error())
		return
	}
	event := AutomationEvent{AdminID: adminID, WorkspaceID: workspaceID, ItemID: itemID, ItemType: after.Type,
		Operation: operation, Before: before, After: after, TriggeredBy: user, TriggeredAt: int64(tool.Time2Int()),
		Timezone: timezone, Source: define.SourceUser}
	if err = EnqueueAutomationEvent(event); err != nil {
		logs.Error("enqueue work item automation failed: %s", err.Error())
	}
}

// RefreshAutomationCreatorSnapshots refreshes rule and history signature snapshots before a member is removed.
func RefreshAutomationCreatorSnapshots(tx *sql.Tx, adminID, userID int) error {
	user, err := automationUserSnapshotWithTx(tx, adminID, userID)
	if err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE automation_rules SET creator_name_snapshot=$1, creator_avatar_snapshot=$2
		WHERE admin_id=$3 AND created_by=$4`, tx, user.Name, user.Avatar, adminID, userID); err != nil {
		return err
	}
	_, err = msql.RawExec(define.Postgres, `UPDATE automation_executions SET creator_name_snapshot=$1, creator_avatar_snapshot=$2
		WHERE admin_id=$3 AND creator_id_snapshot=$4`, tx, user.Name, user.Avatar, adminID, userID)
	return err
}

// normalizeAutomationPage normalizes pagination defaults and the maximum page size.
func normalizeAutomationPage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size != 20 && size != 50 && size != 100 {
		size = 20
	}
	return page, size
}

// isAutomationNameConflict reports whether the rule name uniqueness constraint is violated.
func isAutomationNameConflict(err error) bool {
	var pqError *pq.Error
	return errors.As(err, &pqError) && pqError.Code == "23505"
}

// nextAutomationCopyName generates a non-duplicating localized copy name within the workspace.
func nextAutomationCopyName(adminID, workspaceID int, original, locale string) (string, error) {
	return nextAutomationCopyNameTx(nil, adminID, workspaceID, original, locale)
}

// nextAutomationCopyNameTx generates a non-duplicating localized copy name using the provided transaction.
func nextAutomationCopyNameTx(tx *sql.Tx, adminID, workspaceID int, original, locale string) (string, error) {
	format := "%s (Copy)"
	formatN := "%s (Copy %d)"
	if locale == lib_i18n.LangZhCn {
		format = "%s（副本）"
		formatN = "%s（副本 %d）"
	}
	for index := 1; ; index++ {
		candidate := fmt.Sprintf(format, original)
		if index > 1 {
			candidate = fmt.Sprintf(formatN, original, index)
		}
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM automation_rules WHERE admin_id=$1 AND workspace_id=$2
			AND lower(btrim(name))=lower(btrim($3))`, tx, adminID, workspaceID, candidate)
		if err != nil {
			return "", err
		}
		if len(rows) == 0 {
			return candidate, nil
		}
	}
}

func normalizeAutomationCopyTargetIds(sourceWorkspaceId int, targetWorkspaceIds []int) ([]int, error) {
	if sourceWorkspaceId <= 0 || len(targetWorkspaceIds) == 0 {
		return nil, ErrAutomationCopyTargetInvalid
	}
	seen := make(map[int]bool, len(targetWorkspaceIds))
	result := make([]int, 0, len(targetWorkspaceIds))
	for _, workspaceId := range targetWorkspaceIds {
		if workspaceId <= 0 {
			return nil, ErrAutomationCopyTargetInvalid
		}
		if seen[workspaceId] {
			continue
		}
		seen[workspaceId] = true
		result = append(result, workspaceId)
	}
	// Stable lock order prevents deadlocks when concurrent requests target the same projects.
	sort.Ints(result)
	return result, nil
}

// automationUserSnapshot reads the audit snapshot of the triggering user.
func automationUserSnapshot(adminID, userID int) (AutomationUserSnapshot, error) {
	return automationUserSnapshotWithTx(nil, adminID, userID)
}

func automationUserSnapshotWithTx(tx *sql.Tx, adminID, userID int) (AutomationUserSnapshot, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id,name,username,tel_number,remark,avatar FROM users WHERE admin_id=$1 AND id=$2`, tx, adminID, userID)
	if err != nil {
		return AutomationUserSnapshot{}, err
	}
	if len(rows) == 0 {
		return AutomationUserSnapshot{ID: userID}, nil
	}
	return AutomationUserSnapshot{
		ID: userID, Name: rows[0]["name"], Account: rows[0]["username"], Phone: rows[0]["tel_number"], Remark: rows[0]["remark"], Avatar: rows[0]["avatar"],
	}, nil
}

// automationRelativeSnapshots captures the parent requirement or first child related to the trigger work item.
func automationRelativeSnapshots(adminID, workspaceID int, trigger *AutomationWorkItemSnapshot) (*AutomationWorkItemSnapshot, *AutomationWorkItemSnapshot, error) {
	var parent *AutomationWorkItemSnapshot
	var err error
	if trigger.ParentID > 0 {
		parent, err = CaptureAutomationSnapshot(adminID, workspaceID, trigger.ParentID)
		if err != nil {
			return nil, nil, err
		}
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM requirements WHERE admin_id=$1 AND workspace_id=$2
		AND parent_id=$3 AND type=$4 ORDER BY created_at ASC,id ASC LIMIT 1`, nil, adminID, workspaceID, trigger.ID, define.TypeKeyRequirement)
	if err != nil {
		return nil, nil, err
	}
	var child *AutomationWorkItemSnapshot
	if len(rows) > 0 {
		child, err = CaptureAutomationSnapshot(adminID, workspaceID, cast.ToInt(rows[0]["id"]))
	}
	return parent, child, err
}

// automationTriggerMatches reports whether a raw business event matches the rule trigger.
func automationTriggerMatches(trigger AutomationTrigger, event AutomationEvent) bool {
	if trigger.Object != event.ItemType {
		return false
	}
	switch trigger.Event {
	case "create":
		return event.Operation == "create"
	case "comment":
		return event.Operation == "comment" && (trigger.CommentChange == "any" || trigger.CommentChange == event.CommentChange)
	case "status":
		if event.Operation != "update" || event.Before == nil || event.After == nil {
			return false
		}
		before, after := cast.ToInt(event.Before.Fields["status"]), cast.ToInt(event.After.Fields["status"])
		return before != after && (trigger.FromStatusID == 0 || trigger.FromStatusID == before) && trigger.ToStatusID == after
	case "field":
		if event.Operation != "update" || event.Before == nil || event.After == nil {
			return false
		}
		return matchAutomationFieldChange(trigger.FieldChange, rawJSONValue(trigger.FieldValue), event.Before.Fields[trigger.FieldKey], event.After.Fields[trigger.FieldKey])
	default:
		return false
	}
}

// automationFields merges built-in fields and enabled custom fields of a work item.
func automationFields(adminID, workspaceID int, itemType, lang string) ([]AutomationFieldMeta, error) {
	labels := map[string]string{
		"id": "workspace_field_id", "title": "workspace_field_title", "description": "workspace_field_description",
		"status": "workspace_field_status", "version": "workspace_field_iteration", "priority": "workspace_field_priority",
		"planned_start_date": "workspace_field_planned_start_date", "planned_end_date": "workspace_field_planned_end_date",
		"assignee": "workspace_field_assignee", "created_at": "workspace_field_created_time", "created_by": "workspace_field_created_by",
		"category":             "workspace_field_category",
		"estimated_work_hours": "workspace_field_estimated_work_hours", "work_progress": "workspace_field_work_progress",
		"completed_work_hours": "workspace_field_completed_work_hours", "remaining_work_hours": "workspace_field_remaining_work_hours",
		"overtime_work_hours": "workspace_field_overtime_work_hours",
	}
	types := map[string]string{"id": "number", "title": "text", "description": "textarea", "status": "select_single", "version": "select_single", "priority": "select_single", "planned_start_date": "date", "planned_end_date": "date", "assignee": "user", "created_at": "date", "created_by": "user", "category": "select_single", "estimated_work_hours": "number", "work_progress": "number", "completed_work_hours": "number", "remaining_work_hours": "number", "overtime_work_hours": "number"}
	configModel, err := ListFieldConfigs(ListFieldConfigsModel{
		AdminId: adminID, WsId: cast.ToString(workspaceID), TypeKey: itemType, Lang: lang,
	})
	if err != nil {
		return nil, err
	}
	result := []AutomationFieldMeta{}
	for _, config := range configModel.Items {
		if config.Enabled != 1 {
			continue
		}
		key := config.Key
		if key == "status_id" {
			key = "status"
		}
		result = append(result, AutomationFieldMeta{Key: key, Label: lib_i18n.Show(lang, labels[key]), Type: types[key], Editable: (config.Editable == 1 || key == "title") && key != "status" && key != "id" && key != "created_at" && key != "created_by"})
	}
	// description is stored on the work item even if older workspaces have no field-config row for it.
	foundDescription := false
	for _, item := range result {
		if item.Key == "description" {
			foundDescription = true
		}
	}
	if !foundDescription {
		result = append(result, AutomationFieldMeta{Key: "description", Label: lib_i18n.Show(lang, "workspace_field_description"), Type: "textarea", Editable: true})
	}
	customModel, err := ListCustomFields(ListCustomFieldsModel{
		AdminId: adminID, WsId: cast.ToString(workspaceID), TypeKey: itemType,
	})
	if err != nil {
		return nil, err
	}
	for _, field := range customModel.Items {
		if field.IsEnabled == 1 {
			result = append(result, AutomationFieldMeta{Key: field.Key, Label: field.Name, Type: field.FieldType, Editable: true, Options: field.Options, MemberScope: field.MemberScope, RoleIds: field.RoleIds})
		}
	}
	return result, nil
}

// automationFieldMap indexes field metadata by stable key.
func automationFieldMap(fields []AutomationFieldMeta) map[string]AutomationFieldMeta {
	result := make(map[string]AutomationFieldMeta, len(fields))
	for _, field := range fields {
		result[field.Key] = field
	}
	return result
}

// validateAutomationReferences validates all external references before saving or copying.
func validateAutomationReferences(adminID, workspaceID, creatorID int, config *AutomationConfig) error {
	triggerFields, err := automationFields(adminID, workspaceID, config.Trigger.Object, lib_i18n.LangZhCn)
	if err != nil {
		return err
	}
	if automationHasCascaderJudgment(*config, triggerFields) {
		return ErrAutomationConfigInvalid
	}

	requirementFields, err := automationFields(adminID, workspaceID, define.TypeKeyRequirement, lib_i18n.LangZhCn)
	if err != nil {
		return err
	}
	triggerMap, requirementMap := automationFieldMap(triggerFields), automationFieldMap(requirementFields)
	var memberScope *customFieldMemberScope
	loadMemberScope := func() error {
		if memberScope != nil {
			return nil
		}
		var loadErr error
		memberScope, loadErr = loadAutomationMemberScope(adminID, workspaceID)
		return loadErr
	}
	if config.Trigger.Event == "field" {
		field, ok := triggerMap[config.Trigger.FieldKey]
		if !ok || !field.Editable {
			return ErrAutomationConfigInvalid
		}
		if config.Trigger.FieldChange != "any" && field.Type == "datetime" {
			var text string
			if json.Unmarshal(config.Trigger.FieldValue, &text) != nil {
				return ErrAutomationConfigInvalid
			}
			normalized, ok := normalizeAutomationDatetime(text)
			if !ok {
				return ErrAutomationConfigInvalid
			}
			config.Trigger.FieldValue, err = json.Marshal(normalized)
			if err != nil {
				return err
			}
		}
		if config.Trigger.FieldChange != "any" && customFieldIsMember(field.Type) && !oneOf(field.Key, "assignee", "created_by") {
			if err := loadMemberScope(); err != nil {
				return err
			}
			if !automationMemberValueValid(field, config.Trigger.FieldValue, memberScope) {
				return ErrAutomationConfigInvalid
			}
		}
	}
	if config.Trigger.Event == "status" {
		if !automationStatusExists(adminID, workspaceID, config.Trigger.Object, config.Trigger.ToStatusID) || (config.Trigger.FromStatusID > 0 && !automationStatusExists(adminID, workspaceID, config.Trigger.Object, config.Trigger.FromStatusID)) {
			return ErrAutomationConfigInvalid
		}
	}
	allowedVariables := automationVariableSet(triggerFields, requirementFields, config.Trigger.Event)
	for _, node := range config.Nodes {
		branches := []AutomationConditionBranch{}
		if node.Mode == "conditional" {
			branches = automationConditionBranches(node)
		}
		for _, branch := range branches {
			for _, module := range branch.Modules {
				for _, condition := range module.Conditions {
					field, ok := triggerMap[condition.FieldKey]
					if !ok || !automationOperatorAllowed(field.Type, condition.Operator) {
						return ErrAutomationConfigInvalid
					}
					if field.Type == "datetime" && !oneOf(condition.Operator, "empty", "not_empty") && !automationDatetimeValueValid(condition.Value) {
						return ErrAutomationConfigInvalid
					}
					if customFieldIsMember(field.Type) && !oneOf(field.Key, "assignee", "created_by") && !oneOf(condition.Operator, "empty", "not_empty") {
						if err := loadMemberScope(); err != nil {
							return err
						}
						if !automationMemberConditionValid(field, condition, memberScope) {
							return ErrAutomationConfigInvalid
						}
					}
				}
			}
			for _, action := range branch.Actions {
				if err = validateAutomationActionReferences(adminID, workspaceID, creatorID, config.Trigger.Object, action, triggerMap, requirementMap, allowedVariables); err != nil {
					return err
				}
			}
		}
		if node.Action != nil {
			if err = validateAutomationActionReferences(adminID, workspaceID, creatorID, config.Trigger.Object, *node.Action, triggerMap, requirementMap, allowedVariables); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateAutomationActionReferences validates the statuses, fields, variables and target projects referenced by an action.
func validateAutomationActionReferences(adminID, workspaceID, creatorID int, triggerType string, action AutomationAction, triggerFields, requirementFields map[string]AutomationFieldMeta, variables map[string]bool) error {
	targetFields := triggerFields
	if action.Scope == "parent" || action.Scope == "descendants" {
		targetFields = requirementFields
	}
	switch action.Type {
	case "status_transition":
		for _, transition := range action.Transitions {
			targetType := triggerType
			if transition.Scope != "trigger" {
				targetType = define.TypeKeyRequirement
			}
			if !automationStatusExists(adminID, workspaceID, targetType, transition.TargetStatusID) {
				return ErrAutomationConfigInvalid
			}
		}
	case "field_update":
		for _, update := range action.Updates {
			target, ok := targetFields[update.FieldKey]
			if !ok || !target.Editable {
				return ErrAutomationConfigInvalid
			}
			if update.ValueSource == "fixed" {
				targetType := triggerType
				if action.Scope != "trigger" {
					targetType = define.TypeKeyRequirement
				}
				fixed := rawJSONValue(update.Value)
				if !validateAutomationFixedValue(adminID, workspaceID, targetType, target, fixed) {
					return ErrAutomationConfigInvalid
				}
				// Text fields accept the same template variables as comment and webhook templates.
				if text, ok := fixed.(string); ok && oneOf(target.Type, "text", "textarea") {
					if err := validateTemplateVariables(text, variables); err != nil {
						return err
					}
				}
			} else {
				sourceFields := triggerFields
				if update.ValueSource == "parent" || update.ValueSource == "child" {
					sourceFields = requirementFields
				}
				source, exists := sourceFields[update.SourceFieldKey]
				if !exists || !automationFieldTypesCompatible(target.Type, source.Type) {
					return ErrAutomationConfigInvalid
				}
			}
		}
	case "add_comment", "create_requirement":
		if err := validateTemplateVariables(action.Template, variables); err != nil {
			return err
		}
		if action.Type == "create_requirement" {
			if triggerType == define.TypeKeyDefect && action.Parent == "trigger" {
				return ErrAutomationConfigInvalid
			}
			if err := validateAutomationCreateFields(adminID, workspaceID, action, variables); err != nil {
				return err
			}
		}
	case "copy_work_item":
		if action.TargetWorkspaceID != workspaceID && action.Parent == "trigger" {
			return ErrAutomationConfigInvalid
		}
		if !HasPermission(adminID, creatorID, cast.ToString(action.TargetWorkspaceID), triggerType+":create") || !automationWorkspaceSupportsType(adminID, action.TargetWorkspaceID, triggerType) {
			return ErrAutomationConfigInvalid
		}
	case "webhook":
		if err := validateWebhookURLTemplate(action.URL, variables); err != nil {
			return err
		}
		for _, template := range []string{action.Body, action.Template} {
			if template == "" {
				continue
			}
			if err := validateTemplateVariables(template, variables); err != nil {
				return err
			}
		}
		if !automationMembersExist(adminID, workspaceID, action.AtUserIds) {
			return ErrAutomationConfigInvalid
		}
	}
	return nil
}

// validateAutomationCreateFields validates the description variables and fixed field references of a create_requirement action.
func validateAutomationCreateFields(adminID, workspaceID int, action AutomationAction, variables map[string]bool) error {
	if err := validateTemplateVariables(action.Description, variables); err != nil {
		return err
	}
	if action.Status != nil && action.Status.Source == "fixed" && !automationStatusExists(adminID, workspaceID, define.TypeKeyRequirement, action.Status.Value) {
		return ErrAutomationConfigInvalid
	}
	checks := []struct {
		key   string
		field *AutomationCreateField
	}{{"priority", action.Priority}, {"version", action.Version}, {"assignee", action.Assignee}}
	for _, item := range checks {
		if item.field == nil || item.field.Source != "fixed" {
			continue
		}
		value := any(item.field.Value)
		if item.key == "assignee" {
			value = item.field.Values
		}
		if !validateAutomationFixedValue(adminID, workspaceID, define.TypeKeyRequirement, AutomationFieldMeta{Key: item.key}, value) {
			return ErrAutomationConfigInvalid
		}
	}
	return nil
}

// automationMembersExist checks whether every mentioned user is still a member of the tenant workspace.
func automationMembersExist(adminID, workspaceID int, userIDs []int) bool {
	if len(userIDs) == 0 {
		return true
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT COUNT(DISTINCT user_id) AS total FROM workspace_members
		WHERE admin_id=$1 AND workspace_id=$2 AND user_id=ANY($3)`, nil, adminID, workspaceID, pq.Array(userIDs))
	if err != nil {
		logs.Error("count automation mentioned members failed: %s", err.Error())
		return false
	}
	unique := map[int]bool{}
	for _, userID := range userIDs {
		unique[userID] = true
	}
	return cast.ToInt(rows[0]["total"]) == len(unique)
}

// automationStatusExists checks whether a status belongs to the current tenant workspace and work item type.
func automationStatusExists(adminID, workspaceID int, itemType string, statusID int) bool {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspace_statuses WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`, nil, adminID, workspaceID, itemType, statusID)
	return err == nil && len(rows) > 0
}

// automationWorkspaceSupportsType checks whether a target workspace supports the given work item type.
func automationWorkspaceSupportsType(adminID, workspaceID int, itemType string) bool {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM workspace_statuses WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 LIMIT 1`, nil, adminID, workspaceID, itemType)
	return err == nil && len(rows) > 0
}

// automationOperatorAllowed checks whether a field type supports the given condition operator.
func automationOperatorAllowed(fieldType, operator string) bool {
	if fieldType == "phone" {
		fieldType = "text"
	}
	if fieldType == "datetime" {
		fieldType = "date"
	}
	if fieldType == "numeric" {
		fieldType = "number"
	}
	allowed := map[string]map[string]bool{
		"text":          {"contains": true, "not_contains": true, "is": true, "is_not": true, "empty": true, "not_empty": true},
		"textarea":      {"contains": true, "not_contains": true, "is": true, "is_not": true, "empty": true, "not_empty": true},
		"number":        {"eq": true, "neq": true, "gt": true, "gte": true, "lt": true, "lte": true, "empty": true, "not_empty": true},
		"date":          {"before": true, "before_eq": true, "after": true, "after_eq": true, "is": true, "is_not": true, "empty": true, "not_empty": true},
		"user":          {"is": true, "is_not": true, "in": true, "not_in": true, "empty": true, "not_empty": true},
		"select_single": {"is": true, "is_not": true, "in": true, "not_in": true, "empty": true, "not_empty": true},
		"user_multi":    {"contains_any": true, "contains_all": true, "contains_none": true, "empty": true, "not_empty": true},
		"select_multi":  {"contains_any": true, "contains_all": true, "contains_none": true, "empty": true, "not_empty": true},
	}
	return allowed[fieldType][operator]
}

// validateAutomationFixedValue validates that a fixed value and its referenced object are still valid.
func validateAutomationFixedValue(adminID, workspaceID int, itemType string, field AutomationFieldMeta, value any) bool {
	if automationValueEmpty(value) {
		return false
	}
	if strings.HasPrefix(field.Key, "cf_") {
		fields, err := ListCustomFields(ListCustomFieldsModel{AdminId: adminID, WsId: cast.ToString(workspaceID), TypeKey: itemType})
		if err != nil {
			logs.Error("load automation fixed fields failed: %s", err.Error())
			return false
		}
		for _, custom := range fields.Items {
			if custom.Key != field.Key || custom.IsEnabled != 1 {
				continue
			}
			if custom.FieldType == "date" {
				break
			}
			text, err := automationStoredValue(value)
			if err != nil {
				return false
			}
			valid, err := automationCustomValueValid(adminID, workspaceID, custom, text)
			if err != nil {
				logs.Error("validate automation fixed field failed: %s", err.Error())
			}
			return err == nil && valid
		}
		if field.Type != "date" {
			return false
		}
	}

	if field.Type == "number" {
		_, ok := automationNumber(value)
		return ok
	}
	if field.Type == "date" {
		keyword, ok := value.(string)
		if !ok {
			return false
		}
		_, exists := automationDateOffsets[keyword]
		return exists || keyword == automationDateClear
	}
	if len(field.Options) > 0 {
		allowed := make(map[string]bool, len(field.Options))
		for _, option := range field.Options {
			allowed[option.Key] = true
		}
		if field.Type == "select_multi" {
			selected, ok := automationStringSet(value)
			if !ok {
				return false
			}
			for key := range selected {
				if !allowed[key] {
					return false
				}
			}
			return true
		}
		return allowed[fmt.Sprint(value)]
	}
	var rows []msql.Params
	var err error
	switch field.Key {
	case "priority":
		rows, err = msql.RawValues(define.Postgres, `SELECT id FROM workspace_priorities WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`, nil, adminID, workspaceID, itemType, cast.ToInt(value))
	case "assignee":
		// The assignee field keeps every selected member, so all of them must belong to the workspace.
		assigneeIDs := automationAssigneeIDs(value)
		if len(assigneeIDs) == 0 {
			return false
		}
		rows, err = msql.RawValues(define.Postgres, `SELECT user_id FROM workspace_members WHERE admin_id=$1 AND workspace_id=$2 AND user_id=ANY($3)`, nil, adminID, workspaceID, pq.Array(assigneeIDs))
		return err == nil && len(rows) == len(assigneeIDs)
	case "version":
		rows, err = msql.RawValues(define.Postgres, `SELECT id FROM versions WHERE admin_id=$1 AND workspace_id=$2 AND id=$3`, nil, adminID, workspaceID, cast.ToInt(value))
	default:
		return true
	}
	return err == nil && len(rows) > 0
}

// automationFieldTypesCompatible reports whether a source field can be assigned to a target field.
func automationFieldTypesCompatible(target, source string) bool {
	if (target == "text" || target == "textarea") && (source == "text" || source == "textarea") {
		return true
	}
	return target == source
}

// automationFieldVariableKeys returns the template variable keys for a field.
// user-typed fields split into profile variables; others stay single.
func automationFieldVariableKeys(field AutomationFieldMeta, prefix string) []string {
	base := prefix + ".field." + field.Key
	if customFieldIsMember(field.Type) {
		keys := []string{base + ".name", base + ".account", base + ".phone", base + ".remark"}
		if field.Type == "user_multi" {
			keys = append(keys, base)
		}
		return keys
	}
	return []string{base}
}

// automationFieldVariableMetadata returns the frontend variable metadata for a field.
// user-typed fields include the source field label so assignee and creator variables remain distinguishable.
func automationFieldVariableMetadata(field AutomationFieldMeta, prefix, group, lang string) []AutomationVariableModel {
	// id, title and description already have a fixed variable in every scope, so the field copies stay hidden.
	if oneOf(field.Key, "id", "title", "description") {
		return nil
	}
	base := prefix + ".field." + field.Key
	if customFieldIsMember(field.Type) {
		label := func(key string) string { return field.Label + " · " + lib_i18n.Show(lang, key) }
		items := []AutomationVariableModel{
			{Key: base + ".name", Group: group, Label: label("automation_variable_user_name")},
			{Key: base + ".account", Group: group, Label: label("automation_variable_user_account")},
			{Key: base + ".phone", Group: group, Label: label("automation_variable_user_phone")},
			{Key: base + ".remark", Group: group, Label: label("automation_variable_user_remark")},
		}
		if field.Type == "user_multi" {
			items = append(items, AutomationVariableModel{Key: base, Group: group, Label: field.Label})
		}
		return items
	}
	return []AutomationVariableModel{{Key: base, Group: group, Label: field.Label}}
}

// automationVariableSet builds the set of template variables allowed for use.
func automationVariableSet(triggerFields, requirementFields []AutomationFieldMeta, triggerEvent string) map[string]bool {
	result := map[string]bool{"rule.name": true, "execution.id": true, "event.time": true, "triggered_by.id": true, "triggered_by.name": true, "triggered_by.account": true, "triggered_by.phone": true, "triggered_by.remark": true, "trigger.id": true, "trigger.link": true, "trigger.title": true, "trigger.description": true, "parent.id": true, "parent.link": true, "parent.title": true, "parent.description": true}
	for _, field := range triggerFields {
		for _, key := range automationFieldVariableKeys(field, "trigger") {
			result[key] = true
		}
	}
	for _, field := range requirementFields {
		for _, key := range automationFieldVariableKeys(field, "parent") {
			result[key] = true
		}
		if triggerEvent != "create" {
			for _, key := range automationFieldVariableKeys(field, "child") {
				result[key] = true
			}
		}
	}
	if triggerEvent != "create" {
		result["child.id"] = true
		result["child.link"] = true
		result["child.title"] = true
		result["child.description"] = true
	}
	return result
}

// automationVariableMetadata returns the names and keys for the frontend variable selector.
func automationVariableMetadata(triggerFields, requirementFields []AutomationFieldMeta, lang string) []AutomationVariableModel {
	result := []AutomationVariableModel{
		{Key: "triggered_by.id", Group: "triggered_by", Label: lib_i18n.Show(lang, "automation_variable_id")},
		{Key: "triggered_by.name", Group: "triggered_by", Label: lib_i18n.Show(lang, "automation_variable_user_name")},
		{Key: "triggered_by.account", Group: "triggered_by", Label: lib_i18n.Show(lang, "automation_variable_user_account")},
		{Key: "triggered_by.phone", Group: "triggered_by", Label: lib_i18n.Show(lang, "automation_variable_user_phone")},
		{Key: "triggered_by.remark", Group: "triggered_by", Label: lib_i18n.Show(lang, "automation_variable_user_remark")},
		{Key: "event.time", Group: "event", Label: lib_i18n.Show(lang, "automation_variable_event_time")},
		{Key: "trigger.id", Group: "trigger", Label: lib_i18n.Show(lang, "automation_variable_id")},
		{Key: "trigger.link", Group: "trigger", Label: lib_i18n.Show(lang, "automation_variable_link")},
		{Key: "trigger.title", Group: "trigger", Label: lib_i18n.Show(lang, "automation_variable_title")},
		{Key: "trigger.description", Group: "trigger", Label: lib_i18n.Show(lang, "automation_variable_description")},
	}
	for _, field := range triggerFields {
		result = append(result, automationFieldVariableMetadata(field, "trigger", "trigger", lang)...)
	}
	for _, prefix := range []string{"parent", "child"} {
		result = append(result,
			AutomationVariableModel{Key: prefix + ".id", Group: prefix, Label: lib_i18n.Show(lang, "automation_variable_id")},
			AutomationVariableModel{Key: prefix + ".link", Group: prefix, Label: lib_i18n.Show(lang, "automation_variable_link")},
			AutomationVariableModel{Key: prefix + ".title", Group: prefix, Label: lib_i18n.Show(lang, "automation_variable_title")},
			AutomationVariableModel{Key: prefix + ".description", Group: prefix, Label: lib_i18n.Show(lang, "automation_variable_description")},
		)
		for _, field := range requirementFields {
			result = append(result, automationFieldVariableMetadata(field, prefix, prefix, lang)...)
		}
	}
	result = append(result,
		AutomationVariableModel{Key: "rule.name", Group: "rule", Label: lib_i18n.Show(lang, "automation_variable_rule_name")},
		AutomationVariableModel{Key: "execution.id", Group: "execution", Label: lib_i18n.Show(lang, "automation_variable_execution_id")},
	)
	return result
}

// automationCopyTargets returns the target workspaces where the rule creator has create permission.
func automationCopyTargets(adminID, creatorID int, itemType string) ([]AutomationCopyTargetModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id,name FROM workspaces WHERE admin_id=$1 AND COALESCE(ended_at, 0) = 0 ORDER BY created_at ASC,id ASC`, nil, adminID)
	if err != nil {
		return nil, err
	}
	result := []AutomationCopyTargetModel{}
	for _, row := range rows {
		workspaceID, err := strconv.Atoi(row["id"])
		if err != nil {
			return nil, fmt.Errorf("parse automation copy target id: %w", err)
		}
		if HasPermission(adminID, creatorID, row["id"], itemType+":create") && automationWorkspaceSupportsType(adminID, workspaceID, itemType) {
			result = append(result, AutomationCopyTargetModel{Id: workspaceID, Name: row["name"]})
		}
	}
	return result, nil
}

// automationTxRunner lets node side effects and checkpoints share the same transaction.
type automationTxRunner func(*sql.Tx) (bool, string, error)

// ErrAutomationMemberInvalid means a mapped user is not a member of the workspace.
var ErrAutomationMemberInvalid = errors.New("automation member invalid")

// automationMemberConditionValid 保证成员条件的值形状及候选范围与字段配置一致。
func automationMemberConditionValid(field AutomationFieldMeta, condition AutomationCondition, scope *customFieldMemberScope) bool {
	if field.Type == "user" && oneOf(condition.Operator, "in", "not_in") {
		field.Type = "user_multi"
	}
	return automationMemberValueValid(field, condition.Value, scope)
}

// automationMemberValueValid 校验触发器与条件共用的成员值形状及字段候选范围。
func automationMemberValueValid(field AutomationFieldMeta, value json.RawMessage, scope *customFieldMemberScope) bool {
	ids := []int{}
	if field.Type == "user_multi" {
		if json.Unmarshal(value, &ids) != nil || len(ids) == 0 {
			return false
		}
	} else {
		var id int
		if json.Unmarshal(value, &id) != nil {
			return false
		}
		ids = append(ids, id)
	}
	custom := CustomFieldModel{MemberScope: field.MemberScope, RoleIds: field.RoleIds}
	for _, id := range ids {
		if id <= 0 || !scope.allows(custom, id) {
			return false
		}
	}
	return true
}

// loadAutomationMemberScope 与编辑器共用有效成员范围，停用和移除成员不参与写入。
func loadAutomationMemberScope(adminID, workspaceID int) (*customFieldMemberScope, error) {
	metadata, err := GetWorkspaceMetadata(WorkspaceMetadataModel{AdminId: adminID, WorkspaceId: cast.ToString(workspaceID), Lang: lib_i18n.LangZhCn})
	if err != nil {
		return nil, err
	}
	scope := &customFieldMemberScope{enterpriseRoleIds: map[int]int{}, workspaceMemberIds: map[int]bool{}}
	for _, member := range metadata.CustomFieldMembers {
		if member.Removed || member.UserStatus == 0 {
			continue
		}
		scope.enterpriseRoleIds[member.UserId] = member.EnterpriseRoleId
		scope.workspaceMemberIds[member.UserId] = member.IsWorkspaceMember
	}
	return scope, nil
}

// automationCustomValueValid 在保存、更新和跨工作空间复制时使用目标字段配置校验。
func automationCustomValueValid(adminID, workspaceID int, field CustomFieldModel, text string) (bool, error) {
	if field.FieldType == "datetime" && text != "" && !automationDatetimeValueValid(json.RawMessage(strconv.Quote(text))) {
		return false, nil
	}
	if text == "null" && !oneOf(field.FieldType, "text", "textarea") {
		return false, nil
	}
	if customFieldIsMember(field.FieldType) && text != "" && text != "[]" {
		ids := []string{text}
		if field.FieldType == "user_multi" {
			var err error
			ids, err = decodeStringSlice(text)
			if err != nil {
				return false, nil
			}
		}
		for _, textID := range ids {
			id, err := strconv.Atoi(textID)
			if err != nil || id <= 0 {
				return false, nil
			}
		}
	}

	var scope *customFieldMemberScope
	if customFieldIsMember(field.FieldType) {
		var err error
		scope, err = loadAutomationMemberScope(adminID, workspaceID)
		if err != nil {
			return false, err
		}
	}
	return validateCustomFieldValue(field, text, scope) == nil, nil
}
