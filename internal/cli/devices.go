package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/sh4869221b/azerlay/internal/device"
)

const devicesHelp = `Usage: azerlay devices <command>

Commands:
  list     List supported device groups and access diagnostics
  inspect  Inspect relevant nodes, or one hidraw path

Use devices list --help or devices inspect --help for options.
Discovery does not consume HID reports or change permissions.`

const devicesListHelp = `Usage: azerlay devices list [--json] [--help]

List supported Cyborg II groups and their admitted hidraw nodes.
Complete means one admitted interface04; access failures retain its metadata.

Options:
  --json  Print schema-versioned JSON
  --help  Print this help without discovering devices

Exit codes: 0 readable qualifying node without errors; 1 discovery, access or output failure; 2 usage.`

const devicesInspectHelp = `Usage: azerlay devices inspect [path] [--json] [--help]

Inspect relevant candidates, including excluded nodes and serial values.
With a path, inspect only that hidraw node; symlinks and relative paths are accepted.
Use -- before a path that starts with a dash.

Options:
  --json  Print schema-versioned JSON
  --help  Print this help without discovering devices

Inspection does not consume HID reports or change permissions.
Exit codes: 0 readable qualifying node without errors; 1 discovery, access or output failure; 2 usage.`

type devicesOptions struct {
	command string
	path    string
	json    bool
	help    bool
}

type deviceCollector func(command, path string) (*device.Result, *device.Diagnostic)

func collectDevices(command, path string) (*device.Result, *device.Diagnostic) {
	if command == "devices inspect" {
		return device.Inspect(path)
	}
	return device.Discover()
}

func normalizeDevicesArgs(args []string) ([]string, devicesOptions, bool) {
	o := devicesOptions{command: "devices"}
	// Error mode recognizes literal --json even after an invalid earlier token.
	for _, arg := range args {
		if arg == "--" {
			break
		}
		o.json = o.json || arg == "--json"
	}
	if len(args) == 1 && args[0] == "--help" {
		o.help = true
		return args, o, true
	}
	if len(args) == 0 || args[0] != "list" && args[0] != "inspect" {
		return nil, o, false
	}
	o.command += " " + args[0]
	normalized := []string{args[0]}
	seen := map[string]bool{}
	flags, valid := true, true
	positionals := []string{}
	for _, arg := range args[1:] {
		if flags && arg == "--" {
			flags = false
			continue
		}
		if !flags || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}
		if arg != "--json" && arg != "--help" || seen[arg] {
			valid = false
			continue
		}
		seen[arg] = true
		normalized = append(normalized, arg)
		o.help = o.help || arg == "--help"
	}
	if len(positionals) > 1 || args[0] == "list" && len(positionals) > 0 {
		valid = false
	}
	if len(positionals) == 1 {
		o.path = positionals[0]
		if o.path == "" {
			valid = false
		}
	}
	return append(append(normalized, "--"), positionals...), o, valid
}

func runDevices(o devicesOptions, collect deviceCollector, stdout, stderr io.Writer) int {
	if o.help {
		help := devicesHelp
		if o.command == "devices list" {
			help = devicesListHelp
		}
		if o.command == "devices inspect" {
			help = devicesInspectHelp
		}
		if _, err := fmt.Fprintln(stdout, help); err != nil {
			return 1
		}
		return 0
	}
	result, failure := collect(o.command, o.path)
	return writeDevicesReport(projectDevicesReport(o.command, result, failure), o.json, stdout, stderr)
}

func writeDevicesUsage(o devicesOptions, stdout, stderr io.Writer) int {
	failure := &device.Diagnostic{Code: "ERR_CLI_USAGE", Severity: "error", Stage: "usage", Summary: "Invalid command arguments.", Remediation: "Use devices --help for usage."}
	return writeDevicesReport(devicesReport{SchemaVersion: 2, Command: o.command, Error: failure}, o.json, stdout, stderr)
}
