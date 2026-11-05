package modules

import "context"

type hostModule struct{}

func (hostModule) Fetch(_ context.Context, env Env) (Result, error) {
	text, err := readTrim(env.FS, "proc/sys/kernel/hostname")
	return Result{Text: text}, err
}
