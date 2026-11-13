// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// License system business layer.
//
// The signed license claims are the single source of truth for edition, expiry, quotas and
// features of the commercial build. tenant_plans is only a read projection for legacy display
// and statistics and must never loosen authorization. commercial_licenses is an instance-level
// singleton (id=1) and deliberately carries no admin_id/workspace_id columns.

// License runtime errors carrying the blocking state for controller mapping.
var (
	ErrLicenseNotActivated      = &LicenseError{State: define.LicenseStateNotActivated}
	ErrLicenseExpired           = &LicenseError{State: define.LicenseStateExpired}
	ErrLicenseInvalid           = &LicenseError{State: define.LicenseStateInvalid}
	ErrLicenseDeviceMismatch    = &LicenseError{State: define.LicenseStateDeviceMismatch}
	ErrLicenseNotYetValid       = &LicenseError{State: define.LicenseStateNotYetValid}
	ErrLicenseClockInvalid      = &LicenseError{State: define.LicenseStateClockInvalid}
	ErrLicenseConfigError       = &LicenseError{State: define.LicenseStateConfigError}
	ErrLicenseRevisionRegressed = errors.New(`license revision regression`)
	ErrLicenseRootTenantMissing = errors.New(`commercial root tenant missing`)
)

// LicenseError marks an install/verification failure with the derived blocking state.
type LicenseError struct {
	State string
	Err   error
}

// Error implements the error interface.
func (e *LicenseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf(`license state %s: %v`, e.State, e.Err)
	}
	return `license state ` + e.State
}

// Unwrap exposes the wrapped cause.
func (e *LicenseError) Unwrap() error {
	return e.Err
}

// NewLicenseError wraps a cause with a blocking license state.
func NewLicenseError(state string, cause error) *LicenseError {
	return &LicenseError{State: state, Err: cause}
}

// EffectivePlan is the immutable plan derived from verified license claims.
// It must never be built from request payloads or database defaults.
type EffectivePlan struct {
	LicenseNo      string
	CorpName       string
	ActivationMode string
	LicenseKind    string
	Revision       int
	ExpireAt       int64
	DeviceCode     string
	Quotas         common.LicenseQuotaClaims
	Claims         *common.LicenseClaims
}

// LicenseSnapshot is the process-wide verified license view consumed by middleware and APIs.
type LicenseSnapshot struct {
	State      string
	DeviceCode string
	Parsed     *common.ParsedLicenseKey
	Effective  EffectivePlan
}

// LicenseRecord mirrors the commercial_licenses singleton row.
type LicenseRecord struct {
	Id                 int
	LicenseKey         string
	LicenseNo          string
	ActivationMode     string
	CorpName           string
	DeviceCode         string
	IssuedAt           int64
	NotBefore          int64
	ExpireAt           int64
	Revision           int
	ClaimsHash         string
	OnlineCheckReceipt string
	LastOnlineSyncAt   int64
	LastSeenAt         int64
}

// InstallLicenseResult returns the verified outcome of a successful key installation.
type InstallLicenseResult struct {
	LicenseNo      string
	CorpName       string
	ActivationMode string
	Revision       int
	ExpireAt       int64
	State          string
	Quotas         common.LicenseQuotaClaims
}

// Runtime license state shared across goroutines.
var (
	licenseSnapshotMu   sync.RWMutex
	licenseSnapshot     *LicenseSnapshot
	licenseDeviceCode   string
	licenseFingerprints common.DeviceFingerprints
)

