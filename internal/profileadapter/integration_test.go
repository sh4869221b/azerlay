package profileadapter_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

type normalizedGolden struct {
	SchemaVersion int                    `json:"schema_version"`
	Source        profile.SourceMetadata `json:"source"`
	RootKind      profile.RootKind       `json:"root_kind"`
	Profiles      []goldenProfile        `json:"profiles"`
	Raw           *goldenBundleRaw       `json:"raw"`
}

type goldenBundleRaw struct {
	Version *string           `json:"version"`
	Unknown map[string]string `json:"unknown"`
}

type goldenProfile struct {
	ID       *string          `json:"id"`
	Name     *string          `json:"name"`
	Controls []goldenControl  `json:"controls"`
	Raw      goldenProfileRaw `json:"raw"`
}

type goldenProfileRaw struct {
	ID      *string           `json:"id"`
	Name    *string           `json:"name"`
	Version *string           `json:"version"`
	Unknown map[string]string `json:"unknown"`
}

type goldenControl struct {
	Label    *string                `json:"label"`
	Bindings []goldenTriggerBinding `json:"bindings"`
	Raw      goldenBindingRaw       `json:"raw"`
}

type goldenBindingRaw struct {
	RootKind     profile.RootKind  `json:"root_kind"`
	ProfileIndex int               `json:"profile_index"`
	InputIndex   int               `json:"input_index"`
	Fields       map[string]string `json:"fields"`
}

type goldenTriggerBinding struct {
	Trigger           profile.TriggerKind     `json:"trigger"`
	Kind              profile.BindingKind     `json:"kind"`
	Actions           []goldenAction          `json:"actions"`
	Turbo             *profile.TurboBinding   `json:"turbo"`
	Macro             *profile.MacroBinding   `json:"macro"`
	Stick             *profile.StickBinding   `json:"stick"`
	TriggerDelayMS    *int                    `json:"trigger_delay_ms"`
	TriggerIntervalMS *int                    `json:"trigger_interval_ms"`
	ReleaseBehavior   *string                 `json:"release_behavior"`
	Unknown           *profile.UnknownBinding `json:"unknown"`
}

type goldenAction struct {
	Kind      profile.ActionKind      `json:"kind"`
	Code      profile.CanonicalCode   `json:"code"`
	Modifiers []profile.CanonicalCode `json:"modifiers"`
}

func TestNormalizeGolden(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"bundle", "single", "v1-settings"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input, err := os.ReadFile("testdata/" + name + ".input.json")
			if err != nil {
				t.Fatalf("ReadFile(input) error = %v", err)
			}
			expectedJSON, err := os.ReadFile("testdata/" + name + ".expected.json")
			if err != nil {
				t.Fatalf("ReadFile(expected) error = %v", err)
			}
			var expected normalizedGolden
			if err := json.Unmarshal(expectedJSON, &expected); err != nil {
				t.Fatalf("Unmarshal(expected) error = %v", err)
			}

			document, err := profiledecode.DecodeText(string(input))
			if err != nil {
				t.Fatalf("DecodeText() error = %v", err)
			}
			raw, err := profileraw.Parse(document)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			actual, err := profileadapter.Normalize(raw, admittedSource)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}

			projection := projectNormalized(actual)
			if !reflect.DeepEqual(projection, expected) {
				actualJSON, marshalErr := json.MarshalIndent(projection, "", "  ")
				if marshalErr != nil {
					t.Fatalf("Marshal(actual) error = %v", marshalErr)
				}
				t.Fatalf("normalized projection =\n%s\nwant literal testdata/%s.expected.json", actualJSON, name)
			}
		})
	}
	t.Run("v1-settings-bundle", func(t *testing.T) {
		t.Parallel()
		input, err := os.ReadFile("testdata/v1-settings.input.json")
		if err != nil {
			t.Fatal(err)
		}
		literal, err := os.ReadFile("testdata/v1-settings.expected.json")
		if err != nil {
			t.Fatal(err)
		}
		var expected normalizedGolden
		if err := json.Unmarshal(literal, &expected); err != nil {
			t.Fatal(err)
		}
		expected.RootKind = profile.RootBundle
		expected.Raw = &goldenBundleRaw{}
		for index := range expected.Profiles[0].Controls {
			expected.Profiles[0].Controls[index].Raw.RootKind = profile.RootBundle
		}
		document, err := profiledecode.DecodeText(`{"profiles":[` + string(input) + `]}`)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := profileraw.Parse(document)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := profileadapter.Normalize(raw, admittedSource)
		if err != nil {
			t.Fatal(err)
		}
		if projection := projectNormalized(actual); !reflect.DeepEqual(projection, expected) {
			t.Fatalf("bundle projection = %#v, want literal bundle variant %#v", projection, expected)
		}
	})
}

func projectNormalized(bundle profile.ProfileBundle) normalizedGolden {
	profiles := make([]goldenProfile, len(bundle.Profiles))
	for profileIndex, normalizedProfile := range bundle.Profiles {
		controls := make([]goldenControl, len(normalizedProfile.Controls))
		for controlIndex, control := range normalizedProfile.Controls {
			bindings := make([]goldenTriggerBinding, len(control.Bindings))
			for bindingIndex, binding := range control.Bindings {
				actions := make([]goldenAction, len(binding.Actions))
				for actionIndex, action := range binding.Actions {
					actions[actionIndex] = goldenAction{Kind: action.Kind, Code: action.Code, Modifiers: action.Modifiers}
				}
				if binding.Actions == nil {
					actions = nil
				}
				bindings[bindingIndex] = goldenTriggerBinding{
					Trigger: binding.Trigger, Kind: binding.Kind, Actions: actions,
					Turbo: binding.Turbo, Macro: binding.Macro, Stick: binding.Stick,
					TriggerDelayMS: binding.TriggerDelayMS, TriggerIntervalMS: binding.TriggerIntervalMS,
					ReleaseBehavior: binding.ReleaseBehavior, Unknown: binding.Unknown,
				}
			}
			controls[controlIndex] = goldenControl{
				Label: control.Label, Bindings: bindings,
				Raw: goldenBindingRaw{RootKind: control.Raw.RootKind, ProfileIndex: control.Raw.ProfileIndex, InputIndex: control.Raw.InputIndex, Fields: goldenRawFields(control.Raw.Fields)},
			}
		}
		profiles[profileIndex] = goldenProfile{
			ID: normalizedProfile.ID, Name: normalizedProfile.Name, Controls: controls,
			Raw: goldenProfileRaw{ID: rawToken(normalizedProfile.Raw.ID), Name: rawToken(normalizedProfile.Raw.Name), Version: rawToken(normalizedProfile.Raw.Version), Unknown: goldenRawFields(normalizedProfile.Raw.Unknown)},
		}
	}
	golden := normalizedGolden{SchemaVersion: bundle.SchemaVersion, Source: bundle.Source, RootKind: bundle.RootKind, Profiles: profiles}
	if bundle.Raw != nil {
		golden.Raw = &goldenBundleRaw{Version: rawToken(bundle.Raw.Version), Unknown: goldenRawFields(bundle.Raw.Unknown)}
	}
	return golden
}

func rawToken(raw json.RawMessage) *string {
	if raw == nil {
		return nil
	}
	value := string(raw)
	return &value
}

func goldenRawFields(fields map[string]json.RawMessage) map[string]string {
	if fields == nil {
		return nil
	}
	result := make(map[string]string, len(fields))
	for key, value := range fields {
		result[key] = string(value)
	}
	return result
}
