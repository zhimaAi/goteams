// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_redis"

	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// issueInviteCacheTTL is the one-day lazy cache lifetime.
var issueInviteCacheTTL = time.Duration(define.IssueInviteCacheTTLSeconds) * time.Second

// sha256OpenID hashes an external identifier so it never appears verbatim in Redis keys.
func sha256OpenID(openID string) string {
	sum := sha256.Sum256([]byte(openID))
	return hex.EncodeToString(sum[:])
}

// joinIssueInviteCacheKey joins parts with a colon delimiter.
func joinIssueInviteCacheKey(parts ...string) string {
	return strings.Join(parts, `:`)
}

// BuildIssueInviteConfigCacheKey builds a tenant/project-scoped public config cache key.
func BuildIssueInviteConfigCacheKey(adminID, workspaceID int, inviteKey string) string {
	return joinIssueInviteCacheKey(
		define.CacheKeyIssueInviteConfig,
		strconv.Itoa(adminID),
		strconv.Itoa(workspaceID),
		inviteKey,
	)
}

// BuildIssueInviteTemplateCacheKey builds a tenant/project-scoped template cache key.
func BuildIssueInviteTemplateCacheKey(adminID, workspaceID int, templateKey string) string {
	return joinIssueInviteCacheKey(
		define.CacheKeyIssueInviteTemplate,
		strconv.Itoa(adminID),
		strconv.Itoa(workspaceID),
		templateKey,
	)
}

// BuildIssueInviteHistoryCacheKey builds a versioned fixed-page public history cache key.
func BuildIssueInviteHistoryCacheKey(adminID, workspaceID int, openID string, version, renderVersion int64, page int) string {
	return joinIssueInviteCacheKey(
		define.CacheKeyIssueInviteHistory,
		strconv.Itoa(adminID),
		strconv.Itoa(workspaceID),
		sha256OpenID(openID),
		strconv.FormatInt(version, 10),
		strconv.FormatInt(renderVersion, 10),
		strconv.Itoa(page),
	)
}

// BuildIssueInviteDetailCacheKey builds a submitter-scoped public detail cache key at one render version.
func BuildIssueInviteDetailCacheKey(adminID, workspaceID int, openID string, version int64, requirementID int) string {
	return joinIssueInviteCacheKey(
		define.CacheKeyIssueInviteDetail,
		strconv.Itoa(adminID),
		strconv.Itoa(workspaceID),
		sha256OpenID(openID),
		strconv.FormatInt(version, 10),
		strconv.Itoa(requirementID),
	)
}

// issueInviteHistoryVersionKey builds the submitter history namespace version marker.
func issueInviteHistoryVersionKey(adminID, workspaceID int, openID string) string {
	return joinIssueInviteCacheKey(
		define.CacheKeyIssueInviteHistoryVersion,
		strconv.Itoa(adminID),
		strconv.Itoa(workspaceID),
		sha256OpenID(openID),
	)
}

// cacheGetJSON reads and decodes one cached JSON value; found is false on a cache miss.
func cacheGetJSON(key string, target any) (bool, error) {
	raw, err := lib_redis.Get(define.Redis, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		logs.Error("issue invite redis get error, key:%s, err:%s", key, err.Error())
		return false, err
	}
	if raw == `` || raw == `-1` {
		return false, nil
	}
	if err = tool.JsonDecodeUseNumber(raw, target); err != nil {
		logs.Error("issue invite cache decode error, key:%s, err:%s", key, err.Error())
		return false, err
	}
	return true, nil
}

// cacheSetJSON writes one JSON value without negative-caching empty results.
func cacheSetJSON(key string, value any, ttl time.Duration) error {
	encoded, err := tool.JsonEncode(value)
	if err != nil {
		logs.Error("issue invite cache encode error, key:%s, err:%s", key, err.Error())
		return err
	}
	if encoded == `` || encoded == `{}` || encoded == `[]` {
		return nil
	}
	if err = lib_redis.Set(define.Redis, key, encoded, ttl); err != nil {
		logs.Error("issue invite redis set error, key:%s, err:%s", key, err.Error())
		return err
	}
	return nil
}

