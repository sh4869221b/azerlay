package input

import (
	"context"
	"errors"
	"io"
	"syscall"
)

type physicalReport struct {
	event     PhysicalEvent
	malformed bool
	err       error
}
type reportReader struct {
	source  io.Reader
	decoder physicalDecoder
	output  chan<- physicalReport
}

func (r reportReader) read(ctx context.Context) {
	// hidraw preserves report boundaries. One extra byte detects an overlong
	// report; never concatenate partial reads into a fabricated report.
	var buffer [65]byte
	for ctx.Err() == nil {
		n, err := r.source.Read(buffer[:])
		if n > 0 {
			event, relevant, invalid := r.decoder.decode(buffer[:n])
			if relevant || invalid != nil {
				if !r.send(ctx, physicalReport{event: event, malformed: invalid != nil}) {
					return
				}
			}
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			r.send(ctx, physicalReport{err: inputReadError(err)})
			return
		}
		if n == 0 {
			r.send(ctx, physicalReport{err: inputReadError(io.ErrNoProgress)})
			return
		}
	}
}
func (r reportReader) send(ctx context.Context, report physicalReport) bool {
	select {
	case <-ctx.Done():
		return false
	case r.output <- report:
		return true
	}
}
