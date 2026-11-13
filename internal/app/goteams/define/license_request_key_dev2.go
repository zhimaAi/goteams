// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build commercial_dev2

package define

// RequestSignCurrentKID identifies the dev2 terminal request signing key embedded in this build.
const RequestSignCurrentKID = `goteams-req-dev-2026-09`

// RequestSignPrivateKey is the ES256 key that signs outbound operations-backend requests (GTREQ1).
//
// Unlike the license signing key, this private key must ship inside the terminal binary: it is an
// anti-abuse gate that raises the cost of anonymous calls, not a secret and not an entitlement.
// A passing request signature never authorizes anything on its own. Never reuse the license
// signing key here and never store the two key files together.
var RequestSignPrivateKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgZVqf3zO6YYAzmPiz
Q0xDxO+qDp/VDk3hB+hrw1u9gdqhRANCAAQNNWVr7G1ZLEE/YtAuXvKF+OtgtU4W
wT5IswlyRKyZR/lCS1TeOYiHQyQq01o/o3B+Dnj8ZtIaAyU2BeTay8QJ
-----END PRIVATE KEY-----`
