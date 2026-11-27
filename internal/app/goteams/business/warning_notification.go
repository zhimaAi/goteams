package business

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
)

// ErrWarningInvalid 表示预警规则配置或工作空间范围无效。
var ErrWarningInvalid = errors.New("invalid warning notification")

// ErrWarningNotFound 表示当前工作空间内不存在指定预警规则。
var ErrWarningNotFound = errors.New("warning notification not found")

// WarningRuleModel 表示工作空间内的预警规则及其持久化配置。
type WarningRuleModel struct {
	AdminId, WorkspaceId, Id int
	Name                     string
	Enabled                  bool
	Config                   json.RawMessage
}

// WarningListModel 包含预警规则的分页查询和结果。
type WarningListModel struct {
	AdminId, WorkspaceId, Page, Size, Total int
	Name, Keyword                           string
	Items                                   []WarningRuleModel
}

type warningChannel struct {
	Channel string `json:"channel"`
	URL     string `json:"url"`
	Secret  string `json:"secret,omitempty"`
	Content string `json:"content"`
}

type warningConfig struct {
	Channels        []warningChannel      `json:"channels,omitempty"`
	Type            string                `json:"type"`
	TemplateId      int                   `json:"template_id"`
	Relation        string                `json:"relation"`
	Conditions      []AutomationCondition `json:"conditions"`
	WindowMinutes   int                   `json:"window_minutes"`
	Threshold       int                   `json:"threshold"`
	IntervalMinutes int                   `json:"interval_minutes"`
	Channel         string                `json:"channel"`
	URL             string                `json:"url"`
	Secret          string                `json:"secret,omitempty"`
	Content         string                `json:"content"`
}

// WarningMetadataModel 提供指定模块、模板的条件字段。
type WarningMetadataModel struct {
	AdminId, WorkspaceId, TemplateId int
	Type, Lang                       string
	Templates                        []WorkItemTemplateModel
	Fields                           []AutomationFieldMeta
}

func warningWorkspace(adminId, workspaceId int) error {
	rows, err := msql.RawValues(define.Postgres, "SELECT id FROM "+define.TableWorkspaces+" WHERE admin_id=$1 AND id=$2 AND workspace_type='issue'", nil, adminId, workspaceId)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("%w: workspace scope mismatch (admin_id=%d workspace_id=%d, issue workspace required)", ErrWarningInvalid, adminId, workspaceId)
	}
	return nil
}

