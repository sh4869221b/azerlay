package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/control"
	"github.com/sh4869221b/azerlay/internal/device"
	"github.com/sh4869221b/azerlay/internal/input"
	"github.com/sh4869221b/azerlay/internal/layershell"
	"github.com/sh4869221b/azerlay/internal/live"
	"github.com/sh4869221b/azerlay/internal/overlay"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/profilesource"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

func TestRunLiveOverlayNative(t *testing.T) {
	for _, mode := range []string{"imported", "local", "missing"} {
		t.Run(mode, func(t *testing.T) { testLiveOverlayChild(t, "live-"+mode) })
	}
}

func TestRunLiveOverlayLatency(t *testing.T) { testLiveOverlayChild(t, "live-latency") }

func testLiveOverlayChild(t *testing.T, mode string) {
	t.Helper()
	fixture := newRunFixture(t)
	setRunEnvironment(t, fixture)
	dir := t.TempDir()
	if evidence := os.Getenv("AZERLAY_TEST_LIVE_EVIDENCE"); evidence != "" {
		var err error
		dir, err = filepath.Abs(evidence)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(fixture.env, "AZERLAY_RUN_NATIVE_CHILD="+mode, "AZERLAY_RUN_NATIVE_CONFIG="+fixture.config, "AZERLAY_TEST_LIVE_EVIDENCE="+dir)
	cmd.Env = append(cmd.Env, "AZERLAY_TEST_LIVE_MONITOR="+os.Getenv("AZERLAY_TEST_LIVE_MONITOR"))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil || stderr.Len() != 0 || !strings.Contains(stdout.String(), "closed reader coordinator window socket") {
		t.Fatalf("%s: err=%v stdout=%s stderr=%s", mode, err, &stdout, &stderr)
	}
	t.Log(strings.TrimSpace(stdout.String()))
	checkRunOwnerReleased(t, fixture)
}

type nativePipeInput struct {
	mu                                sync.Mutex
	ctx                               context.Context
	session                           *input.Session
	writer                            *os.File
	changes                           chan struct{}
	forwardDone                       chan struct{}
	generation, events, invalidations uint64
}

func (p *nativePipeInput) reconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		p.writer.Close()
		p.session.Close()
		<-p.forwardDone
		previous := p.session.Latest()
		p.events += previous.EventCount
		p.invalidations += previous.InvalidationCount
	}
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	p.generation++
	session, err := input.StartWithOpener(p.ctx, device.Group{}, input.Generations{Device: p.generation}, func(device.Group) ([]device.OpenedNode, error) {
		return []device.OpenedNode{{File: r}}, nil
	})
	if err != nil {
		r.Close()
		w.Close()
		return err
	}
	p.session, p.writer, p.forwardDone = session, w, make(chan struct{})
	go func(done chan struct{}) {
		defer close(done)
		for {
			select {
			case <-session.Changes():
				p.notify()
			case <-session.Done():
				p.notify()
				return
			}
		}
	}(p.forwardDone)
	p.notify()
	return nil
}
func (p *nativePipeInput) notify() {
	select {
	case p.changes <- struct{}{}:
	default:
	}
}
func (p *nativePipeInput) Changes() <-chan struct{}   { return p.changes }
func (p *nativePipeInput) Update(config.Device) error { return nil }
func (p *nativePipeInput) Latest() input.ManagedSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	snapshot := p.session.Latest()
	snapshot.EventCount += p.events
	snapshot.InvalidationCount += p.invalidations
	state := input.ManagedConnected
	if !snapshot.Connected {
		state = input.ManagedDegraded
	}
	return input.ManagedSnapshot{Snapshot: snapshot, State: state, Device: "synthetic-pipe"}
}
func (p *nativePipeInput) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writer.Close()
	p.session.Close()
	<-p.forwardDone
	return nil
}
func (p *nativePipeInput) write(id, state, counter byte) error {
	report := make([]byte, 64)
	report[2], report[3], report[6], report[7], report[8] = 57, counter, 2, id, state
	p.mu.Lock()
	defer p.mu.Unlock()
	_, err := p.writer.Write(report)
	return err
}

