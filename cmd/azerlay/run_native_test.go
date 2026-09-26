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
	default:
		return 2
	}
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