// WarningMetadata 只返回当前工作空间所关联模板中的可检测字段。
func WarningMetadata(model WarningMetadataModel) (WarningMetadataModel, error) {
	if !oneOf(model.Type, "requirement", "defect") {
		return model, ErrWarningInvalid
	}
	if err := warningWorkspace(model.AdminId, model.WorkspaceId); err != nil {
		return model, err
	}
	templates, err := ListWorkItemTemplates(ListWorkItemTemplatesModel{AdminId: model.AdminId, WorkspaceId: strconv.Itoa(model.WorkspaceId), TypeKey: model.Type})
	if err != nil {
		return model, err
	}
	model.Templates = templates.Items
	invites, err := ListIssueInviteTemplates(ListIssueInviteTemplatesModel{AdminID: model.AdminId, WorkspaceID: model.WorkspaceId, EnabledOnly: true})
	if err != nil {
		return model, err
	}
	for _, invite := range invites {
		if invite.TypeKey != model.Type {
			continue
		}
		fields, err := resolveIssueInviteTemplateFields(model.AdminId, model.WorkspaceId, invite.TypeKey, invite.FieldsJSON)
		if err != nil {
			return model, err
		}
		model.Templates = append(model.Templates, WorkItemTemplateModel{Id: invite.ID, Name: invite.Name, TypeKey: invite.TypeKey, Fields: fields})
	}
	model.Fields = []AutomationFieldMeta{}
	if model.TemplateId == 0 {
		defaultFields, err := issueInviteEnabledFields(model.AdminId, model.WorkspaceId, model.Type)
		if err != nil {
			return model, err
		}
		model.Templates = append(model.Templates, WorkItemTemplateModel{Id: 0, Fields: defaultFields})
	}
	var template *WorkItemTemplateModel
	for i := range model.Templates {
		if model.Templates[i].Id == model.TemplateId {
			template = &model.Templates[i]
			break
		}
	}
	if template == nil {
		return model, fmt.Errorf("%w: template not available (workspace_id=%d template_id=%d type=%s)", ErrWarningInvalid, model.WorkspaceId, model.TemplateId, model.Type)
	}
	fields, err := automationFields(model.AdminId, model.WorkspaceId, model.Type, model.Lang)
	if err != nil {
		return model, err
	}
	custom, err := ListCustomFields(ListCustomFieldsModel{AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId), TypeKey: model.Type})
	if err != nil {
		return model, err
	}
	customKeys := map[int]string{}
	for _, f := range custom.Items {
		customKeys[f.Id] = f.Key
	}
	keys := map[string]bool{"title": true, "description": true}
	for _, f := range template.Fields {
		if !warningTemplateFieldShown(f.Display) {
			continue
		}
		key := f.FieldKey
		if id := templateCustomFieldID(key); id > 0 {
			key = customKeys[id]
		}
		if key == "status_id" {
			key = "status"
		}
		keys[key] = true
	}
	meta, err := GetWorkspaceMetadata(WorkspaceMetadataModel{AdminId: model.AdminId, WorkspaceId: strconv.Itoa(model.WorkspaceId), Lang: model.Lang, IncludeVersions: true})
	if err != nil {
		return model, err
	}
	group := meta.Requirement
	if model.Type == "defect" {
		group = meta.Defect
	}
	options := map[string][]CustomFieldOption{}
	categories, err := listRequirementCategoryItems(model.AdminId, model.WorkspaceId)
	if err != nil {
		return model, err
	}
	for _, v := range categories {
		options["category"] = append(options["category"], CustomFieldOption{Key: strconv.Itoa(v.Id), Name: v.Name})
	}
	for _, v := range group.Statuses {
		options["status"] = append(options["status"], CustomFieldOption{Key: strconv.Itoa(v.Id), Name: v.Name})
	}
	for _, v := range group.Priorities {
		options["priority"] = append(options["priority"], CustomFieldOption{Key: strconv.Itoa(v.Id), Name: v.Label})
	}
	for _, v := range meta.Versions {
		options["version"] = append(options["version"], CustomFieldOption{Key: strconv.Itoa(v.Id), Name: v.Name})
	}
	for _, f := range fields {
		if keys[f.Key] {
			if len(options[f.Key]) > 0 {
				f.Options = options[f.Key]
			}
			if f.Key == "assignee" || f.Key == "created_by" {
				f.Options = warningWorkspaceMemberOptions(meta.Members)
			} else if f.Type == "user" || f.Type == "user_multi" {
				for _, v := range meta.CustomFieldMembers {
					if v.UserStatus == 1 && !v.Removed && (f.MemberScope == "all" || (f.MemberScope == "roles" && warningHasInt(f.RoleIds, v.EnterpriseRoleId)) || (f.MemberScope != "roles" && f.MemberScope != "all" && v.IsWorkspaceMember)) {
						f.Options = append(f.Options, CustomFieldOption{Key: strconv.Itoa(v.UserId), Name: v.UserName})
					}
				}
			}
			for _, customField := range custom.Items {
				if customField.Key == f.Key && customFieldIsCascader(customField.FieldType) {
					var walk func([]CustomFieldCascaderNode, string)
					walk = func(nodes []CustomFieldCascaderNode, prefix string) {
						for _, n := range nodes {
							label := prefix + n.Name
							f.Options = append(f.Options, CustomFieldOption{Key: n.Key, Name: label})
							walk(n.Children, label+" / ")
						}
					}
					walk(customField.Cascader, "")
				}
			}
			f.Type = warningFieldType(f.Type)
			model.Fields = append(model.Fields, f)
		}
	}
	return model, nil
}

func (c warningConfig) deliveryChannels() []warningChannel {
	if c.Channels != nil {
		return c.Channels
	}
	return []warningChannel{{Channel: c.Channel, URL: c.URL, Secret: c.Secret, Content: c.Content}}
}

func (c warningConfig) hasInternalChannel() bool {
	for _, channel := range c.deliveryChannels() {
		if channel.Channel == "internal" {
			return true
		}
	}
	return false
}

