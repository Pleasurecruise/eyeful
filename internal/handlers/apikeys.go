package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
)

type APIKeyRoutes struct {
	Store APIKeyStore
}

func (h APIKeyRoutes) Register(g *echo.Group) {
	g.POST("/api-keys", h.create)
	g.GET("/api-keys", h.list)
	g.GET("/api-keys/:id", h.get)
	g.PATCH("/api-keys/:id", h.update)
	g.DELETE("/api-keys/:id", h.delete)
}

// create godoc
//
//	@ID				createAPIKey
//	@Summary		Create an API key
//	@Description	The secret is returned only here; store it now.
//	@Tags			api-keys
//	@Accept			json
//	@Produce		json
//	@Security		SessionCookie
//	@Param			body	body		CreateAPIKeyRequest	true	"Key name"
//	@Success		201		{object}	CreatedAPIKey
//	@Failure		400		{object}	apperror.Problem
//	@Failure		401		{object}	apperror.Problem
//	@Router			/api-keys [post]
func (h APIKeyRoutes) create(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	var body CreateAPIKeyRequest
	if err := c.Bind(&body); err != nil {
		return apperror.BadRequest("malformed JSON body", err)
	}
	if body.Name == "" {
		return apperror.BadRequest("name is required", nil)
	}
	key, secret, err := h.Store.CreateAPIKey(c.Request().Context(), p.User.ID, body.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, CreatedAPIKey{Key: toAPIKey(key), Secret: secret})
}

// list godoc
//
//	@ID			listAPIKeys
//	@Summary	List API keys
//	@Tags		api-keys
//	@Produce	json
//	@Security	SessionCookie
//	@Success	200	{object}	APIKeyList
//	@Failure	401	{object}	apperror.Problem
//	@Router		/api-keys [get]
func (h APIKeyRoutes) list(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	keys, err := h.Store.ListAPIKeys(c.Request().Context(), p.User.ID)
	if err != nil {
		return err
	}
	out := APIKeyList{Items: make([]APIKeyResponse, len(keys))}
	for i, k := range keys {
		out.Items[i] = toAPIKey(k)
	}
	return c.JSON(http.StatusOK, out)
}

// get godoc
//
//	@ID			getAPIKey
//	@Summary	Get one API key
//	@Tags		api-keys
//	@Produce	json
//	@Security	SessionCookie
//	@Param		id	path		string	true	"Key ID"
//	@Success	200	{object}	APIKeyResponse
//	@Failure	401	{object}	apperror.Problem
//	@Failure	404	{object}	apperror.Problem
//	@Router		/api-keys/{id} [get]
func (h APIKeyRoutes) get(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	key, err := h.Store.GetAPIKey(c.Request().Context(), p.User.ID, c.Param("id"))
	if err != nil {
		return keyError(err)
	}
	return c.JSON(http.StatusOK, toAPIKey(key))
}

// update godoc
//
//	@ID			updateAPIKey
//	@Summary	Rename an API key
//	@Tags		api-keys
//	@Accept		json
//	@Produce	json
//	@Security	SessionCookie
//	@Param		id		path		string				true	"Key ID"
//	@Param		body	body		UpdateAPIKeyRequest	true	"New name"
//	@Success	200		{object}	APIKeyResponse
//	@Failure	400		{object}	apperror.Problem
//	@Failure	401		{object}	apperror.Problem
//	@Failure	404		{object}	apperror.Problem
//	@Router		/api-keys/{id} [patch]
func (h APIKeyRoutes) update(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	var body UpdateAPIKeyRequest
	if err := c.Bind(&body); err != nil {
		return apperror.BadRequest("malformed JSON body", err)
	}
	if body.Name == "" {
		return apperror.BadRequest("name is required", nil)
	}
	key, err := h.Store.UpdateAPIKey(c.Request().Context(), p.User.ID, c.Param("id"), body.Name)
	if err != nil {
		return keyError(err)
	}
	return c.JSON(http.StatusOK, toAPIKey(key))
}

// delete godoc
//
//	@ID			deleteAPIKey
//	@Summary	Revoke an API key
//	@Tags		api-keys
//	@Security	SessionCookie
//	@Param		id	path	string	true	"Key ID"
//	@Success	204
//	@Failure	401	{object}	apperror.Problem
//	@Failure	404	{object}	apperror.Problem
//	@Router		/api-keys/{id} [delete]
func (h APIKeyRoutes) delete(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	if err := h.Store.DeleteAPIKey(c.Request().Context(), p.User.ID, c.Param("id")); err != nil {
		return keyError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func keyError(err error) error {
	if errors.Is(err, auth.ErrKeyNotFound) {
		return apperror.New(apperror.CodeAPIKeyNotFound, http.StatusNotFound, "api key not found", nil)
	}
	return err
}

func toAPIKey(k auth.APIKey) APIKeyResponse {
	return APIKeyResponse{ID: k.ID, Name: k.Name, Prefix: k.Prefix, LastUsedAt: optionalTime(k.LastUsedAt), CreatedAt: k.CreatedAt.UTC()}
}
