package main

import (
	"fmt"
	"io"
	"os"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		if _, err := fmt.Fprintf(stdout, "azerlay %s\n", version); err != nil {
			return 1
		}
		return 0
	}

	if len(args) == 1 && args[0] == "--help" {
		if _, err := fmt.Fprintln(stdout, "Usage: azerlay <command>\n\nCommands:\n  version  Print version information"); err != nil {
			return 1
		}
		return 0
	}

	if _, err := fmt.Fprintln(stderr, "invalid command; use --help for usage"); err != nil {
		return 1
	}

	return 2
}
