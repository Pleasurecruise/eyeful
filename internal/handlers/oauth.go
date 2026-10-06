package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"golang.org/x/oauth2"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
)

type OAuthRoutes struct {
	Providers     []OAuthProvider
	SecureCookies bool
	RateLimits    []echo.MiddlewareFunc
}

const oauthCookie = "eyeful_oauth"

func (h OAuthRoutes) Register(g *echo.Group) {
	g.GET("/oauth/providers", h.list)
	g.GET("/oauth/:provider/authorize", h.authorize, h.RateLimits...)
	g.GET("/oauth/:provider/callback", h.callback, h.RateLimits...)
}

// list godoc
//
//	@ID			listOAuthProviders
//	@Summary	Sign-in providers this server is configured for
//	@Tags		oauth
//	@Produce	json
//	@Success	200	{object}	OAuthProviderList
//	@Failure	500	{object}	apperror.Problem
//	@Router		/oauth/providers [get]
func (h OAuthRoutes) list(c *echo.Context) error {
	items := make([]OAuthProviderResponse, len(h.Providers))
	for i, p := range h.Providers {
		items[i] = OAuthProviderResponse{ID: p.Provider().ID, Title: p.Provider().Title}
	}
	return c.JSON(http.StatusOK, OAuthProviderList{Items: items})
}

// authorize godoc
//
//	@ID			authorizeOAuth
//	@Summary	Redirect to the provider to sign in
//	@Tags		oauth
//	@Param		provider	path	string	true	"Provider"	Enums(github, gitee)
//	@Success	302
//	@Failure	404	{object}	apperror.Problem
//	@Router		/oauth/{provider}/authorize [get]
func (h OAuthRoutes) authorize(c *echo.Context) error {
	provider, err := h.find(c.Param("provider"))
	if err != nil {
		return err
	}
	state, verifier := rand.Text(), oauth2.GenerateVerifier()
	c.SetCookie(&http.Cookie{
		Name:     oauthCookie,
		Value:    state + "." + verifier,
		Path:     "/oauth/" + provider.Provider().ID + "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	return c.Redirect(http.StatusFound, provider.AuthCodeURL(state, verifier))
}

// callback godoc
//
//	@ID			callbackOAuth
//	@Summary	The provider redirects here; sets the session and returns to the console
//	@Tags		oauth
//	@Param		provider	path	string	true	"Provider"	Enums(github, gitee)
//	@Param		code		query	string	true	"Authorization code"
//	@Param		state		query	string	true	"State from the authorize redirect"
//	@Success	302
//	@Failure	400	{object}	apperror.Problem
//	@Failure	404	{object}	apperror.Problem
//	@Router		/oauth/{provider}/callback [get]
func (h OAuthRoutes) callback(c *echo.Context) error {
	provider, err := h.find(c.Param("provider"))
	if err != nil {
		return err
	}
	cookie, err := c.Cookie(oauthCookie)
	if err != nil {
		return apperror.BadRequest("sign-in expired; start again", nil)
	}
	state, verifier, ok := strings.Cut(cookie.Value, ".")
	if !ok || subtle.ConstantTimeCompare([]byte(state), []byte(c.QueryParam("state"))) != 1 {
		return apperror.BadRequest("state mismatch; start again", nil)
	}
	c.SetCookie(&http.Cookie{Name: oauthCookie, Path: cookie.Path, MaxAge: -1, HttpOnly: true, Secure: h.SecureCookies, SameSite: http.SameSiteLaxMode})
	_, token, err := provider.Callback(c.Request().Context(), c.QueryParam("code"), verifier, clientInfo(c))
	if errors.Is(err, auth.ErrNoVerifiedEmail) {
		return apperror.BadRequest("your "+provider.Provider().Title+" account needs a verified primary email", nil)
	}
	if err != nil {
		return err
	}
	setSessionCookie(c, h.SecureCookies, token.Token, token.ExpiresAt)
	return c.Redirect(http.StatusFound, "/")
}

func (h OAuthRoutes) find(id string) (OAuthProvider, error) {
	for _, p := range h.Providers {
		if p.Provider().ID == id {
			return p, nil
		}
	}
	return nil, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "sign-in provider "+id+" is not configured", nil)
}
