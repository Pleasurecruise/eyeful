package tail_test

import (
	"fmt"
	"testing"

	"github.com/Pleasurecruise/eyeful/local/tail"
)

func TestBuffer(t *testing.T) {
	b := tail.New(4)
	if _, err := fmt.Fprint(b, "abcdef"); err != nil {
		t.Fatal(err)
	}
	if b.String() != "cdef" {
		t.Fatalf("%q", b.String())
	}
	if tail.Last("abcdef", 2) != "…ef" || tail.Last("ab", 2) != "ab" {
		t.Fatal("last")
	}
}
