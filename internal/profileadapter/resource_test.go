package profileadapter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func TestNormalizeOpaqueArrayAllocationGrowth(t *testing.T) {
	for _, field := range []string{"types", "keyValues"} {
		t.Run(field, func(t *testing.T) {
			input := func(count int) profileraw.RawExport {
				fields := `"` + field + `":[` + strings.Repeat(`"1",`, count) + `"last"]`
				if field != "types" {
					fields += `,"types":["1","11","11"]`
				}
				return parseExport(t, singleWithInputs("{"+fields+"}"))
			}
			small, large := input(32), input(4096)
			var bundle profile.ProfileBundle
			var err error
			smallAllocs := testing.AllocsPerRun(5, func() { bundle, err = profileadapter.Normalize(small, admittedSource) })
			largeAllocs := testing.AllocsPerRun(5, func() { bundle, err = profileadapter.Normalize(large, admittedSource) })
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bundle.Profiles[0].Controls[0].Raw.Fields[field], large.Single.Inputs[0][field]) {
				t.Fatal("opaque source array was changed")
			}
			if largeAllocs > smallAllocs+16 {
				t.Fatalf("normalization allocations grew with array length: small %.0f, large %.0f", smallAllocs, largeAllocs)
			}
		})
	}
}

func TestMalformedDisplaySourceRejectedBeforeNormalize(t *testing.T) {
	for _, array := range []string{`[1,]`, `["unterminated]`, `[{]`, `[1] [2]`} {
		source := singleWithInputs(`{"types":` + array + `}`)
		document, err := profiledecode.DecodeText(source)
		if err == nil {
			_, err = profileraw.Parse(document)
		}
		if err == nil {
			t.Fatalf("malformed display source %q passed the validated-source boundary", array)
		}
	}
}
