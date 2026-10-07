package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/sh4869221b/azerlay/internal/diagnostics"
	"github.com/urfave/cli/v3"
)

const doctorHelp = `Usage: azerlay doctor [--config PATH] [--json] [--include-bindings]

Diagnose next-start configuration and profile selection, plus live socket status.
Checks do not repair or change configuration, saved profiles, or runtime state.
Hidraw identity and read-only access are checked without consuming reports.
Start the official software in SOFTWARE mode; notification initialization is not checked.
Renderer checks are not implemented and are reported as warnings.

Options:
  --config PATH       Read this configuration instead of the default XDG path
  --json              Print schema-versioned JSON
  --include-bindings  Include selected-profile labels and normalized bindings for this invocation
  --help              Print this help without performing checks

Exit codes: 0 no findings; 1 warnings; 2 errors; 3 internal or output failure.`

func normalizeDoctorArgs(args []string) (normalized []string, jsonMode, valid bool) {
	jsonMode = slices.Contains(args, "--json")
	seen := make(map[string]bool)
	for i := 0; i < len(args); i++ {
		name, value, equals := strings.Cut(args[i], "=")
		if seen[name] {
			return nil, jsonMode, false
		}
		seen[name] = true
		switch name {
		case "--help", "--json", "--include-bindings":
			if equals {
				return nil, jsonMode, false
			}
			normalized = append(normalized, name)
		case "--config":
			if !equals {
				if i+1 == len(args) {
					return nil, jsonMode, false
				}
				i++
				value = args[i]
			}
			if value == "" {
				return nil, jsonMode, false
			}
			normalized = append(normalized, name, value)
		default:
			return nil, jsonMode, false
		}
	}
	return normalized, jsonMode, true
}

func runDoctor(cmd *cli.Command, stdout io.Writer) int {
	if cmd.IsSet("help") {
		if _, err := fmt.Fprintln(stdout, doctorHelp); err != nil {
			return 3
		}
		return 0
	}
	report := diagnostics.Collect(context.Background(), cmd.String("config"), cmd.Bool("include-bindings"))
	if err := diagnostics.WriteReport(stdout, report, cmd.Bool("json")); err != nil {
		return 3
	}
	return report.ExitCode
}

func writeDoctorUsage(jsonMode bool, stdout, stderr io.Writer) int {
	report := diagnostics.NewReport([]diagnostics.Check{{
		Category: "command", Code: "ERR_CLI_USAGE", Summary: "Invalid command arguments.",
		Remediation: "Use doctor --help for usage.", Severity: diagnostics.SeverityError,
	}}, nil)
	output := stderr
	if jsonMode {
		output = stdout
	}
	if err := diagnostics.WriteReport(output, report, jsonMode); err != nil {
		return 3
	}
	return report.ExitCode
}
