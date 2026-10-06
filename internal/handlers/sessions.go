package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
)

type SessionRoutes struct {
	Sessions      SessionStore
	SecureCookies bool
}

func (h SessionRoutes) Register(g *echo.Group) {
	g.GET("/sessions", h.list)
	g.GET("/sessions/current", h.getCurrent)
	g.DELETE("/sessions/current", h.deleteCurrent)
	g.DELETE("/sessions/:id", h.delete)
}

// list godoc
//
//	@ID			listSessions
//	@Summary	List the signed-in user's active sessions
//	@Tags		sessions
//	@Produce	json
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Success	200	{object}	SessionList
//	@Failure	401	{object}	apperror.Problem
//	@Router		/sessions [get]
func (h SessionRoutes) list(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	sessions, err := h.Sessions.ListSessions(c.Request().Context(), p.User.ID)
	if err != nil {
		return err
	}
	out := SessionList{Items: make([]SessionResponse, len(sessions))}
	for i, s := range sessions {
		out.Items[i] = SessionResponse{ID: s.ID, IP: s.IP, UserAgent: s.UserAgent, CreatedAt: s.CreatedAt.UTC(), ExpiresAt: s.ExpiresAt.UTC()}
	}
	return c.JSON(http.StatusOK, out)
}

// getCurrent godoc
//
//	@ID			getCurrentSession
//	@Summary	Who is calling, and how
//	@Tags		sessions
//	@Produce	json
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Success	200	{object}	CurrentSession
//	@Failure	401	{object}	apperror.Problem
//	@Router		/sessions/current [get]
func (h SessionRoutes) getCurrent(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	via := "session"
	if p.APIKeyID != "" {
		via = "api_key"
	}
	return c.JSON(http.StatusOK, CurrentSession{User: toUser(p.User), Via: via})
}

// deleteCurrent godoc
//
//	@ID			deleteCurrentSession
//	@Summary	Sign out of this browser
//	@Tags		sessions
//	@Security	SessionCookie
//	@Success	204
//	@Failure	401	{object}	apperror.Problem
//	@Router		/sessions/current [delete]
func (h SessionRoutes) deleteCurrent(c *echo.Context) error {
	if cookie, err := c.Cookie(auth.SessionCookie); err == nil {
		if err := h.Sessions.DeleteSessionByToken(c.Request().Context(), cookie.Value); err != nil {
			return err
		}
	}
	setSessionCookie(c, h.SecureCookies, "", time.Unix(0, 0))
	return c.NoContent(http.StatusNoContent)
}

// delete godoc
//
//	@ID			deleteSession
//	@Summary	Sign out another device
//	@Tags		sessions
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Session ID"
//	@Success	204
//	@Failure	401	{object}	apperror.Problem
//	@Failure	404	{object}	apperror.Problem
//	@Router		/sessions/{id} [delete]
func (h SessionRoutes) delete(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	err = h.Sessions.DeleteSession(c.Request().Context(), p.User.ID, c.Param("id"))
	if errors.Is(err, auth.ErrSessionNotFound) {
		return apperror.New(apperror.CodeSessionNotFound, http.StatusNotFound, "session not found", nil)
	}
	if err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func setSessionCookie(c *echo.Context, secure bool, token string, expires time.Time) {
	c.SetCookie(&http.Cookie{
		Name:     auth.SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clientInfo(c *echo.Context) auth.ClientInfo {
	return auth.ClientInfo{IP: c.RealIP(), UserAgent: c.Request().UserAgent()}
}

func principal(c *echo.Context) (auth.Principal, error) {
	p, ok := auth.PrincipalFrom(c.Request().Context())
	if !ok {
		return auth.Principal{}, apperror.New(apperror.CodeUnauthenticated, http.StatusUnauthorized, "sign in or send an API key", nil)
	}
	return p, nil
}
