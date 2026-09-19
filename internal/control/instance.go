package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Instance holds the exclusive startup lock until its caller finishes cleanup.
type Instance struct {
	path     string
	lock     *os.File
	close    sync.Once
	closeErr error
}

// AcquireInstance returns either exclusive ownership or a live instance's status.
func AcquireInstance(ctx context.Context) (*Instance, *Status, error) {
	return acquireInstance(ctx, os.Getenv("XDG_RUNTIME_DIR"), connectionTimeout)
}

func acquireInstance(ctx context.Context, runtime string, timeout time.Duration) (owner *Instance, status *Status, err error) {
	if ctx.Err() != nil {
		return nil, nil, transportError(ctx, ctx.Err())
	}
	path, err := socketPath(runtime, true)
	if err != nil {
		return nil, nil, err
	}
	lock, err := os.OpenFile(filepath.Join(filepath.Dir(path), "instance.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, nil, NewError(ERR_CONTROL_PERMISSION)
	}
	instance := &Instance{path: path, lock: lock}
	defer func() {
		if owner == nil {
			err = errors.Join(err, instance.Close())
			if err != nil {
				status = nil
			}
		}
	}()
	info, err := lock.Stat()
	if err != nil || privateInfo(info, 0600) != nil {
		return nil, nil, NewError(ERR_CONTROL_PERMISSION)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, nil, transportError(ctx, err)
		}
		status, _, err := instanceStatus(ctx, path, timeout)
		return nil, status, err
	}
	if ctx.Err() != nil {
		return nil, nil, transportError(ctx, ctx.Err())
	}
	created, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return instance, nil, nil
	}
	if err != nil {
		return nil, nil, transportError(ctx, err)
	}
	if err := privateInfo(created, os.ModeSocket|0600); err != nil {
		return nil, nil, err
	}
	status, refused, err := instanceStatus(ctx, path, timeout)
	if !refused {
		return nil, status, err
	}
	if ctx.Err() != nil {
		return nil, nil, transportError(ctx, ctx.Err())
	}
	if err := removeStaleSocket(path, created); err != nil {
		return nil, nil, err
	}
	return instance, nil, nil
}

func instanceStatus(ctx context.Context, path string, timeout time.Duration) (*Status, bool, error) {
	response, refused, err := callPath(ctx, path, MethodStatus, Params{}, timeout)
	if err != nil {
		return nil, refused, err
	}
	if !response.OK {
		return nil, false, response.Error
	}
	status, ok := response.Result.(Status)
	if !ok {
		return nil, false, NewError(ERR_CONTROL_REQUEST)
	}
	return &status, false, nil
}

func removeStaleSocket(path string, created os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(created, current) {
		return NewError(ERR_CONTROL_UNAVAILABLE)
	}
	if err := privateInfo(current, os.ModeSocket|0600); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return NewError(ERR_CONTROL_UNAVAILABLE)
	}
	return nil
}

// Close releases ownership but retains the stable lock file for future owners.
func (i *Instance) Close() error {
	i.close.Do(func() {
		if err := i.lock.Close(); err != nil {
			i.closeErr = NewError(ERR_CONTROL_UNAVAILABLE)
		}
	})
	return i.closeErr
}
