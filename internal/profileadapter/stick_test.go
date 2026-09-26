package profileadapter_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func TestStickNormalize(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"keyboard", "xbox"} {
		for _, root := range []string{"single", "bundle"} {
			t.Run(mode+"/"+root, func(t *testing.T) {
				t.Parallel()
				input := readStickFixture(t, mode)
				if root == "bundle" {
					input = append(append([]byte(`{"profiles":[`), input...), []byte(`]}`)...)
				}
				actual := normalizeStickExport(t, input)
				control := actual.Profiles[0].Controls[0]
				primary := control.Bindings[0]
				if primary.Kind != profile.BindingStick {
					t.Fatalf("primary kind = %q, want stick", primary.Kind)
				}
				want := profile.StickBinding{Mode: profile.StickMode(mode)}
				if mode == "keyboard" {
					want.KeyboardDirections = profile.KeyboardDirections{Up: profile.KEY_W, Right: profile.KEY_D, Down: profile.KEY_S, Left: profile.KEY_A}
				}
				if primary.Stick == nil || *primary.Stick != want || primary.Actions != nil || primary.Unknown != nil {
					t.Fatalf("primary = %#v, want stick %#v without actions or Unknown", primary, want)
				}
				assertStickTriggerSlots(t, control.Bindings)
				if actual.RootKind != profile.RootKind(root) || actual.SchemaVersion != 1 || actual.Source != admittedSource {
					t.Fatalf("bundle metadata = %#v", actual)
				}
			})
		}
	}
}

func TestStickObservedXboxAngle(t *testing.T) {
	t.Parallel()
	for _, angle := range []struct {
		token string
		value int
	}{{"0", 0}, {"90", 90}} {
		t.Run(angle.token, func(t *testing.T) {
			t.Parallel()
			input := stickInput(t, "xbox-observed")
			setAnalogStickField(t, input, "angle", angle.token)
			actual := normalizeStickExport(t, marshalJSON(t, map[string]any{"id": "synthetic", "inputs": []any{input}}))
			control := actual.Profiles[0].Controls[0]
			want := profile.TriggerBinding{
				Trigger: profile.TriggerSingle,
				Kind:    profile.BindingStick,
				Stick:   &profile.StickBinding{Mode: profile.StickModeXbox, AngleDegrees: angle.value},
			}
			if !reflect.DeepEqual(control.Bindings[0], want) {
				t.Fatalf("primary = %#v, want %#v", control.Bindings[0], want)
			}
			assertStickTriggerSlots(t, control.Bindings)
			if !bytes.Equal(marshalJSON(t, control.Raw.Fields), marshalJSON(t, input)) {
				t.Fatal("observed Xbox lost raw fields")
			}
		})
	}

	for _, test := range []struct {
		name, field, token string
		analog             bool
	}{
		{"angle 45", "angle", "45", true},
		{"string angle 90", "angle", `"90"`, true},
		{"subType present", "subType", `"11"`, false},
		{"subType null", "subType", "null", false},
		{"toggle false", "isToggleOnHold", "false", false},
		{"toggle null", "isToggleOnHold", "null", false},
		{"nonzero lower limit", "lowerLimit", "1", true},
		{"inverted axis", "invertXAxis", "true", true},
		{"changed sensitivity", "sensitivity", "1", true},
		{"wrong hold type", "holdType", "1", true},
		{"missing inactive flag", "isHoldLong", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := stickInput(t, "xbox-observed")
			if test.analog {
				setAnalogStickField(t, input, test.field, test.token)
			} else {
				setStickField(input, test.field, test.token)
			}
			assertUnknownStick(t, input)
		})
	}
}

func setAnalogStickField(t *testing.T, input profileraw.RawInput, field, token string) {
	t.Helper()
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(input["analogSettings"], &settings); err != nil {
		t.Fatal(err)
	}
	setStickField(settings, field, token)
	input["analogSettings"] = marshalJSON(t, settings)
}

func TestStickUnsupportedContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		field string
		value string
	}{
		{"types", `["3","11","11"]`}, {"types", `[4,"11","11"]`},
		{"types", `["4","11","1"]`}, {"subType", `11`},
		{"analogSettings", `null`}, {"analogSettings", `[]`},
		{"keyValues", `["KeyU","0","0","0"]`}, {"metaValues", `["ControlLeft","0","0"]`},
		{"subType", ""}, {"subType", "null"}, {"analogSettings", ""},
		{"keyValues", ""}, {"metaValues", "null"},
	}
	for _, field := range []string{"isHold", "isTurbo", "isToggleOnHold"} {
		tests = append(tests, struct{ field, value string }{field, "true"})
	}
	for _, test := range tests {
		t.Run(test.field+"="+test.value, func(t *testing.T) {
			t.Parallel()
			input := stickInput(t, "keyboard")
			setStickField(input, test.field, test.value)
			assertUnknownStick(t, input)
		})
	}
}

func TestStickUnsupportedSettings(t *testing.T) {
	t.Parallel()
	type settingCase struct{ mode, field, value string }
	tests := []settingCase{
		{"keyboard", "angle", ""}, {"keyboard", "angle", "null"}, {"keyboard", "angle", `"0"`},
		{"keyboard", "invertXAxis", ""}, {"keyboard", "invertXAxis", "null"}, {"keyboard", "invertXAxis", `"false"`},
		{"xbox", "isCombinedAnalog", "true"},
	}
	for _, field := range []string{"angle", "lowerLimit", "upperLimit", "isRightAnalog", "invertXAxis", "invertYAxis", "isCombinedAnalog", "isEightDirectionalTrigger", "isHoldTrigger", "isAnalogSmoothing", "isAngleLock"} {
		value := "true"
		if field == "angle" || field == "lowerLimit" || field == "upperLimit" {
			value = "1"
		}
		tests = append(tests, settingCase{"keyboard", field, value})
	}
	for _, test := range tests {
		t.Run(test.mode+"/"+test.field+"="+test.value, func(t *testing.T) {
			t.Parallel()
			input := stickInput(t, test.mode)
			var settings map[string]json.RawMessage
			if err := json.Unmarshal(input["analogSettings"], &settings); err != nil {
				t.Fatal(err)
			}
			setStickField(settings, test.field, test.value)
			input["analogSettings"] = marshalJSON(t, settings)
			assertUnknownStick(t, input)
		})
	}
}