// InitCommercialLicense prepares the device identity, default administrator and license
// snapshot. It never aborts startup: failures degrade into the config_error state which the
// global middleware turns into blocked business APIs while /api/ping and status stay alive.
func InitCommercialLicense() {
	instanceFile := strings.TrimSpace(define.LicenseConfig[`instance_id_file`])
	info, err := common.LoadDeviceInfo(instanceFile, define.IsDev)
	if err != nil {
		logs.Error(`load device identity failed: %s`, err.Error())
		setLicenseSnapshot(&LicenseSnapshot{State: define.LicenseStateConfigError})
		return
	}
	deviceCode, err := common.GenerateDeviceCode(info)
	if err != nil {
		logs.Error(`generate device code failed: %s`, err.Error())
		setLicenseSnapshot(&LicenseSnapshot{State: define.LicenseStateConfigError})
		return
	}
	licenseSnapshotMu.Lock()
	licenseDeviceCode = deviceCode
	licenseFingerprints = common.ComputeDeviceFingerprints(info)
	licenseSnapshotMu.Unlock()

	if err = EnsureCommercialBootstrap(); err != nil {
		logs.Error(`commercial bootstrap failed: %s`, err.Error())
		setLicenseSnapshot(&LicenseSnapshot{State: define.LicenseStateConfigError, DeviceCode: deviceCode})
		return
	}
	if err = reloadLicenseSnapshot(); err != nil {
		logs.Error(`reload license snapshot failed: %s`, err.Error())
		setLicenseSnapshot(&LicenseSnapshot{State: define.LicenseStateConfigError, DeviceCode: deviceCode})
		return
	}
	// 运维升级授权：人工把新 Key 放入 pending 文件，此处用与 API 完全相同的
	// InstallLicenseKey 事务消费；失败保留原 Key 与原配额。
	consumePendingLicenseKey()
	// Trial licenses run in mandatory-online mode: every process start must pass one
	// synchronous online re-verification before login and business APIs open.
	runTrialStartupOnlineCheck()
}

// consumePendingLicenseKey installs the operator-provisioned key from the pending file using the
// same InstallLicenseKey transaction as the API. Success deletes the file; failure keeps both
// the file and the previously installed key, and never logs the key content.
func consumePendingLicenseKey() {
	path := strings.TrimSpace(define.LicenseConfig[`pending_key_file`])
	if path == `` {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logs.Error(`read pending license key file failed: %s`, err.Error())
		}
		return
	}
	key := strings.TrimSpace(string(raw))
	if key == `` {
		_ = os.Remove(path)
		return
	}
	result, installErr := InstallLicenseKey(key)
	if installErr != nil {
		logs.Error(`install pending license key failed: %s`, installErr.Error())
		return
	}
	_ = os.Remove(path)
	logs.Info(`pending license key installed: license_no=%s revision=%d expire_at=%d`, result.LicenseNo, result.Revision, result.ExpireAt)
}

// CurrentDeviceCode returns the immutable display device code generated at startup.
func CurrentDeviceCode() string {
	licenseSnapshotMu.RLock()
	defer licenseSnapshotMu.RUnlock()
	return licenseDeviceCode
}

// CurrentDeviceFingerprints returns the salted hardware digests for trial requests.
func CurrentDeviceFingerprints() common.DeviceFingerprints {
	licenseSnapshotMu.RLock()
	defer licenseSnapshotMu.RUnlock()
	return licenseFingerprints
}

// GetLicenseSnapshot returns the current verified license snapshot (never nil).
func GetLicenseSnapshot() *LicenseSnapshot {
	licenseSnapshotMu.RLock()
	defer licenseSnapshotMu.RUnlock()
	if licenseSnapshot == nil {
		return &LicenseSnapshot{State: define.LicenseStateNotActivated}
	}
	return licenseSnapshot
}

func setLicenseSnapshot(snapshot *LicenseSnapshot) {
	licenseSnapshotMu.Lock()
	defer licenseSnapshotMu.Unlock()
	licenseSnapshot = snapshot
}

// reloadLicenseSnapshot re-reads the singleton row, re-verifies the stored key from scratch
// and rebuilds the effective plan. Any verification failure degrades the state instead of
// trusting stored projections.
func reloadLicenseSnapshot() error {
	record, err := loadLicenseRecord()
	if err != nil {
		return err
	}
	deviceCode := CurrentDeviceCode()
	if record == nil || record.LicenseKey == `` {
		setLicenseSnapshot(&LicenseSnapshot{State: define.LicenseStateNotActivated, DeviceCode: deviceCode})
		return nil
	}
	parsed, err := common.ParseLicenseKey(record.LicenseKey)
	if err != nil {
		logs.Error(`stored license key verify failed: fingerprint=%s, err=%s`, licenseFingerprintOf(record.LicenseKey), err.Error())
		setLicenseSnapshot(&LicenseSnapshot{State: define.LicenseStateInvalid, DeviceCode: deviceCode})
		return nil
	}
	state := common.EvaluateLicenseClaims(&parsed.Claims, time.Now().Unix(), deviceCode)
	// Trial keys additionally require a fresh signed receipt (mandatory-online mode).
	state = gateTrialLicenseState(parsed, record.OnlineCheckReceipt, state)
	setLicenseSnapshot(&LicenseSnapshot{
		State:      state,
		DeviceCode: deviceCode,
		Parsed:     parsed,
		Effective:  effectivePlanFromClaims(parsed),
	})
	return nil
}

