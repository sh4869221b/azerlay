package overlay

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

type monitorLabel struct {
	connector   string
	description string
}

type monitorWatch struct {
	monitor *gdk.Monitor
	handles []glib.SignalHandle
}

func (w *Window) watchMonitorList() {
	w.disconnectMonitorList()
	for i := uint(0); i < w.monitors.NItems(); i++ {
		object := w.monitors.Item(i)
		if object == nil {
			continue
		}
		monitor := &gdk.Monitor{Object: object}
		w.monitorWatches = append(w.monitorWatches, monitorWatch{monitor, []glib.SignalHandle{
			monitor.Connect("notify::connector", w.selectionChanged),
			monitor.Connect("notify::description", w.selectionChanged),
			monitor.Connect("notify::valid", w.selectionChanged),
		}})
	}
}

func (w *Window) disconnectMonitorList() {
	for _, watch := range w.monitorWatches {
		for _, handle := range watch.handles {
			watch.monitor.HandlerDisconnect(handle)
		}
	}
	w.monitorWatches = nil
}

func (w *Window) selectionChanged() {
	if err := w.reconcile(); err != nil {
		w.Close()
	}
}

func selectMonitor(selector string, labels []monitorLabel) (int, bool) {
	if selector == "" {
		return -1, true
	}
	connector, connectorCount := -1, 0
	for i, label := range labels {
		if label.connector == selector {
			connector, connectorCount = i, connectorCount+1
		}
	}
	if connectorCount != 0 {
		if connectorCount == 1 {
			return connector, true
		}
		return -1, false
	}
	description, descriptionCount := -1, 0
	for i, label := range labels {
		if label.description == selector {
			description, descriptionCount = i, descriptionCount+1
		}
	}
	if descriptionCount == 1 {
		return description, true
	}
	return -1, false
}

func currentMonitor(model *gio.ListModel, selector string) (*gdk.Monitor, bool) {
	if selector == "" {
		return nil, true
	}
	var monitors []*gdk.Monitor
	var labels []monitorLabel
	for i := uint(0); i < model.NItems(); i++ {
		object := model.Item(i)
		if object == nil {
			continue
		}
		monitor := &gdk.Monitor{Object: object}
		if !monitor.IsValid() {
			continue
		}
		monitors = append(monitors, monitor)
		labels = append(labels, monitorLabel{monitor.Connector(), monitor.Description()})
	}
	index, ok := selectMonitor(selector, labels)
	if !ok {
		return nil, false
	}
	return monitors[index], true
}

func sameMonitor(a, b *gdk.Monitor) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return glib.ObjectEq(a, b)
}

func (w *Window) observeMonitor(monitor *gdk.Monitor) {
	if sameMonitor(w.monitor, monitor) {
		return
	}
	w.disconnectMonitor()
	if monitor == nil {
		return
	}
	w.monitor = monitor
	w.monitorHandles = []glib.SignalHandle{
		monitor.ConnectInvalidate(func() {
			if w.widget != nil {
				w.widget.SetVisible(false)
			}
			w.disconnectSurface()
			w.invalidated = true
			if w.pending == 0 {
				w.pending = glib.IdleAdd(func() {
					w.pending = 0
					if err := w.reconcile(); err != nil {
						w.Close()
					}
				})
			}
		}),
		monitor.Connect("notify::geometry", func() { w.schedulePlace() }),
		monitor.Connect("notify::scale", func() { w.schedulePlace() }),
	}
}

func (w *Window) disconnectMonitor() {
	if w.monitor == nil {
		return
	}
	for _, handle := range w.monitorHandles {
		w.monitor.HandlerDisconnect(handle)
	}
	w.monitorHandles = nil
	w.monitor = nil
}
