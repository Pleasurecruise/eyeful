package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func Read(root string) (Config, bool, error) {
	data, err := os.ReadFile(filepath.Join(root, Path))
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read %s: %w", Path, err)
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, false, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	if c.TestOne != "" && !strings.Contains(c.TestOne, Placeholder) {
		return Config{}, false, fmt.Errorf("%w: test_one must contain %s", ErrInvalidConfig, Placeholder)
	}
	return c, true, nil
}

func (c Config) Commands() []Command {
	all := []Command{
		{"setup", c.Setup}, {"lint", c.Lint}, {"test", c.Test}, {"test_one", c.TestOne},
		{"coverage", c.Coverage}, {"e2e", c.E2E},
	}
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

func (c Config) Project(execution bool) workflow.Project {
	p := workflow.Project{Skip: c.Skip, Risk: c.Risk}
	if execution {
		p.TestOne = c.TestOne
	}
	return p
}
