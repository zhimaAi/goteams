package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
)

// ErrSatisfactionInvalid 表示满意度配置或提交不符合生效规则。
var ErrSatisfactionInvalid = errors.New("invalid satisfaction")

// ErrSatisfactionConflict 表示并发保存或重复改评冲突。
var ErrSatisfactionConflict = errors.New("satisfaction conflict")

// ErrSatisfactionForbidden 表示当前空间或访问者不适用满意度功能。
var ErrSatisfactionForbidden = errors.New("satisfaction forbidden")

// SatisfactionTagModel 是配置标签及结果文字快照。
type SatisfactionTagModel struct{ Key, Label string }

// SatisfactionOptionModel 表示一个等级的标签和必填规则。
type SatisfactionOptionModel struct {
	Key, Label, Sentiment         string
	Value                         int
	Tags                          []SatisfactionTagModel
	TagsRequired, CommentRequired bool
}

// SatisfactionConfigModel 是当前配置或工单提交时固化的规则快照。
type SatisfactionConfigModel struct {
	AdminId, WorkspaceId, UserId                     int
	Enabled, ManualEnabled                           bool
	ValiditySeconds                                  int
	AutoCreatorKinds, AutoStatusIds, AutoTemplateIds []int
	RatingMode, Version                              int
	Options                                          []SatisfactionOptionModel
	ResolvedEnabled, ResolvedRequired                bool
	InvitationText, CompletionText                   string
}

// SatisfactionAccessModel 携带认证上下文或既有邀填客户上下文。
type SatisfactionAccessModel struct {
	AdminId, WorkspaceId, UserId, RequirementId int
	InviteKey, OpenID                           string
}

// SatisfactionSubmitModel 表示一次客户评价，不允许客户端指定快照或评价人身份。
type SatisfactionSubmitModel struct {
	Access                SatisfactionAccessModel
	ExpectedConfigVersion int
	OptionKey             string
	TagKeys               []string
	Content               string
	Resolved              *bool
}

// SatisfactionResultModel 区分资格状态和只读提交结果。
type SatisfactionResultModel struct {
	Id, RequirementId                    int
	State                                string
	EligibleAt, ExpiresAt, SubmittedAt   int64
	Rule                                 SatisfactionConfigModel
	OptionKey, Label, Sentiment, Content string
	Value                                int
	Tags                                 []SatisfactionTagModel
	Resolved                             *bool
	ReviewerName, ReviewerAvatar         string
}

// SatisfactionHandlerModel 保留激活时的成员角色对应关系。
type SatisfactionHandlerModel struct {
	UserId, RoleId     int
	UserName, RoleName string
}

type satisfactionQuerier interface {
	Query(string, ...any) (*sql.Rows, error)
	Exec(string, ...any) (sql.Result, error)
}

