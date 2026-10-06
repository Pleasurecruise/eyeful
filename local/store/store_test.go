package store_test

import (
	"path/filepath"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/store"
)

func TestConfirm(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "eyeful")
	if ok, err := store.Confirmed(dir, "/repo", "a"); ok || err != nil {
		t.Fatalf("before: %v, %v", ok, err)
	}
	if err := store.Confirm(dir, "/repo", "a"); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.Confirmed(dir, "/repo", "a"); !ok || err != nil {
		t.Fatalf("after: %v, %v", ok, err)
	}
	if ok, _ := store.Confirmed(dir, "/repo", "b"); ok {
		t.Fatal("another digest counted as confirmed")
	}
	if ok, _ := store.Confirmed(dir, "/other", "a"); ok {
		t.Fatal("a digest confirmed in another repository counted as confirmed")
	}
}
