// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"strings"
	"time"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ApiTokenNameMaxLen 与 api_tokens.name 的 VARCHAR(100) 一致。
const ApiTokenNameMaxLen = 100

// ApiTokenMaxPerUser 是单个用户可创建的 API token 上限，列表卡的「新增」按钮按同一数字禁用。
const ApiTokenMaxPerUser = 10

// ApiTokenModel 是列表卡片展示用的 API token。
type ApiTokenModel struct {
	Id          int
	Name        string
	TokenPrefix string
	// Token 是明文，仅在创建响应与列表卡片中使用。
	Token           string
	Permissions     []string
	Enabled         int
	CumulativeCalls int
	LastUsedAt      int
	CreatedAt       int
	TodayCalls      int
	YesterdayCalls  int
}

// ApiTokenPermissionOption 是一个可勾选的权限点，NameKey 供 controller 查 i18n 后下发。
type ApiTokenPermissionOption struct {
	Key     string
	NameKey string
}

var (
	// ErrApiTokenNotFound 表示 token 不存在或不属于当前用户。
	ErrApiTokenNotFound = errors.New(`api token not found`)
	// ErrApiTokenNameTooLong 表示备注超出长度上限。
	ErrApiTokenNameTooLong = errors.New(`api token name too long`)
	// ErrApiTokenLimitReached 表示已达到单个用户的 token 数量上限。
	ErrApiTokenLimitReached = errors.New(`api token limit reached`)
)

// ListApiTokens 返回当前用户创建的 API token，按创建时间倒序，并附带今日/昨日触发次数。
func ListApiTokens(adminId, userId int) ([]ApiTokenModel, error) {
	m := msql.Model(define.TableApiTokens, define.Postgres)
	rows, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`user_id`, cast.ToString(userId)},
	}).Order(`id desc`).Select()
	if err != nil {
		logs.Error("list api tokens failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	items := make([]ApiTokenModel, 0, len(rows))
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		item := apiTokenFromRow(row)
		items = append(items, item)
		ids = append(ids, item.Id)
	}
	applyApiTokenDailyCalls(items, ids)
	return items, nil
}

