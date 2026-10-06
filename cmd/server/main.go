package main

import (
	"github.com/alecthomas/kong"
)

var version = "dev"

type cli struct {
	Version kong.VersionFlag `short:"v" help:"Print the version and exit."`

	Serve   serveCmd   `cmd:"" help:"Run the API server with its embedded workers."`
	Migrate migrateCmd `cmd:"" help:"Manage the database schema."`
	// TODO(cli): `reviews list|get|cancel` against this server.
	// TODO(cli): `runner` to claim and execute reviews without serving the API.
}

func main() {
	var c cli
	ctx := kong.Parse(&c,
		kong.Name("eyeful-server"),
		kong.Description("The eyeful cloud server: API, console and review workers."),
		kong.Vars{"version": version},
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	ctx.FatalIfErrorf(ctx.Run(ctx.Command()))
}
