package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/review"
)

type Admit interface {
	Create(ctx context.Context, req review.Request) (review.Review, bool, error)
}

type Webhook struct {
	Secret     string
	Configured bool
	Admit      Admit
}

type queuedResponse struct {
	ReviewID string `json:"review_id"`
	Created  bool   `json:"created"`
}

type ignoredResponse struct {
	Ignored string `json:"ignored"`
}

func (h Webhook) Register(g *echo.Group) {
	g.POST("/webhook", h.post)
}

func (h Webhook) post(c *echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return apperror.New(apperror.CodeBadRequest, http.StatusBadRequest, "the webhook body could not be read", err)
	}
	if h.Secret == "" {
		return apperror.New(apperror.CodeInternal, http.StatusInternalServerError, "the webhook secret is not configured", nil)
	}
	if !github.Verify(body, c.Request().Header.Get("X-Hub-Signature-256"), h.Secret) {
		return apperror.New(apperror.CodeUnauthorized, http.StatusUnauthorized, "the webhook signature is invalid", nil)
	}
	if !h.Configured {
		return apperror.New(apperror.CodeInternal, http.StatusInternalServerError, "the GitHub App is not configured", nil)
	}
	delivery, err := github.Parse(c.Request().Header.Get("X-GitHub-Event"), body)
	if err != nil {
		return apperror.New(apperror.CodeBadRequest, http.StatusBadRequest, "the webhook payload is malformed", err)
	}
	if delivery.Ignore != "" {
		return c.JSON(http.StatusOK, ignoredResponse{Ignored: delivery.Ignore})
	}
	row, created, err := h.Admit.Create(c.Request().Context(), github.Admission(delivery.Pull))
	switch {
	case errors.Is(err, review.ErrIdempotencyConflict):
		return apperror.New(apperror.CodeConflict, http.StatusConflict, "this delivery does not match the review already queued for that commit", err)
	case errors.Is(err, review.ErrInvalidSource):
		return apperror.New(apperror.CodeBadRequest, http.StatusBadRequest, "the pull request could not be queued", err)
	case err != nil:
		return apperror.New(apperror.CodeInternal, http.StatusInternalServerError, "the review could not be queued", err)
	}
	return c.JSON(http.StatusOK, queuedResponse{ReviewID: row.ID, Created: created})
}
