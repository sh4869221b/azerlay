package control

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func socketPair(t *testing.T) (*net.UnixConn, *net.UnixConn, string) {
	t.Helper()
	runtime := socketRuntime(t)
	path, err := socketPath(runtime, true)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	client := socketConnect(t, runtime)
	server, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	return server, client, runtime
}

func TestControlTimeoutNonReadingPeer(t *testing.T) {
	t.Parallel()
	serverConn, clientConn, _ := socketPair(t)
	if err := serverConn.SetWriteBuffer(1024); err != nil {
		t.Fatal(err)
	}
	controller := socketController(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	server := &Server{cancel: cancel}
	request := controllerRequest(MethodQuit, "")
	request.ID = strings.Repeat("q", MaxLineBytes-128)
	data, err := EncodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		server.serve(ctx, controller, serverConn, time.Now().Add(60*time.Millisecond))
	}()
	if _, err := clientConn.Write(data); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("nonreading peer blocked shutdown")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("quit did not cancel after failed write")
	}
	response, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	if len(response) == 0 || len(response) >= len(request.ID) {
		t.Fatalf("expected partial write from bounded nonreading connection, got %d", len(response))
	}
}

func TestControlTimeoutClient(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"deadline", "cancel", "malformed", "oversized", "wrong-id"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			runtime := socketRuntime(t)
			path, err := socketPath(runtime, true)
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { listener.Close() })
			if err := os.Chmod(path, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := call(ctx, runtime, MethodShow, Params{}, 80*time.Millisecond); result <- err }()
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
			code := ERR_CONTROL_REQUEST
			switch kind {
			case "deadline":
				code = ERR_CONTROL_TIMEOUT
			case "cancel":
				cancel()
				code = ERR_CONTROL_UNAVAILABLE
			case "malformed":
				_, err = io.WriteString(conn, "{\n")
			case "oversized":
				_, err = io.WriteString(conn, strings.Repeat("x", MaxLineBytes+1))
				code = ERR_CONTROL_TOO_LARGE
			case "wrong-id":
				request.ID = "other"
				var data []byte
				data, err = EncodeResponse(SuccessResponse(request, VisibilityResult{Visible: true}))
				if err == nil {
					_, err = conn.Write(data)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				requireControlError(t, err, code)
			case <-time.After(3 * time.Second):
				t.Fatal("client did not finish")
			}
			var tail [1]byte
			if _, err := conn.Read(tail[:]); err == nil {
				t.Fatal("client left connection open or retried")
			}
		})
	}
}
