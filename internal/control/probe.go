package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type ProbeState string

const (
	ProbeRunning    ProbeState = "running"
	ProbeNotRunning ProbeState = "not_running"
	ProbeStale      ProbeState = "stale"
)

type ProbeResult struct {
	State  ProbeState
	Target string
	Status *Status
}

func Probe(ctx context.Context) (ProbeResult, error) {
	return probe(ctx, os.Getenv("XDG_RUNTIME_DIR"), connectionTimeout)
}

func probe(ctx context.Context, runtime string, timeout time.Duration) (ProbeResult, error) {
	if ctx.Err() != nil {
		return ProbeResult{}, transportError(ctx, ctx.Err())
	}
	path, err := runtimeSocketPath(runtime)
	if err != nil {
		return ProbeResult{}, err
	}
	result := ProbeResult{Target: path}
	if err := privatePath(filepath.Dir(path), os.ModeDir|0700); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.State = ProbeNotRunning
			return result, nil
		}
		return result, NewError(ERR_CONTROL_RUNTIME)
	}
	if err := privatePath(path, os.ModeSocket|0600); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.State = ProbeNotRunning
			return result, nil
		}
		return result, transportError(ctx, err)
	}
	status, refused, err := instanceStatus(ctx, path, timeout)
	if refused {
		result.State = ProbeStale
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.State, result.Status = ProbeRunning, status
	return result, nil
}
