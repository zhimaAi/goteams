package business

import (
	"database/sql"
	"errors"
	"fmt"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// PlanModel 表示租户套餐及其配额。
type PlanModel struct {
	AdminId                       int
	PlanType                      string
	ExpireAt                      int
	IsExpired                     bool
	MemberLimit                   int
	ProjectLimit                  int
	CustomFieldPerProjectLimit    int
	AutomationRulePerProjectLimit int
	AutomationExecLimit           int
	AgentLimit                    int
	StorageLimit                  int
	StorageOverDeadline           int
	ApiCallDailyLimit             int
	RequirementTotal              int
	DefectTotal                   int
}

// PlanLimits 表示运营后台可修改的配额字段。
type PlanLimits struct {
	MemberLimit                   int `json:"member_limit"`
	ProjectLimit                  int `json:"project_limit"`
	CustomFieldPerProjectLimit    int `json:"custom_field_per_project_limit"`
	AutomationRulePerProjectLimit int `json:"automation_rule_per_project_limit"`
	AutomationExecLimit           int `json:"automation_exec_limit"`
	AgentLimit                    int `json:"agent_limit"`
	StorageLimit                  int `json:"storage_limit"`
	ApiCallDailyLimit             int `json:"api_call_daily_limit"`
}

// UsageItemModel 表示版本信息页的一项用量。
type UsageItemModel struct {
	Key             string
	Used            int
	Limit           int
	EnterpriseLimit int
	PerProjectLimit int
	TotalLimit      int
}

// PlanUsageModel 表示套餐及全部用量。
type PlanUsageModel struct {
	AdminId        int
	EnterpriseName string
	PlanType       string
	ExpireAt       int
	IsExpired      bool
	Items          []UsageItemModel
}

// UpdateEnterpriseNameModel 表示修改企业名称业务数据。
type UpdateEnterpriseNameModel struct {
	AdminId        int
	UserId         int
	EnterpriseName string
}

// QuotaExceededError 表示配额已达到上限。
type QuotaExceededError struct {
	Key   string `json:"key"`
	Scope string `json:"scope"`
	Used  int    `json:"used"`
	Limit int    `json:"limit"`
}

// Error 实现 error 接口。
func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("quota exceeded: key=%s scope=%s used=%d limit=%d", e.Key, e.Scope, e.Used, e.Limit)
}

// ErrPlanNotFound 表示租户套餐不存在。
var ErrPlanNotFound = errors.New(`tenant plan not found`)

// ErrEnterpriseNotFound 表示企业不存在。
var ErrEnterpriseNotFound = errors.New(`enterprise not found`)

// ErrEnterpriseAdminRequired 表示操作仅允许租户根管理员执行。
var ErrEnterpriseAdminRequired = errors.New(`enterprise administrator required`)

// ErrStorageExtendUnavailable 表示当前未容量超限，不可延期。
var ErrStorageExtendUnavailable = errors.New(`storage not over limit, extension unavailable`)

// ErrStorageExtendOnlyOnce 表示该企业已延期过一次，不可再次延期。
var ErrStorageExtendOnlyOnce = errors.New(`storage extension already used`)

// StorageExtendSeconds 是容量超限延期时长（3 天）。
const StorageExtendSeconds = 3 * 24 * 60 * 60

// freePlanExpireAt 是体验版默认到期时间：2099-01-01 00:00:00 +0800。
var freePlanExpireAt = int(time.Date(2099, time.January, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60)).Unix())

// EnsureDefaultPlan 为租户幂等创建体验版套餐。
func EnsureDefaultPlan(adminId int) error {
	if adminId <= 0 {
		return ErrPlanNotFound
	}
	m := msql.Model(define.TableTenantPlans, define.Postgres)
	id, err := m.Where(`admin_id`, cast.ToString(adminId)).Value(`id`)
	if err != nil {
		return err
	}
	if id != `` {
		return nil
	}
	now := tool.Time2Int()
	insertM := msql.Model(define.TableTenantPlans, define.Postgres)
	_, err = insertM.Insert(msql.Datas{
		`admin_id`:                          adminId,
		`plan_type`:                         define.PlanExperience,
		`expire_at`:                         freePlanExpireAt,
		`member_limit`:                      50,
		`project_limit`:                     10,
		`custom_field_per_project_limit`:    10,
		`automation_rule_per_project_limit`: 5,
		`automation_exec_limit`:             500,
		`agent_limit`:                       3,
		`storage_limit`:                     50,
		`api_call_daily_limit`:              100,
		`created_at`:                        now,
		`updated_at`:                        now,
	})
	if err != nil {
		// 并发初始化时唯一索引可能已由另一请求写入，存在记录即可视为成功。
		checkM := msql.Model(define.TableTenantPlans, define.Postgres)
		if existing, checkErr := checkM.Where(`admin_id`, cast.ToString(adminId)).Value(`id`); checkErr == nil && existing != `` {
			return nil
		}
		logs.Error("ensure default plan failed: sql=%s, err=%s", insertM.GetLastSql(), err.Error())
	}
	return err
}

