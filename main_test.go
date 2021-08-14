package main

import (
	"testing"

	heroku "github.com/heroku/heroku-go/v5"
)

func Test_Prepare(t *testing.T) {
	withHarness(t, "go-simple", func(t *testing.T, app string, h *heroku.Service) {
		// TODO
	})
}
