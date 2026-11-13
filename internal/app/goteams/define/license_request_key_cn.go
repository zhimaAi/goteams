// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build !commercial_dev2 && !commercial_dev3 && !commercial_dev6

package define

// RequestSignCurrentKID identifies the CN terminal request signing key embedded in this build.
const RequestSignCurrentKID = `goteams-req-cn-2026-09`

// RequestSignPrivateKey is the ES256 key that signs outbound operations-backend requests (GTREQ1).
//
// Unlike the license signing key, this private key must ship inside the terminal binary: it is an
// anti-abuse gate that raises the cost of anonymous calls, not a secret and not an entitlement.
// A passing request signature never authorizes anything on its own. Never reuse the license
// signing key here and never store the two key files together.
var RequestSignPrivateKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgPCbURbivCo3xZfUZ
SmWCQM1085u7yEwwpZmHx7pSwuihRANCAATBG1RGJvVDFqP9hSC7brqFoNmebPkr
AociQ8naQQn1KG5ohKhf3De6B7gEcXj6BxhLDlNcL+vjfrAHbh7qCpsU
-----END PRIVATE KEY-----`
