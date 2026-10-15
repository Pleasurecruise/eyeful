package main

import (
	"errors"
	"fmt"

	"github.com/Pleasurecruise/eyeful/internal/db"
)

type migrateCmd struct {
	DatabaseURL string `required:"" env:"EYEFUL_DATABASE_URL" help:"PostgreSQL connection URL."`

	Up      struct{} `cmd:"" help:"Apply all pending migrations."`
	Down    struct{} `cmd:"" help:"Roll back the latest migration."`
	Version struct{} `cmd:"" help:"Print the current schema version."`
}

func (m *migrateCmd) Run(command string) error {
	migrator, err := db.NewMigrator(m.DatabaseURL)
	if err != nil {
		return err
	}
	switch command {
	case "migrate up":
		err = migrator.Up()
	case "migrate down":
		err = migrator.Down()
	case "migrate version":
		var version uint
		var dirty bool
		version, dirty, err = migrator.Version()
		if err == nil {
			fmt.Printf("%d dirty=%t\n", version, dirty)
		}
	}
	return errors.Join(err, migrator.Close())
}
