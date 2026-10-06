package main

import (
	"github.com/alecthomas/kong"
)

var version = "dev"

type cli struct {
	Version kong.VersionFlag `short:"v" help:"Print the version and exit."`

	Review   reviewCmd   `cmd:"" help:"Review local changes with your own coding agent."`
	Provider providerCmd `cmd:"" help:"List the coding agents eyeful can use."`
	Connect  connectCmd  `cmd:"" help:"Choose the coding agent CLI reviews run with."`
	Commit   commitCmd   `cmd:"" help:"Commit every local change, with a message the connected agent writes."`
}

func main() {
	var c cli
	ctx := kong.Parse(&c,
		kong.Name("eyeful"),
		kong.Description("Review local changes with a group of agents. Given enough eyeballs, all bugs are shallow."),
		kong.Vars{"version": version},
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	ctx.FatalIfErrorf(ctx.Run(ctx.Command()))
}
