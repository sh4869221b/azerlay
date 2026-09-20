package input

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

func recoverNode(ctx context.Context, node device.OpenedNode) (recoveryState, time.Duration, error) {
	timestamp, err := drainEvents(ctx, node.File, syscall.Read)
	if err != nil {
		return recoveryState{}, timestamp, err
	}
	if err := ctx.Err(); err != nil {
		return recoveryState{}, timestamp, err
	}
	state, err := readRecoveryState(node)
	return state, timestamp, err
}

func drainEvents(ctx context.Context, file *os.File, read func(int, []byte) (int, error)) (time.Duration, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return 0, inputReadError(err)
	}
	var timestamp time.Duration
	var readErr error
	err = connection.Control(func(fd uintptr) {
		var buffer [64 * inputEventSize]byte
		for {
			if readErr = ctx.Err(); readErr != nil {
				return
			}
			n, err := read(int(fd), buffer[:])
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if errors.Is(err, syscall.EAGAIN) {
				return
			}
			if err != nil {
				readErr = err
				return
			}
			if n == 0 {
				readErr = io.EOF
				return
			}
			if n%inputEventSize != 0 {
				readErr = &InputError{Code: ERR_INPUT_READ}
				return
			}
			for offset := 0; offset < n; offset += inputEventSize {
				timestamp = max(timestamp, decodeEvent(buffer[offset:offset+inputEventSize]).Timestamp)
			}
		}
	})
	if err != nil {
		return timestamp, inputReadError(err)
	}
	if readErr != nil {
		return timestamp, inputReadError(readErr)
	}
	return timestamp, nil
}
