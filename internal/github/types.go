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

const (
	listPage     = 30
	bodyLimit    = 16 << 20
	commentLimit = 65536
	markerRoom   = 256
)

var ErrTooLong = errors.New("longer than a GitHub comment can be")
