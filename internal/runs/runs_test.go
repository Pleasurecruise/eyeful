package runs

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestStore(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			data, ok := objects[r.URL.EscapedPath()]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(data)
		case http.MethodPut:
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			objects[r.URL.EscapedPath()] = data
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	s := New(srv.URL + "/")
	if _, ok, err := s.Load(t.Context(), "plan/0"); ok || err != nil {
		t.Fatalf("missing key: %v %v", ok, err)
	}
	if err := s.Save(t.Context(), "repro/0/correctness/a b", []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	data, ok, err := s.Load(t.Context(), "repro/0/correctness/a b")
	if !ok || err != nil || string(data) != `{"x":1}` {
		t.Fatalf("%s %v %v", data, ok, err)
	}
	if _, ok := objects["/repro/0/correctness/a%20b"]; !ok {
		t.Fatalf("keys %v", objects)
	}
	for _, key := range []string{"", "../x", "a//b", "a/./b"} {
		if err := s.Save(t.Context(), key, nil); !errors.Is(err, ErrInvalidKey) {
			t.Fatalf("%q: %v", key, err)
		}
	}
}
