package profileadapter_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
)

func TestNormalizeIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, input string
		want        profile.SourceIdentity
	}{
		{"missing", `{}`, profile.SourceIdentity{}},
		{"observed", `{"id":4,"pinOne":5,"pinTwo":255}`, profile.SourceIdentity{InputID: intPointer(4), PinOne: intPointer(5), PinTwo: intPointer(255)}},
		{"zero pin", `{"id":1,"pinOne":0}`, profile.SourceIdentity{InputID: intPointer(1), PinOne: intPointer(0)}},
		{"null ID", `{"id":null,"pinOne":5}`, profile.SourceIdentity{PinOne: intPointer(5), Invalid: true}},
		{"wrong type", `{"id":"4","pinOne":true,"pinTwo":7}`, profile.SourceIdentity{PinTwo: intPointer(7), Invalid: true}},
		{"out of range", `{"id":0,"pinOne":-1,"pinTwo":9999999999999999999999999999}`, profile.SourceIdentity{Invalid: true}},
		{"fraction and exponent", `{"id":1.0,"pinOne":1e0,"pinTwo":255}`, profile.SourceIdentity{PinTwo: intPointer(255), Invalid: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw := parseExport(t, `{"id":"synthetic","inputs":[`+tc.input+`]}`)
			bundle, err := profileadapter.Normalize(raw, admittedSource)
			if err != nil {
				t.Fatal(err)
			}
			control := bundle.Profiles[0].Controls[0]
			if !reflect.DeepEqual(control.SourceIdentity, tc.want) {
				t.Errorf("identity = %#v, want %#v", control.SourceIdentity, tc.want)
			}
			if len(control.Raw.Fields) != len(raw.Single.Inputs[0]) {
				t.Error("raw source field count changed")
			}
			for key, value := range raw.Single.Inputs[0] {
				if !bytes.Equal(control.Raw.Fields[key], value) {
					t.Errorf("raw source field %s changed", key)
				}
			}
		})
	}
}

func TestNormalizeIdentityOwnership(t *testing.T) {
	t.Parallel()
	raw := parseExport(t, `{"id":"synthetic","inputs":[{"id":4,"pinOne":5,"pinTwo":255},{"id":4,"pinOne":5,"pinTwo":255}]}`)
	first, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatal(err)
	}
	second, err := profileadapter.Normalize(raw, admittedSource)
	if err != nil {
		t.Fatal(err)
	}
	*first.Profiles[0].Controls[0].SourceIdentity.InputID = 99
	*first.Profiles[0].Controls[0].SourceIdentity.PinOne = 99
	*first.Profiles[0].Controls[0].SourceIdentity.PinTwo = 99
	if *first.Profiles[0].Controls[1].SourceIdentity.InputID != 4 || *second.Profiles[0].Controls[0].SourceIdentity.InputID != 4 || *second.Profiles[0].Controls[0].SourceIdentity.PinOne != 5 || *second.Profiles[0].Controls[0].SourceIdentity.PinTwo != 255 {
		t.Fatal("identity pointers alias another control or normalization")
	}
	raw.Single.Inputs[0]["id"][0] = '9'
	if string(first.Profiles[0].Controls[0].Raw.Fields["id"]) != "4" {
		t.Fatal("raw source fields alias caller")
	}
}

func intPointer(value int) *int { return &value }
