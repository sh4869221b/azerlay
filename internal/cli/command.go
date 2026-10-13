package cli

import (
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
)

type helpRequested bool

// A named boolean prevents urfave's built-in help shortcut, which ignores
// HideHelp for a flag named help and bypasses our checked output writes.
type helpFlag struct{ cli.BoolFlag }

func (f *helpFlag) Get() any {
	return helpRequested(f.BoolFlag.Get().(bool))
}

// Doctor and run share strict --config parsing, with command-specific boolean
// flags. Keep values literal so whitespace and flag-like paths survive.
func normalizeConfigArgs(args []string, booleanFlags ...string) (normalized []string, valid bool) {
	seen := make(map[string]bool)
	for i := 0; i < len(args); i++ {
		name, value, equals := strings.Cut(args[i], "=")
		if seen[name] {
			return nil, false
		}
		seen[name] = true
		if name == "--config" {
			if !equals {
				if i+1 == len(args) {
					return nil, false
				}
				i++
				value = args[i]
			}
			if value == "" {
				return nil, false
			}
			normalized = append(normalized, name, value)
		} else if !equals && slices.Contains(booleanFlags, name) {
			normalized = append(normalized, name)
		} else {
			return nil, false
		}
	}
	return normalized, true
}

// Keep the original token grammar and JSON error mode before urfave parses
// values. Positionals follow -- so bare stdin and filename whitespace survive.
func normalizeExportArgs(args []string) (normalized []string, jsonMode, valid bool) {
	valid, flags := true, true
	seen := make(map[string]bool)
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flags && arg == "--" {
			flags = false
			continue
		}
		if !flags || arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}
		name, value, equals := strings.Cut(arg, "=")
		if seen[name] {
			valid = false
		}
		seen[name] = true
		switch name {
		case "--json", "--help":
			if equals {
				valid = false
				continue
			}
			jsonMode = jsonMode || name == "--json"
			normalized = append(normalized, name)
		case "--text", "--software-release", "--profile-index":
			// Import-only flags still consume a literal value on validate.
			if !equals {
				if i+1 == len(args) {
					valid = false
					continue
				}
				i++
				value = args[i]
			}
			// Equals-form tokens are trimmed by urfave; separate values are not.
			normalized = append(normalized, name, value)
		default:
			valid = false
		}
	}
	return append(append(normalized, "--"), positionals...), jsonMode, valid
}
