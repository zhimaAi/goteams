// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"goteams/internal/app/goteams/define"
)

// licenseNoPattern is the strict format of the operations-issued license number.
var licenseNoPattern = regexp.MustCompile(`^GTL-[0-9]{8}-[A-Z0-9]{8}$`)

// LicenseClaims mirrors the signed claims JSON of a GTLIC1 commercial license key.
// Every signed field is mandatory: a missing field, wrong type, zero, negative or
// oversized value rejects the whole key. No default value may ever fill a quota.
type LicenseClaims struct {
	SchemaVersion  int                `json:"schema_version"`
	Issuer         string             `json:"issuer"`
	Audience       string             `json:"audience"`
	LicenseNo      string             `json:"license_no"`
	ActivationMode string             `json:"activation_mode"`
	LicenseKind    string             `json:"license_kind"`
	CorpName       string             `json:"corp_name"`
	DeviceCode     string             `json:"device_code"`
	IssuedAt       int64              `json:"issued_at"`
	NotBefore      int64              `json:"not_before"`
	ExpireAt       int64              `json:"expire_at"`
	TrialStartedAt int64              `json:"trial_started_at"`
	TrialEndAt     int64              `json:"trial_end_at"`
	Revision       int                `json:"revision"`
	Status         string             `json:"status"`
	Quotas         LicenseQuotaClaims `json:"quotas"`
	Features       []string           `json:"features"`
}

// LicenseQuotaClaims holds the seven mandatory quota claims of every key.
type LicenseQuotaClaims struct {
	MemberLimit                   int64 `json:"member_limit"`
	ProjectLimit                  int64 `json:"project_limit"`
	CustomFieldPerProjectLimit    int64 `json:"custom_field_per_project_limit"`
	AutomationRulePerProjectLimit int64 `json:"automation_rule_per_project_limit"`
	AutomationExecLimit           int64 `json:"automation_exec_limit"`
	AgentLimit                    int64 `json:"agent_limit"`
	StorageLimit                  int64 `json:"storage_limit"`
}

// LicenseKeyHeader mirrors the signed header JSON of a GTLIC1 key.
type LicenseKeyHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

// ParsedLicenseKey is the verified result of ParseLicenseKey.
type ParsedLicenseKey struct {
	Raw    string
	Header LicenseKeyHeader
	Claims LicenseClaims
	// ClaimsHash is the SHA-256 hex of the signed claims segment, stored as the audit projection.
	ClaimsHash string
	// Fingerprint is the first 12 hex chars of the full key SHA-256, safe for logs.
	Fingerprint string
}

// licensePubkeyCache caches parsed ECDSA public keys by KID so request paths never re-parse PEM.
var licensePubkeyCache sync.Map

// License verification errors (never returned to users; callers map them to fixed i18n states).
var (
	ErrLicenseKeyFormat     = errors.New(`license key format invalid`)
	ErrLicenseKeyTooLong    = errors.New(`license key too long`)
	ErrLicenseKeyAlg        = errors.New(`license key algorithm mismatch`)
	ErrLicenseKeyTyp        = errors.New(`license key type mismatch`)
	ErrLicenseKeyUnknownKID = errors.New(`license key unknown kid`)
	ErrLicenseKeySignature  = errors.New(`license key signature invalid`)
	ErrLicenseClaims        = errors.New(`license claims invalid`)
)

