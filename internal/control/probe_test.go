package control

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestProbeRunning(t *testing.T) {
	_, controller, runtime := socketServer(t, time.Second)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	path := filepath.Join(runtime, "azerlay/control.sock")
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	status := controllerStatus(t, controller)
	result, err := Probe(t.Context())
	if err != nil || result.State != ProbeRunning || result.Target != path || result.Status == nil {
		t.Fatalf("probe=%+v err=%v", result, err)
	}
	if result.Status.Generation != status.Generation || result.Status.Visible != status.Visible || !reflect.DeepEqual(result.Status.ActiveProfile, status.ActiveProfile) || !reflect.DeepEqual(result.Status.DegradedReasons, status.DegradedReasons) {
		t.Fatalf("status changed: got=%+v want=%+v", result.Status, status)
	}
	assertProbeSocketUnchanged(t, path, before)
}

func TestProbeMissing(t *testing.T) {
	t.Parallel()
	for _, appExists := range []bool{false, true} {
		t.Run(map[bool]string{false: "app absent", true: "socket absent"}[appExists], func(t *testing.T) {
			runtime := socketRuntime(t)
			app := filepath.Join(runtime, "azerlay")
			if appExists {
				if err := os.Mkdir(app, 0700); err != nil {
					t.Fatal(err)
				}
			}
			result, err := probe(t.Context(), runtime, time.Second)
			if err != nil || result.State != ProbeNotRunning || result.Target != filepath.Join(app, "control.sock") || result.Status != nil {
				t.Fatalf("probe=%+v err=%v", result, err)
			}
			if appExists {
				entries, err := os.ReadDir(app)
				if err != nil || len(entries) != 0 {
					t.Fatalf("probe created runtime files: %v, %v", entries, err)
				}
			} else if _, err := os.Lstat(app); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("probe created application directory: %v", err)
			}
		})
	}
}

func TestProbeStale(t *testing.T) {
	t.Parallel()
	runtime := socketRuntime(t)
	listener, path := instanceListener(t, runtime)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := probe(t.Context(), runtime, time.Second)
	if err != nil || result.State != ProbeStale || result.Target != path || result.Status != nil {
		t.Fatalf("probe=%+v err=%v", result, err)
	}
	assertProbeSocketUnchanged(t, path, before)
}

func TestProbeUnsafePaths(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"empty", "runtime-mode", "runtime-symlink", "app-mode", "app-symlink", "socket-mode", "socket-symlink"} {
		t.Run(kind, func(t *testing.T) {
			runtime := socketRuntime(t)
			app := filepath.Join(runtime, "azerlay")
			path := filepath.Join(app, "control.sock")
			code, target := ERR_CONTROL_RUNTIME, path
			var err error
			switch kind {
			case "empty":
				runtime, target = "", ""
			case "runtime-mode":
				err = os.Chmod(runtime, 0750)
				target = ""
			case "runtime-symlink":
				link := filepath.Join(socketRuntime(t), "link")
				err = os.Symlink(runtime, link)
				runtime, target = link, ""
			case "app-mode":
				err = os.Mkdir(app, 0755)
			case "app-symlink":
				err = os.Symlink(socketRuntime(t), app)
			case "socket-mode":
				instanceListener(t, runtime)
				err = os.Chmod(path, 0660)
				code = ERR_CONTROL_PERMISSION
			case "socket-symlink":
				if err := os.Mkdir(app, 0700); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink("missing", path)
				code = ERR_CONTROL_PERMISSION
			}
			if err != nil {
				t.Fatal(err)
			}
			result, err := probe(t.Context(), runtime, time.Second)
			requireControlError(t, err, code)
			if result.State != "" || result.Target != target || result.Status != nil {
				t.Fatalf("unsafe probe=%+v", result)
			}
			if _, err := os.Lstat(filepath.Join(app, "instance.lock")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("probe created a lock: %v", err)
			}
		})
	}
}

func TestProbeLiveFailure(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"timeout", "malformed", "remote", "disconnect"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			listener, path := instanceListener(t, runtime)
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			type outcome struct {
				result ProbeResult
				err    error
			}
			finished := make(chan outcome, 1)
			go func() {
				result, err := probe(t.Context(), runtime, 100*time.Millisecond)
				finished <- outcome{result, err}
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
			if err != nil || request.Method != MethodStatus {
				t.Fatalf("request=%+v err=%v", request, err)
			}
			code := ERR_CONTROL_TIMEOUT
			switch kind {
			case "malformed":
				_, err = conn.Write([]byte("{\n"))
				code = ERR_CONTROL_REQUEST
			case "remote":
				var data []byte
				data, err = EncodeResponse(FailureResponse(&request.ID, NewError(ERR_CONTROL_PERMISSION)))
				if err == nil {
					_, err = conn.Write(data)
				}
				code = ERR_CONTROL_PERMISSION
			case "disconnect":
				err = conn.Close()
				code = ERR_CONTROL_REQUEST
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-finished:
				requireControlError(t, got.err, code)
				if got.result.State != "" || got.result.Status != nil || got.result.Target != path {
					t.Fatalf("failed live socket classified as available or stale: %+v", got.result)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("probe did not finish")
			}
			assertProbeSocketUnchanged(t, path, before)
		})
	}
}

func assertProbeSocketUnchanged(t *testing.T, path string, before os.FileInfo) {
	t.Helper()
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || after.Mode() != before.Mode() {
		t.Fatalf("socket was changed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(path), "instance.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("probe created a lock: %v", err)
	}
}
