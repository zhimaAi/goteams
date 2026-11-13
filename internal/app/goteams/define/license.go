// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// BuildEdition is the fixed commercial edition marker. It is a compile-time constant on purpose:
// this branch only ships the commercial build, so the edition can never be switched through INI,
// environment variables or -ldflags, and no cloud runtime branch exists here.
const BuildEdition = `commercial`

// BuildVersion and BuildCommit are injected by the commercial packaging script via -ldflags -X.
// v0.0.0-dev is only allowed for local development startup; packaging and the upgrade tool reject it.
var (
	BuildVersion = `v0.0.0-dev`
	BuildCommit  = `unknown`
)

// AccountConfig stores the [account] section: commercial default administrator bootstrap settings.
var AccountConfig map[string]string

// LicenseConfig stores the [license] section: commercial license runtime settings.
var LicenseConfig map[string]string

// TableCommercialLicenses is the instance-level signed license singleton table (id=1 only).
// Exception to multi-tenancy: installation metadata owned by the binary, never tenant data —
// rows must not be filtered by admin_id/workspace_id and must not be reused for business data.
const TableCommercialLicenses = `commercial_licenses`

// License edition constants are reserved for cross-checking packaged artifacts only.
const (
	EditionCommercial = `commercial`
)

// License lifecycle states (single source of truth for the terminal state machine).
const (
	LicenseStateNotActivated   = `not_activated`
	LicenseStateValid          = `valid`
	LicenseStateExpiring       = `expiring`
	LicenseStateExpired        = `expired`
	LicenseStateInvalid        = `invalid_signature`
	LicenseStateDeviceMismatch = `device_mismatch`
	LicenseStateNotYetValid    = `not_yet_valid`
	LicenseStateClockInvalid   = `clock_invalid`
	LicenseStateConfigError    = `config_error`
)

// Fixed response reasons carried in data.reason for every blocked request.
const (
	LicenseReasonNotActivated   = `license_not_activated`
	LicenseReasonExpired        = `license_expired`
	LicenseReasonInvalid        = `license_invalid`
	LicenseReasonDeviceMismatch = `license_device_mismatch`
	LicenseReasonClockInvalid   = `license_clock_invalid`
	LicenseReasonConfigError    = `license_config_error`
)

// License Key protocol constants (GTLIC1 compact JWS style, ES256 / ECDSA P-256 + SHA-256).
const (
	LicenseKeyPrefix     = `GTLIC1`
	LicenseKeyTyp        = `GTLIC`
	LicenseKeyAlg        = `ES256`
	LicenseIssuer        = `xiaokefu-goteams-license`
	LicenseAudience      = `goteams-commercial`
	LicenseSchemaVersion = 1
	// LicenseDeviceCodePrefix is the display prefix of the terminal device code.
	LicenseDeviceCodePrefix = `GTDEV1`
)

// License activation modes, kinds and signed statuses.
const (
	LicenseModeOnline  = `online`
	LicenseModeOffline = `offline`

	LicenseKindTrial = `trial`
	LicenseKindPaid  = `paid`

	LicenseStatusActive  = `active`
	LicenseStatusRevoked = `revoked`
)

// License claim feature codes ("module purchased" grants; RBAC permission keys are a separate layer).
const (
	LicenseFeatureKnowledgeBase = `kb`
	LicenseFeaturePipeline      = `pipeline`
)

// LicenseKnownFeatures is the whitelist of accepted claim feature codes; unknown values reject the key.
var LicenseKnownFeatures = []string{LicenseFeatureKnowledgeBase, LicenseFeaturePipeline}

// License time and trial rules.
const (
	// LicenseExpiringSeconds is the remaining-time threshold that surfaces the expiring banner (15 days).
	LicenseExpiringSeconds = int64(15 * 24 * 60 * 60)
	// LicenseClockSkewTolerance allows a small positive skew between local clock and issued_at.
	LicenseClockSkewTolerance = int64(600)
	// LicenseKeyMaxBytes caps the raw license key length to reject oversized inputs.
	LicenseKeyMaxBytes = 4096
)

// License quota claim upper bounds; every quota must be 1..max, negatives and magic "unlimited" values reject the key.
const (
	LicenseMaxMemberLimit     = 100000
	LicenseMaxProjectLimit    = 100000
	LicenseMaxPerProjectLimit = 10000
	LicenseMaxAutomationExec  = 100000000
	LicenseMaxAgentLimit      = 100000
	LicenseMaxStorageLimit    = 100000
	LicenseMaxRevision        = 1000000
)

