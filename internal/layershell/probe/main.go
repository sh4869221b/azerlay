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
*/
import "C"

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
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
