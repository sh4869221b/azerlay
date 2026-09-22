package layout

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseLayout(t *testing.T) {
	t.Parallel()

	valid, err := os.ReadFile("testdata/valid-minimal.json")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid minimal layout retains source correspondence", func(t *testing.T) {
		input := append([]byte(nil), valid...)
		got, err := Parse(input)
		if err != nil {
			t.Fatal(err)
		}
		for i := range input {
			input[i] = 0
		}
		if got.SchemaVersion != 1 || got.Model != "cyborg-ii" || got.Hand != "left" {
			t.Fatalf("root fields: %+v", got)
		}
		if got.ViewBox != (Rect{X: 0, Y: 0, Width: 694, Height: 640}) {
			t.Fatalf("view box: %+v", got.ViewBox)
		}
		if got.Applicability.SoftwareRelease != "2.0.2" || got.Applicability.DisplayedFirmware != "111" || got.Applicability.HardwareRevision != nil || got.Applicability.Mode != "keyboard-stick" {
			t.Fatalf("applicability: %+v", got.Applicability)
		}
		if len(got.Controls) != 1 {
			t.Fatalf("controls: %d", len(got.Controls))
		}
		control := got.Controls[0]
		if control.ID != "grid.c1.r1" || control.Group != "grid" || control.SourceInputID != 1 || control.PinOne != 255 || control.PinTwo != 4 || control.ZIndex != 0 {
			t.Fatalf("control fields: %+v", control)
		}
		if control.Shape.Type != "rounded_rect" || control.LabelAnchor != (Point{X: 45, Y: 70}) {
			t.Fatalf("control shape/anchor: %+v", control)
		}
	})

	tests := []struct {
		name string
		data []byte
		code ErrorCode
		path string
	}{
		{"malformed JSON", []byte(`{"secret":"private"`), ERR_LAYOUT_JSON, "$"},
		{"trailing document", append(append([]byte(nil), valid...), []byte(` {"secret":"private"}`)...), ERR_LAYOUT_JSON, "$"},
		{"unknown root field", []byte(strings.Replace(string(valid), `"schema_version": 1,`, `"schema_version": 1, "secret_private_field": true,`, 1)), ERR_LAYOUT_SCHEMA, "$"},
		{"missing required field", []byte(strings.Replace(string(valid), `"schema_version": 1,`, ``, 1)), ERR_LAYOUT_SCHEMA, "schema_version"},
		{"missing nested field", []byte(strings.Replace(string(valid), `"pin_one": 255,`, ``, 1)), ERR_LAYOUT_SCHEMA, "controls[0].pin_one"},
		{"missing shape payload", []byte(strings.Replace(string(valid), `, "radius": 6`, ``, 1)), ERR_LAYOUT_SCHEMA, "controls[0].shape.radius"},
		{"shape field from another variant", []byte(strings.Replace(string(valid), `"radius": 6`, `"radius": 6, "cx": 45`, 1)), ERR_LAYOUT_SCHEMA, "controls[0].shape"},
		{"unsupported schema version", []byte(strings.Replace(string(valid), `"schema_version": 1,`, `"schema_version": 2,`, 1)), ERR_LAYOUT_SCHEMA, "schema_version"},
		{"unsupported model", []byte(strings.Replace(string(valid), `"model": "cyborg-ii"`, `"model": "private-model"`, 1)), ERR_LAYOUT_UNSUPPORTED, "model"},
		{"invalid hand spelling", []byte(strings.Replace(string(valid), `"hand": "left"`, `"hand": "private-hand"`, 1)), ERR_LAYOUT_INVALID, "hand"},
		{"right hand data parses", []byte(strings.Replace(string(valid), `"hand": "left"`, `"hand": "right"`, 1)), "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.data)
			if tt.code == "" {
				if err != nil || got.Hand != "right" {
					t.Fatalf("right hand data: got hand %q, err %v", got.Hand, err)
				}
				return
			}
			var layoutErr *Error
			if !errors.As(err, &layoutErr) {
				t.Fatalf("want layout error, got %v", err)
			}
			if layoutErr.Code != tt.code || layoutErr.Path != tt.path {
				t.Fatalf("want %s at %s, got %s at %s", tt.code, tt.path, layoutErr.Code, layoutErr.Path)
			}
			if strings.Contains(layoutErr.Error(), "private") || strings.Contains(layoutErr.Error(), "secret") {
				t.Fatalf("error leaked source text: %s", layoutErr.Error())
			}
		})
	}
}
