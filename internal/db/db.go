package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pleasurecruise/eyeful/db/migrations"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

type Migrator struct {
	m *migrate.Migrate
}

func NewMigrator(url string) (*Migrator, error) {
	src, err := iofs.New(migrations.Migrations, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+strings.TrimPrefix(strings.TrimPrefix(url, "postgres://"), "postgresql://"))
	if err != nil {
		return nil, fmt.Errorf("open migrations: %w", err)
	}
	return &Migrator{m: m}, nil
}

func (m *Migrator) Up() error   { return ignoreNoChange(m.m.Up()) }
func (m *Migrator) Down() error { return ignoreNoChange(m.m.Steps(-1)) }

func (m *Migrator) Version() (version uint, dirty bool, err error) {
	version, dirty, err = m.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("schema version: %w", err)
	}
	return version, dirty, nil
}

func (m *Migrator) Close() error {
	srcErr, dbErr := m.m.Close()
	return errors.Join(srcErr, dbErr)
}

func ignoreNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}
