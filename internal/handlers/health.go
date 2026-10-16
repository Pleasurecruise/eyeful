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

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (h HealthRoutes) Register(g *echo.Group) {
	g.GET("/health", h.get)
	g.GET("/ready", h.ready)
}

func (h HealthRoutes) ready(c *echo.Context) error {
	if err := h.Ping(c.Request().Context()); err != nil {
		return apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "database unavailable", err)
	}
	return c.JSON(http.StatusOK, HealthResponse{Status: "ready", Version: h.Version})
}

func (h HealthRoutes) get(c *echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{Status: "ok", Version: h.Version})
}
