package controller

import (
	"errors"
	"net/http"
	"strings"

	"goteams/internal/app/goteams/business"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

type createClientLoginSessionRequest struct {
	State       string `json:"state"`
	RedirectURI string `json:"redirect_uri"`
}

type createClientLoginSessionResponse struct {
	State     string `json:"state"`
	LoginURL  string `json:"login_url"`
	ExpiresIn int    `json:"expires_in"`
}

type confirmClientLoginSessionRequest struct {
	State string `json:"state"`
}

type confirmClientLoginSessionResponse struct {
	CallbackURL string `json:"callback_url"`
}

type exchangeClientLoginTicketRequest struct {
	State  string `json:"state"`
	Ticket string `json:"ticket"`
}

// CreateClientLoginSession registers a desktop browser-login handshake.
func CreateClientLoginSession(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req createClientLoginSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `client.login_session.bind`, err)
		return
	}
	if err := validate.ValidateClientLoginCreate(validate.ClientLoginCreateParams{
		State:       req.State,
		RedirectURI: req.RedirectURI,
	}); err != nil {
		key := define.MsgParamInvalid
		var fe *validate.FieldError
		if errors.As(err, &fe) && fe != nil {
			key = fe.Key
		}
		abortBadRequest(c, lib_i18n.Show(lang, key), `client.login_session.validate`, err)
		return
	}
	created, err := business.CreateBrowserLoginSession(req.State, req.RedirectURI, requestSiteURL(c))
	if err != nil {
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), `client.login_session.create`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: createClientLoginSessionResponse{
			State:     created.State,
			LoginURL:  created.LoginURL,
			ExpiresIn: created.ExpiresIn,
		},
	})
}

// ConfirmClientLoginSession lets the signed-in web user authorize the desktop client.
func ConfirmClientLoginSession(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req confirmClientLoginSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `client.login_session.confirm.bind`, err)
		return
	}
	if err := validate.ValidateClientLoginConfirm(validate.ClientLoginConfirmParams{State: req.State}); err != nil {
		abortBadRequest(c, fieldErrorMessage(err, lang, define.MsgParamInvalid), `client.login_session.confirm.validate`, err)
		return
	}
	confirmed, err := business.ConfirmBrowserLoginSession(req.State, cast.ToInt(c.GetInt(`userId`)))
	if err != nil {
		writeClientLoginError(c, lang, `client.login_session.confirm`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    confirmClientLoginSessionResponse{CallbackURL: confirmed.CallbackURL},
	})
}

// ExchangeClientLoginTicket turns a one-time ticket into a desktop session, same shape as password login.
func ExchangeClientLoginTicket(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	var req exchangeClientLoginTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortBadRequest(c, bindErrorMessage(err, lang, define.MsgParamInvalid), `client.login_session.exchange.bind`, err)
		return
	}
	if err := validate.ValidateClientLoginExchange(validate.ClientLoginExchangeParams{
		State:  req.State,
		Ticket: req.Ticket,
	}); err != nil {
		key := define.MsgClientLoginTicketInvalid
		var fe *validate.FieldError
		if errors.As(err, &fe) && fe != nil {
			key = fe.Key
		}
		abortBadRequest(c, lib_i18n.Show(lang, key), `client.login_session.exchange.validate`, err)
		return
	}
	model, err := business.ExchangeBrowserLoginTicket(req.State, req.Ticket, c.ClientIP(), c.GetHeader(`User-Agent`))
	if err != nil {
		writeClientLoginError(c, lang, `client.login_session.exchange`, err)
		return
	}
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data:    toLoginResponse(model),
	})
}

// GetClientLoginSessionStatus reports the handshake phase for the confirmation
// page poll: pending / confirmed / exchanged / expired. Only the authorizing
// user's own states are visible; anything else reports expired.
func GetClientLoginSessionStatus(c *gin.Context) {
	lang := lib_i18n.GetLang(c)
	state := strings.TrimSpace(c.Param(`state`))
	if state == `` {
		abortBadRequest(c, lib_i18n.Show(lang, define.MsgParamInvalid), `client.login_session.status.param`, nil)
		return
	}
	userID := cast.ToInt(c.GetInt(`userId`))
	c.JSON(http.StatusOK, define.Response{
		Code:    define.CodeSuccess,
		Message: lib_i18n.Show(lang, define.MsgSuccess),
		Data: map[string]string{
			`status`: business.GetBrowserLoginStatus(state, userID),
		},
	})
}

func writeClientLoginError(c *gin.Context, lang, stage string, err error) {
	switch {
	case errors.Is(err, business.ErrClientLoginSessionNotFound):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgClientLoginSessionNotFound))
	case errors.Is(err, business.ErrClientLoginTicketInvalid):
		c.JSON(http.StatusUnauthorized, define.Response{
			Code:    http.StatusUnauthorized,
			Message: lib_i18n.Show(lang, define.MsgClientLoginTicketInvalid),
		})
	case errors.Is(err, business.ErrAccountDisabled):
		c.JSON(http.StatusUnauthorized, define.Response{
			Code:    http.StatusUnauthorized,
			Message: lib_i18n.Show(lang, define.MsgAccountDisabled),
		})
	case errors.Is(err, business.ErrUserNotExist):
		abortNotFound(c, lib_i18n.Show(lang, define.MsgUserNotFound))
	default:
		abortInternal(c, lib_i18n.Show(lang, define.MsgError), stage, err)
	}
}

func requestSiteURL(c *gin.Context) string {
	if base := strings.TrimRight(define.Config.WebService[`site_url`], `/`); base != `` {
		return base
	}
	scheme := `http`
	if proto := strings.TrimSpace(c.GetHeader(`X-Forwarded-Proto`)); proto != `` {
		scheme = strings.TrimSpace(strings.Split(proto, `,`)[0])
	} else if c.Request.TLS != nil {
		scheme = `https`
	}
	return scheme + `://` + c.Request.Host
}
