package handlers

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/google/go-github/v92/github"
	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
	"github.com/Pleasurecruise/eyeful/internal/repository"
)

type RepositoryRoutes struct {
	Repos RepositoryReader
}

func (h RepositoryRoutes) Register(g *echo.Group) {
	g.GET("/repositories", h.list)
	g.GET("/repositories/:owner/:name", h.get)
	g.GET("/repositories/:owner/:name/commits/:ref", h.commit)
	g.GET("/repositories/:owner/:name/commits/:ref/diff", h.diff)
	g.GET("/repositories/:owner/:name/git/trees/:sha", h.tree)
}

// list godoc
//
//	@ID				listRepositories
//	@Summary		The GitHub repositories the signed-in user gave eyeful
//	@Description	Every installation of the GitHub App the user can access, with every repository of each. An installation's html_url is where the user changes its repositories.
//	@Tags			repositories
//	@Produce		json
//	@Security		SessionCookie
//	@Security		BearerAuth
//	@Success		200	{object}	RepositoryList
//	@Failure		401	{object}	apperror.Problem
//	@Failure		403	{object}	apperror.Problem
//	@Failure		503	{object}	apperror.Problem
//	@Router			/repositories [get]
func (h RepositoryRoutes) list(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	installs, repos, err := h.Repos.List(c.Request().Context(), p.User.ID)
	if err != nil {
		return repositoryError(c, err)
	}
	items := make([]RepositoryResponse, len(repos))
	for i, r := range repos {
		items[i] = repositoryResponse(r)
	}
	installations := make([]InstallationResponse, len(installs))
	for i, in := range installs {
		installations[i] = InstallationResponse{
			ID:                  in.GetID(),
			RepositorySelection: in.GetRepositorySelection(),
			HTMLURL:             in.GetHTMLURL(),
			TargetType:          in.GetTargetType(),
		}
		if in.Account != nil {
			installations[i].Account = &InstallationAccountResponse{Login: in.Account.GetLogin()}
		}
	}
	return c.JSON(http.StatusOK, RepositoryList{Items: items, Installations: installations})
}

// get godoc
//
//	@ID				getRepository
//	@Summary		A GitHub repository the signed-in user gave eyeful
//	@Description	The fields of GitHub's full-repository schema the console needs, read with the user's GitHub App token.
//	@Tags			repositories
//	@Produce		json
//	@Security		SessionCookie
//	@Security		BearerAuth
//	@Param			owner	path		string	true	"Owner"	example(octo)
//	@Param			name	path		string	true	"Name"	example(app)
//	@Success		200		{object}	RepositoryResponse
//	@Failure		400		{object}	apperror.Problem
//	@Failure		401		{object}	apperror.Problem
//	@Failure		403		{object}	apperror.Problem
//	@Failure		404		{object}	apperror.Problem
//	@Failure		503		{object}	apperror.Problem
//	@Router			/repositories/{owner}/{name} [get]
func (h RepositoryRoutes) get(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	r, err := h.Repos.Get(c.Request().Context(), p.User.ID, c.Param("owner"), c.Param("name"))
	if err != nil {
		return repositoryError(c, err)
	}
	return c.JSON(http.StatusOK, repositoryResponse(r))
}

// commit godoc
//
//	@ID				getRepositoryCommit
//	@Summary		A commit, by sha or branch
//	@Description	The fields of GitHub's commit schema the console needs, without its files.
//	@Tags			repositories
//	@Produce		json
//	@Security		SessionCookie
//	@Security		BearerAuth
//	@Param			owner	path		string	true	"Owner"					example(octo)
//	@Param			name	path		string	true	"Name"					example(app)
//	@Param			ref		path		string	true	"Commit sha or branch"	example(main)
//	@Success		200		{object}	CommitResponse
//	@Failure		400		{object}	apperror.Problem
//	@Failure		401		{object}	apperror.Problem
//	@Failure		403		{object}	apperror.Problem
//	@Failure		404		{object}	apperror.Problem
//	@Failure		503		{object}	apperror.Problem
//	@Router			/repositories/{owner}/{name}/commits/{ref} [get]
func (h RepositoryRoutes) commit(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	rc, err := h.Repos.Commit(c.Request().Context(), p.User.ID, c.Param("owner"), c.Param("name"), c.Param("ref"))
	if err != nil {
		return repositoryError(c, err)
	}
	parents := make([]ParentResponse, len(rc.Parents))
	for i, p := range rc.Parents {
		parents[i] = ParentResponse{SHA: p.GetSHA()}
	}
	gc := rc.GetCommit()
	return c.JSON(http.StatusOK, CommitResponse{
		SHA:     rc.GetSHA(),
		HTMLURL: rc.GetHTMLURL(),
		Commit: GitCommitResponse{
			Author:    gitUser(gc.Author),
			Committer: gitUser(gc.Committer),
			Message:   gc.GetMessage(),
			Tree:      TreeRefResponse{SHA: gc.GetTree().GetSHA()},
		},
		Parents: parents,
	})
}

