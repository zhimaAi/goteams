// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package controller

import (
	"errors"
	"net/http"
	"strconv"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/zhimaAi/go_tools/logs"
)

// errInvalidID indicates an invalid resource ID in the route.
var errInvalidID = errors.New(`invalid resource id`)

// positiveParam parses a positive integer route parameter.
func positiveParam(c *gin.Context, name string) (int, error) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id <= 0 {
		return 0, errInvalidID
	}
	return id, nil
}

// ptrStr returns the value of a string pointer, or an empty string for nil. Used by the validation layer to accept non-pointer parameters.
func ptrStr(p *string) string {
	if p == nil {
		return ``
	}
	return *p
}

// logRequestError records controller-layer errors with request context for troubleshooting.
func logRequestError(c *gin.Context, stage string, err error) {
	if err == nil {
		return
	}

	route := c.FullPath()
	if route == `` {
		route = c.Request.URL.Path
	}

	//nolint:staticcheck // typed nil *FieldError as error interface requires safe extraction
	errStr := func() string {
		defer func() { _ = recover() }()
		return err.Error()
	}()
	logs.Error(
		"controller:%s, method:%s, path:%s, route:%s, query:%s, user_id:%d, admin_id:%d, err:%s",
		stage,
		c.Request.Method,
		c.Request.URL.Path,
		route,
		c.Request.URL.RawQuery,
		c.GetInt(`userId`),
		c.GetInt(`adminId`),
		errStr,
	)
}

// respondLoggedError logs the error first, then returns the unified response payload.
func respondLoggedError(c *gin.Context, httpStatus int, code int, message string, stage string, err error) {
	logRequestError(c, stage, err)
	c.JSON(httpStatus, define.Response{
		Code:    code,
		Message: message,
	})
}

// abortBadRequest logs the bad request error and returns the standard 400 response.
func abortBadRequest(c *gin.Context, message string, stage string, err error) {
	respondLoggedError(c, http.StatusBadRequest, http.StatusBadRequest, message, stage, err)
}

// abortInternal logs the internal error and returns the standard 500 response.
func abortInternal(c *gin.Context, message string, stage string, err error) {
	respondLoggedError(c, http.StatusInternalServerError, define.CodeError, message, stage, err)
}

// abortNotFound returns a standard 404 response.
func abortNotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, define.Response{Code: http.StatusNotFound, Message: message})
}

// abortForbidden returns a standard 403 response.
func abortForbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, define.Response{Code: http.StatusForbidden, Message: message})
}

// abortQuotaExceeded returns a 403 response containing quota dimension, scope, and current usage.
func abortQuotaExceeded(c *gin.Context, message string, stage string, err error) bool {
	var quotaErr *business.QuotaExceededError
	if !errors.As(err, &quotaErr) {
		return false
	}
	logRequestError(c, stage, err)
	c.JSON(http.StatusForbidden, define.Response{
		Code:    http.StatusForbidden,
		Message: message,
		Data:    quotaErr,
	})
	return true
}

// abortConflict returns a standard 409 response.
func abortConflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, define.Response{Code: http.StatusConflict, Message: message})
}

// abortTooLarge returns a standard 413 response for oversized payloads.
func abortTooLarge(c *gin.Context, message string, stage string, err error) {
	respondLoggedError(c, http.StatusRequestEntityTooLarge, http.StatusRequestEntityTooLarge, message, stage, err)
}

// abortServiceUnavailable returns 503 with a retry hint; used when the staging disk or a write
// dependency is saturated so the client backs off instead of retrying immediately.
func abortServiceUnavailable(c *gin.Context, message string, stage string, err error) {
	logRequestError(c, stage, err)
	c.Header(`Retry-After`, `5`)
	c.JSON(http.StatusServiceUnavailable, define.Response{
		Code:    http.StatusServiceUnavailable,
		Message: message,
	})
}

// abortTooManyRequests returns 429 with a short retry hint; used when the same resource is
// already being written by a concurrent request, so the caller retries instead of queueing up.
func abortTooManyRequests(c *gin.Context, message string, stage string, err error) {
	logRequestError(c, stage, err)
	c.Header(`Retry-After`, `2`)
	c.JSON(http.StatusTooManyRequests, define.Response{
		Code:    http.StatusTooManyRequests,
		Message: message,
	})
}

// fieldErrorMessage extracts a field-level i18n message from a validation error. If it is *FieldError, returns the translation for the corresponding key; otherwise returns the translation for defaultKey.
func fieldErrorMessage(err error, lang, defaultKey string) string {
	var fe *validate.FieldError
	if errors.As(err, &fe) && fe != nil {
		return lib_i18n.Show(lang, fe.Key)
	}
	return lib_i18n.Show(lang, defaultKey)
}

// bindErrorMessage extracts a field-level message from a Gin binding error. If extraction fails, returns the translation for defaultKey.
func bindErrorMessage(err error, lang, defaultKey string) string {
	// Gin binding error messages already include the field name; pass through directly.
	if err != nil {
		return err.Error()
	}
	return lib_i18n.Show(lang, defaultKey)
}
