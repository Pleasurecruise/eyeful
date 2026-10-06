package auth_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/Pleasurecruise/eyeful/internal/auth"
	"github.com/Pleasurecruise/eyeful/internal/db/dbtest"
)

type fakeAccount struct {
	id        int64
	emails    string
	expiresIn string
	refreshes *atomic.Int32
}

func sealer(t *testing.T) *auth.Sealer {
	t.Helper()
	s, err := auth.NewSealer(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func fakeProvider(t *testing.T, base auth.Provider, account fakeAccount) auth.Provider {
	t.Helper()
	emailsPath := map[string]string{auth.GitHub.ID: "/user/emails", auth.Gitee.ID: "/emails"}[base.ID]
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.FormValue("grant_type") == "refresh_token" {
			if r.FormValue("refresh_token") != "refresh-1" {
				http.Error(w, `{"error":"bad_refresh_token"}`, http.StatusBadRequest)
				return
			}
			account.refreshes.Add(1)
			time.Sleep(50 * time.Millisecond)
			if _, err := w.Write([]byte(`{"access_token":"tok-2","token_type":"bearer","expires_in":28800,"refresh_token":"refresh-2","refresh_token_expires_in":15897600}`)); err != nil {
				t.Error(err)
			}
			return
		}
		if r.FormValue("code") != "good-code" || r.FormValue("code_verifier") == "" {
			http.Error(w, "bad code", http.StatusBadRequest)
			return
		}
		if _, err := w.Write([]byte(`{"access_token":"tok","token_type":"bearer","expires_in":` + account.expiresIn + `,"refresh_token":"refresh-1","refresh_token_expires_in":15897600}`)); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if base.ID == auth.Gitee.ID && !strings.HasPrefix(r.URL.Query().Get("access_token"), "tok") {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		user := struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		}{ID: account.id, Login: "ada"}
		if err := json.NewEncoder(w).Encode(user); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("GET "+emailsPath, func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(account.emails)); err != nil {
			t.Error(err)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	base.Endpoint = oauth2.Endpoint{AuthURL: srv.URL + "/authorize", TokenURL: srv.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}
	base.APIBase = srv.URL
	return base
}

const (
	githubVerified = `[{"email":"ada@example.com","primary":true,"verified":true}]`
	giteeVerified  = `[{"email":"ada@example.com","state":"confirmed","scope":["primary","committed"]}]`
)

func signIn(t *testing.T, svc *auth.Service) (auth.User, auth.SessionToken) {
	t.Helper()
	provider := fakeProvider(t, auth.GitHub, fakeAccount{42, githubVerified, "28800", new(atomic.Int32)})
	user, session, err := auth.NewOAuth(svc, sealer(t), provider, auth.Client{ID: "id", Secret: "secret"}).Callback(ctx, "good-code", oauth2.GenerateVerifier(), auth.ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	return user, session
}

func TestOAuthCallback(t *testing.T) {
	svc := auth.NewService(dbtest.Open(t), time.Hour)
	first, _ := signIn(t, svc)
	if first.Name != "ada" || first.Email != "ada@example.com" {
		t.Fatalf("new user %+v", first)
	}
	again, session := signIn(t, svc)
	if again.ID != first.ID {
		t.Fatalf("second sign-in made user %s, want %s", again.ID, first.ID)
	}
	if p, err := svc.Authenticate(t.Context(), "", session.Token); err != nil || p.User.ID != first.ID {
		t.Fatalf("session: %+v %v", p, err)
	}
}

func TestOAuthLinking(t *testing.T) {
	svc := auth.NewService(dbtest.Open(t), time.Hour)
	fromGitHub, _ := signIn(t, svc)
	gitee := fakeProvider(t, auth.Gitee, fakeAccount{7, giteeVerified, "86400", new(atomic.Int32)})
	fromGitee, _, err := auth.NewOAuth(svc, sealer(t), gitee, auth.Client{ID: "id", Secret: "secret"}).Callback(ctx, "good-code", oauth2.GenerateVerifier(), auth.ClientInfo{})
	if err != nil || fromGitee.ID != fromGitHub.ID {
		t.Fatalf("gitee user %+v, want %s (%v)", fromGitee, fromGitHub.ID, err)
	}
}

func TestOAuthRejects(t *testing.T) {
	svc := auth.NewService(dbtest.Open(t), time.Hour)
	cases := map[string]struct {
		provider auth.Provider
		code     string
		want     error
	}{
		"github unverified": {fakeProvider(t, auth.GitHub, fakeAccount{9, `[{"email":"x@example.com","primary":true,"verified":false}]`, "28800", new(atomic.Int32)}), "good-code", auth.ErrNoVerifiedEmail},
		"gitee unconfirmed": {fakeProvider(t, auth.Gitee, fakeAccount{9, `[{"email":"x@example.com","state":"unconfirmed","scope":["primary"]}]`, "28800", new(atomic.Int32)}), "good-code", auth.ErrNoVerifiedEmail},
		"gitee not primary": {fakeProvider(t, auth.Gitee, fakeAccount{9, `[{"email":"x@example.com","state":"confirmed","scope":["committed"]}]`, "28800", new(atomic.Int32)}), "good-code", auth.ErrNoVerifiedEmail},
	}
	for name, c := range cases {
		_, _, err := auth.NewOAuth(svc, sealer(t), c.provider, auth.Client{ID: "id", Secret: "secret"}).Callback(ctx, c.code, oauth2.GenerateVerifier(), auth.ClientInfo{})
		if !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
	bad := auth.NewOAuth(svc, sealer(t), fakeProvider(t, auth.GitHub, fakeAccount{9, githubVerified, "28800", new(atomic.Int32)}), auth.Client{ID: "id", Secret: "secret"})
	if _, _, err := bad.Callback(ctx, "bad-code", oauth2.GenerateVerifier(), auth.ClientInfo{}); err == nil {
		t.Fatal("bad code accepted")
	}
}

func TestOAuthToken(t *testing.T) {
	svc := auth.NewService(dbtest.Open(t), time.Hour)
	account := fakeAccount{42, githubVerified, "28800", new(atomic.Int32)}
	gh := auth.NewOAuth(svc, sealer(t), fakeProvider(t, auth.GitHub, account), auth.Client{ID: "id", Secret: "secret"})
	user, _, err := gh.Callback(ctx, "good-code", oauth2.GenerateVerifier(), auth.ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	token, err := gh.Token(ctx, user.ID)
	if err != nil || token.AccessToken != "tok" || account.refreshes.Load() != 0 {
		t.Fatalf("fresh token %+v, refreshes %d, err = %v", token, account.refreshes.Load(), err)
	}
	if _, err := gh.Token(ctx, "usr_nobody"); !errors.Is(err, auth.ErrNotLinked) {
		t.Errorf("unlinked user: err = %v", err)
	}
	other, err := auth.NewSealer(append(make([]byte, 31), 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.NewOAuth(svc, other, auth.GitHub, auth.Client{}).Token(ctx, user.ID); !errors.Is(err, auth.ErrSealedToken) {
		t.Errorf("wrong key: err = %v", err)
	}
}

func TestOAuthTokenRefresh(t *testing.T) {
	svc := auth.NewService(dbtest.Open(t), time.Hour)
	account := fakeAccount{42, githubVerified, "30", new(atomic.Int32)}
	gh := auth.NewOAuth(svc, sealer(t), fakeProvider(t, auth.GitHub, account), auth.Client{ID: "id", Secret: "secret"})
	user, _, err := gh.Callback(ctx, "good-code", oauth2.GenerateVerifier(), auth.ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	tokens := make([]string, 8)
	for i := range tokens {
		wg.Go(func() {
			token, err := gh.Token(ctx, user.ID)
			if err != nil {
				t.Error(err)
				return
			}
			tokens[i] = token.AccessToken
		})
	}
	wg.Wait()
	if n := account.refreshes.Load(); n != 1 {
		t.Fatalf("refreshed %d times, want once: a refresh token is single-use", n)
	}
	for _, tok := range tokens {
		if tok != "tok-2" {
			t.Fatalf("tokens %v, want all tok-2", tokens)
		}
	}
}