// diff godoc
//
//	@ID				getRepositoryCommitDiff
//	@Summary		The unified diff a commit made against its first parent
//	@Tags			repositories
//	@Produce		json
//	@Security		SessionCookie
//	@Security		BearerAuth
//	@Param			owner	path		string	true	"Owner"				example(octo)
//	@Param			name	path		string	true	"Name"				example(app)
//	@Param			ref		path		string	true	"Full commit sha"
//	@Success		200		{object}	CommitDiffResponse
//	@Failure		400		{object}	apperror.Problem
//	@Failure		401		{object}	apperror.Problem
//	@Failure		403		{object}	apperror.Problem
//	@Failure		404		{object}	apperror.Problem
//	@Failure		503		{object}	apperror.Problem
//	@Router			/repositories/{owner}/{name}/commits/{ref}/diff [get]
func (h RepositoryRoutes) diff(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	d, err := h.Repos.Diff(c.Request().Context(), p.User.ID, c.Param("owner"), c.Param("name"), c.Param("ref"))
	if err != nil {
		return repositoryError(c, err)
	}
	return c.JSON(http.StatusOK, CommitDiffResponse{Diff: d})
}

// tree godoc
//
//	@ID				getRepositoryTree
//	@Summary		A git tree, recursively
//	@Description	GitHub's git-tree schema. truncated is true when GitHub returned only part of a large tree.
//	@Tags			repositories
//	@Produce		json
//	@Security		SessionCookie
//	@Security		BearerAuth
//	@Param			owner	path		string	true	"Owner"				example(octo)
//	@Param			name	path		string	true	"Name"				example(app)
//	@Param			sha		path		string	true	"Full tree sha"
//	@Success		200		{object}	GitTreeResponse
//	@Failure		400		{object}	apperror.Problem
//	@Failure		401		{object}	apperror.Problem
//	@Failure		403		{object}	apperror.Problem
//	@Failure		404		{object}	apperror.Problem
//	@Failure		503		{object}	apperror.Problem
//	@Router			/repositories/{owner}/{name}/git/trees/{sha} [get]
func (h RepositoryRoutes) tree(c *echo.Context) error {
	p, err := principal(c)
	if err != nil {
		return err
	}
	t, err := h.Repos.Tree(c.Request().Context(), p.User.ID, c.Param("owner"), c.Param("name"), c.Param("sha"))
	if err != nil {
		return repositoryError(c, err)
	}
	entries := make([]GitTreeEntryResponse, len(t.Entries))
	for i, e := range t.Entries {
		entries[i] = GitTreeEntryResponse{Path: e.GetPath(), Mode: e.GetMode(), Type: e.GetType(), SHA: e.GetSHA(), Size: e.Size}
	}
	return c.JSON(http.StatusOK, GitTreeResponse{SHA: t.GetSHA(), Truncated: t.GetTruncated(), Tree: entries})
}

func repositoryResponse(r *github.Repository) RepositoryResponse {
	return RepositoryResponse{
		Owner:         OwnerResponse{Login: r.GetOwner().GetLogin()},
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Private:       r.GetPrivate(),
		HTMLURL:       r.GetHTMLURL(),
		DefaultBranch: r.GetDefaultBranch(),
	}
}

func gitUser(u *github.CommitAuthor) *GitUserResponse {
	if u == nil {
		return nil
	}
	r := &GitUserResponse{Name: u.GetName(), Email: u.GetEmail()}
	if u.Date != nil {
		r.Date = &u.Date.Time
	}
	return r
}

func repositoryError(c *echo.Context, err error) error {
	limited, isLimited := errors.AsType[*repository.RateLimitedError](err)
	switch {
	case errors.Is(err, repository.ErrInvalidName):
		return apperror.BadRequest(err.Error(), nil)
	case errors.Is(err, repository.ErrNotFound):
		return apperror.New(apperror.CodeRepositoryNotFound, http.StatusNotFound, "not among the repositories you gave eyeful on GitHub", err)
	case errors.Is(err, auth.ErrNotLinked):
		return apperror.New(apperror.CodeProviderNotLinked, http.StatusForbidden, "sign in with GitHub to see your repositories", err)
	case errors.Is(err, auth.ErrGrantExpired):
		return apperror.New(apperror.CodeProviderExpired, http.StatusUnauthorized, "GitHub access expired; sign in with GitHub again", err)
	case isLimited:
		c.Response().Header().Set(echo.HeaderRetryAfter, strconv.Itoa(int(math.Ceil(limited.RetryAfter.Seconds()))))
		return apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "GitHub's rate limit for this server is used up", err)
	}
	return err
}
