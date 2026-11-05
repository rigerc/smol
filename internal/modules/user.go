package modules

import "context"

type userModule struct{}

func (userModule) Fetch(_ context.Context, env Env) (string, error) {
	for _, k := range []string{"USER", "LOGNAME"} {
		if v := env.Getenv(k); v != "" {
			return v, nil
		}
	}
	return "", ErrUnavailable
}
