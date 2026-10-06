package settings

import (
	"errors"
)

type Settings struct {
	Agent      string `json:"agent,omitempty"`
	Repository string `json:"repository,omitempty"`
}

var (
	ErrNoAgent = errors.New("no agent connected; run `eyeful connect claude`, `codex` or `pi`")
)
