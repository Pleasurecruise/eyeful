package runs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Store struct {
	base   string
	client *http.Client
}

var ErrInvalidKey = errors.New("invalid run key")

func New(base string) Store {
	return Store{base: strings.TrimSuffix(base, "/"), client: &http.Client{Timeout: 30 * time.Second}}
}

func (s Store) url(key string) (string, error) {
	segments := strings.Split(key, "/")
	for i, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("%w: %q", ErrInvalidKey, key)
		}
		segments[i] = url.PathEscape(segment)
	}
	return s.base + "/" + strings.Join(segments, "/"), nil
}

func (s Store) Load(ctx context.Context, key string) ([]byte, bool, error) {
	target, err := s.url(key)
	if err != nil {
		return nil, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, false, fmt.Errorf("load %s: %w", key, err)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("load %s: %w", key, err)
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("load %s: %s", key, res.Status)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, false, fmt.Errorf("load %s: %w", key, err)
	}
	return data, true, nil
}

func (s Store) Save(ctx context.Context, key string, data []byte) error {
	target, err := s.url(key)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("save %s: %w", key, err)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("save %s: %w", key, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("save %s: %s", key, res.Status)
	}
	return nil
}
