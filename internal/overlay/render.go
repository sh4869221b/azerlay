package overlay

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

func (w *Window) SetDrawObserver(observer func(*renderer.OverlaySnapshot)) { w.drawObserver = observer }

func (w *Window) SetRenderContent(snapshot *renderer.OverlaySnapshot, overlay config.Overlay, appearance config.Appearance, showAmbiguous bool) {
	w.overlay = overlay
	w.SetRenderState(snapshot, appearance, showAmbiguous)
}

// SetRenderState updates the drawing area on the GTK owner thread.
func (w *Window) SetRenderState(snapshot *renderer.OverlaySnapshot, appearance config.Appearance, showAmbiguous bool) {
	if w.closed {
		return
	}
	w.snapshot, w.appearance, w.showAmbiguous = snapshot, appearance, showAmbiguous
	if !w.requested {
		return
	}
	if snapshot == nil {
		w.detachRender()
		return
	}
	w.attachRender()
	w.prepareRender(false)
	w.updateRenderSize()
}

func (w *Window) attachRender() {
	if w.widget == nil || w.snapshot == nil || w.area != nil {
		return
	}
	area := gtk.NewDrawingArea()
	w.area = area
	w.renderWidth, w.renderHeight = 0, 0
	area.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, _, _ int) {
		frame := w.frame
		if frame != nil {
			frame.Draw(cr)
		}
		if w.drawObserver != nil {
			var drawn *renderer.OverlaySnapshot
			if frame != nil {
				drawn = w.preparedSnapshot
			}
			w.drawObserver(drawn)
		}
	})
	w.resizeHandler = area.ConnectResize(func(width, height int) {
		w.renderWidth, w.renderHeight = width, height
		w.prepareRender(false)
	})
	w.scaleHandler = area.Connect("notify::scale-factor", func() { w.prepareRender(true) })
	w.renderSettings = gtk.SettingsGetForDisplay(w.display)
	w.fontHandler = w.renderSettings.Connect("notify::gtk-font-name", func() {
		w.prepareRender(true)
		w.updateRenderSize()
	})
	w.widget.AddCSSClass("azerlay-render-window")
	w.renderCSS = gtk.NewCSSProvider()
	w.renderCSS.LoadFromString("window.azerlay-render-window { background: transparent; }")
	gtk.StyleContextAddProviderForDisplay(w.display, w.renderCSS, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	w.updateRenderSize()
	w.widget.SetChild(area)
}

func (w *Window) updateRenderSize() {
	if !w.requested || w.area == nil || w.snapshot == nil {
		return
	}
	frame := w.frame
	if frame == nil {
		frame = renderer.Prepare(w.snapshot, renderer.NewOptions(w.overlay, w.appearance, w.showAmbiguous), 1, 1)
	}
	naturalWidth, naturalHeight := frame.NaturalSize()
	width, height := int(math.Ceil(naturalWidth)), int(math.Ceil(naturalHeight))
	if w.area.ContentWidth() != width {
		w.area.SetContentWidth(width)
	}
	if w.area.ContentHeight() != height {
		w.area.SetContentHeight(height)
	}
}

func (w *Window) prepareRender(force bool) {
	if !w.requested || w.area == nil || w.snapshot == nil {
		return
	}
	options := renderer.NewOptions(w.overlay, w.appearance, w.showAmbiguous)
	if !force && w.frame != nil && w.preparedSnapshot == w.snapshot && w.preparedOptions == options && w.preparedWidth == w.renderWidth && w.preparedHeight == w.renderHeight {
		return
	}
	w.preparedSnapshot, w.preparedOptions = w.snapshot, options
	w.preparedWidth, w.preparedHeight = w.renderWidth, w.renderHeight
	w.frame = nil
	if w.renderWidth > 0 && w.renderHeight > 0 {
		w.frame = renderer.Prepare(w.snapshot, options, float64(w.renderWidth), float64(w.renderHeight))
	}
	w.area.QueueDraw()
}

func (w *Window) detachRender() {
	if w.area == nil {
		return
	}
	w.area.HandlerDisconnect(w.resizeHandler)
	w.area.HandlerDisconnect(w.scaleHandler)
	w.renderSettings.HandlerDisconnect(w.fontHandler)
	w.area.SetDrawFunc(nil)
	w.frame = nil
	w.preparedSnapshot = nil
	w.renderWidth, w.renderHeight = 0, 0
	if w.widget != nil {
		w.widget.SetChild(nil)
		gtk.StyleContextRemoveProviderForDisplay(w.display, w.renderCSS)
		w.widget.RemoveCSSClass("azerlay-render-window")
	}
	w.renderCSS = nil
	w.renderSettings = nil
	w.area = nil
}
