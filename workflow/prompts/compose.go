package prompts

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

func input[T task](instructions string, t T) (string, error) {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode task: %w", err)
	}
	return strings.TrimSpace(instructions) + "\n\n## Input\n\n```json\n" + string(data) + "\n```\n", nil
}

func (s Set) Diagnose(t workflow.DiagnoseTask) (string, error) {
	return input(s.Roles[workflow.RoleCI].Body, t)
}

func (s Set) Plan(t workflow.PlanTask) (string, error) {
	var b strings.Builder
	b.WriteString(s.Roles[workflow.RolePlanner].Body + "\n\n## Diff\n\n")
	for _, f := range t.Files {
		b.WriteString(f.Diff)
	}
	if t.Truncated {
		b.WriteString("\n(The rest of the diff is left out; read the files yourself.)\n")
	}
	return input(b.String(), t)
}

func (s Set) Review(t workflow.ReviewTask, skills string) (string, error) {
	e, ok := s.Expert(t.Expert.Name)
	if !ok {
		return "", fmt.Errorf("%w: no prompt for expert %q", ErrInvalidPrompt, t.Expert.Name)
	}
	var b strings.Builder
	b.WriteString(e.Body + "\n\n")
	fmt.Fprintf(&b, "Evidence kinds allowed for you: %v.\n\n", e.Evidence)
	if t.TestOne == "" {
		b.WriteString("No reproduction command is configured, so do not use evidence `repro_test`.\n\n")
	} else {
		fmt.Fprintf(&b, "The reproduction command is `%s`, where %s is your `run` value.\n\n", t.TestOne, project.Placeholder)
	}
	if len(t.Rejected) > 0 {
		fmt.Fprintf(&b, "Your previous report was rejected: %s.\n\n", strings.Join(t.Rejected, "; "))
	}
	b.WriteString(s.Roles[workflow.RoleExpert].Body)
	if skills != "" && len(t.Skills) > 0 {
		b.WriteString("\n\n## Skills\n\nThe skills offered to you are files on disk; read them with your read-only tools in place of `load_skill` and `read_skill_file`.\n\n")
		for _, name := range t.Skills {
			k, ok := s.Skill(name)
			if !ok {
				return "", fmt.Errorf("%w: %s", ErrUnknownSkill, name)
			}
			fmt.Fprintf(&b, "- `%s`, `%s`: %s\n", name, filepath.Join(skills, name, "SKILL.md"), k.Description)
		}
	}
	return input(b.String(), t)
}

func (s Set) Fix(t workflow.FixTask) (string, error) {
	return input(s.Roles[workflow.RoleFix].Body, t)
}

func (s Set) Judge(t workflow.JudgeTask) (string, error) {
	return input(s.Roles[workflow.RoleJudge].Body, t)
}

func (s Set) Summarize(t workflow.SummaryTask) (string, error) {
	return input(s.Roles[workflow.RoleSummarizer].Body, t)
}
