package overlay

import (
	"errors"
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/layershell"
)

var (
	ErrDisplayUnavailable = errors.New("wayland display unavailable")
	ErrLayerUnavailable   = errors.New("layer shell unavailable")
)

// Window and all its methods are confined to GTK's locked main OS thread.
type Window struct {
	display        *gdk.Display
	monitors       *gio.ListModel
	listHandler    glib.SignalHandle
	monitorWatches []monitorWatch
	widget         *gtk.Window
	realize        glib.SignalHandle
	unrealize      glib.SignalHandle
	surface        *gdk.Surface
	surfaceHandles []glib.SignalHandle
	monitor        *gdk.Monitor
	monitorHandles []glib.SignalHandle
	selector       string
	anchor         string
	marginX        int
	marginY        int
	requested      bool
	invalidated    bool
	reconciling    bool
	placing        bool
	closed         bool
	pending        glib.SourceHandle
	placePending   glib.SourceHandle
}

func New(overlay config.Overlay) (*Window, error) {
	if err := os.Setenv("GDK_BACKEND", "wayland"); err != nil {
		return nil, ErrDisplayUnavailable
	}
	if !gtk.InitCheck() {
		return nil, ErrDisplayUnavailable
	}
	if !layershell.Supported() {
		return nil, ErrLayerUnavailable
	}
	display := gdk.DisplayGetDefault()
	if display == nil {
		return nil, ErrDisplayUnavailable
	}
	w := &Window{display: display, monitors: display.Monitors()}
	if err := w.ApplyConfig(overlay); err != nil {
		w.Close()
		return nil, err
	}
	w.watchMonitorList()
	w.listHandler = w.monitors.ConnectItemsChanged(func(_, _, _ uint) {
		w.watchMonitorList()
		w.selectionChanged()
	})
	return w, nil
}

func (w *Window) ApplyConfig(overlay config.Overlay) error {
	if _, err := layershell.Place(layershell.PlacementInput{Anchor: overlay.Anchor}); err != nil {
		return err
	}
	w.invalidated = w.invalidated || w.selector != overlay.Monitor
	w.selector, w.anchor = overlay.Monitor, overlay.Anchor
	w.marginX, w.marginY = overlay.MarginX, overlay.MarginY
	return w.reconcile()
}

func (w *Window) SetVisible(visible bool) error {
	w.requested = visible
	return w.reconcile()
}

func (w *Window) Close() {
	if w.closed {
		return
	}
	w.closed = true
	if w.pending != 0 {
		glib.SourceRemove(w.pending)
		w.pending = 0
	}
	if w.placePending != 0 {
		glib.SourceRemove(w.placePending)
		w.placePending = 0
	}
	if w.listHandler != 0 {
		w.monitors.HandlerDisconnect(w.listHandler)
	}
	w.disconnectMonitorList()
	w.disconnectMonitor()
	w.destroyWindow()
}

func (w *Window) reconcile() error {
	if w.closed || w.reconciling {
		return nil
	}
	w.reconciling = true
	defer func() { w.reconciling = false }()
	target, resolved := currentMonitor(w.monitors, w.selector)
	if !resolved {
		w.disconnectMonitor()
		w.destroyWindow()
		return nil
	}
	if w.widget == nil || w.invalidated || (w.selector != "" && !sameMonitor(w.monitor, target)) {
		w.disconnectMonitor()
		w.destroyWindow()
		w.invalidated = false
		if err := w.createWindow(); err != nil {
			return err
		}
	}
	if w.selector != "" {
		w.observeMonitor(target)
	}
	if w.surface != nil && w.surface.Mapped() {
		w.schedulePlace()
	} else if err := w.place(); err != nil {
		return err
	}
	if w.requested {
		if !w.widget.Visible() {
			w.widget.Present()
		}
	} else {
		w.widget.SetVisible(false)
	}
	return nil
}

func (w *Window) createWindow() error {
	w.widget = gtk.NewWindow()
	if err := layershell.Init(w.widget); err != nil {
		w.widget.Destroy()
		w.widget = nil
		return err
	}
	w.realize = w.widget.ConnectRealize(func() {
		w.disconnectSurface()
		surface, err := layershell.Surface(w.widget)
		if err != nil || surface == nil {
			w.widget.SetVisible(false)
			return
		}
		w.surface = surface
		w.surfaceHandles = []glib.SignalHandle{
			surface.Connect("notify::mapped", func() { w.surfaceChanged() }),
			surface.Connect("notify::width", func() { w.schedulePlace() }),
			surface.Connect("notify::height", func() { w.schedulePlace() }),
		}
		w.surfaceChanged()
	})
	w.unrealize = w.widget.ConnectUnrealize(func() { w.disconnectSurface() })
	return nil
}

func (w *Window) surfaceChanged() {
	if w.surface == nil || !w.surface.Mapped() {
		return
	}
	if w.selector == "" {
		w.observeMonitor(w.display.MonitorAtSurface(w.surface))
	}
	w.schedulePlace()
}

func (w *Window) schedulePlace() {
	if w.closed || w.placePending != 0 || w.surface == nil || !w.surface.Mapped() {
		return
	}
	w.placePending = glib.IdleAddPriority(glib.PriorityLow, func() {
		w.placePending = 0
		if w.closed || w.surface == nil || !w.surface.Mapped() {
			return
		}
		if err := w.place(); err != nil {
			w.widget.SetVisible(false)
		}
	})
}

func (w *Window) place() error {
	if w.widget == nil || w.placing || w.surface != nil && !w.surface.Mapped() {
		return nil
	}
	w.placing = true
	defer func() { w.placing = false }()
	input := layershell.PlacementInput{Anchor: w.anchor, MarginX: w.marginX, MarginY: w.marginY}
	if w.surface != nil && w.monitor != nil && w.monitor.IsValid() {
		geometry := w.monitor.Geometry()
		input.MonitorWidth, input.MonitorHeight = geometry.Width(), geometry.Height()
		input.WindowWidth, input.WindowHeight = w.surface.Width(), w.surface.Height()
		input.HasGeometry = input.MonitorWidth > 0 && input.MonitorHeight > 0 && input.WindowWidth > 0 && input.WindowHeight > 0
	}
	placement, err := layershell.Place(input)
	if err != nil {
		return err
	}
	var selected *gdk.Monitor
	if w.selector != "" {
		selected = w.monitor
	}
	return layershell.Apply(w.widget, selected, placement)
}

func (w *Window) disconnectSurface() {
	if w.surface == nil {
		return
	}
	for _, handle := range w.surfaceHandles {
		w.surface.HandlerDisconnect(handle)
	}
	w.surfaceHandles = nil
	w.surface = nil
}

func (w *Window) destroyWindow() {
	if w.widget == nil {
		return
	}
	w.disconnectSurface()
	w.widget.HandlerDisconnect(w.realize)
	w.widget.HandlerDisconnect(w.unrealize)
	w.widget.Destroy()
	w.widget = nil
}