func satisfactionConnection(tx *sql.Tx) (satisfactionQuerier, error) {
	if tx != nil {
		return tx, nil
	}
	return msql.GetDB(define.Postgres)
}
func satisfactionRows(tx *sql.Tx, q string, args ...any) (*sql.Rows, error) {
	db, e := satisfactionConnection(tx)
	if e != nil {
		return nil, e
	}
	r, e := db.Query(q, args...)
	if e != nil {
		logs.Error("sql:%s, err:%s", q, e.Error())
	}
	return r, e
}
func satisfactionExec(tx *sql.Tx, q string, args ...any) (sql.Result, error) {
	db, e := satisfactionConnection(tx)
	if e != nil {
		return nil, e
	}
	r, e := db.Exec(q, args...)
	if e != nil {
		logs.Error("sql:%s, err:%s", q, e.Error())
	}
	return r, e
}
func ensureSatisfactionWorkspace(tx *sql.Tx, a, w int) error {
	rows, e := satisfactionRows(tx, "SELECT workspace_type FROM "+define.TableWorkspaces+" WHERE admin_id=$1 AND id=$2", a, w)
	if e != nil {
		return e
	}
	defer rows.Close()
	var kind string
	if !rows.Next() {
		if e = rows.Err(); e != nil {
			return e
		}
		return ErrSatisfactionForbidden
	}
	if e = rows.Scan(&kind); e != nil {
		return e
	}
	if kind != define.WorkspaceTypeIssue {
		return ErrSatisfactionForbidden
	}
	return nil
}
func defaultSatisfactionConfig(a, w int) SatisfactionConfigModel {
	return SatisfactionConfigModel{AdminId: a, WorkspaceId: w, ValiditySeconds: 86400, AutoCreatorKinds: []int{1, 2}, AutoStatusIds: []int{}, AutoTemplateIds: []int{}, RatingMode: 2, Options: []SatisfactionOptionModel{
		{Key: "satisfied", Label: "satisfaction_positive", Value: 2, Sentiment: "positive", Tags: []SatisfactionTagModel{}},
		{Key: "dissatisfied", Label: "satisfaction_negative", Value: 1, Sentiment: "negative", Tags: []SatisfactionTagModel{}},
	}}
}
func satisfactionConfigTx(tx *sql.Tx, a, w int, lock bool) (SatisfactionConfigModel, error) {
	c := defaultSatisfactionConfig(a, w)
	q := `SELECT enabled,manual_enabled,validity_seconds,auto_creator_kinds,auto_status_ids,auto_template_ids,rating_mode,rating_options,resolved_enabled,resolved_required,invitation_text,completion_text,version FROM ` + define.TableSatisfactionConfigs + ` WHERE admin_id=$1 AND workspace_id=$2`
	if lock {
		q += " FOR SHARE"
	}
	rows, e := satisfactionRows(tx, q, a, w)
	if e != nil {
		return c, e
	}
	defer rows.Close()
	if !rows.Next() {
		return c, rows.Err()
	}
	var kinds, statuses, templates pq.Int64Array
	var options []byte
	if e = rows.Scan(&c.Enabled, &c.ManualEnabled, &c.ValiditySeconds, &kinds, &statuses, &templates, &c.RatingMode, &options, &c.ResolvedEnabled, &c.ResolvedRequired, &c.InvitationText, &c.CompletionText, &c.Version); e != nil {
		return c, e
	}
	c.AutoCreatorKinds = []int{}
	for _, v := range kinds {
		c.AutoCreatorKinds = append(c.AutoCreatorKinds, int(v))
	}
	c.AutoStatusIds = []int{}
	for _, v := range statuses {
		c.AutoStatusIds = append(c.AutoStatusIds, int(v))
	}
	c.AutoTemplateIds = []int{}
	for _, v := range templates {
		c.AutoTemplateIds = append(c.AutoTemplateIds, int(v))
	}
	e = json.Unmarshal(options, &c.Options)
	return c, e
}

