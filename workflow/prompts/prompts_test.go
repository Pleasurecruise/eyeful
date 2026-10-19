package prompts_test

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
)

func TestLoad(t *testing.T) {
	s, err := prompts.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.New(slog.New(slog.DiscardHandler), s.Roster()); err != nil {
		t.Fatalf("roster: %v", err)
	}
	for _, name := range []string{"correctness", "design", "generalist", "readability", "security", "tests", "usability"} {
		e, ok := s.Expert(name)
		if !ok || len(e.Skills) == 0 || e.Body == "" {
			t.Errorf("%s: %+v", name, e)
		}
	}
	if len(s.Roles) != 6 || len(s.Skills) != 22 {
		t.Fatalf("roles %d, skills %d", len(s.Roles), len(s.Skills))
	}
	for name, r := range s.Roles {
		if (r.Body == "") != (name == workflow.RolePlanner) {
			t.Errorf("role %s: %+v", name, r)
		}
	}
	sk, ok := s.Skill("security-review")
	if !ok || sk.Description == "" || sk.Body == "" || strings.HasPrefix(sk.Body, "---") {
		t.Fatalf("security-review: %+v", sk)
	}
}

func TestReadSkill(t *testing.T) {
	text, err := prompts.ReadSkill("security-review", "")
	if err != nil || !strings.Contains(text, "name: security-review") {
		t.Fatalf("SKILL.md: %v", err)
	}
	if _, err := prompts.ReadSkill("security-review", "references/injection.md"); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{{"security-review", "../tdd/SKILL.md"}, {"../experts", "security.md"}, {"nope", ""}} {
		if _, err := prompts.ReadSkill(c[0], c[1]); !errors.Is(err, prompts.ErrUnknownSkill) {
			t.Errorf("%v: %v", c, err)
		}
	}
}
