package github

import "errors"

type Actions struct {
	Passed bool
	Log    string
}

type Note struct {
	Path string
	Line int
	Body string
}

var ErrNoKey = errors.New("GitHub App private key is not configured")
