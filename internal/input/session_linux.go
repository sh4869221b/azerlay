package input

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/sh4869221b/azerlay/internal/device"
)

// Session owns the selected descriptor and joins its reader before Done closes.
type Session struct {
	latest     atomic.Pointer[Snapshot]
	cancel     context.CancelFunc
	done       chan struct{}
	changes    chan struct{}
	err        error
	cleanupErr error
}
type sessionOps struct {
	open    func(device.Group) ([]device.OpenedNode, error)
	observe func(*Snapshot)
}
type sessionSetupError struct {
	error
	cleanup error
}

func (e *sessionSetupError) Unwrap() error { return e.error }
func Start(ctx context.Context, group device.Group, generations Generations) (*Session, error) {
	return startSession(ctx, group, generations, sessionOps{open: device.OpenGroup})
}
func startSession(ctx context.Context, group device.Group, generations Generations, ops sessionOps) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	decoder, err := newPhysicalDecoder()
	if err != nil {
		return nil, err
	}
	nodes, err := ops.open(group)
	if err != nil {
		return nil, err
	}
	closeFiles := func() error {
		var result error
		for _, node := range nodes {
			if err := node.File.Close(); err != nil {
				result = errors.Join(result, inputReadError(err))
			}
		}
		return result
	}
	rollback := func(err error) error {
		if closeErr := closeFiles(); closeErr != nil {
			return &sessionSetupError{error: errors.Join(err, closeErr), cleanup: closeErr}
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return nil, rollback(err)
	}
	if len(nodes) != 1 {
		return nil, rollback(&InputError{Code: ERR_INPUT_READ})
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{cancel: cancel, done: make(chan struct{}), changes: make(chan struct{}, 1)}
	reducer := newReducer(generations)
	s.latest.Store(reducer.snapshot())
	reports := make(chan physicalReport)
	readerDone := make(chan struct{})
	go func() { defer close(readerDone); reportReader{nodes[0].File, decoder, reports}.read(ctx) }()
	go func() {
		defer func() {
			cancel()
			s.cleanupErr = closeFiles()
			s.err = errors.Join(s.err, s.cleanupErr)
			<-readerDone
			s.latest.Store(reducer.stop())
			s.notify()
			close(s.done)
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case report := <-reports:
				if ctx.Err() != nil {
					return
				}
				if report.err != nil {
					s.err = report.err
					return
				}
				var snapshot *Snapshot
				if report.malformed {
					snapshot = reducer.invalidate(ERR_INPUT_EVENT)
				} else {
					snapshot = reducer.apply(report.event)
					reducer.latest.ReadAt = report.readAt
					snapshot = reducer.snapshot()
				}
				s.latest.Store(snapshot)
				s.notify()
				if ops.observe != nil {
					copy := *snapshot
					ops.observe(&copy)
				}
			}
		}
	}()
	return s, nil
}
func (s *Session) Latest() *Snapshot        { snapshot := *s.latest.Load(); return &snapshot }
func (s *Session) Close() error             { s.cancel(); <-s.done; return s.err }
func (s *Session) Done() <-chan struct{}    { return s.done }
func (s *Session) Err() error               { return s.err }
func (s *Session) Changes() <-chan struct{} { return s.changes }
func (s *Session) notify() {
	select {
	case s.changes <- struct{}{}:
	default:
	}
}
