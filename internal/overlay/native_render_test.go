package overlay

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/sh4869221b/azerlay/internal/config"
	"github.com/sh4869221b/azerlay/internal/layout"
	"github.com/sh4869221b/azerlay/internal/profile"
	"github.com/sh4869221b/azerlay/internal/renderer"
)

func TestNativeRenderer(t *testing.T) {
	if os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY") == "" {
		launcher, err := filepath.Abs("../../scripts/test-wayland.sh")
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), "bash", launcher, os.Args[0], "-test.run=^TestNativeRenderer$", "-test.v")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Wayland fixture: %v\n%s", err, output)
		} else {
			t.Log(string(output))
		}
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = append(os.Environ(), nativeChildEnv+"=renderer", "GDK_BACKEND=wayland", "WAYLAND_DISPLAY="+os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native renderer: %v\n%s", err, output)
	}
	if string(output) != "rendered\n" {
		t.Fatalf("native renderer did not complete: %s", output)
	}
}

func renderPixel(frame *renderer.Frame, width, height, x, y int) ([4]uint8, error) {
	surface := cairo.CreateImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Close()
	cr := cairo.Create(surface)
	frame.Draw(cr)
	cr.Close()
	var encoded bytes.Buffer
	if err := surface.WriteToPNGWriter(&encoded); err != nil {
		return [4]uint8{}, err
	}
	image, err := png.Decode(&encoded)
	if err != nil {
		return [4]uint8{}, err
	}
	r, g, b, a := image.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}, nil
}

