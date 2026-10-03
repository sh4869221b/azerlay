package live

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/gameprofile"
	"github.com/sh4869221b/azerlay/internal/input"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

type ConfigSource interface {
	Snapshot() config.Snapshot
	Changes() <-chan struct{}
}
type ProfileSource interface {
	ProfileSnapshot() *control.ProfileSnapshot
	Changes() <-chan struct{}
	VisibilityChanges() <-chan struct{}
	RequestedVisible() bool
}
type InputSource interface {
	Latest() input.ManagedSnapshot
	Changes() <-chan struct{}
	Update(config.Device) error
	Close() error
}
type Sources struct {
	Config  ConfigSource
	Profile ProfileSource
	Input   InputSource
}
type Generations struct{ Config, Profile, Device, Layout uint64 }
type View struct {
	Config      config.Snapshot
	Profile     *control.ProfileSnapshot
	Input       input.ManagedSnapshot
	Generations Generations
	Visible     bool
	Render      *renderer.OverlaySnapshot
	Diagnostics []control.Diagnostic
}
type Coordinator struct {
	sources   Sources
	latest    atomic.Pointer[View]
	changes   chan struct{}
	done      chan struct{}
	cancel    context.CancelFunc
	closeOnce sync.Once
	err       error
}

func Start(ctx context.Context, sources Sources) *Coordinator {
	ctx, cancel := context.WithCancel(ctx)
	c := &Coordinator{sources: sources, changes: make(chan struct{}, 1), done: make(chan struct{}), cancel: cancel}
	go c.run(ctx)
	return c
}
func (c *Coordinator) Latest() *View            { return c.latest.Load() }
func (c *Coordinator) Changes() <-chan struct{} { return c.changes }
func (c *Coordinator) Done() <-chan struct{}    { return c.done }
func (c *Coordinator) Close() error {
	c.closeOnce.Do(c.cancel)
	<-c.done
	return c.err
}

func (c *Coordinator) run(ctx context.Context) {
	defer close(c.done)
	defer func() { c.err = c.sources.Input.Close() }()
	var assembly Assembly
	var store gameprofile.Store
	var game gameprofile.Definition
	var definition layout.Definition
	var configured config.Device
	var layoutGeneration, configGeneration, labelsGeneration uint64
	var diagnostics []control.Diagnostic
	for ctx.Err() == nil {
		settings := c.sources.Config.Snapshot()
		if settings.Status.ConfigGeneration != configGeneration {
			configGeneration = settings.Status.ConfigGeneration
			labelsGeneration++
			diagnostics = nil
			if err := store.Reload(""); err != nil {
				diagnostics = append(diagnostics, control.Diagnostic{Code: "GAME_LABELS_UNAVAILABLE", Stage: "labels", Reason: "Game label reload failed; previous labels are retained."})
			}
			game, _ = store.Snapshot().Lookup(settings.Config.Profile.Game)
			if configured.Model != settings.Config.Device.Model || configured.Hand != settings.Config.Device.Hand {
				layoutGeneration++
				var err error
				definition, err = layout.LoadEmbedded(settings.Config.Device.Model, settings.Config.Device.Hand)
				if err != nil {
					definition = layout.Definition{}
				}
			}
			configured = settings.Config.Device
			if definition.Model == "" {
				diagnostics = append(diagnostics, control.Diagnostic{Code: "LAYOUT_UNAVAILABLE", Stage: "layout", Reason: "Selected layout is unavailable."})
			}
			if err := c.sources.Input.Update(configured); err != nil {
				diagnostics = append(diagnostics, control.Diagnostic{Code: "INPUT_CLOSE_FAILED", Stage: "input", Reason: "Previous input reader cleanup failed."})
			}
		}
		selected := c.sources.Profile.ProfileSnapshot()
		raw := c.sources.Input.Latest()
		generations := Generations{Config: settings.Status.ConfigGeneration, Profile: selected.Generation, Device: raw.Snapshot.Generations.Device, Layout: layoutGeneration}
		view := &View{Config: settings, Profile: selected, Input: raw, Generations: generations, Visible: c.sources.Profile.RequestedVisible(), Diagnostics: append([]control.Diagnostic(nil), diagnostics...)}
		view.Render = assembly.Build(AssemblyState{Profile: selected, Definition: definition, LayoutGeneration: layoutGeneration, LabelsGeneration: labelsGeneration, Game: game, Input: raw.Snapshot, ReloadFailed: settings.Status.ConfigFailure.Code != ""})
		currentConfig := c.sources.Config.Snapshot().Status.ConfigGeneration
		currentProfile := c.sources.Profile.ProfileSnapshot().Generation
		currentInput := c.sources.Input.Latest().Snapshot
		if currentConfig != generations.Config || currentProfile != generations.Profile || currentInput.Generations.Device != generations.Device {
			continue
		}
		c.latest.Store(view)
		select {
		case c.changes <- struct{}{}:
		default:
		}
		select {
		case <-ctx.Done():
			return
		case <-c.sources.Config.Changes():
		case <-c.sources.Profile.Changes():
		case <-c.sources.Profile.VisibilityChanges():
		case <-c.sources.Input.Changes():
		}
	}
}
