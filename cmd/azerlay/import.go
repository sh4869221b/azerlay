package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
)

const validateHelp = `Usage: azerlay validate [--json] [--software-release RELEASE] {FILE|-|--text PAYLOAD}

Options:
  --json                      Print schema-versioned JSON
  --software-release RELEASE  Attribute the exact Azeron Software release
  --text PAYLOAD              Read literal export text instead of a file
  --help                      Print this help without reading input

Use - for stdin; input is required. No state is saved.`

const importHelp = `Usage: azerlay import [--json] [--software-release RELEASE] [--profile-index N] {FILE|-|--text PAYLOAD}

Options:
  --json                      Print schema-versioned JSON
  --software-release RELEASE  Attribute the exact Azeron Software release
  --profile-index N           Select a one-based profile index for this invocation
  --text PAYLOAD              Read literal export text instead of a file
  --help                      Print this help without reading input

Use - for stdin; input is required. No state is saved.`

type exportOptions struct {
	json, help bool
	release    string
	source     string
	text       *string
	index      int // Zero means no selector; explicit indices are positive.
}

func parseExportArgs(command string, args []string) (exportOptions, *reportError) {
	var options exportOptions
	seen := make(map[string]bool)
	sources := 0
	valid, flags := true, true
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flags && arg == "--" {
			flags = false
			continue
		}
		if !flags || arg == "-" || !strings.HasPrefix(arg, "-") {
			sources++
			options.source = arg
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
			if name == "--json" {
				options.json = true
			} else {
				options.help = true
			}
		case "--text", "--software-release", "--profile-index":
			// Even rejected import-only values consume their literal next token.
			if !equals {
				if i+1 == len(args) {
					valid = false
					continue
				}
				i++
				value = args[i]
			}
			switch name {
			case "--text":
				sources++
				options.text = &value
			case "--software-release":
				options.release = value
			case "--profile-index":
				index, err := strconv.Atoi(value)
				if command != "import" || err != nil || index <= 0 || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) != -1 {
					valid = false
				} else {
					options.index = index
				}
			}
		default:
			valid = false
		}
	}
	if options.help {
		valid = valid && sources == 0 && !seen["--software-release"] && !seen["--text"] && !seen["--profile-index"]
	} else {
		valid = valid && sources == 1
	}
	if !valid {
		return options, &reportError{"ERR_CLI_USAGE", "usage", "Invalid command arguments.", "Use --help for this command and supply exactly one input source."}
	}
	return options, nil
}

func runExport(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	command := args[0]
	options, failure := parseExportArgs(command, args[1:])
	if failure != nil {
		return writeReport(operationReport{SchemaVersion: 1, Command: command, Error: failure}, options.json, stdout, stderr)
	}
	if options.help {
		help := validateHelp
		if command == "import" {
			help = importHelp
		}
		if _, err := fmt.Fprintln(stdout, help); err != nil {
			return 1
		}
		return 0
	}
	result, failure := validateSource(options, stdin)
	// Selection is invocation-local and only sees a fully normalized export.
	if failure == nil && command == "import" {
		switch {
		case len(result.Profiles) == 0 || options.index > len(result.Profiles):
			failure = &reportError{"ERR_PROFILE_NOT_FOUND", "selection", "No profile exists at the requested position.", "Repeat import with --profile-index N and supply the input again; choose an index from a nonempty profile listing."}
		case options.index == 0 && len(result.Profiles) > 1:
			failure = &reportError{"ERR_PROFILE_SELECTION_REQUIRED", "selection", "An explicit profile selection is required.", "Repeat import with --profile-index N and supply the input again."}
		default:
			index := options.index
			if index == 0 {
				index = 1
			}
			result.SelectedProfileIndex = &index
		}
	}
	return writeReport(operationReport{SchemaVersion: 1, Command: command, OK: failure == nil, Result: result, Error: failure}, options.json, stdout, stderr)
}

func validateSource(options exportOptions, stdin io.Reader) (*profileReport, *reportError) {
	var document profiledecode.Document
	var err error
	switch {
	case options.text != nil:
		document, err = profiledecode.DecodeText(*options.text)
	case options.source == "-":
		document, err = profiledecode.DecodeReader(stdin)
	default:
		document, err = profiledecode.DecodeFile(options.source)
	}
	// These boundaries guarantee their respective typed errors. Project only
	// the stable code, never the underlying reader/file cause or source text.
	if err != nil {
		var failure *profiledecode.DecodeError
		errors.As(err, &failure)
		return nil, &reportError{string(failure.Code), "decode", "The export could not be decoded.", "Supply a complete export in a supported input format."}
	}
	raw, err := profileraw.Parse(document)
	if err != nil {
		var failure *profileraw.ParseError
		errors.As(err, &failure)
		return nil, &reportError{string(failure.Code), "raw", "The export structure is invalid.", "Supply a structurally valid software export within the documented limits."}
	}
	bundle, err := profileadapter.Normalize(raw, profile.SourceMetadata{SoftwareRelease: options.release, SourceScope: "azeron-software-export"})
	if err != nil {
		var failure *profileadapter.NormalizeError
		errors.As(err, &failure)
		return nil, &reportError{string(failure.Code), "normalize", "The export could not be normalized.", "Attribute a supported exact software release and use an export within the documented semantic limits."}
	}
	return projectReport(bundle), nil
}
