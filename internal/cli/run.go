package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v3"
)

const runHelp = `Usage: azerlay run [--config PATH] [--foreground]

Run in foreground until quit, SIGINT or SIGTERM. Requires a Wayland session.
Configuration must exist; a saved profile is optional. Show selected profile assignments and last-observed raw input.

Options:
  --config PATH  Read this configuration instead of the default XDG path
  --foreground   Stay in foreground (the default)
  --help         Print this help without starting the application`

func normalizeRunArgs(args []string) (normalized []string, valid bool) {
	seen := make(map[string]bool)
	for i := 0; i < len(args); i++ {
		name, value, equals := strings.Cut(args[i], "=")
		if seen[name] {
			return nil, false
		}
		seen[name] = true
		switch name {
		case "--help", "--foreground":
			if equals {
				return nil, false
			}
			normalized = append(normalized, name)
		case "--config":
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
		default:
			return nil, false
		}
	}
	return normalized, true
}

func runApplication(cmd *cli.Command, stdout, stderr io.Writer, startForeground func(string, io.Writer, io.Writer) int) int {
	if cmd.IsSet("help") {
		if _, err := fmt.Fprintln(stdout, runHelp); err != nil {
			return 1
		}
		return 0
	}
	return startForeground(cmd.String("config"), stdout, stderr)
}
