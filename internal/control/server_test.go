package control

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/profilesource"
)

func socketRuntime(t *testing.T) string {
	t.Helper()
	path, err := os.MkdirTemp("", "az-ctl-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	})
	return path
}

func socketController(t *testing.T) *Controller {
	t.Helper()
	manager, _ := controllerManager(t, "")
	source := profilesource.NewImportedSource(t.TempDir())
	importControllerProfiles(t, source, controllerBundle)
	controller, err := NewController(t.Context(), manager, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controller.Close)
	return controller
}

func socketServer(t *testing.T, timeout time.Duration) (*Server, *Controller, string) {
	t.Helper()
	runtime := socketRuntime(t)
	controller := socketController(t)
	server, err := start(t.Context(), controller, runtime, timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	return server, controller, runtime
}

func socketConnect(t *testing.T, runtime string) *net.UnixConn {
	t.Helper()
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: filepath.Join(runtime, "azerlay/control.sock"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return conn
}

func awaitSocketDone(t *testing.T, server *Server) {
	t.Helper()
	select {
	case <-server.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}

func requireControlError(t *testing.T, err error, code string) {
	t.Helper()
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != code {
		t.Fatalf("error = %v; want %s", err, code)
	}
}

func TestControlSocket(t *testing.T) {
	runtime := socketRuntime(t)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	controller := socketController(t)
	server, err := Start(t.Context(), controller)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	for path, mode := range map[string]os.FileMode{runtime: os.ModeDir | 0700, filepath.Join(runtime, "azerlay"): os.ModeDir | 0700, filepath.Join(runtime, "azerlay/control.sock"): os.ModeSocket | 0600} {
		if err := privatePath(path, mode); err != nil {
			t.Fatalf("permissions: %v", err)
		}
	}
	conn := socketConnect(t, runtime)
	if err := checkPeer(conn); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	response, err := Call(t.Context(), MethodShow, Params{})
	if err != nil || !response.OK || !response.Result.(VisibilityResult).Visible {
		t.Fatalf("show: %+v %v", response, err)
	}
	response, err = Call(t.Context(), MethodSelect, Params{Selector: "First"})
	if err != nil || !response.OK || response.Result.(SelectionResult).ActiveProfile.ProfileIndex != 1 {
		t.Fatalf("select: %+v %v", response, err)
	}
	const toggles = 31
	var workers sync.WaitGroup
	for range toggles {
		workers.Go(func() {
			response, err := Call(t.Context(), MethodToggle, Params{})
			if err != nil || !response.OK {
				t.Errorf("toggle: %+v %v", response, err)
			}
		})
	}
	workers.Wait()
	response, err = Call(t.Context(), MethodStatus, Params{})
	if err != nil || !response.OK {
		t.Fatalf("status: %+v %v", response, err)
	}
	status := response.Result.(Status)
	if status.Visible || status.ActiveProfile.ProfileIndex != 1 || status.Generation != 3+toggles {
		t.Fatalf("status after toggles: %+v", status)
	}
	response, err = Call(t.Context(), MethodQuit, Params{})
	if err != nil || !response.OK || !response.Result.(QuitResult).Quitting {
		t.Fatalf("quit: %+v %v", response, err)
	}
	awaitSocketDone(t, server)
	if _, err := os.Lstat(filepath.Join(runtime, "azerlay/control.sock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains: %v", err)
	}
	if err := privatePath(filepath.Join(runtime, "azerlay"), os.ModeDir|0700); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.manager.RequestReload(t.Context()); err != nil {
		t.Fatalf("server stopped caller's manager: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestControlSocketRejectFraming(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, code, id string }{
		{"malformed", "{\n", ERR_CONTROL_REQUEST, ""},
		{"oversized", strings.Repeat("x", MaxLineBytes+1) + "\n", ERR_CONTROL_TOO_LARGE, ""},
		{"unterminated", `{"version":1,"id":"test","method":"overlay.show","params":{}}`, ERR_CONTROL_REQUEST, ""},
		{"version", `{"version":2,"id":"test","method":"overlay.show","params":{}}` + "\n", ERR_CONTROL_VERSION, "test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, controller, runtime := socketServer(t, time.Second)
			conn := socketConnect(t, runtime)
			if _, err := io.WriteString(conn, test.input); err != nil {
				t.Fatal(err)
			}
			if err := conn.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			var response struct {
				ID    *string
				OK    bool
				Error *Error
			}
			if err := json.NewDecoder(conn).Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.OK || response.Error == nil || response.Error.Code != test.code {
				t.Fatalf("response: %+v", response)
			}
			if test.id == "" && response.ID != nil || test.id != "" && (response.ID == nil || *response.ID != test.id) {
				t.Fatalf("id: %v", response.ID)
			}
			if controllerStatus(t, controller).Visible {
				t.Fatal("invalid request dispatched")
			}
		})
	}
}

func TestControlSocketRejectExtraRequest(t *testing.T) {
	t.Parallel()
	_, controller, runtime := socketServer(t, time.Second)
	conn := socketConnect(t, runtime)
	data, err := EncodeRequest(controllerRequest(MethodToggle, ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(append(data, data...)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadResponse(conn, controllerRequest(MethodToggle, "")); err != nil {
		t.Fatal(err)
	}
	var tail [1]byte
	if n, err := conn.Read(tail[:]); n != 0 || err == nil {
		t.Fatalf("extra response: %d %v", n, err)
	}
	status := controllerStatus(t, controller)
	if !status.Visible || status.Generation != 2 {
		t.Fatalf("extra request dispatched: %+v", status)
	}
}

func TestControlTimeoutPartialRequest(t *testing.T) {
	t.Parallel()
	_, controller, runtime := socketServer(t, 40*time.Millisecond)
	conn := socketConnect(t, runtime)
	if _, err := io.WriteString(conn, `{"version":1`); err != nil {
		t.Fatal(err)
	}
	var data [512]byte
	if _, err := conn.Read(data[:]); err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			t.Fatalf("server did not enforce its deadline: %v", err)
		}
	}
	if controllerStatus(t, controller).Visible {
		t.Fatal("partial request dispatched")
	}
}

func TestControlShutdown(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"cancel", "close", "quit"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			controller := socketController(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			server, err := start(ctx, controller, runtime, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { server.Close() })
			conn := socketConnect(t, runtime)
			if _, err := io.WriteString(conn, `{"version":1`); err != nil {
				t.Fatal(err)
			}
			switch cause {
			case "cancel":
				cancel()
			case "close":
				if err := server.Close(); err != nil {
					t.Fatal(err)
				}
			case "quit":
				response, err := call(t.Context(), runtime, MethodQuit, Params{}, time.Second)
				if err != nil || !response.OK {
					t.Fatalf("quit: %+v %v", response, err)
				}
			}
			awaitSocketDone(t, server)
			var data [1]byte
			if n, err := conn.Read(data[:]); n != 0 || err == nil {
				t.Fatalf("connection remains: %d %v", n, err)
			}
			if _, err := os.Lstat(filepath.Join(runtime, "azerlay/control.sock")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("socket remains: %v", err)
			}
		})
	}
}

func TestControlShutdownPreservesReplacement(t *testing.T) {
	t.Parallel()
	server, _, runtime := socketServer(t, time.Second)
	path := filepath.Join(runtime, "azerlay/control.sock")
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "replacement" {
		t.Fatalf("replacement removed: %q %v", data, err)
	}
}

func TestControlShutdownManaged(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"cancel", "close", "quit"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			owner := instanceOwner(t, runtime)
			controller := socketController(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			entered, release := make(chan struct{}), make(chan struct{})
			releaseConfig := sync.OnceFunc(func() { close(release) })
			var calls atomic.Int32
			server, err := owner.Start(ctx, controller, func() {
				if calls.Add(1) == 1 {
					close(entered)
				}
				select {
				case <-release:
				case <-time.After(3 * time.Second):
					t.Error("config cleanup was not released")
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				releaseConfig()
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			})
			conn := socketConnect(t, runtime)
			if _, err := io.WriteString(conn, `{"version":1`); err != nil {
				t.Fatal(err)
			}
			if response, err := call(t.Context(), runtime, MethodStatus, Params{}, time.Second); err != nil || !response.OK {
				t.Fatalf("status: %+v %v", response, err)
			}
			closed := make(chan error, 1)
			switch cause {
			case "cancel":
				cancel()
			case "close":
				go func() { closed <- server.Close() }()
			case "quit":
				response, err := call(t.Context(), runtime, MethodQuit, Params{}, time.Second)
				if err != nil || !response.OK || !response.Result.(QuitResult).Quitting {
					t.Fatalf("quit: %+v %v", response, err)
				}
			}
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("config cleanup did not start")
			}
			if err := privatePath(owner.path, os.ModeSocket|0600); err != nil {
				t.Fatalf("socket removed before config cleanup: %v", err)
			}
			select {
			case <-server.Done():
				t.Fatal("Done closed before config cleanup")
			case err := <-closed:
				t.Fatalf("Close returned before config cleanup: %v", err)
			default:
			}
			var data [1]byte
			if n, err := conn.Read(data[:]); n != 0 || err == nil {
				t.Fatalf("partial connection remains: %d %v", n, err)
			} else if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				t.Fatalf("partial connection was not closed: %v", err)
			}
			if _, err := call(t.Context(), runtime, MethodShow, Params{}, time.Second); err == nil {
				t.Fatal("listener accepted a new request during config cleanup")
			}
			if controllerStatus(t, controller).Visible {
				t.Fatal("request dispatched during config cleanup")
			}
			releaseConfig()
			awaitSocketDone(t, server)
			if cause == "close" {
				if err := <-closed; err != nil {
					t.Fatal(err)
				}
			}
			if err := server.Close(); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 1 {
				t.Fatalf("config cleanup called %d times", calls.Load())
			}
			if _, err := os.Lstat(owner.path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("socket remains: %v", err)
			}
			other, status, err := acquireInstance(t.Context(), runtime, time.Second)
			if other != nil {
				other.Close()
				t.Fatal("server cleanup released instance ownership")
			}
			if status != nil || err == nil {
				t.Fatalf("contending acquisition: %v %v", status, err)
			}
		})
	}
}

func TestControlShutdownManagedStartupFailure(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"cancel", "occupied"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			owner := instanceOwner(t, socketRuntime(t))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if cause == "cancel" {
				cancel()
			} else if err := os.WriteFile(owner.path, []byte("replacement"), 0600); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			server, err := owner.Start(ctx, socketController(t), func() { calls.Add(1) })
			if server != nil {
				server.Close()
				t.Fatal("startup unexpectedly succeeded")
			}
			if err == nil || calls.Load() != 0 {
				t.Fatalf("startup error = %v; cleanup calls = %d", err, calls.Load())
			}
			if cause == "occupied" {
				data, err := os.ReadFile(owner.path)
				if err != nil || string(data) != "replacement" {
					t.Fatalf("occupied path changed: %q %v", data, err)
				}
			}
		})
	}
}
