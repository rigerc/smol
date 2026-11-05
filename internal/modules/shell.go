package modules

import (
	"context"
	"path"
	"strings"
)

type shellModule struct{}

func (shellModule) Fetch(_ context.Context, env Env) (Result, error) {
	parent := "self"
	for range 16 {
		stat, err := readTrim(env.FS, "proc/"+parent+"/stat")
		if err != nil {
			break
		}
		end := strings.LastIndex(stat, ")")
		if end < 0 {
			break
		}
		parts := strings.Fields(stat[end+1:])
		if len(parts) < 2 {
			break
		}
		parent = parts[1]
		if parent == "0" {
			break
		}
		comm, err := readTrim(env.FS, "proc/"+parent+"/comm")
		if err != nil {
			break
		}
		switch comm {
		case "bash", "zsh", "fish", "dash", "ksh", "tcsh", "csh", "nu":
			return Result{Text: comm}, nil
		case "sudo", "su", "sh", "env", "time", "strace", "gdb":
			continue
		}
		break
	}
	s := env.Getenv("SHELL")
	if s == "" {
		return Result{}, ErrUnavailable
	}
	return Result{Text: path.Base(s)}, nil
}