// GetSatisfactionConfig 读取仅适用于工单管理空间的配置。
func GetSatisfactionConfig(m SatisfactionConfigModel) (SatisfactionConfigModel, error) {
	if e := ensureSatisfactionWorkspace(nil, m.AdminId, m.WorkspaceId); e != nil {
		return m, e
	}
	return satisfactionConfigTx(nil, m.AdminId, m.WorkspaceId, false)
}
func validateSatisfactionConfig(c SatisfactionConfigModel) error {
	if c.ValiditySeconds < 3600 || c.ValiditySeconds > 30*86400 || c.RatingMode < 2 || c.RatingMode > 5 || len(c.Options) != c.RatingMode || c.Version < 0 {
		return ErrSatisfactionInvalid
	}
	if c.AutoStatusIds == nil || (c.Enabled && len(c.AutoStatusIds) == 0) {
		return ErrSatisfactionInvalid
	}
	if c.ResolvedRequired && !c.ResolvedEnabled {
		return ErrSatisfactionInvalid
	}
	if utf8.RuneCountInString(c.InvitationText) > 500 || utf8.RuneCountInString(c.CompletionText) > 500 {
		return ErrSatisfactionInvalid
	}
	keys := map[string]bool{}
	values := map[int]bool{}
	for _, o := range c.Options {
		if len(o.Key) == 0 || len(o.Key) > 64 || keys[o.Key] || o.Value < 1 || o.Value > c.RatingMode || values[o.Value] || strings.TrimSpace(o.Label) == "" || utf8.RuneCountInString(o.Label) > 100 || len(o.Tags) > 20 {
			return ErrSatisfactionInvalid
		}
		keys[o.Key] = true
		values[o.Value] = true
		if o.Sentiment != "positive" && o.Sentiment != "neutral" && o.Sentiment != "negative" {
			return ErrSatisfactionInvalid
		}
		if o.TagsRequired && len(o.Tags) == 0 {
			return ErrSatisfactionInvalid
		}
		tags := map[string]bool{}
		labels := map[string]bool{}
		for _, t := range o.Tags {
			label := strings.TrimSpace(t.Label)
			if t.Key == "" || len(t.Key) > 64 || label == "" || utf8.RuneCountInString(label) > 30 || tags[t.Key] || labels[label] {
				return ErrSatisfactionInvalid
			}
			tags[t.Key] = true
			labels[label] = true
		}
	}
	seen := map[int]bool{}
	for _, v := range c.AutoCreatorKinds {
		if (v != 1 && v != 2) || seen[v] {
			return ErrSatisfactionInvalid
		}
		seen[v] = true
	}
	if len(c.AutoStatusIds) > 0 && len(seen) == 0 {
		return ErrSatisfactionInvalid
	}
	seen = map[int]bool{}
	for _, v := range c.AutoStatusIds {
		if v <= 0 || seen[v] {
			return ErrSatisfactionInvalid
		}
		seen[v] = true
	}
	seen = map[int]bool{}
	for _, v := range c.AutoTemplateIds {
		if v <= 0 || seen[v] {
			return ErrSatisfactionInvalid
		}
		seen[v] = true
	}
	return nil
}

