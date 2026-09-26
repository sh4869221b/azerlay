package overlay

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func runNativeQAChild() int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager, err := config.Start(ctx, os.Getenv("AZERLAY_QA_CONFIG"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { cancel(); <-manager.Done() }()
	dataHome, err := profilesource.ResolveDataHome()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	controller, err := control.NewController(ctx, manager, profilesource.NewImportedSource(dataHome))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	status := func() control.Status {
		return controller.Dispatch(ctx, control.Request{Version: 1, ID: "qa", Method: control.MethodStatus}).Result.(control.Status)
	}
	initialStatus, initialConfig := status(), manager.Snapshot()
	if initialStatus.ActiveProfile == nil {
		fmt.Fprintln(os.Stderr, "QA requires an imported synthetic profile")
		return 1
	}
	settings := initialConfig.Config.Overlay
	w, err := New(settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer w.Close()
	lower := gtk.NewWindow()
	lower.SetTitle("Azerlay QA Receiver")
	lower.SetDecorated(false)
	lower.SetDefaultSize(400, 240)
	button := gtk.NewButtonWithLabel("Azerlay QA Receiver")
	lower.SetChild(button)
	var clicks, scrolls, motions, keys int
	button.ConnectClicked(func() { clicks++ })
	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(_, _ float64) { motions++ })
	button.AddController(motion)
	scroll := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical)
	scroll.ConnectScroll(func(_, _ float64) bool { scrolls++; return false })
	button.AddController(scroll)
	key := gtk.NewEventControllerKey()
	key.ConnectKeyPressed(func(_, _ uint, _ gdk.ModifierType) bool { keys++; return false })
	lower.AddController(key)
	lower.Present()
	defer lower.Destroy()
	regionMode := "empty"
	var contentWindow *gtk.Window
	attach := func() {
		if w.widget != nil && w.widget != contentWindow {
			attachTestContent(w.widget)
			contentWindow = w.widget
		}
	}
	attach()
	changed := w.monitors.ConnectItemsChanged(func(_, _, _ uint) { attach() })
	defer w.monitors.HandlerDisconnect(changed)
	report := func(event string) error {
		current := status()
		retained := reflect.DeepEqual(initialStatus.ActiveProfile, current.ActiveProfile) &&
			reflect.DeepEqual(initialConfig, manager.Snapshot()) && w.selector == settings.Monitor &&
			w.anchor == settings.Anchor && w.marginX == settings.MarginX && w.marginY == settings.MarginY
		if !retained {
			return errors.New("config/profile state changed during native QA")
		}
		_, resolved := currentMonitor(w.monitors, settings.Monitor)
		visible, mapped, width, height := false, false, 0, 0
		if w.widget != nil {
			visible = w.widget.Visible()
		}
		if w.surface != nil {
			mapped, width, height = w.surface.Mapped(), w.surface.Width(), w.surface.Height()
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			Event              string `json:"event"`
			Requested          bool   `json:"requested"`
			Visible            bool   `json:"visible"`
			Mapped             bool   `json:"mapped"`
			InputRegionApplied bool   `json:"input_region_applied"`
			Diagnostic         string `json:"diagnostic"`
			Region             string `json:"region"`
			Resolved           bool   `json:"resolved"`
			Width              int    `json:"width"`
			Height             int    `json:"height"`
			LowerVisible       bool   `json:"lower_visible"`
			LowerActive        bool   `json:"lower_active"`
			Clicks             int    `json:"clicks"`
			Scrolls            int    `json:"scrolls"`
			Motions            int    `json:"motions"`
			Keys               int    `json:"keys"`
			Retained           bool   `json:"state_retained"`
		}{event, w.requested, visible, mapped, w.State().InputRegionApplied, w.State().Diagnostic,
			regionMode, resolved, width, height, lower.Visible(), lower.IsActive(),
			clicks, scrolls, motions, keys, retained})
	}
	commands := make(chan string)
	go func() {
		defer close(commands)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			commands <- scanner.Text()
		}
	}()
	loop := glib.NewMainLoop(nil, false)
	var observed *gdk.Monitor
	var invalidate glib.SignalHandle
	var failure error
	pending := "ready"
	deadline := time.Now().Add(15 * time.Second)
	source := glib.TimeoutAdd(10, func() bool {
		if failure != nil {
			loop.Quit()
			return true
		}
		attach()
		if !sameMonitor(observed, w.monitor) {
			if observed != nil {
				observed.HandlerDisconnect(invalidate)
			}
			observed = w.monitor
			if observed != nil {
				invalidate = observed.ConnectInvalidate(func() { failure = report("invalidated") })
			}
		}
		if pending != "" {
			_, resolved := currentMonitor(w.monitors, settings.Monitor)
			contentReady := mappedLayer(w) && w.surface.Width() > 0 && w.surface.Height() > 0
			if w.placePending != 0 || w.requested && resolved && !contentReady {
				if time.Now().After(deadline) {
					failure = fmt.Errorf("native QA mapping timed out: window_exists=%t owner_closed=%t monitor_selected=%t invalidated=%t", w.widget != nil, w.closed, w.monitor != nil, w.invalidated)
				}
				return true
			}
			failure = report(pending)
			pending = ""
		}
		select {
		case line, ok := <-commands:
			if !ok || line == "quit" {
				w.Close()
				lower.SetVisible(false)
				failure = report("closed")
				loop.Quit()
				return true
			}
			parts := strings.Fields(line)
			if len(parts) == 0 {
				failure = errors.New("empty QA command")
				return true
			}
			switch parts[0] {
			case "place":
				if len(parts) != 5 {
					failure = errors.New("place requires anchor, x, y, monitor")
					return true
				}
				x, xerr := strconv.Atoi(parts[2])
				y, yerr := strconv.Atoi(parts[3])
				if xerr != nil || yerr != nil {
					failure = errors.New("invalid QA margin")
					return true
				}
				settings.Anchor, settings.MarginX, settings.MarginY, settings.Monitor = parts[1], x, y, parts[4]
				if settings.Monitor == "default" {
					settings.Monitor = ""
				}
				failure = w.ApplyConfig(settings)
				attach()
				regionMode = "empty"
			case "show", "hide":
				failure = w.SetVisible(parts[0] == "show")
				regionMode = "empty"
			case "recreate":
				w.invalidated = true
				failure = w.reconcile()
				attach()
				regionMode = "empty"
			case "full":
				if w.surface == nil || !w.surface.Mapped() || w.placePending != 0 {
					failure = errors.New("full requires a mapped surface")
					return true
				}
				w.surface.SetInputRegion(nil)
				w.widget.QueueDraw()
				regionMode = "full"
			case "empty":
				if w.surface != nil && w.surface.Mapped() {
					w.applyInputRegion()
				}
				regionMode = "empty"
			case "state", "count":
			default:
				failure = errors.New("unknown QA command")
			}
			pending, deadline = parts[0], time.Now().Add(15*time.Second)
		default:
		}
		return true
	})
	loop.Run()
	glib.SourceRemove(source)
	if observed != nil {
		observed.HandlerDisconnect(invalidate)
	}
	if failure != nil {
		fmt.Fprintln(os.Stderr, failure)
		return 1
	}
	return 0
}
