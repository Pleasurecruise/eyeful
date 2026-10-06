package project

import "errors"

const (
	Path        = ".eyeful/config.yml"
	Placeholder = "{test}"
)

type Config struct {
	Setup    string   `yaml:"setup"`
	Lint     string   `yaml:"lint"`
	Test     string   `yaml:"test"`
	TestOne  string   `yaml:"test_one"`
	Coverage string   `yaml:"coverage"`
	E2E      string   `yaml:"e2e"`
	Risk     []string `yaml:"risk"`
	Skip     []string `yaml:"skip"`
}

type Command struct {
	Name string
	Run  string
}

var ErrInvalidConfig = errors.New("invalid " + Path)
