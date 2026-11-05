package modules

import "context"

type kernelModule struct{}

func (kernelModule) Fetch(_ context.Context, env Env) (string, error) {
	return readTrim(env.FS, "proc/sys/kernel/osrelease")
}
