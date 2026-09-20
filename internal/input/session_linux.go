package input

import (
	"context"
	"maps"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/sh4869221b/azerlay/internal/device"
)

// Session owns all selected descriptors and publishes immutable input state.
// Stop and join an old session before replacing it with another session.
type Session struct {
	latest atomic.Pointer[Snapshot]
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

type sessionOps struct {
	open  func(device.Group) ([]device.OpenedNode, error)
	clock func(*os.File) error
	axes  func(*os.File, []int) (map[uint16]AxisInfo, error)
	// observe runs on the reducer after each event, including unreported events.
	// It is private so tests can acknowledge processing without polling Latest.
	observe func(int, Event, *Snapshot)
}

// Start revalidates and opens only the selected group. Ownership transfers to
// the session only after every descriptor's clock and axis metadata are ready.
func Start(ctx context.Context, group device.Group, generations Generations) (*Session, error) {
	return startSession(ctx, group, generations, sessionOps{open: device.OpenGroup, clock: setMonotonicClock, axes: readAxisInfo})
}

func startSession(ctx context.Context, group device.Group, generations Generations, ops sessionOps) (*Session, error) {
	nodes, err := ops.open(group)
	if err != nil {
		return nil, err
	}
	closeFiles := func() error {
		var first error
		for _, node := range nodes {
			if err := node.File.Close(); err != nil && first == nil {
				first = &InputError{Code: ERR_INPUT_READ}
			}
		}
		return first
	}
	axisInfo := make([]map[uint16]AxisInfo, len(nodes))
	for index, node := range nodes {
		if err := ops.clock(node.File); err != nil {
			_ = closeFiles()
			return nil, &InputError{Code: ERR_INPUT_READ}
		}
		if codes := node.Node.Capabilities["abs"]; len(codes) > 0 {
			info, err := ops.axes(node.File, codes)
			if err != nil {
				_ = closeFiles()
				return nil, &InputError{Code: ERR_INPUT_READ}
			}
			axisInfo[index] = maps.Clone(info)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	session := &Session{cancel: cancel, done: make(chan struct{})}
	reducer := newReducer(len(nodes), generations)
	reducer.latest.axisInfo = axisInfo
	session.latest.Store(reducer.snapshot())
	events := make(chan nodeEvent)
	var readers sync.WaitGroup
	for index, node := range nodes {
		readers.Go(func() {
			if err := readEvents(ctx, node.File, index, events); err != nil {
				select {
				case events <- nodeEvent{node: index, err: err}:
				case <-ctx.Done():
				}
			}
		})
	}
	go func() {
		defer func() {
			cancel()
			if err := closeFiles(); session.err == nil {
				session.err = err
			}
			readers.Wait()
			session.latest.Store(reducer.stop())
			close(session.done)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case received := <-events:
				if ctx.Err() != nil {
					return
				}
				if received.err != nil {
					session.err = received.err
					return
				}
				snapshot, err := reducer.apply(received.node, received.event)
				if err != nil {
					session.err = err
					return
				}
				if snapshot != nil {
					session.latest.Store(snapshot)
				}
				if ops.observe != nil {
					var observed *Snapshot
					if snapshot != nil {
						copied := *snapshot
						observed = &copied
					}
					ops.observe(received.node, received.event, observed)
				}
			}
		}
	}()
	return session, nil
}

// Latest returns an independently owned scalar header over immutable storage.
func (s *Session) Latest() *Snapshot { snapshot := *s.latest.Load(); return &snapshot }

// Close is idempotent and waits until all owned work has stopped.
func (s *Session) Close() error { s.cancel(); <-s.done; return s.err }

// Done closes after reader/reducer shutdown and terminal publication.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns the first fatal input failure; call it only after Done closes.
// Context cancellation is normal shutdown and returns nil.
func (s *Session) Err() error { return s.err }

func setMonotonicClock(file *os.File) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var ioctlErr error
	err = connection.Control(func(fd uintptr) {
		ioctlErr = setClock(fd, clockIoctl)
	})
	if err != nil {
		return err
	}
	return ioctlErr
}

func setClock(fd uintptr, ioctl func(uintptr, uintptr, *int32) error) error {
	// Linux _IOW('E', 0xa0, int), with CLOCK_MONOTONIC passed by pointer.
	clock := int32(1)
	return ioctl(fd, 0x400445a0, &clock)
}

func clockIoctl(fd, request uintptr, clock *int32) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(clock)))
	if errno != 0 {
		return errno
	}
	return nil
}
