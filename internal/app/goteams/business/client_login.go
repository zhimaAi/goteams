package business

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_redis"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

const (
	clientLoginTTL          = 10 * time.Minute
	clientLoginStateKey     = `goteams:client_login:state:`
	clientLoginTicketKey    = `goteams:client_login:ticket:`
	clientLoginExchangedKey = `goteams:client_login:exchanged:`
	clientLoginTicketBytes  = 32
)

// ErrClientLoginSessionNotFound indicates the desktop login handshake expired or never existed.
var ErrClientLoginSessionNotFound = errors.New(`client login session not found`)

// ErrClientLoginTicketInvalid indicates the ticket is missing, mismatched, or already used.
var ErrClientLoginTicketInvalid = errors.New(`client login ticket invalid`)

// BrowserLoginCreated is returned after the desktop registers a login handshake.
type BrowserLoginCreated struct {
	State     string
	LoginURL  string
	ExpiresIn int
}

// BrowserLoginConfirmed is returned after the signed-in web user authorizes the desktop.
type BrowserLoginConfirmed struct {
	CallbackURL string
}

type browserLoginSession struct {
	RedirectURI string `json:"redirect_uri"`
	Ticket      string `json:"ticket"`
	UserID      int    `json:"user_id"`
}

type browserLoginTicket struct {
	State  string `json:"state"`
	UserID int    `json:"user_id"`
}

// CreateBrowserLoginSession stores a short-lived handshake and returns the browser page URL.
func CreateBrowserLoginSession(state, redirectURI, siteURL string) (BrowserLoginCreated, error) {
	state = strings.TrimSpace(state)
	payload, err := json.Marshal(browserLoginSession{RedirectURI: strings.TrimSpace(redirectURI)})
	if err != nil {
		return BrowserLoginCreated{}, err
	}
	if err = lib_redis.Set(define.Redis, clientLoginStateKey+state, string(payload), clientLoginTTL); err != nil {
		return BrowserLoginCreated{}, err
	}
	loginURL := strings.TrimRight(strings.TrimSpace(siteURL), `/`) + `/client-login?state=` + url.QueryEscape(state)
	return BrowserLoginCreated{
		State:     state,
		LoginURL:  loginURL,
		ExpiresIn: int(clientLoginTTL.Seconds()),
	}, nil
}

// ConfirmBrowserLoginSession binds the current web user to the handshake and issues a one-time ticket.
func ConfirmBrowserLoginSession(state string, userID int) (BrowserLoginConfirmed, error) {
	state = strings.TrimSpace(state)
	if _, err := loadActiveUser(userID); err != nil {
		return BrowserLoginConfirmed{}, err
	}
	sess, err := loadBrowserLoginSession(state)
	if err != nil {
		return BrowserLoginConfirmed{}, err
	}
	ticket, err := newClientLoginTicket()
	if err != nil {
		return BrowserLoginConfirmed{}, err
	}
	if sess.Ticket != `` {
		_ = lib_redis.Del(define.Redis, clientLoginTicketKey+sess.Ticket)
	}
	sess.Ticket = ticket
	sess.UserID = userID
	if err = storeBrowserLoginSession(state, sess); err != nil {
		return BrowserLoginConfirmed{}, err
	}
	ticketPayload, err := json.Marshal(browserLoginTicket{State: state, UserID: userID})
	if err != nil {
		return BrowserLoginConfirmed{}, err
	}
	if err = lib_redis.Set(define.Redis, clientLoginTicketKey+ticket, string(ticketPayload), clientLoginTTL); err != nil {
		return BrowserLoginConfirmed{}, err
	}
	callback, err := url.Parse(validate.ClientLoginCallbackURI)
	if err != nil {
		return BrowserLoginConfirmed{}, err
	}
	query := callback.Query()
	query.Set(`state`, state)
	query.Set(`ticket`, ticket)
	callback.RawQuery = query.Encode()
	return BrowserLoginConfirmed{CallbackURL: callback.String()}, nil
}

// consumeTicketScript atomically reads and deletes a ticket. GETDEL needs Redis 6.2+;
// shared dev still runs Redis 5, so a Lua GET+DEL keeps the one-time guarantee.
const consumeTicketScript = `local v = redis.call('GET', KEYS[1]); if not v then return nil end; redis.call('DEL', KEYS[1]); return v`

