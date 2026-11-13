// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

//go:build !commercial_dev2 && !commercial_dev3 && !commercial_dev6

// Package goteamsconfig exposes the environment-specific commercial INI embedded in goteams.
package goteamsconfig

import _ "embed"

// ConfigIni is the CN commercial config consumed by internal/app/goteams/initialize.
//
//go:embed config_embed_cn.ini
var ConfigIni []byte
