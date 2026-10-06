package prompts

import (
	"embed"
	"errors"

	"github.com/Pleasurecruise/eyeful/workflow"
)

//go:embed experts/*.md roles/*.md tools.yaml all:skills skills-lock.json
var files embed.FS

type Expert struct {
	workflow.Expert
	Body string
}

type Skill struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Path        string `yaml:"-" json:"path"`
}

type LockEntry struct {
	Source       string `json:"source"`
	SourceURL    string `json:"sourceUrl,omitempty"`
	Ref          string `json:"ref,omitempty"`
	SourceType   string `json:"sourceType"`
	SkillPath    string `json:"skillPath,omitempty"`
	ComputedHash string `json:"computedHash"`
}

type Lock struct {
	Version int                  `json:"version"`
	Skills  map[string]LockEntry `json:"skills"`
}

type Tool struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Role struct {
	Role   workflow.Role `yaml:"role"`
	Tools  []string      `yaml:"tools"`
	Submit string        `yaml:"submit"`
	Body   string        `yaml:"-"`
}

type task interface {
	workflow.DiagnoseTask | workflow.PlanTask | workflow.ReviewTask | workflow.FixTask | workflow.JudgeTask | workflow.SummaryTask
}

type Set struct {
	Experts []Expert
	Skills  []Skill
	Lock    Lock
	Tools   []Tool
	Roles   map[workflow.Role]Role
}

type front struct {
	Name     string                  `yaml:"name"`
	Area     string                  `yaml:"area"`
	Tier     workflow.Tier           `yaml:"tier"`
	Evidence []workflow.EvidenceKind `yaml:"evidence"`
	Skills   []workflow.Skill        `yaml:"skills"`
}

var roles = []workflow.Role{
	workflow.RoleCI, workflow.RolePlanner, workflow.RoleExpert, workflow.RoleFix, workflow.RoleJudge, workflow.RoleSummarizer,
}

var (
	ErrInvalidPrompt = errors.New("invalid prompt file")
	ErrUnknownSkill  = errors.New("unknown skill")
	ErrUnknownTool   = errors.New("unknown tool")
	ErrSkillModified = errors.New("skill differs from skills-lock.json")
)