// GetCachedIssueInviteConfig lazily loads the public configuration for one scoped invite.
func GetCachedIssueInviteConfig(scope define.IssueInviteConfigModel) (define.IssueInviteConfigModel, []define.IssueInviteTemplateModel, error) {
	key := BuildIssueInviteConfigCacheKey(scope.AdminID, scope.WorkspaceID, scope.InviteKey)
	var cached struct {
		Config    define.IssueInviteConfigModel     `json:"config"`
		Templates []define.IssueInviteTemplateModel `json:"templates"`
	}
	if found, err := cacheGetJSON(key, &cached); err != nil {
		return define.IssueInviteConfigModel{}, nil, err
	} else if found {
		return cached.Config, cached.Templates, nil
	}

	config, templates, err := loadIssueInviteConfigAndTemplates(scope.AdminID, scope.WorkspaceID, scope.InviteKey)
	if err != nil {
		return define.IssueInviteConfigModel{}, nil, err
	}
	_ = cacheSetJSON(key, struct {
		Config    define.IssueInviteConfigModel     `json:"config"`
		Templates []define.IssueInviteTemplateModel `json:"templates"`
	}{Config: config, Templates: templates}, issueInviteCacheTTL)
	return config, templates, nil
}

// GetCachedIssueInviteTemplate lazily loads one enabled copied public template.
func GetCachedIssueInviteTemplate(scope define.IssueInviteConfigModel, templateKey string) (define.IssueInviteTemplateModel, error) {
	key := BuildIssueInviteTemplateCacheKey(scope.AdminID, scope.WorkspaceID, templateKey)
	// 旧缓存里没有 GuideEnabled 键，直接解到 model 会得到零值 0，把存量文案错当成关闭；
	// 外层指针影子字段区分「缓存里还没有这一列」和「真的关着」，前者按开启回落。
	var cached struct {
		define.IssueInviteTemplateModel
		GuideEnabled *int `json:"GuideEnabled"`
	}
	if found, err := cacheGetJSON(key, &cached); err != nil {
		return define.IssueInviteTemplateModel{}, err
	} else if found {
		template := cached.IssueInviteTemplateModel
		if cached.GuideEnabled == nil {
			template.GuideEnabled = 1
		}
		return template, nil
	}
	template, err := getEnabledIssueInviteTemplateByKey(scope.AdminID, scope.WorkspaceID, templateKey)
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	_ = cacheSetJSON(key, template, issueInviteCacheTTL)
	return template, nil
}

// GetCachedIssueInviteHistory lazily loads one fixed-size public history page.
func GetCachedIssueInviteHistory(scope define.IssueInviteConfigModel, openID string, page int) (define.IssueInviteHistoryModel, error) {
	version, err := getIssueInviteHistoryVersion(scope.AdminID, scope.WorkspaceID, openID)
	if err != nil {
		return define.IssueInviteHistoryModel{}, err
	}
	renderVersion, err := getIssueInviteRenderVersion(scope.AdminID, scope.WorkspaceID)
	if err != nil {
		return define.IssueInviteHistoryModel{}, err
	}
	key := BuildIssueInviteHistoryCacheKey(scope.AdminID, scope.WorkspaceID, openID, version, renderVersion, page)
	var cached define.IssueInviteHistoryModel
	if found, err := cacheGetJSON(key, &cached); err != nil {
		return define.IssueInviteHistoryModel{}, err
	} else if found {
		return cached, nil
	}
	history, err := listPublicIssueHistoryData(scope.AdminID, scope.WorkspaceID, openID, page)
	if err != nil {
		return define.IssueInviteHistoryModel{}, err
	}
	_ = cacheSetJSON(key, history, issueInviteCacheTTL)
	return history, nil
}

