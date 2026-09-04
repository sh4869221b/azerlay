package profileraw

import "encoding/json"

// RawExport is a structurally classified profile export.
type RawExport struct {
	Bundle *RawBundle
	Single *RawProfile
}

// RawBundle preserves a bundle's modeled and unknown fields.
type RawBundle struct {
	Version  *RawScalar
	Profiles []RawProfile
	Unknown  map[string]json.RawMessage
}

// RawProfile preserves a profile's modeled and unknown fields.
type RawProfile struct {
	ID      *RawScalar
	Name    *RawScalar
	Version *RawScalar
	Inputs  []RawInput
	Unknown map[string]json.RawMessage
}

// RawInput preserves every input field as an opaque JSON value.
type RawInput map[string]json.RawMessage

// RawScalarKind identifies a JSON scalar representation.
type RawScalarKind uint8

const (
	ScalarNull RawScalarKind = iota
	ScalarBool
	ScalarString
	ScalarNumber
)

// RawScalar preserves both a JSON scalar token and its typed value.
type RawScalar struct {
	Kind   RawScalarKind
	Raw    json.RawMessage
	Bool   bool
	String string
	Number json.Number
}
