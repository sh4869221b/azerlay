package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profileadapter"
	"github.com/sh4869221b/azerlay/internal/profiledecode"
	"github.com/sh4869221b/azerlay/internal/profileraw"
	"github.com/sh4869221b/azerlay/internal/profilesource"
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
  --profile-index N           Save a one-based profile selection
  --text PAYLOAD              Read literal export text instead of a file
  --help                      Print this help without reading input

Use - for stdin; input is required. Successful import saves the export and selection.`

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
	prepared, origin, err := prepareSource(options, stdin)
	if err != nil {
		return writeReport(operationReport{SchemaVersion: 1, Command: command, Error: preparationFailure(err)}, options.json, stdout, stderr)
	}
	bundle := prepared.Bundle()
	result := projectReport(bundle)
	// Selection only sees the complete normalized export; Prepared stays intact.
	if command == "import" {
		switch {
		case len(bundle.Profiles) == 0 || options.index > len(bundle.Profiles):
			failure = &reportError{"ERR_PROFILE_NOT_FOUND", "selection", "No profile exists at the requested position.", "Repeat import with --profile-index N and supply the input again; choose an index from a nonempty profile listing."}
		case options.index == 0 && len(bundle.Profiles) > 1:
			failure = &reportError{"ERR_PROFILE_SELECTION_REQUIRED", "selection", "An explicit profile selection is required.", "Repeat import with --profile-index N and supply the input again."}
		default:
			index := options.index
			if index == 0 {
				index = 1
			}
			dataHome, err := profilesource.ResolveDataHome()
			if err == nil {
				_, err = profilesource.NewImportedSource(dataHome).Import(context.Background(), prepared, index, origin)
			}
			if err != nil {
				return writeReport(operationReport{SchemaVersion: 1, Command: command, Error: storageFailure(err)}, options.json, stdout, stderr)
			}
			// Publication precedes output. Delivery failure must not undo the commit.
			result.SelectedProfileIndex = &index
		}
	}
	return writeReport(operationReport{SchemaVersion: 1, Command: command, OK: failure == nil, Result: result, Error: failure}, options.json, stdout, stderr)
}

func prepareSource(options exportOptions, stdin io.Reader) (prepared profilesource.Prepared, origin profilesource.Origin, err error) {
	source := profile.SourceMetadata{SoftwareRelease: options.release, SourceScope: "azeron-software-export"}
	switch {
	case options.text != nil:
		prepared, err = profilesource.PrepareText(*options.text, source)
		return prepared, profilesource.Origin{Kind: "text"}, err
	case options.source == "-":
		prepared, err = profilesource.PrepareReader(stdin, source)
		return prepared, profilesource.Origin{Kind: "stdin"}, err
	default:
		origin = profilesource.Origin{Kind: "file", Path: &options.source}
		file, openErr := os.Open(options.source)
		if openErr != nil {
			return profilesource.Prepared{}, origin, errors.Join(&profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_ENCODING}, openErr)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				prepared = profilesource.Prepared{}
				// Decode errors still win; a close failure precedes raw/normalize
				// reporting, matching the former DecodeFile boundary ordering.
				err = errors.Join(err, &profiledecode.DecodeError{Code: profiledecode.ERR_IMPORT_ENCODING}, closeErr)
			}
		}()
		prepared, err = profilesource.PrepareReader(file, source)
		return prepared, origin, err
	}
}

func preparationFailure(err error) *reportError {
	// These boundaries guarantee their respective typed errors. Project only
	// the stable code, never the underlying reader/file cause or source text.
	var decode *profiledecode.DecodeError
	var raw *profileraw.ParseError
	var normalize *profileadapter.NormalizeError
	switch {
	case errors.As(err, &decode):
		return &reportError{string(decode.Code), "decode", "The export could not be decoded.", "Supply a complete export in a supported input format."}
	case errors.As(err, &raw):
		return &reportError{string(raw.Code), "raw", "The export structure is invalid.", "Supply a structurally valid software export within the documented limits."}
	default:
		errors.As(err, &normalize)
		return &reportError{string(normalize.Code), "normalize", "The export could not be normalized.", "Attribute a supported exact software release and use an export within the documented semantic limits."}
	}
}