// SaveSatisfactionConfig 保存完整配置，版本检查不允许覆盖其他人的修改。
func SaveSatisfactionConfig(c SatisfactionConfigModel) (SatisfactionConfigModel, error) {
	if e := validateSatisfactionConfig(c); e != nil {
		return c, e
	}
	tx, e := msql.Begin(define.Postgres)
	if e != nil {
		return c, e
	}
	defer tx.Rollback()
	if e = ensureSatisfactionWorkspace(tx, c.AdminId, c.WorkspaceId); e != nil {
		return c, e
	}
	if len(c.AutoStatusIds) > 0 {
		rows, er := satisfactionRows(tx, "SELECT count(*) FROM "+define.TableWorkItemStatuses+" WHERE admin_id=$1 AND workspace_id=$2 AND id=ANY($3)", c.AdminId, c.WorkspaceId, pq.Array(c.AutoStatusIds))
		if er != nil {
			return c, er
		}
		var n int
		if rows.Next() {
			er = rows.Scan(&n)
		} else {
			er = rows.Err()
		}
		rows.Close()
		if er != nil {
			return c, er
		}
		if n != len(c.AutoStatusIds) {
			return c, ErrSatisfactionInvalid
		}
	}
	if len(c.AutoTemplateIds) > 0 {
		rows, er := satisfactionRows(tx, `SELECT count(*) FROM `+define.TableWorkspaceIssueItemTemplates+` WHERE admin_id=$1 AND workspace_id=$2 AND template_id=ANY($3) AND enabled=1`, c.AdminId, c.WorkspaceId, pq.Array(c.AutoTemplateIds))
		if er != nil {
			return c, er
		}
		var n int
		if rows.Next() {
			er = rows.Scan(&n)
		}
		rows.Close()
		if er != nil {
			return c, er
		}
		if n != len(c.AutoTemplateIds) {
			return c, ErrSatisfactionInvalid
		}
	}
	raw, e := json.Marshal(c.Options)
	if e != nil {
		return c, e
	}
	now := time.Now().Unix()
	q := `INSERT INTO ` + define.TableSatisfactionConfigs + ` (admin_id,workspace_id,enabled,manual_enabled,validity_seconds,auto_creator_kinds,auto_status_ids,auto_template_ids,rating_mode,rating_options,resolved_enabled,resolved_required,invitation_text,completion_text,version,updated_by,created_at,updated_at)
 SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,1,$15,$16,$16 WHERE $17=0
 ON CONFLICT(admin_id,workspace_id) DO NOTHING RETURNING version`
	args := []any{c.AdminId, c.WorkspaceId, boolInt(c.Enabled), boolInt(c.ManualEnabled), c.ValiditySeconds, pq.Array(c.AutoCreatorKinds), pq.Array(c.AutoStatusIds), pq.Array(c.AutoTemplateIds), c.RatingMode, string(raw), c.ResolvedEnabled, c.ResolvedRequired, c.InvitationText, c.CompletionText, c.UserId, now, c.Version}
	if c.Version > 0 {
		q = `UPDATE ` + define.TableSatisfactionConfigs + ` SET enabled=$3,manual_enabled=$4,validity_seconds=$5,auto_creator_kinds=$6,auto_status_ids=$7,auto_template_ids=$8,rating_mode=$9,rating_options=$10,resolved_enabled=$11,resolved_required=$12,invitation_text=$13,completion_text=$14,updated_by=$15,updated_at=$16,version=version+1 WHERE admin_id=$1 AND workspace_id=$2 AND version=$17 RETURNING version`
	}
	rows, e := satisfactionRows(tx, q, args...)
	if e != nil {
		return c, e
	}
	if !rows.Next() {
		er := rows.Err()
		rows.Close()
		if er != nil {
			return c, er
		}
		return c, ErrSatisfactionConflict
	}
	e = rows.Scan(&c.Version)
	rows.Close()
	if e != nil {
		return c, e
	}
	if e = tx.Commit(); e != nil {
		return c, e
	}
	return c, nil
}

