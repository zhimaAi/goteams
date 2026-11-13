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
	"unicode/utf8"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
)

// IsSLAReferenceConflict 识别数据库原子引用保护，供既有状态/优先级入口返回可控冲突。
func IsSLAReferenceConflict(err error) bool {
	var postgresError *pq.Error
	return errors.Is(err, ErrSLAConflict) || (errors.As(err, &postgresError) && postgresError.Code == "23503" && postgresError.Message == "sla_reference_conflict")
}

func slaRows(tx *sql.Tx, query string, args ...any) ([]msql.Params, error) {
	rows, err := msql.RawValues(define.Postgres, query, tx, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return rows, err
}

func slaExec(tx *sql.Tx, query string, args ...any) error {
	_, err := msql.RawExec(define.Postgres, query, tx, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return err
}

func slaNumber(row msql.Params, key string) (int64, error) {
	value, ok := row[key]
	if !ok {
		return 0, fmt.Errorf("SLA row missing %s", key)
	}
	return strconv.ParseInt(value, 10, 64)
}

func ensureSLAWorkspace(tx *sql.Tx, adminId, workspaceId int) error {
	rows, err := slaRows(tx, "SELECT workspace_type FROM "+define.TableWorkspaces+" WHERE admin_id=$1 AND id=$2", adminId, workspaceId)
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0]["workspace_type"] != define.WorkspaceTypeIssue {
		return ErrSLAWorkspace
	}
	return nil
}

func getSLASettingsTx(tx *sql.Tx, adminId, workspaceId int, exclusive bool) (SLASettingsModel, error) {
	model := SLASettingsModel{AdminId: adminId, WorkspaceId: workspaceId}
	lock := " FOR SHARE"
	if exclusive {
		lock = " FOR UPDATE"
	}
	// 共用设置行是空间控制锁，确保独立 SLA 设置行首次创建时仍与工单事件串行。
	if tx != nil {
		if _, err := slaRows(tx, "SELECT id FROM "+define.TableWorkspaceSettings+" WHERE admin_id=$1 AND workspace_id=$2"+lock, adminId, workspaceId); err != nil {
			return model, err
		}
	}
	query := "SELECT enabled,timezone,activated_at,config_revision,clock_revision FROM " + define.TableSLASettings + " WHERE admin_id=$1 AND workspace_id=$2"
	if tx != nil {
		query += lock
	}
	rows, err := slaRows(tx, query, adminId, workspaceId)
	if err != nil || len(rows) == 0 {
		return model, err
	}
	row := rows[0]
	if model.Enabled, err = strconv.ParseBool(row["enabled"]); err != nil {
		return model, err
	}
	model.Timezone = row["timezone"]
	if model.ActivatedAt, err = slaNumber(row, "activated_at"); err != nil {
		return model, err
	}
	if model.Revision, err = slaNumber(row, "config_revision"); err != nil {
		return model, err
	}
	model.ClockRevision, err = slaNumber(row, "clock_revision")
	return model, err
}

// GetSLASettings 读取工单空间的设置，不创建业务记录。
func GetSLASettings(model SLASettingsModel) (SLASettingsModel, error) {
	if err := ensureSLAWorkspace(nil, model.AdminId, model.WorkspaceId); err != nil {
		return model, err
	}
	result, err := getSLASettingsTx(nil, model.AdminId, model.WorkspaceId, false)
	if err != nil {
		return result, err
	}
	if file, loadErr := loadSLAHolidays(); loadErr == nil {
		for _, calendar := range file.Calendars {
			if calendar.Code == file.DefaultCalendar {
				result.CalendarRegion = calendar.Region
				location, timezoneErr := time.LoadLocation(result.Timezone)
				if timezoneErr != nil {
					location = time.UTC
				}
				for _, year := range calendar.Years {
					if year.Year == time.Now().In(location).Year() {
						result.CalendarAvailable = true
					}
				}
			}
		}
	}
	return result, nil
}

func lockSLAConfig(tx *sql.Tx, adminId, workspaceId int) (SLASettingsModel, error) {
	if err := ensureSLAWorkspace(tx, adminId, workspaceId); err != nil {
		return SLASettingsModel{}, err
	}
	now := time.Now().Unix()
	if err := slaExec(tx, "INSERT INTO "+define.TableWorkspaceSettings+" (admin_id,workspace_id,created_at,updated_at) VALUES($1,$2,$3,$3) ON CONFLICT(admin_id,workspace_id) DO NOTHING", adminId, workspaceId, now); err != nil {
		return SLASettingsModel{}, err
	}
	if _, err := getSLASettingsTx(tx, adminId, workspaceId, true); err != nil {
		return SLASettingsModel{}, err
	}
	if err := slaExec(tx, "INSERT INTO "+define.TableSLASettings+" (admin_id,workspace_id,enabled,timezone,activated_at,created_at,updated_at) VALUES($1,$2,TRUE,'Asia/Shanghai',$3,$3,$3) ON CONFLICT(admin_id,workspace_id) DO NOTHING", adminId, workspaceId, now); err != nil {
		return SLASettingsModel{}, err
	}
	return getSLASettingsTx(tx, adminId, workspaceId, true)
}

// SaveSLASettings 原子保存总开关和时区，关闭期间只保留启停控制区间。
func SaveSLASettings(model SLASettingsModel) (SLASettingsModel, error) {
	if _, err := time.LoadLocation(model.Timezone); err != nil || model.Timezone == "" || model.Timezone == "Local" || len(model.Timezone) > 64 {
		return model, ErrSLAInvalid
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return model, err
	}
	defer func() { _ = tx.Rollback() }()
	old, err := lockSLAConfig(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return model, err
	}
	if model.Revision != old.Revision {
		return model, ErrSLAConflict
	}
	if model.Enabled && !old.Enabled {
		policies, err := listSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, false)
		if err != nil {
			return model, err
		}
		for _, policy := range policies {
			policy.AdminId, policy.WorkspaceId = model.AdminId, model.WorkspaceId
			var config slaPolicyConfig
			if err = json.Unmarshal(policy.Config, &config); err != nil {
				return model, err
			}
			if err = checkSLAPolicyReferences(tx, policy, config); err != nil {
				return model, ErrSLAConflict
			}
		}
	}
	now := time.Now().Unix()
	activated, clock := old.ActivatedAt, old.ClockRevision
	if old.Enabled != model.Enabled {
		clock++
		if model.Enabled {
			if activated == 0 {
				activated = now
			}
			err = slaExec(tx, "UPDATE "+define.TableSLASettings+" SET pauses=COALESCE((SELECT jsonb_agg(CASE WHEN (p->>'end')::BIGINT=0 THEN jsonb_set(p,'{end}',to_jsonb($1::BIGINT)) ELSE p END) FROM jsonb_array_elements(pauses) p),'[]'::JSONB) WHERE admin_id=$2 AND workspace_id=$3", now, model.AdminId, model.WorkspaceId)
		} else if activated > 0 {
			pauseJSON, marshalErr := json.Marshal(slaPause{Start: now})
			if marshalErr != nil {
				return model, marshalErr
			}
			err = slaExec(tx, "UPDATE "+define.TableSLASettings+" SET pauses=pauses || jsonb_build_array($1::JSONB) WHERE admin_id=$2 AND workspace_id=$3", string(pauseJSON), model.AdminId, model.WorkspaceId)
		}
		if err != nil {
			return model, err
		}
	}
	err = slaExec(tx, "UPDATE "+define.TableSLASettings+" SET enabled=$1,timezone=$2,activated_at=$3,config_revision=config_revision+1,clock_revision=$4,updated_at=$5 WHERE admin_id=$6 AND workspace_id=$7", model.Enabled, model.Timezone, activated, clock, now, model.AdminId, model.WorkspaceId)
	if err != nil {
		return model, err
	}
	if model.Enabled && !old.Enabled {
		if err = resumeSLACyclesTx(tx, model.AdminId, model.WorkspaceId, now); err != nil {
			return model, err
		}
	}
	if err = tx.Commit(); err != nil {
		return model, err
	}
	return GetSLASettings(model)
}

