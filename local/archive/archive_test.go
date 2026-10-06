package archive_test

import (
	"errors"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/archive"
)

func TestDir(t *testing.T) {
	d := archive.New(t.TempDir())
	if _, ok, err := d.Load(t.Context(), "plan"); ok || err != nil {
		t.Fatalf("empty: %v, %v", ok, err)
	}
	if err := d.Save(t.Context(), "expert/0/security", []byte(`{"runs":[]}`)); err != nil {
		t.Fatal(err)
	}
	if data, ok, err := d.Load(t.Context(), "expert/0/security"); !ok || err != nil || string(data) != `{"runs":[]}` {
		t.Fatalf("load: %q, %v, %v", data, ok, err)
	}
	for _, key := range []string{"", "../outside", "/etc/passwd"} {
		if err := d.Save(t.Context(), key, nil); !errors.Is(err, archive.ErrInvalidKey) {
			t.Errorf("%q: %v", key, err)
		}
	}
}