// recordSatisfactionTx 仅在真正创建/变更状态的原事务中调用，读取页面不追溯激活。
func recordSatisfactionTx(tx *sql.Tx, a, w, id, previousStatus int) error {
	if tx == nil {
		return errors.New("satisfaction activation requires transaction")
	}
	rows, e := satisfactionRows(tx, `SELECT r.status_id,CASE WHEN r.created_by IS NULL THEN 1 WHEN r.created_by>0 AND EXISTS(SELECT 1 FROM users creator WHERE creator.admin_id=r.admin_id AND creator.id=r.created_by) THEN 2 ELSE 0 END AS creator_kind,ir.template_id,ir.openid,COALESCE(r.category_id,0),COALESCE(cat.name,''),COALESCE(r.priority,0),COALESCE(p.name,''),
 COALESCE((SELECT json_agg(json_build_object('UserId',u.id,'RoleId',wm.role_id,'UserName',u.name,'RoleName',ro.name)) FROM unnest(r.assignee_ids) assigned(user_id) JOIN users u ON u.id=assigned.user_id AND u.admin_id=r.admin_id JOIN workspace_members wm ON wm.admin_id=r.admin_id AND wm.workspace_id=r.workspace_id AND wm.user_id=u.id JOIN roles ro ON ro.admin_id=wm.admin_id AND ro.id=wm.role_id),'[]')
 FROM `+define.TableRequirements+` r JOIN `+define.TableWorkspaces+` w ON w.admin_id=r.admin_id AND w.id=r.workspace_id AND w.workspace_type='issue'
 JOIN `+define.TableWorkspaceIssueRequirements+` ir ON ir.admin_id=r.admin_id AND ir.workspace_id=r.workspace_id AND ir.requirement_id=r.id
 LEFT JOIN `+define.TableRequirementCategories+` cat ON cat.admin_id=r.admin_id AND cat.workspace_id=r.workspace_id AND cat.id=r.category_id
 LEFT JOIN `+define.TableWorkItemPriorities+` p ON p.admin_id=r.admin_id AND p.workspace_id=r.workspace_id AND p.id=r.priority
 WHERE r.admin_id=$1 AND r.workspace_id=$2 AND r.id=$3 AND ir.recalled_at=0`, a, w, id)
	if e != nil {
		return e
	}
	if !rows.Next() {
		er := rows.Err()
		rows.Close()
		return er
	}
	var status, creatorKind, template, category, priority int
	var openid, categoryName, priorityName string
	var handlers []byte
	e = rows.Scan(&status, &creatorKind, &template, &openid, &category, &categoryName, &priority, &priorityName, &handlers)
	rows.Close()
	if e != nil {
		return e
	}
	if previousStatus == status {
		return nil
	}
	c, e := satisfactionConfigTx(tx, a, w, true)
	if e != nil {
		return e
	}
	if !c.Enabled || !intSliceContains(c.AutoStatusIds, status) || !intSliceContains(c.AutoCreatorKinds, creatorKind) || (len(c.AutoTemplateIds) > 0 && !intSliceContains(c.AutoTemplateIds, template)) {
		return nil
	}
	c.UserId = 0
	snapshot, e := json.Marshal(c)
	if e != nil {
		return e
	}
	now := time.Now().Unix()
	rows, e = satisfactionRows(tx, `INSERT INTO `+define.TableSatisfactions+` (admin_id,workspace_id,requirement_id,openid,config_version,rule_snapshot,activation_kind,eligible_at,expires_at,rating_mode,category_id,category_name,priority_id,priority_name,handlers_snapshot,created_at,updated_at)
 VALUES($1,$2,$3,$4,$5,$6,'auto',$7,$8,$9,$10,$11,$12,$13,$14,$7,$7) ON CONFLICT(admin_id,workspace_id,requirement_id) DO NOTHING RETURNING id`, a, w, id, openid, c.Version, string(snapshot), now, now+int64(c.ValiditySeconds), c.RatingMode, category, categoryName, priority, priorityName, string(handlers))
	if e != nil {
		return e
	}
	if !rows.Next() {
		er := rows.Err()
		rows.Close()
		return er
	}
	var sid int
	e = rows.Scan(&sid)
	rows.Close()
	if e != nil {
		return e
	}
	_, e = satisfactionExec(tx, `INSERT INTO `+define.TableSatisfactionInvitations+` (admin_id,workspace_id,satisfaction_id,request_key,kind,operator_id,created_at) VALUES($1,$2,$3,$4,'auto',0,$5)`, a, w, sid, fmt.Sprintf("auto:%d", sid), now)
	return e
}
func resolveSatisfactionAccess(m SatisfactionAccessModel) (SatisfactionAccessModel, error) {
	if m.InviteKey != "" {
		if e := validate.IssueInviteOpenID(m.OpenID); e != nil {
			return m, ErrSatisfactionForbidden
		}
		c, e := ResolvePublicIssueInvite(m.InviteKey)
		if e != nil {
			return m, e
		}
		m.AdminId = c.AdminID
		m.WorkspaceId = c.WorkspaceID
	} else {
		if !HasPermission(m.AdminId, m.UserId, strconv.Itoa(m.WorkspaceId), "requirement:view") {
			return m, ErrSatisfactionForbidden
		}
		ok, e := CheckRequirementVisible(m.AdminId, strconv.Itoa(m.WorkspaceId), strconv.Itoa(m.RequirementId), m.UserId)
		if e != nil {
			return m, e
		}
		if !ok {
			return m, ErrRequirementNotFound
		}
	}
	if e := ensureSatisfactionWorkspace(nil, m.AdminId, m.WorkspaceId); e != nil {
		return m, e
	}
	return m, nil
}
func satisfactionResultTx(tx *sql.Tx, m SatisfactionAccessModel, lock bool) (SatisfactionResultModel, error) {
	result := SatisfactionResultModel{RequirementId: m.RequirementId, State: "unavailable", Tags: []SatisfactionTagModel{}}
	q := `SELECT s.id,s.rule_snapshot,s.eligible_at,s.expires_at,s.submitted_at,s.revoked_at,s.rating_option_key,COALESCE(s.rating_value,0),s.rating_label,COALESCE(s.sentiment,''),s.selected_tags,s.content,s.resolved,s.reviewer_name,s.reviewer_avatar FROM ` + define.TableSatisfactions + ` s
 JOIN ` + define.TableRequirements + ` r ON r.admin_id=s.admin_id AND r.workspace_id=s.workspace_id AND r.id=s.requirement_id
 JOIN ` + define.TableWorkspaceIssueRequirements + ` ir ON ir.admin_id=s.admin_id AND ir.workspace_id=s.workspace_id AND ir.requirement_id=s.requirement_id AND ir.openid=s.openid
 WHERE s.admin_id=$1 AND s.workspace_id=$2 AND s.requirement_id=$3`
	args := []any{m.AdminId, m.WorkspaceId, m.RequirementId}
	if m.InviteKey != "" {
		q += " AND s.openid=$4"
		args = append(args, m.OpenID)
	}
	if lock {
		q += " FOR UPDATE OF s"
	}
	rows, e := satisfactionRows(tx, q, args...)
	if e != nil {
		return result, e
	}
	defer rows.Close()
	if !rows.Next() {
		return result, rows.Err()
	}
	var snapshot, tags []byte
	var revoked int64
	var resolved sql.NullBool
	if e = rows.Scan(&result.Id, &snapshot, &result.EligibleAt, &result.ExpiresAt, &result.SubmittedAt, &revoked, &result.OptionKey, &result.Value, &result.Label, &result.Sentiment, &tags, &result.Content, &resolved, &result.ReviewerName, &result.ReviewerAvatar); e != nil {
		return result, e
	}
	rows.Close()
	if result.SubmittedAt > 0 {
		if e = json.Unmarshal(snapshot, &result.Rule); e != nil {
			return result, e
		}
	} else {
		// 待评价读取最新表单；提交事务持有配置共享锁，防止校验后配置又被修改。
		result.Rule, e = satisfactionConfigTx(tx, m.AdminId, m.WorkspaceId, lock)
		if e != nil {
			return result, e
		}
	}
	if e = json.Unmarshal(tags, &result.Tags); e != nil {
		return result, e
	}
	if resolved.Valid {
		v := resolved.Bool
		result.Resolved = &v
	}
	result.State = satisfactionState(result.SubmittedAt, revoked, result.ExpiresAt, time.Now().Unix())
	return result, nil
}

