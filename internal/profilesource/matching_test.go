package profilesource

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func matchingPrepared(t *testing.T) Prepared {
	t.Helper()
	return storePrepared(t, string(storeBytes(t, "../profileadapter/testdata/unbound.input.json")))
}

func TestMatchingModelRoundTrip(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	prepared := matchingPrepared(t)
	want := prepared.Bundle()
	identity := want.Profiles[0].Controls[0].SourceIdentity
	if identity.InputID == nil || *identity.InputID != 4 || identity.PinOne == nil || *identity.PinOne != 5 || identity.PinTwo == nil || *identity.PinTwo != 255 || identity.Invalid || want.Profiles[0].Controls[0].Bindings[0].Kind != profile.BindingUnbound {
		t.Fatal("fixture did not normalize identity and unbound")
	}
	source := NewImportedSource(home)
	selection, err := source.Import(context.Background(), prepared, 1, Origin{Kind: "text"})
	if err != nil {
		t.Fatal(err)
	}
	index := storeIndex(t, home)
	if index.Sources[0].NormalizerVersion != "5" {
		t.Fatalf("normalizer revision = %q", index.Sources[0].NormalizerVersion)
	}
	cacheDir := filepath.Join(home, "azerlay/profiles", selection.Source.Hash)
	cached, err := decodeCacheSet(index.Sources[0], bytes.NewReader(storeBytes(t, filepath.Join(cacheDir, "bundle.json"))), []io.Reader{bytes.NewReader(storeBytes(t, filepath.Join(cacheDir, "p1.json")))})
	if err != nil || !reflect.DeepEqual(cached, want) {
		t.Fatalf("cache round trip = %v", err)
	}
	before := storeTree(t, home)
	loaded, err := NewImportedSource(home).Load(context.Background(), selection.Source)
	if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, want) {
		t.Fatalf("fresh Load = %v", err)
	}
	*loaded.Profiles[0].Controls[0].SourceIdentity.InputID = 99
	*loaded.Profiles[0].Controls[0].SourceIdentity.PinOne = 99
	*loaded.Profiles[0].Controls[0].SourceIdentity.PinTwo = 99
	again, err := source.Load(context.Background(), selection.Source)
	if err != nil || again == nil || !reflect.DeepEqual(*again, want) {
		t.Fatalf("identity pointer mutation escaped Load = %v", err)
	}
	selected, err := source.Selected(context.Background())
	if err != nil || selected != selection || !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatalf("Load changed selection or disk: %v", err)
	}
}

func TestMatchingModelInvalidIdentityRoundTrip(t *testing.T) {
	t.Parallel()
	input := storeBytes(t, "../profileadapter/testdata/unbound.input.json")
	changed := bytes.Replace(input, []byte(`"id": 4`), []byte(`"id": null`), 1)
	if bytes.Equal(changed, input) {
		t.Fatal("identity fixture was not changed")
	}
	bundle := storePrepared(t, string(changed)).Bundle()
	identity := bundle.Profiles[0].Controls[0].SourceIdentity
	if identity.InputID != nil || identity.PinOne == nil || *identity.PinOne != 5 || identity.PinTwo == nil || *identity.PinTwo != 255 || !identity.Invalid {
		t.Fatalf("malformed source identity = %#v", identity)
	}
	source := codecRecord(bundle)
	encoded, err := encodeBundleCache(source, bundle)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeBundleCache(bytes.NewReader(encoded), source)
	if err != nil || !reflect.DeepEqual(decoded, bundle) {
		t.Fatalf("invalid identity round trip = %v", err)
	}
}

func TestMatchingModelHistoricalCache(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	prepared := matchingPrepared(t)
	want := prepared.Bundle()
	source := NewImportedSource(home)
	selection, err := source.Import(context.Background(), prepared, 1, Origin{Kind: "text"})
	if err != nil {
		t.Fatal(err)
	}
	index := storeIndex(t, home)
	index.Sources[0].NormalizerVersion = "3"
	encoded, err := encodeIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	writeLoadFile(t, filepath.Join(home, "azerlay/cache/source-index.json"), encoded)
	cacheDir := filepath.Join(home, "azerlay/profiles", selection.Source.Hash)
	for _, name := range []string{"bundle.json", "p1.json"} {
		path := filepath.Join(cacheDir, name)
		cached := mutateStorage(t, storeBytes(t, path), storageMutation{"normalizer_version", `"3"`})
		identityPath := "bundle/profiles/0/controls/0/source_identity"
		if name == "p1.json" {
			identityPath = "profile/controls/0/source_identity"
		}
		writeLoadFile(t, path, mutateStorage(t, cached, storageMutation{identityPath, ""}))
	}
	before := storeTree(t, home)
	loaded, err := NewImportedSource(home).Load(context.Background(), selection.Source)
	if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, want) {
		t.Fatalf("historical Load = %v", err)
	}
	selected, err := source.Selected(context.Background())
	if err != nil || selected != selection || !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatalf("historical Load changed selection or disk: %v", err)
	}
	originalPath := filepath.Join(home, "azerlay/sources", selection.Source.Hash+".azeron")
	writeLoadFile(t, originalPath, []byte("broken"))
	if got, err := source.Load(context.Background(), selection.Source); got != nil {
		t.Fatal("corrupt original returned a model")
	} else {
		requireStorageError(t, err)
	}
}

func TestMatchingModelInvalidCache(t *testing.T) {
	t.Parallel()
	bundle := codecFixture(t, "unbound")
	source := codecRecord(bundle)
	encoded, err := encodeBundleCache(source, bundle)
	if err != nil {
		t.Fatal(err)
	}
	path := "bundle/profiles/0/controls/0/"
	for _, change := range []storageMutation{
		{path + "bindings/0/actions", `[{"kind":"keyboard","code":"KEY_U","modifiers":null}]`},
		{path + "bindings/0/trigger", `"long"`},
		{path + "bindings/0/unknown", `{"reason":"unmapped_binding"}`},
		{path + "source_identity/input_id", `0`},
		{path + "source_identity/pin_two", `-1`},
		{path + "source_identity/invalid", `null`},
		{path + "source_identity", `null`},
		{path + "source_identity/input_id", `"4"`},
		{path + "source_identity/input_id", `1.5`},
	} {
		t.Run(change.path+"="+change.value, func(t *testing.T) {
			t.Parallel()
			got, err := decodeBundleCache(bytes.NewReader(mutateStorage(t, encoded, change)), source)
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, profile.ProfileBundle{}) {
				t.Fatalf("invalid cache accepted: %#v, %v", got, err)
			}
		})
	}

	home := t.TempDir()
	prepared := matchingPrepared(t)
	selection, err := NewImportedSource(home).Import(context.Background(), prepared, 1, Origin{Kind: "text"})
	if err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(home, "azerlay/profiles", selection.Source.Hash, "bundle.json")
	writeLoadFile(t, cachePath, mutateStorage(t, storeBytes(t, cachePath), storageMutation{path + "bindings/0/actions", `[{"kind":"keyboard","code":"KEY_U","modifiers":null}]`}))
	before := storeTree(t, home)
	loaded, err := NewImportedSource(home).Load(context.Background(), selection.Source)
	if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, prepared.Bundle()) || !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatalf("invalid cache recovery changed model or disk: %v", err)
	}
}