// ParseLicenseKey parses, signature-verifies and fully validates a GTLIC1 license key.
// It is a pure function: no database, no gin, no business dependency. Any failure blocks.
func ParseLicenseKey(rawKey string) (*ParsedLicenseKey, error) {
	key := strings.TrimSpace(rawKey)
	if key == `` {
		return nil, ErrLicenseKeyFormat
	}
	if len(key) > define.LicenseKeyMaxBytes {
		return nil, ErrLicenseKeyTooLong
	}
	parts := strings.Split(key, `.`)
	if len(parts) != 4 || parts[0] != define.LicenseKeyPrefix {
		return nil, ErrLicenseKeyFormat
	}

	// The signing input is the raw prefix + first two segments exactly as received;
	// re-serializing header or claims before verification is forbidden.
	signingInput := parts[0] + `.` + parts[1] + `.` + parts[2]

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrLicenseKeyFormat
	}
	var header LicenseKeyHeader
	if err = json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrLicenseKeyFormat
	}
	if header.Alg != define.LicenseKeyAlg {
		return nil, ErrLicenseKeyAlg
	}
	if header.Typ != define.LicenseKeyTyp {
		return nil, ErrLicenseKeyTyp
	}
	pubPem, ok := define.LicensePublicKeys[header.Kid]
	if !ok || strings.TrimSpace(header.Kid) == `` {
		return nil, ErrLicenseKeyUnknownKID
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrLicenseKeyFormat
	}
	var claims LicenseClaims
	decoder := json.NewDecoder(strings.NewReader(string(claimsBytes)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&claims); err != nil {
		return nil, ErrLicenseKeyFormat
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, ErrLicenseKeyFormat
	}
	pub, err := loadLicensePubkey(header.Kid, pubPem)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(signingInput))
	if !ecdsa.VerifyASN1(pub, digest[:], sigBytes) {
		return nil, ErrLicenseKeySignature
	}

	if err = validateLicenseClaims(&claims); err != nil {
		return nil, err
	}
	claimsHash := sha256.Sum256(claimsBytes)
	fullHash := sha256.Sum256([]byte(key))
	return &ParsedLicenseKey{
		Raw:         key,
		Header:      header,
		Claims:      claims,
		ClaimsHash:  hex.EncodeToString(claimsHash[:]),
		Fingerprint: hex.EncodeToString(fullHash[:])[:12],
	}, nil
}

// LicenseKeyFingerprint returns the first 12 hex chars of the full key SHA-256, safe for logs.
func LicenseKeyFingerprint(key string) string {
	fullHash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(fullHash[:])[:12]
}

// loadLicensePubkey parses and caches the ES256 public key for a KID.
func loadLicensePubkey(kid, pubPem string) (*ecdsa.PublicKey, error) {
	if cached, ok := licensePubkeyCache.Load(kid); ok {
		if pub, ok := cached.(*ecdsa.PublicKey); ok {
			return pub, nil
		}
	}
	block, _ := pem.Decode([]byte(pubPem))
	if block == nil {
		return nil, fmt.Errorf(`license public key %s pem decode failed`, kid)
	}
	generic, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf(`license public key %s parse failed: %w`, kid, err)
	}
	pub, ok := generic.(*ecdsa.PublicKey)
	if !ok || pub.Curve.Params().Name != `P-256` {
		return nil, fmt.Errorf(`license public key %s is not ECDSA P-256`, kid)
	}
	licensePubkeyCache.Store(kid, pub)
	return pub, nil
}

