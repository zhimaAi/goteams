// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ErrSsoConfigIncomplete 表示开启状态下必需凭证不齐。
// 不允许「开着但换不了码」：这类错误必须在保存时就拒绝，而不是留到成员点登录。
var ErrSsoConfigIncomplete = errors.New("sso config is incomplete")

// ErrSsoProviderInvalid 表示 provider 不在平台清单内。
var ErrSsoProviderInvalid = errors.New("sso provider is not supported")

// SsoHourlySyncSeconds 是 hourly 周期对应的固定间隔。
const SsoHourlySyncSeconds = 3600

// SsoFiveMinSyncSeconds 是测试环境 every_5min 周期对应的固定间隔。
// 必须大于 schedulerMinIntervalSeconds（60），否则会被 nextRunAt 悄悄抬到 60 秒。
const SsoFiveMinSyncSeconds = 300

// SsoSyncRunAtHour 是自动同步的执行时刻：东八区 03:00，避开白天与整点备份窗口。
const SsoSyncRunAtHour = 3

// SsoConfigModel 是租户的 SSO 登录配置。AppSecret 明文落库、永不出参（与 api_tokens 口径一致）。
type SsoConfigModel struct {
	AdminId         int
	Enabled         bool
	Provider        string
	AppKey          string
	AppSecret       string
	AgentId         string
	IssuerUrl       string
	AuthorizeUrl    string
	TokenUrl        string
	UserInfoUrl     string
	Scopes          string
	ExternalIdField string
	CallbackKey     string // 随机生成一次后恒定；保存接口不接受也不修改它
}

// SsoSyncConfigModel 是租户的成员同步配置，与登录配置独立存储。
type SsoSyncConfigModel struct {
	AdminId          int
	Enabled          bool
	AutoSyncPeriod   string
	WeeklyDay        int
	ScimBaseUrl      string
	ScimToken        string
	UserFieldMapping string
	DeptFieldMapping string
	LastSyncedAt     int
	LastSyncedCount  int
	LastSyncStatus   string
	LastFailedCount  int
	LastError        string
}

// SaveSsoConfigModel 是保存登录配置的业务输入。密钥用指针表达三态：
// nil=保留库里已存的值，空串=清空，非空=覆盖。值类型会让「没改」和「清空」塌成同一个请求。
type SaveSsoConfigModel struct {
	AdminId         int
	Enabled         bool
	Provider        string
	AppKey          string
	AppSecret       *string
	AgentId         string
	IssuerUrl       string
	AuthorizeUrl    string
	TokenUrl        string
	UserInfoUrl     string
	Scopes          string
	ExternalIdField string
}

// SaveSsoSyncConfigModel 是保存同步配置的业务输入，ScimToken 同样是三态指针。
type SaveSsoSyncConfigModel struct {
	AdminId          int
	Enabled          bool
	AutoSyncPeriod   string
	WeeklyDay        int
	ScimBaseUrl      string
	ScimToken        *string
	UserFieldMapping string
	DeptFieldMapping string
}

// ssoConfigColumns 是读取登录配置时显式选择的列，不使用 SELECT *。
const ssoConfigColumns = `admin_id, enabled, provider, app_key, app_secret, agent_id, issuer_url,
	authorize_url, token_url, userinfo_url, scopes, external_id_field, callback_key`

// ssoSyncConfigColumns 是读取同步配置时显式选择的列。
const ssoSyncConfigColumns = `admin_id, enabled, auto_sync_period, weekly_day, scim_base_url, scim_token,
	user_field_mapping, dept_field_mapping, last_synced_at, last_synced_count, last_sync_status,
	last_failed_count, last_error`

