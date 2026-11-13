// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

type apiTokenPermissionResponse struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type apiTokenResponseItem struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	TokenPrefix string `json:"token_prefix"`
	// Token 是明文 Key，列表卡片展示与复制都用它。
	Token           string                       `json:"token"`
	Permissions     []apiTokenPermissionResponse `json:"permissions"`
	Enabled         int                          `json:"enabled"`
	TodayCalls      int                          `json:"today_calls"`
	YesterdayCalls  int                          `json:"yesterday_calls"`
	CumulativeCalls int                          `json:"cumulative_calls"`
	LastUsedAt      int                          `json:"last_used_at"`
	CreatedAt       int                          `json:"created_at"`
}

type createApiTokenRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// createApiTokenResponse 是明文 token 唯一的出口，之后任何接口都不再返回它。
type createApiTokenResponse struct {
	Token string               `json:"token"`
	Item  apiTokenResponseItem `json:"item"`
}

type updateApiTokenRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// setApiTokenEnabledRequest 用指针区分「未传」与显式停用（0）。
type setApiTokenEnabledRequest struct {
	Enabled *int `json:"enabled"`
}

// ListApiTokens 返回当前用户创建的 API token 列表。
func ListApiTokens(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	items, err := business.ListApiTokens(adminId, userId)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api_token.list`, err)
		return
	}
	names := apiTokenPermissionNames(lang)
	data := make([]apiTokenResponseItem, 0, len(items))
	for _, item := range items {
		data = append(data, toApiTokenResponseItem(item, names))
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: data,
	})
}

// GetApiTokenPermissions 返回可勾选的权限点，顺序与展示名均由后端决定。
func GetApiTokenPermissions(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	options := business.ApiTokenPermissionOptions()
	data := make([]apiTokenPermissionResponse, 0, len(options))
	for _, option := range options {
		data = append(data, apiTokenPermissionResponse{Key: option.Key, Name: lib_i18n.Show(lang, option.NameKey)})
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: data,
	})
}

// CreateApiToken 创建 API token，响应里带一次明文。
func CreateApiToken(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req createApiTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `api_token.create.bind`, err)
		return
	}
	if !checkApiTokenPermissions(req.Permissions) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.create.perms`, nil)
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	plain, model, err := business.CreateApiToken(adminId, userId, req.Name, req.Permissions)
	if err != nil {
		abortApiTokenError(c, lang, `api_token.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: createApiTokenResponse{Token: plain, Item: toApiTokenResponseItem(model, apiTokenPermissionNames(lang))},
	})
}

// UpdateApiToken 修改备注与权限点，不影响启用状态与历史统计。
func UpdateApiToken(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	tokenId, err := positiveParam(c, `tokenId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.update.param`, err)
		return
	}
	var req updateApiTokenRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `api_token.update.bind`, err)
		return
	}
	if !checkApiTokenPermissions(req.Permissions) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.update.perms`, nil)
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	if _, err = business.UpdateApiToken(adminId, userId, tokenId, req.Name, req.Permissions); err != nil {
		abortApiTokenError(c, lang, `api_token.update`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// SetApiTokenEnabled 启停 API token。
func SetApiTokenEnabled(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	tokenId, err := positiveParam(c, `tokenId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.enabled.param`, err)
		return
	}
	var req setApiTokenEnabledRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `api_token.enabled.bind`, err)
		return
	}
	if req.Enabled == nil || (*req.Enabled != 0 && *req.Enabled != 1) {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.enabled.value`, nil)
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	if err = business.SetApiTokenEnabled(adminId, userId, tokenId, *req.Enabled); err != nil {
		abortApiTokenError(c, lang, `api_token.enabled`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DeleteApiToken 删除 API token。
func DeleteApiToken(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	tokenId, err := positiveParam(c, `tokenId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.delete.param`, err)
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	if err = business.DeleteApiToken(adminId, userId, tokenId); err != nil {
		abortApiTokenError(c, lang, `api_token.delete`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess)})
}

// DownloadApiTokenSkill 下载该 API token 的 Agent skill 安装包（zip，内含明文 Key）。
func DownloadApiTokenSkill(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	tokenId, err := positiveParam(c, `tokenId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.skill.param`, err)
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	filename, data, err := business.BuildApiTokenSkill(adminId, userId, tokenId, apiTokenSiteBaseURL(c), lang)
	if err != nil {
		abortApiTokenError(c, lang, `api_token.skill`, err)
		return
	}
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(filename))
	c.Data(http.StatusOK, `application/zip`, data)
}

// createApiTokenSkillLinkResponse 是临时安装链接的响应：url 可直接交给 AI 下载，5 分钟内有效。
type createApiTokenSkillLinkResponse struct {
	Url           string `json:"url"`
	ExpireSeconds int    `json:"expire_seconds"`
}

// CreateApiTokenSkillLink 生成该 token 的临时安装链接（登录态）。
// 链接只存在 Redis、5 分钟过期且下载一次即失效；同一个 token 同时只有一个有效链接，重新生成会让旧的立即失效。
func CreateApiTokenSkillLink(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	tokenId, err := positiveParam(c, `tokenId`)
	if err != nil {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `api_token.skill.link.param`, err)
		return
	}
	siteUrl := apiTokenSiteBaseURL(c)
	if siteUrl == `` {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api_token.skill.link.site`, nil)
		return
	}
	adminId, userId := c.GetInt(`adminId`), c.GetInt(`userId`)
	linkKey, expireSeconds, err := business.CreateApiTokenSkillLink(adminId, userId, tokenId, siteUrl, lang)
	if err != nil {
		abortApiTokenError(c, lang, `api_token.skill.link`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code: define.CodeSuccess, Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: createApiTokenSkillLinkResponse{
			Url:           siteUrl + define.ApiTokenSkillLinkPath + linkKey,
			ExpireSeconds: expireSeconds,
		},
	})
}