// License i18n message keys.
const (
	MsgLicenseNotActivated      = `license_not_activated`
	MsgLicenseExpired           = `license_expired`
	MsgLicenseInvalid           = `license_invalid`
	MsgLicenseDeviceMismatch    = `license_device_mismatch`
	MsgLicenseClockInvalid      = `license_clock_invalid`
	MsgLicenseConfigError       = `license_config_error`
	MsgLicenseAlreadyActive     = `license_already_active`
	MsgLicenseKeyInvalid        = `license_key_invalid`
	MsgLicenseUpdateSuccess     = `license_update_success`
	MsgLicenseActivateSuccess   = `license_activate_success`
	MsgLicenseOwnerRequired     = `license_owner_required`
	MsgLicenseVerifyCodeSent    = `license_verify_code_sent`
	MsgLicenseOnlineFailed      = `license_online_failed`
	MsgLicenseVerifyRateLimited = `license_verify_rate_limited`
	MsgLicenseRequestInvalid    = `license_request_invalid`
	MsgLicenseKeyFormat         = `license_key_format`
	MsgLicenseKeyEnvMismatch    = `license_key_env_mismatch`
	MsgLicenseKeySignature      = `license_key_signature`
	MsgLicenseClaimsInvalid     = `license_claims_invalid`
)

// LicenseReasonForState maps a license state to its fixed response reason.
// expiring is a display sub-state of valid and never blocks requests.
func LicenseReasonForState(state string) string {
	switch state {
	case LicenseStateNotActivated:
		return LicenseReasonNotActivated
	case LicenseStateExpired:
		return LicenseReasonExpired
	case LicenseStateInvalid:
		return LicenseReasonInvalid
	case LicenseStateDeviceMismatch:
		return LicenseReasonDeviceMismatch
	case LicenseStateClockInvalid:
		return LicenseReasonClockInvalid
	case LicenseStateConfigError:
		return LicenseReasonConfigError
	default:
		// valid/expiring do not produce blocking reasons.
		return ``
	}
}

// LicenseMsgForState maps a license state to its i18n message key.
func LicenseMsgForState(state string) string {
	switch state {
	case LicenseStateExpired:
		return MsgLicenseExpired
	case LicenseStateInvalid:
		return MsgLicenseInvalid
	case LicenseStateDeviceMismatch:
		return MsgLicenseDeviceMismatch
	case LicenseStateClockInvalid:
		return MsgLicenseClockInvalid
	case LicenseStateConfigError:
		return MsgLicenseConfigError
	default:
		return MsgLicenseNotActivated
	}
}

// LicenseStateCanLogin reports whether normal licensed business is available.
func LicenseStateCanLogin(state string) bool {
	return state == LicenseStateValid || state == LicenseStateExpiring
}

// LicenseStateAllowsManagementLogin reports whether an installed but blocked license may
// issue a session restricted to the enterprise version page and owner renewal endpoints.
func LicenseStateAllowsManagementLogin(state string) bool {
	switch state {
	case LicenseStateExpired, LicenseStateInvalid, LicenseStateDeviceMismatch,
		LicenseStateNotYetValid, LicenseStateClockInvalid, LicenseStateConfigError:
		return true
	default:
		return LicenseStateCanLogin(state)
	}
}

// License receipt protocol (GTRECEIPT1 short-term online-check receipt, ES256 signed by the
// same key pair as license keys). Only trial licenses are gated by receipts.
const (
	LicenseReceiptPrefix = `GTRECEIPT1`
	LicenseReceiptTyp    = `GTRECEIPT`
	// LicenseReceiptMaxSeconds caps the receipt validity window issued by the operations backend (24 hours).
	LicenseReceiptMaxSeconds = int64(24 * 60 * 60)
)

// License online subsystem defaults, overridden by the [license] INI section.
const (
	LicenseDefaultRequestTimeoutSeconds  = 10
	LicenseDefaultRefreshIntervalSeconds = 21600
)

// Request sign protocol (GTREQ1 outbound ops-backend request signature, ES256).
//
// This key pair is deliberately separate from the license signing pair: its private key is
// compiled into the terminal and is extractable by design, so it only raises the cost of
// anonymous abuse and must never be trusted as a sole authorization boundary. Reusing the
// license signing key here would let a terminal-side leak mint licenses.
const (
	LicenseRequestSignPrefix = `GTREQ1`
	LicenseRequestSignTyp    = `GTREQ`
	LicenseRequestSignAlg    = `ES256`
	// LicenseRequestSignHeader carries the signature string on every ops-backend request.
	LicenseRequestSignHeader = `X-GT-Request-Sign`
)
