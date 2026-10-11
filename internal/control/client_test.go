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
	startupDeadline := time.Now().Add(3 * time.Second)
	if err := clientConn.SetDeadline(startupDeadline); err != nil {
		t.Fatal(err)
	}
	finished := make(chan time.Time, 1)
	go func() {
		server.serve(ctx, controller, serverConn, startupDeadline)
		finished <- time.Now()
	}()
	if _, err := clientConn.Write(data); err != nil {
		t.Fatal(err)
	}
	// Observe the response starting before timing the blocked write. The
	// exchange deadline also covers parsing and encoding, which may consume
	// a short timeout before any bytes are written under race-test load.
	var firstByte [1]byte
	if _, err := io.ReadFull(clientConn, firstByte[:]); err != nil {
		t.Fatalf("response did not start: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("quit canceled before the blocked write finished")
	}
	if err := serverConn.SetWriteDeadline(time.Now().Add(60 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	select {
	case completed := <-finished:
		if !completed.Before(startupDeadline) {
			t.Fatal("blocked write reached the startup deadline instead of its write deadline")
		}
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
	if received := len(firstByte) + len(response); received >= len(request.ID) {
		t.Fatalf("expected partial write from bounded nonreading connection, got %d", received)
	}
}

func TestControlTimeoutNonReadingExchange(t *testing.T) {
	t.Parallel()
	serverConn, clientConn, _ := socketPair(t)
	if err := serverConn.SetWriteBuffer(1024); err != nil {
		t.Fatal(err)
	}
	if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	request := controllerRequest(MethodShow, "")
	request.ID = strings.Repeat("s", MaxLineBytes-128)
	data, err := EncodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clientConn.Write(data); err != nil {
		t.Fatal(err)
	}
	controller := socketController(t)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		server := &Server{}
		server.serve(t.Context(), controller, serverConn, time.Now().Add(60*time.Millisecond))
	}()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("server exchange deadline did not stop the nonreading peer")
	}
	response, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatal(err)
	}
	// Expiry before writing may yield no bytes; a completed response is
	// impossible while the peer leaves the bounded write buffer undrained.
	if len(response) >= len(request.ID) {
		t.Fatalf("expected incomplete response after exchange timeout, got %d bytes", len(response))
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
			// Give cancellation and response validation time to run under load.
			// Only the deadline case tests expiration of a short exchange timeout.
			timeout := connectionTimeout
			if kind == "deadline" {
				timeout = 80 * time.Millisecond
			}
			result := make(chan error, 1)
			go func() { _, err := call(ctx, runtime, MethodShow, Params{}, timeout); result <- err }()
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
