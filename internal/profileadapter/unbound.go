package profileadapter

import (
	"bytes"
	"encoding/json"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

const emptySettingJSON = `{
  "types": ["11", "11", "11"],
  "keyValues": ["0", "0", "0", "0"],
  "metaValues": ["0", "0", "0"],
  "keyValuesLong": ["0", "0", "0", "0"],
  "metaValuesLong": ["0", "0", "0"],
  "keyValuesDouble": ["0", "0", "0", "0"],
  "metaValuesDouble": ["0", "0", "0"],
  "layeringProfileId": "", "isBelkin": false, "isToggleOnHold": false,
  "layeringProfileIdLong": "", "isBelkinLong": false, "isToggleOnHoldLong": false,
  "layeringProfileIdDouble": "", "isBelkinDouble": false, "isToggleOnHoldDouble": false,
  "macro": {"repeat": false, "steps": [], "v": 1},
  "longMacro": {"repeat": false, "steps": [], "v": 1},
  "doubleMacro": {"repeat": false, "steps": [], "v": 1},
  "featureDelay": 500, "doubleDelay": 150, "subType": "11",
  "x": 0, "y": 0, "interval": 0, "yInterval": 0,
  "xLong": 0, "yLong": 0, "xDouble": 0, "yDouble": 0,
  "isHold": false, "isHoldLong": false, "isHoldDouble": false,
  "holdTime": 0, "holdTimeLong": 0, "holdTimeDouble": 0,
  "isTurbo": false, "isTurboLong": false, "isTurboDouble": false,
  "turboInterval": 0, "turboIntervalLong": 0, "turboIntervalDouble": 0,
  "sequenceTriggerSettings": {"isPingPongLoop": false, "sequenceSteps": []},
  "analogSettings": {
    "angle": 0, "lowerLimit": 0, "upperLimit": 0, "sensitivity": 0,
    "analogKeys": {
      "left": {"up": [87,0,0], "right": [68,0,0], "down": [83,0,0], "left": [65,0,0]},
      "right": {"up": ["ArrowUp",0,0], "right": ["ArrowRight",0,0], "down": ["ArrowDown",0,0], "left": ["ArrowLeft",0,0]}
    },
    "diagonalKeys": {
      "left": {"up_right": ["Digit1",0,0], "up_left": ["Digit2",0,0], "down_left": ["Digit3",0,0], "down_right": ["Digit4",0,0]},
      "right": {"up_right": ["Digit1",0,0], "up_left": ["Digit2",0,0], "down_left": ["Digit3",0,0], "down_right": ["Digit4",0,0]}
    },
    "analogCones": {"verticalCone": 45, "horizontalCone": 45},
    "isEightDirectionalTrigger": false, "mouseSensitivity": 5, "analogThrottle": 0,
    "isAnalogSmoothing": false, "triggerMagnitude": 4, "isCombinedAnalog": false,
    "combinedAnalogMagnitude": 6, "isHoldTrigger": false, "holdMagnitude": 9,
    "holdType": "1", "holdKeyValues": ["16","0","0"], "rotateStickButtonId": 0,
    "isAngleLock": false, "lockZoneAngle": 70, "lockZoneSize": 30,
    "isRightAnalog": false, "invertXAxis": false, "invertYAxis": false
  },
  "scrollSpeed": 1, "scrollThreshold": 10, "isSmoothScroll": false
}`

var emptySetting = decodeSemanticJSON(json.RawMessage(emptySettingJSON))

func normalizeUnbound(raw profileraw.RawInput, bindings []profile.TriggerBinding) {
	if len(bindings) != 3 || bindings[0].Trigger != profile.TriggerSingle {
		return
	}
	semanticCount := 0
	expected := emptySetting.(map[string]any)
	for field, value := range raw {
		switch field {
		case "id", "pinOne", "pinTwo", "label":
			continue
		}
		wanted, present := expected[field]
		if !present || !matchesEmptySettingValue(value, wanted) {
			return
		}
		semanticCount++
	}
	if semanticCount == len(expected) {
		bindings[0] = profile.TriggerBinding{Trigger: profile.TriggerSingle, Kind: profile.BindingUnbound}
	}
}

// Compare only the fixed empty-setting shape. An opaque extra array or object
// member never becomes an allocated semantic value merely to reject it.
func matchesEmptySettingValue(raw json.RawMessage, expected any) bool {
	raw = bytes.TrimSpace(raw)
	switch expected := expected.(type) {
	case string:
		return matchesJSONString(raw, expected)
	case json.Number:
		return bytes.Equal(raw, []byte(expected))
	case bool:
		if expected {
			return bytes.Equal(raw, []byte("true"))
		}
		return bytes.Equal(raw, []byte("false"))
	case []any:
		values := newRawArray(raw)
		for _, wanted := range expected {
			value, ok := values.next()
			if !ok || !matchesEmptySettingValue(value, wanted) {
				return false
			}
		}
		return values.ended()
	case map[string]any:
		if len(raw) == 0 || raw[0] != '{' {
			return false
		}
		maxKeyLength := 0
		for key := range expected {
			if len(key) > maxKeyLength {
				maxKeyLength = len(key)
			}
		}
		rest := bytes.TrimSpace(raw[1:])
		count := 0
		for len(rest) > 0 && rest[0] != '}' {
			keyToken, next := takeRawValue(rest)
			if len(keyToken) < 2 || keyToken[0] != '"' || len(keyToken) > 6*maxKeyLength+2 {
				return false
			}
			var key string
			if json.Unmarshal(keyToken, &key) != nil {
				return false
			}
			wanted, present := expected[key]
			next = bytes.TrimSpace(next)
			if !present || len(next) == 0 || next[0] != ':' {
				return false
			}
			value, next := takeRawValue(next[1:])
			if value == nil || !matchesEmptySettingValue(value, wanted) {
				return false
			}
			count++
			rest = bytes.TrimSpace(next)
			if len(rest) > 0 && rest[0] == ',' {
				rest = bytes.TrimSpace(rest[1:])
				continue
			}
			break
		}
		return count == len(expected) && len(rest) > 0 && rest[0] == '}' && len(bytes.TrimSpace(rest[1:])) == 0
	default:
		return false
	}
}

func decodeSemanticJSON(raw json.RawMessage) any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil
	}
	return value
}
