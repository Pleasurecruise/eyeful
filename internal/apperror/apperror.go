package apperror

import (
	"errors"
	"net/http"
)

type Code string // @name ErrorCode

const (
	CodeBadRequest          Code = "bad_request"
	CodeUnauthenticated     Code = "unauthenticated"
	CodeNotFound            Code = "not_found"
	CodeMethodNotAllowed    Code = "method_not_allowed"
	CodeInternal            Code = "internal"
	CodeUnavailable         Code = "unavailable"
	CodeReviewNotFound      Code = "review_not_found"
	CodeIdempotencyConflict Code = "idempotency_conflict"
	CodeRateLimited         Code = "rate_limited"
	CodeInvalidCredentials  Code = "invalid_credentials"
	CodeEmailTaken          Code = "email_taken"
	CodeInvalidEmail        Code = "invalid_email"
	CodeWeakPassword        Code = "weak_password"
	CodeAPIKeyNotFound      Code = "api_key_not_found"
	CodeSessionNotFound     Code = "session_not_found"
	CodeReviewRunning       Code = "review_running"
	CodeOriginForbidden     Code = "origin_forbidden"
	CodeRepositoryNotFound  Code = "repository_not_found"
	CodeProviderNotLinked   Code = "provider_not_linked"
	CodeProviderExpired     Code = "provider_expired"
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

func BadRequest(detail string, cause error) *Error {
	return New(CodeBadRequest, http.StatusBadRequest, detail, cause)
}

func Conflict(code Code, detail string) *Error {
	return New(code, http.StatusConflict, detail, nil)
}

type Problem struct {
	Type      string `json:"type" example:"about:blank" validate:"required"`
	Title     string `json:"title" example:"Not Found" validate:"required"`
	Status    int    `json:"status" example:"404" validate:"required"`
	Code      Code   `json:"code" example:"review_not_found" validate:"required"`
	Detail    string `json:"detail,omitempty" example:"review r_123 does not exist"`
	RequestID string `json:"request_id,omitempty"`
} // @name Problem

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
