package dbtest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pleasurecruise/eyeful/internal/db"
)

const envURL = "EYEFUL_TEST_DATABASE_URL"

func Open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(envURL)
	if url == "" {
		t.Skip(envURL + " is not set")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	m, err := db.NewMigrator(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	return pool
}

func CreateUser(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "INSERT INTO users (id, email) VALUES ($1, $2)", id, id+"@example.com"); err != nil {
		t.Fatal(err)
	}
}
