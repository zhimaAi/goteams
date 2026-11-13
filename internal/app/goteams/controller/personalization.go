package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
)

type savePersonalizationRequest struct {
	AppName      string `json:"app_name"`
	Logo         string `json:"logo"`
	DomainPrefix string `json:"domain_prefix"`
	LoginSlogan  string `json:"login_slogan"`
}

type personalizationResponse struct {
	AppName      string `json:"app_name"`
	Logo         string `json:"logo"`
	LogoURL      string `json:"logo_url"`
	DomainPrefix string `json:"domain_prefix"`
	DomainSuffix string `json:"domain_suffix"`
	DomainScheme string `json:"domain_scheme"`
	AccessURL    string `json:"access_url"`
	LoginSlogan  string `json:"login_slogan"`
	Editable     bool   `json:"editable"`
}

type brandingResponse struct {
	AppName     string `json:"app_name"`
	LogoURL     string `json:"logo_url"`
	LoginSlogan string `json:"login_slogan"`
	Dedicated   bool   `json:"dedicated"`
	SsoURL      string `json:"sso_url"`
	SsoLabel    string `json:"sso_label"`
}

func toPersonalizationResponse(model business.PersonalizationModel) personalizationResponse {
	site, _ := url.Parse(business.PersonalizationSiteURL(""))
	suffix, scheme := "", ""
	if site != nil && site.Host != "" {
		suffix, scheme = "."+site.Host, site.Scheme+"://"
	}
	accessURL := ""
	if model.DomainPrefix != "" {
		accessURL = business.PersonalizationSiteURL(model.DomainPrefix)
	}
	return personalizationResponse{AppName: model.AppName, Logo: model.Logo,
		LogoURL: lib_storage.SignedURL(model.Logo), DomainPrefix: model.DomainPrefix,
		DomainSuffix: suffix, DomainScheme: scheme, AccessURL: accessURL,
		LoginSlogan: model.LoginSlogan, Editable: model.Editable}
}

// GetPersonalization 返回个性化管理表单；普通版可查看但不能保存。
func GetPersonalization(c *gin.Context) {
	model, err := business.GetPersonalization(c.GetInt("adminId"))
	if err != nil {
		abortPersonalizationError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toPersonalizationResponse(model)})
}

// SavePersonalization 校验并保存当前认证租户的品牌。
func SavePersonalization(c *gin.Context) {
	var req savePersonalizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, lib_i18n.Show(lib_i18n.GetLang(c), define.MsgParamInvalid), "personalization.bind", err)
		return
	}
	model, err := business.SavePersonalization(business.PersonalizationModel{
		AdminId: c.GetInt("adminId"), AppName: req.AppName, Logo: req.Logo,
		DomainPrefix: req.DomainPrefix, LoginSlogan: req.LoginSlogan,
	})
	if err != nil {
		abortPersonalizationError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: toPersonalizationResponse(model)})
}

// UploadPersonalizationLogo 暂存当前租户的应用图标，不提前应用未保存的表单。
func UploadPersonalizationLogo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, business.PersonalizationLogoMaxSize+64*1024)
	file, err := c.FormFile("file")
	if err != nil {
		abortPersonalizationError(c, business.ErrPersonalizationLogo)
		return
	}
	path, err := business.UploadPersonalizationLogo(c.GetInt("adminId"), file)
	if err != nil {
		abortPersonalizationError(c, err)
		return
	}
	type uploadLogoResponse struct {
		Logo    string `json:"logo"`
		LogoURL string `json:"logo_url"`
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: uploadLogoResponse{Logo: path, LogoURL: lib_storage.SignedURL(path)}})
}

// GetPublicBranding 只返回当前专属域名的公开品牌与 SSO 入口，不公开管理字段。
func GetPublicBranding(c *gin.Context) {
	model, err := business.GetPublicBranding(c.Request.Host)
	if err != nil {
		abortPersonalizationError(c, err)
		return
	}
	data := brandingResponse{AppName: model.AppName, LogoURL: lib_storage.SignedURL(model.Logo),
		LoginSlogan: model.LoginSlogan, Dedicated: model.AdminId > 0}
	if data.Dedicated {
		entry := business.SsoEntry(model.AdminId, lib_i18n.GetLang(c))
		if entry.Enabled {
			scopeId, encodeErr := business.EncodeSsoScopeId(model.AdminId)
			if encodeErr != nil {
				abortPersonalizationError(c, encodeErr)
				return
			}
			data.SsoURL = "/api/sso/authorize?scope_id=" + url.QueryEscape(scopeId)
			data.SsoLabel = entry.ProviderLabel
		}
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: data})
}

// GetTenantBranding 为登录后的租户成员提供品牌，不要求设置权限。
func GetTenantBranding(c *gin.Context) {
	model, err := business.GetTenantBranding(c.GetInt("adminId"))
	if err != nil {
		abortPersonalizationError(c, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{Code: define.CodeSuccess, Data: brandingResponse{
		AppName: model.AppName, LogoURL: lib_storage.SignedURL(model.Logo), LoginSlogan: model.LoginSlogan,
	}})
}

func abortPersonalizationError(c *gin.Context, err error) {
	lang := lib_i18n.GetLang(c)
	var fieldErr *validate.FieldError
	switch {
	case errors.Is(err, business.ErrPersonalizationEnterprise):
		abortForbidden(c, lib_i18n.Show(lang, define.MsgPersonalizationEnterprise))
	case errors.Is(err, business.ErrPersonalizationDomainTaken):
		abortConflict(c, lib_i18n.Show(lang, define.MsgPersonalizationDomainTaken))
	case errors.Is(err, business.ErrPersonalizationLogo):
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgPersonalizationLogo), "personalization.logo", err)
	case errors.As(err, &fieldErr):
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), "personalization.validate", err)
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), "personalization", err)
	}
}

// ssoPersonalizationBase 仅从已登记的企业配置计算回跳域名，回调地址保持厂商登记的固定地址。
func ssoPersonalizationBase(c *gin.Context, adminId int) string {
	if adminId > 0 {
		model, err := business.GetTenantBranding(adminId)
		if err != nil {
			logRequestError(c, "personalization.sso_return", err)
		} else if model.DomainPrefix != "" {
			return strings.TrimRight(business.PersonalizationSiteURL(model.DomainPrefix), "/")
		}
	}
	return oauthSiteBase(c)
}
