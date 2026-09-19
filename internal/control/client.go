package control

import (
	"context"
	"errors"
	"net"
	"os"
	"time"
)

const connectionTimeout = 5 * time.Second

// Call performs one exchange and never retries a possibly completed mutation.
// A remote failure is returned in Response.Error; transport failures use error.
func Call(ctx context.Context, method Method, params Params) (Response, error) {
	return call(ctx, os.Getenv("XDG_RUNTIME_DIR"), method, params, connectionTimeout)
}

func call(ctx context.Context, runtime string, method Method, params Params, timeout time.Duration) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request := Request{Version: ProtocolVersion, ID: "1", Method: method, Params: params}
	data, err := EncodeRequest(request)
	if err != nil {
		return Response{}, err
	}
	path, err := socketPath(runtime, false)
	if err != nil {
		return Response{}, err
	}
	if err := privatePath(path, os.ModeSocket|0600); err != nil {
		return Response{}, transportError(ctx, err)
	}
	var dialer net.Dialer
	connection, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return Response{}, transportError(ctx, err)
	}
	conn := connection.(*net.UnixConn)
	defer conn.Close()
	stop := closeOnCancel(ctx, conn)
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return Response{}, transportError(ctx, err)
	}
	if err := checkPeer(conn); err != nil {
		return Response{}, err
	}
	if _, err := conn.Write(data); err != nil {
		return Response{}, transportError(ctx, err)
	}
	response, err := ReadResponse(conn, request)
	if err != nil {
		return Response{}, transportError(ctx, err)
	}
	return response, nil
}

func transportError(ctx context.Context, err error) *Error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return NewError(ERR_CONTROL_TIMEOUT)
	}
	if ctx.Err() != nil {
		return NewError(ERR_CONTROL_UNAVAILABLE)
	}
	var safe *Error
	if errors.As(err, &safe) {
		return safe
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return NewError(ERR_CONTROL_TIMEOUT)
	}
	if errors.Is(err, os.ErrPermission) {
		return NewError(ERR_CONTROL_PERMISSION)
	}
	return NewError(ERR_CONTROL_UNAVAILABLE)
}

// Joining a running callback ensures Done includes connection cancellation work.
func closeOnCancel(ctx context.Context, conn *net.UnixConn) func() {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		conn.Close()
	})
	return func() {
		if !stop() {
			<-done
		}
	}
}
