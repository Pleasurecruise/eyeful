package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func Detect(root string) (Config, error) {
	found := map[string]string{}
	for _, name := range manifests {
		data, err := os.ReadFile(filepath.Join(root, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, fmt.Errorf("read %s: %w", name, err)
		}
		found[name] = string(data)
	}
	var c Config
	switch {
	case has(found, "go.mod"):
		c = Config{Setup: "go mod download", Test: "go test ./...", TestOne: "go test ./... -run " + Placeholder}
		if has(found, ".golangci.yml", ".golangci.yaml", ".golangci.toml") {
			c.Lint = "golangci-lint run ./..."
		}
	case has(found, "Cargo.toml"):
		c = Config{Setup: "cargo fetch", Test: "cargo test", TestOne: "cargo test " + Placeholder}
	case has(found, "pyproject.toml", "setup.py", "setup.cfg"):
		c = python(found)
	case has(found, "package.json"):
		var err error
		if c, err = node(found); err != nil {
			return Config{}, err
		}
	}
	if c.Test == "" && c.Lint == "" {
		return Config{}, nil
	}
	return c, nil
}

func has(found map[string]string, names ...string) bool {
	return slices.ContainsFunc(names, func(n string) bool { _, ok := found[n]; return ok })
}

func python(found map[string]string) Config {
	var c Config
	run := "python -m "
	switch {
	case has(found, "uv.lock"):
		c.Setup, run = "uv sync", "uv run "
	case has(found, "poetry.lock"):
		c.Setup, run = "poetry install", "poetry run "
	case has(found, "requirements.txt"):
		c.Setup = "python -m pip install -r requirements.txt"
	default:
		c.Setup = "python -m pip install -e ."
	}
	if has(found, "pytest.ini", "conftest.py") || slices.ContainsFunc([]string{"pyproject.toml", "setup.cfg", "tox.ini", "requirements.txt"}, func(n string) bool { return strings.Contains(found[n], "pytest") }) {
		c.Test, c.TestOne = run+"pytest -q", run+"pytest -q "+Placeholder
	}
	if has(found, "ruff.toml") || strings.Contains(found["pyproject.toml"], "[tool.ruff") {
		c.Lint = run + "ruff check ."
	}
	return c
}

func node(found map[string]string) (Config, error) {
	var m manifest
	if err := json.Unmarshal([]byte(found["package.json"]), &m); err != nil {
		return Config{}, fmt.Errorf("read package.json: %w", err)
	}
	pm, c, exec := "npm", Config{Setup: "npm install"}, "npx "
	switch {
	case has(found, "pnpm-lock.yaml"):
		pm, c.Setup, exec = "pnpm", "pnpm install --frozen-lockfile", "pnpm exec "
	case has(found, "yarn.lock"):
		pm, c.Setup, exec = "yarn", "yarn install", "yarn "
	case has(found, "bun.lock", "bun.lockb"):
		pm, c.Setup, exec = "bun", "bun install", "bunx "
	case has(found, "package-lock.json"):
		c.Setup = "npm ci"
	}
	for _, runner := range []string{"vitest run", "jest", "mocha"} {
		name := strings.Fields(runner)[0]
		if m.Dependencies[name] != "" || m.DevDependencies[name] != "" {
			c.Test, c.TestOne = exec+runner, exec+runner+" "+Placeholder
			break
		}
	}
	if c.Test == "" && m.Scripts["test"] != "" {
		c.Test = pm + " run test"
	}
	if m.Scripts["lint"] != "" {
		c.Lint = pm + " run lint"
	}
	return c, nil
}

func (c Config) Commands() []Command {
	all := []Command{{"setup", c.Setup}, {"lint", c.Lint}, {"test", c.Test}, {"test_one", c.TestOne}}
	return slices.DeleteFunc(all, func(cmd Command) bool { return cmd.Run == "" })
}

func (c Config) Digest() string {
	var b strings.Builder
	for _, cmd := range c.Commands() {
		b.WriteString(cmd.Name + "\x00" + cmd.Run + "\x00")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
