package control

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func instanceOwner(t *testing.T, runtime string) *Instance {
	t.Helper()
	owner, status, err := acquireInstance(t.Context(), runtime, time.Second)
	if err != nil || owner == nil || status != nil {
		t.Fatalf("acquire = %v, %v, %v", owner, status, err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	return owner
}

func instanceListener(t *testing.T, runtime string) (*net.UnixListener, string) {
	t.Helper()
	path, err := socketPath(runtime, true)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	t.Cleanup(func() { listener.Close() })
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	return listener, path
}

func TestInstanceAcquire(t *testing.T) {
	runtime := socketRuntime(t)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	owner, status, err := AcquireInstance(t.Context())
	if err != nil || owner == nil || status != nil {
		t.Fatalf("acquire = %v, %v, %v", owner, status, err)
	}
	t.Cleanup(func() { owner.Close() })
	if err := privatePath(filepath.Join(runtime, "azerlay/instance.lock"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestInstanceDuplicate(t *testing.T) {
	t.Parallel()
	for _, locked := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "owned"}[locked], func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			if locked {
				instanceOwner(t, runtime)
			}
			server, err := start(t.Context(), socketController(t), runtime, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { server.Close() })
			owner, status, err := acquireInstance(t.Context(), runtime, time.Second)
			if err != nil || owner != nil || status == nil {
				t.Fatalf("duplicate = %v, %v, %v", owner, status, err)
			}
			if response, err := call(t.Context(), runtime, MethodStatus, Params{}, time.Second); err != nil || !response.OK {
				t.Fatalf("original instance unusable: %v, %v", response, err)
			}
			if err := server.Close(); err != nil {
				t.Fatal(err)
			}
			if !locked {
				instanceOwner(t, runtime)
			}
		})
	}
}

func TestInstanceContendedStarting(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	instanceOwner(t, runtime)
	server, err := start(t.Context(), socketController(t), runtime, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	path := filepath.Join(runtime, "azerlay/control.sock")
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	owner, status, err := acquireInstance(t.Context(), runtime, time.Second)
	requireControlError(t, err, ERR_CONTROL_UNAVAILABLE)
	if owner != nil || status != nil {
		t.Fatal("contender acquired a starting instance")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || after.Mode() != before.Mode() {
		t.Fatalf("contender changed starting socket: %v", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	owner, status, err = acquireInstance(t.Context(), runtime, time.Second)
	if err != nil || owner != nil || status == nil {
		t.Fatalf("ready duplicate = %v, %v, %v", owner, status, err)
	}
}

func TestInstanceStale(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	listener, path := instanceListener(t, runtime)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	instanceOwner(t, runtime)
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale socket remains: %v", err)
	}
	server, err := start(t.Context(), socketController(t), runtime, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
}

func TestInstanceConcurrent(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	begin := make(chan struct{})
	type result struct {
		owner  *Instance
		status *Status
		err    error
	}
	results := make(chan result, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-begin
			owner, status, err := acquireInstance(t.Context(), runtime, time.Second)
			results <- result{owner, status, err}
		}()
	}
	ready.Wait()
	close(begin)
	owners := 0
	for range 2 {
		result := <-results
		if result.owner != nil {
			owners++
			t.Cleanup(func() { result.owner.Close() })
			if result.err != nil || result.status != nil {
				t.Fatalf("owner returned other result: %+v", result)
			}
		} else {
			requireControlError(t, result.err, ERR_CONTROL_UNAVAILABLE)
		}
	}
	if owners != 1 {
		t.Fatalf("owners = %d", owners)
	}
}

func TestInstanceRelease(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	owner := instanceOwner(t, runtime)
	path := filepath.Join(runtime, "azerlay/instance.lock")
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
	}
	instanceOwner(t, runtime)
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("lock identity changed: %v", err)
	}
}

func TestInstanceRejectUnsafe(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"lock-symlink", "lock-mode", "lock-directory", "lock-fifo", "socket-file", "socket-symlink", "socket-mode"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			path, err := socketPath(runtime, true)
			if err != nil {
				t.Fatal(err)
			}
			if kind[:4] == "lock" {
				path = filepath.Join(filepath.Dir(path), "instance.lock")
			}
			switch kind {
			case "lock-symlink", "socket-symlink":
				err = os.Symlink("missing", path)
			case "lock-mode":
				err = os.WriteFile(path, []byte("keep"), 0644)
			case "lock-directory":
				err = os.Mkdir(path, 0700)
			case "lock-fifo":
				err = syscall.Mkfifo(path, 0600)
			case "socket-file":
				err = os.WriteFile(path, []byte("keep"), 0600)
			case "socket-mode":
				instanceListener(t, runtime)
				err = os.Chmod(path, 0660)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			owner, status, err := acquireInstance(t.Context(), runtime, time.Second)
			requireControlError(t, err, ERR_CONTROL_PERMISSION)
			if owner != nil || status != nil {
				t.Fatal("unsafe target acquired")
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("unsafe target changed: %v", err)
			}
		})
	}
}

func TestInstanceRejectCancelled(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	owner, status, err := acquireInstance(ctx, runtime, time.Second)
	requireControlError(t, err, ERR_CONTROL_UNAVAILABLE)
	if owner != nil || status != nil {
		t.Fatal("cancelled acquisition succeeded")
	}
	if _, err := os.Lstat(filepath.Join(runtime, "azerlay")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled acquisition created app directory: %v", err)
	}
}

func TestInstanceRejectContendedStale(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	instanceOwner(t, runtime)
	listener, path := instanceListener(t, runtime)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	owner, status, err := acquireInstance(t.Context(), runtime, time.Second)
	requireControlError(t, err, ERR_CONTROL_UNAVAILABLE)
	if owner != nil || status != nil {
		t.Fatal("contender acquired an owned instance")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("contender removed owner's socket: %v", err)
	}
}

func TestInstanceRejectLiveFailure(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"timeout", "malformed", "remote"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			listener, path := instanceListener(t, runtime)
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			go func() {
				owner, _, err := acquireInstance(t.Context(), runtime, 100*time.Millisecond)
				if owner != nil {
					owner.Close()
				}
				result <- err
			}()
			if err := listener.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			conn, err := listener.AcceptUnix()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			request, err := ReadRequest(conn)
			if err != nil {
				t.Fatal(err)
			}
			code := ERR_CONTROL_TIMEOUT
			switch kind {
			case "malformed":
				_, err = conn.Write([]byte("{\n"))
				code = ERR_CONTROL_REQUEST
			case "remote":
				data, encodeErr := EncodeResponse(FailureResponse(&request.ID, NewError(ERR_CONTROL_UNAVAILABLE)))
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				_, err = conn.Write(data)
				code = ERR_CONTROL_UNAVAILABLE
			}
			if err != nil {
				t.Fatal(err)
			}
			requireControlError(t, <-result, code)
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("live socket changed: %v", err)
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			instanceOwner(t, runtime)
		})
	}
}

func TestInstanceRejectReplaced(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"replacement", "missing", "mode"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			listener, path := instanceListener(t, socketRuntime(t))
			created, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			code := ERR_CONTROL_UNAVAILABLE
			if kind == "mode" {
				err = os.Chmod(path, 0660)
				code = ERR_CONTROL_PERMISSION
			} else {
				err = os.Rename(path, path+".old")
				if err == nil && kind == "replacement" {
					err = os.WriteFile(path, []byte("keep"), 0600)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			requireControlError(t, removeStaleSocket(path, created), code)
			if kind == "replacement" {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "keep" {
					t.Fatalf("replacement changed: %q, %v", data, err)
				}
			}
		})
	}
}