// EnsureSsoConfigs 物化租户的两行配置。回调地址必须在管理员第一次保存之前就能复制进 IdP 后台，
// 所以 GET 也要保证行存在；callback_key 只在这里首次写入。
func EnsureSsoConfigs(adminId int) error {
	now := time.Now().Unix()
	callbackKey, err := NewSsoCallbackKey()
	if err != nil {
		return err
	}
	// ON CONFLICT DO NOTHING：并发首读时谁先插进去都行，撞唯一索引的一方什么也不改。
	if _, err = msql.RawExec(define.Postgres, `INSERT INTO sso_configs
		(admin_id,enabled,callback_key,created_at,updated_at) VALUES($1,0,$2,$3,$3)
		ON CONFLICT(admin_id) DO NOTHING`, nil, adminId, callbackKey, now); err != nil {
		logs.Error("ensure sso config failed: err=%s", err.Error())
		return err
	}
	_, err = msql.RawExec(define.Postgres, `INSERT INTO sso_sync_configs
		(admin_id,enabled,auto_sync_period,created_at,updated_at) VALUES($1,1,$2,$3,$3)
		ON CONFLICT(admin_id) DO NOTHING`, nil, adminId, define.SsoSyncPeriodNone, now)
	if err != nil {
		logs.Error("ensure sso sync config failed: err=%s", err.Error())
	}
	return err
}

// GetSsoConfig 读取登录配置，缺失时先物化再读。
func GetSsoConfig(adminId int) (SsoConfigModel, error) {
	if err := EnsureSsoConfigs(adminId); err != nil {
		return SsoConfigModel{}, err
	}
	config, exists, err := ssoLoadConfig(adminId)
	if err != nil {
		return SsoConfigModel{}, err
	}
	if !exists {
		return SsoConfigModel{}, errors.New(`sso config row missing`)
	}
	return config, nil
}

