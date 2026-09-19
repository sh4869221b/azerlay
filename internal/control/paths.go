package control

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func socketPath(runtime string, create bool) (string, error) {
	path, err := runtimeSocketPath(runtime)
	if err != nil {
		return "", err
	}
	app := filepath.Dir(path)
	if create {
		if err := os.Mkdir(app, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", NewError(ERR_CONTROL_RUNTIME)
		}
	}
	if err := privatePath(app, os.ModeDir|0700); err != nil {
		if !create && errors.Is(err, os.ErrNotExist) {
			return "", NewError(ERR_CONTROL_UNAVAILABLE)
		}
		return "", NewError(ERR_CONTROL_RUNTIME)
	}
	return path, nil
}

func runtimeSocketPath(runtime string) (string, error) {
	if !filepath.IsAbs(runtime) {
		return "", NewError(ERR_CONTROL_RUNTIME)
	}
	runtime = filepath.Clean(runtime)
	if err := privatePath(runtime, os.ModeDir|0700); err != nil {
		return "", NewError(ERR_CONTROL_RUNTIME)
	}
	return filepath.Join(runtime, "azerlay", "control.sock"), nil
}

func privatePath(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return privateInfo(info, mode)
}

func privateInfo(info os.FileInfo, mode os.FileMode) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode() != mode || !ok || !sameUID(stat.Uid) {
		return NewError(ERR_CONTROL_PERMISSION)
	}
	return nil
}

// A replacement at the same pathname belongs to its creator, not this server.
func removeOwnedSocket(path string, created os.FileInfo) error {
	current, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return NewError(ERR_CONTROL_UNAVAILABLE)
	}
	if !os.SameFile(created, current) {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return NewError(ERR_CONTROL_UNAVAILABLE)
	}
	return nil
}
