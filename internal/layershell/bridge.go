package layershell

/*
#cgo pkg-config: gtk4-layer-shell-0 gtk4
#include <gtk4-layer-shell.h>
#include <stdint.h>

static void init_window(uintptr_t address) {
	GtkWindow *window = (GtkWindow *)address;
	gtk_layer_init_for_window(window);
	gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_OVERLAY);
	gtk_layer_set_namespace(window, "azerlay");
	gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
	gtk_layer_set_exclusive_zone(window, -1);
}

static gboolean is_window(uintptr_t address) {
	return gtk_layer_is_layer_window((GtkWindow *)address);
}

static GdkSurface *window_surface(uintptr_t address) {
	return gtk_native_get_surface(GTK_NATIVE((GtkWindow *)address));
}

static void apply_window(uintptr_t address, uintptr_t monitor_address,
	int left, int right, int top, int bottom,
	int left_margin, int right_margin, int top_margin, int bottom_margin) {
	GtkWindow *window = (GtkWindow *)address;
	gtk_layer_set_monitor(window, (GdkMonitor *)monitor_address);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_LEFT, left);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_RIGHT, right);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_TOP, top);
	gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_BOTTOM, bottom);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_LEFT, left_margin);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_RIGHT, right_margin);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_TOP, top_margin);
	gtk_layer_set_margin(window, GTK_LAYER_SHELL_EDGE_BOTTOM, bottom_margin);
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	gdk "github.com/diamondburned/gotk4/pkg/gdk/v4"
	glib "github.com/diamondburned/gotk4/pkg/glib/v2"
	gtk "github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Call from the locked GTK main OS thread before the window is realized.
func Init(window *gtk.Window) error {
	address, err := windowAddress(window)
	if err != nil {
		return err
	}
	C.init_window(C.uintptr_t(address))
	runtime.KeepAlive(window)
	return nil
}

func Supported() bool { return C.gtk_layer_is_supported() != 0 }

func IsWindow(window *gtk.Window) (bool, error) {
	address, err := windowAddress(window)
	if err != nil {
		return false, err
	}
	result := C.is_window(C.uintptr_t(address)) != 0
	runtime.KeepAlive(window)
	return result, nil
}

// Surface keeps a transfer-none reference without gotk4's dynamic marshaler,
// whose uintptr round-trip is incompatible with checkptr.
func Surface(window *gtk.Window) (*gdk.Surface, error) {
	address, err := windowAddress(window)
	if err != nil {
		return nil, err
	}
	object := coreglib.Take(unsafe.Pointer(C.window_surface(C.uintptr_t(address))))
	runtime.KeepAlive(window)
	if object == nil {
		return nil, nil
	}
	return &gdk.Surface{Object: object}, nil
}

// Apply replaces all four anchors and margins, including edges used by a
// previous placement. A nil monitor leaves output selection to the compositor.
func Apply(window *gtk.Window, monitor *gdk.Monitor, p Placement) error {
	address, err := windowAddress(window)
	if err != nil {
		return err
	}
	var monitorAddress uintptr
	if monitor != nil {
		monitorAddress = glib.BaseObject(monitor).Native()
		if monitorAddress == 0 {
			return fmt.Errorf("invalid monitor wrapper")
		}
	}
	C.apply_window(C.uintptr_t(address), C.uintptr_t(monitorAddress),
		C.int(boolInt(p.Left)), C.int(boolInt(p.Right)), C.int(boolInt(p.Top)), C.int(boolInt(p.Bottom)),
		C.int(p.LeftMargin), C.int(p.RightMargin), C.int(p.TopMargin), C.int(p.BottomMargin))
	runtime.KeepAlive(window)
	runtime.KeepAlive(monitor)
	return nil
}

func windowAddress(window *gtk.Window) (uintptr, error) {
	if window == nil {
		return 0, fmt.Errorf("nil window wrapper")
	}
	address := glib.BaseObject(window).Native()
	if address == 0 {
		return 0, fmt.Errorf("invalid window wrapper")
	}
	return address, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