func slaDefinitionModel(item slaStoredDefinition) SLADefinitionModel {
	return SLADefinitionModel{Id: item.Id, Disabled: item.Disabled, Name: item.Name, Description: item.Description, SortOrder: item.SortOrder, Revision: item.Revision, PriorityIds: item.PriorityIds, CategoryIds: item.CategoryIds, Config: item.Config}
}

func loadSLADefinitionsTx(tx *sql.Tx, adminId, workspaceId int, stages bool) ([]slaStoredDefinition, error) {
	column := "policies"
	if stages {
		column = "stages"
	}
	rows, err := slaRows(tx, "SELECT "+column+"::text AS definitions FROM "+define.TableSLASettings+" WHERE admin_id=$1 AND workspace_id=$2", adminId, workspaceId)
	if err != nil {
		return nil, err
	}
	items := []slaStoredDefinition{}
	if len(rows) > 0 {
		err = json.Unmarshal([]byte(rows[0]["definitions"]), &items)
	}
	return items, err
}

func saveSLADefinitionsTx(tx *sql.Tx, adminId, workspaceId int, stages bool, items []slaStoredDefinition) error {
	column := "policies"
	if stages {
		column = "stages"
	}
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return slaExec(tx, "UPDATE "+define.TableSLASettings+" SET "+column+"=$1,config_revision=config_revision+1,updated_at=$2 WHERE admin_id=$3 AND workspace_id=$4", string(data), time.Now().Unix(), adminId, workspaceId)
}

