package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var Assets embed.FS

func Built() bool {
	_, err := fs.Stat(Assets, "dist/index.html")
	return err == nil
}
