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
	var tokens []string
	for _, field := range fields {
		var values []json.RawMessage
		if json.Unmarshal(input[field], &values) != nil {
			continue
		}
		for i, value := range values {
			if field == "types" && index >= 0 && i != index {
				continue
			}
			if token := displayScalar(value); token != "" {
				tokens = append(tokens, field+"["+strconv.Itoa(i)+"]="+token)
			}
		}
	}
	var result strings.Builder
	for _, token := range tokens {
		separator := ""
		if result.Len() > 0 {
			separator = ", "
		}
		if result.Len()+len(separator)+len(token) > 253 {
			result.WriteString("...")
			break
		}
		result.WriteString(separator)
		result.WriteString(token)
	}
	return result.String()
}

func displayScalar(raw json.RawMessage) string {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return ""
	}
	switch value := value.(type) {
	case string:
		if value == "0" {
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
	case json.Number:
		if number, err := value.Float64(); err == nil && number == 0 {
			return ""
		}
		if len(value) > 80 {
			return string(value[:80]) + "..."
		}
		return string(value)
	default:
		return ""
	}
}
