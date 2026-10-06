package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

func ask(question string) (bool, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false, fmt.Errorf("stdin: %w", err)
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		return false, errors.New("run in a terminal or pass --yes")
	}
	if _, err := fmt.Fprint(os.Stderr, question+" [y/N] "); err != nil {
		return false, fmt.Errorf("write output: %w", err)
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false, fmt.Errorf("read the answer: %w", err)
	}
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes", nil
}
