package workflow

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/sourcegraph/go-diff/diff"
)

const (
	maxPatch = 512 << 10
)

var (
	lockfiles = []string{
		"pnpm-lock.yaml", "package-lock.json", "yarn.lock", "bun.lock", "bun.lockb", "go.sum",
		"Cargo.lock", "poetry.lock", "uv.lock", "Pipfile.lock", "Gemfile.lock", "composer.lock",
	}
	vendoredGlobs  = []string{"**/vendor/**", "**/node_modules/**", "**/third_party/**"}
	generatedGlobs = []string{"**/*.pb.go", "**/*_gen.go", "**/*.gen.go", "**/zz_generated*.go", "**/*.min.js", "**/*.min.css"}
	testGlobs      = []string{
		"**/*_test.go", "**/test_*.py", "**/*_test.py", "**/*.test.*", "**/*.spec.*",
		"**/tests/**", "**/__tests__/**", "**/testdata/**",
	}
	docsGlobs       = []string{"**/*.md", "**/*.mdx", "**/*.rst", "**/*.txt", "docs/**"}
	generatedMarker = regexp.MustCompile(`(?m)^\+.*(Code generated .* DO NOT EDIT\.|@generated)`)
)

func SplitPatch(patch string) ([]File, error) {
	fds, err := diff.ParseMultiFileDiff([]byte(patch))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPatch, err)
	}
	if len(fds) == 0 && strings.TrimSpace(patch) != "" {
		return nil, fmt.Errorf("%w: no file diffs in the input", ErrInvalidPatch)
	}
	files := make([]File, 0, len(fds))
	for _, fd := range fds {
		name := fd.NewName
		if name == "/dev/null" {
			name = fd.OrigName
		}
		name = strings.TrimPrefix(strings.TrimPrefix(name, "a/"), "b/")
		if name == "" {
			return nil, fmt.Errorf("%w: a diff without a path", ErrInvalidPatch)
		}
		text, err := diff.PrintFileDiff(fd)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrInvalidPatch, name, err)
		}
		st := fd.Stat()
		files = append(files, File{
			Path:    name,
			Added:   int(st.Added + st.Changed),
			Deleted: int(st.Deleted + st.Changed),
			Binary: slices.ContainsFunc(fd.Extended, func(h string) bool {
				return strings.HasPrefix(h, "Binary files ") || h == "GIT binary patch"
			}),
			Diff: string(text),
		})
	}
	return files, nil
}

func matchAny(patterns []string, p string) bool {
	return slices.ContainsFunc(patterns, func(g string) bool { return doublestar.MatchUnvalidated(g, p) })
}

func classify(f File, skip []string) Class {
	switch {
	case f.Binary:
		return ClassBinary
	case len(f.Diff) > maxPatch:
		return ClassOversized
	case matchAny(skip, f.Path):
		return ClassSkipped
	case matchAny(vendoredGlobs, f.Path):
		return ClassVendored
	case slices.Contains(lockfiles, path.Base(f.Path)):
		return ClassLock
	case matchAny(generatedGlobs, f.Path), generatedMarker.MatchString(f.Diff):
		return ClassGenerated
	case matchAny(testGlobs, f.Path):
		return ClassTest
	case matchAny(docsGlobs, f.Path):
		return ClassDocs
	default:
		return ClassCode
	}
}

func triage(c Change, p Project) Triage {
	t := Triage{Reviewed: []Entry{}, Excluded: []Entry{}, Risk: []string{}}
	for _, f := range c.Files {
		e := Entry{Path: f.Path, Class: classify(f, p.Skip), Added: f.Added, Deleted: f.Deleted}
		switch e.Class {
		case ClassCode, ClassTest, ClassDocs:
			t.Reviewed = append(t.Reviewed, e)
		case ClassLock, ClassGenerated, ClassVendored, ClassBinary, ClassSkipped, ClassOversized:
			t.Excluded = append(t.Excluded, e)
		}
		if matchAny(p.Risk, f.Path) {
			t.Risk = append(t.Risk, f.Path)
		}
	}
	return t
}
