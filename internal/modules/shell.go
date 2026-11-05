package modules

import (
	"context"
	"path"
)

type shellModule struct{}

func (shellModule) Fetch(_ context.Context, env Env) (string, error) {
	s := env.Getenv("SHELL")
	if s == "" {
		return "", ErrUnavailable
	}
	return path.Base(s), nil
}
