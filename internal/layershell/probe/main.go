package main

/*
#cgo pkg-config: gtk4-layer-shell-0 gtk4
#include <gtk4-layer-shell.h>
#include <stdint.h>

static void init_layer(uintptr_t address) {
	GtkWindow *window = (GtkWindow *)address;
	gtk_layer_init_for_window(window);
	gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_OVERLAY);
	gtk_layer_set_namespace(window, "azerlay-abi-probe");
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_TOP, TRUE);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_LEFT, TRUE);
}

static gboolean is_layer_window(uintptr_t address) {
	return gtk_layer_is_layer_window((GtkWindow *)address);
}

static void init_placement(uintptr_t address, uintptr_t monitor_address, int horizontal, int vertical, int margin_x, int margin_y, int zone) {
	GtkWindow *window = (GtkWindow *)address;
	gtk_layer_init_for_window(window);
	gtk_layer_set_namespace(window, "azerlay-placement-probe");
	gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_OVERLAY);
	gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
	gtk_layer_set_exclusive_zone(window, zone);
	gtk_layer_set_monitor(window, (GdkMonitor *)monitor_address);
	if (horizontal != 0) {
		gtk_layer_set_anchor(window, horizontal < 0 ? GTK_LAYER_SHELL_EDGE_LEFT : GTK_LAYER_SHELL_EDGE_RIGHT, TRUE);
	}
	if (vertical != 0) {
		gtk_layer_set_anchor(window, vertical < 0 ? GTK_LAYER_SHELL_EDGE_TOP : GTK_LAYER_SHELL_EDGE_BOTTOM, TRUE);
	}
	gtk_layer_set_margin(window, horizontal <= 0 ? GTK_LAYER_SHELL_EDGE_LEFT : GTK_LAYER_SHELL_EDGE_RIGHT, margin_x);
	gtk_layer_set_margin(window, vertical <= 0 ? GTK_LAYER_SHELL_EDGE_TOP : GTK_LAYER_SHELL_EDGE_BOTTOM, margin_y);
}
*/
import "C"

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// All probe callbacks run on the locked GTK main thread. Only Go wrappers and
// signal handles survive a call; native pointer values are never retained.
type probe struct {
	window  *gtk.Window
	surface *gdk.Surface
	handler coreglib.SignalHandle
	region  *cairo.Region
	label   string
	applied bool
	hidden  bool
	err     error
}

func currentSurface(widget *gtk.Widget) (*gdk.Surface, error) {
	native := widget.Native()
	if native == nil {
		return nil, fmt.Errorf("native absent: non-success")
	}
	surface := native.Surface()
	if surface == nil {
		return nil, fmt.Errorf("surface absent: non-success")
	}
	base, ok := surface.(*gdk.Surface)
	if !ok || base == nil {
		return nil, fmt.Errorf("unexpected surface type: non-success")
	}
	return base, nil
}

func (p *probe) disconnect() {
	if p.surface != nil {
		p.surface.HandlerDisconnect(p.handler)
		fmt.Printf("%s handler-disconnected\n", p.label)
		p.surface = nil
	}
}

func (p *probe) observe() {
	mapped := p.surface.Mapped()
	fmt.Printf("%s notify::mapped=%t\n", p.label, mapped)
	if !mapped {
		p.hidden = true
		return
	}
	layer := C.is_layer_window(C.uintptr_t(glib.BaseObject(p.window).Native()))
	runtime.KeepAlive(p.window)
	if layer == 0 {
		p.err = fmt.Errorf("mapped surface is not a Layer Shell window")
		return
	}
	p.surface.SetInputRegion(p.region)
	p.window.QueueDraw()
	runtime.KeepAlive(p.surface)
	runtime.KeepAlive(p.region)
	mode := "empty"
	if p.region == nil {
		mode = "full-reactive (negative control)"
	}
	fmt.Printf("%s mapped input-region=%s\n", p.label, mode)
	p.applied = true
}

func (p *probe) createWindow() {
	p.window = gtk.NewWindow()
	p.window.SetTitle("Azerlay ABI probe")
	p.window.SetDefaultSize(160, 80)
	p.window.SetChild(gtk.NewLabel("Overlay ABI probe"))
	C.init_layer(C.uintptr_t(glib.BaseObject(p.window).Native()))
	runtime.KeepAlive(p.window)
	p.window.ConnectRealize(func() {
		p.disconnect()
		p.surface, p.err = currentSurface(&p.window.Widget)
		if p.err != nil {
			return
		}
		p.handler = p.surface.Connect("notify::mapped", func() { p.observe() })
		fmt.Printf("%s surface-acquired mapped=%t\n", p.label, p.surface.Mapped())
		if p.surface.Mapped() {
			p.observe()
		}
	})
	p.window.ConnectUnrealize(func() { p.disconnect() })
	p.window.Present()
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	os.Exit(run())
}