// licenseFingerprintOf logs a short key fingerprint without exposing the key itself.
func licenseFingerprintOf(key string) string {
	return common.LicenseKeyFingerprint(key)
}

// effectivePlanFromClaims builds the immutable plan from verified claims only.
func effectivePlanFromClaims(parsed *common.ParsedLicenseKey) EffectivePlan {
	return EffectivePlan{
		LicenseNo:      parsed.Claims.LicenseNo,
		CorpName:       parsed.Claims.CorpName,
		ActivationMode: parsed.Claims.ActivationMode,
		LicenseKind:    parsed.Claims.LicenseKind,
		Revision:       parsed.Claims.Revision,
		ExpireAt:       parsed.Claims.ExpireAt,
		DeviceCode:     parsed.Claims.DeviceCode,
		Quotas:         parsed.Claims.Quotas,
		Claims:         &parsed.Claims,
	}
}

// loadLicenseRecord reads the commercial_licenses singleton (id=1) without blocking state.
func loadLicenseRecord() (*LicenseRecord, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, license_key, license_no, activation_mode, corp_name,
		device_code, issued_at, not_before, expire_at, revision, claims_hash, online_check_receipt,
		last_online_sync_at, last_seen_at FROM commercial_licenses WHERE id=1`, nil)
	if err != nil {
		logs.Error(`load license record failed: %s`, err.Error())
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	return &LicenseRecord{
		Id:                 cast.ToInt(row[`id`]),
		LicenseKey:         row[`license_key`],
		LicenseNo:          row[`license_no`],
		ActivationMode:     row[`activation_mode`],
		CorpName:           row[`corp_name`],
		DeviceCode:         row[`device_code`],
		IssuedAt:           cast.ToInt64(row[`issued_at`]),
		NotBefore:          cast.ToInt64(row[`not_before`]),
		ExpireAt:           cast.ToInt64(row[`expire_at`]),
		Revision:           cast.ToInt(row[`revision`]),
		ClaimsHash:         row[`claims_hash`],
		OnlineCheckReceipt: row[`online_check_receipt`],
		LastOnlineSyncAt:   cast.ToInt64(row[`last_online_sync_at`]),
		LastSeenAt:         cast.ToInt64(row[`last_seen_at`]),
	}, nil
}

// findRootTenantIds returns ids of tenants whose admin_id points to themselves.
func findRootTenantIds() ([]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM users WHERE id=admin_id AND deleted_at=0`, nil)
	if err != nil {
		logs.Error(`find root tenants failed: %s`, err.Error())
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt(row[`id`]))
	}
	return ids, nil
}

// EnsureCommercialBootstrap guarantees exactly one root tenant for the commercial build:
//   - none: create the default admin from [account] config and seed roles/plan;
//   - one: idempotently backfill roles and plan without touching existing user data;
//   - multiple: refuse to guess and surface config_error.
func EnsureCommercialBootstrap() error {
	username := strings.ToLower(strings.TrimSpace(define.AccountConfig[`admin_username`]))
	password := define.AccountConfig[`admin_password`]
	name := strings.TrimSpace(define.AccountConfig[`admin_name`])
	if username == `` || password == `` || name == `` {
		return fmt.Errorf(`commercial [account] config incomplete`)
	}
	if err := validate.IsValidPassword(password); err != nil {
		return fmt.Errorf(`commercial admin password invalid: %w`, err)
	}
	rootIds, err := findRootTenantIds()
	if err != nil {
		return err
	}
	switch len(rootIds) {
	case 0:
		if UserUsernameExist(username) {
			return fmt.Errorf(`commercial admin username %s already taken by non-root user`, username)
		}
		user, err := CreateUser(CreateUserModel{
			Username:       username,
			Password:       password,
			Name:           name,
			EnterpriseName: name,
		})
		if err != nil {
			return fmt.Errorf(`create commercial admin: %w`, err)
		}
		if err = SetUserAdminId(user.UserId); err != nil {
			return err
		}
		adminRoleId, _, err := SeedEnterpriseRoles(user.UserId)
		if err != nil {
			return err
		}
		if err = SetUserRoleId(user.UserId, adminRoleId); err != nil {
			return err
		}
		if err = EnsureDefaultPlan(user.UserId); err != nil {
			return err
		}
		// Never log the credentials themselves.
		logs.Info(`commercial default admin created: user_id=%d username=%s`, user.UserId, username)
		return nil
	case 1:
		rootId := rootIds[0]
		if _, _, err = SeedEnterpriseRoles(rootId); err != nil {
			return err
		}
		if err = EnsureDefaultPlan(rootId); err != nil {
			return err
		}
		logs.Info(`commercial bootstrap idempotent check done: root_admin_id=%d`, rootId)
		return nil
	default:
		return fmt.Errorf(`%w: found %d root tenants`, ErrLicenseRootTenantMissing, len(rootIds))
	}
}

