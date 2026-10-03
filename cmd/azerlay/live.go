package main

import (
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/live"
	"github.com/sh4869221b/azerlay/internal/overlay"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

func watchLive(coordinator *live.Coordinator, server *control.Server, window *overlay.Window, loop *glib.MainLoop) func() error {
	window.SetDrawObserver(coordinator.Metrics().Draw)
	stop, done := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var pending glib.SourceHandle
	var timer bool
	var appliedOverlay config.Overlay
	var appliedVisible bool
	var appliedRender *renderer.OverlaySnapshot
	var appliedOptions renderer.Options
	var coalescer live.Coalescer
	var applyError error
	var apply func()
	apply = func() {
		mu.Lock()
		defer mu.Unlock()
		pending, timer = 0, false
		select {
		case <-server.Done():
			loop.Quit()
			return
		default:
		}
		view := coordinator.Latest()
		if view == nil {
			return
		}
		settings := view.Config.Config
		if !view.Visible && appliedVisible {
			if err := window.SetVisible(false); err != nil {
				applyError = err
				loop.Quit()
				return
			}
		}
		if appliedOverlay != settings.Overlay {
			if err := window.ApplyPlacement(settings.Overlay); err != nil {
				applyError = err
				loop.Quit()
				return
			}
			appliedOverlay = settings.Overlay
		}
		options := renderer.NewOptions(settings.Overlay, settings.Appearance, settings.Input.ShowAmbiguous)
		dirty := appliedRender != view.Render || appliedOptions != options
		if !view.Visible || view.Visible != appliedVisible {
			window.SetRenderContent(view.Render, settings.Overlay, settings.Appearance, settings.Input.ShowAmbiguous)
			if err := window.SetVisible(view.Visible); err != nil {
				applyError = err
				loop.Quit()
				return
			}
			appliedRender = view.Render
			appliedVisible = view.Visible
			appliedOptions = options
			if view.Visible {
				coalescer.Applied(time.Now())
			}
			return
		}
		if !dirty {
			return
		}
		if delay := coalescer.Delay(time.Now(), settings.Input.RefreshHz); delay > 0 {
			timer = true
			pending = glib.TimeoutAdd(uint(delay/time.Millisecond), func() bool { apply(); return false })
			return
		}
		window.SetRenderContent(view.Render, settings.Overlay, settings.Appearance, settings.Input.ShowAmbiguous)
		appliedRender = view.Render
		coalescer.Applied(time.Now())
		appliedOptions = options
	}
	go func() {
		defer close(done)
		for {
			quitting := false
			select {
			case <-stop:
				return
			case <-server.Done():
				quitting = true
			case <-coordinator.Changes():
			}
			mu.Lock()
			view := coordinator.Latest()
			immediate := quitting || view != nil && (view.Visible != appliedVisible || view.Config.Config.Overlay != appliedOverlay)
			if timer && immediate {
				glib.SourceRemove(pending)
				pending, timer = 0, false
			}
			if pending == 0 {
				pending = glib.IdleAdd(apply)
			}
			mu.Unlock()
			if quitting {
				return
			}
		}
	}()
	return func() error {
		close(stop)
		<-done
		mu.Lock()
		defer mu.Unlock()
		if pending != 0 {
			glib.SourceRemove(pending)
		}
		window.SetObserver(nil)
		window.SetDrawObserver(nil)
		return applyError
	}
}