func run() int {
	if len(os.Args) > 1 && os.Args[1] == "--placement" {
		placement, ok := parsePlacement(os.Args[2:])
		if !ok {
			fmt.Fprintln(os.Stderr, "usage: overlay-abi-probe --placement ANCHOR MARGIN_X MARGIN_Y MONITOR ZONE")
			return 2
		}
		return runPlacement(placement)
	}
	mode := "--lifecycle"
	if len(os.Args) == 2 {
		mode = os.Args[1]
	}
	if len(os.Args) > 2 || (mode != "--smoke" && mode != "--lifecycle" && mode != "--reactive" && mode != "--absent-surface" && mode != "--clicks") {
		fmt.Fprintln(os.Stderr, "usage: overlay-abi-probe [--smoke|--lifecycle|--reactive|--absent-surface|--clicks]")
		return 2
	}
	if !gtk.InitCheck() {
		fmt.Fprintln(os.Stderr, "GTK initialization failed: Wayland display unavailable")
		return 1
	}
	// Exercise both absence guards before Layer Shell initialization or GDK use.
	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	if _, err := currentSurface(&box.Widget); err != nil {
		fmt.Println("before-parent:", err)
	} else {
		fmt.Fprintln(os.Stderr, "unexpected native before parenting")
		return 1
	}
	window := gtk.NewWindow()
	_, err := currentSurface(&window.Widget)
	window.Destroy()
	if err == nil {
		fmt.Fprintln(os.Stderr, "unexpected surface before realization")
		return 1
	}
	fmt.Println("before-realize:", err)
	if mode == "--absent-surface" {
		return 1
	}
	if C.gtk_layer_is_supported() == 0 {
		fmt.Fprintln(os.Stderr, "Layer Shell unavailable on this display")
		return 1
	}
	p := probe{label: "initial"}
	if mode != "--reactive" {
		p.region, err = cairo.RegionCreate()
		if err != nil {
			fmt.Fprintln(os.Stderr, "empty region creation failed:", err)
			return 1
		}
	}
	if mode == "--clicks" {
		return runClicks(&p)
	}
	loop := glib.NewMainLoop(nil, false)
	deadline := time.Now().Add(5 * time.Second)
	phase := "initial"
	complete := false
	glib.TimeoutAdd(250, func() bool {
		if p.err != nil || time.Now().After(deadline) {
			if p.err == nil {
				p.err = fmt.Errorf("lifecycle did not finish within 5s at %s", phase)
			}
			loop.Quit()
			return false
		}
		switch phase {
		case "initial":
			if p.applied {
				if mode == "--smoke" {
					fmt.Println("layer-shell surface mapped")
					complete = true
					loop.Quit()
					return false
				}
				p.applied = false
				p.window.SetVisible(false)
				phase = "hidden"
			}
		case "hidden":
			if p.hidden && !p.surface.Mapped() {
				p.label = "hide-show-remap"
				p.window.Present()
				phase = "remap"
			}
		case "remap":
			if p.applied {
				p.disconnect()
				p.window.Destroy()
				p.applied = false
				p.label = "recreated"
				p.createWindow()
				phase = "recreated"
			}
		case "recreated":
			if p.applied {
				complete = true
				fmt.Println("lifecycle complete")
				loop.Quit()
				return false
			}
		}
		return true
	})
	p.createWindow()
	loop.Run()
	p.disconnect()
	p.window.Destroy()
	if !complete {
		fmt.Fprintln(os.Stderr, p.err)
		return 1
	}
	return 0
}

