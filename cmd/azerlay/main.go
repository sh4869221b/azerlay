package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/urfave/cli/v3"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rootUsage := func() int {
		if _, err := fmt.Fprintln(stderr, "invalid command; use --help for usage"); err != nil {
			return 1
		}
		return 2
	}
	status, handled := 0, false
	usage := rootUsage
	command := func(name string, action func(*cli.Command) int) *cli.Command {
		return &cli.Command{
			Name: name, HideHelp: true, HideVersion: true,
			Action: func(_ context.Context, cmd *cli.Command) error {
				status, handled = action(cmd), true
				return nil
			},
			OnUsageError: func(context.Context, *cli.Command, error, bool) error {
				status, handled = usage(), true
				return nil
			},
		}
	}
	root := command("azerlay", func(cmd *cli.Command) int {
		if len(args) == 1 && args[0] == "--help" && cmd.IsSet("help") {
			if _, err := fmt.Fprintln(stdout, "Usage: azerlay <command>\n\nCommands:\n  version  Print version information\n  validate Validate a profile export without saving state\n  import   Save an export and profile selection\n  profiles Show saved or select active profiles\n  show     Request overlay visibility\n  hide     Request overlay hidden\n  toggle   Toggle requested visibility\n  reload   Request configuration reload\n  status   Show running instance status\n  quit     Stop the running control server\n\n"+validateHelp+"\n\n"+importHelp+"\n\n"+profilesHelp); err != nil {
				return 1
			}
			return 0
		}
		return rootUsage()
	})
	root.Reader, root.Writer, root.ErrWriter = stdin, stdout, stderr
	root.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	root.Flags = []cli.Flag{&helpFlag{BoolFlag: cli.BoolFlag{Name: "help", Local: true}}}
	root.Commands = []*cli.Command{command("version", func(*cli.Command) int {
		if len(args) != 1 {
			return rootUsage()
		}
		if _, err := fmt.Fprintf(stdout, "azerlay %s\n", version); err != nil {
			return 1
		}
		return 0
	})}
	prepareArgs := make(map[*cli.Command]func([]string) ([]string, bool))
	for _, name := range []string{"validate", "import"} {
		leaf := command(name, func(cmd *cli.Command) int {
			options, valid := exportCommandOptions(cmd)
			if !valid {
				return usage()
			}
			return runExport(cmd.Name, options, cmd.Reader, cmd.Writer, cmd.ErrWriter)
		})
		leaf.Flags = []cli.Flag{
			&cli.BoolFlag{Name: "json", Local: true},
			&helpFlag{BoolFlag: cli.BoolFlag{Name: "help", Local: true}},
			&cli.StringFlag{Name: "text", Local: true},
			&cli.StringFlag{Name: "software-release", Local: true},
		}
		if name == "import" {
			leaf.Flags = append(leaf.Flags, &cli.StringFlag{Name: "profile-index", Local: true})
		}
		prepareArgs[leaf] = func(original []string) ([]string, bool) {
			normalized, jsonMode, valid := normalizeExportArgs(original)
			usage = func() int {
				failure := &reportError{"ERR_CLI_USAGE", "usage", "Invalid command arguments.", "Use --help for this command and supply exactly one input source."}
				return writeReport(operationReport{SchemaVersion: 1, Command: name, Error: failure}, jsonMode, stdout, stderr)
			}
			return normalized, valid
		}
		root.Commands = append(root.Commands, leaf)
	}
	addControl := func(name string) *cli.Command {
		leaf := command(name, func(cmd *cli.Command) int {
			return runControl(cmd, stdout, stderr)
		})
		leaf.Flags = []cli.Flag{
			&cli.BoolFlag{Name: "json", Local: true},
			&helpFlag{BoolFlag: cli.BoolFlag{Name: "help", Local: true}},
		}
		prepareArgs[leaf] = func(original []string) ([]string, bool) {
			normalized, jsonMode, valid := normalizeControlArgs(original, name == "select")
			usage = func() int {
				failure := &reportError{"ERR_CLI_USAGE", "usage", "Invalid command arguments.", "Use --help for this command."}
				return writeControlReport(controlReport{SchemaVersion: 1, Command: controlCommandName(name), Error: failure}, jsonMode, stdout, stderr)
			}
			return normalized, valid
		}
		return leaf
	}
	for _, name := range []string{"show", "hide", "toggle", "reload", "status", "quit"} {
		root.Commands = append(root.Commands, addControl(name))
	}
	selectProfile := addControl("select")
	profilesAction := func(cmd *cli.Command) int {
		return runProfiles(cmd.Bool("json"), cmd.IsSet("help"), cmd.Writer, cmd.ErrWriter)
	}
	profiles := command("profiles", profilesAction)
	show := command("show", profilesAction)
	profiles.Commands = []*cli.Command{show, selectProfile}
	for _, node := range []*cli.Command{profiles, show} {
		node.Flags = []cli.Flag{
			&cli.BoolFlag{Name: "json", Local: true},
			&helpFlag{BoolFlag: cli.BoolFlag{Name: "help", Local: true}},
		}
	}
	prepareArgs[profiles] = func(original []string) ([]string, bool) {
		if len(original) > 0 && original[0] == "select" {
			normalized, valid := prepareArgs[selectProfile](original[1:])
			return append([]string{"select"}, normalized...), valid
		}
		usage = func() int {
			failure := &reportError{"ERR_CLI_USAGE", "usage", "Invalid command arguments.", "Use profiles show --help; no input, release or selector is accepted."}
			// Unlike exports, even a terminated or otherwise invalid --json counts.
			return writeReport(operationReport{SchemaVersion: 1, Command: "profiles show", Error: failure}, slices.Contains(original, "--json"), stdout, stderr)
		}
		if len(original) == 1 && original[0] == "--help" {
			return original, true
		}
		if len(original) == 0 || profiles.Command(original[0]) == nil {
			return original, false
		}
		seen := make(map[string]bool)
		for _, arg := range original[1:] {
			if (arg != "--json" && arg != "--help") || seen[arg] {
				return original, false
			}
			seen[arg] = true
		}
		return original, true
	}
	root.Commands = append(root.Commands, profiles)

	if len(args) == 0 || (args[0] != "--help" && root.Command(args[0]) == nil) || (args[0] == "--help" && len(args) != 1) {
		return rootUsage()
	}
	frameworkArgs := args
	if prepare := prepareArgs[root.Command(args[0])]; prepare != nil {
		normalized, valid := prepare(args[1:])
		if !valid {
			return usage()
		}
		frameworkArgs = append([]string{args[0]}, normalized...)
	}
	if err := root.Run(context.Background(), append([]string{"azerlay"}, frameworkArgs...)); err != nil && !handled {
		return usage()
	}
	return status
}