// ExchangeBrowserLoginTicket consumes the one-time ticket and issues a desktop session token.
func ExchangeBrowserLoginTicket(state, ticket, ip, ua string) (LoginModel, error) {
	state = strings.TrimSpace(state)
	ticket = strings.TrimSpace(ticket)
	raw, err := consumeClientLoginTicket(ticket)
	if err != nil {
		return LoginModel{}, err
	}
	var record browserLoginTicket
	if err = json.Unmarshal([]byte(raw), &record); err != nil {
		return LoginModel{}, err
	}
	if record.State != state || record.UserID <= 0 {
		logs.Error("client login ticket payload mismatch ticket_len:%d state_len:%d user_id:%d", len(ticket), len(state), record.UserID)
		return LoginModel{}, ErrClientLoginTicketInvalid
	}
	// The state key is kept until the desktop session is actually created; the
	// exchanged marker below is written only after that, so a failed exchange never
	// makes the confirmation page show a false success.
	user, err := loadActiveUser(record.UserID)
	if err != nil {
		return LoginModel{}, err
	}
	token, err := common.CreateSession(common.SessionData{
		UserId:  user.Id,
		AdminId: user.AdminId,
		Ip:      ip,
		Ua:      ua,
	})
	if err != nil {
		return LoginModel{}, err
	}
	// The state key becomes the exchanged marker so the confirmation page can poll
	// for the client-side completion instead of being closed blind. The marker value
	// carries the authorizing user ID so the poll can be scoped to that user.
	if err = lib_redis.Set(define.Redis, clientLoginExchangedKey+state, strconv.Itoa(user.Id), clientLoginTTL); err != nil {
		// Best-effort: the desktop login already succeeded; only the page feedback degrades.
		logs.Error("client login exchanged marker failed state_len:%d err:%s", len(state), err.Error())
	}
	_ = lib_redis.Del(define.Redis, clientLoginStateKey+state)
	return LoginModel{User: user, Token: token}, nil
}

func loadBrowserLoginSession(state string) (browserLoginSession, error) {
	raw, err := lib_redis.Get(define.Redis, clientLoginStateKey+state)
	if errors.Is(err, redis.Nil) || raw == `` {
		return browserLoginSession{}, ErrClientLoginSessionNotFound
	}
	if err != nil {
		return browserLoginSession{}, err
	}
	var sess browserLoginSession
	if err = json.Unmarshal([]byte(raw), &sess); err != nil {
		return browserLoginSession{}, err
	}
	if strings.TrimSpace(sess.RedirectURI) == `` {
		return browserLoginSession{}, ErrClientLoginSessionNotFound
	}
	return sess, nil
}

func storeBrowserLoginSession(state string, sess browserLoginSession) error {
	payload, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return lib_redis.Set(define.Redis, clientLoginStateKey+state, string(payload), clientLoginTTL)
}

func loadActiveUser(userID int) (UserModel, error) {
	m := msql.Model(define.TableUsers, define.Postgres)
	row, err := m.Field(userTokenFields).Where(`id`, cast.ToString(userID)).Where(`deleted_at`, `0`).Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return UserModel{}, err
	}
	if len(row) == 0 {
		return UserModel{}, ErrUserNotExist
	}
	if cast.ToInt(row[`status`]) == define.MemberStatusDisabled {
		return UserModel{}, ErrAccountDisabled
	}
	return toUserModel(row)
}

func consumeClientLoginTicket(ticket string) (string, error) {
	raw, err := define.Redis.Eval(context.Background(), consumeTicketScript, []string{clientLoginTicketKey + ticket}).Text()
	if errors.Is(err, redis.Nil) {
		return ``, ErrClientLoginTicketInvalid
	}
	if err != nil {
		return ``, err
	}
	if raw == `` {
		return ``, ErrClientLoginTicketInvalid
	}
	return raw, nil
}

func newClientLoginTicket() (string, error) {
	buf := make([]byte, clientLoginTicketBytes)
	if _, err := rand.Read(buf); err != nil {
		return ``, fmt.Errorf(`generate client login ticket: %w`, err)
	}
	return hex.EncodeToString(buf), nil
}

// Browser login handshake statuses observed by the confirmation page.
const (
	BrowserLoginStatusPending   = `pending`   // created, the web user has not confirmed yet
	BrowserLoginStatusConfirmed = `confirmed` // confirmed, waiting for the desktop to exchange the ticket
	BrowserLoginStatusExchanged = `exchanged` // the desktop completed the login
	BrowserLoginStatusExpired   = `expired`   // handshake gone (timeout or never existed)
)

// GetBrowserLoginStatus reports the handshake phase for the confirmation page
// poll. userID scopes the check to the authorizing user: a marker or session
// bound to another user is reported as expired so states cannot be probed
// across accounts.
func GetBrowserLoginStatus(state string, userID int) string {
	state = strings.TrimSpace(state)
	if marker, err := lib_redis.Get(define.Redis, clientLoginExchangedKey+state); err == nil && marker != `` {
		if cast.ToInt(marker) != userID {
			return BrowserLoginStatusExpired
		}
		return BrowserLoginStatusExchanged
	}
	sess, err := loadBrowserLoginSession(state)
	if err != nil {
		return BrowserLoginStatusExpired
	}
	if sess.UserID > 0 && sess.UserID != userID {
		return BrowserLoginStatusExpired
	}
	if sess.Ticket != `` {
		return BrowserLoginStatusConfirmed
	}
	return BrowserLoginStatusPending
}
