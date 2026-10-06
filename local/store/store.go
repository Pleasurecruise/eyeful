package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func Dir(root string) (string, error) {
	dir := filepath.Join(root, ".eyeful", "runs")
	if err := errors.Join(os.MkdirAll(dir, 0o700), os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*\n"), 0o600)); err != nil {
		return "", fmt.Errorf("run directory: %w", err)
	}
	return dir, nil
}

func confirmation(root, digest string) string {
	sum := sha256.Sum256([]byte(root + "\x00" + digest))
	return hex.EncodeToString(sum[:])
}

func Confirmed(dir, root, digest string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, "confirmed"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read confirmations: %w", err)
	}
	return slices.Contains(strings.Split(string(data), "\n"), confirmation(root, digest)), nil
}

func Confirm(dir, root, digest string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("save confirmation: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "confirmed"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("save confirmation: %w", err)
	}
	if _, err := fmt.Fprintln(f, confirmation(root, digest)); err != nil {
		return errors.Join(fmt.Errorf("save confirmation: %w", err), f.Close())
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save confirmation: %w", err)
	}
	return nil
}
