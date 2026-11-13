// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// License online subsystem: verification-code forwarding, online activation, the 6-hour
// signed-receipt re-verification loop and the trial mandatory-online gate.
//
// All operations-backend calls are outbound-only, form-encoded and timeout-bounded; responses
// are capped in size and the full license key or receipts are never logged. The signed receipt
// is the only trust anchor for trial availability between re-verifications (24h max window).

// Ops endpoint paths on the operations backend (controller kf/goTeams).
const (
	opsPathSendVerifyCode = `/kf/goTeams/sendPrivateLicenseVerifyCode`
	opsPathActivate       = `/kf/goTeams/activatePrivateLicense`
	opsPathRefresh        = `/kf/goTeams/refreshPrivateLicense`
)

// LicenseOpsError carries a user-presentable online-license failure. Errors raised by the
// operations backend carry Msg, which the backend contract limits to curated terminal-facing
// copy (never OpenSSL/SQL/mail-gateway internals), so the controller passes it through as-is.
// Errors raised locally carry Key, an i18n message key translated by the controller.
type LicenseOpsError struct {
	Msg string
	Key string
}

// Error implements the error interface.
func (e *LicenseOpsError) Error() string {
	msg := e.Msg
	if msg == `` {
		msg = e.Key
	}
	return `ops backend: ` + msg
}

type opsResponse struct {
	Res  int            `json:"res"`
	Msg  string         `json:"msg"`
	Data map[string]any `json:"data"`
}

// opsBaseURL returns the configured operations backend origin without trailing slash.
func opsBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(define.LicenseConfig[`activation_url`]), `/`)
}

// postOpsForm sends a bounded, timeout-bounded form POST to the operations backend and decodes
// the unified {res,msg,data} response. Failures never leak request payloads into logs.
func postOpsForm(path string, form url.Values) (*opsResponse, error) {
	base := opsBaseURL()
	if base == `` {
		return nil, ErrLicenseConfigError
	}
	timeout := cast.ToInt(define.LicenseConfig[`request_timeout_seconds`])
	if timeout <= 0 {
		timeout = define.LicenseDefaultRequestTimeoutSeconds
	}
	reqBody := []byte(form.Encode())
	// The backend digests the raw bytes it receives, so the signed copy and the sent copy must be
	// the same encoding; a second form.Encode() call could reorder nothing today but is a trap.
	sign, err := common.BuildOpsRequestSign(path, reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set(`Content-Type`, `application/x-www-form-urlencoded`)
	req.Header.Set(define.LicenseRequestSignHeader, sign)
	// A named UA keeps license traffic identifiable on the operations backend; the Go default
	// UA is a common WAF block target and returns HTML challenge pages instead of JSON.
	req.Header.Set(`User-Agent`, `goteams-license/`+define.BuildVersion)
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}
	if path == opsPathReportUsage && (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) {
		return nil, fmt.Errorf("usage report HTTP status %d", resp.StatusCode)
	}
	var decoded opsResponse
	if err = json.Unmarshal(body, &decoded); err != nil {
		if path == opsPathReportUsage {
			// 错误代理页可能回显请求，报告流不记录返回正文以免泄漏Key。
			logs.Error(`ops usage response not json: status=%d`, resp.StatusCode)
		} else {
			logs.Error(`ops response not json: path=%s, status=%d, content_type=%s, body=%q`,
				path, resp.StatusCode, resp.Header.Get(`Content-Type`), opsBodySnippet(body))
		}
		// Plain error on purpose: an unparseable response (WAF challenge, gateway page) is not
		// curated terminal copy, so the controller falls back to the generic availability message.
		return nil, errors.New(`ops response not json`)
	}
	return &decoded, nil
}

// opsBodySnippet flattens an unexpected response body into a single-line bounded snippet for
// logging; it contains only peer-returned bytes and never any request payload.
func opsBodySnippet(body []byte) string {
	const maxSnippet = 200
	if len(body) > maxSnippet {
		body = body[:maxSnippet]
	}
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if unicode.IsPrint(r) {
			return r
		}
		return '?'
	}, string(body))
}

// opsDataText serializes an ops response data map into one compact single-line string for
// logging, dropping credential-bearing fields (the full license key and signed receipt);
// remaining fields (license_no, expire_at, revision, limits) are safe operational metadata.
func opsDataText(data map[string]any) string {
	redacted := make(map[string]any, len(data))
	for key, value := range data {
		if key == `license_key` || key == `receipt` {
			continue
		}
		redacted[key] = value
	}
	if raw, err := json.Marshal(redacted); err == nil {
		return string(raw)
	}
	return fmt.Sprintf(`%v`, redacted)
}

