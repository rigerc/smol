package main

import (
	"fmt"
	"github.com/rigerc/smol/internal/app"
	"math"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/rigerc/smol/internal/modules"
)

// System supplies production hooks at the application boundary.
func system(runtime app.Runtime) app.Runtime {
	runtime.Now = time.Now
	runtime.Statfs = func(path string) (modules.DiskStats, error) {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			return modules.DiskStats{}, err
		}
		return diskStats(stat)
	}
	runtime.Uname = func() (string, error) {
		var name syscall.Utsname
		if err := syscall.Uname(&name); err != nil {
			return "", err
		}
		var b strings.Builder
		for _, v := range name.Machine {
			if v == 0 {
				break
			}
			b.WriteRune(rune(v))
		}
		return b.String(), nil
	}
	runtime.Interfaces = func() ([]modules.Interface, error) {
		entries, err := net.Interfaces()
		if err != nil {
			return nil, err
		}
		var out []modules.Interface
		for _, entry := range entries {
			if entry.Flags&net.FlagLoopback != 0 || entry.Flags&net.FlagUp == 0 {
				continue
			}
			addresses, err := entry.Addrs()
			if err != nil {
				return nil, err
			}
			item := modules.Interface{Name: entry.Name}
			for _, address := range addresses {
				ip, _, err := net.ParseCIDR(address.String())
				if err != nil || ip.IsLoopback() {
					continue
				}
				if ip.To4() != nil {
					item.IPv4 = append(item.IPv4, ip.String())
				} else {
					item.IPv6 = append(item.IPv6, ip.String())
				}
			}
			out = append(out, item)
		}
		return out, nil
	}
	return runtime
}

func diskStats(stat syscall.Statfs_t) (modules.DiskStats, error) {
	if stat.Bsize <= 0 {
		return modules.DiskStats{}, fmt.Errorf("statfs: invalid block size %d", stat.Bsize)
	}
	blockSize := uint64(stat.Bsize)
	if stat.Blocks > math.MaxUint64/blockSize || stat.Bfree > math.MaxUint64/blockSize || stat.Bavail > math.MaxUint64/blockSize {
		return modules.DiskStats{}, fmt.Errorf("statfs: byte count overflow")
	}
	return modules.DiskStats{Total: stat.Blocks * blockSize, Free: stat.Bfree * blockSize, Available: stat.Bavail * blockSize}, nil
}
