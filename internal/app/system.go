package app

import (
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/rigerc/smol/internal/modules"
)

// System supplies production hooks at the application boundary.
func System(runtime Runtime) Runtime {
	runtime.Now = time.Now
	runtime.Statfs = func(path string) (uint64, uint64, error) {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			return 0, 0, err
		}
		if stat.Bsize <= 0 {
			return 0, 0, fmt.Errorf("statfs: invalid block size %d", stat.Bsize)
		}
		blockSize := uint64(stat.Bsize)
		return stat.Blocks * blockSize, stat.Bavail * blockSize, nil
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
