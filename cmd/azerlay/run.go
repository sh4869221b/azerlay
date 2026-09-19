package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/profilesource"
	"github.com/urfave/cli/v3"
)

const runHelp = `Usage: azerlay run [--config PATH] [--foreground]

Run in foreground until quit, SIGINT or SIGTERM. Requires a Wayland session.
Configuration must exist; a saved profile is optional. Renderer and device input are unavailable.

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

func runApplication(cmd *cli.Command, stdout, stderr io.Writer) int {
	if cmd.IsSet("help") {
		if _, err := fmt.Fprintln(stdout, runHelp); err != nil {
			return 1
		}
		return 0
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return writeRunFailure(&reportError{"ERR_RUNTIME_WAYLAND", "runtime", "Wayland display is unavailable.", "Run in a Wayland session with WAYLAND_DISPLAY set."}, stdout, stderr)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pipeSignals := make(chan os.Signal, 1)
	signal.Notify(pipeSignals, syscall.SIGPIPE)
	defer signal.Stop(pipeSignals)
	return runForeground(ctx, cmd.String("config"), stdout, stderr)
}

func runForeground(ctx context.Context, configPath string, stdout, stderr io.Writer) (status int) {
	owner, existing, err := control.AcquireInstance(ctx)
	if err != nil {
		return runStartupFailure(ctx, err, stdout, stderr)
	}
	if existing != nil {
		return writeControlReport(controlReport{SchemaVersion: 1, Command: "run", OK: true, Result: *existing}, false, stdout, stderr)
	}
	defer func() {
		if err := owner.Close(); err != nil {
			status = writeRunError(err, stdout, stderr)
		}
	}()

	// Only startup cancellation may stop config directly. Once transport starts,
	// its cleanup must stop admission and join handlers before joining config.
	managerCtx, managerCancel := context.WithCancel(context.Background())
	stopStartup := context.AfterFunc(ctx, managerCancel)
	defer stopStartup()
	manager, err := config.Start(managerCtx, configPath)
	if err != nil {
		managerCancel()
		return runStartupFailure(ctx, err, stdout, stderr)
	}
	stopConfig := func() { managerCancel(); <-manager.Done() }
	var server *control.Server
	defer func() {
		if server == nil {
			stopConfig()
		} else if err := server.Close(); err != nil {
			status = writeRunError(err, stdout, stderr)
		}
	}()
	dataHome, err := profilesource.ResolveDataHome()
	if err != nil {
		return writeRunError(control.NewError(profilesource.ERR_PROFILE_STORAGE), stdout, stderr)
	}
	controller, err := control.NewController(managerCtx, manager, profilesource.NewImportedSource(dataHome))
	if err != nil {
		return runStartupFailure(ctx, err, stdout, stderr)
	}
	response := controller.Dispatch(managerCtx, control.Request{Version: control.ProtocolVersion, ID: "startup", Method: control.MethodStatus})
	if response.Error != nil {
		return runStartupFailure(ctx, response.Error, stdout, stderr)
	}
	if !stopStartup() || ctx.Err() != nil {
		return 0
	}
	server, err = owner.Start(ctx, controller, stopConfig)
	if err != nil {
		return runStartupFailure(ctx, err, stdout, stderr)
	}
	if status := writeControlReport(controlReport{SchemaVersion: 1, Command: "run", OK: true, Result: response.Result}, false, stdout, stderr); status != 0 {
		return status
	}
	if current, ok := response.Result.(control.Status); ok && current.ActiveProfile == nil {
		if _, err := fmt.Fprintln(stdout, "Import a supported profile with azerlay import, then use azerlay status and azerlay profiles select."); err != nil {
			return 1
		}
	}
	<-server.Done()
	return 0
}

func runStartupFailure(ctx context.Context, err error, stdout, stderr io.Writer) int {
	if ctx.Err() != nil {
		return 0
	}
	return writeRunError(err, stdout, stderr)
}

func writeRunError(err error, stdout, stderr io.Writer) int {
	var configError *config.Error
	if errors.As(err, &configError) {
		return writeRunFailure(&reportError{configError.Code, configError.Stage, configError.Reason, "Check the configuration file and use run --help."}, stdout, stderr)
	}
	failure := control.NewError(control.ERR_CONTROL_UNAVAILABLE)
	errors.As(err, &failure)
	return writeRunFailure(&reportError{failure.Code, failure.Stage, failure.Summary, failure.Remediation}, stdout, stderr)
}

func writeRunFailure(failure *reportError, stdout, stderr io.Writer) int {
	return writeControlReport(controlReport{SchemaVersion: 1, Command: "run", Error: failure}, false, stdout, stderr)
}
