package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Pleasurecruise/eyeful/local/app"
)

type jevCmd struct{}

func (c *jevCmd) Run() error {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return fmt.Errorf("stdin: %w", err)
	}
	if stat.Mode()&os.ModeCharDevice != 0 {
		if _, err := fmt.Fprint(os.Stderr, "TypeSafe API key: "); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("read the key: %w", err)
	}
	if err := app.SetJev(strings.TrimSpace(line)); err != nil {
		return err
	}
	if _, err := fmt.Println("Saved. Reviews now merge findings with Jev."); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
