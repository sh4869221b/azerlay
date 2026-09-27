package overlay

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/layershell"
)

func runNativeChild(mode string) int {
	if mode == "qa" {
		return runNativeQAChild()
	}
	if mode == "renderer" {
		return runNativeRendererChild()
	}
	if mode == "render-qa" {
		return runRenderQAChild()
	}
	if mode == "region-failure" {
		return runRegionFailureChild()
	}
	config := config.Overlay{Anchor: "center"}
	if mode == "invalid-display" {
		_, err := New(config)
		if !errors.Is(err, ErrDisplayUnavailable) {
			fmt.Fprintln(os.Stderr, "unexpected display result:", err)
			return 1
		}
		fmt.Println("display-unavailable")
		return 0
	}
	if mode != "lifecycle" {
		return 2
	}
	w, err := New(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer w.Close()
	var observed []State
	w.SetObserver(func(state State) { observed = append(observed, state) })
	if len(observed) != 1 || observed[0] != (State{}) {
		fmt.Fprintln(os.Stderr, "observer did not publish initial hidden state")
		return 1
	}
	if w.requested || w.widget == nil || w.widget.Visible() {
		fmt.Fprintln(os.Stderr, "initial window was not hidden")
		return 1
	}
	attachTestContent(w.widget)
	if err := w.SetVisible(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	loop := glib.NewMainLoop(nil, false)
	commands := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			commands <- strings.TrimSpace(scanner.Text())
		}
	}()
	phase := 0
	var previous *gtk.Window
	var failure error
	deadline := time.Now().Add(30 * time.Second)
	glib.TimeoutAdd(10, func() bool {
		if time.Now().After(deadline) {
			failure = errors.New("native child timed out")
			loop.Quit()
			return false
		}
		if phase == 0 && mappedLayer(w) {
			if observed[len(observed)-1] != w.State() {
				failure = errors.New("observer missed initial mapped state")
				loop.Quit()
				return false
			}
			fmt.Println("ready")
			phase = 1
		}
		if phase == 2 && !w.State().Mapped && !w.State().InputRegionApplied {
			if observed[len(observed)-1] != w.State() {
				failure = errors.New("observer missed unmap")
				loop.Quit()
				return false
			}
			fmt.Println("hidden")
			phase = 3
		}
		if phase == 6 && mappedLayer(w) {
			if previous == w.widget {
				failure = errors.New("window was not recreated")
				loop.Quit()
				return false
			}
			fmt.Println("restored")
			phase = 7
		}
		if phase == 8 && mappedLayer(w) {
			fmt.Println("explicit")
			phase = 9
		}
		if phase == 10 && mappedLayer(w) && w.monitor != nil && w.placePending == 0 {
			fmt.Println("default-return")
			phase = 11
		}
		select {
		case command := <-commands:
			switch {
			case phase == 1 && command == "continue":
				previous = w.widget
				if err := w.SetVisible(false); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				phase = 2
			case phase == 3 && command == "continue":
				config.Monitor = "azerlay-missing-monitor"
				if err := w.ApplyConfig(config); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				if err := w.SetVisible(true); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				if w.widget != nil || w.surface != nil || w.State() != (State{Diagnostic: "OVERLAY_MONITOR_UNAVAILABLE"}) {
					failure = errors.New("missing selector mapped")
					loop.Quit()
					return false
				}
				if observed[len(observed)-1] != w.State() {
					failure = errors.New("observer missed unavailable monitor")
					loop.Quit()
					return false
				}
				fmt.Println("missing")
				phase = 5
			case phase == 5 && command == "continue":
				config.Monitor = ""
				if err := w.ApplyConfig(config); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				attachTestContent(w.widget)
				phase = 6
			case phase == 7 && command == "continue":
				monitor := w.display.MonitorAtSurface(w.surface)
				if monitor == nil || monitor.Connector() == "" {
					failure = errors.New("mapped output has no connector")
					loop.Quit()
					return false
				}
				config.Monitor = monitor.Connector()
				if err := w.ApplyConfig(config); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				attachTestContent(w.widget)
				phase = 8
			case phase == 9 && command == "continue":
				config.Monitor = ""
				if err := w.ApplyConfig(config); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				attachTestContent(w.widget)
				phase = 10
			case phase == 11 && command == "continue":
				if err := w.SetVisible(false); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				config.Monitor = "azerlay-missing-monitor"
				if err := w.ApplyConfig(config); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				config.Monitor = ""
				if err := w.ApplyConfig(config); err != nil {
					failure = err
					loop.Quit()
					return false
				}
				if w.requested || w.widget == nil || w.widget.Visible() || w.State() != (State{}) {
					failure = errors.New("hidden request became visible")
					loop.Quit()
					return false
				}
				fmt.Println("hidden-return")
				phase = 12
			case phase == 12 && command == "quit":
				w.Close()
				before := len(observed)
				w.selectionChanged()
				if len(observed) != before {
					failure = errors.New("closed observer received an update")
				}
				if w.widget != nil || w.surface != nil || w.monitor != nil {
					failure = errors.New("Close retained live handles")
				}
				fmt.Println("closed")
				loop.Quit()
				return false
			default:
				failure = errors.New("unexpected child command")
				loop.Quit()
				return false
			}
		default:
		}
		return true
	})
	loop.Run()
	if failure != nil {
		fmt.Fprintln(os.Stderr, failure)
		return 1
	}
	return 0
}

func attachTestContent(window *gtk.Window) {
	window.SetDefaultSize(160, 80)
	window.SetChild(gtk.NewLabel("Overlay native test"))
}

func mappedLayer(w *Window) bool {
	if w.surface == nil || !w.surface.Mapped() || w.State() != (State{Mapped: true, InputRegionApplied: true}) {
		return false
	}
	ok, err := layershell.IsWindow(w.widget)
	return err == nil && ok
}

func runRegionFailureChild() int {
	w, err := New(config.Overlay{Anchor: "center"})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer w.Close()
	var observed State
	w.SetObserver(func(state State) { observed = state })
	w.newRegion = func() (*cairo.Region, error) { return nil, errors.New("injected region failure") }
	if err := w.SetVisible(true); err != nil || !w.requested || w.State() != (State{Diagnostic: "ERR_OVERLAY_INPUT_REGION"}) || observed != w.State() || w.widget.Visible() {
		fmt.Fprintln(os.Stderr, "region failure did not preserve hidden request", err, w.State())
		return 1
	}
	fmt.Println("failed")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() || scanner.Text() != "continue" {
		return 1
	}
	w.newRegion = cairo.RegionCreate
	if err := w.SetVisible(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	loop := glib.NewMainLoop(nil, false)
	commands := make(chan string)
	go func() {
		for scanner.Scan() {
			commands <- strings.TrimSpace(scanner.Text())
		}
	}()
	phase := 0
	deadline := time.Now().Add(10 * time.Second)
	failed := false
	glib.TimeoutAdd(10, func() bool {
		if time.Now().After(deadline) {
			failed = true
			loop.Quit()
			return false
		}
		if phase == 0 && mappedLayer(w) {
			fmt.Println("recovered")
			phase = 1
		}
		select {
		case command := <-commands:
			if phase == 1 && command == "quit" {
				w.Close()
				fmt.Println("closed")
				loop.Quit()
				return false
			}
			failed = true
			loop.Quit()
			return false
		default:
		}
		return true
	})
	loop.Run()
	if failed {
		fmt.Fprintln(os.Stderr, "region recovery did not map safely")
		return 1
	}
	return 0
}
