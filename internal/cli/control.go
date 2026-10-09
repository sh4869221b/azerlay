package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/urfave/cli/v3"
)

func controlCommandName(name string) string {
	if name == "select" {
		return "profiles select"
	}
	return name
}

func normalizeControlArgs(args []string, selection bool) (normalized []string, jsonMode, valid bool) {
	valid, flags := true, true
	seen := make(map[string]bool)
	var positionals []string
	for _, arg := range args {
		if flags && arg == "--" {
			flags = false
			continue
		}
		if !flags || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}
		if (arg != "--help" && arg != "--json") || seen[arg] {
			valid = false
			continue
		}
		seen[arg] = true
		jsonMode = jsonMode || arg == "--json"
		normalized = append(normalized, arg)
	}
	if selection {
		valid = valid && (len(positionals) == 1 && positionals[0] != "" || seen["--help"] && len(positionals) == 0)
	} else {
		valid = valid && len(positionals) == 0
	}
	return append(append(normalized, "--"), positionals...), jsonMode, valid
}

func runControl(cmd *cli.Command, stdout, stderr io.Writer) int {
	name := controlCommandName(cmd.Name)
	if cmd.IsSet("help") {
		syntax := " [--json]"
		detail := "Control the running instance."
		if cmd.Name == "select" {
			syntax = " [--json] [--] <id|name>"
			detail = "Select exactly one imported profile by ID or name for this session. Saved selection is unchanged."
		}
		if cmd.Name == "reload" {
			detail = "Accept a configuration reload request. Check status for the eventual result."
		}
		if cmd.Name == "quit" {
			detail = "Stop the running application after acknowledging the request."
		}
		if cmd.Name == "status" {
			detail = "Show active runtime state and unavailable capabilities."
		}
		if _, err := fmt.Fprintf(stdout, "Usage: azerlay %s%s\n\n%s\n\nOptions:\n  --json  Print schema-versioned JSON\n  --help  Print this help without connecting\n", name, syntax, detail); err != nil {
			return 1
		}
		return 0
	}
	var method control.Method
	switch cmd.Name {
	case "show":
		method = control.MethodShow
	case "hide":
		method = control.MethodHide
	case "toggle":
		method = control.MethodToggle
	case "reload":
		method = control.MethodReload
	case "status":
		method = control.MethodStatus
	case "select":
		method = control.MethodSelect
	case "quit":
		method = control.MethodQuit
	}
	response, err := control.Call(context.Background(), method, control.Params{Selector: cmd.Args().First()})
	report := controlReport{SchemaVersion: 1, Command: name, OK: response.OK, Result: response.Result}
	failure := response.Error
	if err != nil {
		errors.As(err, &failure)
	}
	if failure != nil {
		report.Error = &ReportError{failure.Code, failure.Stage, failure.Summary, failure.Remediation}
	}
	return writeControlReport(report, cmd.Bool("json"), stdout, stderr)
}
