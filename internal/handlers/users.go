package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
)

type UserRoutes struct {
	Users         UserStore
	SecureCookies bool
}

func (h UserRoutes) Register(g *echo.Group) {
	g.GET("/users/me", h.getMe)
	g.PATCH("/users/me", h.updateMe)
	g.DELETE("/users/me", h.deleteMe)
}

// getMe godoc
//
//	@ID			getCurrentUser
//	@Summary	The signed-in user
//	@Tags		users
//	@Produce	json
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Success	200	{object}	UserResponse
//	@Failure	401	{object}	apperror.Problem
//	@Router		/users/me [get]
func (h UserRoutes) getMe(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toUser(p.User))
}

// updateMe godoc
//
//	@ID			updateCurrentUser
//	@Summary	Change the signed-in user's name
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Param		body	body		UpdateUserRequest	true	"New name"
//	@Success	200		{object}	UserResponse
//	@Failure	400		{object}	apperror.Problem
//	@Failure	401		{object}	apperror.Problem
//	@Router		/users/me [patch]
func (h UserRoutes) updateMe(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	var body UpdateUserRequest
	if err := c.Bind(&body); err != nil {
		return apperror.BadRequest("malformed JSON body", err)
	}
	user, err := h.Users.UpdateUser(c.Request().Context(), p.User.ID, body.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toUser(user))
}

// deleteMe godoc
//
//	@ID			deleteCurrentUser
//	@Summary	Delete the signed-in user with their sessions, keys and reviews
//	@Tags		users
//	@Security	SessionCookie
//	@Security	BearerAuth
//	@Success	204
//	@Failure	401	{object}	apperror.Problem
//	@Router		/users/me [delete]
func (h UserRoutes) deleteMe(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	if err := h.Users.DeleteUser(c.Request().Context(), p.User.ID); err != nil {
		return err
	}
	setSessionCookie(c, h.SecureCookies, "", time.Unix(0, 0))
	return c.NoContent(http.StatusNoContent)
}

func toUser(u auth.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt.UTC()}
}
