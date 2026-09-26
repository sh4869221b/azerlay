package profileadapter_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
)

const turbo20 = `{"types":["1","11","11"],"keyValues":["KeyT","0","0","0"],"metaValues":["0","0","0"],"isHold":false,"isTurbo":true,"turboInterval":20,"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"turboIntervalLong":0,"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"],"isHoldDouble":false,"isTurboDouble":false,"turboIntervalDouble":0}`

func TestNormalizeTurbo(t *testing.T) {
	t.Parallel()

	t.Run("observed rates retain duplicate source inputs", func(t *testing.T) {
		t.Parallel()
		turbo50 := strings.Replace(turbo20, `"turboInterval":20`, `"turboInterval":50`, 1)
		actual, err := profileadapter.Normalize(parseExport(t, `{"id":"synthetic","inputs":[`+turbo20+`,`+turbo50+`,`+turbo20+`]}`), admittedSource)
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		for index, rate := range []int{25, 10, 25} {
			control := actual.Profiles[0].Controls[index]
			want := profile.TriggerBinding{
				Trigger: profile.TriggerSingle,
				Kind:    profile.BindingTurbo,
				Turbo:   &profile.TurboBinding{Code: profile.KEY_T, ClicksPerSecond: rate},
			}
			if !reflect.DeepEqual(control.Bindings[0], want) {
				t.Errorf("input %d single binding = %#v, want %#v", index, control.Bindings[0], want)
			}
			assertUnknownBindings(t, control.Bindings[1:])
			if control.Raw.InputIndex != index {
				t.Errorf("input %d source index = %d", index, control.Raw.InputIndex)
			}
			raw := turbo20
			if index == 1 {
				raw = turbo50
			}
			assertLiteralRaw(t, control.Raw.Fields, json.RawMessage(raw))
		}
	})

	for _, test := range []struct {
		name string
		raw  string
	}{
		{"non-Turbo T", strings.Replace(turbo20, `"isTurbo":true`, `"isTurbo":false`, 1)},
		{"missing isTurbo", strings.Replace(turbo20, `,"isTurbo":true`, "", 1)},
		{"null isTurbo", strings.Replace(turbo20, `"isTurbo":true`, `"isTurbo":null`, 1)},
		{"explicit toggle false", strings.Replace(turbo20, `"isTurbo":true`, `"isTurbo":true,"isToggleOnHold":false`, 1)},
		{"toggle null", strings.Replace(turbo20, `"isTurbo":true`, `"isTurbo":true,"isToggleOnHold":null`, 1)},
		{"string interval", strings.Replace(turbo20, `"turboInterval":20`, `"turboInterval":"20"`, 1)},
		{"decimal interval", strings.Replace(turbo20, `"turboInterval":20`, `"turboInterval":20.0`, 1)},
		{"unobserved interval", strings.Replace(turbo20, `"turboInterval":20`, `"turboInterval":25`, 1)},
		{"modifier residue", strings.Replace(turbo20, `"metaValues":["0","0","0"]`, `"metaValues":["ControlLeft","0","0"]`, 1)},
		{"missing inactive flag", strings.Replace(turbo20, `,"isHoldLong":false`, "", 1)},
		{"null inactive interval", strings.Replace(turbo20, `"turboIntervalDouble":0`, `"turboIntervalDouble":null`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			control := normalizeKeyboardInput(t, json.RawMessage(test.raw))
			if got := control.Bindings[0]; got.Kind != profile.BindingUnknown || got.Unknown == nil || got.Unknown.Reason != "unmapped_binding" || got.Actions != nil || got.Turbo != nil || got.Stick != nil || got.TriggerDelayMS != nil || got.TriggerIntervalMS != nil || got.ReleaseBehavior != nil {
				t.Fatalf("single near-match = %#v, want whole Unknown", got)
			}
			assertLiteralRaw(t, control.Raw.Fields, json.RawMessage(test.raw))
		})
	}

	t.Run("unsupported source", func(t *testing.T) {
		t.Parallel()
		actual, err := profileadapter.Normalize(parseExport(t, `{"id":"synthetic","inputs":[`+turbo20+`]}`), profile.SourceMetadata{SoftwareRelease: "2.0.3", SourceScope: admittedSource.SourceScope})
		if !reflect.DeepEqual(actual, profile.ProfileBundle{}) {
			t.Fatalf("Normalize() result = %#v, want zero bundle", actual)
		}
		var normalizeError *profileadapter.NormalizeError
		if !errors.As(err, &normalizeError) || normalizeError.Code != profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION {
			t.Fatalf("Normalize() error = %v, want unsupported version", err)
		}
	})
}
