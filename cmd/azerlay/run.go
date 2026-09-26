package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/overlay"
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
	var window *overlay.Window
	var stopEvents func() error
	defer func() {
		if server == nil {
			stopConfig()
		} else if err := server.Close(); err != nil {
			status = writeRunError(err, stdout, stderr)
		}
		if stopEvents != nil {
			if err := stopEvents(); err != nil {
				status = writeRunError(err, stdout, stderr)
			}
		}
		if window != nil {
			window.Close()
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
	if !stopStartup() || ctx.Err() != nil {
		return 0
	}
	initialConfig := manager.Snapshot()
	window, err = overlay.New(initialConfig.Config.Overlay)
	if err != nil {
		return writeRunError(err, stdout, stderr)
	}
	window.SetObserver(func(state overlay.State) {
		var diagnostic *control.Diagnostic
		switch state.Diagnostic {
		case "OVERLAY_MONITOR_UNAVAILABLE":
			diagnostic = &control.Diagnostic{Code: state.Diagnostic, Stage: "overlay", Reason: "Configured monitor is unavailable."}
		case "ERR_OVERLAY_SURFACE":
			diagnostic = &control.Diagnostic{Code: state.Diagnostic, Stage: "overlay", Reason: "Overlay surface is unavailable."}
		case "ERR_OVERLAY_INPUT_REGION":
			diagnostic = &control.Diagnostic{Code: state.Diagnostic, Stage: "overlay", Reason: "Overlay input region is unavailable."}
		case "ERR_OVERLAY_PLACEMENT":
			diagnostic = &control.Diagnostic{Code: state.Diagnostic, Stage: "overlay", Reason: "Overlay placement is unavailable."}
		}
		controller.SetOverlayStatus(control.OverlayStatus{Mapped: state.Mapped, InputRegionApplied: state.InputRegionApplied}, diagnostic)
	})
	response := controller.Dispatch(managerCtx, control.Request{Version: control.ProtocolVersion, ID: "startup", Method: control.MethodStatus})
	if response.Error != nil {
		return runStartupFailure(ctx, response.Error, stdout, stderr)
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
	loop := glib.NewMainLoop(nil, false)
	stopEvents = watchOverlay(manager, controller, server, window, loop, initialConfig.Status.ConfigGeneration)
	loop.Run()
	return 0
}

func watchOverlay(manager *config.Manager, controller *control.Controller, server *control.Server, window *overlay.Window, loop *glib.MainLoop, lastGeneration uint64) func() error {
	stop := make(chan struct{})
	done := make(chan struct{})
	var mu sync.Mutex
	var pending glib.SourceHandle
	var quitPending glib.SourceHandle
	var applyError error
	go func() {
		defer close(done)
		changes := manager.Changes()
		visibility := controller.VisibilityChanges()
		for {
			select {
			case <-server.Done():
				mu.Lock()
				quitPending = glib.IdleAdd(func() {
					mu.Lock()
					quitPending = 0
					mu.Unlock()
					loop.Quit()
				})
				mu.Unlock()
				return
			case <-stop:
				return
			case _, ok := <-changes:
				if !ok {
					changes = nil
					continue
				}
			case <-visibility:
			}
			mu.Lock()
			if pending == 0 {
				pending = glib.IdleAdd(func() {
					mu.Lock()
					pending = 0
					mu.Unlock()
					snapshot := manager.Snapshot()
					if snapshot.Status.ConfigGeneration > lastGeneration {
						lastGeneration = snapshot.Status.ConfigGeneration
						if err := window.ApplyConfig(snapshot.Config.Overlay); err != nil {
							applyError = err
							loop.Quit()
							return
						}
					}
					if err := window.SetVisible(controller.RequestedVisible()); err != nil {
						applyError = err
						loop.Quit()
					}
				})
			}
			mu.Unlock()
		}
	}()
	return func() error {
		close(stop)
		<-done
		mu.Lock()
		if pending != 0 {
			glib.SourceRemove(pending)
		}
		if quitPending != 0 {
			glib.SourceRemove(quitPending)
		}
		mu.Unlock()
		window.SetObserver(nil)
		return applyError
	}
}

func runStartupFailure(ctx context.Context, err error, stdout, stderr io.Writer) int {
	if ctx.Err() != nil {
		return 0
	}
	return writeRunError(err, stdout, stderr)
}

func writeRunError(err error, stdout, stderr io.Writer) int {
	if errors.Is(err, overlay.ErrDisplayUnavailable) {
		return writeRunFailure(&reportError{"ERR_RUNTIME_WAYLAND", "runtime", "Wayland display is unavailable.", "Run in a Wayland session with WAYLAND_DISPLAY set."}, stdout, stderr)
	}
	if errors.Is(err, overlay.ErrLayerUnavailable) {
		return writeRunFailure(&reportError{"ERR_LAYER_SHELL_UNAVAILABLE", "runtime", "Layer Shell is unavailable.", "Use a compositor with Layer Shell support."}, stdout, stderr)
	}
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