// DownloadApiTokenSkillByLink 凭临时链接下载安装包，不需要登录态：链接本身就是凭据。
// 每次下载占用一个名额（有效期内最多 define.ApiTokenSkillLinkMaxDownloads 次），
// 名额用完后链接作废；名额用 INCR 原子发放，并发请求不会发出超额名额。
func DownloadApiTokenSkillByLink(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	model, err := business.UseApiTokenSkillLink(c.Param(`linkKey`))
	if err != nil {
		abortNotFound(c, lib_i18n.Show(lang, define.MsgApiTokenSkillLinkGone))
		return
	}
	filename, data, err := business.BuildApiTokenSkill(model.AdminId, model.UserId, model.TokenId, model.SiteUrl, model.Lang)
	if err != nil {
		abortApiTokenError(c, lang, `api_token.skill.link.download`, err)
		return
	}
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(filename))
	c.Data(http.StatusOK, `application/zip`, data)
}

// DownloadApiTokenDoc 下载开放接口文档（markdown）。文档描述 /api/v1 的全部接口，不含任何密钥，
// 因此与具体 token 无关：不接收 tokenId，也不按权限裁剪；站点地址与 skill 安装包同源，
// 都取当前部署的 site_url（见 apiTokenSiteBaseURL）。
func DownloadApiTokenDoc(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	siteUrl := apiTokenSiteBaseURL(c)
	if siteUrl == `` {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api_token.doc.site`, nil)
		return
	}
	filename, data, err := business.BuildApiTokenDoc(siteUrl, lang)
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `api_token.doc`, err)
		return
	}
	c.Header(`Content-Disposition`, `attachment; filename*=UTF-8''`+url.PathEscape(filename))
	c.Data(http.StatusOK, `text/markdown; charset=utf-8`, data)
}

// apiTokenSiteBaseURL 返回写进 skill 文档的站点地址：优先配置的站点地址，未配置时退回本次请求的 Origin。
func apiTokenSiteBaseURL(c *gin.Context) string {
	if base := strings.TrimRight(define.Config.WebService[`site_url`], `/`); base != `` {
		return base
	}
	scheme := `http`
	if c.Request.TLS != nil {
		scheme = `https`
	}
	host := strings.TrimSpace(c.Request.Host)
	if host == `` {
		return ``
	}
	return scheme + `://` + host
}

// abortApiTokenError 把业务错误映射到稳定对外状态；归属校验失败统一按 404，不泄漏他人 token 是否存在。
func abortApiTokenError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrApiTokenNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgApiTokenNotFound))
	case errors.Is(err, business.ErrApiTokenNameTooLong):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgApiTokenNameInvalid), stage, err)
	case errors.Is(err, business.ErrApiTokenLimitReached):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgApiTokenLimit, business.ApiTokenMaxPerUser), stage, err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

// checkApiTokenPermissions 至少勾选一项且全部已定义：拼错的作用域必须报错，不能静默降级成空权限。
func checkApiTokenPermissions(perms []string) bool {
	if len(perms) == 0 {
		return false
	}
	for _, perm := range perms {
		if !define.IsApiTokenPermission(perm) {
			return false
		}
	}
	return true
}

// apiTokenPermissionNames 返回权限点 key 到按请求语言翻译后展示名的映射。
func apiTokenPermissionNames(lang string) map[string]string {
	names := make(map[string]string, len(define.AllApiTokenPermissions))
	for _, option := range business.ApiTokenPermissionOptions() {
		names[option.Key] = lib_i18n.Show(lang, option.NameKey)
	}
	return names
}

func toApiTokenResponseItem(model business.ApiTokenModel, names map[string]string) apiTokenResponseItem {
	perms := make([]apiTokenPermissionResponse, 0, len(model.Permissions))
	for _, key := range model.Permissions {
		perms = append(perms, apiTokenPermissionResponse{Key: key, Name: names[key]})
	}
	return apiTokenResponseItem{
		Id:              model.Id,
		Name:            model.Name,
		TokenPrefix:     model.TokenPrefix,
		Token:           model.Token,
		Permissions:     perms,
		Enabled:         model.Enabled,
		TodayCalls:      model.TodayCalls,
		YesterdayCalls:  model.YesterdayCalls,
		CumulativeCalls: model.CumulativeCalls,
		LastUsedAt:      model.LastUsedAt,
		CreatedAt:       model.CreatedAt,
	}
}
