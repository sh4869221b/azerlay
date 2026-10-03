package profilesource

import (
	"bytes"
	"context"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const localSourceLimit = 16 << 20
const localReadInterval = 200 * time.Millisecond

// LocalCandidate owns exact source bytes and a normalized bundle. Copies and
// accessor results share storage; trusted callers must treat them as read-only.
type LocalCandidate struct {
	original []byte
	bundle   profile.ProfileBundle
}

func (c LocalCandidate) Original() []byte              { return c.original }
func (c LocalCandidate) Bundle() profile.ProfileBundle { return c.bundle }

// AzeronLocalSource reads only saved definitions inside a resolved userData root.
type AzeronLocalSource struct {
	root string
	// hook is a test-only barrier around real reads.
	hook func(string)
}

func NewAzeronLocalSource(root string) *AzeronLocalSource { return &AzeronLocalSource{root: root} }
func (s *AzeronLocalSource) ID() SourceID                 { return "local" }
func (s *AzeronLocalSource) Load(ctx context.Context, ref SourceRef) (*profile.ProfileBundle, error) {
	candidate, err := s.LoadCandidate(ctx, ref)
	if err != nil {
		return nil, err
	}
	bundle := candidate.Bundle()
	return &bundle, nil
}

// LoadCandidate publishes only two fully admitted identical reads of the same ref.
func (s *AzeronLocalSource) LoadCandidate(ctx context.Context, ref SourceRef) (LocalCandidate, error) {
	if !validLocalRef(ref) || ref.Local.Root != s.root {
		return LocalCandidate{}, &Error{Code: ERR_PROFILE_LOCAL_READ, Cause: errInvalidSelection}
	}
	first, err := s.read(ctx, ref.Local)
	if err != nil {
		return LocalCandidate{}, err
	}
	if s.hook != nil {
		s.hook("between")
	}
	timer := time.NewTimer(localReadInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return LocalCandidate{}, ctx.Err()
	case <-timer.C:
	}
	second, err := s.read(ctx, ref.Local)
	if err != nil {
		return LocalCandidate{}, err
	}
	if !bytes.Equal(first.original, second.original) {
		return LocalCandidate{}, &Error{Code: ERR_PROFILE_LOCAL_READ, Cause: errSourceConflict}
	}
	return second, nil
}

func (s *AzeronLocalSource) read(ctx context.Context, ref LocalRef) (candidate LocalCandidate, err error) {
	defer func() {
		if err != nil && ctx.Err() == nil {
			if _, ok := err.(*Error); !ok {
				err = &Error{Code: ERR_PROFILE_LOCAL_READ, Cause: err}
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return LocalCandidate{}, err
	}
	root, err := os.OpenRoot(ref.Root)
	if err != nil {
		return LocalCandidate{}, err
	}
	defer root.Close()
	path := localRelativePath(ref)
	file, err := root.OpenFile(path, os.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return LocalCandidate{}, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return LocalCandidate{}, err
	}
	if !before.Mode().IsRegular() || before.Size() > localSourceLimit {
		return LocalCandidate{}, errUnsafePath
	}
	if s.hook != nil {
		s.hook("opened")
	}
	data, err := io.ReadAll(io.LimitReader(file, localSourceLimit+1))
	if err != nil {
		return LocalCandidate{}, err
	}
	if s.hook != nil {
		s.hook("read")
	}
	after, err := file.Stat()
	if err != nil {
		return LocalCandidate{}, err
	}
	current, err := root.Stat(path)
	if err != nil {
		return LocalCandidate{}, err
	}
	if len(data) > localSourceLimit || int64(len(data)) != before.Size() || !sameLocalFile(before, after) || !sameLocalFile(after, current) {
		return LocalCandidate{}, errSourceConflict
	}
	if err := file.Close(); err != nil {
		return LocalCandidate{}, err
	}
	if err := ctx.Err(); err != nil {
		return LocalCandidate{}, err
	}
	return admitLocal(data, ref.File)
}
func sameLocalFile(a, b os.FileInfo) bool {
	left, lok := a.Sys().(*syscall.Stat_t)
	right, rok := b.Sys().(*syscall.Stat_t)
	return lok && rok && left.Dev == right.Dev && left.Ino == right.Ino && left.Size == right.Size && left.Mtim == right.Mtim && left.Ctim == right.Ctim
}
func admitLocal(data []byte, basename string) (LocalCandidate, error) {
	// Construct a JSON-only document: local storage never admits encoded exports.
	raw, err := profileraw.Parse(profiledecode.Document{JSON: data, Kind: profiledecode.RootSingle})
	if err != nil {
		return LocalCandidate{}, &Error{Code: ERR_PROFILE_LOCAL_UNSUPPORTED, Cause: err}
	}
	bundle, err := profileadapter.NormalizeLocal(raw, basename)
	if err != nil {
		return LocalCandidate{}, &Error{Code: ERR_PROFILE_LOCAL_UNSUPPORTED, Cause: err}
	}
	return LocalCandidate{original: data, bundle: bundle}, nil
}

func (s *AzeronLocalSource) Discover(ctx context.Context) ([]SourceDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_READ, Cause: err}
	}
	defer root.Close()
	devices, err := root.Open("Storage/DevicesStorage")
	if err != nil {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_READ, Cause: err}
	}
	defer devices.Close()
	entries, err := devices.ReadDir(-1)
	if err != nil {
		return nil, &Error{Code: ERR_PROFILE_LOCAL_READ, Cause: err}
	}
	var descriptors []SourceDescriptor
	for _, device := range entries {
		if !device.IsDir() || !validLocalComponent(device.Name()) {
			continue
		}
		parent, err := root.Open(filepath.Join("Storage", "DevicesStorage", device.Name(), "ProfileStorage"))
		if err != nil {
			continue
		}
		files, readErr := parent.ReadDir(-1)
		closeErr := parent.Close()
		if readErr != nil || closeErr != nil {
			continue
		}
		for _, file := range files {
			if !file.Type().IsRegular() || !validLocalComponent(file.Name()) || !strings.HasPrefix(file.Name(), "profile_") || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			ref := SourceRef{Local: LocalRef{Root: s.root, Device: device.Name(), File: file.Name()}}
			candidate, err := s.LoadCandidate(ctx, ref)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				continue
			}
			descriptors = append(descriptors, SourceDescriptor{Ref: ref, Source: candidate.bundle.Source, Origin: Origin{Kind: "local"}, ProfileCount: 1})
		}
	}
	return descriptors, nil
}

// Watch implementation follows in the local watch task; never claim watch support.
func (s *AzeronLocalSource) Watch(ctx context.Context, ref SourceRef) (<-chan SourceChange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, &Error{Code: ERR_PROFILE_LOCAL_WATCH}
}
