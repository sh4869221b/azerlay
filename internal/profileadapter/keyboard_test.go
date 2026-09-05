package profileadapter_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
)

type keyboardFixture struct {
	Case            string                     `json:"case"`
	SoftwareRelease string                     `json:"software_release"`
	SourceRefs      []string                   `json:"source_refs"`
	Raw             json.RawMessage            `json:"raw"`
	Context         map[string]json.RawMessage `json:"context"`
	Expected        struct {
		Disposition string                   `json:"disposition"`
		Bindings    []keyboardFixtureBinding `json:"bindings"`
		Raw         json.RawMessage          `json:"raw"`
	} `json:"expected"`
	RawTokens map[string]string `json:"raw_tokens,omitempty"`
}

type keyboardFixtureBinding struct {
	Kind              profile.BindingKind `json:"kind"`
	Trigger           profile.TriggerKind `json:"trigger"`
	Actions           []profile.Action    `json:"actions"`
	TriggerDelayMS    *int                `json:"trigger_delay_ms,omitempty"`
	TriggerIntervalMS *int                `json:"trigger_interval_ms,omitempty"`
	ReleaseBehavior   *string             `json:"release_behavior,omitempty"`
}

func TestNormalizeKeyboard(t *testing.T) {
	t.Parallel()

	fixtures := readKeyboardFixtures(t)
	if len(fixtures) != 7 {
		t.Fatalf("fixture count = %d, want 7", len(fixtures))
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Case, func(t *testing.T) {
			t.Parallel()
			if fixture.SoftwareRelease != "2.0.2" || len(fixture.SourceRefs) == 0 || string(fixture.Context["source_scope"]) != `"azeron-software-export"` || fixture.Expected.Disposition != "mapped" {
				t.Fatalf("fixture contract = %#v", fixture)
			}
			control := normalizeKeyboardInput(t, fixture.Raw)
			if got := mappedFixtureBindings(control.Bindings); !reflect.DeepEqual(got, fixture.Expected.Bindings) {
				t.Fatalf("mapped bindings = %#v, want %#v", got, fixture.Expected.Bindings)
			}
			assertUnknownBindings(t, control.Bindings)
			assertLiteralRaw(t, control.Raw.Fields, fixture.Expected.Raw)
			for field, token := range fixture.RawTokens {
				if string(control.Raw.Fields[field]) != token {
					t.Errorf("raw token %s = %q, want %q", field, control.Raw.Fields[field], token)
				}
			}
		})
	}

	t.Run("original predicates coexist and duplicate controls stay ordered", func(t *testing.T) {
		t.Parallel()
		const source = `{"id":"combined","inputs":[{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500,"keyValuesDouble":["KeyP","KeyL","0","0"],"metaValuesDouble":["0","0","0"],"doubleDelay":150},{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500},{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}]}`
		actual, err := profileadapter.Normalize(parseExport(t, source), admittedSource)
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		want := [][]profile.TriggerKind{{profile.TriggerLong, profile.TriggerDouble}, {profile.TriggerLong}, {profile.TriggerLong}}
		for index, control := range actual.Profiles[0].Controls {
			if got := mappedTriggers(control.Bindings); !reflect.DeepEqual(got, want[index]) || control.Raw.InputIndex != index {
				t.Errorf("control %d mapped/provenance = %v/%d, want %v/%d", index, got, control.Raw.InputIndex, want[index], index)
			}
		}
	})

	negativeCases := []struct {
		name    string
		raw     string
		mapped  []profile.TriggerKind
		unknown profile.TriggerKind
	}{
		{"missing types", `{"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerUnknown},
		{"null types", `{"types":null,"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerUnknown},
		{"wrong-type types", `{"types":[1,1,1],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerUnknown},
		{"missing flag", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"null flag", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":null,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"wrong-type flag", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":"false","featureDelay":500}`, nil, profile.TriggerLong},
		{"true flag", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":true,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"missing key", `{"types":["1","1","1"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"null key", `{"types":["1","1","1"],"keyValuesLong":null,"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"wrong-type key", `{"types":["1","1","1"],"keyValuesLong":22,"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"missing modifier", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"null modifier", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":null,"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"wrong-type modifier", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":0,"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500}`, nil, profile.TriggerLong},
		{"missing time", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false}`, nil, profile.TriggerLong},
		{"null time", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":null}`, nil, profile.TriggerLong},
		{"wrong-type time", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":"500"}`, nil, profile.TriggerLong},
		{"reordered keys", `{"types":["1","1","1"],"keyValuesDouble":["KeyL","KeyP","0","0"],"metaValuesDouble":["0","0","0"],"doubleDelay":150}`, nil, profile.TriggerDouble},
		{"reordered modifiers", `{"types":["1","11","11"],"keyValues":["KeyU","0","0","0"],"metaValues":["0","ControlLeft","0"],"isHold":false,"isTurbo":false,"isToggleOnHold":false,"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"]}`, nil, profile.TriggerSingle},
		{"wrong key array length", `{"types":["1","1","1"],"keyValuesDouble":["KeyP","KeyL","0"],"metaValuesDouble":["0","0","0"],"doubleDelay":150}`, nil, profile.TriggerDouble},
		{"wrong modifier array length", `{"types":["1","1","1"],"keyValuesDouble":["KeyP","KeyL","0","0"],"metaValuesDouble":["0","0"],"doubleDelay":150}`, nil, profile.TriggerDouble},
		{"unknown modifier", followupSingle(`"KeyU"`, `"SyntheticModifier"`), nil, profile.TriggerSingle},
		{"alternate decimal token", originalLong(`500.0`), nil, profile.TriggerLong},
		{"alternate exponent token", originalLong(`5e2`), nil, profile.TriggerLong},
		{"alternate timing", originalLong(`501`), nil, profile.TriggerLong},
		{"numeric legacy values", followupSingle(`22`, `0`), nil, profile.TriggerSingle},
		{"numeric token spelling", followupSingle(`"22"`, `"0"`), nil, profile.TriggerSingle},
		{"unmapped symbol family", followupSingle(`"KeyW"`, `"0"`), nil, profile.TriggerSingle},
		{"missing inactive array", `{"types":["11","1","11"],"keyValues":["0","0","0","0"],"metaValues":["0","0","0"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":1278,"keyValuesDouble":["0","0","0","0"]}`, nil, profile.TriggerLong},
		{"inactive timing creates no long action", `{"types":["11","11","1"],"keyValues":["0","0","0","0"],"metaValues":["0","0","0"],"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"featureDelay":1278,"keyValuesDouble":["KeyI","0","0","0"],"metaValuesDouble":["0","0","0"],"isHoldDouble":false,"isTurboDouble":false,"isToggleOnHoldDouble":false,"doubleDelay":123}`, []profile.TriggerKind{profile.TriggerDouble}, profile.TriggerLong},
		{"empty setting has no defaults", `{"types":["11","11","11"],"keyValues":["0","0","0","0"],"metaValues":["0","0","0"],"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"]}`, nil, profile.TriggerSingle},
		{"type array length is exact", `{"types":["1","11"],"keyValues":["KeyU","0","0","0"],"metaValues":["0","0","0"],"isHold":false,"isTurbo":false,"isToggleOnHold":false}`, nil, profile.TriggerUnknown},
		{"type array order is exact", `{"types":["11","1","11"],"keyValues":["KeyU","0","0","0"],"metaValues":["0","0","0"],"isHold":false,"isTurbo":false,"isToggleOnHold":false}`, nil, profile.TriggerSingle},
		{"long near-match leaves double mapped", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500.0,"keyValuesDouble":["KeyP","KeyL","0","0"],"metaValuesDouble":["0","0","0"],"doubleDelay":150}`, []profile.TriggerKind{profile.TriggerDouble}, profile.TriggerLong},
		{"double near-match leaves long mapped", `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":500,"keyValuesDouble":["KeyP","KeyL","0","0"],"metaValuesDouble":["0","0","0"],"doubleDelay":150.0}`, []profile.TriggerKind{profile.TriggerLong}, profile.TriggerDouble},
	}
	for _, test := range negativeCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			control := normalizeKeyboardInput(t, json.RawMessage(test.raw))
			if got := mappedTriggers(control.Bindings); !reflect.DeepEqual(got, test.mapped) {
				t.Fatalf("mapped triggers = %v, want %v", got, test.mapped)
			}
			binding := bindingForTrigger(t, control.Bindings, test.unknown)
			if binding.Kind != profile.BindingUnknown || binding.Unknown == nil || binding.Unknown.Reason != "unmapped_binding" || binding.Actions != nil || binding.TriggerDelayMS != nil || binding.TriggerIntervalMS != nil || binding.ReleaseBehavior != nil {
				t.Fatalf("unknown binding gained partial semantics: %#v", binding)
			}
			assertLiteralRaw(t, control.Raw.Fields, json.RawMessage(test.raw))
		})
	}
}