func nativeLiveProfile() (string, error) {
	data, err := os.ReadFile("../../internal/profileadapter/testdata/unbound.input.json")
	if err != nil {
		return "", err
	}
	var fixture struct {
		Inputs []map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		return "", err
	}
	unbound := fixture.Inputs[0]
	unbound["id"] = json.RawMessage("8")
	delete(unbound, "pinOne")
	delete(unbound, "pinTwo")
	delete(unbound, "label")
	encoded, err := json.Marshal(unbound)
	if err != nil {
		return "", err
	}
	keyboard := `"types":["1","11","11"],"keyValues":["KeyU","0","0","0"],"metaValues":["0","0","0"],"keyValuesLong":["0","0","0","0"],"metaValuesLong":["0","0","0"],"keyValuesDouble":["0","0","0","0"],"metaValuesDouble":["0","0","0"],"isHold":false,"isTurbo":false,"isToggleOnHold":false`
	return `{"id":"native","name":"Synthetic live","version":1,"inputs":[{"id":4,` + keyboard + `},{"id":3,` + keyboard + `},` + string(encoded) + `,{"id":12,"types":["999","11","11"]}],"isSoftware":true,"metaData":{"changedLogs":[{"softwareVersion":"2.0.2"}]}}`, nil
}

func runLiveOverlayChild(mode string) int {
	ctx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	path, dir := os.Getenv("AZERLAY_RUN_NATIVE_CONFIG"), os.Getenv("AZERLAY_TEST_LIVE_EVIDENCE")
	profileJSON, err := nativeLiveProfile()
	if err != nil {
		return liveChildFailure(err)
	}
	local := mode == "live-local"
	store := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "local")
	profilePath := filepath.Join(store, "Storage/DevicesStorage/device/ProfileStorage/profile_native.json")
	sourceSettings := "source='imported'\n"
	if local {
		if err := os.MkdirAll(filepath.Dir(profilePath), 0700); err != nil {
			return liveChildFailure(err)
		}
		if err := os.WriteFile(profilePath, []byte(profileJSON), 0600); err != nil {
			return liveChildFailure(err)
		}
		sourceSettings = fmt.Sprintf("source='local'\nlocal_store_path=%q\nlocal_device='device'\nlocal_profile_file='profile_native.json'\n", store)
	}
	monitor := os.Getenv("AZERLAY_TEST_LIVE_MONITOR")
	settings := func(hz int, output string) string {
		return fmt.Sprintf("schema_version=1\n[profile]\n%sgame='native-live'\n[overlay]\nmonitor=%q\nanchor='center'\nmargin_x=0\nmargin_y=0\nopacity=1\nmode='detailed'\nshow_unbound=true\n[input]\nrefresh_hz=%d\n", sourceSettings, output, hz)
	}
	if err := os.WriteFile(path, []byte(settings(60, monitor)), 0600); err != nil {
		return liveChildFailure(err)
	}
	games := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "azerlay/games")
	if err := os.MkdirAll(games, 0700); err != nil {
		return liveChildFailure(err)
	}
	if err := os.WriteFile(filepath.Join(games, "native.toml"), []byte("schema_version=1\nid='native-live'\nname='Synthetic labels'\n[controls]\n'input:4:single'='Synthetic action'\n"), 0600); err != nil {
		return liveChildFailure(err)
	}
	manager, err := config.Start(ctx, path)
	if err != nil {
		return liveChildFailure(err)
	}
	defer func() { cancel(); <-manager.Done() }()
	source := profilesource.NewImportedSource(os.Getenv("XDG_DATA_HOME"))
	if !local && mode != "live-missing" {
		prepared, err := profilesource.PrepareText(profileJSON, profile.SourceMetadata{SoftwareRelease: "2.0.2", SourceScope: "azeron-software-export"})
		if err != nil {
			return liveChildFailure(err)
		}
		if _, err := source.Import(ctx, prepared, 1, profilesource.Origin{Kind: "text"}); err != nil {
			return liveChildFailure(err)
		}
	}
	controller, err := control.NewController(ctx, manager, source)
	if err != nil {
		return liveChildFailure(err)
	}
	defer controller.Close()
	if selected := controller.ProfileSnapshot().Profile; selected != nil {
		if len(selected.Controls) != 4 {
			return liveChildFailure(errors.New("synthetic control count"))
		}
		for i, kind := range []profile.BindingKind{profile.BindingKeyboard, profile.BindingKeyboard, profile.BindingUnbound, profile.BindingUnknown} {
			if selected.Controls[i].Bindings[0].Kind != kind {
				return liveChildFailure(fmt.Errorf("synthetic assignment %d not admitted as %s", i, kind))
			}
		}
	}
	window, err := overlay.New(manager.Snapshot().Config.Overlay)
	if err != nil {
		return liveChildFailure(err)
	}
	defer window.Close()
	defer func() {
		if liveNativeBackground != nil {
			liveNativeBackground.Destroy()
		}
	}()
	window.SetObserver(func(state overlay.State) {
		controller.SetOverlayStatus(control.OverlayStatus{Mapped: state.Mapped, InputRegionApplied: state.InputRegionApplied}, nil)
	})
	raw := &nativePipeInput{ctx: ctx, changes: make(chan struct{}, 1)}
	if err := raw.reconnect(); err != nil {
		return liveChildFailure(err)
	}
	coordinator := live.Start(ctx, live.Sources{Config: manager, Profile: controller, Input: raw})
	defer coordinator.Close()
	coordinator.Metrics().EnableMeasurement(400)
	controller.SetLiveStatusProvider(coordinator.Status)
	owner, existing, err := control.AcquireInstance(ctx)
	if err != nil || existing != nil {
		return liveChildFailure(fmt.Errorf("instance: %v", err))
	}
	defer owner.Close()
	server, err := owner.Start(ctx, controller, func() { coordinator.Close(); controller.Close(); cancel(); <-manager.Done() })
	if err != nil {
		return liveChildFailure(err)
	}
	defer server.Close()
	loop := glib.NewMainLoop(nil, false)
	stopBridge := watchLive(coordinator, server, window, loop)
	defer func() { stopBridge() }()
	var drawn atomic.Pointer[renderer.OverlaySnapshot]
	var requests atomic.Uint64
	var requestTimes []time.Time
	window.SetDrawObserver(func(snapshot *renderer.OverlaySnapshot) { coordinator.Metrics().Draw(snapshot); drawn.Store(snapshot) })
	window.SetRenderRequestObserver(func() { requests.Add(1); requestTimes = append(requestTimes, time.Now()) })
	defer window.SetRenderRequestObserver(nil)
	result := make(chan error, 1)
	go func() {
		err := exerciseLiveOverlay(ctx, mode, path, dir, profilePath, profileJSON, settings, controller, coordinator, raw, window, &drawn, &requests, &requestTimes)
		result <- err
		if err != nil {
			cancel()
		}
	}()
	loop.Run()
	if err := <-result; err != nil {
		return liveChildFailure(err)
	}
	if err := server.Close(); err != nil {
		return liveChildFailure(err)
	}
	if err := stopBridge(); err != nil {
		return liveChildFailure(err)
	}
	stopBridge = func() error { return nil }
	window.SetRenderRequestObserver(nil)
	window.Close()
	if liveNativeBackground != nil {
		liveNativeBackground.Destroy()
		liveNativeBackground = nil
	}
	select {
	case <-coordinator.Done():
	default:
		return liveChildFailure(errors.New("coordinator not joined"))
	}
	select {
	case <-raw.session.Done():
	default:
		return liveChildFailure(errors.New("reader not joined"))
	}
	if gtk.WindowGetToplevels().NItems() != 0 {
		return liveChildFailure(errors.New("window survived shutdown"))
	}
	fmt.Println("closed reader coordinator window socket")
	return 0
}

