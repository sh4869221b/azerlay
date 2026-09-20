package profilesource

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func TestImportedStickRoundTrip(t *testing.T) {
	for _, mode := range []string{"keyboard", "xbox"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			input := storeBytes(t, "../profileadapter/testdata/stick-"+mode+".json")
			prepared := storePrepared(t, string(input))
			want := prepared.Bundle()
			binding := want.Profiles[0].Controls[0].Bindings[0]
			if binding.Kind != profile.BindingStick || binding.Stick == nil || string(binding.Stick.Mode) != mode {
				t.Fatalf("prepared stick = %+v", binding)
			}
			source := NewImportedSource(home)
			selection, err := source.Import(context.Background(), prepared, 1, Origin{Kind: "text"})
			if err != nil {
				t.Fatal(err)
			}
			index := storeIndex(t, home)
			cache := storeBytes(t, filepath.Join(home, "azerlay/profiles", selection.Source.Hash, "bundle.json"))
			cached, err := decodeBundleCache(bytes.NewReader(cache), index.Sources[0])
			if err != nil || !reflect.DeepEqual(cached, want) {
				t.Fatalf("cached stick changed: %v", err)
			}
			cache = storeBytes(t, filepath.Join(home, "azerlay/profiles", selection.Source.Hash, "p1.json"))
			cachedProfile, err := decodeProfileCache(bytes.NewReader(cache), index.Sources[0], 1)
			if err != nil || !reflect.DeepEqual(cachedProfile, want.Profiles[0]) {
				t.Fatalf("cached profile stick changed: %v", err)
			}
			before := storeTree(t, home)
			loaded, err := NewImportedSource(home).Load(context.Background(), selection.Source)
			if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, want) {
				t.Fatalf("loaded stick changed: %v", err)
			}
			selected, err := source.Selected(context.Background())
			if err != nil || selected != selection || !reflect.DeepEqual(before, storeTree(t, home)) {
				t.Fatalf("load changed selection or disk: %v", err)
			}
			original := storeBytes(t, filepath.Join(home, "azerlay/sources", selection.Source.Hash+".azeron"))
			if !bytes.Equal(original, input) {
				t.Fatal("original source changed")
			}
		})
	}
}

func TestImportedStickRecovery(t *testing.T) {
	for _, mode := range []string{"keyboard", "xbox"} {
		for _, damage := range []string{"revision 1", "malformed stick", "corrupt original"} {
			t.Run(mode+"/"+damage, func(t *testing.T) {
				home := t.TempDir()
				input := storeBytes(t, "../profileadapter/testdata/stick-"+mode+".json")
				prepared := storePrepared(t, string(input))
				want := prepared.Bundle()
				source := NewImportedSource(home)
				selection, err := source.Import(context.Background(), prepared, 1, Origin{Kind: "text"})
				if err != nil {
					t.Fatal(err)
				}
				index := storeIndex(t, home)
				index.Sources[0].NormalizerVersion = "1"
				data, err := encodeIndex(index)
				if err != nil {
					t.Fatal(err)
				}
				writeLoadFile(t, filepath.Join(home, "azerlay/cache/source-index.json"), data)
				if damage == "revision 1" {
					old := storePrepared(t, string(input)).Bundle()
					old.Profiles[0].Controls[0].Bindings = []profile.TriggerBinding{{Trigger: profile.TriggerUnknown, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding"}}}
					artifacts, err := encodeArtifacts(index.Sources[0], old)
					if err != nil {
						t.Fatal(err)
					}
					for _, artifact := range artifacts {
						data := mutateStorage(t, artifact.data, storageMutation{"normalizer_version", `"1"`})
						writeLoadFile(t, filepath.Join(home, "azerlay", artifact.path), data)
					}
				} else {
					path := filepath.Join(home, "azerlay/profiles", selection.Source.Hash, "bundle.json")
					data := mutateStorage(t, storeBytes(t, path), storageMutation{"bundle/profiles/0/controls/0/bindings/0/stick/mode", `"directinput"`})
					writeLoadFile(t, path, data)
					if damage == "corrupt original" {
						writeLoadFile(t, filepath.Join(home, "azerlay/sources", selection.Source.Hash+".azeron"), []byte("broken"))
					}
				}
				before := storeTree(t, home)
				got, err := source.Load(context.Background(), selection.Source)
				if damage == "corrupt original" {
					requireStorageError(t, err)
					if got != nil {
						t.Fatal("corrupt original returned partial model")
					}
				} else if err != nil || got == nil || !reflect.DeepEqual(*got, want) {
					t.Fatalf("stick recovery = %v", err)
				}
				selected, err := source.Selected(context.Background())
				if err != nil || selected != selection || !reflect.DeepEqual(before, storeTree(t, home)) {
					t.Fatalf("recovery changed selection or disk: %v", err)
				}
			})
		}
	}
}

func TestStorageCodecStickInvalid(t *testing.T) {
	input := storeBytes(t, "../profileadapter/testdata/stick-keyboard.json")
	bundle := storePrepared(t, string(input)).Bundle()
	source := codecRecord(bundle)
	data, err := encodeBundleCache(source, bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []storageMutation{
		{"stick", ""},
		{"stick", `null`},
		{"stick", `{"Mode":"keyboard","keyboard_directions":{"up":"KEY_W","right":"KEY_D","down":"KEY_S","left":"KEY_A"}}`},
		{"stick/mode", `"xbox"`},
		{"stick/keyboard_directions", `null`},
		{"stick/keyboard_directions/up", ""},
		{"stick/keyboard_directions/up", `"KEY_U"`},
		{"trigger", `"long"`},
		{"actions", `[]`},
		{"unknown", `{"reason":"unmapped_binding"}`},
		{"kind", `"unknown"`},
	} {
		t.Run(change.path+"/"+change.value, func(t *testing.T) {
			change.path = "bundle/profiles/0/controls/0/bindings/0/" + change.path
			got, err := decodeBundleCache(bytes.NewReader(mutateStorage(t, data, change)), source)
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, profile.ProfileBundle{}) {
				t.Fatalf("invalid stick returned model: %v", err)
			}
		})
	}
	for _, binding := range codecFixture(t, "bundle").Profiles[0].Controls[0].Bindings {
		binding.Stick = bundle.Profiles[0].Controls[0].Bindings[0].Stick
		if validTrigger(binding) {
			t.Fatalf("%s accepted stick payload", binding.Kind)
		}
	}
	xbox := storePrepared(t, string(storeBytes(t, "../profileadapter/testdata/stick-xbox.json"))).Bundle()
	data, err = encodeBundleCache(codecRecord(xbox), xbox)
	if err != nil {
		t.Fatal(err)
	}
	data = mutateStorage(t, data, storageMutation{"bundle/profiles/0/controls/0/bindings/0/stick/keyboard_directions/up", `null`})
	if _, err := decodeBundleCache(bytes.NewReader(data), codecRecord(xbox)); err == nil {
		t.Fatal("Xbox accepted null direction instead of an empty code")
	}
}
