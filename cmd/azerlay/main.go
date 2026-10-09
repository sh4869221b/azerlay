package main

import (
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/sh4869221b/azerlay/internal/cli"
)

var version = "dev"

func main() {
	runtime.LockOSThread()
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 && (args[0] == "doctor" || args[0] == "devices") {
		pipeSignals := make(chan os.Signal, 1)
		signal.Notify(pipeSignals, syscall.SIGPIPE)
		defer signal.Stop(pipeSignals)
	}
	return cli.Run(args, version, stdin, stdout, stderr, runApplication)
}
