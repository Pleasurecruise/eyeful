package prompts

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func input[T task](instructions string, t T) (string, error) {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode task: %w", err)
	}
	return strings.TrimSpace(instructions) + "\n\n## Input\n\n```json\n" + string(data) + "\n```\n", nil
}

func (s Set) offer(dir string, names []string) (string, error) {
	if dir == "" || len(names) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("\n\n## Skills\n\nThe skills offered to you are files on disk; read them with your read-only tools in place of `load_skill` and `read_skill_file`.\n\n")
	for _, name := range names {
		k, ok := s.Skill(name)
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrUnknownSkill, name)
		}
		fmt.Fprintf(&b, "- `%s`, `%s`: %s\n", name, filepath.Join(dir, name, "SKILL.md"), k.Description)
	}
	return b.String(), nil
}

func (s Set) Diagnose(t workflow.DiagnoseTask) (string, error) {
	return input(s.Roles[workflow.RoleCI].Body, t)
}

func (s Set) Plan(t workflow.PlanTask, patchPath string) (string, json.RawMessage, error) {
	prompt, err := s.core.Prompt(t.Diff, patchPath)
	if err != nil {
		return "", nil, err
	}
	if len(t.Rejected) > 0 {
		prompt += "\n\nAn earlier answer was rejected: " + strings.Join(t.Rejected, "; ")
	}
	schema, err := s.core.AnalysisJSONSchema()
	return prompt, schema, err
}

func (s Set) Review(t workflow.ReviewTask, skills string) (string, error) {
	e, ok := s.Expert(t.Expert.Name)
	if !ok {
		return "", fmt.Errorf("%w: no prompt for expert %q", ErrInvalidPrompt, t.Expert.Name)
	}
	var b strings.Builder
	b.WriteString(e.Body + "\n\n")
	fmt.Fprintf(&b, "Evidence kinds allowed for you: %v.\n\n", t.Expert.Evidence)
	if t.TestOne == "" {
		b.WriteString("No reproduction command is configured, so do not use evidence `repro_test`.\n\n")
	}
	if len(t.Rejected) > 0 {
		fmt.Fprintf(&b, "Your previous report was rejected: %s.\n\n", strings.Join(t.Rejected, "; "))
	}
	offered, err := s.offer(skills, t.Skills)
	if err != nil {
		return "", err
	}
	return input(b.String()+s.Roles[workflow.RoleExpert].Body+offered, t)
}

func (s Set) Fix(t workflow.FixTask) (string, error) {
	return input(s.Roles[workflow.RoleFix].Body, t)
}

func (s Set) Retest(t workflow.RetestTask, skills string) (string, error) {
	role := s.Roles[workflow.RoleRepro]
	offered, err := s.offer(skills, workflow.SkillsFor(role.Skills, []string{t.Finding.Path}))
	if err != nil {
		return "", err
	}
	return input(role.Body+offered, t)
}

func (s Set) Judge(t workflow.JudgeTask) (string, error) {
	return input(s.Roles[workflow.RoleJudge].Body, t)
}

func (s Set) Summarize(t workflow.SummaryTask) (string, error) {
	return input(s.Roles[workflow.RoleSummarizer].Body, t)
}
