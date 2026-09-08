package profilesource

import (
	"bytes"
	"io"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

// Prepared owns exact original input bytes and the complete normalized export.
// Construct it with PrepareText or PrepareReader; the zero value is not a candidate.
// Copies share owned storage and must be treated as read-only.
type Prepared struct {
	original  []byte
	bundle    profile.ProfileBundle
	inputKind string
}

// Bundle returns a read-only view for trusted projection and selection.
// Callers must not mutate its slices, maps, pointers or raw token bytes.
func (p Prepared) Bundle() profile.ProfileBundle { return p.bundle }

// PrepareText captures exact text after complete Decode -> Parse -> Normalize success.
// It retains DecodeText's text-only semantics and returns zero on any failure.
func PrepareText(text string, source profile.SourceMetadata) (Prepared, error) {
	document, err := profiledecode.DecodeText(text)
	if err != nil {
		return Prepared{}, err
	}
	bundle, err := normalizeDocument(document, source)
	if err != nil {
		return Prepared{}, err
	}
	return Prepared{original: []byte(text), bundle: bundle, inputKind: "text"}, nil
}

// PrepareReader captures exact bytes within DecodeReader's bounded read, including
// binary input. It returns zero on any failure and does not close the caller's reader.
func PrepareReader(reader io.Reader, source profile.SourceMetadata) (Prepared, error) {
	var original bytes.Buffer
	input := reader
	// Preserve DecodeReader's typed nil-reader error instead of wrapping nil in a tee.
	if reader != nil {
		input = io.TeeReader(reader, &original)
	}
	document, err := profiledecode.DecodeReader(input)
	if err != nil {
		return Prepared{}, err
	}
	bundle, err := normalizeDocument(document, source)
	if err != nil {
		return Prepared{}, err
	}
	return Prepared{original: original.Bytes(), bundle: bundle, inputKind: "reader"}, nil
}

func normalizeDocument(document profiledecode.Document, source profile.SourceMetadata) (profile.ProfileBundle, error) {
	raw, err := profileraw.Parse(document)
	if err != nil {
		return profile.ProfileBundle{}, err
	}
	return profileadapter.Normalize(raw, source)
}
