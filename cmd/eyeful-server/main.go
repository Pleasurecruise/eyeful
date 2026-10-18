package main

import (
	"github.com/alecthomas/kong"
)

var version = "dev"

type cli struct {
	Version kong.VersionFlag `short:"v" help:"Print the version and exit."`

	Serve serveCmd `cmd:"" help:"Run the review named by EYEFUL_REVIEW and answer for it on GET /review."`
}

func main() {
	var c cli
	ctx := kong.Parse(&c,
		kong.Name("eyeful-server"),
		kong.Description("The eyeful cloud server: one pull request review per process."),
		kong.Vars{"version": version},
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	ctx.FatalIfErrorf(ctx.Run(ctx.Command()))
}
