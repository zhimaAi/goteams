// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

// Package assets 收录随程序分发的静态素材（模板、脚本等）。
//
// 素材必须放在某个 Go 包的目录或其子目录下：go:embed 不支持 `..`，包外路径引用不了，
// 因此这里放一个只负责嵌入的包，由 business 等层 import 使用。
package assets

import "embed"

// ApiTokenSkill 是 API Token skill 安装包的素材：SKILL.md 说明文档 + scripts/ 下每个接口一个脚本。
//
//go:embed all:api_token_skill
var ApiTokenSkill embed.FS

// ApiTokenDoc 是开放接口文档（markdown）的素材，按语言分目录存放（zh-CN / en-US）。
// 它面向对接开发者而不是 AI Agent，因此与 skill 的 SKILL.md 分开维护，且不含任何密钥。
//
//go:embed all:api_token_doc
var ApiTokenDoc embed.FS
