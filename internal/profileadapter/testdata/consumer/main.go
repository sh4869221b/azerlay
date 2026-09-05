package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

const singleFixture = `{"id":"consumer","inputs":[{"types":["1","11","11"],"keyValues":["KeyU","0","0","0"],"metaValues":["0","0","0"],"isHold":false,"isTurbo":false,"isToggleOnHold":false,"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"]}]}`

var admitted = profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"}

type successOutput struct {
	Profiles int                   `json:"profiles"`
	Controls int                   `json:"controls"`
	Trigger  profile.TriggerKind   `json:"trigger"`
	Code     profile.CanonicalCode `json:"code"`
}

type errorOutput struct {
	Error profileadapter.ErrorCode `json:"error"`
	Zero  bool                     `json:"zero"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	mode := "success"
	if len(args) == 1 {
		mode = args[0]
	} else if len(args) != 0 {
		return fmt.Errorf("expected zero or one mode argument")
	}

	switch mode {
	case "success":
		bundle, err := decodeParseNormalize(singleFixture, admitted)
		if err != nil {
			return err
		}
		if len(bundle.Profiles) != 1 || len(bundle.Profiles[0].Controls) != 1 || len(bundle.Profiles[0].Controls[0].Bindings) != 3 {
			return fmt.Errorf("unexpected normalized shape")
		}
		binding := bundle.Profiles[0].Controls[0].Bindings[0]
		if binding.Trigger != profile.TriggerSingle || binding.Kind != profile.BindingKeyboard || len(binding.Actions) != 1 || binding.Actions[0].Kind != profile.ActionKeyboard || binding.Actions[0].Code != profile.KEY_U || len(binding.Actions[0].Modifiers) != 0 || binding.ReleaseBehavior == nil || *binding.ReleaseBehavior != "regular" || binding.Unknown != nil {
			return fmt.Errorf("unexpected normalized action")
		}
		return json.NewEncoder(os.Stdout).Encode(successOutput{Profiles: 1, Controls: 1, Trigger: binding.Trigger, Code: binding.Actions[0].Code})
	case "unsupported":
		bundle, err := decodeParseNormalize(singleFixture, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "unsupported"})
		return assertAndPrintError(bundle, err, profileadapter.ERR_IMPORT_UNSUPPORTED_VERSION)
	case "macro-overflow":
		step := `{"type":"Button","direction":"Full","duration":1,"keyCode":22}`
		fixture := `{"id":"consumer","inputs":[{"types":["16","11","11"],"macro":{"v":1,"repeat":false,"steps":[` + strings.Repeat(step+",", 1000) + step + `]}}]}`
		bundle, err := decodeParseNormalize(fixture, admitted)
		return assertAndPrintError(bundle, err, profileadapter.ERR_IMPORT_LIMIT_EXCEEDED)
	default:
		return fmt.Errorf("unknown mode %q", mode)
	}
}

func decodeParseNormalize(fixture string, source profile.SourceMetadata) (profile.ProfileBundle, error) {
	document, err := profiledecode.DecodeText(fixture)
	if err != nil {
		return profile.ProfileBundle{}, fmt.Errorf("DecodeText: %w", err)
	}
	raw, err := profileraw.Parse(document)
	if err != nil {
		return profile.ProfileBundle{}, fmt.Errorf("Parse: %w", err)
	}
	return profileadapter.Normalize(raw, source)
}

func assertAndPrintError(bundle profile.ProfileBundle, err error, expected profileadapter.ErrorCode) error {
	var normalizeError *profileadapter.NormalizeError
	zero := reflect.DeepEqual(bundle, profile.ProfileBundle{})
	if !errors.As(err, &normalizeError) || normalizeError.Code != expected || err.Error() != string(expected) || !zero {
		return fmt.Errorf("Normalize = %#v, %T %v; want %s and zero bundle", bundle, err, err, expected)
	}
	return json.NewEncoder(os.Stdout).Encode(errorOutput{Error: normalizeError.Code, Zero: true})
}