// ssoLoadConfig 只读配置行、不物化。
//
// 公开入口与回调都是匿名请求，让它们去建配置行等于给任意 admin_id 写入两行数据；
// 因此凡是走公开链路读配置的都用这个函数，企业管理界面的读取才用 GetSsoConfig。
func ssoLoadConfig(adminId int) (SsoConfigModel, bool, error) {
	m := msql.Model(define.TableSsoConfigs, define.Postgres)
	row, err := m.Field(ssoConfigColumns).Where(`admin_id`, cast.ToString(adminId)).Find()
	if err != nil {
		logs.Error("load sso config failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return SsoConfigModel{}, false, err
	}
	if len(row) == 0 {
		return SsoConfigModel{}, false, nil
	}
	config, err := ssoConfigFromRow(row)
	return config, err == nil, err
}

// GetSsoConfigByCallbackKey 按回调路径里的租户标识反查配置行。
// callback_key 是随机值、唯一索引命中一行；查不到即拒绝，同样不物化。
func GetSsoConfigByCallbackKey(callbackKey string) (SsoConfigModel, bool, error) {
	if callbackKey == `` {
		return SsoConfigModel{}, false, nil
	}
	m := msql.Model(define.TableSsoConfigs, define.Postgres)
	row, err := m.Field(ssoConfigColumns).Where(`callback_key`, callbackKey).Find()
	if err != nil {
		logs.Error("load sso config by callback key failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return SsoConfigModel{}, false, err
	}
	if len(row) == 0 {
		return SsoConfigModel{}, false, nil
	}
	config, err := ssoConfigFromRow(row)
	return config, err == nil, err
}

// GetSsoSyncConfig 读取同步配置，缺失时先物化再读。
func GetSsoSyncConfig(adminId int) (SsoSyncConfigModel, error) {
	if err := EnsureSsoConfigs(adminId); err != nil {
		return SsoSyncConfigModel{}, err
	}
	m := msql.Model(define.TableSsoSyncConfigs, define.Postgres)
	row, err := m.Field(ssoSyncConfigColumns).Where(`admin_id`, cast.ToString(adminId)).Find()
	if err != nil {
		logs.Error("get sso sync config failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return SsoSyncConfigModel{}, err
	}
	if len(row) == 0 {
		return SsoSyncConfigModel{}, errors.New(`sso sync config row missing`)
	}
	return ssoSyncConfigFromRow(row)
}

func ssoConfigFromRow(row msql.Params) (SsoConfigModel, error) {
	adminId, err := parseRowInt(row, `admin_id`)
	if err != nil {
		return SsoConfigModel{}, err
	}
	enabled, err := parseRowInt(row, `enabled`)
	if err != nil {
		return SsoConfigModel{}, err
	}
	return SsoConfigModel{
		AdminId: adminId, Enabled: enabled == 1, Provider: row[`provider`], AppKey: row[`app_key`],
		AppSecret: row[`app_secret`], AgentId: row[`agent_id`], IssuerUrl: row[`issuer_url`],
		AuthorizeUrl: row[`authorize_url`], TokenUrl: row[`token_url`], UserInfoUrl: row[`userinfo_url`],
		Scopes: row[`scopes`], ExternalIdField: row[`external_id_field`], CallbackKey: row[`callback_key`],
	}, nil
}

func ssoSyncConfigFromRow(row msql.Params) (SsoSyncConfigModel, error) {
	adminId, err := parseRowInt(row, `admin_id`)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	enabled, err := parseRowInt(row, `enabled`)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	weeklyDay, err := parseRowInt(row, `weekly_day`)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	lastSyncedAt, err := parseRowInt(row, `last_synced_at`)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	syncedCount, err := parseRowInt(row, `last_synced_count`)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	failedCount, err := parseRowInt(row, `last_failed_count`)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	return SsoSyncConfigModel{
		AdminId: adminId, Enabled: enabled == 1, AutoSyncPeriod: row[`auto_sync_period`], WeeklyDay: weeklyDay,
		ScimBaseUrl: row[`scim_base_url`], ScimToken: row[`scim_token`],
		UserFieldMapping: row[`user_field_mapping`], DeptFieldMapping: row[`dept_field_mapping`],
		LastSyncedAt: lastSyncedAt, LastSyncedCount: syncedCount, LastSyncStatus: row[`last_sync_status`],
		LastFailedCount: failedCount, LastError: row[`last_error`],
	}, nil
}

// MergeSsoConfig 把保存请求合并成「本次有效的完整配置」。
//
// 测试连接必须复用这份合并结果：两条路径任何一处解释不同，就会出现「旧 secret 测试通过 → 保存按
// 另一套规则清空 secret → 启用后全企业登不进来，而界面上留着测试成功的记录」。
func MergeSsoConfig(saved SsoConfigModel, model SaveSsoConfigModel) SsoConfigModel {
	merged := saved
	merged.AdminId = model.AdminId
	merged.Enabled = model.Enabled
	merged.Provider = model.Provider
	merged.AppKey = strings.TrimSpace(model.AppKey)
	merged.AgentId = strings.TrimSpace(model.AgentId)
	merged.IssuerUrl = strings.TrimSpace(model.IssuerUrl)
	merged.AuthorizeUrl = strings.TrimSpace(model.AuthorizeUrl)
	merged.TokenUrl = strings.TrimSpace(model.TokenUrl)
	merged.UserInfoUrl = strings.TrimSpace(model.UserInfoUrl)
	merged.Scopes = strings.TrimSpace(model.Scopes)
	merged.ExternalIdField = strings.TrimSpace(model.ExternalIdField)
	if model.AppSecret != nil {
		merged.AppSecret = strings.TrimSpace(*model.AppSecret)
	}
	return merged
}

// MergeSsoSyncConfig 把同步配置保存请求合并成有效配置，语义与 MergeSsoConfig 相同。
func MergeSsoSyncConfig(saved SsoSyncConfigModel, model SaveSsoSyncConfigModel) SsoSyncConfigModel {
	merged := saved
	merged.AdminId = model.AdminId
	merged.Enabled = model.Enabled
	merged.AutoSyncPeriod = model.AutoSyncPeriod
	merged.WeeklyDay = model.WeeklyDay
	merged.ScimBaseUrl = strings.TrimSpace(model.ScimBaseUrl)
	merged.UserFieldMapping = strings.TrimSpace(model.UserFieldMapping)
	merged.DeptFieldMapping = strings.TrimSpace(model.DeptFieldMapping)
	if model.ScimToken != nil {
		merged.ScimToken = strings.TrimSpace(*model.ScimToken)
	}
	return merged
}

// ValidateSsoConfig 校验平台标识，以及 enabled=1 时各平台的必需凭证是否齐全。
func ValidateSsoConfig(config SsoConfigModel) error {
	if !SsoProviderSupports(config.Provider) {
		return fmt.Errorf("%w: %s", ErrSsoProviderInvalid, config.Provider)
	}
	if !config.Enabled {
		// 关闭状态允许半截配置：管理员常常先填一半去 IdP 后台复制回调地址，再回来补另一半。
		return nil
	}
	if field := ssoRequiredFields(config); field != `` {
		return fmt.Errorf("%w: %s", ErrSsoConfigIncomplete, field)
	}
	return nil
}

// ssoRequiredFields 返回 enabled=1 时该平台缺失的必需字段名（无则空串）。
func ssoRequiredFields(config SsoConfigModel) string {
	missing := func(value string, name string) string {
		if strings.TrimSpace(value) == `` {
			return name
		}
		return ``
	}
	if name := missing(config.AppKey, `app_key`); name != `` {
		return name
	}
	if name := missing(config.AppSecret, `app_secret`); name != `` {
		return name
	}
	switch config.Provider {
	case define.SsoProviderWecom:
		return missing(config.AgentId, `agent_id`)
	case define.SsoProviderOidc:
		if name := missing(config.IssuerUrl, `issuer_url`); name != `` {
			return name
		}
		// openid 是 OIDC 拿 id_token 的前提，缺了整条登录链路失效，保存即拒。
		if !ssoScopesContain(config.Scopes, `openid`) {
			return `scopes`
		}
	case define.SsoProviderOauth2:
		for _, item := range []struct{ value, name string }{
			{config.AuthorizeUrl, `authorize_url`}, {config.TokenUrl, `token_url`},
			{config.UserInfoUrl, `userinfo_url`}, {config.ExternalIdField, `external_id_field`},
			{config.Scopes, `scopes`},
		} {
			if name := missing(item.value, item.name); name != `` {
				return name
			}
		}
		// 部分 OAuth2.0 服务要求 scope 含 openid 才放行授权，缺了保存即拒。
		if !ssoScopesContain(config.Scopes, `openid`) {
			return `scopes`
		}
	}
	return ``
}

// ssoScopesContain 按空白分词判断 scope 清单是否包含某个值（scope 标识区分大小写，OIDC 规范为小写）。
func ssoScopesContain(scopes, want string) bool {
	return slices.Contains(strings.Fields(scopes), want)
}

// SsoTest 用「合并后但未落库」的有效配置做一次连接预检，返回实际验到的层级。
// 保存前预检与保存后预检都走这里，因此复用同一套校验与出站限制。
// 没有授权码，验不了换 token 那半段，界面不得把它显示成「登录验证通过」。
func SsoTest(config SsoConfigModel) (string, error) {
	if !SsoProviderSupports(config.Provider) {
		return ``, fmt.Errorf("%w: %s", ErrSsoProviderInvalid, config.Provider)
	}
	if field := ssoRequiredFields(config); field != `` {
		return ``, fmt.Errorf("%w: %s", ErrSsoConfigIncomplete, field)
	}
	provider, ok := SsoProviderByKey(config.Provider)
	if !ok {
		return ``, fmt.Errorf("%w: %s", ErrSsoProviderInvalid, config.Provider)
	}
	return provider.TestConnect(config)
}

// SaveSsoConfig 保存登录配置。provider 或 app_key 变化时，在同一事务内清空该租户的全部身份绑定：
// 只删绑定行，成员与部门账号一律保留（它们可能已经是工作项负责人、评论作者）。
func SaveSsoConfig(model SaveSsoConfigModel) (SsoConfigModel, error) {
	saved, err := GetSsoConfig(model.AdminId)
	if err != nil {
		return SsoConfigModel{}, err
	}
	merged := MergeSsoConfig(saved, model)
	if err = ValidateSsoConfig(merged); err != nil {
		return SsoConfigModel{}, err
	}
	// 单平台生效：换平台或换应用后，上一平台的绑定与 SCIM 配置都不能留。
	platformSwitched := merged.Provider != saved.Provider || merged.AppKey != saved.AppKey
	now := time.Now().Unix()
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return SsoConfigModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = msql.RawExec(define.Postgres, `UPDATE sso_configs SET enabled=$1, provider=$2, app_key=$3,
		app_secret=$4, agent_id=$5, issuer_url=$6, authorize_url=$7, token_url=$8, userinfo_url=$9,
		scopes=$10, external_id_field=$11, updated_at=$12 WHERE admin_id=$13`, tx,
		ssoFlag(merged.Enabled), merged.Provider, merged.AppKey, merged.AppSecret, merged.AgentId,
		merged.IssuerUrl, merged.AuthorizeUrl, merged.TokenUrl, merged.UserInfoUrl, merged.Scopes,
		merged.ExternalIdField, now, merged.AdminId); err != nil {
		logs.Error("save sso config failed: err=%s", err.Error())
		return SsoConfigModel{}, err
	}
	if platformSwitched {
		// 不带 provider 条件：历史行可能属于上一个平台，留着就会让旧身份被新平台的同名标识认领。
		for _, table := range []string{define.TableUserSsoBindings, define.TableDepartmentSsoBindings} {
			if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+table+` WHERE admin_id=$1`, tx, merged.AdminId); err != nil {
				logs.Error("clear sso bindings failed: table=%s, err=%s", table, err.Error())
				return SsoConfigModel{}, err
			}
		}
		if err = resetSsoSyncConfigTx(tx, merged.AdminId, now); err != nil {
			return SsoConfigModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return SsoConfigModel{}, err
	}
	if platformSwitched {
		// 厂商通讯录无需额外凭证，可沿用原周期；SCIM 凭证已清空，待重新填写后再调度。
		syncConfig, loadErr := GetSsoSyncConfig(merged.AdminId)
		if loadErr != nil {
			return SsoConfigModel{}, loadErr
		}
		if err = syncSsoScheduledTaskForLogin(syncConfig, merged); err != nil {
			return SsoConfigModel{}, err
		}
	}
	return merged, nil
}

// resetSsoSyncConfigTx 清掉该租户的旧 SCIM 配置；新身份源的同步默认开启。
// 它指向的是上一个平台的通讯录，留着会让同步继续从旧 IdP 拉人、再按新平台的身份命名空间建号。
// 自动周期是管理员自己的偏好、与平台无关，保留。
func resetSsoSyncConfigTx(tx *sql.Tx, adminId int, now int64) error {
	_, err := msql.RawExec(define.Postgres, `UPDATE sso_sync_configs SET enabled=1,
		scim_base_url='', scim_token='', user_field_mapping=NULL, dept_field_mapping=NULL,
		last_synced_at=0, last_synced_count=0, last_sync_status='', last_failed_count=0, last_error='',
		updated_at=$1 WHERE admin_id=$2`, tx, now, adminId)
	if err != nil {
		logs.Error("reset sso sync config failed: admin_id=%d, err=%s", adminId, err.Error())
	}
	return err
}

// SaveSsoSyncConfig 保存同步配置，并把自动周期同步到 scheduled_tasks。
// 周期为 none、登录未开启或 SCIM 凭证未配齐时，调度行不参与自动领取，手动触发仍可领取。
func SaveSsoSyncConfig(model SaveSsoSyncConfigModel) (SsoSyncConfigModel, error) {
	saved, err := GetSsoSyncConfig(model.AdminId)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	merged := MergeSsoSyncConfig(saved, model)
	if err = ValidateSsoSyncConfig(merged); err != nil {
		return SsoSyncConfigModel{}, err
	}
	now := time.Now().Unix()
	if _, err = msql.Model(define.TableSsoSyncConfigs, define.Postgres).Where(`admin_id`, cast.ToString(merged.AdminId)).
		Update(msql.Datas{
			`enabled`: ssoFlag(merged.Enabled), `auto_sync_period`: merged.AutoSyncPeriod,
			`weekly_day`: merged.WeeklyDay, `scim_base_url`: merged.ScimBaseUrl, `scim_token`: merged.ScimToken,
			`user_field_mapping`: merged.UserFieldMapping, `dept_field_mapping`: merged.DeptFieldMapping,
			`updated_at`: now,
		}); err != nil {
		logs.Error("save sso sync config failed: err=%s", err.Error())
		return SsoSyncConfigModel{}, err
	}
	loginConfig, err := GetSsoConfig(merged.AdminId)
	if err != nil {
		return SsoSyncConfigModel{}, err
	}
	if err = syncSsoScheduledTaskForLogin(merged, loginConfig); err != nil {
		return SsoSyncConfigModel{}, err
	}
	return merged, nil
}

// ValidateSsoSyncConfig 校验周期枚举；SCIM 凭证可分项自动保存，未配齐时不安排定时任务。
// 周期清单按环境取：生产拒收测试环境专属的短周期，不依赖界面隐藏。
func ValidateSsoSyncConfig(config SsoSyncConfigModel) error {
	if err := validate.IsInList(`auto_sync_period`, config.AutoSyncPeriod, define.SsoSyncPeriodsForEnv()); err != nil {
		return err
	}
	if config.AutoSyncPeriod == define.SsoSyncPeriodWeekly && (config.WeeklyDay < 1 || config.WeeklyDay > 7) {
		return fmt.Errorf("%w: weekly_day", ErrSsoConfigIncomplete)
	}
	return nil
}

// 登录平台或 SCIM 凭证尚不可用时保留周期偏好，但暂不领取自动任务。
func syncSsoScheduledTaskForLogin(syncConfig SsoSyncConfigModel, loginConfig SsoConfigModel) error {
	vendor := loginConfig.Provider == define.SsoProviderDingtalk || loginConfig.Provider == define.SsoProviderFeishu
	scimReady := (loginConfig.Provider == define.SsoProviderOidc || loginConfig.Provider == define.SsoProviderOauth2) &&
		strings.TrimSpace(syncConfig.ScimBaseUrl) != `` && strings.TrimSpace(syncConfig.ScimToken) != ``
	syncConfig.Enabled = syncConfig.Enabled && loginConfig.Enabled && (vendor || scimReady)
	return syncSsoScheduledTask(syncConfig)
}

// syncSsoScheduledTask 把同步周期落到定时任务表。计时层每次领取都重读这行，改周期不必重启调度器。
func syncSsoScheduledTask(config SsoSyncConfigModel) error {
	schedule := ScheduledTask{
		TaskKey:  define.SsoSyncTaskKey,
		ScopeKey: SsoSyncScopeKey(config.AdminId),
		AdminId:  config.AdminId,
	}
	switch config.AutoSyncPeriod {
	case define.SsoSyncPeriodHourly:
		schedule.Schedule, schedule.IntervalSec = ScheduleTypeInterval, SsoHourlySyncSeconds
	case define.SsoSyncPeriodFiveMin:
		schedule.Schedule, schedule.IntervalSec = ScheduleTypeInterval, SsoFiveMinSyncSeconds
	case define.SsoSyncPeriodDaily:
		schedule.Schedule, schedule.Hour = ScheduleTypeDaily, SsoSyncRunAtHour
	case define.SsoSyncPeriodWeekly:
		schedule.Schedule, schedule.Hour, schedule.Weekday = ScheduleTypeWeekly, SsoSyncRunAtHour, config.WeeklyDay
	default:
		// none：置为不参与自动调度，行保留，手动触发仍然可以领取它。
		return UpsertScheduledTask(schedule)
	}
	schedule.Enabled = ssoFlag(config.Enabled)
	return UpsertScheduledTask(schedule)
}

// ssoSyncPeriodLabels 把周期取值映射到文案 i18n key。
var ssoSyncPeriodLabels = map[string]string{
	define.SsoSyncPeriodNone:    `sso_sync_period_none`,
	define.SsoSyncPeriodHourly:  `sso_sync_period_hourly`,
	define.SsoSyncPeriodFiveMin: `sso_sync_period_every_5min`,
	define.SsoSyncPeriodDaily:   `sso_sync_period_daily`,
	define.SsoSyncPeriodWeekly:  `sso_sync_period_weekly`,
}

// SsoSyncPeriodMeta 是自动同步周期的一个下拉项：取值 + 已翻译文案。
type SsoSyncPeriodMeta struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SsoSyncPeriodMetas 按当前环境下发可选周期，文案按请求语言翻译。
// 清单由后端下发而不是前端硬编码：测试环境多出的短周期要跟着环境出现/消失，
// 前端自己判环境就多出一份可能漂移的判据。
func SsoSyncPeriodMetas(lang string) []SsoSyncPeriodMeta {
	periods := define.SsoSyncPeriodsForEnv()
	list := make([]SsoSyncPeriodMeta, 0, len(periods))
	for _, period := range periods {
		list = append(list, SsoSyncPeriodMeta{
			Value: period,
			Label: lib_i18n.Show(lang, ssoSyncPeriodLabels[period]),
		})
	}
	return list
}

// SsoSyncScopeKey 是一个租户成员同步任务在定时表里的业务唯一键。
func SsoSyncScopeKey(adminId int) string {
	return fmt.Sprintf("%s:%d", define.SsoSyncTaskKey, adminId)
}

// IsSsoSyncRunning 判定该租户的成员同步是否正在执行（租约未过期即视为在跑）。
// 前端配置卡片靠它把「立即同步」按钮置为进行态，不另建运行历史表。
func IsSsoSyncRunning(adminId int) (bool, error) {
	rows, err := msql.RawValues(define.Postgres,
		`SELECT id FROM scheduled_tasks WHERE task_key=$1 AND scope_key=$2 AND lease_until>$3 LIMIT 1`, nil,
		define.SsoSyncTaskKey, SsoSyncScopeKey(adminId), time.Now().Unix())
	if err != nil {
		logs.Error("check sso sync running failed: err=%s", err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

// SsoSiteUrl 返回拼装入口与回调地址用的站点基址（trusted_domain 就是它，恒等派生、不单独存列）。
func SsoSiteUrl() string {
	return strings.TrimRight(define.Config.WebService[`site_url`], `/`)
}

// SsoLoginEntryUrl 拼登录入口地址。scope_id_key 缺失时返回空串，由前端把该行置灰，
// 不降级成明文 adminId。
func SsoLoginEntryUrl(adminId int) (string, error) {
	site := SsoSiteUrl()
	if site == `` {
		return ``, nil
	}
	scopeId, err := EncodeSsoScopeId(adminId)
	if err != nil {
		return ``, err
	}
	return fmt.Sprintf(`%s/sso/login?scope_id=%s`, site, scopeId), nil
}

// SsoCallbackUrl 拼回调地址。provider 决定路径段，callbackKey 是该租户恒定不变的标识。
func SsoCallbackUrl(provider, callbackKey string, bind bool) string {
	site := SsoSiteUrl()
	if site == `` || callbackKey == `` {
		return ``
	}
	url := fmt.Sprintf(`%s/api/sso/callback/%s/%s`, site, provider, callbackKey)
	if bind {
		url += `/bind`
	}
	return url
}

// ssoFlag 把布尔转成 SMALLINT 列取值。
func ssoFlag(value bool) int {
	if value {
		return 1
	}
	return 0
}
