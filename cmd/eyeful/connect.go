package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Pleasurecruise/eyeful/local/app"
)

type connectCmd struct {
	Agent string `arg:"" enum:"claude,codex,pi" help:"Coding agent CLI to review with: claude, codex or pi."`
}

func (c *connectCmd) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	a, err := app.Connect(ctx, c.Agent)
	if err != nil {
		return err
	}
	if _, err := fmt.Printf("Connected to %s %s. Reviews now use it.\n", a.Name, a.Version); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