// GetPlan 获取租户套餐，不存在时自动补齐体验套餐。
// 开源版：授权不可用时直接阻断；授权有效时以已验签 claims 覆盖期限与配额，
// tenant_plans 字段仅作本地展示投影，任何授权判定不得回退到该表数值。
func GetPlan(adminId int) (PlanModel, error) {
	return getPlan(adminId, false)
}

// GetPlanForLicenseManagement returns the verified plan projection for the restricted renewal
// page. It must not be used by quota or feature authorization paths.
func GetPlanForLicenseManagement(adminId int) (PlanModel, error) {
	return getPlan(adminId, true)
}

func getPlan(adminId int, allowBlocked bool) (PlanModel, error) {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil || (!allowBlocked && !define.LicenseStateCanLogin(snapshot.State)) {
		return PlanModel{}, ErrLicenseNotActivated
	}
	if err := EnsureDefaultPlan(adminId); err != nil {
		return PlanModel{}, err
	}
	m := msql.Model(define.TableTenantPlans, define.Postgres)
	row, err := m.Field(`admin_id, plan_type, expire_at, member_limit, project_limit, custom_field_per_project_limit, automation_rule_per_project_limit, automation_exec_limit, agent_limit, storage_limit, storage_over_deadline, api_call_daily_limit, requirement_total, defect_total`).
		Where(`admin_id`, cast.ToString(adminId)).Find()
	if err != nil {
		logs.Error("get plan failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return PlanModel{}, err
	}
	if len(row) == 0 {
		return PlanModel{}, ErrPlanNotFound
	}
	plan, err := planFromRow(row)
	if err != nil {
		return PlanModel{}, err
	}
	applyLicensePlanOverride(&plan)
	plan.IsExpired = isPlanExpired(plan.ExpireAt)
	return plan, nil
}

// isPlanExpired 判断套餐是否已过期；expire_at 为 0 表示永久有效。
func isPlanExpired(expireAt int) bool {
	if expireAt <= 0 {
		return false
	}
	return time.Now().Unix() > int64(expireAt)
}

// GetPlanUsage 汇总版本信息页需要的真实用量。
func GetPlanUsage(model PlanUsageModel) (PlanUsageModel, error) {
	return getPlanUsage(model, GetPlan)
}

// GetPlanUsageForLicenseManagement returns display-only usage while business APIs are blocked.
func GetPlanUsageForLicenseManagement(model PlanUsageModel) (PlanUsageModel, error) {
	return getPlanUsage(model, GetPlanForLicenseManagement)
}

func getPlanUsage(model PlanUsageModel, loadPlan func(int) (PlanModel, error)) (PlanUsageModel, error) {
	plan, err := loadPlan(model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	model.EnterpriseName, err = getEnterpriseName(model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	memberUsed, err := countByAdmin(define.TableUsers, model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	projectUsed, err := countByAdmin(define.TableWorkspaces, model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	customFieldUsed, err := countByAdmin(define.TableCustomFields, model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	agentUsed, err := countAgents(model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	automationRuleUsed, err := countByAdmin(define.TableAutomationRules, model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	automationExecUsed, err := countAutomationExecutions(model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	customTotal := plan.CustomFieldPerProjectLimit * plan.ProjectLimit
	automationTotal := plan.AutomationRulePerProjectLimit * plan.ProjectLimit
	// Storage usage is aggregated from the enterprise file ledger shard. Used returns raw bytes;
	// the frontend displays in KB/MB/GB and calculates percentages after converting bytes to GB.
	storageUsedBytes, err := GetEnterpriseStorageUsed(model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	// API 调用按自然日计数，用量取自租户当日累计，上限即套餐的每日额度。
	apiCallUsed, err := countApiCallsToday(model.AdminId)
	if err != nil {
		return PlanUsageModel{}, err
	}
	model.PlanType = plan.PlanType
	model.ExpireAt = plan.ExpireAt
	model.IsExpired = plan.IsExpired
	model.Items = []UsageItemModel{
		{Key: define.QuotaMember, Used: memberUsed, Limit: plan.MemberLimit, EnterpriseLimit: 10},
		{Key: define.QuotaProject, Used: projectUsed, Limit: plan.ProjectLimit, EnterpriseLimit: 200},
		{Key: define.QuotaCustomField, Used: customFieldUsed, Limit: customTotal, EnterpriseLimit: 10000, PerProjectLimit: plan.CustomFieldPerProjectLimit, TotalLimit: customTotal},
		{Key: define.QuotaAutomationRule, Used: automationRuleUsed, Limit: automationTotal, EnterpriseLimit: 10000, PerProjectLimit: plan.AutomationRulePerProjectLimit, TotalLimit: automationTotal},
		{Key: define.QuotaAutomationExec, Used: automationExecUsed, Limit: plan.AutomationExecLimit, EnterpriseLimit: 100000},
		{Key: define.QuotaAgent, Used: agentUsed, Limit: plan.AgentLimit, EnterpriseLimit: 10},
		{Key: define.QuotaStorage, Used: int(storageUsedBytes), Limit: plan.StorageLimit, EnterpriseLimit: 200},
		{Key: define.QuotaApiCall, Used: apiCallUsed, Limit: plan.ApiCallDailyLimit, EnterpriseLimit: 10000},
	}
	return model, nil
}

// countApiCallsToday 返回租户当天的 API 调用次数；未产生调用时返回 0。
func countApiCallsToday(adminId int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT used_calls FROM `+define.TableApiAdminDailyQuotas+`
		WHERE admin_id = $1 AND day = $2 LIMIT 1`, nil, adminId, common.ApiQuotaDay(time.Now()))
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	return cast.ToInt(rows[0][`used_calls`]), nil
}

// storageCapacityStatus 计算存储容量状态：0=正常或宽限期内、1=首次超限可延期、2=已延期过且超限（禁止进入）。
// usedBytes 为已用字节；limitGB<=0 视为无限配额；deadline<=0 表示从未延期；now==deadline 视为已过期。
func storageCapacityStatus(usedBytes int64, limitGB, deadline, now int64) int {
	if limitGB <= 0 {
		return 0
	}
	limitBytes := limitGB * 1024 * 1024 * 1024
	if usedBytes <= limitBytes {
		return 0
	}
	if deadline <= 0 {
		return 1
	}
	if now >= deadline {
		return 2
	}
	return 0
}

// GetCapacityStatus 返回当前租户的存储容量状态（0/1/2）。
func GetCapacityStatus(plan PlanModel) (int, error) {
	usedBytes, err := GetEnterpriseStorageUsed(plan.AdminId)
	if err != nil {
		return 0, err
	}
	return storageCapacityStatus(usedBytes, int64(plan.StorageLimit), int64(plan.StorageOverDeadline), time.Now().Unix()), nil
}

// ExtendStorageDeadline 将容量超限截止时间延后 3 天，仅允许租户根管理员且每企业仅一次。
// 返回新的截止时间（Unix 秒）。
func ExtendStorageDeadline(adminId, userId int) (int, error) {
	if adminId <= 0 || userId != adminId {
		return 0, ErrEnterpriseAdminRequired
	}
	plan, err := GetPlan(adminId)
	if err != nil {
		return 0, err
	}
	usedBytes, err := GetEnterpriseStorageUsed(adminId)
	if err != nil {
		return 0, err
	}
	switch storageCapacityStatus(usedBytes, int64(plan.StorageLimit), int64(plan.StorageOverDeadline), time.Now().Unix()) {
	case 0:
		// 未超限或宽限期内 → 无需延期
		return 0, ErrStorageExtendUnavailable
	case 2:
		// 已延期过且宽限期已过 → 仅一次机会
		return 0, ErrStorageExtendOnlyOnce
	case 1:
		// 首次超限，可延期 → 继续
	}
	deadline := time.Now().Unix() + StorageExtendSeconds
	m := msql.Model(define.TableTenantPlans, define.Postgres)
	affected, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`storage_over_deadline`, `0`},
	}).Update(msql.Datas{
		`storage_over_deadline`: deadline,
		`updated_at`:            tool.Time2Int(),
	})
	if err != nil {
		logs.Error("extend storage deadline failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return 0, err
	}
	if affected == 0 {
		// 并发下另一请求已延期，保证每企业仅一次。
		return 0, ErrStorageExtendOnlyOnce
	}
	return int(deadline), nil
}

// UpdateEnterpriseName 修改租户根管理员记录中的企业名称。
func UpdateEnterpriseName(model UpdateEnterpriseNameModel) (UpdateEnterpriseNameModel, error) {
	model.EnterpriseName = strings.TrimSpace(model.EnterpriseName)
	if model.AdminId <= 0 || !IsTenantOwner(model.AdminId, model.UserId) {
		return UpdateEnterpriseNameModel{}, ErrEnterpriseAdminRequired
	}
	if err := validate.ValidateEnterpriseName(model.EnterpriseName); err != nil {
		return UpdateEnterpriseNameModel{}, err
	}
	m := msql.Model(define.TableUsers, define.Postgres)
	_, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`id`, cast.ToString(model.AdminId)},
	}).Update(msql.Datas{
		`enterprise_name`: model.EnterpriseName,
		`updated_at`:      tool.Time2Int(),
	})
	if err != nil {
		logs.Error("update enterprise name failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return UpdateEnterpriseNameModel{}, err
	}
	return model, nil
}

func getEnterpriseName(adminId int) (string, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(`enterprise_name`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(adminId)},
	}).Find()
	if err != nil {
		logs.Error("get enterprise name failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return ``, err
	}
	if len(row) == 0 {
		return ``, ErrEnterpriseNotFound
	}
	return row[`enterprise_name`], nil
}

// CheckQuota 保留共享调用入口的授权检查；私有化不再限制工作空间、字段或 Agent 数量。
func CheckQuota(adminId int, key string, wsId ...string) error {
	_, err := GetPlan(adminId)
	return err
}

// CheckMemberQuotaTx 在事务内以咨询锁串行校验成员席位，防止并发超卖套餐配额。
// 调用前需保证租户套餐存在（EnsureDefaultPlan），且用户插入必须在同一事务内完成。
// 开源版：席位上限以已验签授权 claims 为准，覆盖 tenant_plans 投影值；授权不可用时阻断。
func CheckMemberQuotaTx(tx *sql.Tx, adminId int) error {
	// 命名空间 (-adminId, -1)：负 adminId 为配额锁，-1 与 automation 的 wsId>=0 隔离
	if _, err := msql.RawValues(define.Postgres, `SELECT pg_advisory_xact_lock($1, -1)`, tx, -adminId); err != nil {
		logs.Error("lock member quota failed: %s", err.Error())
		return err
	}
	limitRows, err := msql.RawValues(define.Postgres, `SELECT member_limit FROM tenant_plans WHERE admin_id=$1 FOR SHARE`, tx, adminId)
	if err != nil {
		logs.Error("get member limit failed: %s", err.Error())
		return err
	}
	if len(limitRows) == 0 {
		return ErrPlanNotFound
	}
	countRows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS used FROM users WHERE admin_id=$1`, tx, adminId)
	if err != nil {
		logs.Error("count tenant members failed: %s", err.Error())
		return err
	}
	used, limit := cast.ToInt(countRows[0][`used`]), cast.ToInt(limitRows[0][`member_limit`])
	quotas, licenseOk := licenseQuotasIfValid()
	if !licenseOk {
		return ErrLicenseNotActivated
	}
	limit = int(quotas.MemberLimit)
	if limit <= 0 || used >= limit {
		return &QuotaExceededError{Key: define.QuotaMember, Scope: `total`, Used: used, Limit: limit}
	}
	return nil
}

// UpdatePlanLimits 更新租户配额。
func UpdatePlanLimits(adminId int, limits PlanLimits) error {
	if err := EnsureDefaultPlan(adminId); err != nil {
		return err
	}
	m := msql.Model(define.TableTenantPlans, define.Postgres)
	_, err := m.Where(`admin_id`, cast.ToString(adminId)).Update(msql.Datas{
		`member_limit`:                      limits.MemberLimit,
		`project_limit`:                     limits.ProjectLimit,
		`custom_field_per_project_limit`:    limits.CustomFieldPerProjectLimit,
		`automation_rule_per_project_limit`: limits.AutomationRulePerProjectLimit,
		`automation_exec_limit`:             limits.AutomationExecLimit,
		`agent_limit`:                       limits.AgentLimit,
		`storage_limit`:                     limits.StorageLimit,
		`api_call_daily_limit`:              limits.ApiCallDailyLimit,
		// 运营后台调整配额即视为续费/恢复，清空容量超限截止时间。
		`storage_over_deadline`: 0,
		`updated_at`:            tool.Time2Int(),
	})
	return err
}

// ActivateEnterprise 开通企业版并写入到期时间和配额。
func ActivateEnterprise(adminId int, expireAt int, limits PlanLimits) error {
	if err := UpdatePlanLimits(adminId, limits); err != nil {
		return err
	}
	m := msql.Model(define.TableTenantPlans, define.Postgres)
	_, err := m.Where(`admin_id`, cast.ToString(adminId)).Update(msql.Datas{
		`plan_type`:  define.PlanEnterprise,
		`expire_at`:  expireAt,
		`updated_at`: tool.Time2Int(),
	})
	return err
}

// UpdatePlanExpireAt 更新套餐到期时间，不触发套餐自动切换。
func UpdatePlanExpireAt(adminId int, expireAt int) error {
	if err := EnsureDefaultPlan(adminId); err != nil {
		return err
	}
	m := msql.Model(define.TableTenantPlans, define.Postgres)
	_, err := m.Where(`admin_id`, cast.ToString(adminId)).Update(msql.Datas{
		`expire_at`: expireAt,
		// 运营后台续费后清空容量超限截止时间，恢复延期机会。
		`storage_over_deadline`: 0,
		`updated_at`:            tool.Time2Int(),
	})
	return err
}

func planFromRow(row msql.Params) (PlanModel, error) {
	requirementTotal, err := parseRowInt(row, `requirement_total`)
	if err != nil {
		return PlanModel{}, err
	}
	defectTotal, err := parseRowInt(row, `defect_total`)
	if err != nil {
		return PlanModel{}, err
	}
	adminId, err := parseRowInt(row, `admin_id`)
	if err != nil {
		return PlanModel{}, err
	}
	expireAt, err := parseRowInt(row, `expire_at`)
	if err != nil {
		return PlanModel{}, err
	}
	memberLimit, err := parseRowInt(row, `member_limit`)
	if err != nil {
		return PlanModel{}, err
	}
	projectLimit, err := parseRowInt(row, `project_limit`)
	if err != nil {
		return PlanModel{}, err
	}
	customFieldLimit, err := parseRowInt(row, `custom_field_per_project_limit`)
	if err != nil {
		return PlanModel{}, err
	}
	automationRuleLimit, err := parseRowInt(row, `automation_rule_per_project_limit`)
	if err != nil {
		return PlanModel{}, err
	}
	automationExecLimit, err := parseRowInt(row, `automation_exec_limit`)
	if err != nil {
		return PlanModel{}, err
	}
	agentLimit, err := parseRowInt(row, `agent_limit`)
	if err != nil {
		return PlanModel{}, err
	}
	storageLimit, err := parseRowInt(row, `storage_limit`)
	if err != nil {
		return PlanModel{}, err
	}
	storageOverDeadline, err := parseRowInt(row, `storage_over_deadline`)
	if err != nil {
		return PlanModel{}, err
	}
	// 老租户在迁移加列前已存在套餐行？不会：迁移带 NOT NULL DEFAULT，缺失只可能是查询未取该列。
	apiCallDailyLimit := 0
	if _, ok := row[`api_call_daily_limit`]; ok {
		apiCallDailyLimit, err = parseRowInt(row, `api_call_daily_limit`)
		if err != nil {
			return PlanModel{}, err
		}
	}
	return PlanModel{
		AdminId:                       adminId,
		PlanType:                      row[`plan_type`],
		ExpireAt:                      expireAt,
		MemberLimit:                   memberLimit,
		ProjectLimit:                  projectLimit,
		CustomFieldPerProjectLimit:    customFieldLimit,
		AutomationRulePerProjectLimit: automationRuleLimit,
		AutomationExecLimit:           automationExecLimit,
		AgentLimit:                    agentLimit,
		StorageLimit:                  storageLimit,
		StorageOverDeadline:           storageOverDeadline,
		ApiCallDailyLimit:             apiCallDailyLimit,
		RequirementTotal:              requirementTotal,
		DefectTotal:                   defectTotal,
	}, nil
}

func parseRowInt(row msql.Params, field string) (int, error) {
	value, ok := row[field]
	if !ok {
		return 0, fmt.Errorf("missing %s", field)
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}
	return result, nil
}

func countByAdmin(table string, adminId int) (int, error) {
	m := msql.Model(table, define.Postgres)
	return m.Where(`admin_id`, cast.ToString(adminId)).Count()
}

// countAgents 统计未软删除的 Agent 数量。
func countAgents(adminId int) (int, error) {
	m := msql.Model(define.TablePipeline, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`deleted_at`, `0`},
	}).Count()
}

// countAutomationExecutions counts automation executions in the current month, matching the execution quota check.
func countAutomationExecutions(adminId int) (int, error) {
	monthStart, monthEnd := automationMonthRange(time.Now())
	m := msql.Model(define.TableAutomationExecutions, define.Postgres)
	return m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`created_at`, `>=`, cast.ToString(monthStart)},
		{`created_at`, `<`, cast.ToString(monthEnd)},
	}).Count()
}
