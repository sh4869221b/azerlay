package profilesource

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/sh4869221b/azerlay/internal/profile"
)

type SourceID string
type SourceRef struct{ Hash string }
type Selection struct {
	Source       SourceRef
	ProfileIndex int // One-based source ordinal, not an Azeron ID.
}
type Origin struct {
	Kind string
	Path *string
}
type SourceDescriptor struct {
	Ref          SourceRef
	Source       profile.SourceMetadata
	Origin       Origin
	ImportedAt   time.Time
	ProfileCount int
}
type SourceChange struct{ Ref SourceRef }
type ProfileSource interface {
	ID() SourceID
	Discover(context.Context) ([]SourceDescriptor, error)
	Load(context.Context, SourceRef) (*profile.ProfileBundle, error)
	Watch(context.Context, SourceRef) (<-chan SourceChange, error)
}

type ImportedSource struct {
	dataHome string
	now      func() time.Time
	// hook is a test-only barrier/fault seam around real publication operations.
	hook func(publicationEvent)
}

type publicationEvent struct {
	phase   string
	path    string
	file    *os.File
	staging string
}

// NewImportedSource is lazy; dataHome is the absolute XDG data parent.
func NewImportedSource(dataHome string) *ImportedSource {
	return &ImportedSource{dataHome: dataHome, now: time.Now}
}

// Import requires a successful, unmodified Prepared and an explicit ordinal.
// Its four parameters are the approved public API contract.
func (s *ImportedSource) Import(ctx context.Context, prepared Prepared, profileIndex int, origin Origin) (selection Selection, err error) {
	defer func() {
		if err != nil {
			selection = Selection{}
			err = &Error{Code: ERR_PROFILE_STORAGE, Cause: err}
		}
	}()
	if err := ctx.Err(); err != nil {
		return Selection{}, err
	}
	if len(prepared.original) == 0 || profileIndex < 1 || profileIndex > len(prepared.bundle.Profiles) {
		return Selection{}, errInvalidSelection
	}
	source := diskSource{
		SourceHash:      fmt.Sprintf("%x", sha256.Sum256(prepared.original)),
		SoftwareRelease: prepared.bundle.Source.SoftwareRelease, SourceScope: prepared.bundle.Source.SourceScope,
		InputKind: prepared.inputKind, Origin: diskOrigin(origin), ImportedAt: s.now().UTC().Format(time.RFC3339Nano),
		DecoderVersion: decoderVersion, NormalizerVersion: normalizerVersion, ModelSchemaVersion: modelSchemaVersion,
		ProfileCount: len(prepared.bundle.Profiles),
	}
	switch prepared.bundle.RootKind {
	case profile.RootBundle:
		source.ExportVersion = tokenToDisk(prepared.bundle.Raw.Version)
	case profile.RootSingle:
		source.ExportVersion = tokenToDisk(prepared.bundle.Profiles[0].Raw.Version)
	default:
		return Selection{}, errCodecShape
	}
	// Encode the complete bounded candidate before creating even a lock directory.
	artifacts, err := encodeArtifacts(source, prepared.bundle)
	if err != nil {
		return Selection{}, err
	}
	root, err := openStore(s.dataHome)
	if err != nil {
		return Selection{}, err
	}
	// Read-only directory and lock handles carry no buffered data. Their release
	// cannot roll back a published index; all data-file closes are checked below.
	defer root.Close()
	lock, err := openPrivateFile(root, "cache/import.lock", os.O_RDWR|os.O_CREATE)
	if err != nil {
		return Selection{}, err
	}
	defer lock.Close() // Closing the stable descriptor also releases flock.
	s.event(publicationEvent{phase: "lock", path: "cache/import.lock", file: lock})
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return Selection{}, err
	}
	if err := ctx.Err(); err != nil {
		return Selection{}, err
	}
	tx := importTransaction{root: root, source: source, artifacts: artifacts, owner: s}
	if err := tx.commit(ctx, prepared.original, profileIndex); err != nil {
		return Selection{}, err
	}
	return Selection{Source: SourceRef{Hash: source.SourceHash}, ProfileIndex: profileIndex}, nil
}

func (s *ImportedSource) event(event publicationEvent) {
	if s.hook != nil {
		s.hook(event)
	}
}
