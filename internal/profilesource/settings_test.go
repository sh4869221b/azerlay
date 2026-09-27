package profilesource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

const settingsInactive = `"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"turboIntervalLong":0,"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"],"isHoldDouble":false,"isTurboDouble":false,"turboIntervalDouble":0`
const settingsTurbo20 = `{"types":["1","11","11"],"keyValues":["KeyT","0","0","0"],"metaValues":["0","0","0"],"isHold":false,"isTurbo":true,"turboInterval":20,` + settingsInactive + `}`
const settingsMacroFalse = `{"types":["16","11","11"],"keyValues":["0","0","0","0"],"metaValues":["0","0","0"],"isHold":false,"isTurbo":false,"turboInterval":0,` + settingsInactive + `,"macro":{"v":1,"repeat":false,"steps":[{"type":"Button","direction":"Full","duration":50,"keyCode":87},{"type":"Delay","direction":"Full","duration":100}]}}`

func settingsExport(t *testing.T) string {
	t.Helper()
	var observed struct {
		Inputs []json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(storeBytes(t, "../profileadapter/testdata/stick-xbox-observed.json"), &observed); err != nil || len(observed.Inputs) != 1 {
		t.Fatalf("observed stick fixture: %v", err)
	}
	zero := string(observed.Inputs[0])
	ninety := strings.Replace(zero, `"angle": 0`, `"angle": 90`, 1)
	if ninety == zero {
		t.Fatal("observed stick angle token missing")
	}
	inputs := []string{
		settingsTurbo20,
		strings.Replace(settingsTurbo20, `"turboInterval":20`, `"turboInterval":50`, 1),
		settingsMacroFalse,
		strings.Replace(settingsMacroFalse, `"repeat":false`, `"repeat":true`, 1),
		zero,
		ninety,
	}
	return `{"id":"settings","inputs":[` + strings.Join(inputs, ",") + `]}`
}

func settingsPrepared(t *testing.T) Prepared {
	t.Helper()
	prepared := storePrepared(t, settingsExport(t))
	want := []profile.TriggerBinding{
		{Trigger: profile.TriggerSingle, Kind: profile.BindingTurbo, Turbo: &profile.TurboBinding{Code: profile.KEY_T, ClicksPerSecond: 25}},
		{Trigger: profile.TriggerSingle, Kind: profile.BindingTurbo, Turbo: &profile.TurboBinding{Code: profile.KEY_T, ClicksPerSecond: 10}},
		{Trigger: profile.TriggerSingle, Kind: profile.BindingMacro, Macro: &profile.MacroBinding{Steps: []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_W, DurationMS: 50}, {Kind: profile.MacroStepDelay, DurationMS: 100}}}},
		{Trigger: profile.TriggerSingle, Kind: profile.BindingMacro, Macro: &profile.MacroBinding{RepeatWhileHeld: true, Steps: []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_W, DurationMS: 50}, {Kind: profile.MacroStepDelay, DurationMS: 100}}}},
		{Trigger: profile.TriggerSingle, Kind: profile.BindingStick, Stick: &profile.StickBinding{Mode: profile.StickModeXbox}},
		{Trigger: profile.TriggerSingle, Kind: profile.BindingStick, Stick: &profile.StickBinding{Mode: profile.StickModeXbox, AngleDegrees: 90}},
	}
	controls := prepared.Bundle().Profiles[0].Controls
	if len(controls) != len(want) {
		t.Fatalf("controls = %d, want %d", len(controls), len(want))
	}
	for i, control := range controls {
		if !reflect.DeepEqual(control.Bindings[0], want[i]) {
			t.Fatalf("control %d = %#v, want %#v", i, control.Bindings[0], want[i])
		}
	}
	return prepared
}

func TestImportedSettingsRoundTrip(t *testing.T) {
	home := t.TempDir()
	prepared := settingsPrepared(t)
	want := prepared.Bundle()
	source := NewImportedSource(home)
	selection, err := source.Import(context.Background(), prepared, 1, Origin{Kind: "text"})
	if err != nil {
		t.Fatal(err)
	}
	index := storeIndex(t, home)
	if index.Sources[0].NormalizerVersion != "5" {
		t.Fatalf("index normalizer revision = %q", index.Sources[0].NormalizerVersion)
	}
	cacheDir := filepath.Join(home, "azerlay/profiles", selection.Source.Hash)
	bundleCache := storeBytes(t, filepath.Join(cacheDir, "bundle.json"))
	memberCache := storeBytes(t, filepath.Join(cacheDir, "p1.json"))
	if cached, err := decodeCacheSet(index.Sources[0], bytes.NewReader(bundleCache), []io.Reader{bytes.NewReader(memberCache)}); err != nil || !reflect.DeepEqual(cached, want) {
		t.Fatalf("cache set changed settings: %v", err)
	}
	before := storeTree(t, home)
	loaded, err := NewImportedSource(home).Load(context.Background(), selection.Source)
	if err != nil || loaded == nil || !reflect.DeepEqual(*loaded, want) {
		t.Fatalf("fresh Load changed settings: %v", err)
	}
	selected, err := source.Selected(context.Background())
	if err != nil || selected != selection || !reflect.DeepEqual(before, storeTree(t, home)) {
		t.Fatalf("Load changed selection or disk: %v", err)
	}
}

