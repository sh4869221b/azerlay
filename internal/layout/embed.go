package layout

import _ "embed"

//go:embed assets/cyborg-ii-left.json
var cyborgIILeft []byte

func LoadEmbedded(model, hand string) (Definition, error) {
	if model != "cyborg-ii" {
		return Definition{}, &Error{Code: ERR_LAYOUT_UNSUPPORTED, Path: "model"}
	}
	if hand != "left" {
		return Definition{}, &Error{Code: ERR_LAYOUT_UNSUPPORTED, Path: "hand"}
	}
	return Parse(cyborgIILeft)
}