// GetSatisfaction 返回资格及结果，已提交优先于到期状态且不会返回客户归属标识。
func GetSatisfaction(m SatisfactionAccessModel) (SatisfactionResultModel, error) {
	m, e := resolveSatisfactionAccess(m)
	if e != nil {
		if m.InviteKey != "" && errors.Is(e, ErrSatisfactionForbidden) {
			return SatisfactionResultModel{RequirementId: m.RequirementId, State: "unavailable", Tags: []SatisfactionTagModel{}}, nil
		}
		return SatisfactionResultModel{}, e
	}
	return satisfactionResultTx(nil, m, false)
}
func validateSatisfactionSubmission(m SatisfactionSubmitModel, r SatisfactionResultModel) (SatisfactionResultModel, error) {
	var chosen *SatisfactionOptionModel
	for i := range r.Rule.Options {
		if r.Rule.Options[i].Key == m.OptionKey {
			chosen = &r.Rule.Options[i]
			break
		}
	}
	if chosen == nil {
		return r, ErrSatisfactionInvalid
	}
	content := strings.TrimSpace(m.Content)
	if utf8.RuneCountInString(content) > 200 || (chosen.CommentRequired && content == "") {
		return r, ErrSatisfactionInvalid
	}
	tags := []SatisfactionTagModel{}
	seen := map[string]bool{}
	for _, key := range m.TagKeys {
		if seen[key] {
			return r, ErrSatisfactionInvalid
		}
		seen[key] = true
		found := false
		for _, t := range chosen.Tags {
			if t.Key == key {
				tags = append(tags, t)
				found = true
				break
			}
		}
		if !found {
			return r, ErrSatisfactionInvalid
		}
	}
	if chosen.TagsRequired && len(tags) == 0 {
		return r, ErrSatisfactionInvalid
	}
	if (!r.Rule.ResolvedEnabled && m.Resolved != nil) || (r.Rule.ResolvedRequired && m.Resolved == nil) {
		return r, ErrSatisfactionInvalid
	}
	r.OptionKey = chosen.Key
	r.Value = chosen.Value
	r.Label = chosen.Label
	r.Sentiment = chosen.Sentiment
	r.Tags = tags
	r.Content = content
	r.Resolved = m.Resolved
	return r, nil
}