func decodeWarningConfig(raw json.RawMessage) (warningConfig, error) {
	var c warningConfig
	if len(raw) > 65536 {
		return c, fmt.Errorf("%w: config exceeds 65536 bytes", ErrWarningInvalid)
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		// 解析错误可能包含配置中的敏感值，仅记录错误类型。
		return c, fmt.Errorf("%w: config JSON decode failed (%T)", ErrWarningInvalid, err)
	}
	if !oneOf(c.Type, "requirement", "defect") {
		return c, fmt.Errorf("%w: config.type unsupported", ErrWarningInvalid)
	}
	if c.TemplateId < 0 {
		return c, fmt.Errorf("%w: config.template_id must be nonnegative", ErrWarningInvalid)
	}
	if !oneOf(c.Relation, "AND", "OR") {
		return c, fmt.Errorf("%w: config.relation unsupported", ErrWarningInvalid)
	}
	if len(c.Conditions) == 0 || len(c.Conditions) > 30 {
		return c, fmt.Errorf("%w: config.conditions count out of range [1,30]", ErrWarningInvalid)
	}
	if c.WindowMinutes < 1 || c.WindowMinutes > 43200 {
		return c, fmt.Errorf("%w: config.window_minutes out of range [1,43200]", ErrWarningInvalid)
	}
	if c.IntervalMinutes < 1 || c.IntervalMinutes > 43200 {
		return c, fmt.Errorf("%w: config.interval_minutes out of range [1,43200]", ErrWarningInvalid)
	}
	if c.Threshold < 1 || c.Threshold > 1000000 {
		return c, fmt.Errorf("%w: config.threshold out of range [1,1000000]", ErrWarningInvalid)
	}
	if c.Channels != nil && len(c.Channels) == 0 {
		return c, fmt.Errorf("%w: config.channels empty", ErrWarningInvalid)
	}
	if len(c.Channels) > 4 {
		return c, fmt.Errorf("%w: config.channels count exceeds 4", ErrWarningInvalid)
	}
	seen := map[string]bool{}
	for i, channel := range c.deliveryChannels() {
		if !oneOf(channel.Channel, "feishu", "wecom", "dingtalk", "internal") {
			return c, fmt.Errorf("%w: channel[%d] unsupported", ErrWarningInvalid, i)
		}
		if seen[channel.Channel] {
			return c, fmt.Errorf("%w: channel[%d] duplicate %s", ErrWarningInvalid, i, channel.Channel)
		}
		if strings.TrimSpace(channel.Content) == "" || utf8.RuneCountInString(channel.Content) > 1000 {
			return c, fmt.Errorf("%w: channel[%d] %s content empty or exceeds 1000 characters", ErrWarningInvalid, i, channel.Channel)
		}
		seen[channel.Channel] = true
		if channel.Channel != "internal" {
			if err := validateAutomationWebhookAction(AutomationAction{PushSystem: channel.Channel, URL: channel.URL, Secret: channel.Secret, Template: channel.Content, AtMode: automationAtModes(channel.Channel)[0]}); err != nil {
				return c, fmt.Errorf("%w: channel[%d] %s action: %v", ErrWarningInvalid, i, channel.Channel, err)
			}
			if err := validateWebhookURLTemplate(channel.URL, nil); err != nil {
				// URL 校验的原始错误可能包含完整 Webhook 和密钥。
				return c, fmt.Errorf("%w: channel[%d] %s webhook URL invalid", ErrWarningInvalid, i, channel.Channel)
			}
		}
	}
	return c, nil
}

