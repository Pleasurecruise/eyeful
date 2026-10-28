package main

import (
	"github.com/Pleasurecruise/eyeful/local/app"
)

func (Jev) Ready() (bool, error) {
	s, err := app.Current()
	return s.TypeSafeKey != "", err
}

func (Jev) Save(key string) error {
	return app.SetJev(key)
}
