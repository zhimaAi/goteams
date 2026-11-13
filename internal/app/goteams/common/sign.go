package common

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// OpenApiSignMiddleware 校验运营开放接口的时间戳、随机串和签名。
// 签名算法：MD5(rawBody + timestamp + nonce + secret)。
func OpenApiSignMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := lib_i18n.GetLang(c)
		timestamp := c.Query(`timestamp`)
		nonce := c.Query(`nonce`)
		sign := strings.ToLower(c.Query(`sign`))
		secret := define.Config.OpenApi[`secret`]
		unixTime, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || nonce == `` || sign == `` || secret == `` || abs64(time.Now().Unix()-unixTime) > 300 {
			abortOpenApi(c, lang)
			return
		}
		body, err := c.GetRawData()
		if err != nil {
			abortOpenApi(c, lang)
			return
		}
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		sum := md5.Sum([]byte(string(body) + timestamp + nonce + secret))
		expected := hex.EncodeToString(sum[:])
		if len(sign) != len(expected) || subtle.ConstantTimeCompare([]byte(sign), []byte(expected)) != 1 {
			abortOpenApi(c, lang)
			return
		}
		stripOSSRequestURLs(c)
		if c.IsAborted() {
			return
		}
		c.Next()
	}
}

func abortOpenApi(c *gin.Context, lang string) {
	c.JSON(http.StatusUnauthorized, define.Response{Code: http.StatusUnauthorized, Message: lib_i18n.Show(lang, define.MsgOpenApiUnauthorized)})
	c.Abort()
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