func validateWarningRule(m WarningRuleModel) error {
	if strings.TrimSpace(m.Name) == "" || utf8.RuneCountInString(m.Name) > 20 {
		return fmt.Errorf("%w: name empty or exceeds 20 characters", ErrWarningInvalid)
	}
	c, err := decodeWarningConfig(m.Config)
	if err != nil {
		return err
	}
	meta, err := WarningMetadata(WarningMetadataModel{AdminId: m.AdminId, WorkspaceId: m.WorkspaceId, TemplateId: c.TemplateId, Type: c.Type, Lang: "zh-CN"})
	if err != nil {
		return fmt.Errorf("metadata validation (admin_id=%d workspace_id=%d template_id=%d): %w", m.AdminId, m.WorkspaceId, c.TemplateId, err)
	}
	fields := map[string]AutomationFieldMeta{}
	for _, f := range meta.Fields {
		fields[f.Key] = f
	}
	for i, cond := range c.Conditions {
		f, ok := fields[cond.FieldKey]
		if !ok {
			return fmt.Errorf("%w: condition[%d] field unavailable in template", ErrWarningInvalid, i)
		}
		if cond.SkipWhenEmpty {
			return fmt.Errorf("%w: condition[%d] skip_when_empty unsupported", ErrWarningInvalid, i)
		}
		if !automationOperatorAllowed(f.Type, cond.Operator) {
			return fmt.Errorf("%w: condition[%d] operator unsupported for field type", ErrWarningInvalid, i)
		}
		if !oneOf(cond.Operator, "empty", "not_empty") {
			var v any
			if len(cond.Value) == 0 || json.Unmarshal(cond.Value, &v) != nil || v == nil {
				return fmt.Errorf("%w: condition[%d] value missing or invalid JSON", ErrWarningInvalid, i)
			}
			if err := validateWarningValue(f, cond, v); err != nil {
				return fmt.Errorf("condition[%d] value invalid for field type: %w", i, err)
			}
			if f.Type == "number" || f.Type == "numeric" {
				if _, ok := automationNumber(v); !ok {
					return fmt.Errorf("%w: condition[%d] value is not numeric", ErrWarningInvalid, i)
				}
			}
		}
	}
	return nil
}

// ListWarningRules 返回租户工作空间范围内的规则分页。
func ListWarningRules(m WarningListModel) (WarningListModel, error) {
	if err := warningWorkspace(m.AdminId, m.WorkspaceId); err != nil {
		return m, err
	}
	if m.Page < 1 || m.Page > 1000000 || m.Size < 1 || m.Size > 100 || len(m.Name) > 1024 || len(m.Keyword) > 1024 {
		return m, ErrWarningInvalid
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return m, err
	}
	where := " FROM " + define.TableWarningRules + " WHERE admin_id=$1 AND workspace_id=$2 AND name ILIKE $3 AND (config_json->'conditions')::text ILIKE $4"
	args := []any{m.AdminId, m.WorkspaceId, "%" + m.Name + "%", "%" + m.Keyword + "%"}
	if err = (warningSQL{db}).QueryRowContext(context.Background(), "SELECT count(*)"+where, args...).Scan(&m.Total); err != nil {
		return m, err
	}
	rows, err := (warningSQL{db}).QueryContext(context.Background(), "SELECT id,name,enabled,config_json"+where+" ORDER BY id DESC LIMIT $5 OFFSET $6", append(args, m.Size, (m.Page-1)*m.Size)...)
	if err != nil {
		return m, err
	}
	defer rows.Close()
	m.Items = []WarningRuleModel{}
	for rows.Next() {
		item := WarningRuleModel{AdminId: m.AdminId, WorkspaceId: m.WorkspaceId}
		if err := rows.Scan(&item.Id, &item.Name, &item.Enabled, &item.Config); err != nil {
			return m, err
		}
		m.Items = append(m.Items, item)
	}
	return m, rows.Err()
}

// SaveWarningRule 校验字段和模板作用域后创建或更新规则。
func SaveWarningRule(m WarningRuleModel) (WarningRuleModel, error) {
	if err := validateWarningRule(m); err != nil {
		return m, err
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return m, err
	}
	if m.Id == 0 {
		err = (warningSQL{db}).QueryRowContext(context.Background(), "INSERT INTO "+define.TableWarningRules+" (admin_id,workspace_id,name,enabled,config_json) VALUES($1,$2,$3,$4,$5) RETURNING id", m.AdminId, m.WorkspaceId, strings.TrimSpace(m.Name), m.Enabled, []byte(m.Config)).Scan(&m.Id)
	} else {
		result, e := (warningSQL{db}).ExecContext(context.Background(), "UPDATE "+define.TableWarningRules+" SET name=$4,enabled=$5,config_json=$6,next_check_at=0,config_revision=config_revision+1,lease_token='',lease_until=0 WHERE admin_id=$1 AND workspace_id=$2 AND id=$3", m.AdminId, m.WorkspaceId, m.Id, strings.TrimSpace(m.Name), m.Enabled, []byte(m.Config))
		err = e
		if err == nil {
			n, e := result.RowsAffected()
			err = e
			if err == nil && n == 0 {
				err = ErrWarningNotFound
			}
		}
	}
	return m, err
}

