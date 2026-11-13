// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package initialize

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
)

// ossSignWriter 先攒住 json 响应，等 handler 写完再把里面的 oss 地址换成签名地址。
// 非 json 响应（文件下载、导出）不攒，直接透传
type ossSignWriter struct {
	gin.ResponseWriter
	code    int
	decided bool
	isJSON  bool
	buf     bytes.Buffer
}

// WriteHeader 先记住状态码。gin 是先写状态码再设 Content-Type，
// 所以这里判不了响应类型，等第一次 Write 时再判
func (w *ossSignWriter) WriteHeader(code int) {
	w.code = code
	if w.decided && !w.isJSON {
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write 第一次写数据时看 Content-Type 决定攒还是透传
func (w *ossSignWriter) Write(data []byte) (int, error) {
	if !w.decided {
		w.decided = true
		w.isJSON = strings.Contains(w.Header().Get(`Content-Type`), `json`)
		if !w.isJSON {
			w.ResponseWriter.WriteHeader(w.Status())
		}
	}
	if !w.isJSON {
		return w.ResponseWriter.Write(data)
	}
	w.buf.Write(data)
	return len(data), nil
}

// WriteString 走和 Write 一样的逻辑，别绕过缓冲
func (w *ossSignWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// WriteHeaderNow 立刻把状态码写出去。gin 在只写状态码不写内容时会直接调这个，
// 不转过去的话状态码会丢（比如 AbortWithStatus(403) 会变成 200）
func (w *ossSignWriter) WriteHeaderNow() {
	w.ResponseWriter.WriteHeader(w.Status())
	w.ResponseWriter.WriteHeaderNow()
}

// Status reports the buffered status before the response is flushed.
func (w *ossSignWriter) Status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}

// finish 把攒下来的响应换掉 oss 地址再写出去
func (w *ossSignWriter) finish() {
	if !w.decided {
		// handler 只写了状态码没写内容（比如 304），补上
		if w.code != 0 && !w.ResponseWriter.Written() {
			w.ResponseWriter.WriteHeader(w.code)
			w.ResponseWriter.WriteHeaderNow()
		}
		return
	}
	if !w.isJSON {
		return
	}
	body := lib_storage.RewriteOSSURLs(w.buf.String())
	w.Header().Set(`Content-Length`, strconv.Itoa(len(body)))
	w.ResponseWriter.WriteHeader(w.Status())
	if body != `` {
		_, _ = w.ResponseWriter.Write([]byte(body))
	}
}

// ossURLSignMiddleware 统一把响应里的 oss 地址换成带签名的临时地址
func ossURLSignMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !lib_storage.SignEnabled() {
			c.Next()
			return
		}
		writer := &ossSignWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		writer.finish()
	}
}
