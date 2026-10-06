package handlers

// @title                      eyeful API
// @version                    0.1.0
// @description                Multi-agent code review with executable evidence.
// @BasePath                   /
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                "Bearer eyf_…" API key for the CLI and CI
// @securityDefinitions.apikey SessionCookie
// @in                         cookie
// @name                       eyeful_session
// @description                Browser session set by sign-in

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/swaggo/swag/v2"

	_ "github.com/Pleasurecruise/eyeful/spec"
)

type DocsRoutes struct{}

var PublicRoutes = []string{
	"GET /health",
	"GET /ready",
	"GET /api/openapi.json",
	"GET /api/docs",
	"GET /oauth/providers",
	"GET /oauth/:provider/authorize",
	"GET /oauth/:provider/callback",
}

func (DocsRoutes) Register(g *echo.Group) {
	g.GET("/api/openapi.json", spec)
	g.GET("/api/docs", ui)
}

func spec(c *echo.Context) error {
	doc, err := swag.ReadDoc()
	if err != nil {
		return fmt.Errorf("read openapi document: %w", err)
	}
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, []byte(doc))
}

func ui(c *echo.Context) error { return c.HTML(http.StatusOK, swaggerUI) }

const swaggerUI = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>eyeful API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>window.onload = () => SwaggerUIBundle({ url: "/api/openapi.json", dom_id: "#swagger-ui" });</script>
</body>
</html>`