// SubmitSatisfaction 原子写入一次客户评价，重复网络请求不会变成第二次提交。
func SubmitSatisfaction(m SatisfactionSubmitModel) (SatisfactionResultModel, error) {
	if m.Access.InviteKey == "" {
		return SatisfactionResultModel{}, ErrSatisfactionForbidden
	}
	access, e := resolveSatisfactionAccess(m.Access)
	if e != nil {
		return SatisfactionResultModel{}, e
	}
	tx, e := msql.Begin(define.Postgres)
	if e != nil {
		return SatisfactionResultModel{}, e
	}
	defer tx.Rollback()
	// 与状态更新共用工单行锁，防止转移/删除和提交交错。
	rows, e := satisfactionRows(tx, "SELECT id FROM "+define.TableRequirements+" WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 FOR UPDATE", access.AdminId, access.WorkspaceId, access.RequirementId)
	if e != nil {
		return SatisfactionResultModel{}, e
	}
	ok := rows.Next()
	e = rows.Err()
	rows.Close()
	if e != nil {
		return SatisfactionResultModel{}, e
	}
	if !ok {
		return SatisfactionResultModel{}, ErrRequirementNotFound
	}
	old, e := satisfactionResultTx(tx, access, true)
	if e != nil {
		return old, e
	}
	if old.State != "submitted" && (old.State != "available" || m.ExpectedConfigVersion <= 0 || m.ExpectedConfigVersion != old.Rule.Version) {
		return old, ErrSatisfactionConflict
	}
	next, e := validateSatisfactionSubmission(m, old)
	if e != nil {
		return old, e
	}
	if old.State == "submitted" {
		a, er := json.Marshal(old.Tags)
		if er != nil {
			return old, er
		}
		b, er := json.Marshal(next.Tags)
		if er != nil {
			return old, er
		}
		sameResolved := (old.Resolved == nil && next.Resolved == nil) || (old.Resolved != nil && next.Resolved != nil && *old.Resolved == *next.Resolved)
		if old.OptionKey == next.OptionKey && old.Content == next.Content && string(a) == string(b) && sameResolved {
			return old, nil
		}
		return old, ErrSatisfactionConflict
	}
	now := time.Now().Unix()
	if now >= old.ExpiresAt {
		return old, ErrSatisfactionConflict
	}
	tags, e := json.Marshal(next.Tags)
	if e != nil {
		return old, e
	}
	snapshot, e := json.Marshal(next.Rule)
	if e != nil {
		return old, e
	}
	_, e = satisfactionExec(tx, `UPDATE `+define.TableSatisfactions+` SET rating_option_key=$4,rating_value=$5,rating_label=$6,sentiment=$7,selected_tags=$8,content=$9,resolved=$10,submitted_at=$11,updated_at=$11,rule_snapshot=$12,config_version=$13,rating_mode=$14 WHERE admin_id=$1 AND workspace_id=$2 AND id=$3 AND submitted_at=0`, access.AdminId, access.WorkspaceId, old.Id, next.OptionKey, next.Value, next.Label, next.Sentiment, string(tags), next.Content, next.Resolved, now, string(snapshot), next.Rule.Version, next.Rule.RatingMode)
	if e != nil {
		return old, e
	}
	if e = tx.Commit(); e != nil {
		return old, e
	}
	next.SubmittedAt = now
	next.State = "submitted"
	return next, nil
}

