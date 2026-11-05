package modules

import "context"

type hostModule struct{}

func (hostModule) Fetch(_ context.Context, env Env) (string, error) {
	return readTrim(env.FS, "proc/sys/kernel/hostname")
}