// InstallLicenseKey verifies a GTLIC1 key end to end and atomically replaces the stored
// authorization. Steps:
//  1. outside the transaction: parse, signature, product, device, time, status, quota checks;
//  2. build the immutable effective plan from verified claims only;
//  3. lock the singleton row, compare revisions (same key idempotent, lower rejected);
//  4. persist the full key plus projections and project expiry/quotas onto tenant_plans;
//  5. commit first, then swap the in-process snapshot; failure keeps the previous key.
//
// Trial keys additionally refresh their online receipt immediately after a successful
// commit, so the mandatory-online gate opens without waiting for the periodic loop.
func InstallLicenseKey(rawKey string) (InstallLicenseResult, error) {
	return installLicenseKey(rawKey, false)
}

// installLicenseKey is InstallLicenseKey with an internal switch: skipReceiptRefresh lets
// callers that already hold a fresh receipt (online activation, online refresh) avoid a
// redundant ops-backend round trip and re-entrancy.
func installLicenseKey(rawKey string, skipReceiptRefresh bool) (InstallLicenseResult, error) {
	parsed, err := common.ParseLicenseKey(rawKey)
	if err != nil {
		return InstallLicenseResult{}, NewLicenseError(define.LicenseStateInvalid, err)
	}
	deviceCode := CurrentDeviceCode()
	if deviceCode == `` {
		return InstallLicenseResult{}, ErrLicenseConfigError
	}
	state := common.EvaluateLicenseClaims(&parsed.Claims, time.Now().Unix(), deviceCode)
	switch state {
	case define.LicenseStateValid, define.LicenseStateExpiring:
		// installable
	case define.LicenseStateExpired:
		return InstallLicenseResult{}, ErrLicenseExpired
	case define.LicenseStateDeviceMismatch:
		return InstallLicenseResult{}, ErrLicenseDeviceMismatch
	case define.LicenseStateNotYetValid:
		return InstallLicenseResult{}, ErrLicenseNotYetValid
	case define.LicenseStateClockInvalid:
		return InstallLicenseResult{}, ErrLicenseClockInvalid
	default:
		return InstallLicenseResult{}, NewLicenseError(define.LicenseStateInvalid, nil)
	}
	effective := effectivePlanFromClaims(parsed)

	rootIds, err := findRootTenantIds()
	if err != nil {
		return InstallLicenseResult{}, err
	}
	if len(rootIds) != 1 {
		return InstallLicenseResult{}, ErrLicenseRootTenantMissing
	}
	rootAdminId := rootIds[0]

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return InstallLicenseResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Ensure the singleton row exists, then lock it for the revision check.
	if _, err = msql.RawValues(define.Postgres, `INSERT INTO commercial_licenses (id, created_at, updated_at)
		VALUES (1, $1, $1) ON CONFLICT (id) DO NOTHING`, tx, tool.Time2Int()); err != nil {
		logs.Error(`ensure license singleton failed: %s`, err.Error())
		return InstallLicenseResult{}, err
	}
	currentRows, err := msql.RawValues(define.Postgres, `SELECT license_key, revision, claims_hash
		FROM commercial_licenses WHERE id=1 FOR UPDATE`, tx)
	if err != nil {
		logs.Error(`lock license singleton failed: %s`, err.Error())
		return InstallLicenseResult{}, err
	}
	now := time.Now().Unix()
	if len(currentRows) > 0 && currentRows[0][`license_key`] != `` {
		currentRevision := cast.ToInt(currentRows[0][`revision`])
		currentHash := currentRows[0][`claims_hash`]
		if parsed.Claims.Revision < currentRevision {
			return InstallLicenseResult{}, fmt.Errorf(`%w: incoming %d < current %d`, ErrLicenseRevisionRegressed, parsed.Claims.Revision, currentRevision)
		}
		if parsed.Claims.Revision == currentRevision && currentHash == parsed.ClaimsHash {
			// Same key replay: idempotent success without rewriting the row. Commit releases the
			// row lock first; trial receipt refresh updates the same row on another connection.
			if err = tx.Commit(); err != nil {
				logs.Error(`commit license replay check failed: %s`, err.Error())
				return InstallLicenseResult{}, err
			}
			if err = reloadLicenseSnapshot(); err != nil {
				logs.Error(`refresh license snapshot after replay failed: %s`, err.Error())
			}
			finalizeTrialReceipt(parsed, skipReceiptRefresh)
			reportLicenseUsageAfterInstall(parsed)
			syncLicensedEnterpriseName(rootAdminId, parsed.Claims.CorpName)
			return installResultFromEffective(effective, GetLicenseSnapshot().State), nil
		}
	}
	if _, err = msql.RawValues(define.Postgres, `UPDATE commercial_licenses SET
		license_key=$2, license_no=$3, activation_mode=$4, corp_name=$5, device_code=$6,
		issued_at=$7, not_before=$8, expire_at=$9, revision=$10, claims_hash=$11, updated_at=$12
		WHERE id=$1`, tx,
		1, parsed.Raw, parsed.Claims.LicenseNo, parsed.Claims.ActivationMode, parsed.Claims.CorpName,
		parsed.Claims.DeviceCode, parsed.Claims.IssuedAt, parsed.Claims.NotBefore, parsed.Claims.ExpireAt,
		parsed.Claims.Revision, parsed.ClaimsHash, now); err != nil {
		logs.Error(`update license singleton failed: %s`, err.Error())
		return InstallLicenseResult{}, err
	}

	// Project expiry and quotas onto the root tenant plan. Read-only compatibility for legacy
	// display: authorization decisions keep reading the verified claims.
	if _, err = msql.RawValues(define.Postgres, `UPDATE tenant_plans SET
		plan_type=$2, expire_at=$3, member_limit=$4, project_limit=$5,
		custom_field_per_project_limit=$6, automation_rule_per_project_limit=$7,
		automation_exec_limit=$8, agent_limit=$9, storage_limit=$10, updated_at=$11
		WHERE admin_id=$1`, tx,
		rootAdminId, define.PlanEnterprise, int(parsed.Claims.ExpireAt),
		int(parsed.Claims.Quotas.MemberLimit), int(parsed.Claims.Quotas.ProjectLimit),
		int(parsed.Claims.Quotas.CustomFieldPerProjectLimit), int(parsed.Claims.Quotas.AutomationRulePerProjectLimit),
		int(parsed.Claims.Quotas.AutomationExecLimit), int(parsed.Claims.Quotas.AgentLimit),
		int(parsed.Claims.Quotas.StorageLimit), now); err != nil {
		logs.Error(`project license onto tenant plan failed: admin_id=%d, err=%s`, rootAdminId, err.Error())
		return InstallLicenseResult{}, err
	}

	if err = tx.Commit(); err != nil {
		logs.Error(`commit license install failed: %s`, err.Error())
		return InstallLicenseResult{}, err
	}

	// Swap the snapshot only after a successful commit.
	if err = reloadLicenseSnapshot(); err != nil {
		logs.Error(`refresh license snapshot after install failed: %s`, err.Error())
	}
	finalizeTrialReceipt(parsed, skipReceiptRefresh)
	reportLicenseUsageAfterInstall(parsed)
	syncLicensedEnterpriseName(rootAdminId, parsed.Claims.CorpName)
	// Trial keys are gated on receipt freshness, so the claim-derived state can be misleading;
	// report the post-gate snapshot state instead.
	return installResultFromEffective(effective, GetLicenseSnapshot().State), nil
}

