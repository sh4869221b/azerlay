package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/sh4869221b/azerlay/internal/cli"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/live"
	"github.com/sh4869221b/azerlay/internal/overlay"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func runApplication(configPath string, stdout, stderr io.Writer) int {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return cli.WriteRunFailure(&cli.ReportError{Code: "ERR_RUNTIME_WAYLAND", Stage: "runtime", Summary: "Wayland display is unavailable.", Remediation: "Run in a Wayland session with WAYLAND_DISPLAY set."}, stdout, stderr)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pipeSignals := make(chan os.Signal, 1)
	signal.Notify(pipeSignals, syscall.SIGPIPE)
	defer signal.Stop(pipeSignals)
	return runForeground(ctx, configPath, stdout, stderr)
}

func runForeground(ctx context.Context, configPath string, stdout, stderr io.Writer) (status int) {
	owner, existing, err := control.AcquireInstance(ctx)
	if err != nil {
		return runStartupFailure(ctx, err, stdout, stderr)
	}
	if existing != nil {
		return cli.WriteRunResult(*existing, stdout, stderr)
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
	var coordinator *live.Coordinator
	var controller *control.Controller
	defer func() {
		if server == nil {
			if coordinator != nil {
				if err := coordinator.Close(); err != nil {
					status = writeRunError(err, stdout, stderr)
				}
			}
			if controller != nil {
				controller.Close()
			}
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
	var source *profilesource.ImportedSource
	var importedUnavailable bool
	if manager.Snapshot().Config.Profile.Source != "local" {
		dataHome, err := profilesource.ResolveDataHome()
		if err != nil {
			if manager.Snapshot().Config.Profile.Source != "auto" {
				return writeRunError(control.NewError(profilesource.ERR_PROFILE_STORAGE), stdout, stderr)
			}
			importedUnavailable = true
		} else {
			source = profilesource.NewImportedSource(dataHome)
		}
	}
	controller, err = control.NewController(managerCtx, manager, source)
	if err != nil {
		return runStartupFailure(ctx, err, stdout, stderr)
	}
	if !stopStartup() || ctx.Err() != nil {
		return 0
	}
	if importedUnavailable {
		initial := controller.Dispatch(managerCtx, control.Request{Version: control.ProtocolVersion, ID: "startup", Method: control.MethodStatus}).Result.(control.Status)
		if initial.ActiveProfile == nil || initial.ActiveProfile.Source != "local" {
			return writeRunError(control.NewError(profilesource.ERR_PROFILE_STORAGE), stdout, stderr)
		}
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
	device := live.StartDevice(managerCtx, initialConfig.Config.Device)
	coordinator = live.Start(managerCtx, live.Sources{Config: manager, Profile: controller, Input: device})
	controller.SetLiveStatusProvider(coordinator.Status)
	var liveError error
	server, err = owner.Start(ctx, controller, func() { liveError = coordinator.Close(); controller.Close(); stopConfig() })
	if err != nil {
		return runStartupFailure(ctx, err, stdout, stderr)
	}
	if status := cli.WriteRunResult(response.Result, stdout, stderr); status != 0 {
		return status
	}
	if current, ok := response.Result.(control.Status); ok && current.ActiveProfile == nil {
		if _, err := fmt.Fprintln(stdout, "Import a supported profile with azerlay import, then use azerlay status and azerlay profiles select."); err != nil {
			return 1
		}
	}
	loop := glib.NewMainLoop(nil, false)
	stopEvents = watchLive(coordinator, server, window, loop)
	loop.Run()
	if err := server.Close(); err != nil {
		return writeRunError(err, stdout, stderr)
	}
	if liveError != nil {
		return writeRunError(liveError, stdout, stderr)
	}
	return 0
}

func runStartupFailure(ctx context.Context, err error, stdout, stderr io.Writer) int {
	if ctx.Err() != nil {
		return 0
	}
	return writeRunError(err, stdout, stderr)
}

func writeRunError(err error, stdout, stderr io.Writer) int {
	if errors.Is(err, overlay.ErrDisplayUnavailable) {
		return cli.WriteRunFailure(&cli.ReportError{Code: "ERR_RUNTIME_WAYLAND", Stage: "runtime", Summary: "Wayland display is unavailable.", Remediation: "Run in a Wayland session with WAYLAND_DISPLAY set."}, stdout, stderr)
	}
	if errors.Is(err, overlay.ErrLayerUnavailable) {
		return cli.WriteRunFailure(&cli.ReportError{Code: "ERR_LAYER_SHELL_UNAVAILABLE", Stage: "runtime", Summary: "Layer Shell is unavailable.", Remediation: "Use a compositor with Layer Shell support."}, stdout, stderr)
	}
	var configError *config.Error
	if errors.As(err, &configError) {
		return cli.WriteRunFailure(&cli.ReportError{Code: configError.Code, Stage: configError.Stage, Summary: configError.Reason, Remediation: "Check the configuration file and use run --help."}, stdout, stderr)
	}
	failure := control.NewError(control.ERR_CONTROL_UNAVAILABLE)
	errors.As(err, &failure)
	return cli.WriteRunFailure(&cli.ReportError{Code: failure.Code, Stage: failure.Stage, Summary: failure.Summary, Remediation: failure.Remediation}, stdout, stderr)
}
