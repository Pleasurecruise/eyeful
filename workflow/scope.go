package workflow

import (
	"fmt"
	"path"
	"strings"
)

const (
	largeChange = 2000
	scopeDepth  = 2
)

func size(entries []Entry) int {
	n := 0
	for _, e := range entries {
		n += e.Added + e.Deleted
	}
	return n
}

func scopes(entries []Entry, depth int) []Scope {
	if size(entries) <= largeChange {
		return []Scope{scopeOf(entries)}
	}
	if depth == scopeDepth {
		var out []Scope
		start := 0
		for i := range entries {
			if i > start && size(entries[start:i+1]) > largeChange {
				out = append(out, scopeOf(entries[start:i]))
				start = i
			}
		}
		out = append(out, scopeOf(entries[start:]))
		for i := range out {
			out[i].Name = fmt.Sprintf("%s (part %d of %d)", out[i].Name, i+1, len(out))
		}
		return out
	}
	var keys []string
	buckets := map[string][]Entry{}
	for _, e := range entries {
		key := path.Dir(e.Path)
		if parts := strings.Split(e.Path, "/"); len(parts) > depth+1 {
			key = strings.Join(parts[:depth+1], "/")
		}
		if _, ok := buckets[key]; !ok {
			keys = append(keys, key)
		}
		buckets[key] = append(buckets[key], e)
	}
	var out []Scope
	for _, k := range keys {
		for _, sc := range scopes(buckets[k], depth+1) {
			if last := len(out) - 1; depth == 0 && last >= 0 && out[last].Lines+sc.Lines <= largeChange {
				out[last].Name += ", " + sc.Name
				out[last].Files = append(out[last].Files, sc.Files...)
				out[last].Lines += sc.Lines
				continue
			}
			out = append(out, sc)
		}
	}
	return out
}

func scopeOf(entries []Entry) Scope {
	s := Scope{Files: make([]string, 0, len(entries)), Lines: size(entries)}
	for _, e := range entries {
		s.Files = append(s.Files, e.Path)
	}
	dir := ""
	for i, f := range s.Files {
		d := path.Dir(f)
		if i == 0 {
			dir = d
		}
		for dir != "." && d != dir && !strings.HasPrefix(d, dir+"/") {
			dir = path.Dir(dir)
		}
	}
	s.Name = dir
	if dir == "" || dir == "." {
		s.Name = "/"
	}
	return s
}
