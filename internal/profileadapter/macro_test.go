package profileadapter_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

const (
	buttonStep = `{"type":"Button","direction":"Full","duration":25,"keyCode":"KeyQ"}`
	delayStep  = `{"type":"Delay","direction":"Full","duration":10}`
)

func TestNormalizeMacro(t *testing.T) {
	t.Run("recognized_boundaries_stay_unknown", func(t *testing.T) {
		for _, count := range []int{0, 1000} {
			t.Run(strconv.Itoa(count), func(t *testing.T) {
				input := macroInput(repeatedSteps(buttonStep, count))
				bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(input)), admittedSource)
				if err != nil {
					t.Fatalf("Normalize() error = %v", err)
				}
				assertMacroUnknown(t, bundle.Profiles[0].Controls[0])
				assertLiteralRaw(t, bundle.Profiles[0].Controls[0].Raw.Fields, json.RawMessage(input))
			})
		}
	})

	t.Run("limit_is_per_input", func(t *testing.T) {
		input := macroInput(repeatedSteps(delayStep, 1000))
		bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(input+","+input)), admittedSource)
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		if len(bundle.Profiles[0].Controls) != 2 {
			t.Fatalf("controls = %d, want 2", len(bundle.Profiles[0].Controls))
		}
		assertMacroUnknown(t, bundle.Profiles[0].Controls[0])
		assertMacroUnknown(t, bundle.Profiles[0].Controls[1])
	})

	t.Run("recognized_overflow_returns_code_and_zero_bundle", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			macro string
		}{
			{name: "button", macro: macroObject(repeatedSteps(buttonStep, 1001))},
			{name: "delay", macro: macroObject(repeatedSteps(delayStep, 1001))},
			{name: "repeat_true", macro: `{"v":1,"repeat":true,"steps":[` + repeatedSteps(buttonStep, 1001) + `]}`},
			{name: "opaque_numeric_parameters", macro: macroObject(repeatedSteps(`{"type":"Button","direction":"Full","duration":1e1000000,"keyCode":987654321098765432109876543210}`, 1001))},
		} {
			t.Run(test.name, func(t *testing.T) {
				input := `{"types":["16","11","11"],"macro":` + test.macro + `}`
				assertMacroLimit(t, parseExport(t, singleWithInputs(input)), admittedSource)
			})
		}
	})

	t.Run("later_overflow_discards_mapped_result", func(t *testing.T) {
		mapped := followupSingle(`"KeyU"`, `"0"`)
		overflow := macroInput(repeatedSteps(buttonStep, 1001))
		assertMacroLimit(t, parseExport(t, singleWithInputs(mapped+","+overflow)), admittedSource)
	})

	t.Run("admission_precedes_macro", func(t *testing.T) {
		input := macroInput(repeatedSteps(buttonStep, 1001))
		bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(input)), profile.SourceMetadata{
			SoftwareRelease: "2.0.3",
			SourceScope:     admittedSource.SourceScope,
		})
		if !reflect.DeepEqual(bundle, profile.ProfileBundle{}) {
			t.Fatalf("Normalize() = %#v, want zero bundle", bundle)
		}
		var normalizeError *profileadapter.NormalizeError
		if !errors.As(err, &normalizeError) || normalizeError.Code != profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION {
			t.Fatalf("Normalize() error = %T %v, want %q", err, err, profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION)
		}
	})

	t.Run("unknown_grammar_has_no_semantic_count", func(t *testing.T) {
		unknownAfter1001 := repeatedSteps(buttonStep, 1001) + `,{"type":"Button","direction":"Full","duration":25,"keyCode":"KeyQ","extra":true}`
		for _, test := range []struct {
			name  string
			input string
		}{
			{name: "unknown_last_of_1001", input: macroInput(repeatedSteps(buttonStep, 1000) + `,{"type":"Other","direction":"Full","duration":1,"keyCode":1}`)},
			{name: "unknown_after_1001", input: macroInput(unknownAfter1001)},
			{name: "other_type_context", input: `{"types":["11","11","11"],"macro":` + macroObject(repeatedSteps(buttonStep, 1001)) + `}`},
			{name: "long_macro", input: `{"types":["16","11","11"],"longMacro":` + macroObject(repeatedSteps(buttonStep, 1001)) + `}`},
			{name: "double_macro", input: `{"types":["16","11","11"],"doubleMacro":` + macroObject(repeatedSteps(buttonStep, 1001)) + `}`},
		} {
			t.Run(test.name, func(t *testing.T) {
				bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(test.input)), admittedSource)
				if err != nil {
					t.Fatalf("Normalize() error = %v", err)
				}
				assertMacroUnknown(t, bundle.Profiles[0].Controls[0])
				assertLiteralRaw(t, bundle.Profiles[0].Controls[0].Raw.Fields, json.RawMessage(test.input))
			})
		}
	})

	t.Run("exact_grammar_boundary", func(t *testing.T) {
		validSteps := repeatedSteps(buttonStep, 1001)
		for _, test := range []struct {
			name  string
			macro string
		}{
			{name: "v_string", macro: `{"v":"1","repeat":false,"steps":[` + validSteps + `]}`},
			{name: "v_null", macro: `{"v":null,"repeat":false,"steps":[` + validSteps + `]}`},
			{name: "v_other_number", macro: `{"v":1.0,"repeat":false,"steps":[` + validSteps + `]}`},
			{name: "repeat_string", macro: `{"v":1,"repeat":"false","steps":[` + validSteps + `]}`},
			{name: "extra_container_key", macro: `{"v":1,"repeat":false,"steps":[` + validSteps + `],"extra":0}`},
			{name: "button_wrong_direction", macro: macroObject(repeatedSteps(buttonStep, 1000) + `,{"type":"Button","direction":"Down","duration":1,"keyCode":1}`)},
			{name: "button_missing_key_code", macro: macroObject(repeatedSteps(buttonStep, 1000) + `,{"type":"Button","direction":"Full","duration":1}`)},
			{name: "button_boolean_key_code", macro: macroObject(repeatedSteps(buttonStep, 1000) + `,{"type":"Button","direction":"Full","duration":1,"keyCode":true}`)},
			{name: "button_extra_key", macro: macroObject(repeatedSteps(buttonStep, 1000) + `,{"type":"Button","direction":"Full","duration":1,"keyCode":1,"extra":0}`)},
			{name: "delay_has_key_code", macro: macroObject(repeatedSteps(buttonStep, 1000) + `,{"type":"Delay","direction":"Full","duration":1,"keyCode":1}`)},
			{name: "delay_string_duration", macro: macroObject(repeatedSteps(buttonStep, 1000) + `,{"type":"Delay","direction":"Full","duration":"1"}`)},
		} {
			t.Run(test.name, func(t *testing.T) {
				input := `{"types":["16","11","11"],"macro":` + test.macro + `}`
				bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(input)), admittedSource)
				if err != nil {
					t.Fatalf("Normalize() error = %v", err)
				}
				assertMacroUnknown(t, bundle.Profiles[0].Controls[0])
				assertLiteralRaw(t, bundle.Profiles[0].Controls[0].Raw.Fields, json.RawMessage(input))
			})
		}
	})

	t.Run("missing_steps_preserves_raw", func(t *testing.T) {
		input := `{"types":["16","11","11"],"macro":{"v":1,"repeat":false}}`
		bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(input)), admittedSource)
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		assertMacroUnknown(t, bundle.Profiles[0].Controls[0])
		assertLiteralRaw(t, bundle.Profiles[0].Controls[0].Raw.Fields, json.RawMessage(input))
	})
}

