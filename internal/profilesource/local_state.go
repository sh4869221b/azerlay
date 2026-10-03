package profilesource

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const localStateLimit = 24<<20 + 64<<10

// LocalStateSelection identifies the configured selection across restarts.
type LocalStateSelection struct {
	Policy    string
	StorePath string
	Device    string
	File      string
}

type localStateEnvelope struct {
	SchemaVersion int      `json:"schema_version"`
	Policy        string   `json:"policy"`
	Ref           LocalRef `json:"ref"`
	Release       string   `json:"release"`
	Scope         string   `json:"scope"`
	Original      []byte   `json:"original"`
}

func (selection LocalStateSelection) matches(ref LocalRef) error {
	if selection.Policy != "auto" && selection.Policy != "local" || !validLocalRef(SourceRef{Local: ref}) || ref.Device != selection.Device || ref.File != selection.File {
		return errInvalidSelection
	}
	roots, err := localRoots(selection.StorePath)
	if err != nil {
		return err
	}
	for _, root := range roots {
		if ref.Root == root {
			return nil
		}
	}
	return errInvalidSelection
}

// RestoreLocalState reads and re-admits saved bytes without creating state or
// requiring that the original definition still exists.
func RestoreLocalState(selection LocalStateSelection) (ref LocalRef, candidate LocalCandidate, err error) {
	defer func() {
		if err != nil {
			err = &Error{Code: ERR_PROFILE_LOCAL_STATE, Cause: err}
		}
	}()
	home, err := localStateHome(selection)
	if err != nil {
		return ref, candidate, err
	}
	parent, err := os.OpenRoot(home)
	if err != nil {
		return ref, candidate, err
	}
	defer parent.Close()
	data, err := readPrivate(parent, "azerlay/last-good.json", localStateLimit)
	if err != nil {
		return ref, candidate, err
	}
	var envelope localStateEnvelope
	if err := readStorage(bytes.NewReader(data), &envelope, localStateLimit); err != nil {
		return ref, candidate, err
	}
	if envelope.SchemaVersion != 1 || envelope.Policy != selection.Policy || envelope.Release != "2.0.2" || envelope.Scope != "azeron-software-local-json" || len(envelope.Original) > localSourceLimit {
		return ref, candidate, errInvalidSelection
	}
	if err := selection.matches(envelope.Ref); err != nil {
		return ref, candidate, err
	}
	candidate, err = admitLocal(envelope.Original, envelope.Ref.File)
	if err != nil {
		return ref, LocalCandidate{}, err
	}
	return envelope.Ref, candidate, nil
}

// SaveLocalState publishes the complete admitted definition before a controller
// activates it. Cancellation before rename leaves existing state intact.
func SaveLocalState(ctx context.Context, selection LocalStateSelection, ref LocalRef, candidate LocalCandidate) (err error) {
	defer func() {
		if err != nil {
			err = &Error{Code: ERR_PROFILE_LOCAL_STATE, Cause: err}
		}
	}()
	if err := selection.matches(ref); err != nil {
		return err
	}
	if len(candidate.original) == 0 || len(candidate.original) > localSourceLimit {
		return errInvalidSelection
	}
	if _, err := admitLocal(candidate.original, ref.File); err != nil {
		return err
	}
	data, err := json.Marshal(localStateEnvelope{1, selection.Policy, ref, "2.0.2", "azeron-software-local-json", candidate.original})
	if err != nil {
		return err
	}
	if len(data) > localStateLimit {
		return errCodecLimit
	}
	home, err := localStateHome(selection)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	parent, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer parent.Close()
	if _, err := ensurePrivateDir(parent, "azerlay"); err != nil {
		return err
	}
	root, err := parent.OpenRoot("azerlay")
	if err != nil {
		return err
	}
	defer root.Close()
	if err := checkPrivate(root, "last-good.json", false); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	name := ".last-good." + rand.Text() + ".tmp"
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(name)
	n, writeErr := file.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		return errors.Join(writeErr, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return root.Rename(name, "last-good.json")
}

func localStateHome(selection LocalStateSelection) (string, error) {
	home := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(home) {
		if home = os.Getenv("HOME"); !filepath.IsAbs(home) {
			return "", errUnsafePath
		}
		home = filepath.Join(home, ".local/state")
	}
	roots, err := localRoots(selection.StorePath)
	if err != nil {
		return "", err
	}
	destination, err := physicalLocalPath(filepath.Join(home, "azerlay"))
	if err != nil {
		return "", err
	}
	for _, root := range roots {
		source, err := physicalLocalPath(root)
		if err != nil {
			return "", err
		}
		if filepath.IsLocal(relativePath(source, destination)) || filepath.IsLocal(relativePath(destination, source)) {
			return "", errUnsafePath
		}
	}
	return filepath.Clean(home), nil
}

func relativePath(base, path string) string {
	relative, err := filepath.Rel(base, path)
	if err != nil {
		return ".."
	}
	return relative
}

// Resolve existing symlink ancestors before any new destination is created.
func physicalLocalPath(path string) (string, error) {
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) || path == filepath.Dir(path) {
			return "", err
		}
		if _, statErr := os.Lstat(path); statErr == nil {
			return "", errUnsafePath
		}
		tail = append(tail, filepath.Base(path))
		path = filepath.Dir(path)
	}
}
