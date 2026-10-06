package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/settings"
)

func TestSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eyeful", "settings.json")
	if s, err := settings.Load(path); err != nil || s.Agent != "" {
		t.Fatalf("missing file: %+v, %v", s, err)
	}
	want := settings.Settings{Agent: "pi"}
	if err := settings.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := settings.Load(path)
	if err != nil || got.Agent != "pi" {
		t.Fatalf("%+v, %v", got, err)
	}
	if err := os.WriteFile(path, []byte(`{"agent":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.Load(path); err == nil {
		t.Fatal("a broken settings file was accepted")
	}
}
