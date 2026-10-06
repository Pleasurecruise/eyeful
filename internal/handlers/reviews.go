package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/review"
)

type ReviewRoutes struct {
	Store            ReviewStore
	CreateRateLimits []echo.MiddlewareFunc
}

func (h ReviewRoutes) Register(g *echo.Group) {
	g.POST("/reviews", h.create, h.CreateRateLimits...)
	g.GET("/reviews", h.list)
	g.GET("/reviews/:id", h.get)
	g.DELETE("/reviews/:id", h.delete)
	// TODO(api): GET /reviews/:id/events (SSE progress).
	// TODO(api): GET /reviews/:id/report?format=md|json.
	// TODO(api): delivering a review to more sinks.
	// TODO(api): POST /findings/:id/approve and /reject.
}

// create godoc
//
//	@ID			createReview
//	@Summary		Queue a review
//	@Description	Returns 202 with the queued review. A review cannot be stopped once it runs, so clients confirm before creating one. Repeating a request with the same Idempotency-Key returns the first review; reusing the key with a different body is a 409.
//	@Tags			reviews
//	@Accept			json
//	@Produce		json
//	@Security		SessionCookie
//	@Security		BearerAuth
//	@Param			Idempotency-Key	header		string				false	"Makes retries safe"
//	@Param			body			body		CreateReviewRequest	true	"What to review"
//	@Success		202				{object}	ReviewResponse
//	@Failure		400				{object}	apperror.Problem
//	@Failure		401				{object}	apperror.Problem
//	@Failure		409				{object}	apperror.Problem
//	@Failure		429				{object}	apperror.Problem
//	@Router			/reviews [post]
func (h ReviewRoutes) create(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	var body CreateReviewRequest
	if err := c.Bind(&body); err != nil {
		return apperror.BadRequest("malformed JSON body", err)
	}
	req := review.Request{
		UserID:         p.User.ID,
		Source:         review.Source{Repo: body.Source.Repo, Base: body.Source.Base, Head: body.Source.Head},
		Note:           body.Note,
		IdempotencyKey: c.Request().Header.Get("Idempotency-Key"),
	}
	r, created, err := h.Store.Create(c.Request().Context(), req)
	if err != nil {
		return reviewError(err, "")
	}
	if !created {
		c.Response().Header().Set("Idempotent-Replayed", "true")
	}
	return c.JSON(http.StatusAccepted, toResponse(r))
}

// list godoc
//
//	@ID			listReviews
//	@Summary	List reviews, newest first
//	@Tags		reviews
//	@Produce	json
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Param		limit	query		int	false	"At most this many (1-100)"	default(20)
//	@Success	200		{object}	ReviewList
//	@Failure	400		{object}	apperror.Problem
//	@Failure	401		{object}	apperror.Problem
//	@Router		/reviews [get]
func (h ReviewRoutes) list(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	limit := int32(20)
	if s := c.QueryParam("limit"); s != "" {
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil || n < 1 || n > 100 {
			return apperror.BadRequest("limit must be an integer from 1 to 100", err)
		}
		limit = int32(n)
	}
	rs, err := h.Store.List(c.Request().Context(), p.User.ID, limit)
	if err != nil {
		return err
	}
	out := ReviewList{Items: make([]ReviewResponse, len(rs))}
	for i, r := range rs {
		out.Items[i] = toResponse(r)
	}
	return c.JSON(http.StatusOK, out)
}

// get godoc
//
//	@ID			getReview
//	@Summary	Get one review
//	@Tags		reviews
//	@Produce	json
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Review ID"
//	@Success	200	{object}	ReviewResponse
//	@Failure	401	{object}	apperror.Problem
//	@Failure	404	{object}	apperror.Problem
//	@Router		/reviews/{id} [get]
func (h ReviewRoutes) get(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	id := c.Param("id")
	r, err := h.Store.Get(c.Request().Context(), p.User.ID, id)
	if err != nil {
		return reviewError(err, id)
	}
	return c.JSON(http.StatusOK, toResponse(r))
}

// delete godoc
//
//	@ID			deleteReview
//	@Summary	Delete a queued or done review
//	@Tags		reviews
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Review ID"
//	@Success	204
//	@Failure	401	{object}	apperror.Problem
//	@Failure	404	{object}	apperror.Problem
//	@Failure	409	{object}	apperror.Problem
//	@Router		/reviews/{id} [delete]
func (h ReviewRoutes) delete(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	id := c.Param("id")
	if err := h.Store.Delete(c.Request().Context(), p.User.ID, id); err != nil {
		return reviewError(err, id)
	}
	return c.NoContent(http.StatusNoContent)
}

func reviewError(err error, id string) error {
	switch {
	case errors.Is(err, review.ErrNotFound):
		return apperror.New(apperror.CodeReviewNotFound, http.StatusNotFound, "review "+id+" does not exist", nil)
	case errors.Is(err, review.ErrRunning):
		return apperror.Conflict(apperror.CodeReviewRunning, "review "+id+" is running; it can be deleted once it is done")
	case errors.Is(err, review.ErrIdempotencyConflict):
		return apperror.Conflict(apperror.CodeIdempotencyConflict, "Idempotency-Key was used with a different request body")
	case errors.Is(err, review.ErrInvalidSource):
		return apperror.BadRequest(err.Error(), nil)
	}
	return err
}

func toResponse(r review.Review) ReviewResponse {
	return ReviewResponse{
		ID:         r.ID,
		Source:     SourceBody{Repo: r.Source.Repo, Base: r.Source.Base, Head: r.Source.Head},
		Note:       r.Note,
		Status:     string(r.Status),
		Result:     string(r.Result),
		Reason:     r.Reason,
		Attempts:   r.Attempts,
		CreatedAt:  r.CreatedAt.UTC(),
		StartedAt:  optionalTime(r.StartedAt),
		FinishedAt: optionalTime(r.FinishedAt),
	}
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}
