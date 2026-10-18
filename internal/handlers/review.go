package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/harness"
)

type ReviewRoutes struct {
	Status func() harness.Status
}

func (h ReviewRoutes) Register(g *echo.Group) {
	g.GET("/review", h.get)
}

func (h ReviewRoutes) get(c *echo.Context) error {
	return c.JSON(http.StatusOK, h.Status())
}