// Stdin carries only probe commands; all GTK state stays on the main thread.
func runClicks(p *probe) int {
	commands := make(chan string)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(commands)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case commands <- scanner.Text():
			case <-done:
				return
			}
		}
	}()
	lower := gtk.NewWindow()
	lower.SetTitle("Azerlay ABI lower target")
	lower.SetDecorated(false)
	lower.SetDefaultSize(400, 240)
	button := gtk.NewButtonWithLabel("Lower clicks: 0")
	count := 0
	button.ConnectClicked(func() {
		count++
		button.SetLabel(fmt.Sprintf("Lower clicks: %d", count))
		fmt.Printf("lower count=%d\n", count)
	})
	lower.SetChild(button)
	lower.Present()
	defer lower.Destroy()
	loop := glib.NewMainLoop(nil, false)
	empty := p.region
	deadline := time.Now().Add(5 * time.Minute)
	complete := false
	glib.TimeoutAdd(50, func() bool {
		if p.err != nil || time.Now().After(deadline) {
			if p.err == nil {
				p.err = fmt.Errorf("click probe exceeded 5 minutes")
			}
			loop.Quit()
			return false
		}
		select {
		case command, ok := <-commands:
			if !ok || command == "quit" {
				complete = true
				loop.Quit()
				return false
			}
			switch command {
			case "empty", "full":
				p.region = empty
				if command == "full" {
					p.region = nil
				}
				if p.surface != nil {
					p.observe()
				}
			case "hide":
				p.window.SetVisible(false)
			case "show":
				p.label = "hide-show-remap"
				p.window.Present()
			case "recreate":
				p.disconnect()
				p.window.Destroy()
				p.label = "recreated"
				p.createWindow()
			case "count":
				fmt.Printf("lower count=%d\n", count)
			default:
				fmt.Println("unknown probe command")
			}
		default:
		}
		return true
	})
	p.createWindow()
	fmt.Println("commands: count empty full hide show recreate quit; lower count=0")
	loop.Run()
	p.disconnect()
	p.window.Destroy()
	if !complete {
		fmt.Fprintln(os.Stderr, p.err)
		return 1
	}
	return 0
}

type placementArgs struct {
	anchor  string
	marginX int
	marginY int
	monitor string
	zone    int
}

func parsePlacement(args []string) (placementArgs, bool) {
	if len(args) != 5 {
		return placementArgs{}, false
	}
	switch args[0] {
	case "top-left", "top", "top-right", "left", "center", "right", "bottom-left", "bottom", "bottom-right":
	default:
		return placementArgs{}, false
	}
	mx, errX := strconv.Atoi(args[1])
	my, errY := strconv.Atoi(args[2])
	zone, errZone := strconv.Atoi(args[4])
	if errX != nil || errY != nil || errZone != nil || mx < -16384 || mx > 16384 || my < -16384 || my > 16384 || (zone != -1 && zone != 0) || args[3] == "" {
		return placementArgs{}, false
	}
	return placementArgs{args[0], mx, my, args[3], zone}, true
}

func resolveMonitor(monitors *gio.ListModel, selector string) (*gdk.Monitor, string) {
	if selector == "default" {
		return nil, "default"
	}
	var connector, description *gdk.Monitor
	connectorCount, descriptionCount := 0, 0
	for i := uint(0); i < monitors.NItems(); i++ {
		object := monitors.Item(i)
		if object == nil {
			continue
		}
		monitor := &gdk.Monitor{Object: object}
		if monitor.Connector() == selector {
			connector, connectorCount = monitor, connectorCount+1
		} else if monitor.Description() == selector {
			description, descriptionCount = monitor, descriptionCount+1
		}
	}
	if connectorCount == 1 {
		return connector, "connector"
	}
	if connectorCount > 1 || descriptionCount > 1 {
		return nil, "ambiguous selector"
	}
	if descriptionCount == 1 {
		return description, "description"
	}
	return nil, "missing selector"
}

func sameMonitor(a, b *gdk.Monitor) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.IsValid() && glib.BaseObject(a).Native() == glib.BaseObject(b).Native()
}

func placementAxes(anchor string, monitor *gdk.Monitor, x, y int) (int, int, int, int) {
	horizontal, vertical := 0, 0
	if strings.Contains(anchor, "left") {
		horizontal = -1
	} else if strings.Contains(anchor, "right") {
		horizontal = 1
	}
	if strings.Contains(anchor, "top") {
		vertical = -1
	} else if strings.Contains(anchor, "bottom") {
		vertical = 1
	}
	if monitor != nil {
		geometry := monitor.Geometry()
		if horizontal == 0 {
			horizontal, x = -1, (geometry.Width()-160)/2+x
		}
		if vertical == 0 {
			vertical, y = -1, (geometry.Height()-80)/2+y
		}
	}
	return horizontal, vertical, x, y
}