func TestStickDirectionAdmission(t *testing.T) {
	t.Parallel()
	for _, tuple := range []string{`[88,0,0]`, `[87,null,0]`, `["87",0,0]`, `[87,0]`, `null`} {
		t.Run(tuple, func(t *testing.T) {
			t.Parallel()
			input := stickInput(t, "keyboard")
			input["analogSettings"] = bytes.Replace(input["analogSettings"], []byte("[87, 0, 0]"), []byte(tuple), 1)
			assertUnknownStick(t, input)
		})
	}
	for _, mode := range []string{"keyboard", "xbox"} {
		t.Run(mode+" missing direction assignments", func(t *testing.T) {
			t.Parallel()
			input := stickInput(t, mode)
			var settings map[string]json.RawMessage
			if err := json.Unmarshal(input["analogSettings"], &settings); err != nil {
				t.Fatal(err)
			}
			delete(settings, "analogKeys")
			input["analogSettings"] = marshalJSON(t, settings)
			if mode == "keyboard" {
				assertUnknownStick(t, input)
				return
			}
			actual := normalizeStickExport(t, marshalJSON(t, map[string]any{"id": "synthetic", "inputs": []any{input}}))
			binding := actual.Profiles[0].Controls[0].Bindings[0]
			if binding.Stick == nil || *binding.Stick != (profile.StickBinding{Mode: profile.StickModeXbox}) {
				t.Fatalf("Xbox dormant keyboard assignment affected binding: %#v", binding)
			}
		})
	}
}

func TestStickDuplicateControlsAndOpaqueFields(t *testing.T) {
	t.Parallel()
	input := stickInput(t, "keyboard")
	input["future"] = json.RawMessage(`{"opaque":true}`)
	input["pin"] = json.RawMessage(`"synthetic"`)
	actual := normalizeStickExport(t, marshalJSON(t, map[string]any{"id": "synthetic", "inputs": []any{profileraw.RawInput{}, input, input}}))
	controls := actual.Profiles[0].Controls
	if len(controls) != 3 {
		t.Fatalf("control count = %d, want 3", len(controls))
	}
	for index := 1; index < 3; index++ {
		if controls[index].Bindings[0].Kind != profile.BindingStick || controls[index].Raw.InputIndex != index || !bytes.Equal(marshalJSON(t, controls[index].Raw.Fields), marshalJSON(t, input)) {
			t.Fatalf("duplicate control %d lost interpretation or raw fields: %#v", index, controls[index])
		}
	}
	controls[1].Raw.Fields["future"][0] = 'X'
	if controls[2].Raw.Fields["future"][0] != '{' || input["future"][0] != '{' {
		t.Fatal("normalized raw data aliases another control or source")
	}
}

func assertUnknownStick(t *testing.T, input profileraw.RawInput) {
	t.Helper()
	actual := normalizeStickExport(t, marshalJSON(t, map[string]any{"id": "synthetic", "inputs": []any{input}}))
	control := actual.Profiles[0].Controls[0]
	primary := control.Bindings[0]
	if primary.Kind != profile.BindingUnknown || primary.Unknown == nil || primary.Stick != nil || primary.Actions != nil {
		t.Fatalf("unsupported context primary = %#v, want Unknown", primary)
	}
	if !bytes.Equal(marshalJSON(t, control.Raw.Fields), marshalJSON(t, input)) {
		t.Fatal("unsupported context lost raw fields")
	}
	if bytes.Equal(input["types"], []byte(`["4", "11", "11"]`)) || bytes.Equal(input["types"], []byte(`["21", "11", "11"]`)) {
		assertStickTriggerSlots(t, control.Bindings)
	}
}

func assertStickTriggerSlots(t *testing.T, bindings []profile.TriggerBinding) {
	t.Helper()
	if len(bindings) != 3 {
		t.Fatalf("binding count = %d, want 3", len(bindings))
	}
	for index, trigger := range []profile.TriggerKind{profile.TriggerSingle, profile.TriggerLong, profile.TriggerDouble} {
		if bindings[index].Trigger != trigger || (index > 0 && (bindings[index].Kind != profile.BindingUnknown || bindings[index].Unknown == nil)) {
			t.Fatalf("trigger slot %d = %#v", index, bindings[index])
		}
	}
}

func stickInput(t *testing.T, mode string) profileraw.RawInput {
	t.Helper()
	return parseExport(t, string(readStickFixture(t, mode))).Single.Inputs[0]
}

func setStickField(fields map[string]json.RawMessage, field, value string) {
	if value == "" {
		delete(fields, field)
	} else {
		fields[field] = json.RawMessage(value)
	}
}

func marshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readStickFixture(t *testing.T, mode string) []byte {
	t.Helper()
	input, err := os.ReadFile("testdata/stick-" + mode + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func normalizeStickExport(t *testing.T, input []byte) profile.ProfileBundle {
	t.Helper()
	document, err := profiledecode.DecodeReader(bytes.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeReader() error = %v", err)
	}
	raw, err := profileraw.Parse(document)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	actual, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	return actual
}
