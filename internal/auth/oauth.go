package auth

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v92/github"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"

	"github.com/Pleasurecruise/eyeful/internal/db/sqlc"
)

type OAuth struct {
	svc      *Service
	sealer   *Sealer
	provider Provider
	conf     *oauth2.Config
}

func NewOAuth(svc *Service, sealer *Sealer, p Provider, c Client) *OAuth {
	return &OAuth{
		svc:      svc,
		sealer:   sealer,
		provider: p,
		conf: &oauth2.Config{
			ClientID:     c.ID,
			ClientSecret: c.Secret,
			RedirectURL:  c.RedirectURL,
			Endpoint:     p.Endpoint,
			Scopes:       p.Scopes,
		},
	}
}

func (o *OAuth) Provider() Provider {
	return o.provider
}

func (o *OAuth) AuthCodeURL(state, verifier string) string {
	return o.conf.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}

func (o *OAuth) Callback(ctx context.Context, code, verifier string, c ClientInfo) (User, SessionToken, error) {
	token, err := o.conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return User{}, SessionToken{}, fmt.Errorf("exchange %s code: %w", o.provider.ID, err)
	}
	who, err := o.fetchIdentity(ctx, token)
	if err != nil {
		return User{}, SessionToken{}, err
	}
	g := o.grant(token)

	tx, err := o.svc.pool.Begin(ctx)
	if err != nil {
		return User{}, SessionToken{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := o.svc.q.WithTx(tx)
	var row sqlc.User
	existing, err := q.GetAccountUser(ctx, sqlc.GetAccountUserParams{Provider: o.provider.ID, ProviderAccountID: who.ID})
	switch {
	case err == nil:
		row = existing.User
		if err := q.UpdateAccountGrant(ctx, g.update(o.provider.ID, who.ID)); err != nil {
			return User{}, SessionToken{}, fmt.Errorf("update %s grant: %w", o.provider.ID, err)
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return User{}, SessionToken{}, fmt.Errorf("get %s account: %w", o.provider.ID, err)
	default:
		row, err = q.GetUserByEmail(ctx, who.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			row, err = q.CreateUser(ctx, sqlc.CreateUserParams{ID: newID("usr_"), Email: who.Email, Name: who.Name})
		}
		if err != nil {
			return User{}, SessionToken{}, fmt.Errorf("find or create %s user: %w", o.provider.ID, err)
		}
		account := sqlc.CreateAccountParams{
			ID: newID("acc_"), UserID: row.ID, Provider: o.provider.ID, ProviderAccountID: who.ID, Scopes: g.Scopes,
			AccessToken: g.AccessToken, AccessExpiresAt: g.AccessExpiresAt, RefreshToken: g.RefreshToken, RefreshExpiresAt: g.RefreshExpiresAt,
		}
		if err := q.CreateAccount(ctx, account); err != nil {
			return User{}, SessionToken{}, fmt.Errorf("link %s account: %w", o.provider.ID, err)
		}
	}
	session, err := o.svc.issueSession(ctx, q, row.ID, c)
	if err != nil {
		return User{}, SessionToken{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, SessionToken{}, fmt.Errorf("commit: %w", err)
	}
	return userOf(row), session, nil
}

// Token returns the user's access token for this provider, refreshing it first when it has expired.
func (o *OAuth) Token(ctx context.Context, userID string) (*oauth2.Token, error) {
	tx, err := o.svc.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := o.svc.q.WithTx(tx)
	row, err := q.GetAccountGrant(ctx, sqlc.GetAccountGrantParams{UserID: userID, Provider: o.provider.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%s: %w", o.provider.ID, ErrNotLinked)
	}
	if err != nil {
		return nil, fmt.Errorf("get %s grant: %w", o.provider.ID, err)
	}
	access, err := o.sealer.open(row.AccessToken)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if !row.AccessExpiresAt.Valid || row.AccessExpiresAt.Time.After(now.Add(time.Minute)) {
		return &oauth2.Token{AccessToken: access, TokenType: "Bearer", Expiry: row.AccessExpiresAt.Time}, nil
	}
	if row.RefreshToken == nil || (row.RefreshExpiresAt.Valid && !row.RefreshExpiresAt.Time.After(now)) {
		return nil, fmt.Errorf("%s: %w", o.provider.ID, ErrGrantExpired)
	}
	refresh, err := o.sealer.open(row.RefreshToken)
	if err != nil {
		return nil, err
	}
	token, err := o.conf.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh, Expiry: now.Add(-time.Second)}).Token()
	if _, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
		return nil, fmt.Errorf("%s: %w: %w", o.provider.ID, ErrGrantExpired, err)
	}
	if err != nil {
		return nil, fmt.Errorf("refresh %s token: %w", o.provider.ID, err)
	}
	if err := q.UpdateAccountGrant(ctx, o.grant(token).update(o.provider.ID, row.ProviderAccountID)); err != nil {
		return nil, fmt.Errorf("store %s grant: %w", o.provider.ID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return token, nil
}

func (o *OAuth) grant(token *oauth2.Token) grant {
	g := grant{
		Scopes:      []string{},
		AccessToken: o.sealer.seal(token.AccessToken),
	}
	if scope, ok := token.Extra("scope").(string); ok {
		g.Scopes = strings.FieldsFunc(scope, func(r rune) bool { return r == ',' || r == ' ' })
	}
	if !token.Expiry.IsZero() {
		g.AccessExpiresAt = pgtype.Timestamptz{Time: token.Expiry, Valid: true}
	}
	if token.RefreshToken != "" {
		g.RefreshToken = o.sealer.seal(token.RefreshToken)
	}
	if seconds, ok := token.Extra("refresh_token_expires_in").(float64); ok {
		g.RefreshExpiresAt = pgtype.Timestamptz{Time: time.Now().Add(time.Duration(seconds) * time.Second), Valid: true}
	}
	return g
}

func (g grant) update(provider, accountID string) sqlc.UpdateAccountGrantParams {
	return sqlc.UpdateAccountGrantParams{
		Provider: provider, ProviderAccountID: accountID, Scopes: g.Scopes,
		AccessToken: g.AccessToken, AccessExpiresAt: g.AccessExpiresAt, RefreshToken: g.RefreshToken, RefreshExpiresAt: g.RefreshExpiresAt,
	}
}

func (o *OAuth) fetchIdentity(ctx context.Context, token *oauth2.Token) (identity, error) {
	client := o.conf.Client(ctx, token)
	api := o.provider.APIBase
	var who identity
	switch o.provider.ID {
	case GitHub.ID:
		gh, err := github.NewClient(github.WithHTTPClient(client), github.WithURLs(&api, nil))
		if err != nil {
			return identity{}, fmt.Errorf("github client: %w", err)
		}
		user, _, err := gh.Users.Get(ctx, "")
		if err != nil {
			return identity{}, fmt.Errorf("get github user: %w", err)
		}
		emails, _, err := gh.Users.ListEmails(ctx, nil)
		if err != nil {
			return identity{}, fmt.Errorf("list github emails: %w", err)
		}
		who = identity{ID: strconv.FormatInt(user.GetID(), 10), Name: cmp.Or(user.GetName(), user.GetLogin())}
		for _, e := range emails {
			if e.GetPrimary() && e.GetVerified() {
				who.Email = e.GetEmail()
			}
		}
	case Gitee.ID:
		query := url.Values{"access_token": {token.AccessToken}}
		user, err := getJSON[giteeUserDetail](ctx, client, api+"/user", query)
		if err != nil {
			return identity{}, err
		}
		emails, err := getJSON[[]giteeUserEmail](ctx, client, api+"/emails", query)
		if err != nil {
			return identity{}, err
		}
		who = identity{ID: strconv.FormatInt(user.ID, 10), Name: cmp.Or(user.Name, user.Login)}
		for _, e := range emails {
			if e.State == "confirmed" && slices.Contains(e.Scope, "primary") {
				who.Email = e.Email
			}
		}
	default:
		return identity{}, fmt.Errorf("unknown provider %q", o.provider.ID)
	}
	if who.Email == "" {
		return identity{}, ErrNoVerifiedEmail
	}
	return who, nil
}

func getJSON[T giteeUserDetail | []giteeUserEmail](ctx context.Context, client *http.Client, endpoint string, query url.Values) (out T, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, fmt.Errorf("request %s: %w", endpoint, err)
	}
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return out, fmt.Errorf("get %s: %w", req.URL.Path, err)
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("get %s: status %d", req.URL.Path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("decode %s: %w", req.URL.Path, err)
	}
	return out, nil
}