func runNativeRendererChild() int {
	settings := config.Overlay{Anchor: "center", Scale: 1, Opacity: .75, Mode: "detailed", ShowProfileName: true, ShowStatus: true}
	w, err := New(settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer w.Close()
	definition := layout.Definition{ViewBox: layout.Rect{Width: 160, Height: 100}, Controls: []layout.Control{{ID: "button", Shape: layout.Shape{Type: "rounded_rect", X: 15, Y: 15, Width: 130, Height: 70}, LabelAnchor: layout.Point{X: 80, Y: 50}}}}
	snapshot := renderer.NewSnapshot(definition, renderer.Content{ProfileName: "Synthetic", Status: renderer.StatusDisconnected, Controls: map[string]renderer.Control{"button": {Known: true, Down: true, Assignments: []renderer.Assignment{{Kind: profile.BindingKeyboard, Label: "日本語 action", BindingDisplay: "K"}}}}})
	appearance := config.Appearance{Theme: "dark", FontScale: 1}
	w.SetRenderState(snapshot, appearance, true)
	if w.area == nil || w.frame != nil || w.area.ContentHeight() <= 150 {
		fmt.Fprintln(os.Stderr, "render area was not attached with deferred frame")
		return 1
	}
	if err := w.SetVisible(true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	loop := glib.NewMainLoop(nil, false)
	deadline := time.Now().Add(10 * time.Second)
	phase := 0
	var priorFrame *renderer.Frame
	var priorWidget any
	var dark [4]uint8
	var failure error
	check := func(ok bool, message string) bool {
		if ok {
			return true
		}
		failure = errors.New(message)
		loop.Quit()
		return false
	}
	glib.TimeoutAdd(10, func() bool {
		if time.Now().After(deadline) {
			failure = errors.New("native renderer timed out")
			loop.Quit()
			return false
		}
		switch phase {
		case 0:
			if !mappedLayer(w) || w.frame == nil {
				break
			}
			if !check(w.area != nil && w.area.Mapped() && w.renderWidth > 0 && w.renderHeight > 0, "mapped renderer lacks allocated child") {
				return false
			}
			dark, err = renderPixel(w.frame, w.renderWidth, w.renderHeight, w.renderWidth/2, w.renderHeight/2)
			if !check(err == nil && dark[3] > 0 && dark[3] < 255, "renderer did not draw with configured opacity") {
				return false
			}
			priorFrame = w.frame
			w.SetRenderState(snapshot, appearance, true)
			if !check(w.frame == priorFrame, "unchanged render state was prepared twice") {
				return false
			}
			w.area.SetContentWidth(w.renderWidth + 64)
			phase = 1
		case 1:
			if w.frame == priorFrame || w.renderWidth <= 176 {
				break
			}
			priorFrame = w.frame
			appearance.Theme = "light"
			appearance.FontScale = 1.5
			w.SetRenderState(snapshot, appearance, false)
			if !check(w.frame != priorFrame, "appearance update reused stale frame") {
				return false
			}
			light, drawErr := renderPixel(w.frame, w.renderWidth, w.renderHeight, w.renderWidth/2, w.renderHeight/2)
			if !check(drawErr == nil && light != dark, "appearance update did not change native raster") {
				return false
			}
			priorFrame = w.frame
			settings.Mode = "compact"
			settings.Scale = 2
			oldContentWidth := w.area.ContentWidth()
			if !check(w.ApplyConfig(settings) == nil && w.frame != priorFrame && w.area.ContentWidth() > oldContentWidth, "overlay drawing options and requested size were not reapplied") {
				return false
			}
			if !check(w.SetVisible(false) == nil, "hide failed") {
				return false
			}
			phase = 2
		case 2:
			if w.State().Mapped {
				break
			}
			if !check(w.SetVisible(true) == nil, "show failed") {
				return false
			}
			phase = 3
		case 3:
			if !mappedLayer(w) {
				break
			}
			if !check(w.snapshot == snapshot && w.frame != nil, "hide/show dropped render state") {
				return false
			}
			priorWidget = w.widget
			settings.Monitor = "azerlay-missing-monitor"
			if !check(w.ApplyConfig(settings) == nil && w.widget == nil && w.area == nil && w.frame == nil && w.snapshot == snapshot, "missing monitor kept widget or lost snapshot") {
				return false
			}
			settings.Monitor = ""
			if !check(w.ApplyConfig(settings) == nil && w.widget != nil && w.widget != priorWidget && w.area != nil, "monitor recovery did not recreate renderer") {
				return false
			}
			phase = 4
		case 4:
			if !mappedLayer(w) || w.frame == nil {
				break
			}
			if !check(w.snapshot == snapshot && w.area != nil, "recreated widget lost render state") {
				return false
			}
			recreated, drawErr := renderPixel(w.frame, w.renderWidth, w.renderHeight, w.renderWidth/2, w.renderHeight/2)
			if !check(drawErr == nil && recreated[3] > 0, "recreated child did not render snapshot") {
				return false
			}
			w.SetRenderState(nil, appearance, false)
			if !check(w.area == nil && w.frame == nil, "nil snapshot retained renderer") {
				return false
			}
			w.SetRenderState(snapshot, appearance, false)
			if !check(w.area != nil, "reattach failed") {
				return false
			}
			w.renderWidth, w.renderHeight = 0, 0
			w.prepareRender(false)
			if !check(w.frame == nil, "zero allocation retained frame") {
				return false
			}
			w.renderWidth, w.renderHeight = 176, 116
			w.prepareRender(false)
			if !check(w.frame != nil, "positive allocation did not prepare frame") {
				return false
			}
			w.Close()
			if !check(w.area == nil && w.frame == nil && w.widget == nil, "Close retained renderer") {
				return false
			}
			fmt.Println("rendered")
			loop.Quit()
			return false
		}
		return true
	})
	loop.Run()
	if failure != nil {
		fmt.Fprintln(os.Stderr, failure)
		return 1
	}
	return 0
}

func qaRenderSnapshot(status renderer.Status) (*renderer.OverlaySnapshot, error) {
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		return nil, err
	}
	controls := make(map[string]renderer.Control, len(definition.Controls))
	for i, region := range definition.Controls {
		control := renderer.Control{Known: i%3 != 0, Down: i%3 == 2}
		switch i % 7 {
		case 0:
			control.Assignments = []renderer.Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingUnbound, Label: "Unbound"}}
		case 1:
			control.Assignments = []renderer.Assignment{{Trigger: profile.TriggerUnknown, Kind: profile.BindingUnknown, Label: "Unknown"}}
		case 2:
			control.Assignments = []renderer.Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "日本語 action", BindingDisplay: "W"}}
		case 3:
			control.Assignments = []renderer.Assignment{
				{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "Primary action", BindingDisplay: "Q"},
				{Trigger: profile.TriggerLong, Kind: profile.BindingMacro, Label: "Hold macro", BindingDisplay: "Macro"},
				{Trigger: profile.TriggerDouble, Kind: profile.BindingKeyboard, Label: "Double action", BindingDisplay: "D"},
			}
		case 4:
			control.Assignments = []renderer.Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "Duplicate", BindingDisplay: "E", Ambiguous: true}}
		case 5:
			control.Assignments = []renderer.Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingKeyboard, Label: "長い日本語と English label é 👾 that must ellipsize", BindingDisplay: "R"}}
		default:
			control.Assignments = []renderer.Assignment{{Trigger: profile.TriggerSingle, Kind: profile.BindingMacro, Label: "Macro", BindingDisplay: "Macro"}}
		}
		controls[region.ID] = control
	}
	controls["stick.main"] = renderer.Control{Assignments: []renderer.Assignment{{Kind: profile.BindingStick, Label: "Stick", BindingDisplay: "Analog"}}}
	return renderer.NewSnapshot(definition, renderer.Content{ProfileName: "Synthetic Cyborg II · 日本語", Controls: controls, Status: status}), nil
}

