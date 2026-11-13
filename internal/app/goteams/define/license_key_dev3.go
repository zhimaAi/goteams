// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build commercial_dev3

package define

// LicenseCurrentKID identifies the dev3 operations signing key embedded in this build (dev2/dev3 share one dev keyring).
const LicenseCurrentKID = `goteams-dev-2026-09`

// LicensePublicKeys contains only public verification keys; private keys never ship in GoTeams.
var LicensePublicKeys = map[string]string{
	LicenseCurrentKID: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEPK22odJV9k1UbzewliQFdfZZz454
rSCu5NGWEKqtEoz5UNHAhTn5oIKwJiUln//Ug0KHw6l/i51u3ChcyTs69w==
-----END PUBLIC KEY-----`,
}