func listSLADefinitionsTx(tx *sql.Tx, adminId, workspaceId int, stages bool) ([]SLADefinitionModel, error) {
	stored, err := loadSLADefinitionsTx(tx, adminId, workspaceId, stages)
	if err != nil {
		return nil, err
	}
	items := make([]SLADefinitionModel, 0, len(stored))
	for _, item := range stored {
		if item.ArchivedAt == 0 {
			items = append(items, slaDefinitionModel(item))
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder == items[j].SortOrder {
			return items[i].Id < items[j].Id
		}
		return items[i].SortOrder < items[j].SortOrder
	})
	return items, nil
}

func listSLADefinitions(model ListSLADefinitionsModel, stages bool) (ListSLADefinitionsModel, error) {
	if model.Page < 1 || model.Size < 1 || model.Size > 100 {
		return model, ErrSLAInvalid
	}
	if err := ensureSLAWorkspace(nil, model.AdminId, model.WorkspaceId); err != nil {
		return model, err
	}
	items, err := listSLADefinitionsTx(nil, model.AdminId, model.WorkspaceId, stages)
	if err != nil {
		return model, err
	}
	model.Total = len(items)
	start := min((model.Page-1)*model.Size, len(items))
	end := min(start+model.Size, len(items))
	model.Items = items[start:end]
	return model, nil
}

// ListSLAStages 返回当前有效阶段。
func ListSLAStages(model ListSLADefinitionsModel) (ListSLADefinitionsModel, error) {
	return listSLADefinitions(model, true)
}

// ListSLAPolicies 返回按匹配顺序排列的服务目标。
func ListSLAPolicies(model ListSLADefinitionsModel) (ListSLADefinitionsModel, error) {
	model, err := listSLADefinitions(model, false)
	if err != nil {
		return model, err
	}
	if !HasPermission(model.AdminId, model.UserId, strconv.Itoa(model.WorkspaceId), "sla:manage") {
		for i := range model.Items {
			model.Items[i].Config, err = redactSLACredentials(model.Items[i].Config)
			if err != nil {
				return model, err
			}
		}
	}
	return model, nil
}

// redactSLACredentials 不向只读用户暴露可直接发送消息的地址或签名密钥。
func redactSLACredentials(data json.RawMessage) (json.RawMessage, error) {
	var config slaPolicyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	for i := range config.Actions {
		for j := range config.Actions[i].Webhooks {
			config.Actions[i].Webhooks[j].URL = ""
			config.Actions[i].Webhooks[j].Secret = ""
		}
	}
	return json.Marshal(config)
}

func validateSLASelector(selector slaSelector) error {
	switch selector.Mode {
	case "any", "workspace_initial", "workspace_final":
		if len(selector.StatusIds) != 0 {
			return ErrSLAInvalid
		}
	case "selected":
		if len(selector.StatusIds) == 0 {
			return ErrSLAInvalid
		}
		seen := map[int]bool{}
		for _, id := range selector.StatusIds {
			if id <= 0 || seen[id] {
				return ErrSLAInvalid
			}
			seen[id] = true
		}
	default:
		return ErrSLAInvalid
	}
	return nil
}