func opsErrorMessage(resp *opsResponse, err error) error {
	if err != nil {
		return err
	}
	if resp.Res != 0 {
		return &LicenseOpsError{Msg: resp.Msg}
	}
	return nil
}

// ======================== Verification code forwarding ========================

// licenseVerifyResendInterval is the first-layer resend frequency guard (seconds), matching
// the operations backend rule so obvious hammering never reaches the mail gateway.
const licenseVerifyResendInterval = int64(60)

var (
	licenseVerifyLimiterMu sync.Mutex
	licenseVerifyLimiter   = make(map[string]int64)
)

// tryAcquireVerifyQuota allows one forwarded request per email per interval.
func tryAcquireVerifyQuota(email string) bool {
	now := time.Now().Unix()
	licenseVerifyLimiterMu.Lock()
	defer licenseVerifyLimiterMu.Unlock()
	if len(licenseVerifyLimiter) > 4096 {
		for key, last := range licenseVerifyLimiter {
			if now-last > 3600 {
				delete(licenseVerifyLimiter, key)
			}
		}
	}
	if last, ok := licenseVerifyLimiter[email]; ok && now-last < licenseVerifyResendInterval {
		return false
	}
	licenseVerifyLimiter[email] = now
	return true
}

// ForwardTrialVerifyCode relays the online-trial email verification-code request to the
// operations backend after local format and frequency checks.
func ForwardTrialVerifyCode(email string) error {
	if !tryAcquireVerifyQuota(email) {
		return &LicenseOpsError{Key: define.MsgLicenseVerifyRateLimited}
	}
	form := url.Values{}
	form.Set(`email`, email)
	form.Set(`device_code`, CurrentDeviceCode())
	form.Set(`build_version`, define.BuildVersion)
	resp, err := postOpsForm(opsPathSendVerifyCode, form)
	if err = opsErrorMessage(resp, err); err != nil {
		logs.Error(`forward trial verify code failed: email fingerprint=%s, err=%s`, common.LicenseKeyFingerprint(email), err.Error())
		return err
	}
	logs.Info(`trial verify code forwarded: email fingerprint=%s`, common.LicenseKeyFingerprint(email))
	return nil
}

// ======================== Online activation ========================

// OnlineActivateModel carries the already-validated trial activation input.
type OnlineActivateModel struct {
	CorpName   string
	Phone      string
	Email      string
	VerifyCode string
	RequestId  string
}

// OnlineActivateResult is the verified outcome of an online trial activation.
type OnlineActivateResult struct {
	Install           InstallLicenseResult
	FirstActivation   bool
	BootstrapUsername string
	BootstrapPassword string
}

// ActivateLicenseOnline requests the trial license from the operations backend, then verifies
// and installs it locally like any other key. The operations backend owns trial uniqueness
// (HMAC identity table) and the default expiry; the terminal only trusts the signed key.
func ActivateLicenseOnline(model OnlineActivateModel) (OnlineActivateResult, error) {
	result := OnlineActivateResult{}
	deviceCode := CurrentDeviceCode()
	if deviceCode == `` {
		return result, ErrLicenseConfigError
	}
	prior, err := loadLicenseRecord()
	if err != nil {
		return result, err
	}
	firstActivation := prior == nil || prior.LicenseKey == ``

	form := url.Values{}
	form.Set(`corp_name`, model.CorpName)
	form.Set(`phone`, model.Phone)
	form.Set(`email`, model.Email)
	form.Set(`verify_code`, model.VerifyCode)
	form.Set(`request_id`, model.RequestId)
	form.Set(`device_code`, deviceCode)
	form.Set(`build_version`, define.BuildVersion)
	fingerprintForm(form, CurrentDeviceFingerprints())

	requestURL := opsBaseURL() + opsPathActivate
	resp, err := postOpsForm(opsPathActivate, form)
	if resp != nil {
		logs.Info(`ops activate response: url=%s, res=%d, msg=%s, data=%s`,
			requestURL, resp.Res, resp.Msg, opsDataText(resp.Data))
	}
	if err = opsErrorMessage(resp, err); err != nil {
		logs.Error(`online activation rejected: device=%s, err=%s`, deviceCode, err.Error())
		return result, err
	}
	rawKey := cast.ToString(resp.Data[`license_key`])
	if err = validate.ValidateLicenseKeyInput(rawKey); err != nil {
		return result, fmt.Errorf(`ops activation response missing license key`)
	}
	installed, err := installLicenseKey(rawKey, true)
	if err != nil {
		return result, err
	}
	// The activation response already carries a receipt, saved below; no extra ops round trip.
	if receiptText := cast.ToString(resp.Data[`receipt`]); receiptText != `` {
		if err = SaveLicenseReceipt(receiptText); err != nil {
			return result, err
		}
	}
	result.Install = installed
	result.FirstActivation = firstActivation
	if firstActivation {
		// The default administrator was created by bootstrap from the [account] section;
		// surfacing it once here is the documented first-activation experience.
		result.BootstrapUsername = strings.ToLower(strings.TrimSpace(define.AccountConfig[`admin_username`]))
		result.BootstrapPassword = define.AccountConfig[`admin_password`]
	}
	logs.Info(`online activation installed: device=%s, license_no=%s, revision=%d, first=%v`,
		deviceCode, installed.LicenseNo, installed.Revision, firstActivation)
	return result, nil
}