func TestImportedSettingsRecovery(t *testing.T) {
	for _, damage := range []string{"revision 1", "revision 2", "malformed cache", "missing cache", "corrupt original"} {
		t.Run(damage, func(t *testing.T) {
			home := t.TempDir()
			prepared := settingsPrepared(t)
			want := prepared.Bundle()
			source := NewImportedSource(home)
			selection, err := source.Import(context.Background(), prepared, 1, Origin{Kind: "text"})
			if err != nil {
				t.Fatal(err)
			}
			cacheDir := filepath.Join(home, "azerlay/profiles", selection.Source.Hash)
			bundlePath := filepath.Join(cacheDir, "bundle.json")
			switch damage {
			case "revision 1", "revision 2":
				revision := strings.TrimPrefix(damage, "revision ")
				index := storeIndex(t, home)
				index.Sources[0].NormalizerVersion = revision
				data, err := encodeIndex(index)
				if err != nil {
					t.Fatal(err)
				}
				writeLoadFile(t, filepath.Join(home, "azerlay/cache/source-index.json"), data)
				for _, path := range []string{bundlePath, filepath.Join(cacheDir, "p1.json")} {
					data := mutateStorage(t, storeBytes(t, path), storageMutation{"normalizer_version", `"` + revision + `"`})
					writeLoadFile(t, path, data)
				}
			case "malformed cache":
				writeLoadFile(t, bundlePath, []byte("{"))
			case "missing cache", "corrupt original":
				if err := os.Remove(bundlePath); err != nil {
					t.Fatal(err)
				}
				if damage == "corrupt original" {
					writeLoadFile(t, filepath.Join(home, "azerlay/sources", selection.Source.Hash+".azeron"), []byte("broken"))
				}
			}
			before := storeTree(t, home)
			got, err := source.Load(context.Background(), selection.Source)
			if damage == "corrupt original" {
				requireStorageError(t, err)
				if got != nil {
					t.Fatal("corrupt original returned a model")
				}
			} else if err != nil || got == nil || !reflect.DeepEqual(*got, want) {
				t.Fatalf("settings recovery = %v", err)
			}
			selected, err := source.Selected(context.Background())
			if err != nil || selected != selection || !reflect.DeepEqual(before, storeTree(t, home)) {
				t.Fatalf("recovery changed selection or disk: %v", err)
			}
		})
	}
}

func TestStorageCodecSettingsInvalid(t *testing.T) {
	bundle := settingsPrepared(t).Bundle()
	source := codecRecord(bundle)
	data, err := encodeBundleCache(source, bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []storageMutation{
		{"0/turbo/code", `"KEY_U"`},
		{"0/turbo/clicks_per_second", `24`},
		{"0/turbo/clicks_per_second", `null`},
		{"0/turbo/clicks_per_second", ""},
		{"2/macro/repeat_while_held", `null`},
		{"2/macro/repeat_while_held", ""},
		{"2/macro/steps/0/code", `null`},
		{"2/macro/steps/1/code", ""},
		{"2/macro/steps/1/code", `"KEY_W"`},
		{"2/macro/steps/1/duration_ms", `101`},
		{"5/stick/angle_degrees", `45`},
		{"5/actions", `[]`},
	} {
		t.Run(change.path+"/"+change.value, func(t *testing.T) {
			change.path = "bundle/profiles/0/controls/" + strings.Replace(change.path, "/", "/bindings/0/", 1)
			got, err := decodeBundleCache(bytes.NewReader(mutateStorage(t, data, change)), source)
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, profile.ProfileBundle{}) {
				t.Fatalf("invalid settings cache accepted: %#v, %v", got, err)
			}
		})
	}
	for _, test := range []struct {
		name, field, value string
		control            int
	}{
		{"macro on turbo", "macro", `{"repeat_while_held":false,"steps":[]}`, 0},
		{"turbo on macro", "turbo", `{"code":"KEY_T","clicks_per_second":25}`, 2},
		{"null neutral angle", "angle_degrees", `null`, 4},
		{"null extra turbo", "turbo", `null`, 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			var envelope map[string]any
			if err := json.Unmarshal(data, &envelope); err != nil {
				t.Fatal(err)
			}
			trigger := envelope["bundle"].(map[string]any)["profiles"].([]any)[0].(map[string]any)["controls"].([]any)[test.control].(map[string]any)["bindings"].([]any)[0].(map[string]any)
			var value any
			if err := json.Unmarshal([]byte(test.value), &value); err != nil {
				t.Fatal(err)
			}
			if test.field == "angle_degrees" {
				trigger["stick"].(map[string]any)[test.field] = value
			} else {
				trigger[test.field] = value
			}
			changed, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decodeBundleCache(bytes.NewReader(changed), source)
			var failure *codecError
			if !errors.As(err, &failure) || !reflect.DeepEqual(got, profile.ProfileBundle{}) {
				t.Fatalf("extra settings payload accepted: %#v, %v", got, err)
			}
		})
	}
	for _, index := range []int{0, 2, 4} {
		binding := bundle.Profiles[0].Controls[index].Bindings[0]
		binding.Actions = []profile.Action{}
		if validTrigger(binding) {
			t.Fatalf("kind %q accepted Actions", binding.Kind)
		}
	}
}
