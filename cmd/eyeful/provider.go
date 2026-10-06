package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/Pleasurecruise/eyeful/local/app"
)

type providerCmd struct{}

func (c *providerCmd) Run() error {
	providers, err := app.Providers()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	rows := []string{"PROVIDER\tCONNECTED\tINSTALLED"}
	mark := map[bool]string{true: "yes", false: "-"}
	for _, p := range providers {
		installed := mark[p.Installed]
		if !p.Installed {
			installed = "no (" + p.Tool + " not on PATH)"
		}
		rows = append(rows, strings.Join([]string{p.Name, mark[p.Connected], installed}, "\t"))
	}
	if _, err := io.WriteString(w, strings.Join(rows, "\n")+"\n"); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
