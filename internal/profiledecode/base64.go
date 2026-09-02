package profiledecode

import (
	"encoding/base64"
	"strings"
)

type outerForm uint8

const (
	outerRawJSON outerForm = iota
	outerBase64URLPadded
	outerBase64URLUnpadded
	outerBase64StandardPadded
	outerBase64StandardUnpadded
)

func detectOuter(text string) (outerForm, error) {
	if text == "" {
		return outerRawJSON, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}
	if text[0] == '{' || text[0] == '[' {
		return outerRawJSON, nil
	}

	standardAlphabet := false
	urlAlphabet := false
	paddingAt := -1
	for index := 0; index < len(text); index++ {
		character := text[index]
		if paddingAt >= 0 && character != '=' {
			return outerRawJSON, newDecodeError(ERR_IMPORT_ENCODING, nil)
		}
		switch {
		case character >= 'A' && character <= 'Z':
		case character >= 'a' && character <= 'z':
		case character >= '0' && character <= '9':
		case character == '+' || character == '/':
			standardAlphabet = true
		case character == '-' || character == '_':
			urlAlphabet = true
		case character == '=':
			if paddingAt < 0 {
				paddingAt = index
			}
		default:
			return outerRawJSON, newDecodeError(ERR_IMPORT_ENCODING, nil)
		}
	}

	if standardAlphabet && urlAlphabet {
		return outerRawJSON, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}
	if paddingAt >= 0 {
		if len(text)-paddingAt > 2 || len(text)%4 != 0 {
			return outerRawJSON, newDecodeError(ERR_IMPORT_ENCODING, nil)
		}
		if standardAlphabet {
			return outerBase64StandardPadded, nil
		}
		return outerBase64URLPadded, nil
	}
	if len(text)%4 == 1 {
		return outerRawJSON, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}
	if standardAlphabet {
		return outerBase64StandardUnpadded, nil
	}
	return outerBase64URLUnpadded, nil
}

func decodeBase64(text string, form outerForm) ([]byte, error) {
	if text == "" {
		return nil, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}

	const maxEncodedSize = (maxCompressedSize + 2) / 3 * 4
	if len(text) > maxEncodedSize {
		return nil, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}

	missingPadding := (4 - len(text)%4) % 4
	if missingPadding == 3 {
		return nil, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}
	padded := text + strings.Repeat("=", missingPadding)
	terminalPadding := missingPadding
	for index := len(text) - 1; index >= 0 && text[index] == '='; index-- {
		terminalPadding++
	}
	decodedSize := len(padded)/4*3 - terminalPadding
	if decodedSize > maxCompressedSize {
		return nil, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}

	var codec *base64.Encoding
	switch form {
	case outerBase64URLPadded, outerBase64URLUnpadded:
		codec = base64.URLEncoding
	case outerBase64StandardPadded, outerBase64StandardUnpadded:
		codec = base64.StdEncoding
	case outerRawJSON:
		return nil, newDecodeError(ERR_IMPORT_ENCODING, nil)
	default:
		return nil, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}

	decoded := make([]byte, decodedSize)
	written, err := codec.Decode(decoded, []byte(padded))
	if err != nil {
		return nil, newDecodeError(ERR_IMPORT_ENCODING, nil)
	}
	return decoded[:written], nil
}
