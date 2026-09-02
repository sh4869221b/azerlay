package profiledecode

import (
	"errors"
	"strings"
)

var errMalformedOuterWrapper = errors.New("malformed outer wrapper")

func normalizeOuterText(text string) (string, error) {
	normalized := strings.TrimSpace(text)
	normalized = strings.TrimSpace(strings.TrimPrefix(normalized, "\ufeff"))

	startsFence := strings.HasPrefix(normalized, "```")
	endsFence := strings.HasSuffix(normalized, "```")
	if startsFence != endsFence {
		return "", errMalformedOuterWrapper
	}
	if startsFence {
		if len(normalized) < 6 {
			return "", errMalformedOuterWrapper
		}
		normalized = strings.TrimSpace(normalized[3 : len(normalized)-3])
	}

	for _, quote := range [...]string{"'''", `"""`, "'", `"`} {
		startsQuote := strings.HasPrefix(normalized, quote)
		endsQuote := strings.HasSuffix(normalized, quote)
		if startsQuote != endsQuote {
			return "", errMalformedOuterWrapper
		}
		if startsQuote {
			if len(normalized) < 2*len(quote) {
				return "", errMalformedOuterWrapper
			}
			return strings.TrimSpace(normalized[len(quote) : len(normalized)-len(quote)]), nil
		}
	}

	return strings.TrimSpace(normalized), nil
}
