package db

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

func (r Row) Text(name string) (string, bool) {
	text, ok := r.values[name]
	if !ok || text == nil {
		return "", false
	}
	return *text, true
}

func (r Row) Int(name string) (int64, bool, error) {
	text, ok := r.Text(name)
	if !ok {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("column %s: %w", name, err)
	}
	return n, true, nil
}

func (r Row) Int32(name string) (int32, error) {
	n, ok, err := r.Int(name)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("column %s is null", name)
	}
	if n > math.MaxInt32 || n < math.MinInt32 {
		return 0, fmt.Errorf("column %s out of range", name)
	}
	return int32(n), nil
}

func (r Row) Time(name string) (time.Time, error) {
	n, ok, err := r.Int(name)
	if err != nil || !ok {
		return time.Time{}, err
	}
	if n == 0 {
		return time.Time{}, nil
	}
	return time.UnixMilli(n).UTC(), nil
}
