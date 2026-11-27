package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// AutoAssignmentConfig 是持久化的自动分配配置，与自动化共用条件协议。
type AutoAssignmentConfig struct {
	Rules []AutoAssignmentRule `json:"rules"`
}

// AutoAssignmentRule 按创建时间降序匹配，ID 用于保存独立的轮流分配游标。
type AutoAssignmentRule struct {
	UpdatedAt      int64                       `json:"updated_at"`
	CreatedAt      int64                       `json:"created_at"`
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Enabled        bool                        `json:"enabled"`
	ModuleRelation string                      `json:"module_relation"`
	Modules        []AutomationConditionModule `json:"modules"`
	Mode           string                      `json:"mode"`
	Target         string                      `json:"target"`
	MemberIDs      []int                       `json:"member_ids"`
	DepartmentIDs  []int                       `json:"department_ids"`
}

// AutoAssignmentModel 是读取和保存工作空间分配配置的用例。
type AutoAssignmentModel struct {
	AdminID     int
	WorkspaceID int
	UserID      int
	Revision    int
	Config      json.RawMessage
}

// ErrAutoAssignmentConflict 表示配置已被另一位管理员更新。
var ErrAutoAssignmentConflict = errors.New("auto assignment revision conflict")

// GetAutoAssignment 读取配置，未配置的工作空间返回关闭的空规则集。
func GetAutoAssignment(model AutoAssignmentModel) (AutoAssignmentModel, error) {
	issue, err := IsIssueWorkspace(model.AdminID, model.WorkspaceID)
	if err != nil {
		return model, err
	}
	if !issue {
		return model, ErrAutomationConfigInvalid
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT config, revision FROM `+define.TableWorkItemAutoAssignment+` WHERE admin_id=$1 AND workspace_id=$2`, nil, model.AdminID, model.WorkspaceID)
	if err != nil {
		logs.Error("get auto assignment sql:%s err:%s", define.TableWorkItemAutoAssignment, err.Error())
		return model, err
	}
	model.Config = json.RawMessage(`{"rules":[]}`)
	if len(rows) > 0 {
		stored := strings.TrimSpace(rows[0]["config"])
		var config AutoAssignmentConfig
		if len(stored) == 0 || stored[0] != '{' || json.Unmarshal([]byte(stored), &config) != nil {
			return model, ErrAutomationConfigInvalid
		}
		if config.Rules == nil {
			config.Rules = []AutoAssignmentRule{}
		}
		sortAutoAssignmentRules(config.Rules)
		model.Config, err = json.Marshal(config)
		if err != nil {
			return model, err
		}
		model.Revision, err = strconv.Atoi(rows[0]["revision"])
	}
	return model, err
}

// SaveAutoAssignment 通过修订号防止覆盖其他管理员的编辑，保留运行游标。
func SaveAutoAssignment(model AutoAssignmentModel) (AutoAssignmentModel, error) {
	stored, err := GetAutoAssignment(model)
	if err != nil {
		return model, err
	}
	if stored.Revision != model.Revision {
		return model, ErrAutoAssignmentConflict
	}
	config, parseErr := parseAutoAssignmentConfig(model.Config)
	if parseErr != nil || model.Revision < 0 {
		return model, ErrAutomationConfigInvalid
	}
	active, err := autoAssignmentActiveMembers(nil, model.AdminID, model.WorkspaceID)
	if err != nil {
		return model, err
	}
	seen := map[string]bool{}
	for _, rule := range config.Rules {
		if rule.ID == "" || len(rule.ID) > 64 || seen[rule.ID] || strings.TrimSpace(rule.Name) == "" || utf8.RuneCountInString(rule.Name) > 20 || !oneOf(rule.Mode, "all", "round_robin") || !oneOf(rule.Target, "members", "departments") {
			return model, ErrAutomationConfigInvalid
		}
		seen[rule.ID] = true
		if err := validateAutomationConditionModules(rule.ModuleRelation, rule.Modules); err != nil {
			return model, err
		}
		if !rule.Enabled {
			continue
		}
		cfg := AutomationConfig{Trigger: AutomationTrigger{Object: define.TypeKeyRequirement, Event: "create"}, Nodes: []AutomationFlowNode{{Mode: "conditional", ModuleRelation: rule.ModuleRelation, Modules: rule.Modules}}}
		if err := validateAutomationReferences(model.AdminID, model.WorkspaceID, model.UserID, &cfg); err != nil {
			return model, err
		}
		if rule.Target == "members" {
			if len(rule.MemberIDs) == 0 {
				return model, ErrAutomationConfigInvalid
			}
			for _, id := range rule.MemberIDs {
				if !active[id] {
					return model, ErrAutomationConfigInvalid
				}
			}
		} else {
			if len(rule.DepartmentIDs) == 0 {
				return model, ErrAutomationConfigInvalid
			}
			rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableDepartments+` WHERE admin_id=$1 AND id=ANY($2)`, nil, model.AdminID, pq.Array(rule.DepartmentIDs))
			if err != nil {
				return model, err
			}
			ids := map[int]bool{}
			for _, r := range rows {
				id, e := strconv.Atoi(r["id"])
				if e != nil {
					return model, e
				}
				ids[id] = true
			}
			for _, id := range rule.DepartmentIDs {
				if !ids[id] {
					return model, ErrAutomationConfigInvalid
				}
			}
		}
	}
	var previous AutoAssignmentConfig
	if err := json.Unmarshal(stored.Config, &previous); err != nil {
		return model, err
	}
	prepareAutoAssignmentRules(&config, previous, time.Now().UnixMilli())
	data, err := json.Marshal(config)
	if err != nil {
		return model, err
	}
	var rows []msql.Params
	if model.Revision == 0 {
		rows, err = msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkItemAutoAssignment+` (admin_id,workspace_id,config,revision) SELECT $1,$2,$3,1 WHERE $4=0
 ON CONFLICT (admin_id,workspace_id) DO UPDATE SET config=EXCLUDED.config,revision=`+define.TableWorkItemAutoAssignment+`.revision+1 WHERE `+define.TableWorkItemAutoAssignment+`.revision=$4 RETURNING revision`, nil, model.AdminID, model.WorkspaceID, string(data), model.Revision)
	}
	if model.Revision > 0 {
		rows, err = msql.RawValues(define.Postgres, `UPDATE `+define.TableWorkItemAutoAssignment+` SET config=$3,revision=revision+1 WHERE admin_id=$1 AND workspace_id=$2 AND revision=$4 RETURNING revision`, nil, model.AdminID, model.WorkspaceID, string(data), model.Revision)
	}
	if err != nil {
		logs.Error("save auto assignment sql:%s err:%s", define.TableWorkItemAutoAssignment, err.Error())
		return model, err
	}
	if len(rows) == 0 {
		return model, ErrAutoAssignmentConflict
	}
	model.Revision, err = strconv.Atoi(rows[0]["revision"])
	model.Config = data
	return model, err
}

// applyAutoAssignmentTx 在创建事务内完成匹配、分配和游标推进，回滚时一并撤销。
// ponytail: 同一工作空间的有效分配串行；压测达到瓶颈时再拆分按规则的游标锁。
func applyAutoAssignmentTx(tx *sql.Tx, adminID, workspaceID, itemID int) (recipients []int, err error) {
	defer func() {
		if err != nil {
			logs.Error("auto assignment sql tables:%s,%s admin_id:%d workspace_id:%d item_id:%d err:%s", define.TableWorkItemAutoAssignment, define.TableRequirements, adminID, workspaceID, itemID, err.Error())
		}
	}()

	rows, err := msql.RawValues(define.Postgres, `SELECT a.config,a.cursors FROM `+define.TableWorkItemAutoAssignment+` a JOIN `+define.TableWorkspaces+` w ON w.admin_id=a.admin_id AND w.id=a.workspace_id WHERE a.admin_id=$1 AND a.workspace_id=$2 AND w.workspace_type='issue' AND EXISTS(SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(a.config->'rules')='array' THEN a.config->'rules' ELSE '[]'::jsonb END) rule WHERE (rule->>'enabled')::boolean=true) FOR UPDATE OF a`, tx, adminID, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	snapshot, err := captureAutomationSnapshot(tx, adminID, workspaceID, itemID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, errors.New("auto assignment item missing")
	}
	if snapshot.Type != define.TypeKeyRequirement || len(snapshot.AssigneeIDs) > 0 {
		return nil, nil
	}
	var config AutoAssignmentConfig
	if err = json.Unmarshal([]byte(rows[0]["config"]), &config); err != nil {
		return nil, err
	}
	sortAutoAssignmentRules(config.Rules)
	cursors := map[string]int{}
	if err = json.Unmarshal([]byte(rows[0]["cursors"]), &cursors); err != nil {
		return nil, err
	}
	active, err := autoAssignmentActiveMembers(tx, adminID, workspaceID)
	if err != nil {
		return nil, err
	}
	selected := []int{}
	creator := cast.ToInt(snapshot.Fields["created_by"])
	fields, err := automationFieldsTx(tx, adminID, workspaceID, snapshot.Type, "zh-CN")
	if err != nil {
		return nil, err
	}
	types := map[string]string{}
	for _, f := range fields {
		types[f.Key] = f.Type
	}
	for _, rule := range config.Rules {
		if !rule.Enabled {
			continue
		}
		valid := true
		for _, module := range rule.Modules {
			for _, condition := range module.Conditions {
				_, present := snapshot.Fields[condition.FieldKey]
				if !present {
					logs.Error("auto assignment missing snapshot field: workspace_id=%d item_id=%d field=%s", workspaceID, itemID, condition.FieldKey)
				}
				if !present || types[condition.FieldKey] == "" || !automationOperatorAllowed(types[condition.FieldKey], condition.Operator) {
					valid = false
				}
			}
		}
		if !valid {
			continue
		}
		matched, _ := evaluateAutomationModules(AutomationConditionBranch{ModuleRelation: rule.ModuleRelation, Modules: rule.Modules}, snapshot.Fields, types)
		if !matched {
			continue
		}
		candidates := rule.MemberIDs
		if rule.Target == "departments" {
			groupRows, e := msql.RawValues(define.Postgres, `WITH RECURSIVE assignment_groups AS (SELECT id FROM `+define.TableDepartments+` WHERE admin_id=$1 AND id=ANY($2) UNION SELECT d.id FROM `+define.TableDepartments+` d JOIN assignment_groups g ON d.parent_id=g.id WHERE d.admin_id=$1) SELECT DISTINCT ud.user_id FROM `+define.TableUserDepartments+` ud JOIN assignment_groups g ON g.id=ud.department_id WHERE ud.admin_id=$1`, tx, adminID, pq.Array(rule.DepartmentIDs))
			if e != nil {
				return nil, e
			}
			candidates = nil
			for _, r := range groupRows {
				id, e := strconv.Atoi(r["user_id"])
				if e != nil {
					return nil, e
				}
				candidates = append(candidates, id)
			}
		}
		selected = autoAssignmentRecipients(candidates, active, rule.Mode, cursors[rule.ID])
		if len(selected) > 0 && rule.Mode == "round_robin" {
			cursors[rule.ID] = selected[0]
			data, e := json.Marshal(cursors)
			if e != nil {
				return nil, e
			}
			if _, e = msql.RawExec(define.Postgres, `UPDATE `+define.TableWorkItemAutoAssignment+` SET cursors=$3 WHERE admin_id=$1 AND workspace_id=$2`, tx, adminID, workspaceID, string(data)); e != nil {
				return nil, e
			}
		}
		break
	}
	if len(selected) == 0 {
		return nil, nil
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+` SET assignee_ids=$4 WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND cardinality(assignee_ids)=0`, tx, adminID, workspaceID, itemID, pq.Array(selected)); err != nil {
		return nil, err
	}
	if err = createHistoryTx(tx, adminID, workspaceID, itemID, 0, define.HistoryActionAssignChanged, "assignee", "", assigneeIDsText(selected)); err != nil {
		return nil, fmt.Errorf("auto assignment history: %w", err)
	}
	actorName := ""
	actorRows, e := msql.RawValues(define.Postgres, `SELECT COALESCE(NULLIF(name,''),username,'') AS name FROM `+define.TableUsers+` WHERE admin_id=$1 AND id=$2`, tx, adminID, creator)
	if e != nil {
		return nil, e
	}
	if len(actorRows) > 0 {
		actorName = actorRows[0]["name"]
	}
	for _, id := range newAssigneeNotificationRecipients(creator, selected, nil) {
		event := notificationEvent{AdminID: adminID, RecipientID: id, WorkspaceID: workspaceID, ItemID: itemID, EventType: define.NotificationEventItemAssigned, ActorID: creator}
		event.Content = notificationContent(event, actorName, notificationItemType(snapshot.Type))
		if err := CreateNotificationTx(tx, event); err != nil {
			return nil, err
		}
	}
	return selected, nil
}

func autoAssignmentRecipients(candidates []int, active map[int]bool, mode string, last int) []int {
	result := []int{}
	seen := map[int]bool{}
	for _, id := range candidates {
		if active[id] && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	sort.Ints(result)
	if mode != "round_robin" || len(result) == 0 {
		return result
	}
	for _, id := range result {
		if id > last {
			return []int{id}
		}
	}
	return result[:1]
}

func autoAssignmentActiveMembers(tx *sql.Tx, adminID, workspaceID int) (map[int]bool, error) {
	members, err := msql.RawValues(define.Postgres, `SELECT u.id FROM `+define.TableWorkspaceMembers+` m JOIN `+define.TableUsers+` u ON u.admin_id=m.admin_id AND u.id=m.user_id WHERE m.admin_id=$1 AND m.workspace_id=$2 AND u.status=1 AND u.deleted_at=0 ORDER BY u.id`, tx, adminID, workspaceID)
	if err != nil {
		return nil, err
	}
	active := map[int]bool{}
	for _, m := range members {
		id, e := strconv.Atoi(m["id"])
		if e != nil {
			return nil, e
		}
		active[id] = true
	}
	return active, nil
}

func parseAutoAssignmentConfig(data []byte) (AutoAssignmentConfig, error) {
	var config AutoAssignmentConfig
	if json.Unmarshal(data, &config) != nil || config.Rules == nil || len(config.Rules) > 100 {
		return config, ErrAutomationConfigInvalid
	}
	return config, nil
}

// prepareAutoAssignmentRules 保留已有创建时间，新规则时间仅由服务端生成。
func prepareAutoAssignmentRules(config *AutoAssignmentConfig, previous AutoAssignmentConfig, now int64) {
	created := map[string]AutoAssignmentRule{}
	for _, rule := range previous.Rules {
		created[rule.ID] = rule
		if rule.CreatedAt >= now {
			now = rule.CreatedAt + 1
		}
	}
	for i := range config.Rules {
		rule := &config.Rules[i]
		if old, exists := created[rule.ID]; exists {
			rule.CreatedAt = old.CreatedAt
			rule.UpdatedAt = old.UpdatedAt
			if !reflect.DeepEqual(*rule, old) {
				rule.UpdatedAt = now
			}
		} else {
			rule.CreatedAt = now
			rule.UpdatedAt = now
			now++
		}
	}
	sortAutoAssignmentRules(config.Rules)
}

func sortAutoAssignmentRules(rules []AutoAssignmentRule) {
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].CreatedAt > rules[j].CreatedAt })
}
