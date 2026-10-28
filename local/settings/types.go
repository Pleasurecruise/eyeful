package settings

import (
	"errors"
)

type Settings struct {
	Agent       string `json:"agent,omitempty"`
	Repository  string `json:"repository,omitempty"`
	TypeSafeKey string `json:"typesafe_api_key,omitempty"`
}

var (
	ErrNoAgent = errors.New("no agent connected; run `eyeful connect claude`, `codex` or `pi`")
)