func validateSLAStage(config slaStageConfig) error {
	for _, selector := range []slaSelector{config.From, config.To} {
		if err := validateSLASelector(selector); err != nil {
			return err
		}
	}
	return nil
}

// validateSLAStageForSave 仅在保存时拒绝具体状态重叠，保留历史配置的运行能力。
func validateSLAStageForSave(config slaStageConfig) error {
	if err := validateSLAStage(config); err != nil {
		return err
	}
	if config.From.Mode == "selected" && config.To.Mode == "selected" {
		for _, id := range config.From.StatusIds {
			if slaHasId(config.To.StatusIds, id) {
				return ErrSLAInvalid
			}
		}
	}
	return nil
}

func validateSLAPolicy(config slaPolicyConfig) error {
	if config.Calendar.Mode != "all_day" && config.Calendar.Mode != "weekly" {
		return ErrSLAInvalid
	}
	if config.Calendar.Mode == "weekly" && len(config.Calendar.Slots) == 0 {
		return ErrSLAInvalid
	}
	if len(config.Calendar.HolidayCalendarCode) > 64 {
		return ErrSLAInvalid
	}
	if config.Calendar.PauseOnHolidays {
		if file, err := loadSLAHolidays(); err == nil {
			code := config.Calendar.HolidayCalendarCode
			if code == "" || code == "default" {
				code = file.DefaultCalendar
			}
			found := false
			for _, calendar := range file.Calendars {
				if calendar.Code == code {
					found = true
					break
				}
			}
			if !found {
				return ErrSLAInvalid
			}
		}
	}
	if len(config.Calendar.Slots) > 64 || len(config.Targets) == 0 || len(config.Targets) > 2000 || len(config.Actions) > 100 {
		return ErrSLAInvalid
	}
	if config.Calendar.Mode == "all_day" && len(config.Calendar.Slots) > 0 {
		return ErrSLAInvalid
	}
	for _, slot := range config.Calendar.Slots {
		if len(slot.Weekdays) == 0 || slot.StartMinute < 0 || slot.EndMinute > 1440 || slot.EndMinute <= slot.StartMinute {
			return ErrSLAInvalid
		}
		for _, day := range slot.Weekdays {
			if day < 1 || day > 7 {
				return ErrSLAInvalid
			}
		}
	}
	// 保存前限制时长，避免超过计时器的百年遍历上界后阻断工单事件。
	clockBudget := int64(36599 * 86400)
	if config.Calendar.Mode == "weekly" {
		var weeklySeconds int64
		for day := 1; day <= 7; day++ {
			var windows []slaPause
			for _, slot := range config.Calendar.Slots {
				if slaHasId(slot.Weekdays, day) {
					windows = append(windows, slaPause{int64(slot.StartMinute * 60), int64(slot.EndMinute * 60)})
				}
			}
			for _, window := range mergeSLAIntervals(windows) {
				weeklySeconds += window.End - window.Start
			}
		}
		clockBudget = weeklySeconds * 5200
	}
	targets := map[[2]int]int64{}
	for _, target := range config.Targets {
		key := [2]int{target.PriorityId, target.StageId}
		if target.PriorityId < 0 || target.StageId <= 0 || target.LimitSeconds <= 0 || target.LimitSeconds > clockBudget || target.LimitSeconds%60 != 0 || targets[key] > 0 {
			return ErrSLAInvalid
		}
		targets[key] = target.LimitSeconds
	}
	keys := map[string]bool{}
	for _, action := range config.Actions {
		if action.Key == "" || len(action.Key) > 64 || keys[action.Key] || action.OffsetSeconds < 0 || action.OffsetSeconds%60 != 0 {
			return ErrSLAInvalid
		}
		if action.Channel != "" && action.Channel != "notification" && action.Channel != "webhook" {
			return ErrSLAInvalid
		}
		if action.Channel == "webhook" {
			if len(action.Webhooks) == 0 || len(action.Webhooks) > 3 {
				return ErrSLAInvalid
			}
			platforms := map[string]bool{}
			for _, hook := range action.Webhooks {
				if len(hook.URL) > 4096 || len(hook.Secret) > 1024 || len(hook.AtUserIds) > 1000 || len(hook.AtRoles) > 2 {
					return ErrSLAInvalid
				}
				if !oneOf(hook.Platform, "feishu", "wecom", "dingtalk") || platforms[hook.Platform] {
					return ErrSLAInvalid
				}
				platforms[hook.Platform] = true
				if validateAutomationWebhookAction(AutomationAction{PushSystem: hook.Platform, URL: hook.URL, Secret: hook.Secret, Template: "SLA", AtMode: hook.AtMode, AtRoles: hook.AtRoles, AtUserIds: hook.AtUserIds}) != nil || validateWebhookURLTemplate(hook.URL, nil) != nil {
					return ErrSLAInvalid
				}
			}
		} else if len(action.Recipients) == 0 || len(action.Webhooks) != 0 {
			return ErrSLAInvalid
		}
		keys[action.Key] = true
		if action.Position != "before_deadline" && action.Position != "after_deadline" {
			return ErrSLAInvalid
		}
		found := false
		for key, limit := range targets {
			if key[1] == action.StageId {
				found = true
				if action.Position == "before_deadline" && action.OffsetSeconds > limit {
					return ErrSLAInvalid
				}
				if action.OffsetSeconds > int64(^uint64(0)>>1)-limit || (action.Position == "after_deadline" && action.OffsetSeconds+limit > clockBudget) {
					return ErrSLAInvalid
				}
			}
		}
		if !found {
			return ErrSLAInvalid
		}
		for _, recipient := range action.Recipients {
			if recipient.Type != "assignees" && (recipient.Type != "workspace_role" || recipient.RoleId <= 0) {
				return ErrSLAInvalid
			}
		}
	}
	return nil
}

