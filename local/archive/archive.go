package archive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Dir struct {
	path string
}

var ErrInvalidKey = errors.New("invalid archive key")

func New(path string) Dir { return Dir{path: path} }

func (d Dir) file(key string) (string, error) {
	p := filepath.Clean(filepath.FromSlash(key))
	if key == "" || filepath.IsAbs(p) || p == ".." || strings.HasPrefix(p, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return filepath.Join(d.path, p+".json"), nil
}

func (d Dir) Load(_ context.Context, key string) ([]byte, bool, error) {
	f, err := d.file(key)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(f)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load %s: %w", key, err)
	}
	return data, true, nil
}

func (d Dir) Save(_ context.Context, key string, data []byte) error {
	f, err := d.file(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return fmt.Errorf("save %s: %w", key, err)
	}
	tmp := f + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("save %s: %w", key, err)
	}
	if err := os.Rename(tmp, f); err != nil {
		return fmt.Errorf("save %s: %w", key, err)
	}
	return nil
}
