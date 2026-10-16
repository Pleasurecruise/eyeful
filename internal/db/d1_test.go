package db_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Pleasurecruise/eyeful/internal/db"
)

func writeAPI(t *testing.T, w http.ResponseWriter, status int, body db.QueryResponse) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Error(err)
	}
}

func TestQuery(t *testing.T) {
	const token = "super-secret-token"
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(string(body), `"params":["r_1"]`) || strings.Contains(string(body), "r_1'") {
			t.Errorf("body %s", body)
		}
		writeAPI(t, w, http.StatusOK, db.QueryResponse{
			Success:  true,
			Errors:   []db.ResponseInfo{},
			Messages: []db.ResponseInfo{},
			Result: []db.QueryResult{{
				Success: true,
				Results: []map[string]json.RawMessage{{
					"id":       json.RawMessage(`"r_1"`),
					"attempts": json.RawMessage(`2`),
				}},
			}},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := db.New(srv.URL, "account", "database", token)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.Query(t.Context(), "SELECT id, attempts FROM reviews WHERE id = ?", "r_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || hits.Load() != 1 {
		t.Fatalf("rows %d hits %d", len(rows), hits.Load())
	}
	id, ok := rows[0].Text("id")
	attempts, err := rows[0].Int32("attempts")
	if !ok || id != "r_1" || err != nil || attempts != 2 {
		t.Fatalf("id %q ok %v attempts %d err %v", id, ok, attempts, err)
	}
}

func TestQueryRetries(t *testing.T) {
	const token = "super-secret-token"
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			writeAPI(t, w, http.StatusOK, db.QueryResponse{
				Errors:   []db.ResponseInfo{{Code: 7500, Message: "Network connection lost " + token}},
				Messages: []db.ResponseInfo{},
				Result:   []db.QueryResult{},
			})
			return
		}
		writeAPI(t, w, http.StatusOK, db.QueryResponse{
			Success:  true,
			Errors:   []db.ResponseInfo{},
			Messages: []db.ResponseInfo{},
			Result: []db.QueryResult{{
				Success: true,
				Results: []map[string]json.RawMessage{{"ok": json.RawMessage(`1`)}},
			}},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := db.New(srv.URL, "account", "database", token)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits %d", hits.Load())
	}
}

func TestQueryDoesNotRetry(t *testing.T) {
	const token = "super-secret-token"
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		writeAPI(t, w, http.StatusBadRequest, db.QueryResponse{
			Errors:   []db.ResponseInfo{{Code: 7500, Message: "no such table: reviews " + token}},
			Messages: []db.ResponseInfo{},
			Result:   []db.QueryResult{},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := db.New(srv.URL, "account", "database", token)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(t.Context(), "SELECT 1")
	if err == nil || strings.Contains(err.Error(), token) || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("err %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits %d", hits.Load())
	}
}