func checkSLAIdsTx(tx *sql.Tx, table string, adminId, workspaceId int, ids []int, allowZero bool, typeKey bool) error {
	seen := map[int]bool{}
	positive := []int{}
	for _, id := range ids {
		if id < 0 || (!allowZero && id == 0) || seen[id] {
			return ErrSLAInvalid
		}
		seen[id] = true
		if id > 0 {
			positive = append(positive, id)
		}
	}
	if len(positive) == 0 {
		return nil
	}
	query := "SELECT id FROM " + table + " WHERE admin_id=$1 AND workspace_id=$2 AND id=ANY($3)"
	if typeKey {
		query += " AND type_key='requirement'"
	}
	rows, err := slaRows(tx, query, adminId, workspaceId, pq.Array(positive))
	if err != nil {
		return err
	}
	if len(rows) != len(positive) {
		return ErrSLAInvalid
	}
	return nil
}

func checkSLAPolicyReferences(tx *sql.Tx, model SLADefinitionModel, config slaPolicyConfig) error {
	if err := checkSLAIdsTx(tx, define.TableWorkItemPriorities, model.AdminId, model.WorkspaceId, model.PriorityIds, true, true); err != nil {
		return err
	}
	if err := checkSLAIdsTx(tx, define.TableRequirementCategories, model.AdminId, model.WorkspaceId, model.CategoryIds, true, false); err != nil {
		return err
	}
	stages, err := listSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, true)
	if err != nil {
		return err
	}
	valid := map[int]bool{}
	for _, stage := range stages {
		valid[stage.Id] = true
	}
	priorities := append([]int{}, model.PriorityIds...)
	if len(priorities) == 0 {
		rows, err := slaRows(tx, "SELECT id FROM "+define.TableWorkItemPriorities+" WHERE admin_id=$1 AND workspace_id=$2 AND type_key='requirement'", model.AdminId, model.WorkspaceId)
		if err != nil {
			return err
		}
		priorities = append(priorities, 0)
		for _, row := range rows {
			id, err := slaNumber(row, "id")
			if err != nil {
				return err
			}
			priorities = append(priorities, int(id))
		}
	}
	selected := map[int]bool{}
	matrix := map[[2]int]bool{}
	for _, target := range config.Targets {
		if !valid[target.StageId] {
			return ErrSLAInvalid
		}
		selected[target.StageId] = true
		matrix[[2]int{target.PriorityId, target.StageId}] = true
	}
	if len(matrix) != len(priorities)*len(selected) {
		return ErrSLAInvalid
	}
	for _, priority := range priorities {
		for stage := range selected {
			if !matrix[[2]int{priority, stage}] {
				return ErrSLAInvalid
			}
		}
	}
	for _, action := range config.Actions {
		for _, hook := range action.Webhooks {
			if len(hook.AtUserIds) > 0 {
				rows, err := slaRows(tx, "SELECT id FROM "+define.TableUsers+" WHERE admin_id=$1 AND id=ANY($2)", model.AdminId, pq.Array(hook.AtUserIds))
				if err != nil {
					return err
				}
				if len(rows) != len(hook.AtUserIds) {
					return ErrSLAInvalid
				}
			}
		}
		for _, recipient := range action.Recipients {
			if recipient.Type == "workspace_role" {
				if err := checkSLAIdsTx(tx, define.TableRoles, model.AdminId, model.WorkspaceId, []int{recipient.RoleId}, false, false); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func saveSLADefinition(model SLADefinitionModel, stage bool) (SLADefinitionModel, error) {
	model.Name = strings.TrimSpace(model.Name)
	if model.PriorityIds == nil {
		model.PriorityIds = []int{}
	}
	if model.CategoryIds == nil {
		model.CategoryIds = []int{}
	}
	if model.Name == "" || utf8.RuneCountInString(model.Name) > 100 || len(model.Description) > 20000 || len(model.Config) > 1<<20 || model.Id < 0 {
		return model, ErrSLAInvalid
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return model, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = lockSLAConfig(tx, model.AdminId, model.WorkspaceId); err != nil {
		return model, err
	}
	if stage {
		var config slaStageConfig
		if err = json.Unmarshal(model.Config, &config); err != nil {
			return model, ErrSLAInvalid
		}
		if err = validateSLAStageForSave(config); err != nil {
			return model, err
		}
		if err = checkSLAStageReferencesTx(tx, model.AdminId, model.WorkspaceId, config); err != nil {
			return model, err
		}
		model.Config, err = json.Marshal(config)
		if err != nil {
			return model, err
		}
	} else {
		var config slaPolicyConfig
		if err = json.Unmarshal(model.Config, &config); err != nil {
			return model, ErrSLAInvalid
		}
		if err = validateSLAPolicy(config); err != nil {
			return model, err
		}
		if err = checkSLAPolicyReferences(tx, model, config); err != nil {
			return model, err
		}
		// 客户端不能注入计时时区或年度快照。
		config.Calendar.Timezone = ""
		config.Calendar.HolidayYears = nil
		model.Config, err = json.Marshal(config)
		if err != nil {
			return model, err
		}
	}
	stored, err := loadSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, stage)
	if err != nil {
		return model, err
	}
	index := -1
	if model.Id == 0 {
		// ID 在同一空间配置锁内递增，归档后不复用，周期快照仍可准确引用。
		rows, err := slaRows(tx, "UPDATE "+define.TableSLASettings+" SET next_definition_id=next_definition_id+1 WHERE admin_id=$1 AND workspace_id=$2 RETURNING next_definition_id-1 AS id", model.AdminId, model.WorkspaceId)
		if err != nil {
			return model, err
		}
		if len(rows) != 1 {
			return model, ErrSLAConflict
		}
		id, err := slaNumber(rows[0], "id")
		if err != nil {
			return model, err
		}
		model.Id = int(id)
		model.Revision = 1
		index = len(stored)
		stored = append(stored, slaStoredDefinition{})
	} else {
		for i, item := range stored {
			if item.Id == model.Id && item.ArchivedAt == 0 && item.Revision == model.Revision {
				index = i
				break
			}
		}
		if index < 0 {
			return model, ErrSLAConflict
		}
		model.Revision++
	}
	stored[index] = slaStoredDefinition{Id: model.Id, Disabled: model.Disabled, Name: model.Name, Description: model.Description, SortOrder: model.SortOrder, Revision: model.Revision, PriorityIds: model.PriorityIds, CategoryIds: model.CategoryIds, Config: model.Config}
	if err = saveSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, stage, stored); err != nil {
		return model, err
	}
	if err = tx.Commit(); err != nil {
		return model, err
	}
	return model, nil
}

// SaveSLAStage 创建或按 revision 更新阶段定义。
func SaveSLAStage(model SLADefinitionModel) (SLADefinitionModel, error) {
	return saveSLADefinition(model, true)
}

// SaveSLAPolicy 创建或按 revision 更新服务目标。
func SaveSLAPolicy(model SLADefinitionModel) (SLADefinitionModel, error) {
	return saveSLADefinition(model, false)
}

// SaveSLAPolicyOrder 原子保存所有有效服务目标的顺序。
func SaveSLAPolicyOrder(model SLAOrderModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := lockSLAConfig(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	if settings.Revision != model.Revision {
		return ErrSLAConflict
	}
	stored, err := loadSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, false)
	if err != nil {
		return err
	}
	valid := map[int]int{}
	for i, item := range stored {
		if item.ArchivedAt == 0 {
			valid[item.Id] = i
		}
	}
	if len(valid) != len(model.Ids) {
		return ErrSLAInvalid
	}
	for order, id := range model.Ids {
		index, ok := valid[id]
		if !ok {
			return ErrSLAInvalid
		}
		delete(valid, id)
		stored[index].SortOrder = order
	}
	if err = saveSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, false, stored); err != nil {
		return err
	}
	return tx.Commit()
}

func slaHasId(ids []int, id int) bool {
	for _, value := range ids {
		if value == id {
			return true
		}
	}
	return false
}

func matchSLAPolicy(items []SLADefinitionModel, state SLAWorkItemStateModel) (*SLADefinitionModel, error) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SortOrder == items[j].SortOrder {
			return items[i].Id < items[j].Id
		}
		return items[i].SortOrder < items[j].SortOrder
	})
	for i := range items {
		item := &items[i]
		if item.Disabled {
			continue
		}
		if (len(item.PriorityIds) == 0 || slaHasId(item.PriorityIds, state.PriorityId)) && (len(item.CategoryIds) == 0 || slaHasId(item.CategoryIds, state.CategoryId)) {
			return item, nil
		}
	}
	return nil, nil
}

