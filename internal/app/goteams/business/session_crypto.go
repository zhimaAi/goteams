// Copyright © 2026 Wuhan Sesame Small Customer Service Network Co., Ltd.

// Package business hosts the session-log envelope encryption used by 需求 1896 优化点3.
//
// Scheme (决策 Q2e/Q2f/Q2g):
//   - A master key lives in the server config [session-encryption] master_key (base64, 32 bytes).
//   - KEK = HKDF-SHA256(master_key, salt="", info="goteams-session-log:{adminID}") — derived per tenant.
//   - Each session object gets a fresh random 32-byte DEK; content is AES-256-GCM encrypted with the DEK.
//   - The DEK is wrapped with the tenant KEK via AES-256-GCM and stored alongside the object key.
//   - When master_key is absent the system degrades to plaintext upload (enc = "plain"), so a
//     misconfigured environment never blocks task reporting; enabling the key later is seamless.
package business

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"goteams/internal/app/goteams/define"
)

// Encryption algorithm markers stored in pipeline_task_sessions.conversation_enc.
const (
	// SessionEncPlain marks an unencrypted session object (master key not configured).
	SessionEncPlain = `plain`
	// SessionEncAES256GCM marks AES-256-GCM envelope encryption with a per-object DEK.
	SessionEncAES256GCM = `aes-256-gcm`
)

// hkdfInfo is the domain-separation label for KEK derivation.
const sessionKEKInfo = `goteams-session-log`

// sessionEncryptionEnabled reports whether a usable master key is configured.
func sessionEncryptionEnabled() bool {
	_, err := sessionMasterKey()
	return err == nil
}

// sessionMasterKey decodes the configured master key (base64, must be 32 bytes).
func sessionMasterKey() ([]byte, error) {
	raw := strings.TrimSpace(define.Config.SessionEncryption[`master_key`])
	// goconfig 保留带引号的字符串值，去掉可选的双引号包裹。
	raw = strings.Trim(raw, `"`)
	raw = strings.TrimSpace(raw)
	if raw == `` {
		return nil, errors.New(`session-encryption master_key is not configured`)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf(`session-encryption master_key is not valid base64: %w`, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf(`session-encryption master_key must be 32 bytes, got %d`, len(key))
	}
	return key, nil
}

// deriveSessionKEK derives the per-tenant key-encryption-key via HKDF-SHA256.
func deriveSessionKEK(master []byte, adminId int) ([]byte, error) {
	// hkdf.Extract + expand with the tenant id as the info label.
	salt := make([]byte, sha256.Size) // zero salt; the master key already has full entropy.
	prk := hkdfExtract(salt, master)
	info := fmt.Sprintf(`%s:%d`, sessionKEKInfo, adminId)
	return hkdfExpand(prk, []byte(info), 32)
}

// hkdfExtract implements HKDF-Extract (RFC 5869).
func hkdfExtract(salt, ikm []byte) []byte {
	if len(salt) == 0 {
		salt = make([]byte, sha256.Size)
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write(ikm)
	return mac.Sum(nil)
}

// hkdfExpand implements HKDF-Expand (RFC 5869) for a single 32-byte block (length <= 255*32).
func hkdfExpand(prk, info []byte, length int) ([]byte, error) {
	if length > 255*sha256.Size {
		return nil, errors.New(`hkdf length too large`)
	}
	out := make([]byte, 0, length)
	var prev []byte
	for counter := byte(1); len(out) < length; counter++ {
		mac := hmac.New(sha256.New, prk)
		mac.Write(prev)
		mac.Write(info)
		mac.Write([]byte{counter})
		prev = mac.Sum(nil)
		out = append(out, prev...)
	}
	return out[:length], nil
}

// encryptSessionContent envelope-encrypts a session conversation payload for one tenant.
// Returns the ciphertext bytes, the wrapped DEK as a base64 string (TEXT column), and the
// algorithm marker to persist. When no master key is configured it returns the plaintext
// with enc = SessionEncPlain and an empty wrapped DEK (degraded mode, never blocks reporting).
func encryptSessionContent(adminId int, plaintext []byte) (ciphertext []byte, wrappedDekB64, enc string, err error) {
	master, err := sessionMasterKey()
	if err != nil {
		return plaintext, ``, SessionEncPlain, nil
	}
	kek, err := deriveSessionKEK(master, adminId)
	if err != nil {
		return nil, ``, ``, err
	}

	// Fresh per-object DEK.
	dek := make([]byte, 32)
	if _, err = rand.Read(dek); err != nil {
		return nil, ``, ``, err
	}

	// Encrypt content with the DEK (AES-256-GCM).
	contentCipher, err := newGCM(dek)
	if err != nil {
		return nil, ``, ``, err
	}
	contentNonce := make([]byte, contentCipher.NonceSize())
	if _, err = rand.Read(contentNonce); err != nil {
		return nil, ``, ``, err
	}
	ciphertext = append(contentNonce, contentCipher.Seal(nil, contentNonce, plaintext, nil)...)

	// Wrap the DEK with the tenant KEK (AES-256-GCM).
	wrapCipher, err := newGCM(kek)
	if err != nil {
		return nil, ``, ``, err
	}
	wrapNonce := make([]byte, wrapCipher.NonceSize())
	if _, err = rand.Read(wrapNonce); err != nil {
		return nil, ``, ``, err
	}
	wrappedDek := append(wrapNonce, wrapCipher.Seal(nil, wrapNonce, dek, nil)...)

	return ciphertext, base64.StdEncoding.EncodeToString(wrappedDek), SessionEncAES256GCM, nil
}

// decryptSessionContent reverses encryptSessionContent. wrappedDekB64 is the base64-encoded
// wrapped DEK from the TEXT column. For enc = SessionEncPlain the ciphertext is returned as-is.
func decryptSessionContent(adminId int, ciphertext []byte, wrappedDekB64, enc string) ([]byte, error) {
	if enc == `` || enc == SessionEncPlain {
		return ciphertext, nil
	}
	if enc != SessionEncAES256GCM {
		return nil, fmt.Errorf(`unsupported session enc: %s`, enc)
	}
	wrappedDek, err := base64.StdEncoding.DecodeString(wrappedDekB64)
	if err != nil {
		return nil, fmt.Errorf(`decode wrapped dek failed: %w`, err)
	}
	master, err := sessionMasterKey()
	if err != nil {
		return nil, err
	}
	kek, err := deriveSessionKEK(master, adminId)
	if err != nil {
		return nil, err
	}
	dek, err := gcmOpen(wrappedDek, kek)
	if err != nil {
		return nil, fmt.Errorf(`unwrap session dek failed: %w`, err)
	}
	return gcmOpen(ciphertext, dek)
}

// newGCM builds an AES-256-GCM AEAD for a 32-byte key.
func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// gcmOpen splits nonce||ciphertext and decrypts.
func gcmOpen(data, key []byte) ([]byte, error) {
	aead, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(data) < aead.NonceSize() {
		return nil, errors.New(`ciphertext too short`)
	}
	nonce, body := data[:aead.NonceSize()], data[aead.NonceSize():]
	return aead.Open(nil, nonce, body, nil)
}
