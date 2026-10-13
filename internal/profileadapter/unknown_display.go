package profileadapter

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func unknownDisplay(input profileraw.RawInput, trigger profile.TriggerKind) string {
	fields := []string{"types"}
	index := -1
	switch trigger {
	case profile.TriggerSingle:
		index = 0
		fields = append(fields, "keyValues", "metaValues")
	case profile.TriggerLong:
		index = 1
		fields = append(fields, "keyValuesLong", "metaValuesLong")
	case profile.TriggerDouble:
		index = 2
		fields = append(fields, "keyValuesDouble", "metaValuesDouble")
	}
	var result strings.Builder
	for _, field := range fields {
		values := newRawArray(input[field])
		for i := 0; ; i++ {
			value, ok := values.next()
			if !ok {
				break
			}
			if field == "types" && index >= 0 && i != index {
				continue
			}
			if token := displayScalar(value); token != "" {
				token = field + "[" + strconv.Itoa(i) + "]=" + token
				separator := ""
				if result.Len() > 0 {
					separator = ", "
				}
				if result.Len()+len(separator)+len(token) > 253 {
					result.WriteString("...")
					return result.String()
				}
				result.WriteString(separator)
				result.WriteString(token)
			}
			if field == "types" && index >= 0 {
				break
			}
		}
	}
	return result.String()
}

func displayScalar(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || (raw[0] != '"' && !isJSONNumber(raw)) {
		return ""
	}
	if raw[0] == '"' {
		// These are the only valid JSON spellings of the skipped string "0".
		if bytes.Equal(raw, []byte(`"0"`)) || bytes.Equal(raw, []byte(`"\u0030"`)) {
			return ""
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return ""
		}
		var quoted strings.Builder
		for len(value) > 0 {
			r, size := utf8.DecodeRuneInString(value)
			part := strconv.QuoteToASCII(string(r))
			if quoted.Len()+len(part)-2 > 80 {
				quoted.WriteString("...")
				break
			}
			quoted.WriteString(part[1 : len(part)-1])
			value = value[size:]
		}
		return `"` + quoted.String() + `"`
	}
	// Literal zero mantissas are common inactive-slot values. Checking them in
	// place also handles arbitrarily long zero spellings without scratch copies.
	zero := true
	for _, b := range raw {
		if b == 'e' || b == 'E' {
			break
		}
		if b >= '1' && b <= '9' {
			zero = false
			break
		}
	}
	if zero {
		return ""
	}
	if number, err := strconv.ParseFloat(string(raw), 64); err == nil && number == 0 {
		return ""
	}
	if len(raw) > 80 {
		return string(raw[:80]) + "..."
	}
	return string(raw)
}
