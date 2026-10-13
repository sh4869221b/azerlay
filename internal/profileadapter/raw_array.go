package profileadapter

import (
	"bytes"
	"encoding/json"
)

// rawArray visits values without copying or decoding their contents. Its input
// comes from profileraw.Parse, which has already validated the complete JSON.
type rawArray struct {
	rest []byte
	ok   bool
}

func newRawArray(raw json.RawMessage) rawArray {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return rawArray{}
	}
	return rawArray{rest: bytes.TrimSpace(raw[1:]), ok: true}
}

func (a *rawArray) next() (json.RawMessage, bool) {
	if !a.ok || len(a.rest) == 0 || a.rest[0] == ']' {
		return nil, false
	}
	value, rest := takeRawValue(a.rest)
	if value == nil {
		a.ok = false
		return nil, false
	}
	rest = bytes.TrimSpace(rest)
	if len(rest) == 0 {
		a.ok = false
		return nil, false
	}
	switch rest[0] {
	case ',':
		a.rest = bytes.TrimSpace(rest[1:])
		if len(a.rest) == 0 || a.rest[0] == ']' {
			a.ok = false
		}
	case ']':
		a.rest = rest
	default:
		a.ok = false
	}
	return value, a.ok
}

func (a *rawArray) ended() bool {
	return a.ok && len(a.rest) > 0 && a.rest[0] == ']' && len(bytes.TrimSpace(a.rest[1:])) == 0
}

// takeRawValue returns a view of one validated JSON value, including nested
// containers. Strings and containers are skipped without allocating their data.
func takeRawValue(raw []byte) (json.RawMessage, []byte) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, nil
	}
	depth := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '"':
			end := i + 1
			for ; end < len(raw) && raw[end] != '"'; end++ {
				if raw[end] == '\\' {
					end++
				}
			}
			if end >= len(raw) {
				return nil, nil
			}
			i = end
			if depth == 0 {
				return raw[:i+1], raw[i+1:]
			}
		case '[', '{':
			depth++
		case ']', '}':
			if depth == 0 {
				return raw[:i], raw[i:]
			}
			depth--
			if depth == 0 {
				return raw[:i+1], raw[i+1:]
			}
		case ',', ':', ' ', '\t', '\r', '\n':
			if depth == 0 {
				return raw[:i], raw[i:]
			}
		}
	}
	if depth != 0 {
		return nil, nil
	}
	return raw, nil
}
