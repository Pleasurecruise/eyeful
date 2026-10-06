package app

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/local/git"
	"github.com/Pleasurecruise/eyeful/local/settings"
)

func connected(ctx context.Context) (agents.Provider, agents.Agent, error) {
	s, err := Current()
	if err != nil {
		return agents.Provider{}, agents.Agent{}, err
	}
	if s.Agent == "" {
		return agents.Provider{}, agents.Agent{}, settings.ErrNoAgent
	}
	p, ok := agents.Find(s.Agent)
	if !ok {
		return agents.Provider{}, agents.Agent{}, fmt.Errorf("%w: %q", ErrUnknownAgent, s.Agent)
	}
	a, err := agents.Detect(ctx, p)
	return p, a, err
}

func Connect(ctx context.Context, name string) (agents.Agent, error) {
	p, ok := agents.Find(name)
	if !ok {
		return agents.Agent{}, fmt.Errorf("%w: %q", ErrUnknownAgent, name)
	}
	a, err := agents.Detect(ctx, p)
	if err != nil {
		return agents.Agent{}, err
	}
	path, err := settings.Path()
	if err != nil {
		return agents.Agent{}, err
	}
	s, err := settings.Load(path)
	if err != nil {
		return agents.Agent{}, err
	}
	s.Agent = name
	return a, settings.Save(path, s)
}

func Remember(dir string) error {
	path, err := settings.Path()
	if err != nil {
		return err
	}
	s, err := settings.Load(path)
	if err != nil {
		return err
	}
	s.Repository = dir
	return settings.Save(path, s)
}

func Repository(ctx context.Context, dir string) (string, error) {
	if _, err := git.Run(ctx, dir, nil, "rev-parse", "--show-toplevel"); err == nil {
		return dir, nil
	}
	s, err := Current()
	if err != nil || s.Repository == "" {
		return dir, err
	}
	return s.Repository, nil
}

func Current() (settings.Settings, error) {
	path, err := settings.Path()
	if err != nil {
		return settings.Settings{}, err
	}
	return settings.Load(path)
}

func Providers() ([]Provider, error) {
	s, err := Current()
	if err != nil {
		return nil, err
	}
	out := make([]Provider, 0, len(agents.Providers))
	for _, p := range agents.Providers {
		_, missing := exec.LookPath(p.Tool)
		out = append(out, Provider{Provider: p, Connected: s.Agent == p.Name, Installed: missing == nil})
	}
	return out, nil
}
