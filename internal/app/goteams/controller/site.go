// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"net/http"
	"regexp"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

var crawlerUserAgentPattern = regexp.MustCompile(`(?i)(bot|crawler|spider|slurp|bingpreview|facebookexternalhit|headlesschrome|lighthouse)`)

var countryCodeHeaders = []string{
	`CF-IPCountry`,
	`CloudFront-Viewer-Country`,
	`X-Country-Code`,
	`X-Forwarded-Country`,
	`X-Geo-Country`,
}

type siteRegionResponse struct {
	IsChinaMainland bool `json:"is_china_mainland"`
}

// SiteRegion 根据 IP 地区信号和浏览器 UA 信号判断访问者是否来自中国大陆。
func SiteRegion(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: siteRegionResponse{
			IsChinaMainland: isChinaMainlandVisit(
				getCountryCode(c),
				c.GetHeader(`User-Agent`),
				c.Query(`languages`),
				c.Query(`timezone`),
			),
		},
	})
}

func getCountryCode(c *gin.Context) string {
	for _, header := range countryCodeHeaders {
		if value := strings.TrimSpace(c.GetHeader(header)); value != `` {
			return value
		}
	}
	return ``
}

func isChinaMainlandVisit(countryCode, userAgent, languages, timezone string) bool {
	// 搜索引擎和预览机器人保持在原域名，避免重定向影响国际站收录。
	if crawlerUserAgentPattern.MatchString(userAgent) {
		return false
	}

	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if code == `CN` || code == `CHN` {
		return true
	}
	// 两位国家码代表边缘代理已经完成 IP 定位，此时不再用语言覆盖定位结果。
	if len(code) == 2 && code != `XX` {
		return false
	}

	return timezone == `Asia/Shanghai` && hasMainlandChineseLanguage(languages)
}

func hasMainlandChineseLanguage(languages string) bool {
	for _, language := range strings.Split(strings.ToLower(languages), `,`) {
		normalized := strings.TrimSpace(language)
		if normalized == `zh-cn` || strings.HasPrefix(normalized, `zh-hans`) {
			return true
		}
	}
	return false
}
