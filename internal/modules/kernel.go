package modules

import "context"

type kernelModule struct{}

func (kernelModule) Fetch(_ context.Context, env Env) (Result, error) {
	text, err := readTrim(env.FS, "proc/sys/kernel/osrelease")
	if err != nil {
		return Result{}, err
	}
	return coreResult("kernel", text, nil, env), nil
}
