package review

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case StatusQueued, StatusRunning, StatusDone:
		return st, nil
	}
	return "", fmt.Errorf("unknown review status %q", s)
}

func ParseResult(s string) (Result, error) {
	switch r := Result(s); r {
	case "", ResultComplete, ResultNone:
		return r, nil
	}
	return "", fmt.Errorf("unknown review result %q", s)
}

func (o Outcome) Validate() error {
	switch {
	case o.Result != ResultComplete && o.Result != ResultNone:
		return fmt.Errorf("finish with result %q", o.Result)
	case o.Result == ResultNone && o.Reason == "":
		return errors.New("finish with no result and no reason")
	}
	return nil
}

func (s Source) Validate() error {
	owner, name, ok := strings.Cut(s.Repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("%w: repo %q is not owner/name", ErrInvalidSource, s.Repo)
	}
	if s.Base == "" || s.Head == "" {
		return fmt.Errorf("%w: base and head branches are required", ErrInvalidSource)
	}
	if s.Base == s.Head {
		return fmt.Errorf("%w: base and head are both %q", ErrInvalidSource, s.Base)
	}
	return nil
}

func (r Request) Validate() error {
	if r.UserID == "" {
		return errors.New("request without a user")
	}
	return r.Source.Validate()
}

func NewID() string {
	return "r_" + strings.ToLower(rand.Text()[:16])
}
