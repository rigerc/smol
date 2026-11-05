package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCommitFaults(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, stage := range []string{"mkdir", "temp", "write", "chmod", "sync", "close", "recheck", "rename", "published"} {
			t.Run(stage+map[bool]string{true: "-existing", false: "-new"}[existing], func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "sub", "config.toml")
				if existing {
					if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				before, err := inspectTarget(path)
				if err != nil {
					t.Fatal(err)
				}
				sentinel := errors.New("injected " + stage)
				backend := store{fault: func(name string) error {
					if name == stage {
						return sentinel
					}
					return nil
				}}
				published, err := backend.commit(context.Background(), path, before, []byte("new"))
				if !errors.Is(err, sentinel) || published != (stage == "published") {
					t.Fatalf("published %v err %v", published, err)
				}
				data, readErr := os.ReadFile(path) //nolint:gosec // G304: reads the configuration fixture inside t.TempDir().
				switch {
				case published:
					if string(data) != "new" || readErr != nil {
						t.Fatalf("%q %v", data, readErr)
					}
				case existing:
					if string(data) != "old" || readErr != nil {
						t.Fatalf("%q %v", data, readErr)
					}
				case !errors.Is(readErr, os.ErrNotExist):
					t.Fatal(readErr)
				}
				_ = filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if strings.HasSuffix(path, ".tmp") {
						t.Errorf("leftover %s", path)
					}
					return nil
				})
			})
		}
	}
}
func TestDestinationConflicts(t *testing.T) {
	for _, change := range []string{"create", "content", "mode", "inode", "parent", "symlink"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "dir", "config.toml")
			if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if change != "create" {
				if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := inspectTarget(path)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "create", "content":
				err = os.WriteFile(path, []byte("external"), 0o600)
			case "mode":
				err = os.Chmod(path, 0o640) //nolint:gosec // G302: deliberately changes permissions to test conflict detection.
			case "inode":
				err = os.Rename(path, path+".old")
				if err == nil {
					err = os.WriteFile(path, []byte("old"), 0o600)
				}
			case "parent":
				err = os.Rename(filepath.Dir(path), filepath.Dir(path)+".old")
				if err == nil {
					err = os.Mkdir(filepath.Dir(path), 0o700)
				}
			case "symlink":
				err = os.Remove(path)
				if err == nil {
					err = os.Symlink(filepath.Join(root, "other"), path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			published, err := (store{}).commit(context.Background(), path, before, []byte("ours"))
			if err == nil || published {
				t.Fatalf("conflict accepted %v %v", published, err)
			}
		})
	}
}
func TestUnsafeDestinations(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "regular")
	if err := os.WriteFile(regular, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(root, "fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	parentLink := filepath.Join(root, "parent")
	if err := os.Symlink(root, parentLink); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, fifo, root, filepath.Join(parentLink, "regular"), filepath.Join(regular, "config.toml")} {
		if _, err := inspectTarget(path); err == nil {
			t.Fatalf("unsafe %s", path)
		}
	}
}
func TestCommitPublicationRace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before, err := inspectTarget(path)
	if err != nil {
		t.Fatal(err)
	}
	backend := store{fault: func(stage string) error {
		if stage == "rename" {
			return os.WriteFile(path, []byte("external"), 0o600)
		}
		return nil
	}}
	published, err := backend.commit(context.Background(), path, before, []byte("ours"))
	if published || !errors.Is(err, errConflict) {
		t.Fatalf("%v %v", published, err)
	}
	data, _ := os.ReadFile(path) //nolint:gosec // G304: reads the configuration fixture inside t.TempDir().
	if string(data) != "external" {
		t.Fatal(string(data))
	}
}
func TestCommitCancellation(t *testing.T) {
	for _, stage := range []string{"initial", "write", "recheck", "rename", "published"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			before, err := inspectTarget(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if stage == "initial" {
				cancel()
			}
			backend := store{fault: func(name string) error {
				if name == stage {
					cancel()
				}
				return nil
			}}
			published, err := backend.commit(ctx, path, before, []byte("new"))
			wantPublished := stage == "published"
			if published != wantPublished {
				t.Fatalf("%s: published %v err %v", stage, published, err)
			}
			if !published && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestNewParentReplacement(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "new")
	path := filepath.Join(directory, "config.toml")
	snap, err := inspectTarget(path)
	if err != nil {
		t.Fatal(err)
	}
	backend := store{fault: func(stage string) error {
		if stage != "recheck" {
			return nil
		}
		if err := os.Rename(directory, directory+".moved"); err != nil {
			return err
		}
		return os.Mkdir(directory, 0o700)
	}}
	published, err := backend.commit(context.Background(), path, snap, []byte("reviewed"))
	if published || !errors.Is(err, errConflict) {
		t.Fatalf("%v %v", published, err)
	}
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("published through a replaced parent")
	}
	if _, err = os.Stat(filepath.Join(directory+".moved", "config.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("published to a detached parent")
	}
}

func TestNewDirectoryPermissionsUnderUmask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "config.toml")
	snap, err := inspectTarget(path)
	if err != nil {
		t.Fatal(err)
	}
	previous := unix.Umask(0o777)
	defer unix.Umask(previous)
	published, err := (store{}).commit(context.Background(), path, snap, []byte("new"))
	if err != nil || !published {
		t.Fatalf("%v %v", published, err)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil || dir.Mode().Perm() != 0o700 {
		t.Fatalf("parent permissions %v %v", dir, err)
	}
	file, err := os.Stat(path)
	if err != nil || file.Mode().Perm() != 0o600 {
		t.Fatalf("file permissions %v %v", file, err)
	}
}
