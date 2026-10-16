package github

import "errors"

type Pull struct {
	Owner          string
	Name           string
	Number         int64
	Draft          bool
	BaseRef        string
	HeadRef        string
	BaseSHA        string
	HeadSHA        string
	CloneURL       string
	InstallationID int64
}

type Delivery struct {
	Ignore string
	Pull   Pull
}

type Actions struct {
	Passed bool
	Log    string
}

type Note struct {
	Path string
	Line int
	Body string
}

var (
	ErrNoKey     = errors.New("GitHub App private key is not configured")
	ErrMalformed = errors.New("malformed webhook")
)
