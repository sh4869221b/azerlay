package profileadapter

import (
	"strconv"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

func decodeSourceIdentity(raw profileraw.RawInput) profile.SourceIdentity {
	id, invalidID := decodeIdentityInteger(raw, "id", true)
	pinOne, invalidPinOne := decodeIdentityInteger(raw, "pinOne", false)
	pinTwo, invalidPinTwo := decodeIdentityInteger(raw, "pinTwo", false)
	return profile.SourceIdentity{
		InputID: id, PinOne: pinOne, PinTwo: pinTwo,
		Invalid: invalidID || invalidPinOne || invalidPinTwo,
	}
}

func decodeIdentityInteger(raw profileraw.RawInput, field string, positive bool) (*int, bool) {
	token, present := raw[field]
	if !present {
		return nil, false
	}
	if len(token) == 0 || (token[0] != '-' && (token[0] < '0' || token[0] > '9')) {
		return nil, true
	}
	value, err := strconv.Atoi(string(token))
	if err != nil || (positive && value <= 0) || (!positive && value < 0) {
		return nil, true
	}
	return &value, false
}