// syncLicensedEnterpriseName 让登录后的企业名称跟这次安装的授权企业一致。
// 启动时创建的默认企业名不能覆盖客户刚填写的公司。
func syncLicensedEnterpriseName(adminId int, corpName string) {
	corpName = strings.TrimSpace(corpName)
	if adminId <= 0 || corpName == `` {
		return
	}
	if _, err := UpdateEnterpriseName(UpdateEnterpriseNameModel{
		AdminId: adminId, UserId: adminId, EnterpriseName: corpName,
	}); err != nil {
		logs.Error(`sync licensed enterprise name failed: admin_id=%d, err=%s`, adminId, err.Error())
	}
}

func installResultFromEffective(effective EffectivePlan, state string) InstallLicenseResult {
	return InstallLicenseResult{
		LicenseNo:      effective.LicenseNo,
		CorpName:       effective.CorpName,
		ActivationMode: effective.ActivationMode,
		Revision:       effective.Revision,
		ExpireAt:       effective.ExpireAt,
		State:          state,
		Quotas:         effective.Quotas,
	}
}

// licenseQuotasIfValid returns the verified claim quotas when the license allows business.
// Callers keep reading their local projection first and use this to override the effective
// limit, so upstream query code stays untouched and merge-friendly.
func licenseQuotasIfValid() (common.LicenseQuotaClaims, bool) {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil || !define.LicenseStateCanLogin(snapshot.State) {
		return common.LicenseQuotaClaims{}, false
	}
	return snapshot.Effective.Quotas, true
}

