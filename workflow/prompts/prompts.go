package prompts

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func parse[T Role | front | Skill](p string, strict bool) (T, string, error) {
	var v T
	data, err := fs.ReadFile(files, p)
	if err != nil {
		return v, "", fmt.Errorf("%w: %s: %w", ErrInvalidPrompt, p, err)
	}
	text, opened := strings.CutPrefix(string(data), "---\n")
	head, body, closed := strings.Cut(text, "\n---\n")
	if !opened || !closed {
		return v, "", fmt.Errorf("%w: %s has no frontmatter", ErrInvalidPrompt, p)
	}
	dec := yaml.NewDecoder(strings.NewReader(head))
	dec.KnownFields(strict)
	if err := dec.Decode(&v); err != nil {
		return v, "", fmt.Errorf("%w: %s: %w", ErrInvalidPrompt, p, err)
	}
	return v, strings.TrimSpace(body), nil
}

func Load() (Set, error) {
	s := Set{Roles: map[workflow.Role]Role{}}
	if err := loadRoles(&s); err != nil {
		return Set{}, err
	}
	if err := loadSkills(&s); err != nil {
		return Set{}, err
	}
	if err := loadExperts(&s); err != nil {
		return Set{}, err
	}
	return s, nil
}

func loadRoles(s *Set) error {
	data, err := fs.ReadFile(files, "tools.yaml")
	if err != nil {
		return fmt.Errorf("tools: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s.Tools); err != nil {
		return fmt.Errorf("%w: tools.yaml: %w", ErrInvalidPrompt, err)
	}
	paths, err := fs.Glob(files, "roles/*.md")
	if err != nil {
		return fmt.Errorf("roles: %w", err)
	}
	for _, p := range paths {
		r, body, err := parse[Role](p, true)
		if err != nil {
			return err
		}
		r.Body = body
		for _, name := range append(slices.Clone(r.Tools), r.Submit) {
			if !slices.ContainsFunc(s.Tools, func(t Tool) bool { return t.Name == name }) {
				return fmt.Errorf("%w: %s names %q", ErrUnknownTool, p, name)
			}
		}
		if _, dup := s.Roles[r.Role]; dup || !slices.Contains(roles, r.Role) {
			return fmt.Errorf("%w: %s declares role %q", ErrInvalidPrompt, p, r.Role)
		}
		s.Roles[r.Role] = r
	}
	if len(s.Roles) != len(roles) {
		return fmt.Errorf("%w: %d role files for %d roles", ErrInvalidPrompt, len(s.Roles), len(roles))
	}
	return nil
}

func loadSkills(s *Set) error {
	lock, err := fs.ReadFile(files, "skills-lock.json")
	if err != nil {
		return fmt.Errorf("skills lock: %w", err)
	}
	if err := json.Unmarshal(lock, &s.Lock); err != nil {
		return fmt.Errorf("%w: skills-lock.json: %w", ErrInvalidPrompt, err)
	}
	dirs, err := fs.ReadDir(files, "skills")
	if err != nil {
		return fmt.Errorf("skills: %w", err)
	}
	if len(dirs) != len(s.Lock.Skills) {
		return fmt.Errorf("%w: %d skill directories for %d lock entries", ErrSkillModified, len(dirs), len(s.Lock.Skills))
	}
	for _, name := range slices.Sorted(maps.Keys(s.Lock.Skills)) {
		dir := path.Join("skills", name)
		sum, err := hash(dir)
		if err != nil {
			return err
		}
		if sum != s.Lock.Skills[name].ComputedHash {
			return fmt.Errorf("%w: %s", ErrSkillModified, name)
		}
		sk, _, err := parse[Skill](path.Join(dir, "SKILL.md"), false)
		if err != nil {
			return err
		}
		if sk.Name != name || sk.Description == "" {
			return fmt.Errorf("%w: %s needs a name matching its directory and a description", ErrInvalidPrompt, name)
		}
		sk.Path = dir
		s.Skills = append(s.Skills, sk)
	}
	return nil
}

func loadExperts(s *Set) error {
	experts, err := fs.Glob(files, "experts/*.md")
	if err != nil {
		return fmt.Errorf("experts: %w", err)
	}
	for _, p := range experts {
		f, body, err := parse[front](p, true)
		if err != nil {
			return err
		}
		for _, sk := range f.Skills {
			if _, ok := s.Skill(sk.Name); !ok {
				return fmt.Errorf("%w: %s names %q", ErrUnknownSkill, p, sk.Name)
			}
		}
		s.Experts = append(s.Experts, Expert{Name: f.Name, Area: f.Area, Tier: f.Tier, Evidence: f.Evidence, Skills: f.Skills, Body: body})
	}
	return nil
}

func hash(dir string) (string, error) {
	var paths []string
	err := fs.WalkDir(files, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return fs.SkipDir
		}
		if d.Type().IsRegular() {
			paths = append(paths, strings.TrimPrefix(p, dir+"/"))
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrUnknownSkill, dir, err)
	}
	slices.SortFunc(paths, func(a, b string) int {
		return cmp.Or(strings.Compare(strings.ToLower(a), strings.ToLower(b)), strings.Compare(a, b))
	})
	h := sha256.New()
	for _, p := range paths {
		data, err := fs.ReadFile(files, path.Join(dir, p))
		if err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
		h.Write([]byte(p))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s Set) Roster() []workflow.Expert {
	out := make([]workflow.Expert, len(s.Experts))
	for i, e := range s.Experts {
		out[i] = e.Expert
	}
	return out
}

func (s Set) Expert(name string) (Expert, bool) {
	i := slices.IndexFunc(s.Experts, func(e Expert) bool { return e.Name == name })
	if i < 0 {
		return Expert{}, false
	}
	return s.Experts[i], true
}

func (s Set) Skill(name string) (Skill, bool) {
	i := slices.IndexFunc(s.Skills, func(k Skill) bool { return k.Name == name })
	if i < 0 {
		return Skill{}, false
	}
	return s.Skills[i], true
}

func WriteSkills(dir string) error {
	sub, err := fs.Sub(files, "skills")
	if err != nil {
		return fmt.Errorf("skills: %w", err)
	}
	if err := os.CopyFS(dir, sub); err != nil {
		return fmt.Errorf("write skills: %w", err)
	}
	return nil
}

func ReadSkill(name, file string) (string, error) {
	if file == "" {
		file = "SKILL.md"
	}
	p := path.Clean(path.Join("skills", name, file))
	if !strings.HasPrefix(p, path.Join("skills", name)+"/") || strings.Contains(name, "/") {
		return "", fmt.Errorf("%w: %s/%s", ErrUnknownSkill, name, file)
	}
	data, err := fs.ReadFile(files, p)
	if err != nil {
		return "", fmt.Errorf("%w: %s/%s", ErrUnknownSkill, name, file)
	}
	return string(data), nil
}
