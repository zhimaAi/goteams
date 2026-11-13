package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"
	"goteams/internal/pkg/lib_storage"

	"github.com/gin-gonic/gin"
	"github.com/zhimaAi/go_tools/logs"
)

const maxOSSRequestBody = 1 << 20

// OSSURLRequestMiddleware keeps temporary credentials out of request values persisted by handlers.
func OSSURLRequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// These endpoints authenticate the original bytes and normalize only after verification.
		path := c.FullPath()
		if strings.HasPrefix(path, `/api/open/`) || strings.HasPrefix(path, `/mobile/open/`) {
			c.Next()
			return
		}
		stripOSSRequestURLs(c)
		if !c.IsAborted() {
			c.Next()
		}
	}
}

func stripOSSRequestURLs(c *gin.Context) {
	if err := normalizeOSSRequest(c.Request, c.ContentType()); err != nil {
		logs.Error("normalize OSS request URLs failed: %s", err.Error())
		c.AbortWithStatusJSON(http.StatusBadRequest, define.Response{
			Code: http.StatusBadRequest, Message: lib_i18n.Show(lib_i18n.GetLang(c), define.MsgParamInvalid),
		})
	}
}

func normalizeOSSRequest(request *http.Request, contentType string) error {
	query := request.URL.Query()
	if stripOSSFormValues(query) {
		request.URL.RawQuery = query.Encode()
	}
	if request.Body == nil || request.Body == http.NoBody {
		return nil
	}
	contentType = strings.ToLower(contentType)
	switch contentType {
	case `multipart/form-data`:
		if err := request.ParseMultipartForm(32 << 20); err != nil {
			return err
		}
		stripOSSFormValues(request.MultipartForm.Value)
		stripOSSFormValues(request.PostForm)
		stripOSSFormValues(request.Form)
		return nil
	case `application/x-www-form-urlencoded`:
		if err := request.ParseForm(); err != nil {
			return err
		}
		stripOSSFormValues(request.PostForm)
		stripOSSFormValues(request.Form)
		return nil
	case `application/json`:
	default:
		if !strings.HasPrefix(contentType, `application/`) || !strings.HasSuffix(contentType, `+json`) {
			return nil
		}
	}
	if request.ContentLength > maxOSSRequestBody {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxOSSRequestBody+1))
	if err != nil {
		return err
	}
	if len(body) > maxOSSRequestBody {
		// Replay the prefix without closing the stream; endpoint-specific limits still apply.
		request.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(body), request.Body), request.Body}
		return nil
	}
	if err = request.Body.Close(); err != nil {
		return err
	}
	text := string(body)
	if len(body) > 0 {
		if !json.Valid(body) {
			return errors.New(`invalid JSON request body`)
		}
		text = lib_storage.StripOSSURLSignatures(text)
	}
	request.Body = io.NopCloser(strings.NewReader(text))
	request.ContentLength = int64(len(text))
	if request.Header.Get(`Content-Length`) != `` {
		request.Header.Set(`Content-Length`, strconv.Itoa(len(text)))
	}
	return nil
}

func stripOSSFormValues(values url.Values) bool {
	changed := false
	for _, items := range values {
		for i, value := range items {
			replaced := lib_storage.StripOSSURLSignatures(value)
			if replaced != value {
				items[i] = replaced
				changed = true
			}
		}
	}
	return changed
}