// DeleteWarningRule 删除指定范围内的规则及其待发送通知。
func DeleteWarningRule(m WarningRuleModel) error {
	if m.Id <= 0 || m.AdminId <= 0 || m.WorkspaceId <= 0 {
		return ErrWarningInvalid
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	result, err := (warningSQL{db}).ExecContext(context.Background(), "DELETE FROM "+define.TableWarningRules+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3", m.AdminId, m.WorkspaceId, m.Id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrWarningNotFound
	}
	return err
}

// 内置选项在数据库存数字 ID，规则选项存字符串；空值和业务数值保持原语义。
func warningConditionActual(key string, actual any) any {
	if warningBuiltinTypes[key] == "select_single" || key == "created_by" {
		if id, ok := automationEqualityNumber(actual); ok {
			return strconv.FormatFloat(id, 'f', -1, 64)
		}
	}
	return actual
}

func warningMatches(c warningConfig, types map[string]string, fields map[string]any) bool {
	for _, condition := range c.Conditions {
		typ, exists := types[condition.FieldKey]
		if !exists || !automationOperatorAllowed(typ, condition.Operator) {
			return false
		}
	}
	for _, condition := range c.Conditions {
		typ, exists := types[condition.FieldKey]
		if !exists || !automationOperatorAllowed(typ, condition.Operator) {
			return false
		}
		actual := warningConditionActual(condition.FieldKey, fields[condition.FieldKey])
		if condition.FieldKey == "assignee" {
			values := []string{}
			for _, id := range automationAssigneeIDs(actual) {
				values = append(values, strconv.Itoa(id))
			}
			actual = values
			switch condition.Operator {
			case "is", "is_not":
				var value any
				if json.Unmarshal(condition.Value, &value) != nil {
					return false
				}
				expected, err := json.Marshal([]any{value})
				if err != nil {
					return false
				}
				condition.Value = expected
				if condition.Operator == "is" {
					condition.Operator = "contains_any"
				} else {
					condition.Operator = "contains_none"
				}
			case "in":
				condition.Operator = "contains_any"
			case "not_in":
				condition.Operator = "contains_none"
			}
		}
		matched, _ := evaluateAutomationCondition(typ, condition, actual)
		if c.Relation == "AND" && !matched {
			return false
		}
		if c.Relation == "OR" && matched {
			return true
		}
	}
	return c.Relation == "AND" && len(c.Conditions) > 0
}

func rollbackWarning(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logs.Error("warning rollback: %v", err)
	}
}

