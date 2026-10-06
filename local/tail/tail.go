package tail

import "sync"

type Buffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func New(limit int) *Buffer { return &Buffer{limit: limit} }

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.limit; over > 0 {
		b.buf = b.buf[over:]
	}
	return len(p), nil
}

func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

func Last(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
