package profileadapter_test

import (
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
)

func TestUnknownDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		trigger profile.TriggerKind
		want    string
	}{
		{"unsupported key", `{"types":["1","11","11"],"keyValues":["KeyPrivate","0",0,"KeyU"],"metaValues":["ControlLeft","0","0"],"label":"PRIVATE_LABEL","macro":{"steps":["PRIVATE_MACRO"]},"other":"PRIVATE_FIELD"}`, profile.TriggerSingle, `types[0]="1", keyValues[0]="KeyPrivate", keyValues[3]="KeyU", metaValues[0]="ControlLeft"`},
		{"unknown trigger", `{"types":["3","11","11"],"keyValues":["PRIVATE_KEY"],"metaValues":["PRIVATE_META"]}`, profile.TriggerUnknown, `types[0]="3", types[1]="11", types[2]="11"`},
		{"escaped", `{"types":["1","11","11"],"keyValues":["ESC\n\u00e9\"\\"],"metaValues":["0"]}`, profile.TriggerSingle, `types[0]="1", keyValues[0]="ESC\n\u00e9\"\\"`},
		{"structured", `{"types":[{},false,null],"keyValues":{"private":"PRIVATE"},"metaValues":[[],true,null]}`, profile.TriggerUnknown, ""},
		{"no allowed fields", `{"label":"PRIVATE_LABEL","macro":{"steps":["PRIVATE_MACRO"]}}`, profile.TriggerUnknown, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(test.input)), admittedSource)
			if err != nil {
				t.Fatal(err)
			}
			binding := bindingForTrigger(t, bundle.Profiles[0].Controls[0].Bindings, test.trigger)
			if binding.Kind != profile.BindingUnknown || binding.Unknown == nil || binding.Unknown.Reason != "unmapped_binding" || binding.Unknown.RawDisplay != test.want {
				t.Fatalf("unknown display = %#v; want %q", binding, test.want)
			}
			if len(binding.Unknown.RawDisplay) > 256 || strings.ContainsAny(binding.Unknown.RawDisplay, "\n\r") || strings.Contains(binding.Unknown.RawDisplay, "PRIVATE_LABEL") || strings.Contains(binding.Unknown.RawDisplay, "PRIVATE_MACRO") || strings.Contains(binding.Unknown.RawDisplay, "PRIVATE_FIELD") {
				t.Fatal("unsafe display")
			}
		})
	}
}

func TestUnknownDisplayBound(t *testing.T) {
	t.Parallel()
	input := `{"types":["1","11","11"],"keyValues":["` + strings.Repeat("\u00e9", 150) + `","` + strings.Repeat("A", 150) + `","` + strings.Repeat("B", 150) + `"],"metaValues":["ControlLeft"]}`
	bundle, err := profileadapter.Normalize(parseExport(t, singleWithInputs(input)), admittedSource)
	if err != nil {
		t.Fatal(err)
	}
	display := bundle.Profiles[0].Controls[0].Bindings[0].Unknown.RawDisplay
	if len(display) > 256 || !strings.HasSuffix(display, "...") || !strings.Contains(display, `\u00e9`) {
		t.Fatalf("bounded display = %q (%d bytes)", display, len(display))
	}
	for _, b := range []byte(display) {
		if b < ' ' || b > '~' {
			t.Fatalf("non-ASCII display = %q", display)
		}
	}
}
