// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

// Package templateassets embeds the offline template package so a deployment that ships only
// the binary (the commercial package mounts bin/data/logs and no assets directory) still
// carries the default template projects.
//
// The file lives here — not under internal/app/goteams/assets — because the export/publish
// tooling reads and writes assets/template/template.tar.gz directly, and go:embed cannot reach
// a parent directory through `..`, so the embed declaration must sit next to the file.
package templateassets

import _ "embed"

// Package is the default offline template package. business.LoadTemplatePackage falls back to
// it only when no file exists at the configured path, so a file dropped at
// assets/template/template.tar.gz still wins (and local/dev builds keep working unchanged).
//
//go:embed template.tar.gz
var Package []byte
