package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

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

func exportCommandOptions(cmd *cli.Command) (exportOptions, bool) {
	options := exportOptions{json: cmd.Bool("json"), help: cmd.IsSet("help"), release: cmd.String("software-release"), source: cmd.Args().First()}
	sources := cmd.Args().Len()
	if cmd.IsSet("text") {
		text := cmd.String("text")
		options.text = &text
		sources++
	}
	if cmd.IsSet("profile-index") {
		value := cmd.String("profile-index")
		index, err := strconv.Atoi(value)
		if err != nil || index <= 0 || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) != -1 {
			return options, false
		}
		options.index = index
	}
	if options.help {
		return options, sources == 0 && !cmd.IsSet("software-release") && !cmd.IsSet("profile-index")
	}
	return options, sources == 1
}

func runExport(command string, options exportOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	var failure *reportError
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
