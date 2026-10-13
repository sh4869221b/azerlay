package profileadapter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRawArrayPrefixConsumption(t *testing.T) {
	const prefix = `["1", "11", "11", `
	for _, count := range []int{32, 4096} {
		raw := json.RawMessage(prefix + strings.Repeat(`{"nested":["opaque"]},`, count) + `null]`)
		values := newRawArray(raw)
		for _, wanted := range []string{`"1"`, `"11"`, `"11"`} {
			value, ok := values.next()
			if !ok || !bytes.Equal(value, []byte(wanted)) {
				t.Fatalf("value = %q, %t; want %q", value, ok, wanted)
			}
		}
		if values.ended() || len(raw)-len(values.rest) != len(prefix) {
			t.Fatalf("iterator consumed its opaque tail: consumed %d bytes; want %d", len(raw)-len(values.rest), len(prefix))
		}
		if matchesRawStringArray(raw, []string{"1", "11", "11"}) {
			t.Fatal("extra opaque members matched an exact three-element predicate")
		}
	}
}

func TestRawArrayValues(t *testing.T) {
	raw := json.RawMessage(` ["a\"b", {"nested":[[],{"x":"[,]}"}]}, -1e2, null] `)
	values := newRawArray(raw)
	for _, wanted := range []string{`"a\"b"`, `{"nested":[[],{"x":"[,]}"}]}`, `-1e2`, `null`} {
		value, ok := values.next()
		if !ok || !bytes.Equal(value, []byte(wanted)) {
			t.Fatalf("value = %q, %t; want %q", value, ok, wanted)
		}
	}
	if !values.ended() {
		t.Fatal("valid array did not end")
	}
	for _, malformed := range []string{`null`, `[`, `[1,]`, `["unterminated]`, `[[`, `[1] trailing`} {
		values := newRawArray(json.RawMessage(malformed))
		for {
			if _, ok := values.next(); !ok {
				break
			}
		}
		if values.ended() {
			t.Fatalf("malformed array %q reached a complete array end", malformed)
		}
	}
}