func deliverWarning(parent context.Context, internal bool) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	var id int64
	var adminId, wsId, ruleId, attempt int
	var raw, recipientsRaw []byte
	token := uuid.NewString()
	now := time.Now().Unix()
	err = (warningSQL{db}).QueryRowContext(ctx, "WITH candidate AS (SELECT d.id FROM "+define.TableWarningDeliveries+" d JOIN "+define.TableWarningRules+" r ON r.admin_id=d.admin_id AND r.workspace_id=d.workspace_id AND r.id=d.rule_id WHERE r.enabled AND (d.config_json->>'channel'='internal')=$3 AND d.attempts<3 AND d.sent_at=0 AND d.next_attempt_at<=$1 ORDER BY d.id FOR UPDATE OF d SKIP LOCKED LIMIT 1) UPDATE "+define.TableWarningDeliveries+" d SET lease_token=$2,next_attempt_at=$1+120,attempts=attempts+1 FROM candidate WHERE d.id=candidate.id RETURNING d.id,d.admin_id,d.workspace_id,d.rule_id,d.config_json,d.recipients,d.attempts", now, token, internal).Scan(&id, &adminId, &wsId, &ruleId, &raw, &recipientsRaw, &attempt)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	c, err := decodeWarningConfig(raw)
	if err != nil {
		logs.Error("warning delivery config invalid: delivery=%d: %v", id, err)
		return err
	}
	var ruleName, workspaceName string
	err = (warningSQL{db}).QueryRowContext(ctx, "SELECT r.name,w.name FROM "+define.TableWarningRules+" r JOIN "+define.TableWorkspaces+" w ON w.admin_id=r.admin_id AND w.id=r.workspace_id WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.id=$3", adminId, wsId, ruleId).Scan(&ruleName, &workspaceName)
	if err != nil {
		return err
	}
	if c.Channel == "internal" {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer rollbackWarning(tx)
		// 先锁父规则再锁队列，和删除级联保持同一顺序，阻止关闭或删除后的站内写入。
		var locked int
		err = (warningSQL{tx}).QueryRowContext(ctx, "SELECT id FROM "+define.TableWarningRules+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND enabled FOR SHARE", adminId, wsId, ruleId).Scan(&locked)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		err = (warningSQL{tx}).QueryRowContext(ctx, "SELECT id FROM "+define.TableWarningDeliveries+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND lease_token=$4 AND sent_at=0 FOR UPDATE", adminId, wsId, id, token).Scan(&locked)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		var recipients []int
		if err = json.Unmarshal(recipientsRaw, &recipients); err != nil {
			return err
		}
		// 写入时再核对有效成员，避免排队期间离开工作空间的成员收到消息。
		_, err = (warningSQL{tx}).ExecContext(ctx, "INSERT INTO "+define.TableNotifications+" (admin_id,recipient_user_id,workspace_id,requirement_id,event_type,actor_user_id,field_name,content,read_at,created_at) SELECT wm.admin_id,wm.user_id,wm.workspace_id,NULL,$7,0,$6,$4,0,$5 FROM "+define.TableWorkspaceMembers+" wm JOIN "+define.TableUsers+" u ON u.admin_id=wm.admin_id AND u.id=wm.user_id WHERE wm.admin_id=$1 AND wm.workspace_id=$2 AND wm.user_id=ANY($3) AND u.status=1 AND u.deleted_at=0", adminId, wsId, pq.Array(recipients), c.Content, now, ruleName, define.NotificationEventWarningNotification)
		if err != nil {
			return err
		}
		if _, err = (warningSQL{tx}).ExecContext(ctx, "UPDATE "+define.TableWarningDeliveries+" SET sent_at=$5,lease_token='' WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND lease_token=$4", adminId, wsId, id, token, now); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		return nil
	}
	// 领取已提交，外发请求不占用数据库事务；同一通知最多尝试三次。
	err = sendExternalNotificationWebhookContext(ctx, c.Channel, c.URL, c.Secret, notificationAtConfig{AtMode: automationAtModes(c.Channel)[0]}, lib_i18n.Show(notificationMessageLang, "warning_push_content", c.Content, workspaceName, ruleName), AutomationEvent{AdminID: adminId, WorkspaceID: wsId})
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer finishCancel()
	if err != nil {
		_, updateErr := (warningSQL{db}).ExecContext(finishCtx, "UPDATE "+define.TableWarningDeliveries+" SET next_attempt_at=$5,lease_token='' WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND lease_token=$4", adminId, wsId, id, token, now+60)
		if updateErr != nil {
			return updateErr
		}
		logs.Error("warning webhook failed: admin=%d workspace=%d rule=%d delivery=%d platform=%s attempt=%d error_type=%T next_attempt_at=%d exhausted=%t", adminId, wsId, ruleId, id, c.Channel, attempt, err, now+60, attempt >= 3)
		return nil
	}
	_, err = (warningSQL{db}).ExecContext(finishCtx, "UPDATE "+define.TableWarningDeliveries+" SET sent_at=$5,lease_token='' WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND lease_token=$4", adminId, wsId, id, token, now)
	return err
}

func warningFieldType(typ string) string {
	switch typ {
	case "cascader_single", "cascader_multi", "checkbox":
		return "select_multi"
	case "radio":
		return "select_single"
	}
	return typ
}

func warningHasInt(values []int, id int) bool {
	for _, v := range values {
		if v == id {
			return true
		}
	}
	return false
}

