package validate

import (
	"regexp"
	"strings"
)

const ClientLoginCallbackURI = `teamsboard://auth/callback`

var clientLoginStateRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
var clientLoginTicketRe = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

// ClientLoginCreateParams is the unauthenticated create-session input.
type ClientLoginCreateParams struct {
	State       string
	RedirectURI string
}

// ClientLoginConfirmParams is the authenticated confirm-session input.
type ClientLoginConfirmParams struct {
	State string
}

// ClientLoginExchangeParams is the unauthenticated ticket-exchange input.
type ClientLoginExchangeParams struct {
	State  string
	Ticket string
}

// ValidateClientLoginCreate checks state and the desktop callback URI.
func ValidateClientLoginCreate(params ClientLoginCreateParams) error {
	if err := validateClientLoginState(params.State); err != nil {
		return err
	}
	if strings.TrimSpace(params.RedirectURI) != ClientLoginCallbackURI {
		return &FieldError{Field: "redirect_uri", Key: "client_login_redirect_invalid"}
	}
	return nil
}

// ValidateClientLoginConfirm checks the state carried by the confirmation page.
func ValidateClientLoginConfirm(params ClientLoginConfirmParams) error {
	return validateClientLoginState(params.State)
}

// ValidateClientLoginExchange checks state and the one-time ticket together.
func ValidateClientLoginExchange(params ClientLoginExchangeParams) error {
	if err := validateClientLoginState(params.State); err != nil {
		return err
	}
	ticket := strings.TrimSpace(params.Ticket)
	if !clientLoginTicketRe.MatchString(ticket) {
		return &FieldError{Field: "ticket", Key: "client_login_ticket_invalid"}
	}
	return nil
}

func validateClientLoginState(state string) error {
	value := strings.TrimSpace(state)
	if !clientLoginStateRe.MatchString(value) {
		return &FieldError{Field: "state", Key: "common_param_invalid"}
	}
	return nil
}
