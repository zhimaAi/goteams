// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build !commercial_dev2 && !commercial_dev3 && !commercial_dev6

package define

// LicenseCurrentKID identifies the CN operations signing key embedded in this build.
const LicenseCurrentKID = `goteams-cn-2026-09`

// LicensePublicKeys contains only public verification keys; private keys never ship in GoTeams.
var LicensePublicKeys = map[string]string{
	LicenseCurrentKID: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEyBxFHxiJN40Z5tRrEcOPgbxigOJm
BuydlIqSbdspuL8hLYj0jMu/oiC1s4EgLOor2Bnpduf6MwxKstYP97Okcw==
-----END PUBLIC KEY-----`,
	// Keep the previous CN public key during rotation so already-issued licenses remain valid.
	`goteams-cn-2026-01`: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8t/xFYXvx3lgYHmlW6j8Atgswf/j
ze6w+4zmIGaB76jokeEsnJ3zrpvQ6r6lAQ8ZTT8+t9JqTF57HAVSFYcgvw==
-----END PUBLIC KEY-----`,
}
