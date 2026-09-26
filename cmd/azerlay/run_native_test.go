package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/layershell"
)

// GTK initialization must happen on the process main thread, including when
// exercising failures that occur after window creation.
func runNativeChild(mode string) int {
	runtime.LockOSThread()
	switch mode {
	case "output-failure":
		return run([]string{"run"}, nil, &refusingWriter{}, os.Stderr)
	case "server-start-failure":
		return runForeground(context.Background(), os.Getenv("AZERLAY_RUN_NATIVE_CONFIG"), io.Discard, os.Stderr)
	case "config-placement":
		return runConfigPlacementChild()
	case "visibility":
		return runVisibilityChild()
	default:
		return 2
	}
}

func TestRunNativeVisibility(t *testing.T) {
	fixture := newRunFixture(t)
	setRunEnvironment(t, fixture)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(fixture.env, "AZERLAY_RUN_NATIVE_CHILD=visibility", "AZERLAY_RUN_NATIVE_CONFIG="+fixture.config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var width, height int
	parsed, scanErr := fmt.Sscanf(stdout.String(), "visibility lifecycle mapped %dx%d closed\n", &width, &height)
	if err != nil || stderr.Len() != 0 || parsed != 2 || scanErr != nil || width <= 0 || height <= 0 {
		t.Fatalf("native visibility: err=%v stdout=%q stderr=%q", err, &stdout, &stderr)
	}
	t.Logf("content-free mapped surface: %dx%d", width, height)
	checkRunOwnerReleased(t, fixture)
}

func runVisibilityChild() int {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	path := os.Getenv("AZERLAY_RUN_NATIVE_CONFIG")
	phase := 0
	var failure error
	var configGeneration, requestGeneration uint64
	var mappedWidth, mappedHeight int
	call := func(method control.Method) error {
		response, err := control.Call(ctx, method, control.Params{})
		if err != nil {
			return err
		}
		if response.Error != nil {
			return response.Error
		}
		return nil
	}
	source := glib.TimeoutAdd(10, func() bool {
		if ctx.Err() != nil {
			return true
		}
		response, err := control.Call(ctx, control.MethodStatus, control.Params{})
		if err != nil || response.Error != nil {
			failure = fmt.Errorf("status: %v %v", err, response.Error)
			cancel()
			return true
		}
		status := response.Result.(control.Status)
		if status.Overlay == nil {
			failure = errors.New("overlay observation was not published")
			cancel()
			return true
		}
		windows := gtk.WindowGetToplevels()
		mapped, width, height := false, 0, 0
		if windows.NItems() == 1 {
			widget := &gtk.Window{Object: windows.Item(0)}
			surface, err := layershell.Surface(widget)
			if err != nil {
				failure = err
				cancel()
				return true
			}
			if surface != nil {
				mapped, width, height = surface.Mapped(), surface.Width(), surface.Height()
			}
		}
		switch phase {
		case 0:
			if status.Visible || status.Overlay.Mapped || status.Overlay.InputRegionApplied || windows.NItems() != 1 || mapped {
				failure = errors.New("startup was not hidden")
				break
			}
			failure = call(control.MethodShow)
			phase++
		case 1, 3, 8:
			if !status.Visible || !status.Overlay.Mapped || !status.Overlay.InputRegionApplied || !mapped {
				return true
			}
			if width <= 0 || height <= 0 || windows.NItems() != 1 {
				failure = fmt.Errorf("mapped surface has invalid size %dx%d", width, height)
				break
			}
			mappedWidth, mappedHeight = width, height
			switch phase {
			case 1:
				failure = call(control.MethodHide)
			case 3:
				failure = call(control.MethodToggle)
			case 8:
				if len(status.DegradedReasons) != 4 {
					failure = errors.New("overlay diagnostic did not clear after recovery")
					break
				}
				for _, method := range []control.Method{control.MethodHide, control.MethodShow, control.MethodHide} {
					if failure = call(method); failure != nil {
						break
					}
				}
			}
			phase++
		case 2, 4, 9:
			if status.Visible || status.Overlay.Mapped || status.Overlay.InputRegionApplied || mapped {
				return true
			}
			switch phase {
			case 2:
				failure = call(control.MethodToggle)
			case 4:
				configGeneration = status.LastReload.ConfigGeneration
				failure = os.WriteFile(path, []byte("schema_version = 1\n[overlay]\nmonitor = 'azerlay-missing-monitor'\n"), 0600)
			case 9:
				failure = call(control.MethodShow)
				if failure == nil {
					failure = call(control.MethodQuit)
				}
			}
			phase++
		case 5:
			if status.LastReload.ConfigGeneration <= configGeneration || windows.NItems() != 0 {
				return true
			}
			failure = call(control.MethodShow)
			phase++
		case 6:
			if !status.Visible || status.Overlay.Mapped || status.Overlay.InputRegionApplied || windows.NItems() != 0 || len(status.DegradedReasons) != 5 {
				return true
			}
			found := false
			for _, diagnostic := range status.DegradedReasons {
				found = found || diagnostic.Code == "OVERLAY_MONITOR_UNAVAILABLE" && diagnostic.Stage == "overlay"
			}
			if !found {
				failure = errors.New("missing monitor diagnostic was not published")
				break
			}
			requestGeneration = status.Generation
			failure = call(control.MethodShow)
			phase++
		case 7:
			if status.Generation != requestGeneration {
				failure = errors.New("idempotent show changed generation")
				break
			}
			configGeneration = status.LastReload.ConfigGeneration
			failure = os.WriteFile(path, []byte("schema_version = 1\n"), 0600)
			phase++
		}
		if failure != nil {
			cancel()
		}
		return true
	})
	status := runForeground(ctx, path, io.Discard, os.Stderr)
	glib.SourceRemove(source)
	if status != 0 || failure != nil || phase != 10 || gtk.WindowGetToplevels().NItems() != 0 {
		fmt.Fprintf(os.Stderr, "visibility lifecycle: status=%d phase=%d failure=%v windows=%d\n", status, phase, failure, gtk.WindowGetToplevels().NItems())
		return 1
	}
	fmt.Fprintf(os.Stdout, "visibility lifecycle mapped %dx%d closed\n", mappedWidth, mappedHeight)
	return 0
}

func TestRunConfigPlacement(t *testing.T) {
	fixture := newRunFixture(t)
	setRunEnvironment(t, fixture)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(fixture.env, "AZERLAY_RUN_NATIVE_CHILD=config-placement", "AZERLAY_RUN_NATIVE_CONFIG="+fixture.config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil || stderr.Len() != 0 || stdout.String() != "hidden missing rejected restored closed\n" {
		t.Fatalf("config placement: err=%v stdout=%q stderr=%q", err, &stdout, &stderr)
	}
	checkRunOwnerReleased(t, fixture)
}

func runConfigPlacementChild() int {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	path := os.Getenv("AZERLAY_RUN_NATIVE_CONFIG")
	phase := 0
	var failure error
	source := glib.TimeoutAdd(10, func() bool {
		if ctx.Err() != nil {
			return true
		}
		response, err := control.Call(ctx, control.MethodStatus, control.Params{})
		if err != nil {
			failure = err
			cancel()
			return true
		}
		encoded, err := json.Marshal(response.Result)
		var status control.Status
		if err == nil {
			err = json.Unmarshal(encoded, &status)
		}
		if err != nil || response.Error != nil || status.Visible {
			failure = errors.New("invalid runtime status")
			cancel()
			return true
		}
		windows := gtk.WindowGetToplevels()
		var contents string
		switch phase {
		case 0:
			if windows.NItems() != 1 {
				failure = errors.New("initial hidden GTK owner missing")
				cancel()
				return true
			}
			widget := &gtk.Widget{Object: windows.Item(0)}
			if widget.Visible() {
				failure = errors.New("initial GTK window visible")
				cancel()
				return true
			}
			contents = "schema_version = 1\n[overlay]\nmonitor = 'azerlay-missing-monitor'\n"
		case 1:
			if windows.NItems() != 0 {
				return true
			}
			contents = "schema_version = 1\n[overlay]\nanchor = 'invalid'\n"
		case 2:
			if status.LastReload.ConfigFailure == nil {
				return true
			}
			if windows.NItems() != 0 {
				failure = errors.New("rejected config replaced placement")
				cancel()
				return true
			}
			contents = "schema_version = 1\n"
		case 3:
			if windows.NItems() != 1 || status.LastReload.ConfigFailure != nil {
				return true
			}
			phase++
			cancel()
			return true
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			failure = err
			cancel()
			return true
		}
		phase++
		return true
	})
	status := runForeground(ctx, path, io.Discard, os.Stderr)
	glib.SourceRemove(source)
	if status != 0 || failure != nil || phase != 4 || gtk.WindowGetToplevels().NItems() != 0 {
		fmt.Fprintf(os.Stderr, "placement lifecycle: status=%d phase=%d failure=%v\n", status, phase, failure)
		return 1
	}
	fmt.Println("hidden missing rejected restored closed")
	return 0
}
