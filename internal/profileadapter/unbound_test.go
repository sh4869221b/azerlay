package profileadapter_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
)

func TestNormalizeUnbound(t *testing.T) {
	t.Parallel()
	fixture, err := os.ReadFile("testdata/unbound.input.json")
	if err != nil {
		t.Fatal(err)
	}
	raw := parseExport(t, string(fixture))
	bundle, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatal(err)
	}
	control := bundle.Profiles[0].Controls[0]
	if len(control.Bindings) != 3 || !reflect.DeepEqual(control.Bindings[0], profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingUnbound}) {
		t.Fatalf("SINGLE = %#v, want exact unbound", control.Bindings)
	}
	for _, binding := range control.Bindings[1:] {
		if binding.Kind != profile.BindingUnknown || binding.Unknown == nil {
			t.Fatalf("inactive slot = %#v, want unknown", binding)
		}
	}
	if string(control.Raw.Fields["pinTwo"]) != "255" {
		t.Fatal("raw fields changed")
	}

	mutations := []struct {
		name, field, value string
		remove             bool
	}{
		{"missing semantic", "featureDelay", "", true},
		{"null semantic", "featureDelay", "null", false},
		{"different delay", "featureDelay", "501", false},
		{"changed hold", "isHold", "true", false},
		{"changed turbo", "isTurbo", "true", false},
		{"nonempty macro", "macro", `{"repeat":false,"steps":[{"synthetic":true}],"v":1}`, false},
		{"extra semantic", "future", "true", false},
		{"number token", "featureDelay", "5e2", false},
		{"array order", "types", `["11","1","11"]`, false},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := make(map[string]json.RawMessage, len(raw.Single.Inputs[0]))
			for key, value := range raw.Single.Inputs[0] {
				input[key] = value
			}
			if tc.remove {
				delete(input, tc.field)
			} else {
				input[tc.field] = json.RawMessage(tc.value)
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			modified := parseExport(t, `{"id":"synthetic","inputs":[`+string(encoded)+`]}`)
			result, err := profileadapter.Normalize(modified, admittedSource)
			if err != nil {
				t.Fatal(err)
			}
			if result.Profiles[0].Controls[0].Bindings[0].Kind == profile.BindingUnbound {
				t.Fatal("near match incorrectly normalized as unbound")
			}
		})
	}
}
