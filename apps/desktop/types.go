package main

import (
	"errors"
	"sync"
)

type Worktree struct {
	mu  sync.Mutex
	dir string
}

type Changes struct {
	Root  string `json:"root"`
	Base  string `json:"base"`
	Patch string `json:"patch"`
}

type Agents struct{}

type Commit struct {
	tree *Worktree
}

type Review struct {
	tree    *Worktree
	mu      sync.Mutex
	running bool
	answers chan bool
}

type emitter string

const (
	EventLog     = "review:log"
	EventConfirm = "review:confirm"
)

var ErrRunning = errors.New("a review is already running")