// GetSLAPolicy 读取指定空间的有效服务目标。
func GetSLAPolicy(model SLADefinitionModel) (SLADefinitionModel, error) {
	if err := ensureSLAWorkspace(nil, model.AdminId, model.WorkspaceId); err != nil {
		return model, err
	}
	items, err := listSLADefinitionsTx(nil, model.AdminId, model.WorkspaceId, false)
	if err != nil {
		return model, err
	}
	for _, item := range items {
		if item.Id == model.Id {
			if !HasPermission(model.AdminId, model.UserId, strconv.Itoa(model.WorkspaceId), "sla:manage") {
				item.Config, err = redactSLACredentials(item.Config)
				if err != nil {
					return model, err
				}
			}
			return item, nil
		}
	}
	return model, ErrSLANotFound
}

// ArchiveSLADefinition 归档未被活动规则或周期引用的定义，保留历史关联。
func ArchiveSLADefinition(model SLADefinitionModel, stage bool) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = lockSLAConfig(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	column := "policy_id"
	if stage {
		column = "stage_id"
	}
	rows, err := slaRows(tx, "SELECT id FROM "+define.TableSLACycles+" WHERE admin_id=$1 AND workspace_id=$2 AND "+column+"=$3 AND state='running' LIMIT 1", model.AdminId, model.WorkspaceId, model.Id)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return ErrSLAConflict
	}
	if stage {
		items, err := listSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, false)
		if err != nil {
			return err
		}
		for _, item := range items {
			var config slaPolicyConfig
			if err = json.Unmarshal(item.Config, &config); err != nil {
				return err
			}
			for _, target := range config.Targets {
				if target.StageId == model.Id {
					return ErrSLAConflict
				}
			}
		}
	}
	stored, err := loadSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, stage)
	if err != nil {
		return err
	}
	found := false
	for i, item := range stored {
		if item.Id == model.Id && item.Revision == model.Revision && item.ArchivedAt == 0 {
			stored[i].ArchivedAt = time.Now().Unix()
			stored[i].Revision++
			found = true
			break
		}
	}
	if !found {
		return ErrSLAConflict
	}
	if err = saveSLADefinitionsTx(tx, model.AdminId, model.WorkspaceId, stage, stored); err != nil {
		return err
	}
	return tx.Commit()
}

