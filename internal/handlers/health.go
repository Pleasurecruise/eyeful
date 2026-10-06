package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
)

type HealthRoutes struct {
	Version string
	Ping    func(context.Context) error
}

func (h HealthRoutes) Register(g *echo.Group) {
	g.GET("/health", h.get)
	g.GET("/ready", h.ready)
}

// ready godoc
//
//	@ID			getReadiness
//	@Summary	Readiness: the database answers
//	@Tags		system
//	@Produce	json
//	@Success	200	{object}	HealthResponse
//	@Failure	503	{object}	apperror.Problem
//	@Router		/ready [get]
func (h HealthRoutes) ready(c *echo.Context) error {
	if err := h.Ping(c.Request().Context()); err != nil {
		return apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "database unavailable", err)
	}
	return c.JSON(http.StatusOK, HealthResponse{Status: "ready", Version: h.Version})
}

// get godoc
//
//	@ID			getHealth
//	@Summary	Liveness
//	@Tags		system
//	@Produce	json
//	@Success	200	{object}	HealthResponse
//	@Router		/health [get]
func (h HealthRoutes) get(c *echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{Status: "ok", Version: h.Version})
}
