package profileraw

type rootFields uint8

const (
	profilesField rootFields = 1 << iota
	inputsField
	idField
	nameField
)

// preflightRootContext scopes modeled array limits from source key presence,
// independently of the provisional kind. Inspection neither adopts values nor
// reports errors: the bounded JSON preflight owns validation and error order.
func preflightRootContext(raw []byte) jsonContext {
	fields, complete := inspectRootFields(raw)
	if !complete {
		// Preserve early resource rejection when an oversized array has an
		// incomplete tail that prevents inspection of later root fields.
		return rootJSON
	}
	isBundle := fields&profilesField != 0
	isSingle := fields&inputsField != 0 && fields&(idField|nameField) != 0
	if isBundle && !isSingle {
		return bundleJSON
	}
	if isSingle && !isBundle {
		return profileJSON
	}
	return opaqueJSON
}

// inspectRootFields traverses only root keys. Values are skipped directly in
// the owned source bytes, so a rejected array doesn't allocate per member or
// get materialized merely to determine its root context.
func inspectRootFields(raw []byte) (rootFields, bool) {
	index := skipJSONSpace(raw, 0)
	if index == len(raw) || raw[index] != '{' {
		return 0, false
	}
	index++
	var fields rootFields
	for {
		index = skipJSONSpace(raw, index)
		if index == len(raw) {
			return fields, false
		}
		if raw[index] == '}' {
			return fields, true
		}
		if raw[index] != '"' {
			return fields, false
		}
		end, ok := skipJSONString(raw, index)
		if !ok {
			return fields, false
		}
		key := raw[index:end]
		switch {
		case isJSONKey(key, "profiles"):
			fields |= profilesField
		case isJSONKey(key, "inputs"):
			fields |= inputsField
		case isJSONKey(key, "id"):
			fields |= idField
		case isJSONKey(key, "name"):
			fields |= nameField
		}
		index = skipJSONSpace(raw, end)
		if index == len(raw) || raw[index] != ':' {
			return fields, false
		}
		index, ok = skipRootValue(raw, skipJSONSpace(raw, index+1))
		if !ok {
			return fields, false
		}
		index = skipJSONSpace(raw, index)
		if index == len(raw) {
			return fields, false
		}
		switch raw[index] {
		case '}':
			return fields, true
		case ',':
			index++
		default:
			return fields, false
		}
	}
}

func skipJSONSpace(raw []byte, index int) int {
	for index < len(raw) {
		switch raw[index] {
		case ' ', '\t', '\r', '\n':
			index++
		default:
			return index
		}
	}
	return index
}

func skipJSONString(raw []byte, index int) (int, bool) {
	for index++; index < len(raw); index++ {
		switch raw[index] {
		case '\\':
			index++
		case '"':
			return index + 1, true
		}
	}
	return len(raw), false
}

func skipRootValue(raw []byte, index int) (int, bool) {
	if index == len(raw) {
		return index, false
	}
	if raw[index] == '"' {
		return skipJSONString(raw, index)
	}
	if raw[index] != '{' && raw[index] != '[' {
		for index < len(raw) && raw[index] != ',' && raw[index] != '}' {
			index++
		}
		return index, index < len(raw)
	}
	depth := 0
	for index < len(raw) {
		switch raw[index] {
		case '"':
			var ok bool
			index, ok = skipJSONString(raw, index)
			if !ok {
				return index, false
			}
			continue
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return index + 1, true
			}
		}
		index++
	}
	return index, false
}

// The modeled keys contain only ASCII letters. Their only alternate valid
// JSON spelling is a mixture of literal letters and \uXXXX escapes, which can
// be compared without decoding or allocating storage for arbitrary root keys.
func isJSONKey(raw []byte, key string) bool {
	if len(raw) < 2 || len(raw) > 6*len(key)+2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return false
	}
	index := 1
	for letter := range len(key) {
		if index >= len(raw)-1 {
			return false
		}
		value := int(raw[index])
		index++
		if value == '\\' {
			if index+5 > len(raw)-1 || raw[index] != 'u' {
				return false
			}
			index++
			value = 0
			for range 4 {
				digit := hexDigit(raw[index])
				if digit < 0 {
					return false
				}
				value = value*16 + digit
				index++
			}
		}
		if value != int(key[letter]) {
			return false
		}
	}
	return index == len(raw)-1
}

func hexDigit(value byte) int {
	switch {
	case value >= '0' && value <= '9':
		return int(value - '0')
	case value >= 'a' && value <= 'f':
		return int(value-'a') + 10
	case value >= 'A' && value <= 'F':
		return int(value-'A') + 10
	default:
		return -1
	}
}