func assertMacroLimit(t *testing.T, raw profileraw.RawExport, source profile.SourceMetadata) {
	t.Helper()
	bundle, err := profileadapter.Normalize(raw, source)
	if !reflect.DeepEqual(bundle, profile.ProfileBundle{}) {
		t.Fatal("Normalize() returned a nonzero bundle, want exact zero bundle")
	}
	var normalizeError *profileadapter.NormalizeError
	if !errors.As(err, &normalizeError) || normalizeError.Code != profileadapter.ERR_IMPORT_LIMIT_EXCEEDED {
		t.Fatalf("Normalize() error = %T %v, want %q", err, err, profileadapter.ERR_IMPORT_LIMIT_EXCEEDED)
	}
	if err.Error() != string(profileadapter.ERR_IMPORT_LIMIT_EXCEEDED) {
		t.Fatalf("error text = %q, want exact code", err.Error())
	}
}

func assertMacroUnknown(t *testing.T, control profile.ControlBinding) {
	t.Helper()
	if len(control.Bindings) != 3 {
		t.Fatalf("bindings = %d, want 3", len(control.Bindings))
	}
	for _, binding := range control.Bindings {
		if binding.Kind != profile.BindingUnknown || binding.Unknown == nil || binding.Unknown.Reason != "unmapped_binding" {
			t.Fatalf("binding = %#v, want Unknown", binding)
		}
	}
}

func singleWithInputs(inputs string) string {
	return `{"id":"macro-test","inputs":[` + inputs + `]}`
}

func macroInput(steps string) string {
	return `{"types":["16","11","11"],"macro":` + macroObject(steps) + `}`
}

func macroObject(steps string) string {
	return `{"v":1,"repeat":false,"steps":[` + steps + `]}`
}

func repeatedSteps(step string, count int) string {
	if count == 0 {
		return ""
	}
	return strings.Repeat(step+",", count-1) + step
}
