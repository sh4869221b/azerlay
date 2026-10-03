package config

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Load reads a complete validated candidate without modifying the source file.
func Load(explicit string) (Config, []Warning, error) {
	path, err := ResolvePath(explicit)
	if err != nil {
		return Config{}, nil, err
	}
	return loadFile(path)
}

func loadFile(path string) (Config, []Warning, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		code := ERR_CONFIG_INVALID
		if errors.Is(err, os.ErrNotExist) {
			code = ERR_CONFIG_NOT_FOUND
		}
		return Config{}, nil, &Error{Code: code, Stage: "read", Reason: "configuration read failed", Cause: err}
	}
	candidate := defaults()
	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	err = decoder.Decode(&candidate)
	var missing *toml.StrictMissingError
	if err != nil && !errors.As(err, &missing) {
		return Config{}, nil, &Error{Code: ERR_CONFIG_INVALID, Stage: "decode", Reason: "invalid TOML or field type", Cause: err}
	}
	if candidate.SchemaVersion != 1 {
		return Config{}, nil, &Error{Code: ERR_CONFIG_INVALID, Stage: "schema", Reason: "schema_version must be 1"}
	}
	if err := validate(candidate); err != nil {
		return Config{}, nil, err
	}
	var warnings []Warning
	if missing != nil {
		for _, field := range missing.Errors {
			line, column := field.Position()
			warnings = append(warnings, Warning{Code: WARN_CONFIG_UNKNOWN_KEY, Line: line, Column: column, Key: slices.Clone(field.Key())})
		}
	}
	return candidate, warnings, nil
}

func validate(c Config) error {
	local := c.Profile.LocalDevice != "" || c.Profile.LocalProfileFile != ""
	if local && (!validLocalComponent(c.Profile.LocalDevice) || !validLocalComponent(c.Profile.LocalProfileFile)) {
		return &Error{Code: ERR_CONFIG_INVALID, Stage: "validation", Reason: "profile local device and file must be complete single path components"}
	}
	if c.Profile.SelectedID != "" && (local || c.Profile.Source == "local") || local && c.Profile.Source == "imported" {
		return &Error{Code: ERR_CONFIG_INVALID, Stage: "validation", Reason: "profile selectors conflict with the source policy"}
	}
	for _, field := range []struct {
		name, value string
		allowed     []string
	}{
		{"device.model", c.Device.Model, []string{"cyborg-ii"}},
		{"device.hand", c.Device.Hand, []string{"left", "right"}},
		{"input.source", c.Input.Source, []string{"auto", "evdev"}},
		{"profile.source", c.Profile.Source, []string{"auto", "imported", "local"}},
		{"overlay.anchor", c.Overlay.Anchor, []string{"top-left", "top", "top-right", "left", "center", "right", "bottom-left", "bottom", "bottom-right"}},
		{"overlay.mode", c.Overlay.Mode, []string{"compact", "normal", "detailed"}},
		{"appearance.theme", c.Appearance.Theme, []string{"dark", "light"}},
		{"diagnostics.log_level", c.Diagnostics.LogLevel, []string{"error", "warn", "info", "debug", "trace"}},
	} {
		if !slices.Contains(field.allowed, field.value) {
			return &Error{Code: ERR_CONFIG_INVALID, Stage: "validation", Reason: field.name + " has an unsupported value"}
		}
	}
	if !slices.Contains([]int{30, 60, 120}, c.Input.RefreshHz) {
		return &Error{Code: ERR_CONFIG_INVALID, Stage: "validation", Reason: "input.refresh_hz must be 30, 60 or 120"}
	}
	for _, field := range []struct {
		name  string
		value int
	}{{"overlay.margin_x", c.Overlay.MarginX}, {"overlay.margin_y", c.Overlay.MarginY}} {
		if field.value < -16384 || field.value > 16384 {
			return &Error{Code: ERR_CONFIG_INVALID, Stage: "validation", Reason: field.name + " is outside its range"}
		}
	}
	for _, field := range []struct {
		name                    string
		value, minimum, maximum float64
	}{
		{"overlay.scale", c.Overlay.Scale, .25, 4},
		{"overlay.opacity", c.Overlay.Opacity, 0, 1},
		{"appearance.font_scale", c.Appearance.FontScale, .25, 4},
	} {
		if math.IsNaN(field.value) || math.IsInf(field.value, 0) || field.value < field.minimum || field.value > field.maximum {
			return &Error{Code: ERR_CONFIG_INVALID, Stage: "validation", Reason: field.name + " must be finite and within its range"}
		}
	}
	return nil
}

func validLocalComponent(value string) bool {
	return value != "" && value != "." && value != ".." && !filepath.IsAbs(value) && !strings.ContainsAny(value, "/\\\x00")
}
