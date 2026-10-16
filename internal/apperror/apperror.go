package apperror

import (
	"errors"
	"net/http"
)

type Code string

const (
	CodeBadRequest       Code = "bad_request"
	CodeUnauthorized     Code = "unauthorized"
	CodeConflict         Code = "conflict"
	CodeNotFound         Code = "not_found"
	CodeMethodNotAllowed Code = "method_not_allowed"
	CodeInternal         Code = "internal"
	CodeUnavailable      Code = "unavailable"
)

type Error struct {
	Code   Code
	Status int
	Detail string
	cause  error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return string(e.Code) + ": " + e.Detail + ": " + e.cause.Error()
	}
	return string(e.Code) + ": " + e.Detail
}

func (e *Error) Unwrap() error { return e.cause }

func New(code Code, status int, detail string, cause error) *Error {
	return &Error{Code: code, Status: status, Detail: detail, cause: cause}
}

type Problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      Code   `json:"code"`
	Detail    string `json:"detail,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func ProblemFor(err error, requestID string) Problem {
	var e *Error
	if !errors.As(err, &e) {
		e = New(CodeInternal, http.StatusInternalServerError, "internal error", err)
	}
	return Problem{
		Type:      "about:blank",
		Title:     http.StatusText(e.Status),
		Status:    e.Status,
		Code:      e.Code,
		Detail:    e.Detail,
		RequestID: requestID,
	}
}