// GetCachedIssueInviteDetail lazily loads one read-only public issue detail.
func GetCachedIssueInviteDetail(scope define.IssueInviteConfigModel, openID string, requirementID int) (define.IssueInviteRequirementModel, error) {
	version, err := getIssueInviteRenderVersion(scope.AdminID, scope.WorkspaceID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	key := BuildIssueInviteDetailCacheKey(scope.AdminID, scope.WorkspaceID, openID, version, requirementID)
	var cached define.IssueInviteRequirementModel
	if found, err := cacheGetJSON(key, &cached); err != nil {
		return define.IssueInviteRequirementModel{}, err
	} else if found {
		// 催单窗口跟当前时间有关，缓存里那份会过期，读出来按现在的时间重算
		applyIssueInviteUrgeWindow(&cached, int64(tool.Time2Int()))
		return cached, nil
	}
	detail, err := getPublicIssueDetailData(scope.AdminID, scope.WorkspaceID, openID, requirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	_ = cacheSetJSON(key, detail, issueInviteCacheTTL)
	return detail, nil
}

// loadIssueInviteConfigAndTemplates builds the uncached public config plus enabled templates.
func loadIssueInviteConfigAndTemplates(adminID, workspaceID int, inviteKey string) (define.IssueInviteConfigModel, []define.IssueInviteTemplateModel, error) {
	config, err := GetIssueInviteConfig(adminID, workspaceID)
	if err != nil {
		return define.IssueInviteConfigModel{}, nil, err
	}
	if config.InviteKey != inviteKey {
		return define.IssueInviteConfigModel{}, nil, ErrIssueInviteInvalid
	}
	templates, err := ListIssueInviteTemplates(adminID, workspaceID, true)
	if err != nil {
		return define.IssueInviteConfigModel{}, nil, err
	}
	return config, templates, nil
}

// getEnabledIssueInviteTemplateByKey returns an enabled template by public key.
func getEnabledIssueInviteTemplateByKey(adminID, workspaceID int, templateKey string) (define.IssueInviteTemplateModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT `+issueInviteTemplateFields+`, a.enabled, a.sort_order
		FROM `+define.TableWorkspaceIssueItemTemplates+` a
		JOIN `+define.TableWorkItemTemplates+` t
			ON t.admin_id = a.admin_id AND t.id = a.template_id
		WHERE a.admin_id = $1 AND a.workspace_id = $2 AND t.invite_key = $3`, nil, adminID, workspaceID, templateKey)
	if err != nil {
		logs.Error("get enabled issue invite template by key failed, err:%s", err.Error())
		return define.IssueInviteTemplateModel{}, err
	}
	if len(rows) == 0 {
		return define.IssueInviteTemplateModel{}, ErrIssueInviteTemplateNotFound
	}
	converted, err := toIssueInviteTemplateRow(rows[0])
	if err != nil {
		return define.IssueInviteTemplateModel{}, err
	}
	if converted.Enabled != 1 {
		return define.IssueInviteTemplateModel{}, ErrIssueInviteTemplateDisabled
	}
	return rowToIssueInviteTemplateModel(converted, adminID, workspaceID), nil
}

// issueInviteRenderVersionKey builds the project render-version marker key.
func issueInviteRenderVersionKey(adminID, workspaceID int) string {
	return joinIssueInviteCacheKey(
		define.CacheKeyIssueInviteRenderVersion,
		strconv.Itoa(adminID),
		strconv.Itoa(workspaceID),
	)
}

// getIssueInviteRenderVersion returns the current project render version (defaults to zero).
func getIssueInviteRenderVersion(adminID, workspaceID int) (int64, error) {
	key := issueInviteRenderVersionKey(adminID, workspaceID)
	raw, err := lib_redis.Get(define.Redis, key)
	if err != nil && !errors.Is(err, redis.Nil) {
		logs.Error("issue invite render version get error, err:%s", err.Error())
		return 0, err
	}
	if raw == `` {
		return 0, nil
	}
	version, convErr := strconv.ParseInt(raw, 10, 64)
	if convErr != nil || version < 0 {
		// A corrupted marker could never be INCR-ed later; drop it so the next rotation restarts at zero.
		if delErr := lib_redis.Del(define.Redis, key); delErr != nil {
			logs.Error("issue invite render version reset error, key:%s, err:%s", key, delErr.Error())
			return 0, delErr
		}
		logs.Error("issue invite render version corrupt, key:%s, raw:%s", key, raw)
		return 0, nil
	}
	return version, nil
}

// RotateIssueInviteRenderVersion 递增工作空间渲染版本，使公开列表和详情缓存同时失效。
// 配置或自动化变更提交后调用；旧缓存继续按原 TTL 过期。
func RotateIssueInviteRenderVersion(adminID, workspaceID int) error {
	key := issueInviteRenderVersionKey(adminID, workspaceID)
	ctx := context.Background()
	if _, err := define.Redis.Incr(ctx, key).Result(); err != nil {
		logs.Error("rotate issue invite render version failed, err:%s", err.Error())
		return err
	}
	// The marker must outlive the snapshots it namespaces, otherwise a stale snapshot could
	// outlive the marker and be read again under a reset version.
	if err := define.Redis.Expire(ctx, key, 2*issueInviteCacheTTL).Err(); err != nil {
		logs.Error("set issue invite render version ttl failed, err:%s", err.Error())
		return err
	}
	return nil
}

// getIssueInviteHistoryVersion returns the current submitter history version (defaults to zero).
func getIssueInviteHistoryVersion(adminID, workspaceID int, openID string) (int64, error) {
	key := issueInviteHistoryVersionKey(adminID, workspaceID, openID)
	raw, err := lib_redis.Get(define.Redis, key)
	if err != nil && !errors.Is(err, redis.Nil) {
		logs.Error("issue invite history version get error, err:%s", err.Error())
		return 0, err
	}
	if raw == `` {
		return 0, nil
	}
	version, convErr := strconv.ParseInt(raw, 10, 64)
	if convErr != nil || version < 0 {
		// A corrupted marker could never be INCR-ed later; drop it so the next rotation restarts at zero.
		if delErr := lib_redis.Del(define.Redis, key); delErr != nil {
			logs.Error("issue invite history version reset error, key:%s, err:%s", key, delErr.Error())
			return 0, delErr
		}
		logs.Error("issue invite history version corrupt, key:%s, raw:%s", key, raw)
		return 0, nil
	}
	return version, nil
}

// listPublicIssueHistoryData returns one fixed ten-row submitter history page.
func listPublicIssueHistoryData(adminID, workspaceID int, openID string, page int) (define.IssueInviteHistoryModel, error) {
	if page < 1 {
		page = 1
	}
	pageSize := define.IssueInvitePublicPageSize
	where := `ir.admin_id = $1 AND ir.workspace_id = $2 AND ir.openid = $3`
	countRows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total
		FROM `+define.TableWorkspaceIssueRequirements+` ir WHERE `+where, nil, adminID, workspaceID, openID)
	if err != nil {
		logs.Error("count public issue history failed, err:%s", err.Error())
		return define.IssueInviteHistoryModel{}, err
	}
	total := 0
	if len(countRows) > 0 {
		total = cast.ToInt(countRows[0][`total`])
	}

	offset := (page - 1) * pageSize
	rows, err := msql.RawValues(define.Postgres, `SELECT r.id, r.title, r.description, r.created_at, r.updated_at,
		r.status_id, r.priority, ir.template_name, ir.template_type_key,
		a.guide_description,
		s.name AS status_name, s.color AS status_color, p.name AS priority_name
		FROM `+define.TableWorkspaceIssueRequirements+` ir
		JOIN `+define.TableRequirements+` r ON r.id = ir.requirement_id AND r.admin_id = ir.admin_id
		LEFT JOIN `+define.TableWorkspaceIssueItemTemplates+` a
			ON a.admin_id = ir.admin_id AND a.workspace_id = ir.workspace_id AND a.template_id = ir.template_id
		LEFT JOIN `+define.TableWorkItemStatuses+` s
			ON s.id = r.status_id AND s.admin_id = ir.admin_id AND s.workspace_id = ir.workspace_id AND s.type_key = ir.template_type_key
		LEFT JOIN `+define.TableWorkItemPriorities+` p
			ON p.id = r.priority AND p.admin_id = ir.admin_id AND p.workspace_id = ir.workspace_id AND p.type_key = ir.template_type_key
		WHERE `+where+`
		ORDER BY ir.created_at DESC, ir.id DESC
		LIMIT $4 OFFSET $5`, nil, adminID, workspaceID, openID, pageSize, offset)
	if err != nil {
		logs.Error("list public issue history failed, err:%s", err.Error())
		return define.IssueInviteHistoryModel{}, err
	}

	items := make([]define.IssueInviteRequirementModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, define.IssueInviteRequirementModel{
			RequirementID:    cast.ToInt(row[`id`]),
			Title:            row[`title`],
			Description:      row[`description`],
			GuideDescription: row[`guide_description`],
			TemplateName:     row[`template_name`],
			TemplateType:     row[`template_type_key`],
			StatusName:       row[`status_name`],
			StatusColor:      row[`status_color`],
			PriorityName:     row[`priority_name`],
			CreatedAt:        cast.ToInt64(row[`created_at`]),
			UpdatedAt:        cast.ToInt64(row[`updated_at`]),
		})
	}
	return define.IssueInviteHistoryModel{
		Items: items, Page: page, PageSize: pageSize, Total: total,
	}, nil
}