// fingerprintForm copies the salted hardware digests into top-level form fields.
func fingerprintForm(form url.Values, prints common.DeviceFingerprints) {
	if prints.MachineIDHash != `` {
		form.Set(`machine_id_hash`, prints.MachineIDHash)
	}
	if prints.ProductUUIDHash != `` {
		form.Set(`product_uuid_hash`, prints.ProductUUIDHash)
	}
	if prints.BoardSerialHash != `` {
		form.Set(`board_serial_hash`, prints.BoardSerialHash)
	}
	if prints.Combined != `` {
		form.Set(`hardware_fingerprint_hash`, prints.Combined)
	}
}

// ======================== Receipt persistence and trial gate ========================

// SaveLicenseReceipt verifies and persists a fresh online-check receipt for the singleton row.
func SaveLicenseReceipt(receiptText string) error {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil {
		return ErrLicenseNotActivated
	}
	parsedReceipt, err := common.ParseLicenseReceipt(receiptText)
	if err != nil {
		logs.Error(`receipt verify failed: fingerprint=%s, err=%s`, licenseFingerprintOf(receiptText), err.Error())
		return NewLicenseError(define.LicenseStateInvalid, err)
	}
	if blocked := common.EvaluateLicenseReceipt(parsedReceipt, time.Now().Unix(),
		snapshot.Parsed.Claims.LicenseNo, snapshot.Parsed.Claims.DeviceCode); blocked != `` {
		return NewLicenseError(blocked, nil)
	}
	now := time.Now().Unix()
	if _, err = msql.RawValues(define.Postgres, `UPDATE commercial_licenses SET
		online_check_receipt=$2, last_online_sync_at=$3, updated_at=$3 WHERE id=$1`, nil,
		1, parsedReceipt.Raw, now); err != nil {
		logs.Error(`save license receipt failed: %s`, err.Error())
		return err
	}
	logs.Info(`license receipt saved: fingerprint=%s, valid_until=%d`, parsedReceipt.Fingerprint, parsedReceipt.Claims.ValidUntil)
	return reloadLicenseSnapshot()
}

// finalizeTrialReceipt fetches a fresh receipt right after a trial key install, so the
// mandatory-online gate opens immediately instead of waiting for the next periodic
// re-verification. Failure is logged only: the install itself stays successful and the
// startup check or periodic loop retries later.
func finalizeTrialReceipt(parsed *common.ParsedLicenseKey, skip bool) {
	if skip || parsed == nil || parsed.Claims.LicenseKind != define.LicenseKindTrial {
		return
	}
	if err := RefreshLicenseOnline(); err != nil {
		logs.Error(`trial receipt refresh after install failed: license_no=%s, err=%s`,
			parsed.Claims.LicenseNo, err.Error())
	}
}

// gateTrialLicenseState enforces the trial mandatory-online rule: a trial key only counts as
// valid/expiring while a fresh signed receipt exists. Paid and offline licenses pass through.
func gateTrialLicenseState(parsed *common.ParsedLicenseKey, receiptText, state string) string {
	if parsed == nil {
		return state
	}
	if state != define.LicenseStateValid && state != define.LicenseStateExpiring {
		return state
	}
	if parsed.Claims.LicenseKind != define.LicenseKindTrial {
		return state
	}
	receipt, err := common.ParseLicenseReceipt(receiptText)
	if err != nil {
		// No usable receipt (missing, tampered or expired): the trial stays blocked until the
		// next successful online re-verification. Database time fields cannot extend it.
		logs.Error(`trial receipt unavailable: license_no=%s, fingerprint=%s, err=%s`,
			parsed.Claims.LicenseNo, licenseFingerprintOf(receiptText), err.Error())
		return define.LicenseStateExpired
	}
	if blocked := common.EvaluateLicenseReceipt(receipt, time.Now().Unix(),
		parsed.Claims.LicenseNo, parsed.Claims.DeviceCode); blocked != `` {
		return blocked
	}
	return state
}