// GetSLAMetadata 读取同工作空间的状态、优先级、分类和角色。
func GetSLAMetadata(model SLAMetadataModel) (SLAMetadataModel, error) {
	if err := ensureSLAWorkspace(nil, model.AdminId, model.WorkspaceId); err != nil {
		return model, err
	}
	queries := []struct {
		query       string
		destination *[]SLAOptionModel
	}{
		{"SELECT id,name,color,0 AS parent_id FROM " + define.TableWorkItemStatuses + " WHERE admin_id=$1 AND workspace_id=$2 AND type_key='requirement' ORDER BY sort_order,id", &model.Statuses},
		{"SELECT id,name,color,0 AS parent_id FROM " + define.TableWorkItemPriorities + " WHERE admin_id=$1 AND workspace_id=$2 AND type_key='requirement' ORDER BY sort_order,id", &model.Priorities},
		{"SELECT id,name,'' AS color,parent_id FROM " + define.TableRequirementCategories + " WHERE admin_id=$1 AND workspace_id=$2 ORDER BY sort_order,id", &model.Categories},
		{"SELECT id,name,'' AS color,0 AS parent_id FROM " + define.TableRoles + " WHERE admin_id=$1 AND workspace_id=$2 AND scope='project' ORDER BY id", &model.Roles},
	}
	for _, item := range queries {
		rows, err := slaRows(nil, item.query, model.AdminId, model.WorkspaceId)
		if err != nil {
			return model, err
		}
		options := make([]SLAOptionModel, 0, len(rows))
		for _, row := range rows {
			id, err := slaNumber(row, "id")
			if err != nil {
				return model, err
			}
			parent, err := slaNumber(row, "parent_id")
			if err != nil {
				return model, err
			}
			options = append(options, SLAOptionModel{Id: int(id), ParentId: int(parent), Name: row["name"], Color: row["color"]})
		}
		*item.destination = options
	}
	return model, nil
}