// validateLicenseClaims enforces every structural rule of section 6.1 of the license design.
// Time-dependent states (expired / not_yet_valid / clock_invalid) are evaluated separately.
func validateLicenseClaims(claims *LicenseClaims) error {
	if claims.SchemaVersion != define.LicenseSchemaVersion {
		return ErrLicenseClaims
	}
	if claims.Issuer != define.LicenseIssuer || claims.Audience != define.LicenseAudience {
		return ErrLicenseClaims
	}
	if claims.ActivationMode != define.LicenseModeOnline && claims.ActivationMode != define.LicenseModeOffline {
		return ErrLicenseClaims
	}
	if claims.LicenseKind != define.LicenseKindTrial && claims.LicenseKind != define.LicenseKindPaid {
		return ErrLicenseClaims
	}
	if claims.Status != define.LicenseStatusActive && claims.Status != define.LicenseStatusRevoked {
		return ErrLicenseClaims
	}
	if !licenseNoPattern.MatchString(claims.LicenseNo) {
		return ErrLicenseClaims
	}
	corpLen := len([]rune(claims.CorpName))
	if corpLen < 1 || corpLen > 100 {
		return ErrLicenseClaims
	}
	if _, err := NormalizeDeviceCode(claims.DeviceCode); err != nil {
		return ErrLicenseClaims
	}
	if claims.IssuedAt <= 0 || claims.NotBefore < claims.IssuedAt || claims.ExpireAt <= claims.NotBefore {
		return ErrLicenseClaims
	}
	if claims.Revision < 1 || claims.Revision > define.LicenseMaxRevision {
		return ErrLicenseClaims
	}
	// No permanent license: expire_at must be an explicit future Unix second.
	if claims.ExpireAt <= 0 {
		return ErrLicenseClaims
	}
	// Trial window may be extended by operator renewal: trial_end_at must equal expire_at and
	// end after it started, but is not pinned to the initial 30-day sign-on window anymore.
	if claims.LicenseKind == define.LicenseKindTrial {
		if claims.TrialStartedAt <= 0 ||
			claims.TrialEndAt != claims.ExpireAt ||
			claims.TrialEndAt <= claims.TrialStartedAt {
			return ErrLicenseClaims
		}
	} else if claims.TrialStartedAt != 0 || claims.TrialEndAt != 0 {
		return ErrLicenseClaims
	}
	q := claims.Quotas
	bounds := []struct {
		value int64
		max   int64
	}{
		{q.MemberLimit, define.LicenseMaxMemberLimit},
		{q.ProjectLimit, define.LicenseMaxProjectLimit},
		{q.CustomFieldPerProjectLimit, define.LicenseMaxPerProjectLimit},
		{q.AutomationRulePerProjectLimit, define.LicenseMaxPerProjectLimit},
		{q.AutomationExecLimit, define.LicenseMaxAutomationExec},
		{q.AgentLimit, define.LicenseMaxAgentLimit},
		{q.StorageLimit, define.LicenseMaxStorageLimit},
	}
	for _, bound := range bounds {
		if bound.value < 1 || bound.value > bound.max {
			return ErrLicenseClaims
		}
	}
	if len(claims.Features) > len(define.LicenseKnownFeatures) {
		return ErrLicenseClaims
	}
	seen := make(map[string]struct{}, len(claims.Features))
	for _, feature := range claims.Features {
		if !featureAllowed(feature) {
			return ErrLicenseClaims
		}
		if _, dup := seen[feature]; dup {
			return ErrLicenseClaims
		}
		seen[feature] = struct{}{}
	}
	return nil
}

// featureAllowed reports whether the claim feature code is whitelisted.
func featureAllowed(feature string) bool {
	for _, known := range define.LicenseKnownFeatures {
		if known == feature {
			return true
		}
	}
	return false
}

// EvaluateLicenseClaims derives the blocking state from verified claims against the local
// clock and device code. It is fail-closed: every check that fails yields a blocking state.
func EvaluateLicenseClaims(claims *LicenseClaims, now int64, currentDeviceCode string) string {
	if claims.Status != define.LicenseStatusActive {
		return define.LicenseStateInvalid
	}
	// A key claiming issuance in the future means the local clock is behind — do not trust it.
	if claims.IssuedAt > now+define.LicenseClockSkewTolerance {
		return define.LicenseStateClockInvalid
	}
	if now < claims.NotBefore {
		return define.LicenseStateNotYetValid
	}
	if now >= claims.ExpireAt {
		return define.LicenseStateExpired
	}
	if claims.DeviceCode != currentDeviceCode {
		return define.LicenseStateDeviceMismatch
	}
	if claims.ExpireAt-now <= define.LicenseExpiringSeconds {
		return define.LicenseStateExpiring
	}
	return define.LicenseStateValid
}