func clearSatisfactionItemsTx(tx *sql.Tx, a, w int, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if _, e := satisfactionExec(tx, `DELETE FROM `+define.TableSatisfactionInvitations+` WHERE admin_id=$1 AND workspace_id=$2 AND satisfaction_id IN (SELECT id FROM `+define.TableSatisfactions+` WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=ANY($3))`, a, w, pq.Array(ids)); e != nil {
		return e
	}
	_, e := satisfactionExec(tx, `DELETE FROM `+define.TableSatisfactions+` WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=ANY($3)`, a, w, pq.Array(ids))
	return e
}
func revokeSatisfactionTx(tx *sql.Tx, a, w, id int, reason string) error {
	_, e := satisfactionExec(tx, `UPDATE `+define.TableSatisfactions+` SET revoked_at=$4,revoke_reason=$5,updated_at=$4 WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND submitted_at=0 AND revoked_at=0`, a, w, id, time.Now().Unix(), reason)
	return e
}

// SatisfactionStateModel 是客户列表的轻量能力投影。
type SatisfactionStateModel struct {
	RequirementId int
	State         string
}

// ListPublicSatisfactionStates 批量读取当前页，避免逐工单查询或污染已有邀填缓存。
func ListPublicSatisfactionStates(m SatisfactionAccessModel, ids []int) ([]SatisfactionStateModel, error) {
	result := []SatisfactionStateModel{}
	if len(ids) > 50 {
		return result, ErrSatisfactionInvalid
	}
	for _, id := range ids {
		if id <= 0 {
			return result, ErrSatisfactionInvalid
		}
	}
	if e := validate.IssueInviteOpenID(m.OpenID); e != nil {
		return result, ErrSatisfactionForbidden
	}
	scope, e := ResolvePublicIssueInvite(m.InviteKey)
	if e != nil {
		return result, e
	}
	rows, e := satisfactionRows(nil, `SELECT s.requirement_id,s.submitted_at,s.expires_at,s.revoked_at FROM `+define.TableSatisfactions+` s
 JOIN `+define.TableWorkspaces+` w ON w.admin_id=s.admin_id AND w.id=s.workspace_id AND w.workspace_type='issue'
 JOIN `+define.TableRequirements+` r ON r.admin_id=s.admin_id AND r.workspace_id=s.workspace_id AND r.id=s.requirement_id
 JOIN `+define.TableWorkspaceIssueRequirements+` ir ON ir.admin_id=s.admin_id AND ir.workspace_id=s.workspace_id AND ir.requirement_id=s.requirement_id AND ir.openid=s.openid
 WHERE s.admin_id=$1 AND s.workspace_id=$2 AND s.openid=$3 AND s.requirement_id=ANY($4)`, scope.AdminID, scope.WorkspaceID, m.OpenID, pq.Array(ids))
	if e != nil {
		return result, e
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var submitted, expiry, revoked int64
		if e = rows.Scan(&id, &submitted, &expiry, &revoked); e != nil {
			return result, e
		}
		state := satisfactionState(submitted, revoked, expiry, time.Now().Unix())
		result = append(result, SatisfactionStateModel{RequirementId: id, State: state})
	}
	return result, rows.Err()
}

// 已提交结果必须先于到期判断，避免客户在窗口结束后无法查看评价。
func satisfactionState(submitted, revoked, expiry, now int64) string {
	if submitted > 0 {
		return "submitted"
	}
	if revoked > 0 {
		return "revoked"
	}
	if now >= expiry {
		return "expired"
	}
	return "available"
}
