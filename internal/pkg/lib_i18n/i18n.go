// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package lib_i18n

import (
	"embed"
	"fmt"
	"strings"

	"github.com/Unknwon/goconfig"
	"github.com/gin-gonic/gin"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/tool"
)

// Language constants (aligned with frontend vue-i18n locale values).
const (
	LangEnUs = `en-US`
	LangZhCn = `zh-CN`
)

// Langs supported language list.
var Langs = []string{
	LangEnUs,
	LangZhCn,
}

//go:embed locale_*.ini
var localeFiles embed.FS

// translations stores lang → *goconfig.ConfigFile.
var translations = map[string]*goconfig.ConfigFile{}

// init loads all locale files from embed.
func init() {
	for _, lang := range Langs {
		fileName := fmt.Sprintf(`locale_%s.ini`, lang)
		data, err := localeFiles.ReadFile(fileName)
		if err != nil {
			logs.Error("i18n init: cannot read embedded %s: %s", fileName, err.Error())
			continue
		}
		cfg, err := goconfig.LoadFromData(data)
		if err != nil {
			logs.Error("i18n init parse %s error: %s", fileName, err.Error())
			continue
		}
		translations[lang] = cfg
	}
}

// NormalizeLang returns the canonical supported language for an incoming value.
// Unrecognized or empty input resolves to zh-CN, so a client can never pin an
// unsupported locale; matching is case-insensitive (e.g. `en-us` → `en-US`).
func NormalizeLang(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == `` {
		return LangZhCn
	}
	for _, supported := range Langs {
		if strings.EqualFold(supported, lang) {
			return supported
		}
	}
	return LangZhCn
}

// GetLang gets the language preference from request header (defaults to zh-CN).
func GetLang(c *gin.Context) string {
	return NormalizeLang(c.GetHeader(`lang`))
}

// Show returns translated text based on language.
// Usage: lib_i18n.Show(lang, "user_not_found")
func Show(lang, message string, args ...any) string {
	if !tool.InArrayString(lang, Langs) {
		lang = Langs[0]
	}
	// Look up from DEFAULT section using full key.
	if cfg, ok := translations[lang]; ok {
		if v, err := cfg.GetValue(goconfig.DEFAULT_SECTION, message); err == nil {
			message = v
		}
	}
	// Replace escape sequences.
	message = strings.ReplaceAll(message, `\r`, "\r")
	message = strings.ReplaceAll(message, `\n`, "\n")
	message = strings.ReplaceAll(message, `\f`, "\f")
	if len(args) > 0 {
		return fmt.Sprintf(message, args...)
	}
	return message
}

// WrapKey marks a system i18n key that needs to be persisted to the database.
func WrapKey(key string) string {
	if IsWrappedKey(key) {
		return key
	}
	return `${` + key + `}`
}

// IsWrappedKey determines whether the text is in the system i18n key wrapped format.
func IsWrappedKey(message string) bool {
	return strings.HasPrefix(message, `${`) && strings.HasSuffix(message, `}`) && len(message) > 3
}

// UnwrapKey removes the system i18n key wrapper.
func UnwrapKey(message string) string {
	if !IsWrappedKey(message) {
		return message
	}
	return strings.TrimSuffix(strings.TrimPrefix(message, `${`), `}`)
}

// ShowStored translates only system i18n keys wrapped in ${xxx}; plain user input is returned as-is.
func ShowStored(lang, message string, args ...any) string {
	if !IsWrappedKey(message) {
		return message
	}
	return Show(lang, UnwrapKey(message), args...)
}

// ShowByContext is a convenience method: auto-detects language from gin.Context.
func ShowByContext(c *gin.Context, message string, args ...any) string {
	return Show(GetLang(c), message, args...)
}