func validateWarningValue(f AutomationFieldMeta, c AutomationCondition, v any) error {
	if oneOf(f.Type, "select_single", "select_multi", "user", "user_multi") {
		multiple := oneOf(f.Type, "select_multi", "user_multi") || oneOf(c.Operator, "in", "not_in")
		values := []any{v}
		if multiple {
			var ok bool
			values, ok = v.([]any)
			if !ok || len(values) == 0 || len(values) > 100 {
				return ErrWarningInvalid
			}
		}
		allowed := map[string]bool{}
		for _, option := range f.Options {
			allowed[option.Key] = true
		}
		for _, value := range values {
			key, ok := value.(string)
			if !ok || !allowed[key] {
				return ErrWarningInvalid
			}
		}
	}
	if oneOf(f.Type, "text", "textarea", "phone") {
		if _, ok := v.(string); !ok {
			return ErrWarningInvalid
		}
	}
	if f.Type == "datetime" {
		text, ok := v.(string)
		if !ok {
			return ErrWarningInvalid
		}
		if _, ok = normalizeAutomationDatetime(text); !ok {
			return ErrWarningInvalid
		}
	}
	if f.Type == "date" {
		value, ok := automationNumber(v)
		if !ok || value < 0 {
			return ErrWarningInvalid
		}
	}
	return nil
}

// DB 与事务共用 SQL 日志边界；只记录语句，不展开包含密钥的参数。
type warningSQL struct {
	runner interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}
}
type warningRow struct {
	row   *sql.Row
	query string
}

func (r warningRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	warningSQLError(r.query, err)
	return err
}
func (s warningSQL) QueryRowContext(ctx context.Context, query string, args ...any) warningRow {
	return warningRow{s.runner.QueryRowContext(ctx, query, args...), query}
}
func (s warningSQL) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := s.runner.QueryContext(ctx, query, args...)
	warningSQLError(query, err)
	return rows, err
}
func (s warningSQL) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := s.runner.ExecContext(ctx, query, args...)
	warningSQLError(query, err)
	return result, err
}
func warningSQLError(query string, err error) {
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logs.Error("sql:%s, err:%v", query, err)
	}
}

// SetWarningRuleEnabled 独立切换启停，失效字段不妨碍关闭已有规则。
func SetWarningRuleEnabled(m WarningRuleModel) error {
	if m.AdminId <= 0 || m.WorkspaceId <= 0 || m.Id <= 0 {
		return ErrWarningInvalid
	}
	if m.Enabled {
		db, err := msql.GetDB(define.Postgres)
		if err != nil {
			return err
		}
		if err = (warningSQL{db}).QueryRowContext(context.Background(), "SELECT name,config_json FROM "+define.TableWarningRules+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3", m.AdminId, m.WorkspaceId, m.Id).Scan(&m.Name, &m.Config); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrWarningNotFound
			}
			return err
		}
		if err = validateWarningRule(m); err != nil {
			return err
		}
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return err
	}
	result, err := (warningSQL{db}).ExecContext(context.Background(), "UPDATE "+define.TableWarningRules+" SET enabled=$4,next_check_at=0,config_revision=config_revision+1,lease_token='',lease_until=0 WHERE admin_id=$1 AND workspace_id=$2 AND id=$3", m.AdminId, m.WorkspaceId, m.Id, m.Enabled)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrWarningNotFound
	}
	return err
}

func warningWorkspaceMemberOptions(members []WorkspaceMemberModel) []CustomFieldOption {
	result := []CustomFieldOption{}
	seen := map[int]bool{}
	for _, m := range members {
		if m.UserId <= 0 || m.UserStatus != 1 || m.Removed || seen[m.UserId] {
			continue
		}
		seen[m.UserId] = true
		name := m.UserName
		if name == "" {
			name = m.UserUsername
		}
		result = append(result, CustomFieldOption{Key: strconv.Itoa(m.UserId), Name: name})
	}
	return result
}

// warningTemplateFieldShown 沿用模板显示约定：未设置时显示，显式关闭时不参与预警。
func warningTemplateFieldShown(display *bool) bool {
	return display == nil || *display
}
