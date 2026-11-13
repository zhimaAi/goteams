package business

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// LicenseUsageTaskKey 标识实例级报告，不属于某个租户的可关闭配置。
const LicenseUsageTaskKey = "commercial_license_usage_report"

const opsPathReportUsage = "/kf/goTeams/reportPrivateLicenseUsage"

type licenseUsageReportModel struct {
	ReportId         string
	LicenseKey       string
	DeviceCode       string
	BuildVersion     string
	ReportedAt       int64
	MemberUsed       int
	StorageUsedBytes int64
	SiteHost         string
}

// EnsureLicenseUsageReportSchedule 只在首次创建调度行时安排首报，重启不重置游标。
func EnsureLicenseUsageReportSchedule() error {
	digest := sha256.Sum256([]byte(CurrentDeviceCode()))
	minute := int(digest[0])<<8 | int(digest[1])
	minute %= 24 * 60
	now := time.Now().Unix()
	_, err := msql.RawExec(define.Postgres, `INSERT INTO scheduled_tasks
		(task_key,scope_key,admin_id,payload,enabled,schedule_type,interval_seconds,
		run_at_weekday,run_at_hour,run_at_minute,next_run_at,created_at,updated_at)
		VALUES ($1,$1,0,'',1,'daily',0,0,$2,$3,$4,$4,$4)
		ON CONFLICT(task_key,scope_key) DO NOTHING`, nil,
		LicenseUsageTaskKey, minute/60, minute%60, now)
	if err != nil {
		logs.Error("ensure license usage schedule failed: sql=INSERT scheduled_tasks ON CONFLICT DO NOTHING, err=%s", err.Error())
	}
	return err
}

// LicenseUsageReportHandler 每日汇总实际用量；网络失败不改变授权快照或试用回执。
func LicenseUsageReportHandler(task ScheduledTask) error {
	return sendCurrentLicenseUsage(strconv.FormatInt(task.NextRunAt, 10))
}

// reportLicenseUsageAfterInstall 在新 Key 已进入内存快照后立即上报一次。
// 上报失败不能回滚已提交的授权；快照未更新时不上报，避免把旧 Key 标成已生效。
func reportLicenseUsageAfterInstall(parsed *common.ParsedLicenseKey) {
	event, ok := installedLicenseReportEvent(GetLicenseSnapshot(), parsed, time.Now().UnixNano())
	if !ok {
		logs.Error("skip license usage report after install: snapshot fingerprint mismatch")
		return
	}
	if err := sendCurrentLicenseUsage(event); err != nil {
		logs.Error("license usage report after install failed: fingerprint=%s, err=%s", parsed.Fingerprint, err.Error())
	}
}

func installedLicenseReportEvent(snapshot *LicenseSnapshot, parsed *common.ParsedLicenseKey, now int64) (string, bool) {
	if parsed == nil || snapshot == nil || snapshot.Parsed == nil || snapshot.Parsed.Fingerprint != parsed.Fingerprint {
		return "", false
	}
	return "install:" + parsed.Fingerprint + ":" + strconv.FormatInt(now, 10), true
}

func sendCurrentLicenseUsage(eventKey string) error {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil || snapshot.DeviceCode == "" {
		return nil
	}
	rootIds, err := findRootTenantIds()
	if err != nil {
		return err
	}
	if len(rootIds) != 1 {
		return ErrLicenseRootTenantMissing
	}
	members, err := countByAdmin(define.TableUsers, rootIds[0])
	if err != nil {
		return err
	}
	bytes, err := GetEnterpriseStorageUsed(rootIds[0])
	if err != nil {
		return err
	}
	return sendLicenseUsageReport(licenseUsageReportModel{
		ReportId: licenseUsageReportID(snapshot.DeviceCode, snapshot.Parsed.Fingerprint, eventKey), LicenseKey: snapshot.Parsed.Raw,
		DeviceCode: snapshot.DeviceCode, BuildVersion: define.BuildVersion,
		ReportedAt: time.Now().Unix(), MemberUsed: members, StorageUsedBytes: bytes,
		SiteHost: configuredSiteHost(),
	})
}

func licenseUsageReportID(deviceCode, fingerprint, eventKey string) string {
	digest := sha256.Sum256([]byte(deviceCode + ":" + eventKey + ":" + fingerprint))
	return hex.EncodeToString(digest[:])
}

func sendLicenseUsageReport(model licenseUsageReportModel) error {
	if model.MemberUsed < 0 || model.StorageUsedBytes < 0 {
		return fmt.Errorf("negative license usage")
	}
	form := url.Values{
		"report_id": {model.ReportId}, "license_key": {model.LicenseKey},
		"device_code": {model.DeviceCode}, "build_version": {model.BuildVersion},
		"reported_at":        {strconv.FormatInt(model.ReportedAt, 10)},
		"member_used":        {strconv.Itoa(model.MemberUsed)},
		"storage_used_bytes": {strconv.FormatInt(model.StorageUsedBytes, 10)},
	}
	if model.SiteHost != "" {
		form.Set("site_host", model.SiteHost)
	}
	resp, err := postOpsForm(opsPathReportUsage, form)
	if err != nil {
		return err
	}
	// 不处理返回的Key或receipt，也不将可能回显凭据的远端消息写入调度错误。
	if resp.Res != 0 {
		return fmt.Errorf("license usage report rejected")
	}
	if reportId, ok := resp.Data["report_id"].(string); !ok || reportId != model.ReportId {
		return fmt.Errorf("license usage report acknowledgment mismatch")
	}
	return nil
}

// configuredSiteHost 取部署配置的站点主机名。没配或不是 http(s) 地址时留空，上报仍继续。
func configuredSiteHost() string {
	raw := strings.TrimSpace(define.Config.WebService["site_url"])
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.Hostname()
}
