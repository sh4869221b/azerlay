package profileraw

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"
)

// UnmarshalJSON parses one lossless JSON scalar.
func (scalar *RawScalar) UnmarshalJSON(data []byte) error {
	*scalar = RawScalar{}
	token := bytes.TrimSpace(data)
	if len(token) == 0 || !utf8.Valid(token) {
		return &ParseError{Code: ERR_IMPORT_ROOT}
	}

	var decoded RawScalar
	switch token[0] {
	case 'n':
		if !bytes.Equal(token, []byte("null")) {
			return &ParseError{Code: ERR_IMPORT_ROOT}
		}
		decoded.Kind = ScalarNull
	case 't', 'f':
		if err := json.Unmarshal(token, &decoded.Bool); err != nil {
			return &ParseError{Code: ERR_IMPORT_ROOT}
		}
		decoded.Kind = ScalarBool
	case '"':
		if err := json.Unmarshal(token, &decoded.String); err != nil {
			return &ParseError{Code: ERR_IMPORT_ROOT}
		}
		decoded.Kind = ScalarString
	default:
		if err := json.Unmarshal(token, &decoded.Number); err != nil {
			return &ParseError{Code: ERR_IMPORT_ROOT}
		}
		decoded.Kind = ScalarNumber
	}
	decoded.Raw = append(json.RawMessage(nil), token...)
	*scalar = decoded
	return nil
}
