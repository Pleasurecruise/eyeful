package main

import (
	"context"
	"time"

	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/local/app"
)

func (Agents) Providers() ([]app.Provider, error) {
	return app.Providers()
}

func (Agents) Connect(ctx context.Context, name string) (agents.Agent, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	return app.Connect(ctx, name)
}