// applyLicensePlanOverride rewrites a table-projected plan with the verified claim expiry and
// quotas. storage_over_deadline is local tenant state and is intentionally preserved.
func applyLicensePlanOverride(plan *PlanModel) {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil {
		return
	}
	plan.PlanType = define.PlanEnterprise
	plan.ExpireAt = int(snapshot.Effective.ExpireAt)
	plan.MemberLimit = int(snapshot.Effective.Quotas.MemberLimit)
	plan.ProjectLimit = int(snapshot.Effective.Quotas.ProjectLimit)
	plan.CustomFieldPerProjectLimit = int(snapshot.Effective.Quotas.CustomFieldPerProjectLimit)
	plan.AutomationRulePerProjectLimit = int(snapshot.Effective.Quotas.AutomationRulePerProjectLimit)
	plan.AutomationExecLimit = int(snapshot.Effective.Quotas.AutomationExecLimit)
	plan.AgentLimit = int(snapshot.Effective.Quotas.AgentLimit)
	plan.StorageLimit = int(snapshot.Effective.Quotas.StorageLimit)
}

// GetEffectivePlan returns the plan derived from verified license claims, supplemented with
// local-only tenant state (storage_over_deadline). It never falls back to tenant_plans quotas
// and never auto-creates plans: bootstrap owns initialization.
func GetEffectivePlan(adminId int) (PlanModel, error) {
	if adminId <= 0 {
		return PlanModel{}, ErrPlanNotFound
	}
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil || !define.LicenseStateCanLogin(snapshot.State) {
		return PlanModel{}, ErrLicenseNotActivated
	}
	claims := snapshot.Effective.Claims
	plan := PlanModel{
		AdminId:                       adminId,
		PlanType:                      define.PlanEnterprise,
		ExpireAt:                      int(claims.ExpireAt),
		IsExpired:                     snapshot.State == define.LicenseStateExpired,
		MemberLimit:                   int(claims.Quotas.MemberLimit),
		ProjectLimit:                  int(claims.Quotas.ProjectLimit),
		CustomFieldPerProjectLimit:    int(claims.Quotas.CustomFieldPerProjectLimit),
		AutomationRulePerProjectLimit: int(claims.Quotas.AutomationRulePerProjectLimit),
		AutomationExecLimit:           int(claims.Quotas.AutomationExecLimit),
		AgentLimit:                    int(claims.Quotas.AgentLimit),
		StorageLimit:                  int(claims.Quotas.StorageLimit),
	}
	// Local storage-grace state lives in tenant_plans and is the only value read back from it.
	rows, err := msql.RawValues(define.Postgres, `SELECT storage_over_deadline FROM tenant_plans WHERE admin_id=$1`, nil, adminId)
	if err != nil {
		logs.Error(`read storage deadline failed: %s`, err.Error())
		return PlanModel{}, err
	}
	if len(rows) > 0 {
		plan.StorageOverDeadline = cast.ToInt(rows[0][`storage_over_deadline`])
	}
	return plan, nil
}

// HasFeature reports whether the verified claims grant the product feature code.
// RBAC permission keys are a separate layer and must be checked independently.
func HasFeature(feature string) bool {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil || !define.LicenseStateCanLogin(snapshot.State) {
		return false
	}
	for _, item := range snapshot.Effective.Claims.Features {
		if item == feature {
			return true
		}
	}
	return false
}