// ======================== Online refresh ========================

// RefreshLicenseOnline re-verifies the current key against the operations backend. It accepts
// a fresh signed receipt, installs any higher-revision key the backend returns (enabling
// auto-recovery after operator renewal) and never downgrades a locally valid key.
func RefreshLicenseOnline() error {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil || snapshot.DeviceCode == `` {
		return ErrLicenseNotActivated
	}
	form := url.Values{}
	form.Set(`license_key`, snapshot.Parsed.Raw)
	form.Set(`device_code`, snapshot.DeviceCode)
	form.Set(`build_version`, define.BuildVersion)
	resp, err := postOpsForm(opsPathRefresh, form)
	if err = opsErrorMessage(resp, err); err != nil {
		return err
	}
	// A newer key (renewal, quota change) is installed first; the revision guard rejects rollbacks.
	// skipReceiptRefresh: this flow saves the receipt below itself, so a re-entrant trial
	// refresh right after install would duplicate the ops request.
	if newKey := cast.ToString(resp.Data[`license_key`]); newKey != `` && newKey != snapshot.Parsed.Raw {
		if _, err = installLicenseKey(newKey, true); err != nil {
			logs.Error(`refresh returned key install failed: license_no=%s, err=%s`,
				cast.ToString(resp.Data[`license_no`]), err.Error())
			return err
		}
	}
	if receiptText := cast.ToString(resp.Data[`receipt`]); receiptText != `` {
		return SaveLicenseReceipt(receiptText)
	}
	return reloadLicenseSnapshot()
}

// runTrialStartupOnlineCheck implements the trial mandatory startup verification: every process
// start must succeed one online check before login and business APIs open, even when a receipt
// is still locally valid.
func runTrialStartupOnlineCheck() {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil || snapshot.Parsed.Claims.LicenseKind != define.LicenseKindTrial {
		return
	}
	if !define.LicenseStateCanLogin(snapshot.State) {
		return
	}
	if err := RefreshLicenseOnline(); err != nil {
		logs.Error(`trial startup online check failed, blocking business: %s`, err.Error())
		if errors.Is(err, ErrLicenseConfigError) {
			setLicenseSnapshot(&LicenseSnapshot{
				State: define.LicenseStateConfigError, DeviceCode: snapshot.DeviceCode,
				Parsed: snapshot.Parsed, Effective: snapshot.Effective,
			})
			return
		}
		setLicenseSnapshot(&LicenseSnapshot{
			State: define.LicenseStateExpired, DeviceCode: snapshot.DeviceCode,
			Parsed: snapshot.Parsed, Effective: snapshot.Effective,
		})
	}
}

// ======================== Periodic re-verification loop ========================

var licenseRefreshCancel context.CancelFunc

// StartLicenseRefreshLoop launches the 6-hour (plus jitter) online re-verification loop.
// Online licenses re-verify periodically; offline keys never touch the network. The loop
// exits on StopLicenseRefreshLoop and never retries in a tight cycle.
func StartLicenseRefreshLoop() {
	interval := cast.ToInt64(define.LicenseConfig[`refresh_interval_seconds`])
	if interval <= 0 {
		interval = define.LicenseDefaultRefreshIntervalSeconds
	}
	ctx, cancel := context.WithCancel(context.Background())
	licenseRefreshCancel = cancel
	go func() {
		// Random jitter avoids synchronized re-verification storms across customer instances.
		jitter := time.Duration(rand.Int63n(interval/10+1)) * time.Second
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter):
		}
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				licenseRefreshTick()
			}
		}
	}()
	logs.Info(`license refresh loop started: interval=%ds`, interval)
}

// StopLicenseRefreshLoop cancels the periodic re-verification loop for graceful shutdown.
func StopLicenseRefreshLoop() {
	if licenseRefreshCancel != nil {
		licenseRefreshCancel()
		licenseRefreshCancel = nil
	}
}

func licenseRefreshTick() {
	snapshot := GetLicenseSnapshot()
	if snapshot.Parsed == nil {
		return
	}
	if snapshot.Effective.ActivationMode != define.LicenseModeOnline {
		return
	}
	if err := RefreshLicenseOnline(); err != nil {
		// Non-fatal: the trial gate already blocks business while no fresh receipt exists,
		// and paid keys stay valid by design during network failures.
		logs.Error(`license online refresh failed: license_no=%s, err=%s`, snapshot.Effective.LicenseNo, err.Error())
	}
}
