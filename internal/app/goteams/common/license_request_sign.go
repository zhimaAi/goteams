// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"goteams/internal/app/goteams/define"
)

// Ops-backend request signing (GTREQ1). Every outbound terminal call carries an ES256 signature
// over a timestamp, a single-use nonce, the target path and the SHA-256 of the raw request body.
//
// The gate exists to keep anonymous scripted abuse away from the public trial endpoints. It is not
// an entitlement check and the private key is extractable from the binary by design, so nothing on
// the trust boundary may depend on it alone.

// requestSignNonceBytes sizes the replay nonce as hex text; the backend accepts 8-64 hex chars.
const requestSignNonceBytes = 8

// ErrRequestSignKey marks an unusable embedded request signing key.
var ErrRequestSignKey = errors.New(`request sign key unavailable`)

var (
	requestSignKeyOnce sync.Once
	requestSignKeyVal  *ecdsa.PrivateKey
	requestSignKeyErr  error
)

// requestSignHeader mirrors the GTREQ1 protected header verified by the operations backend.
type requestSignHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

// requestSignClaims mirrors the GTREQ1 payload; the field names are part of the wire contract.
type requestSignClaims struct {
	Ts         int64  `json:"ts"`
	Nonce      string `json:"nonce"`
	Path       string `json:"path"`
	BodySha256 string `json:"body_sha256"`
}

// loadRequestSignKey parses and caches the build-embedded ES256 request signing private key.
func loadRequestSignKey() (*ecdsa.PrivateKey, error) {
	requestSignKeyOnce.Do(func() {
		block, _ := pem.Decode([]byte(strings.TrimSpace(define.RequestSignPrivateKey)))
		if block == nil {
			requestSignKeyErr = fmt.Errorf(`request sign key pem decode failed`)
			return
		}
		generic, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			requestSignKeyErr = fmt.Errorf(`request sign key parse failed: %w`, err)
			return
		}
		priv, ok := generic.(*ecdsa.PrivateKey)
		if !ok || priv.Curve.Params().Name != `P-256` {
			requestSignKeyErr = fmt.Errorf(`request sign key is not ECDSA P-256`)
			return
		}
		requestSignKeyVal = priv
	})
	if requestSignKeyErr != nil {
		return nil, requestSignKeyErr
	}
	if requestSignKeyVal == nil {
		return nil, ErrRequestSignKey
	}
	return requestSignKeyVal, nil
}

// BuildOpsRequestSign returns the GTREQ1 signature string carried in the X-GT-Request-Sign header.
//
// body must be the exact bytes written to the request: the operations backend hashes the raw body
// it receives, so re-encoding the form between signing and sending breaks the digest match.
func BuildOpsRequestSign(path string, body []byte) (string, error) {
	priv, err := loadRequestSignKey()
	if err != nil {
		return ``, err
	}
	nonceBuf := make([]byte, requestSignNonceBytes)
	if _, err = rand.Read(nonceBuf); err != nil {
		return ``, err
	}
	digest := sha256.Sum256(body)
	headerBytes, err := json.Marshal(requestSignHeader{
		Alg: define.LicenseRequestSignAlg,
		Typ: define.LicenseRequestSignTyp,
		Kid: define.RequestSignCurrentKID,
	})
	if err != nil {
		return ``, err
	}
	claimsBytes, err := json.Marshal(requestSignClaims{
		Ts:         time.Now().Unix(),
		Nonce:      hex.EncodeToString(nonceBuf),
		Path:       path,
		BodySha256: hex.EncodeToString(digest[:]),
	})
	if err != nil {
		return ``, err
	}
	// The signing input is the raw prefix plus the two base64url segments exactly as transmitted.
	signingInput := define.LicenseRequestSignPrefix + `.` +
		base64.RawURLEncoding.EncodeToString(headerBytes) + `.` +
		base64.RawURLEncoding.EncodeToString(claimsBytes)
	claimsDigest := sha256.Sum256([]byte(signingInput))
	sig, err := ecdsa.SignASN1(rand.Reader, priv, claimsDigest[:])
	if err != nil {
		return ``, err
	}
	return signingInput + `.` + base64.RawURLEncoding.EncodeToString(sig), nil
}