func placementStatus(p *probe, display *gdk.Display, args placementArgs, phase, resolution string) {
	actual := "none"
	if p.surface != nil && p.surface.Mapped() {
		if monitor := display.MonitorAtSurface(p.surface); monitor != nil {
			// The raw GDK description may contain a device serial. Only report its availability.
			actual = fmt.Sprintf("connector=%q description-available=%t", monitor.Connector(), monitor.Description() != "")
		}
	}
	selector := args.monitor
	if selector != "default" {
		selector = "<explicit redacted>"
	}
	fmt.Printf("phase=%s requested=%q resolution=%s mapped=%t actual=%s\n", phase, selector, resolution, p.surface != nil && p.surface.Mapped(), actual)
}

func runPlacement(args placementArgs) int {
	if !gtk.InitCheck() {
		fmt.Fprintln(os.Stderr, "GTK initialization failed: Wayland display unavailable")
		return 1
	}
	if C.gtk_layer_is_supported() == 0 {
		fmt.Fprintln(os.Stderr, "Layer Shell unavailable on this display")
		return 1
	}
	region, err := cairo.RegionCreate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "empty region creation failed:", err)
		return 1
	}
	display := gdk.DisplayGetDefault()
	monitors := display.Monitors()
	p := probe{label: "initial", region: region}
	commands := make(chan string)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer close(commands)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case commands <- scanner.Text():
			case <-done:
				return
			}
		}
	}()
	var target *gdk.Monitor
	create := func(monitor *gdk.Monitor) {
		p.window = gtk.NewWindow()
		p.window.SetTitle("Azerlay placement probe")
		p.window.SetDefaultSize(160, 80)
		p.window.SetChild(gtk.NewLabel("Placement probe"))
		h, v, mx, my := placementAxes(args.anchor, monitor, args.marginX, args.marginY)
		var nativeMonitor C.uintptr_t
		if monitor != nil {
			nativeMonitor = C.uintptr_t(glib.BaseObject(monitor).Native())
		}
		C.init_placement(C.uintptr_t(glib.BaseObject(p.window).Native()), nativeMonitor, C.int(h), C.int(v), C.int(mx), C.int(my), C.int(args.zone))
		runtime.KeepAlive(p.window)
		runtime.KeepAlive(monitor)
		p.window.ConnectRealize(func() {
			p.disconnect()
			p.surface, p.err = currentSurface(&p.window.Widget)
			if p.err != nil {
				return
			}
			p.handler = p.surface.Connect("notify::mapped", func() { p.observe() })
			fmt.Printf("%s surface-acquired mapped=%t\n", p.label, p.surface.Mapped())
			if p.surface.Mapped() {
				p.observe()
			}
		})
		p.window.ConnectUnrealize(func() { p.disconnect() })
		p.window.Present()
		target = monitor
	}
	destroy := func() {
		if p.window != nil {
			p.disconnect()
			p.window.Destroy()
			p.window = nil
			p.applied = false
		}
	}
	defer destroy()
	loop := glib.NewMainLoop(nil, false)
	deadline := time.Now().Add(5 * time.Minute)
	lastResolution := ""
	userHidden := false
	reported := false
	complete := false
	glib.TimeoutAdd(50, func() bool {
		if p.err != nil || time.Now().After(deadline) {
			if p.err == nil {
				p.err = fmt.Errorf("placement probe exceeded 5 minutes")
			}
			loop.Quit()
			return false
		}
		monitor, resolution := resolveMonitor(monitors, args.monitor)
		if resolution != lastResolution || (p.window != nil && !sameMonitor(target, monitor)) {
			destroy()
			target = nil
			reported = false
			lastResolution = resolution
			placementStatus(&p, display, args, "selection", resolution)
		}
		if !userHidden && p.window == nil && (monitor != nil || resolution == "default") {
			create(monitor)
		}
		if p.applied && !reported {
			placementStatus(&p, display, args, "mapped", resolution)
			reported = true
		}
		select {
		case command, ok := <-commands:
			if !ok || command == "quit" {
				complete = true
				loop.Quit()
				return false
			}
			switch command {
			case "status":
				placementStatus(&p, display, args, "status", resolution)
			case "hide":
				userHidden = true
				if p.window != nil {
					p.window.SetVisible(false)
					p.applied = false
				}
				placementStatus(&p, display, args, "hidden", resolution)
			case "show":
				userHidden = false
				p.label = "hide-show-remap"
				reported = false
				if p.window != nil {
					p.window.Present()
				}
			case "recreate":
				destroy()
				p.label = "recreated"
				reported = false
			default:
				fmt.Println("unknown probe command")
			}
		default:
		}
		return true
	})
	fmt.Println("commands: status hide show recreate quit")
	loop.Run()
	if !complete {
		fmt.Fprintln(os.Stderr, p.err)
		return 1
	}
	return 0
}