func readKeyboardFixtures(t *testing.T) []keyboardFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/bindings.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var fixtures []keyboardFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return fixtures
}

func normalizeKeyboardInput(t *testing.T, raw json.RawMessage) profile.ControlBinding {
	t.Helper()
	actual, err := profileadapter.Normalize(parseExport(t, `{"id":"keyboard","inputs":[`+string(raw)+`]}`), admittedSource)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	return actual.Profiles[0].Controls[0]
}

func mappedFixtureBindings(bindings []profile.TriggerBinding) []keyboardFixtureBinding {
	var mapped []keyboardFixtureBinding
	for _, binding := range bindings {
		if binding.Kind == profile.BindingKeyboard {
			mapped = append(mapped, keyboardFixtureBinding{binding.Kind, binding.Trigger, binding.Actions, binding.TriggerDelayMS, binding.TriggerIntervalMS, binding.ReleaseBehavior})
		}
	}
	return mapped
}

func mappedTriggers(bindings []profile.TriggerBinding) []profile.TriggerKind {
	var triggers []profile.TriggerKind
	for _, binding := range bindings {
		if binding.Kind == profile.BindingKeyboard {
			triggers = append(triggers, binding.Trigger)
		}
	}
	return triggers
}

func assertUnknownBindings(t *testing.T, bindings []profile.TriggerBinding) {
	t.Helper()
	for _, binding := range bindings {
		if binding.Kind == profile.BindingUnknown && (binding.Unknown == nil || binding.Unknown.Reason != "unmapped_binding" || binding.Actions != nil || binding.TriggerDelayMS != nil || binding.TriggerIntervalMS != nil || binding.ReleaseBehavior != nil) {
			t.Errorf("unmapped slot = %#v", binding)
		}
	}
}

