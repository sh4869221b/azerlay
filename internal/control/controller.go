package control

import (
	"context"
	"sync"
	"time"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/profilesource"
)

// Controller owns requested visibility and the session-only active profile.
type Controller struct {
	manager           *config.Manager
	source            *profilesource.ImportedSource
	started           time.Time
	imported          bool
	mu                sync.Mutex
	visible           bool
	visibilityChanges chan struct{}
	overlay           *OverlayStatus
	overlayDiagnostic *Diagnostic
	generation        uint64
	active            *activeSelection
	selectionFailure  *Error
	localFailure      *Error
	localWatchFailure *Error
	localEnabled      bool
	localCancel       context.CancelFunc
	localDone         <-chan struct{}
	closed            bool
	selectionMu       sync.Mutex
}

func NewController(ctx context.Context, manager *config.Manager, source *profilesource.ImportedSource) (*Controller, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c := &Controller{manager: manager, source: source, started: time.Now(), generation: 1, visibilityChanges: make(chan struct{}, 1)}
	settings := manager.Snapshot().Config.Profile
	c.imported = settings.Source != "local"
	c.initializeProfile(ctx, settings)
	if err := ctx.Err(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Dispatch accepts a protocol-validated request. Quit returns an intent; the
// socket server owns its response attempt and subsequent transport shutdown.
func (c *Controller) Dispatch(ctx context.Context, request Request) Response {
	if ctx.Err() != nil {
		return FailureResponse(&request.ID, NewError(ERR_CONTROL_UNAVAILABLE))
	}
	switch request.Method {
	case MethodStatus:
		return SuccessResponse(request, c.status())
	case MethodReload:
		generation, err := c.manager.RequestReload(ctx)
		if err != nil {
			return FailureResponse(&request.ID, NewError(ERR_CONTROL_UNAVAILABLE))
		}
		return SuccessResponse(request, ReloadResult{Accepted: true, RequestGeneration: generation})
	case MethodSelect:
		return c.selectProfile(ctx, request)
	case MethodShow, MethodHide, MethodToggle, MethodQuit:
		c.mu.Lock()
		defer c.mu.Unlock()
		if ctx.Err() != nil {
			return FailureResponse(&request.ID, NewError(ERR_CONTROL_UNAVAILABLE))
		}
		visible := c.visible
		switch request.Method {
		case MethodShow:
			visible = true
		case MethodHide:
			visible = false
		case MethodToggle:
			visible = !visible
		case MethodQuit:
			return SuccessResponse(request, QuitResult{Quitting: true})
		}
		if visible != c.visible {
			c.visible = visible
			c.generation++
		}
		select {
		case c.visibilityChanges <- struct{}{}:
		default:
		}
		return SuccessResponse(request, VisibilityResult{Visible: c.visible})
	default:
		return FailureResponse(&request.ID, NewError(ERR_CONTROL_METHOD))
	}
}

func (c *Controller) VisibilityChanges() <-chan struct{} { return c.visibilityChanges }

func (c *Controller) RequestedVisible() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.visible
}

func (c *Controller) SetOverlayStatus(status OverlayStatus, diagnostic *Diagnostic) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.overlay = &status
	if diagnostic == nil {
		c.overlayDiagnostic = nil
	} else {
		copy := *diagnostic
		c.overlayDiagnostic = &copy
	}
}

func (c *Controller) selectProfile(ctx context.Context, request Request) Response {
	if !c.imported {
		return FailureResponse(&request.ID, NewError(ERR_PROFILE_SOURCE_UNAVAILABLE))
	}
	candidate, failure := c.resolve(ctx, request.Params.Selector, false)
	if failure != nil {
		return FailureResponse(&request.ID, failure)
	}
	c.selectionMu.Lock()
	defer c.selectionMu.Unlock()
	c.mu.Lock()
	if ctx.Err() != nil || c.closed {
		c.mu.Unlock()
		return FailureResponse(&request.ID, NewError(ERR_CONTROL_UNAVAILABLE))
	}
	c.localEnabled = false
	cancel, done := c.localCancel, c.localDone
	c.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil {
		return FailureResponse(&request.ID, NewError(ERR_CONTROL_UNAVAILABLE))
	}
	if c.active == nil || c.active.selection != candidate.selection {
		c.active = candidate
		c.generation++
	}
	c.selectionFailure = nil
	c.localFailure = nil
	c.localWatchFailure = nil
	return SuccessResponse(request, SelectionResult{ActiveProfile: c.active.status(), Generation: c.generation})
}

// Close joins local reads and watching before the controller is discarded.
func (c *Controller) Close() {
	c.selectionMu.Lock()
	defer c.selectionMu.Unlock()
	c.mu.Lock()
	c.closed = true
	c.localEnabled = false
	cancel, done := c.localCancel, c.localDone
	c.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
