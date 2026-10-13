package profileadapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func TestUnknownDisplayAllocationGrowth(t *testing.T) {
	for _, tc := range []struct {
		member  string
		skipped int
	}{
		{`"entry",`, 0},
		{`"0","\u0030",0,-0.00e200,false,null,["opaque"],{"private":[1,2]},`, 8},
		{`1e-999,`, 1},
	} {
		t.Run(tc.member, func(t *testing.T) {
			input := func(count int) profileraw.RawInput {
				return profileraw.RawInput{"types": json.RawMessage("[" + strings.Repeat(tc.member, count) + `"last"]`)}
			}
			small, large := input(32), input(4096)
			want := unknownDisplay(small, profile.TriggerUnknown)
			if tc.skipped > 0 {
				want = fmt.Sprintf(`types[%d]="last"`, 4096*tc.skipped)
			}
			var got string
			smallAllocs := testing.AllocsPerRun(5, func() { got = unknownDisplay(small, profile.TriggerUnknown) })
			largeAllocs := testing.AllocsPerRun(5, func() { got = unknownDisplay(large, profile.TriggerUnknown) })
			if got != want {
				t.Fatalf("display changed with an opaque tail: %q; want %q", got, want)
			}
			if largeAllocs > smallAllocs+8 {
				t.Fatalf("display allocations grew with array length: small %.0f, large %.0f", smallAllocs, largeAllocs)
			}
		})
	}
}

func TestUnknownDisplayCompatibility(t *testing.T) {
	tests := []struct {
		name, array, want string
	}{
		{"escaped and numeric zeros", `["0","\u0030",0,-0,0.0,0e99999,1e-999,"",[],{},true,null,7]`, `types[7]="", types[12]=7`},
		{"unicode", `["\ud83d\ude00","\ud800","\udc00","é","\/","\b\f\n\r\t"]`, `types[0]="\U0001f600", types[1]="\ufffd", types[2]="\ufffd", types[3]="\u00e9", types[4]="/", types[5]="\b\f\n\r\t"`},
		{"numeric spelling and overflow", `[1e3,1.00,-2.5e-1,1e999]`, `types[0]=1e3, types[1]=1.00, types[2]=-2.5e-1, types[3]=1e999`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := unknownDisplay(profileraw.RawInput{"types": json.RawMessage(tc.array)}, profile.TriggerUnknown); got != tc.want {
				t.Fatalf("display = %q; want %q", got, tc.want)
			}
		})
	}
	array := fmt.Sprintf(`["%s","%s","%s","extra"]`, strings.Repeat("A", 80), strings.Repeat("B", 80), strings.Repeat("C", 56))
	got := unknownDisplay(profileraw.RawInput{"types": json.RawMessage(array)}, profile.TriggerUnknown)
	if len(got) != 256 || !strings.HasSuffix(got, `types[2]="`+strings.Repeat("C", 56)+`"...`) {
		t.Fatalf("exact 253-byte prefix plus ellipsis = %q (%d bytes)", got, len(got))
	}
}

func TestUnknownDisplayTriggerIndex(t *testing.T) {
	input := profileraw.RawInput{
		"types":           json.RawMessage(`[{},"long","double","ignored"]`),
		"keyValues":       json.RawMessage(`["single-key"]`),
		"keyValuesLong":   json.RawMessage(`[0,{},"long-key"]`),
		"keyValuesDouble": json.RawMessage(`[null,"double-key"]`),
	}
	for _, tc := range []struct {
		trigger profile.TriggerKind
		want    string
	}{
		{profile.TriggerSingle, `keyValues[0]="single-key"`},
		{profile.TriggerLong, `types[1]="long", keyValuesLong[2]="long-key"`},
		{profile.TriggerDouble, `types[2]="double", keyValuesDouble[1]="double-key"`},
	} {
		if got := unknownDisplay(input, tc.trigger); got != tc.want {
			t.Fatalf("trigger %s display = %q; want %q", tc.trigger, got, tc.want)
		}
	}
}

func TestEmptySettingComparisonAllocationGrowth(t *testing.T) {
	expected := emptySetting.(map[string]any)["keyValues"]
	input := func(count int) json.RawMessage {
		return json.RawMessage("[" + strings.Repeat(`"0",`, count) + `"0"]`)
	}
	small, large := input(32), input(4096)
	var matched bool
	smallAllocs := testing.AllocsPerRun(5, func() { matched = matchesEmptySettingValue(small, expected) })
	largeAllocs := testing.AllocsPerRun(5, func() { matched = matchesEmptySettingValue(large, expected) })
	if matched || largeAllocs > smallAllocs+4 {
		t.Fatalf("oversized empty-setting array: matched %t, small %.0f allocations, large %.0f", matched, smallAllocs, largeAllocs)
	}
}