func runRenderQAChild() int {
	settings := config.Overlay{Anchor: "center", Monitor: os.Getenv("AZERLAY_RENDER_QA_MONITOR"), Scale: 1, Opacity: .88, Mode: "normal", ShowProfileName: true, ShowStatus: true, ShowUnbound: true}
	appearance := config.Appearance{Theme: "dark", FontScale: 1}
	snapshot, err := qaRenderSnapshot(renderer.StatusNone)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	w, err := New(settings)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer w.Close()
	w.SetRenderState(snapshot, appearance, true)
	loop := glib.NewMainLoop(nil, false)
	commands := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			commands <- strings.TrimSpace(scanner.Text())
		}
	}()
	var pending string
	var readyForAck bool
	var failure error
	deadline := time.Now().Add(60 * time.Second)
	fmt.Println("ready")
	glib.TimeoutAdd(10, func() bool {
		if time.Now().After(deadline) {
			failure = errors.New("render QA child timed out")
			loop.Quit()
			return false
		}
		if pending == "hidden" && !w.State().Mapped {
			fmt.Println("hidden")
			pending = ""
		} else if pending != "" && mappedLayer(w) && w.area != nil && w.area.Mapped() && w.frame != nil && w.placePending == 0 {
			if !readyForAck {
				readyForAck = true
				w.area.QueueDraw()
			} else {
				fmt.Printf("%s %d %d\n", pending, w.renderWidth, w.renderHeight)
				pending = ""
			}
		}
		select {
		case command := <-commands:
			if pending != "" {
				failure = errors.New("command arrived before previous draw")
				loop.Quit()
				return false
			}
			readyForAck = false
			switch command {
			case "show":
				err = w.SetVisible(true)
				pending = "shown"
			case "sync":
				w.prepareRender(true)
				pending = "synced"
			case "theme-light":
				appearance.Theme = "light"
				w.SetRenderState(snapshot, appearance, true)
				pending = "theme-light"
			case "theme-dark":
				appearance.Theme = "dark"
				w.SetRenderState(snapshot, appearance, true)
				pending = "theme-dark"
			case "normal", "compact", "detailed":
				settings.Mode = command
				err = w.ApplyConfig(settings)
				pending = command
			case "normal-dark", "compact-light", "detailed-dark-hc", "detailed-light-hc":
				settings.Mode = strings.Split(command, "-")[0]
				appearance.Theme = strings.Split(command, "-")[1]
				appearance.HighContrast = strings.HasSuffix(command, "-hc")
				if command == "normal-dark" {
					snapshot, err = qaRenderSnapshot(renderer.StatusNone)
					if err != nil {
						break
					}
				}
				err = w.ApplyConfig(settings)
				w.SetRenderState(snapshot, appearance, true)
				pending = command
			case "status-disconnected", "status-profile-missing", "status-reload-failed":
				status := map[string]renderer.Status{"status-disconnected": renderer.StatusDisconnected, "status-profile-missing": renderer.StatusProfileMissing, "status-reload-failed": renderer.StatusReloadFailed}[command]
				snapshot, err = qaRenderSnapshot(status)
				if err == nil {
					w.SetRenderState(snapshot, appearance, true)
				}
				pending = command
			case "font-large":
				appearance.FontScale = 4
				w.SetRenderState(snapshot, appearance, true)
				pending = command
			case "narrow":
				w.area.SetContentWidth(360)
				w.area.SetContentHeight(400)
				w.widget.SetDefaultSize(360, 400)
				pending = command
			case "recreate":
				settings.Monitor = "azerlay-missing-monitor"
				err = w.ApplyConfig(settings)
				if err == nil && (w.widget != nil || w.frame != nil || w.snapshot != snapshot) {
					err = errors.New("missing monitor retained widget or lost snapshot")
				}
				settings.Monitor = os.Getenv("AZERLAY_RENDER_QA_MONITOR")
				if err == nil {
					err = w.ApplyConfig(settings)
				}
				pending = command
			case "hide":
				err = w.SetVisible(false)
				pending = "hidden"
			case "quit":
				w.Close()
				fmt.Println("closed")
				loop.Quit()
				return false
			default:
				err = fmt.Errorf("unknown render QA command %q", command)
			}
			if err != nil {
				failure = err
				loop.Quit()
				return false
			}
		default:
		}
		return true
	})
	loop.Run()
	if failure != nil {
		fmt.Fprintln(os.Stderr, failure)
		return 1
	}
	return 0
}

