package profilesource

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ResolveDataHome resolves XDG data placement without accessing the filesystem.
// A relative XDG_DATA_HOME is ignored, as is an empty value.
func ResolveDataHome() (string, error) {
	if data := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(data) {
		return filepath.Clean(data), nil
	}
	home := os.Getenv("HOME")
	if !filepath.IsAbs(home) {
		return "", &Error{Code: ERR_PROFILE_STORAGE, Cause: errUnsafePath}
	}
	return filepath.Join(home, ".local/share"), nil
}

func privateInfo(info os.FileInfo, directory bool) error {
	mode := os.FileMode(0600)
	if directory {
		mode = os.ModeDir | 0700
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if info.Mode() != mode || !ok || uint64(stat.Uid) != uint64(os.Geteuid()) {
		return errUnsafePath
	}
	return nil
}

// checkPrivate rejects symlinks in EVERY app-owned component. os.Root contains
// traversal, but by itself permits symlinks whose targets remain inside root.
func checkPrivate(root *os.Root, name string, directory bool) error {
	parts := strings.Split(name, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return err
		}
		if err := privateInfo(info, i < len(parts)-1 || directory); err != nil {
			return err
		}
	}
	return nil
}

func ensurePrivateDir(root *os.Root, name string) (bool, error) {
	if err := checkPrivate(root, name, true); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := root.Mkdir(name, 0700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, checkPrivate(root, name, true)
		}
		return false, err
	}
	return true, nil
}

func openStore(dataHome string) (*os.Root, error) {
	if !filepath.IsAbs(dataHome) {
		return nil, errUnsafePath
	}
	// Only new ancestors receive a mode; existing HOME/XDG ancestors are untouched.
	if err := os.MkdirAll(dataHome, 0700); err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(dataHome)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if _, err := ensurePrivateDir(parent, "azerlay"); err != nil {
		return nil, err
	}
	root, err := parent.OpenRoot("azerlay")
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"sources", "profiles", "cache"} {
		if _, err := ensurePrivateDir(root, name); err != nil {
			return nil, errors.Join(err, root.Close())
		}
	}
	return root, nil
}

func openPrivateFile(root *os.Root, name string, flags int) (*os.File, error) {
	if err := checkPrivate(root, name, false); err != nil {
		if !errors.Is(err, os.ErrNotExist) || flags&os.O_CREATE == 0 {
			return nil, err
		}
		// The final file may be missing; its containing directory must be safe.
		if err := checkPrivate(root, filepath.Dir(name), true); err != nil {
			return nil, err
		}
	}
	return root.OpenFile(name, flags, 0600)
}

func readPrivate(root *os.Root, name string, limit int64) (data []byte, err error) {
	file, err := openPrivateFile(root, name, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	data, err = io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errCodecLimit
	}
	return data, nil
}

func readIndex(root *os.Root) (diskIndex, error) {
	data, err := readPrivate(root, "cache/source-index.json", maxIndexBytes)
	if errors.Is(err, os.ErrNotExist) {
		return diskIndex{SchemaVersion: storageSchemaVersion, Sources: []diskSource{}}, nil
	}
	if err != nil {
		return diskIndex{}, err
	}
	return decodeIndex(strings.NewReader(string(data)))
}
