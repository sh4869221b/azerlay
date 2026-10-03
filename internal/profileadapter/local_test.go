package profileadapter_test

import (
	"encoding/json"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"os"
	"reflect"
	"strings"
	"testing"
)

const localJSON = `{"id":"a","name":"Synthetic","version":1,"inputs":[{"types":["11","11","11"],"label":"control"}],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]},"extra":{"value":7}}`

func TestNormalizeLocal(t *testing.T) {
	t.Parallel()
	bundle, err := profileadapter.NormalizeLocal(parseExport(t, localJSON), "profile_a.json")
	if err != nil || bundle.Source.SourceScope != "azeron-software-local-json" || bundle.Source.SoftwareRelease != "2.0.2" || len(bundle.Profiles) != 1 || len(bundle.Profiles[0].Controls) != 1 || string(bundle.Profiles[0].Raw.Unknown["extra"]) != `{"value":7}` {
		t.Fatalf("local normalization: %#v %v", bundle, err)
	}
	if _, err := profileadapter.Normalize(parseExport(t, localJSON), bundle.Source); err == nil {
		t.Fatal("local scope admitted by export normalizer")
	}
	for _, tc := range []struct{ name, from, to string }{
		{"bool version", `"version":1`, `"version":true`}, {"string version", `"version":1`, `"version":"1"`}, {"fraction version", `"version":1`, `"version":1.0`}, {"missing version", `"version":1,`, ``}, {"missing id", `"id":"a",`, ``}, {"empty id", `"id":"a"`, `"id":""`}, {"numeric name", `"name":"Synthetic"`, `"name":1`}, {"hardware", `"isSoftware":true`, `"isSoftware":false`}, {"history missing", `"changedLogs"`, `"other"`}, {"history empty", `[{"softwareVersion":"2.0.2"}]`, `[]`}, {"history mixed", `[{"softwareVersion":"2.0.2"}]`, `[{"softwareVersion":"2.0.2"},{"softwareVersion":"2.0.3"}]`}, {"history null", `[{"softwareVersion":"2.0.2"}]`, `[null]`}, {"metadata null", `{"changedLogs":[{"softwareVersion":"2.0.2"}]}`, `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := profileadapter.NormalizeLocal(parseExport(t, strings.Replace(localJSON, tc.from, tc.to, 1)), "profile_a.json"); err == nil {
				t.Fatal("unsupported local accepted")
			}
		})
	}
	if _, err := profileadapter.NormalizeLocal(parseExport(t, localJSON), "profile_b.json"); err == nil {
		t.Fatal("filename mismatch accepted")
	}
	if _, err := profileadapter.NormalizeLocal(parseExport(t, `{"profiles":[`+localJSON+`]}`), "profile_a.json"); err == nil {
		t.Fatal("bundle accepted")
	}
}

func TestNormalizeLocalReusesBindingSemantics(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/bindings.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Raw json.RawMessage }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		input := strings.Replace(localJSON, `[{"types":["11","11","11"],"label":"control"}]`, `[`+string(tc.Raw)+`]`, 1)
		raw := parseExport(t, input)
		local, err := profileadapter.NormalizeLocal(raw, "profile_a.json")
		if err != nil {
			t.Fatal(err)
		}
		exported, err := profileadapter.Normalize(raw, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(local.Profiles, exported.Profiles) {
			t.Fatal("local changed existing binding interpretation")
		}
	}
}
