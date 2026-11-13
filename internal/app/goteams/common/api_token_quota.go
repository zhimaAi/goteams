// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"time"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// apiQuotaLocation 是 API 调用配额的切日时区，与自动化配额一致取 Asia/Shanghai。
var apiQuotaLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// DefaultApiCallDailyLimit 是套餐行缺失或额度非正数时的兜底额度，与体验版一致。
const DefaultApiCallDailyLimit = 100

// ApiQuotaDay 返回 t 在配额时区下的自然日，格式 YYYYMMDD。
func ApiQuotaDay(t time.Time) int {
	local := t.In(apiQuotaLocation)
	return local.Year()*10000 + int(local.Month())*100 + local.Day()
}

// ApiCallDailyLimit 读取租户每日 API 调用上限，取不到时返回体验版默认值。
func ApiCallDailyLimit(adminId int) int {
	rows, err := msql.RawValues(define.Postgres, `SELECT api_call_daily_limit FROM `+define.TableTenantPlans+` WHERE admin_id = $1 LIMIT 1`, nil, adminId)
	if err != nil {
		logs.Error("get api call daily limit failed: admin_id=%d, err=%s", adminId, err.Error())
		return DefaultApiCallDailyLimit
	}
	if len(rows) == 0 {
		return DefaultApiCallDailyLimit
	}
	if limit := cast.ToInt(rows[0][`api_call_daily_limit`]); limit > 0 {
		return limit
	}
	return DefaultApiCallDailyLimit
}

// ApiQuotaConsume 原子累计私有化当日调用量，不再按套餐日额度拒绝。
func ApiQuotaConsume(adminId, day int) (bool, error) {
	sql := `INSERT INTO ` + define.TableApiAdminDailyQuotas + ` (admin_id, day, used_calls)
		VALUES ($1, $2, 1)
		ON CONFLICT (admin_id, day) DO UPDATE SET used_calls = ` + define.TableApiAdminDailyQuotas + `.used_calls + 1
		RETURNING used_calls`
	rows, err := msql.RawValues(define.Postgres, sql, nil, adminId, day)
	if err != nil {
		logs.Error("consume api quota failed: admin_id=%d, err=%s", adminId, err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

// ApiTokenStatAdd 累加 token 的当日调用明细与累计次数。
// 只在调用成功时执行，对应列表「今日/昨日/累计触发」的统计口径。
func ApiTokenStatAdd(tokenId, day int) {
	if tokenId <= 0 {
		return
	}
	if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableApiTokenDailyStats+` (token_id, day, calls)
		VALUES ($1, $2, 1)
		ON CONFLICT (token_id, day) DO UPDATE SET calls = `+define.TableApiTokenDailyStats+`.calls + 1`, nil, tokenId, day); err != nil {
		logs.Error("add api token daily stat failed: token_id=%d, err=%s", tokenId, err.Error())
	}
	if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableApiTokens+`
		SET cumulative_calls = cumulative_calls + 1, last_used_at = $1 WHERE id = $2`, nil, time.Now().Unix(), tokenId); err != nil {
		logs.Error("add api token cumulative failed: token_id=%d, err=%s", tokenId, err.Error())
	}
}
