package control

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestControlSocketRejectPaths(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"empty", "relative", "missing", "runtime-mode", "runtime-symlink", "app-mode", "app-symlink"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			app := filepath.Join(runtime, "azerlay")
			switch kind {
			case "empty":
				runtime = ""
			case "relative":
				runtime = "relative"
			case "missing":
				runtime = filepath.Join(runtime, "missing")
			case "runtime-mode":
				if err := os.Chmod(runtime, 0750); err != nil {
					t.Fatal(err)
				}
			case "runtime-symlink":
				link := filepath.Join(socketRuntime(t), "link")
				if err := os.Symlink(runtime, link); err != nil {
					t.Fatal(err)
				}
				runtime = link
			case "app-mode":
				if err := os.Mkdir(app, 0755); err != nil {
					t.Fatal(err)
				}
			case "app-symlink":
				if err := os.Symlink(socketRuntime(t), app); err != nil {
					t.Fatal(err)
				}
			}
			_, err := start(t.Context(), nil, runtime, time.Second)
			requireControlError(t, err, ERR_CONTROL_RUNTIME)
			_, err = call(t.Context(), runtime, MethodStatus, Params{}, time.Second)
			requireControlError(t, err, ERR_CONTROL_RUNTIME)
		})
	}
}

func TestControlSocketRejectOccupied(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"file", "directory", "symlink", "socket"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			path, err := socketPath(runtime, true)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "file":
				err = os.WriteFile(path, []byte("keep"), 0600)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "symlink":
				err = os.Symlink("missing", path)
			case "socket":
				var listener *net.UnixListener
				listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
				if err == nil {
					listener.SetUnlinkOnClose(false)
					err = listener.Close()
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			_, err = start(t.Context(), nil, runtime, time.Second)
			requireControlError(t, err, ERR_CONTROL_UNAVAILABLE)
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("occupied path changed: %v", err)
			}
		})
	}
}

func TestControlSocketRejectClientTarget(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"missing", "file", "symlink", "mode"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			path, err := socketPath(runtime, true)
			if err != nil {
				t.Fatal(err)
			}
			code := ERR_CONTROL_PERMISSION
			switch kind {
			case "missing":
				code = ERR_CONTROL_UNAVAILABLE
			case "file":
				err = os.WriteFile(path, []byte("keep"), 0600)
			case "symlink":
				err = os.Symlink("missing", path)
			case "mode":
				var listener *net.UnixListener
				listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
				if err == nil {
					t.Cleanup(func() { listener.Close() })
					err = os.Chmod(path, 0660)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = call(t.Context(), runtime, MethodStatus, Params{}, time.Second)
			requireControlError(t, err, code)
		})
	}
}

func TestControlPeer(t *testing.T) {
	t.Parallel()
	if !sameUID(uint32(os.Geteuid())) || sameUID(uint32(os.Geteuid())+1) {
		t.Fatal("UID comparison failed")
	}
	// Actual credential retrieval is exercised on both sides of TestControlSocket.
	_, _, runtime := socketServer(t, time.Second)
	conn := socketConnect(t, runtime)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	requireControlError(t, checkPeer(conn), ERR_CONTROL_PERMISSION)
}

func TestControlShutdownCancelledStart(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := start(ctx, nil, runtime, time.Second)
	requireControlError(t, err, ERR_CONTROL_UNAVAILABLE)
	if _, err := os.Lstat(filepath.Join(runtime, "azerlay")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled start created app: %v", err)
	}
}
