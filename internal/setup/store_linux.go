package setup

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

var errConflict = errors.New("destination changed; refresh and review before saving")

type identity struct {
	Device uint64
	Inode  uint64
	Mode   uint32
}
type snapshot struct {
	exists  bool
	target  identity
	digest  [32]byte
	parents map[string]identity
	data    []byte
}
type store struct{ fault func(string) error }

func fileIdentity(st *unix.Stat_t) identity {
	return identity{Device: uint64(st.Dev), Inode: st.Ino, Mode: st.Mode}
}

// parent walks from the filesystem root without following a symlink. Returned
// directory handles keep subsequent operations anchored to the inspected path.
func parent(path string, create bool, created *[]createdDirectory) (*os.File, map[string]identity, error) {
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	dir := ownedFile(fd, "/")
	parents := make(map[string]identity)
	current := "/"
	parts := strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/")
	for _, part := range append([]string{""}, parts...) {
		if part != "" {
			next, openErr := unix.Openat(descriptor(dir), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if errors.Is(openErr, unix.ENOENT) && create {
				if err = unix.Mkdirat(descriptor(dir), part, 0o700); err != nil {
					_ = dir.Close()
					return nil, nil, err
				}
				var made unix.Stat_t
				if err = unix.Fstatat(descriptor(dir), part, &made, unix.AT_SYMLINK_NOFOLLOW); err != nil {
					_ = dir.Close()
					return nil, nil, err
				}
				*created = append(*created, createdDirectory{path: filepath.Join(current, part), identity: fileIdentity(&made)})
				// Enforce the promised mode even under an unusually restrictive umask.
				if err = unix.Fchmodat(descriptor(dir), part, 0o700, unix.AT_SYMLINK_NOFOLLOW); err != nil {
					_ = dir.Close()
					return nil, nil, err
				}
				(*created)[len(*created)-1].identity.Mode = unix.S_IFDIR | 0o700
				next, openErr = unix.Openat(descriptor(dir), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			}
			if openErr != nil {
				_ = dir.Close()
				return nil, parents, openErr
			}
			_ = dir.Close()
			current = filepath.Join(current, part)
			dir = ownedFile(next, current)
		}
		var st unix.Stat_t
		if err = unix.Fstat(descriptor(dir), &st); err != nil {
			_ = dir.Close()
			return nil, nil, err
		}
		parents[current] = fileIdentity(&st)
	}
	return dir, parents, nil
}

func inspectTarget(path string) (snapshot, error) {
	var snap snapshot
	dir, parents, err := parent(path, false, nil)
	snap.parents = parents
	if errors.Is(err, unix.ENOENT) {
		return snap, nil
	}
	if err != nil {
		return snap, err
	}
	defer func() { _ = dir.Close() }()
	return readTarget(dir, filepath.Base(path), snap)
}
func readTarget(dir *os.File, name string, snap snapshot) (snapshot, error) {
	fd, err := unix.Openat(descriptor(dir), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		return snap, nil
	}
	if err != nil {
		return snap, err
	}
	f := ownedFile(fd, name)
	defer func() { _ = f.Close() }()
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err != nil {
		return snap, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return snap, errors.New("destination must be a regular file; symlinks and special files are refused")
	}
	snap.data, err = io.ReadAll(f)
	if err != nil {
		return snap, err
	}
	snap.exists = true
	snap.target = fileIdentity(&st)
	snap.digest = sha256.Sum256(snap.data)
	return snap, nil
}
func matches(before, after snapshot) bool {
	if before.exists != after.exists || before.target != after.target || before.digest != after.digest {
		return false
	}
	for path, id := range before.parents {
		if after.parents[path] != id {
			return false
		}
	}
	return true
}
func (s store) stage(name string) error {
	if s.fault != nil {
		return s.fault(name)
	}
	return nil
}

// commit reports publication separately from cleanup/durability failures. Once
// rename succeeds, cancellation cannot turn the operation into a failed save.
func (s store) commit(ctx context.Context, path string, before snapshot, data []byte) (published bool, err error) {
	if err = ctx.Err(); err != nil {
		return false, err
	}
	fresh, err := inspectTarget(path)
	if err != nil {
		return false, err
	}
	if !matches(before, fresh) {
		return false, errConflict
	}
	var created []createdDirectory
	var dir *os.File
	var temporary string
	defer func() {
		if temporary != "" && dir != nil {
			if cleanErr := unix.Unlinkat(descriptor(dir), temporary, 0); cleanErr != nil {
				err = errors.Join(err, fmt.Errorf("leftover %s: %w", filepath.Join(filepath.Dir(path), temporary), cleanErr))
			}
		}
		if dir != nil {
			err = errors.Join(err, dir.Close())
		}
		if !published {
			for i := len(created) - 1; i >= 0; i-- {
				if cleanErr := removeCreated(created[i]); cleanErr != nil {
					err = errors.Join(err, fmt.Errorf("leftover %s: %w", created[i].path, cleanErr))
				}
			}
		}
	}()
	if err = s.stage("mkdir"); err != nil {
		return false, err
	}
	var parents map[string]identity
	dir, parents, err = parent(path, true, &created)
	if err != nil {
		return false, err
	}
	for name, id := range before.parents {
		if parents[name] != id {
			return false, errConflict
		}
	}
	if err = s.stage("temp"); err != nil {
		return false, err
	}
	// O_EXCL prevents a pre-existing name from being opened or truncated.
	for attempt := 0; attempt < 32; attempt++ {
		temporary = fmt.Sprintf(".smol-%d-%d.tmp", os.Getpid(), attempt)
		fd, openErr := unix.Openat(descriptor(dir), temporary, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
		if errors.Is(openErr, unix.EEXIST) {
			temporary = ""
			continue
		}
		if openErr != nil {
			temporary = ""
			return false, openErr
		}
		f := ownedFile(fd, temporary)
		err = s.write(f, before, data)
		if err != nil {
			return false, err
		}
		break
	}
	if temporary == "" {
		return false, errors.New("could not allocate a temporary config")
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	if err = s.stage("recheck"); err != nil {
		return false, err
	}
	fresh, err = inspectTarget(path)
	if err != nil {
		return false, err
	}
	if !matches(before, fresh) {
		return false, errConflict
	}
	// Include parents created after authorization: publication must still refer
	// to the directory handle we opened, even when an ancestor did not exist at review.
	for name, id := range parents {
		if fresh.parents[name] != id {
			return false, errConflict
		}
	}
	if err = s.stage("rename"); err != nil {
		return false, err
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	// This is the short publication critical section. A signal is observed after
	// its outcome; existing-target rename has the documented check-to-rename race.
	if before.exists {
		err = unix.Renameat(descriptor(dir), temporary, descriptor(dir), filepath.Base(path))
	} else {
		err = unix.Renameat2(descriptor(dir), temporary, descriptor(dir), filepath.Base(path), unix.RENAME_NOREPLACE)
	}
	if errors.Is(err, unix.EEXIST) {
		return false, errConflict
	}
	if err != nil {
		return false, err
	}
	temporary = ""
	published = true
	if err = s.stage("published"); err != nil {
		return true, err
	}
	return true, dir.Sync()
}
func (s store) write(f *os.File, before snapshot, data []byte) (err error) {
	defer func() { err = errors.Join(err, f.Close()) }()
	if err = s.stage("write"); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	mode := uint32(0o600)
	if before.exists {
		mode = before.target.Mode & 0o7777
	}
	if err = s.stage("chmod"); err != nil {
		return err
	}
	if err = unix.Fchmod(descriptor(f), mode); err != nil {
		return err
	}
	if err = s.stage("sync"); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	return s.stage("close")
}

// descriptor converts a descriptor allocated by the kernel, which fits in int.
func descriptor(f *os.File) int              { return int(f.Fd()) }                   //nolint:gosec // G115: these files come from successful unix.Open/Openat calls returning int descriptors.
func ownedFile(fd int, name string) *os.File { return os.NewFile(uintptr(fd), name) } //nolint:gosec // G115: only successful nonnegative unix.Open/Openat descriptors are passed here.

type createdDirectory struct {
	path     string
	identity identity
}

func removeCreated(created createdDirectory) error {
	dir, _, err := parent(created.path, false, nil)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	var actual unix.Stat_t
	if err = unix.Fstatat(descriptor(dir), filepath.Base(created.path), &actual, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if fileIdentity(&actual) != created.identity {
		return errConflict
	}
	return unix.Unlinkat(descriptor(dir), filepath.Base(created.path), unix.AT_REMOVEDIR)
}