func assertLiteralRaw(t *testing.T, actual map[string]json.RawMessage, expectedObject json.RawMessage) {
	t.Helper()
	var expected map[string]json.RawMessage
	if err := json.Unmarshal(expectedObject, &expected); err != nil {
		t.Fatalf("Unmarshal(expected raw) error = %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("raw fields = %#v, want %#v", actual, expected)
	}
}

func bindingForTrigger(t *testing.T, bindings []profile.TriggerBinding, trigger profile.TriggerKind) profile.TriggerBinding {
	t.Helper()
	for _, binding := range bindings {
		if binding.Trigger == trigger {
			return binding
		}
	}
	t.Fatalf("trigger %q absent from %#v", trigger, bindings)
	return profile.TriggerBinding{}
}

func originalLong(timing string) string {
	return `{"types":["1","1","1"],"keyValuesLong":["KeyU","0","0","0"],"metaValuesLong":["0","0","0"],"isHoldLong":false,"isTurboLong":false,"isToggleOnHoldLong":false,"featureDelay":` + timing + `}`
}

func followupSingle(key, modifier string) string {
	return `{"types":["1","11","11"],"keyValues":[` + key + `,"0","0","0"],"metaValues":[` + modifier + `,"0","0"],"isHold":false,"isTurbo":false,"isToggleOnHold":false,"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"]}`
}
