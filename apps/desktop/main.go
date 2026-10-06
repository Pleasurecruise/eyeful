package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := fs.Stat(dist, "index.html"); err != nil {
		log.Fatal("apps/desktop/frontend/dist is empty: run `mise run build:desktop`")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if shell := os.Getenv("SHELL"); shell != "" && runtime.GOOS != "windows" {
		cmd := exec.CommandContext(ctx, shell, "-i", "-l", "-c", `printf '\n%s\n' "$PATH"`)
		cmd.WaitDelay = time.Second
		out, err := cmd.Output()
		lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		if path := lines[len(lines)-1]; err == nil && path != "" {
			if err := os.Setenv("PATH", path); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Printf("read PATH from %s: %v; agent CLIs are looked up in %s", shell, err, os.Getenv("PATH"))
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	dir, err := app.Repository(ctx, cwd)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	application.RegisterEvent[string](EventLog)
	application.RegisterEvent[[]project.Command](EventConfirm)
	tree := &Worktree{dir: dir}
	app := application.New(application.Options{
		Name:        "eyeful",
		Description: "Multi-agent code review with executable evidence",
		Services: []application.Service{
			application.NewService(tree),
			application.NewService(&Agents{}),
			application.NewService(&Commit{tree: tree}),
			application.NewService(&Review{tree: tree, answers: make(chan bool)}),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(dist)},
		Mac:    application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "eyeful",
		Width:  1100,
		Height: 760,
		Mac:    application.MacWindow{TitleBar: application.MacTitleBarHiddenInset},
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
