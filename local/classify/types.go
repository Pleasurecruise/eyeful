package classify

import (
	"errors"
	"log/slog"
	"net/http"

	jev "github.com/mattn/go-jev"
)

type Jev struct {
	log    *slog.Logger
	url    string
	header http.Header
	client *http.Client
}

type finding struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Subject    string `json:"subject"`
	Discussion string `json:"discussion"`
}

type input struct {
	State     map[string]map[string]finding `json:"state"`
	Questions map[string]jev.Question       `json:"questions"`
}

type reply struct {
	Result jev.Response `json:"result"`
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

const (
	sameQuestion = "Two code reviewers each wrote a finding about the same change. Do `findings.%s` and `findings.%s` report the same underlying problem, so that one fix would resolve both? Answer no when they are different problems that only sit on nearby lines."
)

var (
	ErrNotConfigured = errors.New("Jev is not configured: set EYEFUL_CLOUDFLARE_ACCOUNT_ID and EYEFUL_AI_GATEWAY_TOKEN")
	ErrRejected      = errors.New("jev rejected the request")
	ErrUnanswered    = errors.New("jev left a question unanswered")
)
