package pulls

import (
	"errors"

	"github.com/Calcium-Ion/moejs"
)

type Core struct {
	mod *moejs.Module
}

type FileChangeStatus string

type DiffCategory string

type DiffSide string

type SourceRef struct {
	Kind   string `json:"kind"`
	Repo   string `json:"repo"`
	Target string `json:"target"`
}

type DiffHunk struct {
	Header   string `json:"header"`
	OldStart int    `json:"oldStart"`
	OldLines int    `json:"oldLines"`
	NewStart int    `json:"newStart"`
	NewLines int    `json:"newLines"`
	Patch    string `json:"patch"`
}

type FileChange struct {
	Path         string           `json:"path"`
	PreviousPath string           `json:"previousPath,omitempty"`
	Status       FileChangeStatus `json:"status"`
	Additions    int              `json:"additions"`
	Deletions    int              `json:"deletions"`
	IsBinary     bool             `json:"isBinary"`
	Sha          string           `json:"sha"`
	Hunks        []DiffHunk       `json:"hunks"`
	Truncated    bool             `json:"truncated,omitempty"`
}

type Commit struct {
	Sha     string `json:"sha"`
	Message string `json:"message"`
}

type DiffsPayload struct {
	Ref     SourceRef    `json:"ref"`
	Title   string       `json:"title"`
	Commits []Commit     `json:"commits,omitempty"`
	Files   []FileChange `json:"files"`
}

type FileNote struct {
	Path     string `json:"path"`
	Text     string `json:"text"`
	Critical bool   `json:"critical,omitempty"`
}

type LineNote struct {
	Path     string   `json:"path"`
	Side     DiffSide `json:"side"`
	Line     int      `json:"line"`
	Text     string   `json:"text"`
	Critical bool     `json:"critical,omitempty"`
}

type DiffGroupLeaf struct {
	Key       string       `json:"key"`
	Label     string       `json:"label"`
	Summary   string       `json:"summary,omitempty"`
	Category  DiffCategory `json:"category"`
	FilePaths []string     `json:"filePaths"`
	Critical  bool         `json:"critical,omitempty"`
	FileNotes []FileNote   `json:"fileNotes,omitempty"`
	LineNotes []LineNote   `json:"lineNotes,omitempty"`
}

type DiffGroup struct {
	DiffGroupLeaf
	Children []DiffGroupLeaf `json:"children,omitempty"`
}

type Analysis struct {
	OverallSummary string      `json:"overallSummary"`
	Groups         []DiffGroup `json:"groups"`
}

type checked struct {
	Problem  string   `json:"problem"`
	Analysis Analysis `json:"analysis"`
}

var ErrCore = errors.New("pulls.review core failed")
