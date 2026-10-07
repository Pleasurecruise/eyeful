package pulls_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

const patch = `diff --git a/app/session.py b/app/session.py
index 1111111..2222222 100644
--- a/app/session.py
+++ b/app/session.py
@@ -1,3 +1,3 @@ def is_expired(session, now):
 def is_expired(session, now):
-    return now > session.expires_at
+    return now >= session.expires_at
diff --git a/docs/sessions.md b/docs/sessions.md
index 3333333..4444444 100644
--- a/docs/sessions.md
+++ b/docs/sessions.md
@@ -1 +1 @@
-Sessions expire after expires_at.
+Sessions expire at expires_at.
`

func TestCore(t *testing.T) {
	core, err := pulls.Load()
	if err != nil {
		t.Fatal(err)
	}
	files, err := core.ParsePatch(patch)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Path != "app/session.py" || files[0].Additions != 1 || len(files[0].Hunks) != 1 {
		t.Fatalf("files %+v", files)
	}
	d := pulls.DiffsPayload{Ref: pulls.SourceRef{Kind: "local"}, Title: "fix: expire sessions at the boundary", Commits: []pulls.Commit{{Message: "fix: expire sessions at the boundary"}, {Message: "docs: say so"}}, Files: files}
	prompt, err := core.Prompt(d, "/run/change.diff")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Group by intent", "/run/change.diff", "---MANIFEST--- (2 files, +2/-2)", "---COMMITS--- (2", "session.py  M  +1/-1"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	schema, err := core.AnalysisJSONSchema()
	if err != nil || !json.Valid(schema) || !strings.Contains(string(schema), `"overallSummary"`) {
		t.Fatalf("schema %s: %v", schema, err)
	}

	group := pulls.DiffGroup{Key: "expiry", Label: "Session expiry", Category: "security", Critical: true, FilePaths: []string{"app/session.py"}}
	docs := pulls.DiffGroup{Key: "docs", Label: "Docs", Category: "docs", FilePaths: []string{"docs/sessions.md"}}
	for _, tt := range []struct {
		name    string
		answer  pulls.Analysis
		problem string
	}{
		{"a file in no group", pulls.Analysis{OverallSummary: "s", Groups: []pulls.DiffGroup{group}}, "docs/sessions.md"},
		{"an unknown category", pulls.Analysis{OverallSummary: "s", Groups: []pulls.DiffGroup{group, {Key: "d", Label: "D", Category: "prose", FilePaths: []string{"docs/sessions.md"}}}}, "category"},
		{"a complete grouping", pulls.Analysis{OverallSummary: "s", Groups: []pulls.DiffGroup{group, docs}}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, problem, err := core.Check(d, tt.answer)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(problem, tt.problem) || (tt.problem == "") != (problem == "") {
				t.Fatalf("problem %q", problem)
			}
			if tt.problem == "" && (len(a.Groups) != 2 || !a.Groups[0].Critical) {
				t.Fatalf("analysis %+v", a)
			}
		})
	}
}