// checkSLAStageReferencesTx 防止虚拟选择器解析为空或不可达状态。
func checkSLAStageReferencesTx(tx *sql.Tx, adminId, workspaceId int, config slaStageConfig) error {
	rows, err := slaRows(tx, "SELECT requirement_initial_status_id,requirement_final_status_ids FROM "+define.TableWorkspaceSettings+" WHERE admin_id=$1 AND workspace_id=$2", adminId, workspaceId)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return ErrSLAInvalid
	}
	initial, err := slaNumber(rows[0], "requirement_initial_status_id")
	if err != nil {
		return err
	}
	var finals []int
	if text := rows[0]["requirement_final_status_ids"]; text != "" {
		for _, raw := range strings.Split(text, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil || id <= 0 {
				return ErrSLAInvalid
			}
			finals = append(finals, id)
		}
	}
	for _, selector := range []slaSelector{config.From, config.To} {
		resolved := resolveSLASelector(selector, int(initial), finals)
		if resolved.Mode == "selected" && len(resolved.StatusIds) == 0 {
			return ErrSLAInvalid
		}
		if err := checkSLAIdsTx(tx, define.TableWorkItemStatuses, adminId, workspaceId, resolved.StatusIds, false, true); err != nil {
			return err
		}
	}
	return nil
}

func checkSLAWorkspaceStatusUpdateTx(tx *sql.Tx, adminId, workspaceId int) error {
	stages, err := listSLADefinitionsTx(tx, adminId, workspaceId, true)
	if err != nil {
		return err
	}
	for _, stage := range stages {
		var config slaStageConfig
		if err = json.Unmarshal(stage.Config, &config); err != nil {
			return err
		}
		if err = checkSLAStageReferencesTx(tx, adminId, workspaceId, config); err != nil {
			if errors.Is(err, ErrSLAInvalid) {
				return ErrSLAConflict
			}
			return err
		}
	}
	return nil
}
