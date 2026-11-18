package profilesource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
)

// snapshot opens only existing paths. A nil root denotes an absent application;
// the caller owns any returned root, including an application with no index.
func (s *ImportedSource) snapshot(ctx context.Context) (root *os.Root, index diskIndex, err error) {
	defer func() {
		if err != nil {
			if root != nil {
				err = errors.Join(err, root.Close())
				root = nil
			}
			index = diskIndex{}
			err = &Error{Code: ERR_PROFILE_STORAGE, Cause: err}
		}
	}()
	index = diskIndex{SchemaVersion: storageSchemaVersion, Sources: []diskSource{}}
	if err := ctx.Err(); err != nil {
		return nil, index, err
	}
	if !filepath.IsAbs(s.dataHome) {
		return nil, index, errUnsafePath
	}
	parent, err := os.OpenRoot(s.dataHome)
	if errors.Is(err, os.ErrNotExist) {
		return nil, index, nil
	}
	if err != nil {
		return nil, index, err
	}
	defer parent.Close()
	if err := checkPrivate(parent, "azerlay", true); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, index, nil
		}
		return nil, index, err
	}
	root, err = parent.OpenRoot("azerlay")
	if err != nil {
		return root, index, err
	}
	for _, name := range []string{"sources", "profiles", "cache"} {
		if err := checkPrivate(root, name, true); err != nil && !errors.Is(err, os.ErrNotExist) {
			return root, index, err
		}
	}
	index, err = readIndex(root)
	return root, index, err
}

func indexedSource(index diskIndex, ref SourceRef) (diskSource, error) {
	if !validSourceHash(ref.Hash) || ref.Local != (LocalRef{}) {
		return diskSource{}, &Error{Code: ERR_PROFILE_STORAGE, Cause: errCodecShape}
	}
	for _, source := range index.Sources {
		if source.SourceHash == ref.Hash {
			return source, nil
		}
	}
	return diskSource{}, &Error{Code: ERR_PROFILE_NOT_FOUND}
}

// Load returns an owned full model. Even a complete valid cache cannot substitute
// for the immutable original. Recovery uses its saved bytes, never Origin.Path,
// and deliberately performs no disk repair or selection update.
func (s *ImportedSource) Load(ctx context.Context, ref SourceRef) (bundle *profile.ProfileBundle, err error) {
	defer func() {
		if err != nil {
			bundle = nil
			var failure *Error
			if !errors.As(err, &failure) {
				err = &Error{Code: ERR_PROFILE_STORAGE, Cause: err}
			}
		}
	}()
	root, index, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if root != nil {
		defer root.Close()
	}
	source, err := indexedSource(index, ref)
	if err != nil {
		return nil, err
	}
	if source.SoftwareRelease != "2.0.2" || source.SourceScope != "azeron-software-export" {
		return nil, &profileadapter.NormalizeError{Code: profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION}
	}
	// The existing DecodeText/DecodeReader source-byte admission cap is 16 MiB.
	original, err := readPrivate(root, "sources/"+source.SourceHash+".azeron", 16<<20)
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(original)) != source.SourceHash {
		return nil, errSourceConflict
	}
	cached, cacheErr := loadCacheSet(ctx, root, source)
	if cacheErr == nil {
		return &cached, nil
	}
	var codecFailure *codecError
	if !errors.Is(cacheErr, os.ErrNotExist) && !errors.Is(cacheErr, errCodecLimit) && !errors.As(cacheErr, &codecFailure) {
		return nil, cacheErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	metadata := profile.SourceMetadata{SoftwareRelease: source.SoftwareRelease, SourceScope: source.SourceScope}
	var prepared Prepared
	switch source.InputKind {
	case "text":
		prepared, err = PrepareText(string(original), metadata)
	case "reader":
		prepared, err = PrepareReader(bytes.NewReader(original), metadata)
	default:
		return nil, errCodecShape
	}
	if err != nil {
		return nil, err
	}
	if !validBundle(prepared.bundle, source) {
		return nil, errSourceConflict
	}
	return &prepared.bundle, nil
}

func loadCacheSet(ctx context.Context, root *os.Root, source diskSource) (profile.ProfileBundle, error) {
	var bundle profile.ProfileBundle
	dir := "profiles/" + source.SourceHash + "/"
	paths := []string{dir + "bundle.json"}
	for i := range source.ProfileCount {
		paths = append(paths, fmt.Sprintf("%sp%d.json", dir, i+1))
	}
	// Reject unsafe existing members even when another member is absent.
	for _, path := range paths {
		if err := checkPrivate(root, path, false); err != nil && !errors.Is(err, os.ErrNotExist) {
			return bundle, err
		}
	}
	if err := ctx.Err(); err != nil {
		return bundle, err
	}
	if err := readCacheArtifact(root, paths[0], func(reader io.Reader) error {
		var err error
		bundle, err = decodeBundleCache(reader, source)
		return err
	}); err != nil {
		return profile.ProfileBundle{}, err
	}
	for i, path := range paths[1:] {
		if err := ctx.Err(); err != nil {
			return profile.ProfileBundle{}, err
		}
		if err := readCacheArtifact(root, path, func(reader io.Reader) error {
			return checkProfileCache(reader, source, i+1, bundle.Profiles[i])
		}); err != nil {
			return profile.ProfileBundle{}, err
		}
	}
	return bundle, nil
}

// Only one cache file is held open at a time, regardless of bundle size.
func readCacheArtifact(root *os.Root, path string, decode func(io.Reader) error) (err error) {
	file, err := openPrivateFile(root, path, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	// Avoid the read allocation for files already known to exceed the codec cap.
	if info.Size() > maxCacheBytes {
		return errCodecLimit
	}
	return decode(file)
}