// getPublicIssueDetailData returns one submitter-owned read-only issue detail.
func getPublicIssueDetailData(adminID, workspaceID int, openID string, requirementID int) (define.IssueInviteRequirementModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT r.id, r.title, r.description, r.created_at, r.updated_at,
		r.status_id, r.priority, r.version_id, r.category_id, r.planned_start_date, r.planned_end_date,
		r.work_hours, r.completed_work_hours,
		COALESCE(array_to_json(r.assignee_ids), '[]')::text AS edit_assignee_ids,
		ir.template_name, ir.template_type_key, ir.template_key_snapshot AS template_key,
		a.guide_description, COALESCE(a.recall_enabled, 0) AS recall_enabled,
		t.fields_json AS template_fields_json,
		ir.recalled_at, ir.last_urged_at,
		s.name AS status_name, s.color AS status_color, p.name AS priority_name,
		v.name AS version_name, c.name AS category_name,
		COALESCE(_assignee.assignee_names, '[]') AS assignee_names
		FROM `+define.TableWorkspaceIssueRequirements+` ir
		JOIN `+define.TableRequirements+` r ON r.id = ir.requirement_id AND r.admin_id = ir.admin_id
		LEFT JOIN `+define.TableWorkspaceIssueItemTemplates+` a
			ON a.admin_id = ir.admin_id AND a.workspace_id = ir.workspace_id AND a.template_id = ir.template_id
		LEFT JOIN `+define.TableWorkItemTemplates+` t
			ON t.admin_id = a.admin_id AND t.id = a.template_id
		LEFT JOIN `+define.TableVersions+` v
			ON v.id = r.version_id AND v.admin_id = r.admin_id AND v.workspace_id = r.workspace_id
		LEFT JOIN `+define.TableRequirementCategories+` c
			ON c.id = r.category_id AND c.admin_id = r.admin_id AND c.workspace_id = r.workspace_id
		LEFT JOIN `+define.TableWorkItemStatuses+` s
			ON s.id = r.status_id AND s.admin_id = ir.admin_id AND s.workspace_id = ir.workspace_id AND s.type_key = ir.template_type_key
		LEFT JOIN `+define.TableWorkItemPriorities+` p
			ON p.id = r.priority AND p.admin_id = ir.admin_id AND p.workspace_id = ir.workspace_id AND p.type_key = ir.template_type_key
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(COALESCE(u.name, '') ORDER BY assigned.ordinality), '[]'::json)::text AS assignee_names
			FROM unnest(r.assignee_ids) WITH ORDINALITY AS assigned(user_id, ordinality)
			JOIN users u ON u.id = assigned.user_id AND u.admin_id = r.admin_id
		) _assignee ON true
		WHERE ir.admin_id = $1 AND ir.workspace_id = $2 AND ir.openid = $3 AND ir.requirement_id = $4`,
		nil, adminID, workspaceID, openID, requirementID)
	if err != nil {
		logs.Error("get public issue detail failed, err:%s", err.Error())
		return define.IssueInviteRequirementModel{}, err
	}
	if len(rows) == 0 {
		return define.IssueInviteRequirementModel{}, ErrRequirementNotFound
	}
	row := rows[0]
	detail := define.IssueInviteRequirementModel{
		RequirementID:    requirementID,
		Title:            row[`title`],
		Description:      row[`description`],
		GuideDescription: row[`guide_description`],
		TemplateName:     row[`template_name`],
		TemplateType:     row[`template_type_key`],
		TemplateKey:      row[`template_key`],
		StatusName:       row[`status_name`],
		StatusColor:      row[`status_color`],
		CreatedAt:        cast.ToInt64(row[`created_at`]),
		UpdatedAt:        cast.ToInt64(row[`updated_at`]),
	}
	recalled := cast.ToInt64(row[`recalled_at`]) > 0
	detail.RecallEnabled = cast.ToInt(row[`recall_enabled`]) == 1
	detail.Recalled = recalled
	detail.LastUrgedAt = cast.ToInt64(row[`last_urged_at`])
	// 终态和已撤回都能改并重新提交，其余状态只能撤回
	isFinal, err := isFinalIssueInviteStatus(adminID, workspaceID, detail.TemplateType, cast.ToInt(row[`status_id`]))
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	detail.CanModify = recalled || isFinal
	detail.CanRecall = detail.RecallEnabled && !recalled && !isFinal
	applyIssueInviteUrgeWindow(&detail, int64(tool.Time2Int()))
	detail.EditFields = buildIssueInviteEditFields(row)
	detail.Fields = buildIssueInviteBuiltinFields(row)
	// 自定义字段排在最后，按配置顺序取，别用 map 的遍历顺序（每次请求都不一样）
	values, err := ListCustomFieldValues(adminID, requirementID)
	if err != nil {
		return define.IssueInviteRequirementModel{}, err
	}
	if len(values) > 0 {
		customFields, fieldErr := ListCustomFields(ListCustomFieldsModel{AdminId: adminID, WsId: cast.ToString(workspaceID), TypeKey: detail.TemplateType})
		if fieldErr != nil {
			return define.IssueInviteRequirementModel{}, fieldErr
		}
		for _, field := range customFields.Items {
			value, ok := values[cast.ToString(field.Id)]
			if !ok {
				continue
			}
			// 名字和类型带出去，公开详情好按类型渲染下拉/日期值，也不用暴露字段 key
			detail.Fields = append(detail.Fields, define.IssueInviteFieldValue{
				FieldKey: `custom:` + cast.ToString(field.Id),
				Name:     field.Name,
				Type:     field.FieldType,
				Value:    formatIssueInviteFieldValue(field, value),
			})
			// 原始值原样带出去：下拉要的是选项 key，展示值那边已经换成名字了
			detail.EditFields = append(detail.EditFields, define.IssueInviteFieldValue{
				FieldKey: `custom:` + cast.ToString(field.Id),
				Name:     field.Name,
				Type:     field.FieldType,
				Value:    value,
			})
		}
	}
	// 展示顺序跟着模板字段列表的拖动顺序走
	sortIssueInviteDetailFields(detail.Fields, row[`template_fields_json`])
	return detail, nil
}

// sortIssueInviteDetailFields 把详情页的字段列表排成模板配置的顺序。模板里没记到的字段
// （老数据、项目后加的字段）保持原来的相对顺序，排在最后。
func sortIssueInviteDetailFields(fields []define.IssueInviteFieldValue, fieldsJSON string) {
	stored, err := decodeTemplateFields(fieldsJSON)
	if err != nil || len(stored) == 0 {
		return
	}
	rank := make(map[string]int, len(stored))
	for i, field := range stored {
		if _, exists := rank[field.FieldKey]; !exists {
			rank[field.FieldKey] = i
		}
	}
	unranked := len(stored)
	position := func(field define.IssueInviteFieldValue) int {
		if index, exists := rank[field.FieldKey]; exists {
			return index
		}
		return unranked
	}
	sort.SliceStable(fields, func(i, j int) bool {
		return position(fields[i]) < position(fields[j])
	})
}

// issueInviteDetailBuiltinOrder 是详情页内置字段的展示顺序，跟属性字段的预设顺序一致。
var issueInviteDetailBuiltinOrder = []string{
	`version`, `category`, `priority`, `planned_start_date`, `planned_end_date`,
	`assignee`, `estimated_work_hours`, `completed_work_hours`,
}

// buildIssueInviteBuiltinFields 挑出有值的内置字段，按预设顺序给展示值。
// 日期给时间戳、工时给数字，具体怎么显示由前端按浏览器时区和单位决定。
func buildIssueInviteBuiltinFields(row map[string]string) []define.IssueInviteFieldValue {
	values := make(map[string]any, len(issueInviteDetailBuiltinOrder))
	if name := row[`version_name`]; name != `` {
		values[`version`] = name
	}
	if name := row[`category_name`]; name != `` {
		values[`category`] = name
	}
	if name := row[`priority_name`]; name != `` {
		values[`priority`] = name
	}
	if timestamp := cast.ToInt64(row[`planned_start_date`]); timestamp > 0 {
		values[`planned_start_date`] = timestamp
	}
	if timestamp := cast.ToInt64(row[`planned_end_date`]); timestamp > 0 {
		values[`planned_end_date`] = timestamp
	}
	var assigneeNames []string
	if raw := row[`assignee_names`]; raw != `` && raw != `[]` {
		_ = json.Unmarshal([]byte(raw), &assigneeNames)
	}
	if len(assigneeNames) > 0 {
		values[`assignee`] = assigneeNames
	}
	if hours := cast.ToFloat64(row[`work_hours`]); hours > 0 {
		values[`estimated_work_hours`] = hours
	}
	if hours := cast.ToFloat64(row[`completed_work_hours`]); hours > 0 {
		values[`completed_work_hours`] = hours
	}
	fields := make([]define.IssueInviteFieldValue, 0, len(values))
	for _, key := range issueInviteDetailBuiltinOrder {
		if value, ok := values[key]; ok {
			fields = append(fields, define.IssueInviteFieldValue{FieldKey: key, Value: value})
		}
	}
	return fields
}

// buildIssueInviteEditFields 把这张工单的内置字段值整理成编辑态预填数据。
// 值按提交接口收的格式给：下拉给 id、日期给时间戳、工时给数字。
func buildIssueInviteEditFields(row map[string]string) []define.IssueInviteFieldValue {
	// 描述里的附件块是服务端拼的，编辑态只给正文，用户在文本框里看不到那段 HTML
	body, _ := splitIssueInviteAttachmentBlock(row[`description`])
	fields := []define.IssueInviteFieldValue{
		{FieldKey: `title`, Value: row[`title`]},
		{FieldKey: `description`, Value: body},
	}
	if id := cast.ToInt(row[`status_id`]); id > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `status`, Value: id})
	}
	if id := cast.ToInt(row[`version_id`]); id > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `version`, Value: id})
	}
	if id := cast.ToInt(row[`category_id`]); id > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `category`, Value: id})
	}
	if id := cast.ToInt(row[`priority`]); id > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `priority`, Value: id})
	}
	if timestamp := cast.ToInt64(row[`planned_start_date`]); timestamp > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `planned_start_date`, Value: timestamp})
	}
	if timestamp := cast.ToInt64(row[`planned_end_date`]); timestamp > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `planned_end_date`, Value: timestamp})
	}
	var assigneeIDs []int
	if raw := row[`edit_assignee_ids`]; raw != `` && raw != `[]` {
		_ = json.Unmarshal([]byte(raw), &assigneeIDs)
	}
	if len(assigneeIDs) > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `assignee`, Value: assigneeIDs})
	}
	if hours := cast.ToFloat64(row[`work_hours`]); hours > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `estimated_work_hours`, Value: hours})
	}
	if hours := cast.ToFloat64(row[`completed_work_hours`]); hours > 0 {
		fields = append(fields, define.IssueInviteFieldValue{FieldKey: `completed_work_hours`, Value: hours})
	}
	return fields
}

// formatIssueInviteFieldValue renders a stored custom field value for the public detail:
// select options resolve to their display names; a value whose option is no longer
// configured renders empty instead of leaking the raw option key. Other types keep the
// stored text so the frontend can format them (e.g. date values converted to the browser
// timezone).
func formatIssueInviteFieldValue(field CustomFieldModel, value string) any {
	switch field.FieldType {
	case `select_single`:
		for _, option := range field.Options {
			if option.Key == value {
				return option.Name
			}
		}
		return ``
	case `select_multi`:
		var keys []string
		if err := tool.JsonDecodeUseNumber(value, &keys); err != nil {
			return ``
		}
		names := make([]string, 0, len(keys))
		for _, key := range keys {
			for _, option := range field.Options {
				if option.Key == key {
					names = append(names, option.Name)
					break
				}
			}
		}
		return names
	default:
		return value
	}
}

// InvalidateIssueInviteProjectCache invalidates public configuration/template cache after commit.
func InvalidateIssueInviteProjectCache(adminID, workspaceID int) error {
	config, err := GetIssueInviteConfig(adminID, workspaceID)
	if err != nil {
		return err
	}
	keys := make([]string, 0, 4)
	if config.InviteKey != `` {
		keys = append(keys, BuildIssueInviteConfigCacheKey(adminID, workspaceID, config.InviteKey))
	}
	templates, err := ListIssueInviteTemplates(adminID, workspaceID, false)
	if err != nil {
		return err
	}
	for _, template := range templates {
		if template.InviteKey != `` {
			keys = append(keys, BuildIssueInviteTemplateCacheKey(adminID, workspaceID, template.InviteKey))
		}
	}
	if len(keys) == 0 {
		return nil
	}
	if err = lib_redis.Del(define.Redis, keys...); err != nil {
		logs.Error("invalidate issue invite project cache failed, err:%s", err.Error())
		return err
	}
	return nil
}

// InvalidateIssueInviteTemplateCache deletes one exact public template cache key.
// Removed templates are no longer listed, so InvalidateIssueInviteProjectCache cannot
// discover their key; callers must pass the key captured before deletion.
func InvalidateIssueInviteTemplateCache(adminID, workspaceID int, templateKey string) error {
	if templateKey == `` {
		return nil
	}
	key := BuildIssueInviteTemplateCacheKey(adminID, workspaceID, templateKey)
	if err := lib_redis.Del(define.Redis, key); err != nil {
		logs.Error("invalidate issue invite template cache failed, err:%s", err.Error())
		return err
	}
	return nil
}

// RotateIssueInviteHistoryCache rotates one submitter history namespace after commit.
func RotateIssueInviteHistoryCache(adminID, workspaceID int, openID string) error {
	key := issueInviteHistoryVersionKey(adminID, workspaceID, openID)
	ctx := context.Background()
	if _, err := define.Redis.Incr(ctx, key).Result(); err != nil {
		logs.Error("rotate issue invite history cache failed, err:%s", err.Error())
		return err
	}
	// The marker must outlive the page caches it namespaces, otherwise a stale page
	// could outlive the marker and be read under the reset default version.
	if err := define.Redis.Expire(ctx, key, 2*issueInviteCacheTTL).Err(); err != nil {
		logs.Error("set issue invite history version ttl failed, err:%s", err.Error())
		return err
	}
	return nil
}

// InvalidateIssueInviteDetailCache invalidates one submitter's exact public detail cache.
func InvalidateIssueInviteDetailCache(adminID, workspaceID int, openID string, requirementID int) error {
	version, err := getIssueInviteRenderVersion(adminID, workspaceID)
	if err != nil {
		return err
	}
	key := BuildIssueInviteDetailCacheKey(adminID, workspaceID, openID, version, requirementID)
	if err := lib_redis.Del(define.Redis, key); err != nil {
		logs.Error("invalidate issue invite detail cache failed, err:%s", err.Error())
		return err
	}
	return nil
}

// BuildPendingAttachmentCacheKey builds the ownership key granted to one public submitter
// for a still-unbound attachment; the one-day TTL bounds the delete window.
func BuildPendingAttachmentCacheKey(adminID, workspaceID int, openID string, attachmentID int) string {
	return joinIssueInviteCacheKey(
		define.CacheKeyIssueInviteAttachment,
		strconv.Itoa(adminID),
		strconv.Itoa(workspaceID),
		sha256OpenID(openID),
		strconv.Itoa(attachmentID),
	)
}

// CachePendingIssueAttachment grants the uploader delete ownership of one unbound attachment.
func CachePendingIssueAttachment(adminID, workspaceID int, openID string, attachmentID int) error {
	key := BuildPendingAttachmentCacheKey(adminID, workspaceID, openID, attachmentID)
	if err := lib_redis.Set(define.Redis, key, attachmentID, issueInviteCacheTTL); err != nil {
		logs.Error("cache pending issue attachment failed, err:%s", err.Error())
		return err
	}
	return nil
}

// DeletePendingIssueAttachment removes an unbound attachment only when the caller can prove
// ownership through the one-day upload cache; the cache entry and the enterprise file ledger
// row are removed together with the stored file.
func DeletePendingIssueAttachment(adminID, workspaceID int, openID string, attachmentID int) error {
	key := BuildPendingAttachmentCacheKey(adminID, workspaceID, openID, attachmentID)
	if _, err := lib_redis.Get(define.Redis, key); err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrAttachmentNotFound
		}
		logs.Error("get pending issue attachment cache failed, err:%s", err.Error())
		return err
	}
	if err := DeletePendingAttachment(adminID, workspaceID, attachmentID); err != nil {
		return err
	}
	if err := lib_redis.Del(define.Redis, key); err != nil {
		logs.Error("clear pending issue attachment cache failed, err:%s", err.Error())
	}
	return nil
}

// InvalidateIssueInvitePublicCaches invalidates the public detail caches and rotates the
// submitter history versions of the given requirements; no-op when none came from an invitation.
// Callers must invoke it after the data change is committed, while the association rows still exist.
func InvalidateIssueInvitePublicCaches(adminID int, requirementIDs []int) error {
	if len(requirementIDs) == 0 {
		return nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT workspace_id, openid FROM `+define.TableWorkspaceIssueRequirements+`
		WHERE admin_id = $1 AND requirement_id = ANY($2)`, nil, adminID, pq.Array(requirementIDs))
	if err != nil {
		logs.Error("load issue invite cache owners failed, err:%s", err.Error())
		return err
	}
	for _, row := range rows {
		workspaceID := cast.ToInt(row[`workspace_id`])
		openID := row[`openid`]
		for _, requirementID := range requirementIDs {
			if err = InvalidateIssueInviteDetailCache(adminID, workspaceID, openID, requirementID); err != nil {
				return err
			}
		}
		if err = RotateIssueInviteHistoryCache(adminID, workspaceID, openID); err != nil {
			return err
		}
	}
	return nil
}
