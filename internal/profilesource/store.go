package profilesource

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sh4869221b/azerlay/internal/profile"
)

type importArtifact struct {
	path    string
	data    []byte
	staging string
	fresh   bool
}
type importTransaction struct {
	root      *os.Root
	source    diskSource
	artifacts []importArtifact
	owner     *ImportedSource
	owned     []string
}

func encodeArtifacts(source diskSource, bundle profile.ProfileBundle) ([]importArtifact, error) {
	dir := "profiles/" + source.SourceHash + "/"
	b, err := encodeBundleCache(source, bundle)
	if err != nil {
		return nil, err
	}
	result := []importArtifact{{path: dir + "bundle.json", data: b}}
	for i := range bundle.Profiles {
		data, err := encodeProfileCache(source, bundle, i+1)
		if err != nil {
			return nil, err
		}
		result = append(result, importArtifact{path: fmt.Sprintf("%sp%d.json", dir, i+1), data: data})
	}
	return result, nil
}

func (tx *importTransaction) cachesCurrent() (bool, error) {
	readers := make([]io.Reader, 0, len(tx.artifacts))
	for _, artifact := range tx.artifacts {
		data, err := readPrivate(tx.root, artifact.path, maxCacheBytes)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errCodecLimit) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		readers = append(readers, bytes.NewReader(data))
	}
	_, err := decodeCacheSet(tx.source, readers[0], readers[1:])
	return err == nil, nil // A malformed/incompatible derived set is replaceable.
}

func (tx *importTransaction) commit(ctx context.Context, original []byte, ordinal int) (err error) {
	committed := false
	defer func() {
		if !committed {
			for i := len(tx.owned) - 1; i >= 0; i-- {
				if removeErr := tx.root.Remove(tx.owned[i]); !errors.Is(removeErr, os.ErrNotExist) {
					err = errors.Join(err, removeErr)
				}
			}
		}
	}()
	// This read MUST be under the stable flock, never from a constructor snapshot.
	index, err := readIndex(tx.root)
	if err != nil {
		return err
	}
	found := false
	for _, stored := range index.Sources {
		if stored.SourceHash != tx.source.SourceHash {
			continue
		}
		if stored.SoftwareRelease != tx.source.SoftwareRelease || stored.SourceScope != tx.source.SourceScope || stored.ProfileCount != tx.source.ProfileCount {
			return errSourceConflict
		}
		tx.source = stored // Input route, provenance and revisions are historical.
		found = true
		break
	}
	if !found {
		index.Sources = append(index.Sources, tx.source)
	}
	index.Selected = &diskSelection{SourceHash: tx.source.SourceHash, ProfileIndex: ordinal}
	indexData, err := encodeIndex(index)
	if err != nil {
		return err
	}
	current, err := tx.cachesCurrent()
	if err != nil {
		return err
	}
	if current {
		tx.artifacts = nil
	}
	originalPath := "sources/" + tx.source.SourceHash + ".azeron"
	existing, err := readPrivate(tx.root, originalPath, int64(len(original)))
	switch {
	case errors.Is(err, os.ErrNotExist):
		tx.artifacts = append(tx.artifacts, importArtifact{path: originalPath, data: original})
	case err != nil:
		return err
	case !bytes.Equal(existing, original):
		return errSourceConflict
	}
	dir := "profiles/" + tx.source.SourceHash
	fresh, err := ensurePrivateDir(tx.root, dir)
	if err != nil {
		return err
	}
	if fresh {
		tx.owned = append(tx.owned, dir)
	}
	tx.artifacts = append(tx.artifacts, importArtifact{path: "cache/source-index.json", data: indexData})
	// Stage the COMPLETE set before publishing any of it, with the index last.
	for i := range tx.artifacts {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := tx.stage(&tx.artifacts[i]); err != nil {
			return err
		}
	}
	for _, artifact := range tx.artifacts {
		tx.owner.event(publicationEvent{phase: "rename", path: artifact.path, staging: artifact.staging})
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := tx.root.Rename(artifact.staging, artifact.path); err != nil {
			return err
		}
		if artifact.fresh {
			tx.owned = append(tx.owned, artifact.path)
		}
	}
	committed = true
	return nil
}

func (tx *importTransaction) stage(artifact *importArtifact) error {
	err := checkPrivate(tx.root, artifact.path, false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	artifact.fresh = errors.Is(err, os.ErrNotExist)
	artifact.staging = filepath.Join(filepath.Dir(artifact.path), "."+filepath.Base(artifact.path)+"."+rand.Text()+".tmp")
	file, err := openPrivateFile(tx.root, artifact.staging, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	tx.owned = append(tx.owned, artifact.staging)
	tx.owner.event(publicationEvent{phase: "write", path: artifact.path, file: file})
	n, writeErr := file.Write(artifact.data)
	if writeErr == nil && n != len(artifact.data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr != nil {
		return errors.Join(writeErr, file.Close())
	}
	tx.owner.event(publicationEvent{phase: "sync", path: artifact.path, file: file})
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	tx.owner.event(publicationEvent{phase: "close", path: artifact.path, file: file})
	return file.Close()
}
