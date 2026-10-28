package classify_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	jev "github.com/mattn/go-jev"

	"github.com/Pleasurecruise/eyeful/local/classify"
	"github.com/Pleasurecruise/eyeful/workflow"
)

func TestSame(t *testing.T) {
	first := workflow.Finding{ID: "0/correctness/0", Path: "app/session.py", Line: 10, Subject: "expiry is off by one"}
	pairs := [][2]workflow.Finding{
		{first, {ID: "0/security/0", Path: "app/session.py", Line: 11, Subject: "a session outlives its expiry"}},
		{first, {ID: "0/design/0", Path: "app/session.py", Line: 11, Subject: "the name is misleading"}},
	}
	serve := func(body string) *classify.Jev {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sent, err := io.ReadAll(r.Body)
			if err != nil || r.Header.Get("Authorization") != "Bearer token" || !strings.HasPrefix(string(sent), `{"state":{"findings":`) || !strings.Contains(string(sent), "`findings.f0` and `findings.f2`") {
				t.Errorf("request %s with %q: %v", sent, r.Header.Get("Authorization"), err)
			}
			if strings.Contains(body, `"errors"`) {
				w.WriteHeader(http.StatusUnauthorized)
			}
			if _, err := io.WriteString(w, body); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(srv.Close)
		j, err := classify.New(slog.New(slog.DiscardHandler), "token", jev.WithURL(srv.URL), jev.WithMaxRetries(0))
		if err != nil {
			t.Fatal(err)
		}
		return j
	}

	t.Run("each pair gets its probability", func(t *testing.T) {
		same, usage, err := serve(`{"model":"jev-1.13.0","answers":{"q0":{"type":"noul","noul":0.97},"q1":{"type":"noul","noul":0.04}},"usage":{"input_tokens":120,"output_tokens":8}}`).Same(t.Context(), pairs)
		if err != nil || !slices.Equal(same, []float64{0.97, 0.04}) || usage.Model != "jev-1.13.0" || usage.Tokens != 128 {
			t.Fatalf("same %v, usage %+v: %v", same, usage, err)
		}
	})

	t.Run("a pair left unanswered is an error", func(t *testing.T) {
		if _, _, err := serve(`{"model":"jev-1.13.0","answers":{"q0":{"type":"noul","noul":0.97}},"usage":{}}`).Same(t.Context(), pairs); !errors.Is(err, classify.ErrUnanswered) {
			t.Fatalf("error %v", err)
		}
	})

	t.Run("a rejected request is an error", func(t *testing.T) {
		if _, _, err := serve(`{"errors":[{"message":"Authentication error"}]}`).Same(t.Context(), pairs); !errors.Is(err, classify.ErrRejected) || !strings.Contains(err.Error(), "Authentication error") {
			t.Fatalf("error %v", err)
		}
	})

	t.Run("a key is required", func(t *testing.T) {
		if _, err := classify.New(slog.New(slog.DiscardHandler), ""); !errors.Is(err, classify.ErrNoKey) {
			t.Fatalf("error %v", err)
		}
	})
}