func liveChildFailure(err error) int { fmt.Fprintln(os.Stderr, err); return 1 }

func awaitLive(ctx context.Context, condition func() bool) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for !condition() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}

func liveGTK(ctx context.Context, call func() error) error {
	result := make(chan error, 1)
	glib.IdleAdd(func() { result <- call() })
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

var liveNativeBackground *gtk.Window

func liveOverlayWidget() (*gtk.Window, error) {
	windows := gtk.WindowGetToplevels()
	for i := uint(0); i < windows.NItems(); i++ {
		object := windows.Item(i)
		widget := &gtk.Window{Object: object, Widget: gtk.Widget{Object: object}}
		if widget.Title() != "Azerlay live QA background" {
			return widget, nil
		}
	}
	return nil, errors.New("overlay window unavailable")
}

func liveCapture(path string) ([]byte, error) {
	widget, err := liveOverlayWidget()
	if err != nil {
		return nil, err
	}
	if !widget.Mapped() {
		return nil, errors.New("capture before map")
	}
	surface, err := layershell.Surface(widget)
	if err != nil {
		return nil, err
	}
	monitor := widget.Widget.Display().MonitorAtSurface(surface)
	if liveNativeBackground == nil || !liveNativeBackground.Mapped() {
		return nil, errors.New("owned capture background not mapped")
	}
	command := exec.Command("grim", "-o", monitor.Connector(), "-")
	encoded, err := command.Output()
	if err != nil {
		return nil, err
	}
	full, err := png.Decode(bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	scale := monitor.ScaleFactor()
	w, h := widget.Width()*scale, widget.Height()*scale
	x, y := (full.Bounds().Dx()-w)/2, (full.Bounds().Dy()-h)/2
	crop := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(crop, crop.Bounds(), full, image.Pt(x, y), draw.Src)
	var result bytes.Buffer
	if err := png.Encode(&result, crop); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, result.Bytes(), 0600); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func exerciseLiveOverlay(ctx context.Context, mode, path, dir, profilePath, profileJSON string, settings func(int, string) string, controller *control.Controller, coordinator *live.Coordinator, raw *nativePipeInput, window *overlay.Window, drawn *atomic.Pointer[renderer.OverlaySnapshot], requests *atomic.Uint64, requestTimes *[]time.Time) error {
	call := func(method control.Method) error {
		response, err := control.Call(ctx, method, control.Params{})
		if err != nil {
			return err
		}
		if response.Error != nil {
			return response.Error
		}
		return nil
	}
	currentDraw := func() bool {
		view := coordinator.Latest()
		return view != nil && view.Visible && drawn.Load() == view.Render
	}
	if err := call(control.MethodShow); err != nil {
		return err
	}
	if err := awaitLive(ctx, currentDraw); err != nil {
		return fmt.Errorf("first draw: %w", err)
	}
	if err := liveGTK(ctx, func() error {
		widget, err := liveOverlayWidget()
		if err != nil {
			return err
		}
		surface, err := layershell.Surface(widget)
		if err != nil {
			return err
		}
		monitor := widget.Widget.Display().MonitorAtSurface(surface)
		geometry := monitor.Geometry()
		fmt.Printf("output=%s geometry=%dx%d scale=%d refresh_millihz=%d GTK=%d.%d.%d backend=Wayland renderer=%s\n", monitor.Connector(), geometry.Width(), geometry.Height(), monitor.ScaleFactor(), monitor.RefreshRate(), gtk.GetMajorVersion(), gtk.GetMinorVersion(), gtk.GetMicroVersion(), os.Getenv("GSK_RENDERER"))
		if mode != "live-latency" {
			liveNativeBackground = gtk.NewWindow()
			liveNativeBackground.SetTitle("Azerlay live QA background")
			liveNativeBackground.SetDecorated(false)
			area := gtk.NewDrawingArea()
			area.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, _, _ int) { cr.SetSourceRGB(.03, .04, .05); cr.Paint() })
			liveNativeBackground.SetChild(area)
			liveNativeBackground.FullscreenOnMonitor(monitor)
			liveNativeBackground.Present()
		}
		return nil
	}); err != nil {
		return err
	}
	if mode != "live-latency" {
		time.Sleep(150 * time.Millisecond)
	}
	if selected := controller.ProfileSnapshot(); mode == "live-missing" {
		if selected.Profile != nil {
			return errors.New("missing profile unexpectedly selected")
		}
	} else if selected.Profile == nil || selected.Source.SourceScope != map[bool]string{true: "azeron-software-local-json", false: "azeron-software-export"}[mode == "live-local"] {
		return errors.New("selected source provenance mismatch")
	}
	var counter byte
	send := func(id, state byte) error {
		counter++
		before := raw.Latest().Snapshot.Sequence
		if err := raw.write(id, state, counter); err != nil {
			return err
		}
		return awaitLive(ctx, func() bool {
			view := coordinator.Latest()
			return view.Input.Snapshot.Sequence > before && drawn.Load() == view.Render
		})
	}
	capture := func(name string) ([]byte, error) {
		var image []byte
		err := liveGTK(ctx, func() error {
			var err error
			image, err = liveCapture(filepath.Join(dir, mode+"-"+name+".png"))
			return err
		})
		return image, err
	}
	if mode == "live-latency" {
		for i := 0; i < 225; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(24 * time.Millisecond):
			}
			if err := send(4, byte(i%2)); err != nil {
				return err
			}
		}
		measurement := coordinator.Metrics().Measurement()
		if len(measurement.Samples) < 200 {
			return fmt.Errorf("corresponding samples=%d, need 200", len(measurement.Samples))
		}
		if err := writeLiveTiming(filepath.Join(dir, "latency-60hz.csv"), measurement); err != nil {
			return err
		}
		fmt.Printf("refresh_hz=60 synthetic pipe read-to-actual-GTK-draw; GSK_RENDERER=%s; physical presentation unmeasured\n", os.Getenv("GSK_RENDERER"))
	} else {
		unknown, err := capture("initial-unknown")
		if err != nil {
			return err
		}
		if err := send(4, 1); err != nil {
			return err
		}
		pressed, err := capture("pressed")
		if err != nil {
			return err
		}
		if err := send(4, 0); err != nil {
			return err
		}
		released, err := capture("released")
		if err != nil {
			return err
		}
		if bytes.Equal(pressed, released) || bytes.Equal(unknown, pressed) {
			return errors.New("GTK press/release/unknown pixels did not change")
		}
		for _, id := range []byte{3, 8, 12} {
			if err := send(id, 1); err != nil {
				return err
			}
			if state := coordinator.Latest().Input.Snapshot.Physical(map[byte]string{3: "grid.c1.r2", 8: "grid.c2.r1", 12: "grid.c3.r1"}[id]); !state.Known || !state.Down {
				return errors.New("raw highlight suppressed by assignment")
			}
		}
		before := raw.Latest().Snapshot.InvalidationCount
		if err := send(4, 2); err != nil {
			return err
		}
		if state := coordinator.Latest().Input.Snapshot; state.InvalidationCount != before+1 || state.Physical("grid.c1.r1").Known {
			return errors.New("malformed did not invalidate")
		}
		if _, err := capture("malformed-unknown"); err != nil {
			return err
		}
		if err := send(8, 1); err != nil {
			return err
		}
		counter += 2
		if err := send(4, 1); err != nil {
			return err
		}
		if state := coordinator.Latest().Input.Snapshot; state.Reason != input.ERR_INPUT_DROPPED || state.Physical("grid.c2.r1").Known {
			return errors.New("counter discontinuity retained old state")
		}
		raw.mu.Lock()
		raw.writer.Close()
		raw.mu.Unlock()
		if err := awaitLive(ctx, func() bool {
			return !coordinator.Latest().Input.Snapshot.Connected && drawn.Load() == coordinator.Latest().Render
		}); err != nil {
			return err
		}
		if _, err := capture("disconnected"); err != nil {
			return err
		}
		generation := coordinator.Latest().Generations.Device
		if err := raw.reconnect(); err != nil {
			return err
		}
		if err := awaitLive(ctx, func() bool {
			view := coordinator.Latest()
			return view.Generations.Device > generation && view.Input.Snapshot.Connected && !view.Input.Snapshot.Physical("grid.c1.r1").Known && drawn.Load() == view.Render
		}); err != nil {
			return err
		}
		counter = 0
		if err := send(4, 1); err != nil {
			return err
		}
		if err := call(control.MethodHide); err != nil {
			return err
		}
		if err := awaitLive(ctx, func() bool { return !coordinator.Latest().Visible }); err != nil {
			return err
		}
		time.Sleep(40 * time.Millisecond)
		baseline := requests.Load()
		counter++
		if err := raw.write(4, 0, counter); err != nil {
			return err
		}
		if err := awaitLive(ctx, func() bool { return !coordinator.Latest().Input.Snapshot.Physical("grid.c1.r1").Down }); err != nil {
			return err
		}
		time.Sleep(40 * time.Millisecond)
		if requests.Load() != baseline {
			return errors.New("hidden input queued render")
		}
		if err := call(control.MethodShow); err != nil {
			return err
		}
		if err := awaitLive(ctx, currentDraw); err != nil {
			return err
		}
		if _, err := capture("shown-latest"); err != nil {
			return err
		}
		if mode == "live-local" {
			generation := controller.ProfileSnapshot().Generation
			if err := os.WriteFile(profilePath, []byte(strings.Replace(profileJSON, "Synthetic live", "Synthetic updated", 1)), 0600); err != nil {
				return err
			}
			if err := awaitLive(ctx, func() bool {
				return controller.ProfileSnapshot().Generation > generation && coordinator.Latest().Generations.Profile == controller.ProfileSnapshot().Generation && currentDraw()
			}); err != nil {
				return err
			}
			if _, err := capture("local-updated"); err != nil {
				return err
			}
			lastGood := controller.ProfileSnapshot()
			if err := os.WriteFile(profilePath, []byte("{"), 0600); err != nil {
				return err
			}
			if err := awaitLive(ctx, func() bool {
				response := controller.Dispatch(ctx, control.Request{Version: control.ProtocolVersion, ID: "local", Method: control.MethodStatus})
				for _, d := range response.Result.(control.Status).DegradedReasons {
					if d.Code == profilesource.ERR_PROFILE_LOCAL_UNSUPPORTED && d.Stage == "profile" {
						return true
					}
				}
				return false
			}); err != nil {
				return err
			}
			if controller.ProfileSnapshot() != lastGood {
				return errors.New("local failure removed last-good")
			}
		}
		for _, hz := range []int{30, 60, 120} {
			generation := coordinator.Latest().Generations.Config
			if err := os.WriteFile(path, []byte(settings(hz, os.Getenv("AZERLAY_TEST_LIVE_MONITOR"))), 0600); err != nil {
				return err
			}
			if err := awaitLive(ctx, func() bool { return coordinator.Latest().Generations.Config > generation && currentDraw() }); err != nil {
				return err
			}
			time.Sleep(60 * time.Millisecond)
			if err := liveGTK(ctx, func() error { *requestTimes = nil; return nil }); err != nil {
				return err
			}
			events := raw.Latest().Snapshot.EventCount
			for i := 0; i < 80; i++ {
				counter++
				if err := raw.write(4, byte((i+1)%2), counter); err != nil {
					return err
				}
				time.Sleep(time.Millisecond)
			}
			if err := awaitLive(ctx, func() bool { return coordinator.Latest().Input.Snapshot.EventCount >= events+80 && currentDraw() }); err != nil {
				return err
			}
			time.Sleep(40 * time.Millisecond)
			if err := liveGTK(ctx, func() error {
				for i := 1; i < len(*requestTimes); i++ {
					if (*requestTimes)[i].Sub((*requestTimes)[i-1]) < time.Second/time.Duration(hz)-time.Millisecond {
						return fmt.Errorf("%dHz render request cap exceeded", hz)
					}
				}
				fmt.Printf("backlog refresh_hz=%d requests=%d latest-only=true\n", hz, len(*requestTimes))
				return nil
			}); err != nil {
				return err
			}
			if coordinator.Latest().Input.Snapshot.Physical("grid.c1.r1").Down {
				return errors.New("backlog did not draw last release")
			}
		}
		generation = coordinator.Latest().Generations.Config
		if err := os.WriteFile(path, []byte(settings(60, "azerlay-live-missing-monitor")), 0600); err != nil {
			return err
		}
		if err := awaitLive(ctx, func() bool { return coordinator.Latest().Generations.Config > generation }); err != nil {
			return err
		}
		if err := liveGTK(ctx, func() error {
			if gtk.WindowGetToplevels().NItems() != 1 {
				return errors.New("missing-monitor window not removed")
			}
			return nil
		}); err != nil {
			return err
		}
		counter++
		if err := raw.write(4, 1, counter); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(settings(60, os.Getenv("AZERLAY_TEST_LIVE_MONITOR"))), 0600); err != nil {
			return err
		}
		if err := awaitLive(ctx, func() bool { return currentDraw() && coordinator.Latest().Input.Snapshot.Physical("grid.c1.r1").Down }); err != nil {
			return err
		}
		if _, err := capture("recreated-latest"); err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
		baseline = requests.Load()
		for i := 0; i < 20; i++ {
			if err := call(control.MethodStatus); err != nil {
				return err
			}
			time.Sleep(5 * time.Millisecond)
		}
		if requests.Load() != baseline {
			return errors.New("idle/status queued draw")
		}
		fmt.Println("raw independent press/release unknown malformed counter-loss disconnect/reopen hidden/show recreate last-good idle/status requests=0")
	}
	response, err := control.Call(ctx, control.MethodStatus, control.Params{})
	if err != nil {
		return err
	}
	status := response.Result.(control.Status)
	if status.EventRate == nil || status.RenderRate == nil || status.DroppedCount == nil || status.ResyncCount != nil || status.Generation < 2 || status.LastReload.ConfigGeneration < 1 || status.Device == nil {
		return errors.New("live schema1 measurements missing")
	}
	if mode == "live-missing" {
		return syscall.Kill(os.Getpid(), syscall.SIGINT)
	}
	return call(control.MethodQuit)
}

func writeLiveTiming(path string, measurement live.Measurement) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	err = writer.Write([]string{"sequence", "generation", "relative_read_ns", "relative_draw_ns", "duration_ns"})
	var durations []time.Duration
	for _, sample := range measurement.Samples {
		if err != nil {
			break
		}
		err = writer.Write([]string{strconv.FormatUint(sample.Sequence, 10), strconv.FormatUint(sample.Generation, 10), strconv.FormatInt(int64(sample.Read), 10), strconv.FormatInt(int64(sample.Draw), 10), strconv.FormatInt(int64(sample.Duration), 10)})
		durations = append(durations, sample.Duration)
	}
	writer.Flush()
	err = errors.Join(err, writer.Error(), file.Close())
	if err != nil {
		return err
	}
	slices.Sort(durations)
	fmt.Printf("samples=%d p50=%s p95=%s max=%s excluded=%v\n", len(durations), durations[(len(durations)-1)*50/100], durations[(len(durations)-1)*95/100], durations[len(durations)-1], measurement.Excluded)
	return nil
}
