package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestConfigUnknownKeys(t *testing.T) {
	t.Parallel()
	text := "schema_version=1\nfuture='value'\n[overlay]\nextra=true\nopacity=0.5\n[future_table]\nkey='value'\n"
	path := configFile(t, text)
	got, warnings, err := Load(path)
	if err != nil || got.Overlay.Opacity != .5 || len(warnings) != 3 {
		t.Fatalf("Load() = %v, %v, %v", got, warnings, err)
	}
	for i, want := range []struct {
		key  []string
		line int
	}{{[]string{"future"}, 2}, {[]string{"overlay", "extra"}, 4}, {[]string{"future_table"}, 6}} {
		if warnings[i].Code != WARN_CONFIG_UNKNOWN_KEY || !reflect.DeepEqual(warnings[i].Key, want.key) || warnings[i].Line != want.line || warnings[i].Column < 1 {
			t.Fatalf("warning %d = %#v", i, warnings[i])
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != text {
		t.Fatalf("source changed: %v", err)
	}
	for _, text := range []string{
		"schema_version=1\nfuture=true\n[overlay]\nopacity=2.0",
		"schema_version=1\nfuture=true\n[input]\nrefresh_hz='bad'",
		"future=true\n[overlay]\nopacity=0.5",
	} {
		got, _, err := Load(configFile(t, text))
		if err == nil || got != (Config{}) {
			t.Fatalf("unknown key hid invalid field: %v, %v", got, err)
		}
	}
}

func TestConfigDiagnostics(t *testing.T) {
	t.Parallel()
	const secret = "fixture-private-value"
	for _, text := range []string{
		"schema_version=1\n[device]\nmodel='" + secret + "'",
		"schema_version=1\n[input]\nrefresh_hz='" + secret + "'",
		"schema_version=1\n'" + secret + "'='" + secret + "'\n[overlay]\nopacity=",
	} {
		_, _, err := Load(configFile(t, text))
		var configErr *Error
		if !errors.As(err, &configErr) {
			t.Fatalf("expected safe error, got %v", err)
		}
		for _, display := range []string{err.Error(), fmt.Sprint(err), fmt.Sprintf("%+v", err)} {
			if strings.Contains(display, secret) || strings.Contains(display, "schema_version") {
				t.Fatalf("diagnostic leaked source: %s", display)
			}
		}
	}
	_, warnings, err := Load(configFile(t, "schema_version=1\n'"+secret+"'='"+secret+"'"))
	if err != nil || len(warnings) != 1 {
		t.Fatalf("warning Load() = %v, %v", warnings, err)
	}
	if warnings[0].Key[0] != secret {
		t.Fatal("structured key lost")
	}
	for _, display := range []string{fmt.Sprint(warnings[0]), fmt.Sprintf("%+v", warnings[0])} {
		if strings.Contains(display, secret) {
			t.Fatalf("warning leaked: %s", display)
		}
	}
}
