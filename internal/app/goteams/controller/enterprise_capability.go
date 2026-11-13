package controller

import (
	"net/http"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
)

// EnterpriseCapabilityDisabled 拒绝开源版企业能力，不删除存量配置或账号。
func EnterpriseCapabilityDisabled(c *gin.Context) {
	c.JSON(http.StatusForbidden, define.Response{
		Code:    http.StatusForbidden,
		Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgFeatureDisabled),
	})
}
