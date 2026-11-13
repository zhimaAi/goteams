// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"goteams/internal/app/goteams/define"
)

// License receipt verification (GTRECEIPT1).
//
// Trial licenses run in mandatory-online mode: the operations backend issues a short-term
// receipt signed by the same ES256 key pair as license keys, and the terminal blocks business
// APIs once no valid receipt exists. Receipt parsing is strict and fail-closed, mirroring
// ParseLicenseKey: unknown fields, wrong types, oversized windows or mismatched device codes
// reject the whole receipt.

// LicenseReceiptClaims mirrors the signed claims JSON of a GTRECEIPT1 receipt.
type LicenseReceiptClaims struct {
	LicenseNo       string `json:"license_no"`
	DeviceCode      string `json:"device_code"`
	ServerCheckedAt int64  `json:"server_checked_at"`
	ValidUntil      int64  `json:"valid_until"`
	Status          string `json:"status"`
}

// ParsedLicenseReceipt is the verified result of ParseLicenseReceipt.
type ParsedLicenseReceipt struct {
	Raw         string
	Header      LicenseKeyHeader
	Claims      LicenseReceiptClaims
	Fingerprint string
}

// License receipt verification errors (never returned to users; callers map them to blocking states).
var (
	ErrLicenseReceiptFormat     = errors.New(`license receipt format invalid`)
	ErrLicenseReceiptTooLong    = errors.New(`license receipt too long`)
	ErrLicenseReceiptAlg        = errors.New(`license receipt algorithm mismatch`)
	ErrLicenseReceiptTyp        = errors.New(`license receipt type mismatch`)
	ErrLicenseReceiptUnknownKID = errors.New(`license receipt unknown kid`)
	ErrLicenseReceiptSignature  = errors.New(`license receipt signature invalid`)
	ErrLicenseReceiptClaims     = errors.New(`license receipt claims invalid`)
)

// ParseLicenseReceipt parses, signature-verifies and fully validates a GTRECEIPT1 receipt.
// It is a pure function: no database, no gin, no business dependency. Any failure blocks.
func ParseLicenseReceipt(rawReceipt string) (*ParsedLicenseReceipt, error) {
	receipt := strings.TrimSpace(rawReceipt)
	if receipt == `` {
		return nil, ErrLicenseReceiptFormat
	}
	if len(receipt) > define.LicenseKeyMaxBytes {
		return nil, ErrLicenseReceiptTooLong
	}
	parts := strings.Split(receipt, `.`)
	if len(parts) != 4 || parts[0] != define.LicenseReceiptPrefix {
		return nil, ErrLicenseReceiptFormat
	}

	// Verify against the raw prefix + first two segments exactly as received.
	signingInput := parts[0] + `.` + parts[1] + `.` + parts[2]

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrLicenseReceiptFormat
	}
	var header LicenseKeyHeader
	if err = json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrLicenseReceiptFormat
	}
	if header.Alg != define.LicenseKeyAlg {
		return nil, ErrLicenseReceiptAlg
	}
	if header.Typ != define.LicenseReceiptTyp {
		return nil, ErrLicenseReceiptTyp
	}
	pubPem, ok := define.LicensePublicKeys[header.Kid]
	if !ok || strings.TrimSpace(header.Kid) == `` {
		return nil, ErrLicenseReceiptUnknownKID
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrLicenseReceiptFormat
	}
	var claims LicenseReceiptClaims
	decoder := json.NewDecoder(strings.NewReader(string(claimsBytes)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&claims); err != nil {
		return nil, ErrLicenseReceiptFormat
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, ErrLicenseReceiptFormat
	}
	pub, err := loadLicensePubkey(header.Kid, pubPem)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(signingInput))
	if !ecdsa.VerifyASN1(pub, digest[:], sigBytes) {
		return nil, ErrLicenseReceiptSignature
	}

	if err = validateLicenseReceiptClaims(&claims); err != nil {
		return nil, err
	}
	fullHash := sha256.Sum256([]byte(receipt))
	return &ParsedLicenseReceipt{
		Raw:         receipt,
		Header:      header,
		Claims:      claims,
		Fingerprint: hex.EncodeToString(fullHash[:])[:12],
	}, nil
}

// validateLicenseReceiptClaims enforces every structural rule of the receipt protocol.
func validateLicenseReceiptClaims(claims *LicenseReceiptClaims) error {
	if claims.Status != define.LicenseStatusActive {
		return ErrLicenseReceiptClaims
	}
	if !licenseNoPattern.MatchString(claims.LicenseNo) {
		return ErrLicenseReceiptClaims
	}
	if _, err := NormalizeDeviceCode(claims.DeviceCode); err != nil {
		return ErrLicenseReceiptClaims
	}
	if claims.ServerCheckedAt <= 0 || claims.ValidUntil <= claims.ServerCheckedAt {
		return ErrLicenseReceiptClaims
	}
	// The operations backend may not issue windows longer than 24 hours.
	if claims.ValidUntil-claims.ServerCheckedAt > define.LicenseReceiptMaxSeconds {
		return ErrLicenseReceiptClaims
	}
	return nil
}

// EvaluateLicenseReceipt derives the receipt state at a given time: empty string means valid,
// any other value is a blocking state mapped from the receipt lifecycle.
func EvaluateLicenseReceipt(parsed *ParsedLicenseReceipt, now int64, licenseNo, currentDeviceCode string) string {
	if parsed == nil {
		return define.LicenseStateExpired
	}
	if parsed.Claims.LicenseNo != licenseNo || parsed.Claims.DeviceCode != currentDeviceCode {
		return define.LicenseStateInvalid
	}
	if now >= parsed.Claims.ValidUntil {
		return define.LicenseStateExpired
	}
	return ``
}
