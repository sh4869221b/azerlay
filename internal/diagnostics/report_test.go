package diagnostics

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
)

func TestReportGolden(t *testing.T) {
	t.Parallel()
	for _, include := range []bool{false, true} {
		for _, jsonMode := range []bool{false, true} {
			name := "default"
			if include {
				name = "details"
			}
			extension := ".txt"
			if jsonMode {
				extension = ".json"
			}
			t.Run(name+extension, func(t *testing.T) {
				var details *ProfileDetails
				if include {
					details = ProjectProfileDetails(2, selectedProfile())
				}
				target := "/synthetic/config\n\x1b.toml"
				report := NewReport([]Check{
					{Category: "configuration", Code: "OK_CONFIGURATION", Summary: "Configuration loaded.", Target: &target, Severity: SeverityOK},
					{Category: "session", Code: "ERR_SESSION_WAYLAND_REQUIRED", Summary: "Wayland environment is unavailable.", Remediation: "Run from a Wayland session.", Severity: SeverityError},
				}, details)
				var output bytes.Buffer
				if err := WriteReport(&output, report, jsonMode); err != nil {
					t.Fatal(err)
				}
				want, err := os.ReadFile(filepath.Join("testdata", name+extension))
				if err != nil {
					t.Fatal(err)
				}
				if output.String() != string(want) {
					t.Fatalf("output=%q\nwant=%q", output.String(), want)
				}
				if jsonMode {
					decoder := json.NewDecoder(&output)
					var got Report
					decoder.DisallowUnknownFields()
					if err := decoder.Decode(&got); err != nil {
						t.Fatal(err)
					}
					if err := decoder.Decode(&got); !errors.Is(err, io.EOF) {
						t.Fatalf("extra JSON document: %v", err)
					}
				}
			})
		}
	}
}

func TestProfileDetailsPrivacy(t *testing.T) {
	t.Parallel()
	bundle := profile.ProfileBundle{Profiles: []profile.Profile{
		{Controls: []profile.ControlBinding{{Label: stringPointer("UNSELECTED_LABEL")}}}, selectedProfile(),
	}}
	bundle.Profiles[1].Controls = append(bundle.Profiles[1].Controls, profile.ControlBinding{
		Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerSingle, Kind: profile.BindingMacro, Macro: &profile.MacroBinding{
			RepeatWhileHeld: true,
			Steps:           []profile.MacroStep{{Kind: profile.MacroStepButton, Code: profile.KEY_W, DurationMS: 50}, {Kind: profile.MacroStepDelay, DurationMS: 100}},
		}}},
	})
	for _, include := range []bool{false, true} {
		for _, jsonMode := range []bool{false, true} {
			var details *ProfileDetails
			if include {
				details = ProjectProfileDetails(2, bundle.Profiles[1])
			}
			var output bytes.Buffer
			if err := WriteReport(&output, NewReport(nil, details), jsonMode); err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"PRIVATE_ID", "PRIVATE_NAME", "PRIVATE_MACRO", "PRIVATE_FIELD", "UNSELECTED_LABEL"} {
				if strings.Contains(output.String(), secret) {
					t.Fatalf("include=%v json=%v disclosed %s", include, jsonMode, secret)
				}
			}
			for _, detail := range []string{"KEY_W", "repeat_while_held", "duration_ms"} {
				if strings.Contains(output.String(), detail) {
					t.Fatalf("include=%v json=%v disclosed Macro detail %s", include, jsonMode, detail)
				}
			}
			if strings.Contains(output.String(), "CHOSEN_LABEL") != include || strings.Contains(output.String(), "KEY_U") != include {
				t.Fatalf("include=%v json=%v output=%q", include, jsonMode, output.String())
			}
			if strings.ContainsRune(output.String(), '\x1b') || strings.Contains(output.String(), "CHOSEN_LABEL\n") {
				t.Fatalf("unescaped label: %q", output.String())
			}
		}
	}
}

func TestReportOutputFailure(t *testing.T) {
	t.Parallel()
	failure := errors.New("private output failure")
	for _, jsonMode := range []bool{false, true} {
		for _, writeErr := range []error{failure, nil} {
			writer := &partialWriter{err: writeErr}
			err := WriteReport(writer, NewReport([]Check{{Category: "session", Severity: SeverityOK}}, nil), jsonMode)
			want := writeErr
			if want == nil {
				want = io.ErrShortWrite
			}
			if !errors.Is(err, want) || writer.calls != 1 || writer.output.Len() != 1 {
				t.Fatalf("json=%v error=%v calls=%d bytes=%d", jsonMode, err, writer.calls, writer.output.Len())
			}
		}
	}
}

type partialWriter struct {
	err    error
	calls  int
	output bytes.Buffer
}

func (w *partialWriter) Write(data []byte) (int, error) {
	w.calls++
	w.output.Write(data[:1])
	return 1, w.err
}

func selectedProfile() profile.Profile {
	delay, interval := 120, 40
	return profile.Profile{
		ID: stringPointer("PRIVATE_ID"), Name: stringPointer("PRIVATE_NAME"),
		Raw: profile.RawProfileReference{Unknown: map[string]json.RawMessage{"private": json.RawMessage(`"PRIVATE_FIELD"`)}},
		Controls: []profile.ControlBinding{
			{Label: stringPointer("CHOSEN_LABEL\n\x1b"), Bindings: []profile.TriggerBinding{{
				Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard,
				Actions:        []profile.Action{{Kind: profile.ActionKeyboard, Code: profile.KEY_U, Modifiers: []profile.CanonicalCode{profile.KEY_LEFTCTRL}}},
				TriggerDelayMS: &delay, TriggerIntervalMS: &interval, ReleaseBehavior: stringPointer("regular"),
			}}},
			{Bindings: []profile.TriggerBinding{{Trigger: profile.TriggerLong, Kind: profile.BindingUnknown, Unknown: &profile.UnknownBinding{Reason: "unmapped_binding"}}},
				Raw: profile.RawBindingReference{Fields: map[string]json.RawMessage{"macro": json.RawMessage(`"PRIVATE_MACRO"`)}},
			},
		},
	}
}

func stringPointer(value string) *string { return &value }
