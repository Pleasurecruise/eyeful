package dbtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ncruces/go-sqlite3"

	"github.com/Pleasurecruise/eyeful/internal/db"
	"github.com/Pleasurecruise/eyeful/internal/db/migrations"
)

func Open(t *testing.T) *db.Client {
	t.Helper()
	dsn := url.URL{
		Scheme:   "file",
		Path:     filepath.Join(t.TempDir(), "eyeful.db"),
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)",
	}
	conn, err := sqlite3.OpenContext(t.Context(), dsn.String())
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close sqlite: %v", err)
		}
	})
	srv := httptest.NewServer(&sqliteAPI{conn: conn})
	t.Cleanup(srv.Close)
	client, err := db.New(srv.URL, "account", "database", "token")
	if err != nil {
		t.Fatalf("d1 client: %v", err)
	}
	entries, err := fs.ReadDir(migrations.Migrations, ".")
	if err != nil {
		t.Fatalf("migrations: %v", err)
	}
	for _, entry := range entries {
		body, err := fs.ReadFile(migrations.Migrations, entry.Name())
		if err != nil {
			t.Fatalf("migration %s: %v", entry.Name(), err)
		}
		for stmt := range strings.SplitSeq(string(body), ";") {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if err := client.Exec(t.Context(), stmt); err != nil {
				t.Fatalf("migration %s: %v", entry.Name(), err)
			}
		}
	}
	return client
}

type sqliteAPI struct {
	mu   sync.Mutex
	conn *sqlite3.Conn
}

func (a *sqliteAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var q db.SingleQuery
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		fail(w, err)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.conn.SetInterrupt(r.Context())
	defer a.conn.SetInterrupt(context.Background())
	stmt, tail, err := a.conn.Prepare(q.SQL)
	if err != nil || stmt == nil {
		if err == nil {
			err = errors.New("empty sql")
		}
		fail(w, err)
		return
	}
	defer func() { _ = stmt.Close() }()
	if strings.Trim(tail, "; \t\r\n") != "" {
		fail(w, errors.New("d1 query: one statement"))
		return
	}
	for i, param := range q.Params {
		if err = stmt.BindText(i+1, param); err != nil {
			fail(w, err)
			return
		}
	}
	results := []map[string]json.RawMessage{}
	for stmt.Step() {
		rec := make(map[string]json.RawMessage, stmt.ColumnCount())
		for col := range stmt.ColumnCount() {
			val, cellErr := cellJSON(stmt, col)
			if cellErr != nil {
				fail(w, cellErr)
				return
			}
			rec[stmt.ColumnName(col)] = val
		}
		results = append(results, rec)
	}
	if err = stmt.Err(); err != nil {
		fail(w, err)
		return
	}
	if err := json.NewEncoder(w).Encode(db.QueryResponse{
		Success:  true,
		Errors:   []db.ResponseInfo{},
		Messages: []db.ResponseInfo{},
		Result:   []db.QueryResult{{Success: true, Results: results}},
	}); err != nil {
		return
	}
}

func cellJSON(stmt *sqlite3.Stmt, col int) (json.RawMessage, error) {
	name := stmt.ColumnName(col)
	var raw []byte
	var err error
	switch stmt.ColumnType(col) {
	case sqlite3.NULL:
		return json.RawMessage("null"), nil
	case sqlite3.INTEGER:
		raw, err = json.Marshal(stmt.ColumnInt64(col))
	case sqlite3.FLOAT:
		raw, err = json.Marshal(stmt.ColumnFloat(col))
	case sqlite3.TEXT:
		raw, err = json.Marshal(stmt.ColumnText(col))
	case sqlite3.BLOB:
		return nil, fmt.Errorf("column %s is a blob", name)
	default:
		return nil, fmt.Errorf("column %s: unsupported type", name)
	}
	if err != nil {
		return nil, fmt.Errorf("column %s: %w", name, err)
	}
	return raw, nil
}

func fail(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	if encErr := json.NewEncoder(w).Encode(db.QueryResponse{
		Errors:   []db.ResponseInfo{{Code: 7500, Message: err.Error()}},
		Messages: []db.ResponseInfo{},
		Result:   []db.QueryResult{},
	}); encErr != nil {
		return
	}
}