// CreateApiToken 创建 API token，返回明文与列表卡片展示所需字段。
func CreateApiToken(adminId, userId int, name string, perms []string) (string, ApiTokenModel, error) {
	model := ApiTokenModel{}
	name, err := checkApiTokenName(name)
	if err != nil {
		return ``, model, err
	}
	count, err := msql.Model(define.TableApiTokens, define.Postgres).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`user_id`, cast.ToString(userId)},
	}).Count()
	if err != nil {
		logs.Error("count api tokens failed: err=%s", err.Error())
		return ``, model, err
	}
	// 先查后插在并发创建下可能多出一条；数量上限是体验约束，不为它加锁。
	if count >= ApiTokenMaxPerUser {
		return ``, model, ErrApiTokenLimitReached
	}
	plain, prefix, hash, err := common.GenerateApiToken()
	if err != nil {
		return ``, model, err
	}
	now := tool.Time2Int()
	m := msql.Model(define.TableApiTokens, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`:         adminId,
		`user_id`:          userId,
		`name`:             name,
		`token_prefix`:     prefix,
		`token_plain`:      plain,
		`token_hash`:       hash,
		`permissions`:      pq.Array(define.NormalizeApiTokenPermissions(perms)),
		`enabled`:          1,
		`cumulative_calls`: 0,
		`last_used_at`:     0,
		`created_at`:       now,
		`updated_at`:       now,
	}, `id`)
	if err != nil {
		logs.Error("create api token failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return ``, model, err
	}
	// 新增默认开启。
	return plain, ApiTokenModel{
		Id:          cast.ToInt(id),
		Name:        name,
		TokenPrefix: prefix,
		Token:       plain,
		Permissions: define.NormalizeApiTokenPermissions(perms),
		Enabled:     1,
		CreatedAt:   now,
	}, nil
}

// UpdateApiToken 修改备注与权限点，不影响启用状态与历史统计。
func UpdateApiToken(adminId, userId, id int, name string, perms []string) (ApiTokenModel, error) {
	if !apiTokenOwned(adminId, userId, id) {
		return ApiTokenModel{}, ErrApiTokenNotFound
	}
	name, err := checkApiTokenName(name)
	if err != nil {
		return ApiTokenModel{}, err
	}
	m := msql.Model(define.TableApiTokens, define.Postgres)
	_, err = m.Where(`id`, cast.ToString(id)).Update(msql.Datas{
		`name`:        name,
		`permissions`: pq.Array(define.NormalizeApiTokenPermissions(perms)),
		`updated_at`:  tool.Time2Int(),
	})
	if err != nil {
		logs.Error("update api token failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return ApiTokenModel{}, err
	}
	return ApiTokenModel{
		Id:          id,
		Name:        name,
		Permissions: define.NormalizeApiTokenPermissions(perms),
	}, nil
}

// SetApiTokenEnabled 启停 API token。停用只阻断调用，不清空历史统计，重新启用继续累加。
func SetApiTokenEnabled(adminId, userId, id, enabled int) error {
	if !apiTokenOwned(adminId, userId, id) {
		return ErrApiTokenNotFound
	}
	m := msql.Model(define.TableApiTokens, define.Postgres)
	_, err := m.Where(`id`, cast.ToString(id)).Update(msql.Datas{
		`enabled`:    enabled,
		`updated_at`: tool.Time2Int(),
	})
	if err != nil {
		logs.Error("toggle api token failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
	return err
}

// DeleteApiToken 删除 API token 及其每日统计明细。归属条件直接写进 DELETE，
// 两张表在同一事务内删除，避免部分失败留下无主明细行。
func DeleteApiToken(adminId, userId, id int) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin delete api token failed: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableApiTokens+`
		WHERE id = $1 AND admin_id = $2 AND user_id = $3`, tx, id, adminId, userId)
	if err != nil {
		logs.Error("delete api token failed: token_id=%d, err=%s", id, err.Error())
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		logs.Error("count deleted api token failed: token_id=%d, err=%s", id, err.Error())
		return err
	}
	if affected == 0 {
		return ErrApiTokenNotFound
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableApiTokenDailyStats+`
		WHERE token_id = $1`, tx, id); err != nil {
		logs.Error("delete api token stats failed: token_id=%d, err=%s", id, err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// token 已经不存在，它的临时安装链接再留着也没有意义，顺手清掉。
	DropApiTokenSkillLink(adminId, userId, id)
	return nil
}

// ApiTokenPermissionOptions 返回全部可勾选的权限点，顺序即前端展示顺序。
func ApiTokenPermissionOptions() []ApiTokenPermissionOption {
	options := make([]ApiTokenPermissionOption, 0, len(define.AllApiTokenPermissions))
	for _, perm := range define.AllApiTokenPermissions {
		options = append(options, ApiTokenPermissionOption{Key: perm, NameKey: define.ApiTokenPermissionNameKey(perm)})
	}
	return options
}

// apiTokenOwned 校验 token 属于该用户在当前租户下，防止越权改删他人 token。
func apiTokenOwned(adminId, userId, id int) bool {
	if id <= 0 {
		return false
	}
	m := msql.Model(define.TableApiTokens, define.Postgres)
	count, err := m.Where2([][]string{
		{`id`, cast.ToString(id)},
		{`admin_id`, cast.ToString(adminId)},
		{`user_id`, cast.ToString(userId)},
	}).Count()
	if err != nil {
		logs.Error("check api token owner failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return false
	}
	return count > 0
}

// applyApiTokenDailyCalls 一次性查出今日与昨日次数并回填，避免每条 token 各查一次。
func applyApiTokenDailyCalls(items []ApiTokenModel, ids []int) {
	if len(ids) == 0 {
		return
	}
	today := common.ApiQuotaDay(time.Now())
	yesterday := common.ApiQuotaDay(time.Now().AddDate(0, 0, -1))
	rows, err := msql.RawValues(define.Postgres, `SELECT token_id, day, calls FROM `+define.TableApiTokenDailyStats+`
		WHERE token_id = ANY($1) AND day IN ($2, $3)`, nil, pq.Array(ids), today, yesterday)
	if err != nil {
		logs.Error("get api token daily calls failed: err=%s", err.Error())
		return
	}
	byToken := make(map[int]map[int]int, len(ids))
	for _, row := range rows {
		tokenId, day := cast.ToInt(row[`token_id`]), cast.ToInt(row[`day`])
		if byToken[tokenId] == nil {
			byToken[tokenId] = make(map[int]int, 2)
		}
		byToken[tokenId][day] = cast.ToInt(row[`calls`])
	}
	for i := range items {
		items[i].TodayCalls = byToken[items[i].Id][today]
		items[i].YesterdayCalls = byToken[items[i].Id][yesterday]
	}
}

// checkApiTokenName 去掉首尾空白并校验长度；备注允许为空，列表展示为占位符。
func checkApiTokenName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) > ApiTokenNameMaxLen {
		return ``, ErrApiTokenNameTooLong
	}
	return name, nil
}

// apiTokenFromRow 把一行记录转成模型，permissions 缺列时按空权限处理。
func apiTokenFromRow(row msql.Params) ApiTokenModel {
	return ApiTokenModel{
		Id:              cast.ToInt(row[`id`]),
		Name:            row[`name`],
		TokenPrefix:     row[`token_prefix`],
		Token:           row[`token_plain`],
		Permissions:     common.ParseTextArray(row[`permissions`]),
		Enabled:         cast.ToInt(row[`enabled`]),
		CumulativeCalls: cast.ToInt(row[`cumulative_calls`]),
		LastUsedAt:      cast.ToInt(row[`last_used_at`]),
		CreatedAt:       cast.ToInt(row[`created_at`]),
	}
}
