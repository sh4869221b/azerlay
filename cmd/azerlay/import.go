package main

import (
	"errors"
	"fmt"
	"io"
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

type validateOptions struct {
	json, help bool
	release    string
	source     string
	text       *string
}

func parseValidateArgs(args []string) (validateOptions, *reportError) {
	var options validateOptions
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
				valid = false
			}
		default:
			valid = false
		}
	}
	if options.help {
		valid = valid && sources == 0 && !seen["--software-release"] && !seen["--text"]
	} else {
		valid = valid && sources == 1
	}
	if !valid {
		return options, &reportError{"ERR_CLI_USAGE", "usage", "Invalid command arguments.", "Use validate --help and supply exactly one input source."}
	}
	return options, nil
}

func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, failure := parseValidateArgs(args)
	if failure != nil {
		return writeReport(operationReport{SchemaVersion: 1, Command: "validate", Error: failure}, options.json, stdout, stderr)
	}
	if options.help {
		if _, err := fmt.Fprintln(stdout, validateHelp); err != nil {
			return 1
		}
		return 0
	}
	result, failure := validateSource(options, stdin)
	return writeReport(operationReport{SchemaVersion: 1, Command: "validate", OK: failure == nil, Result: result, Error: failure}, options.json, stdout, stderr)
}

func validateSource(options validateOptions, stdin io.Reader) (*profileReport, *reportError) {
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
