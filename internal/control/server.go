package control

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"
)

// Server owns only its listener, accepted connections, and created socket.
type Server struct {
	cancel   context.CancelFunc
	done     chan struct{}
	closeErr error
}

func Start(ctx context.Context, controller *Controller) (*Server, error) {
	return start(ctx, controller, os.Getenv("XDG_RUNTIME_DIR"), connectionTimeout)
}

func start(ctx context.Context, controller *Controller, runtime string, timeout time.Duration) (*Server, error) {
	if ctx.Err() != nil {
		return nil, transportError(ctx, ctx.Err())
	}
	path, err := socketPath(runtime, true)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return nil, NewError(ERR_CONTROL_UNAVAILABLE)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, transportError(ctx, err)
	}
	listener.SetUnlinkOnClose(false)
	created, err := os.Lstat(path)
	if err != nil {
		listener.Close()
		return nil, NewError(ERR_CONTROL_UNAVAILABLE)
	}
	if err := os.Chmod(path, 0600); err != nil {
		listener.Close()
		return nil, errors.Join(NewError(ERR_CONTROL_PERMISSION), removeOwnedSocket(path, created))
	}
	ctx, cancel := context.WithCancel(ctx)
	server := &Server{cancel: cancel, done: make(chan struct{})}
	accepted := make(chan struct{})
	var handlers sync.WaitGroup
	go func() {
		defer close(accepted)
		defer cancel()
		for {
			conn, err := listener.AcceptUnix()
			if err != nil {
				return
			}
			deadline := time.Now().Add(timeout)
			handlers.Add(1)
			go func() {
				defer handlers.Done()
				server.serve(ctx, controller, conn, deadline)
			}()
		}
	}()
	go func() {
		<-ctx.Done()
		listener.Close()
		<-accepted
		handlers.Wait()
		server.closeErr = removeOwnedSocket(path, created)
		close(server.done)
	}()
	return server, nil
}

func (s *Server) Done() <-chan struct{} { return s.done }

func (s *Server) Close() error {
	s.cancel()
	<-s.done
	return s.closeErr
}

func (s *Server) serve(ctx context.Context, controller *Controller, conn *net.UnixConn, deadline time.Time) {
	defer conn.Close()
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	stop := closeOnCancel(ctx, conn)
	defer stop()
	if conn.SetDeadline(deadline) != nil || checkPeer(conn) != nil {
		return
	}
	request, err := ReadRequest(conn)
	var response Response
	if err != nil {
		var id *string
		if request.ID != "" {
			id = &request.ID
		}
		response = FailureResponse(id, transportError(ctx, err))
	} else {
		response = controller.Dispatch(ctx, request)
	}
	quit, quitting := response.Result.(QuitResult)
	if response.OK && quitting && quit.Quitting {
		defer s.cancel()
	}
	data, err := EncodeResponse(response)
	if err != nil {
		return
	}
	// A failed write cannot undo dispatch. Quit still shuts down via the defer.
	conn.Write(data)
}
