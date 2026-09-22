package layout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func Parse(data []byte) (Definition, error) {
	var raw json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return Definition{}, &Error{Code: ERR_LAYOUT_JSON, Path: "$"}
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return Definition{}, &Error{Code: ERR_LAYOUT_JSON, Path: "$"}
	}
	if err := checkDefinition(raw); err != nil {
		return Definition{}, err
	}
	var definition Definition
	if err := json.Unmarshal(raw, &definition); err != nil {
		return Definition{}, &Error{Code: ERR_LAYOUT_SCHEMA, Path: "$"}
	}
	if definition.SchemaVersion != 1 {
		return Definition{}, &Error{Code: ERR_LAYOUT_SCHEMA, Path: "schema_version"}
	}
	if definition.Model != "cyborg-ii" {
		return Definition{}, &Error{Code: ERR_LAYOUT_UNSUPPORTED, Path: "model"}
	}
	if definition.Hand != "left" && definition.Hand != "right" {
		return Definition{}, &Error{Code: ERR_LAYOUT_INVALID, Path: "hand"}
	}
	if err := validateDefinition(definition); err != nil {
		return Definition{}, err
	}
	return definition, nil
}

func checkDefinition(raw json.RawMessage) error {
	fields, err := checkObject(raw, "$", []string{"schema_version", "model", "hand", "applicability", "view_box", "controls", "decorations"}, []string{"schema_version", "model", "hand", "applicability", "view_box", "controls"})
	if err != nil {
		return err
	}
	if _, err := checkObject(fields["applicability"], "applicability", []string{"software_release", "displayed_firmware", "hardware_revision", "mode"}, []string{"software_release", "displayed_firmware", "hardware_revision", "mode"}); err != nil {
		return err
	}
	if _, err := checkObject(fields["view_box"], "view_box", []string{"x", "y", "width", "height"}, []string{"x", "y", "width", "height"}); err != nil {
		return err
	}
	if err := checkArray(fields["controls"], "controls", checkControl); err != nil {
		return err
	}
	if decorations, ok := fields["decorations"]; ok {
		return checkArray(decorations, "decorations", checkShape)
	}
	return nil
}

func checkControl(raw json.RawMessage, path string) error {
	fields, err := checkObject(raw, path, []string{"id", "group", "source_input_id", "pin_one", "pin_two", "z_index", "shape", "label_anchor"}, []string{"id", "group", "source_input_id", "pin_one", "pin_two", "z_index", "shape", "label_anchor"})
	if err != nil {
		return err
	}
	if err := checkShape(fields["shape"], path+".shape"); err != nil {
		return err
	}
	return checkPoint(fields["label_anchor"], path+".label_anchor")
}

func checkShape(raw json.RawMessage, path string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return &Error{Code: ERR_LAYOUT_SCHEMA, Path: path}
	}
	shapeType, ok := fields["type"]
	if !ok || string(shapeType) == "null" {
		return &Error{Code: ERR_LAYOUT_SCHEMA, Path: path + ".type"}
	}
	var kind string
	if err := json.Unmarshal(shapeType, &kind); err != nil {
		return &Error{Code: ERR_LAYOUT_SCHEMA, Path: path + ".type"}
	}
	var payload []string
	switch kind {
	case "rounded_rect":
		payload = []string{"x", "y", "width", "height", "radius"}
	case "polygon":
		payload = []string{"points"}
	case "circle":
		payload = []string{"cx", "cy", "radius"}
	case "ellipse":
		payload = []string{"cx", "cy", "rx", "ry"}
	case "line":
		payload = []string{"from", "to"}
	case "path":
		payload = []string{"commands"}
	case "group":
		payload = []string{"transform", "children"}
	default:
		return &Error{Code: ERR_LAYOUT_INVALID, Path: path + ".type"}
	}
	fields, err := checkObject(raw, path, append([]string{"type"}, payload...), append([]string{"type"}, payload...))
	if err != nil {
		return err
	}
	if points, ok := fields["points"]; ok {
		if err := checkArray(points, path+".points", checkPoint); err != nil {
			return err
		}
	}
	for _, name := range []string{"from", "to"} {
		if point, ok := fields[name]; ok {
			if err := checkPoint(point, path+"."+name); err != nil {
				return err
			}
		}
	}
	if commands, ok := fields["commands"]; ok {
		if err := checkArray(commands, path+".commands", checkCommand); err != nil {
			return err
		}
	}
	if transform, ok := fields["transform"]; ok {
		if _, err := checkObject(transform, path+".transform", []string{"translate_x", "translate_y", "scale_x", "scale_y"}, []string{"translate_x", "translate_y", "scale_x", "scale_y"}); err != nil {
			return err
		}
	}
	if children, ok := fields["children"]; ok {
		return checkArray(children, path+".children", checkShape)
	}
	return nil
}

func checkCommand(raw json.RawMessage, path string) error {
	fields, err := checkObject(raw, path, []string{"op", "points"}, []string{"op", "points"})
	if err != nil {
		return err
	}
	return checkArray(fields["points"], path+".points", checkPoint)
}

func checkPoint(raw json.RawMessage, path string) error {
	_, err := checkObject(raw, path, []string{"x", "y"}, []string{"x", "y"})
	return err
}

func checkArray(raw json.RawMessage, path string, check func(json.RawMessage, string) error) error {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || items == nil {
		return &Error{Code: ERR_LAYOUT_SCHEMA, Path: path}
	}
	for i, item := range items {
		if err := check(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func checkObject(raw json.RawMessage, path string, allowed, required []string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, &Error{Code: ERR_LAYOUT_SCHEMA, Path: path}
	}
	for _, name := range required {
		value, ok := fields[name]
		if !ok || (string(value) == "null" && !(path == "applicability" && name == "hardware_revision")) {
			return nil, &Error{Code: ERR_LAYOUT_SCHEMA, Path: fieldPath(path, name)}
		}
	}
	known := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		known[name] = true
	}
	for name := range fields {
		if !known[name] {
			return nil, &Error{Code: ERR_LAYOUT_SCHEMA, Path: path}
		}
	}
	return fields, nil
}

func fieldPath(path, name string) string {
	if path == "$" {
		return name
	}
	return path + "." + name
}
