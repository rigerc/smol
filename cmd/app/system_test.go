package main

import (
	"github.com/rigerc/smol/internal/app"
	"math"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSystemHooks(t *testing.T) {
	r := system(app.Runtime{})
	if r.Now().IsZero() {
		t.Fatal("zero time")
	}
	stats, err := r.Statfs("/")
	if err != nil || stats.Total < stats.Free || stats.Total == 0 {
		t.Fatal(stats, err)
	}
	if _, err := r.Statfs(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing mount error")
	}
	arch, err := r.Uname()
	if err != nil || arch == "" {
		t.Fatal(arch, err)
	}
	if _, err := r.Interfaces(); err != nil {
		t.Fatal(err)
	}
}

func TestDiskStatsValidation(t *testing.T) {
	for _, stat := range []syscall.Statfs_t{{Bsize: 0}, {Bsize: -1}, {Bsize: 4096, Blocks: math.MaxUint64}, {Bsize: 4096, Bfree: math.MaxUint64}, {Bsize: 4096, Bavail: math.MaxUint64}} {
		if _, err := diskStats(stat); err == nil {
			t.Fatal(stat)
		}
	}
	got, err := diskStats(syscall.Statfs_t{Bsize: 1024, Blocks: 100, Bfree: 20, Bavail: 15})
	if err != nil || got.Total != 102400 || got.Free != 20480 || got.Available != 15360 {
		t.Fatal(got, err)
	}
}