func TestNativeRendererScaled(t *testing.T) {
	if os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY") == "" {
		launcher, err := filepath.Abs("../../scripts/test-wayland.sh")
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), "bash", launcher, os.Args[0], "-test.run=^TestNativeRendererScaled$", "-test.v")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("Wayland fixture: %v\n%s", err, output)
		} else {
			t.Log(string(output))
		}
		return
	}
	dir := os.Getenv("AZERLAY_RENDER_QA_DIR")
	if dir == "" {
		dir = t.TempDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sockets, err := filepath.Glob(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "sway-ipc.*.sock"))
	if err != nil || len(sockets) != 1 {
		t.Fatalf("expected task-owned Sway IPC socket, found %v: %v", sockets, err)
	}
	socket := sockets[0]
	sway := func(args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "swaymsg", append([]string{"-s", socket}, args...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("swaymsg %v: %v\n%s", args, err, output)
		}
		return output
	}
	setScale := func(scale float64) {
		t.Helper()
		sway("output", "HEADLESS-1", "mode", "1920x1600")
		sway("output", "HEADLESS-1", "scale", fmt.Sprint(scale))
		var outputs []struct {
			Name        string  `json:"name"`
			Active      bool    `json:"active"`
			Scale       float64 `json:"scale"`
			CurrentMode struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"current_mode"`
		}
		if err := json.Unmarshal(sway("-t", "get_outputs", "-r"), &outputs); err != nil {
			t.Fatal(err)
		}
		for _, output := range outputs {
			if output.Name == "HEADLESS-1" {
				if !output.Active || output.CurrentMode.Width != 1920 || output.CurrentMode.Height != 1600 || output.Scale != scale {
					t.Fatalf("HEADLESS-1 mode/scale mismatch: %+v", output)
				}
				return
			}
		}
		t.Fatal("HEADLESS-1 missing from task compositor")
	}
	setScale(1)
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0])
	child.Env = append(os.Environ(), nativeChildEnv+"=render-qa", "AZERLAY_RENDER_QA_MONITOR=HEADLESS-1", "GDK_BACKEND=wayland", "GSK_RENDERER=cairo", "WAYLAND_DISPLAY="+os.Getenv("AZERLAY_TEST_WAYLAND_DISPLAY"))
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	child.Stderr = &stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	var waited bool
	var waitErr error
	finishChild := func(stop bool) error {
		if !waited {
			if stop {
				cancel()
			}
			stdin.Close()
			waitErr = child.Wait()
			waited = true
		}
		return waitErr
	}
	defer finishChild(true)
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "ready" {
		finishChild(true)
		t.Fatalf("render QA child not ready: %q %v %s", scanner.Text(), scanner.Err(), stderr.String())
	}
	send := func(command, want string) (int, int) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, command); err != nil {
			t.Fatal(err)
		}
		if !scanner.Scan() {
			finishChild(true)
			t.Fatalf("render QA child stopped after %s: %v %s", command, scanner.Err(), stderr.String())
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || fields[0] != want {
			finishChild(true)
			t.Fatalf("render QA command %s: got %q, want %s; stderr=%s", command, scanner.Text(), want, stderr.String())
		}
		if want != "hidden" && want != "closed" && (len(fields) != 3 || fields[1] == "0" || fields[2] == "0") {
			t.Fatalf("render QA %s had no positive GTK allocation: %q", command, scanner.Text())
		}
		t.Log(scanner.Text())
		if want == "hidden" || want == "closed" {
			return 0, 0
		}
		width, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		height, err := strconv.Atoi(fields[2])
		if err != nil {
			t.Fatal(err)
		}
		return width, height
	}
	var priorImage image.Image
	capture := func(name string, expectChange bool) {
		t.Helper()
		path := filepath.Join(dir, name+".png")
		deadline := time.Now().Add(4 * time.Second)
		for {
			cmd := exec.CommandContext(t.Context(), "grim", "-o", "HEADLESS-1", path)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("grim %s: %v\n%s", name, err, output)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			current, err := png.Decode(file)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if current.Bounds().Dx() != 1920 || current.Bounds().Dy() != 1600 {
				t.Fatalf("native screenshot %s: %v", path, current.Bounds())
			}
			painted, minX, minY, maxX, maxY := 0, 1920, 1600, 0, 0
			for y := 0; y < 1600; y += 20 {
				for x := 0; x < 1920; x += 20 {
					r, g, b, _ := current.At(x, y).RGBA()
					if r+g+b != 0 {
						painted++
						minX, minY = min(minX, x), min(minY, y)
						maxX, maxY = max(maxX, x), max(maxY, y)
					}
				}
			}
			changed := 0
			if expectChange && priorImage != nil {
				for y := 0; y < 1600 && changed < 10; y += 4 {
					for x := 0; x < 1920 && changed < 10; x += 4 {
						if current.At(x, y) != priorImage.At(x, y) {
							changed++
						}
					}
				}
			}
			centerX, centerY := (minX+maxX)/2, (minY+maxY)/2
			centered := painted >= 10 && centerX >= 880 && centerX <= 1040 && centerY >= 720 && centerY <= 880
			if centered && (!expectChange || priorImage == nil || changed >= 10) {
				priorImage = current
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("native screenshot %s did not show centered updated renderer: painted=%d changed=%d bounds=(%d,%d)-(%d,%d)", path, painted, changed, minX, minY, maxX, maxY)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	send("show", "shown")
	capture("scale1-normal-dark", false)
	send("compact-light", "compact-light")
	capture("scale1-compact-light", true)
	setScale(1.25)
	send("normal-dark", "normal-dark")
	send("sync", "synced")
	capture("scale1.25-normal-dark", true)
	send("detailed-dark-hc", "detailed-dark-hc")
	capture("scale1.25-detailed-dark-hc", true)
	send("status-disconnected", "status-disconnected")
	capture("scale1.25-status-disconnected", true)
	send("status-profile-missing", "status-profile-missing")
	capture("scale1.25-status-profile-missing", true)
	setScale(2)
	send("normal-dark", "normal-dark")
	send("sync", "synced")
	capture("scale2-normal-dark", true)
	send("detailed-light-hc", "detailed-light-hc")
	capture("scale2-detailed-light-hc", true)
	send("status-reload-failed", "status-reload-failed")
	capture("scale2-status-reload-failed", true)
	send("font-large", "font-large")
	narrowWidth, narrowHeight := send("narrow", "narrow")
	if narrowWidth > 400 || narrowHeight > 450 {
		t.Fatalf("narrow allocation stayed at %dx%d", narrowWidth, narrowHeight)
	}
	capture("scale2-font-large-narrow", true)
	send("recreate", "recreate")
	capture("scale2-recreated", false)
	send("hide", "hidden")
	send("quit", "closed")
	if err := finishChild(false); err != nil {
		t.Fatalf("render QA child exited: %v\n%s", err, stderr.String())
	}
}
