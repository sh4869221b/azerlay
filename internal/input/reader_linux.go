package input

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"syscall"
	"time"
	"unsafe"
)

const timestampWordSize = int(unsafe.Sizeof(syscall.Timeval{}.Sec))
const inputEventSize = 2*timestampWordSize + 8

type nodeEvent struct {
	node        int
	event       Event
	err         error
	replacement *recoveryState
}

type eventReader struct {
	source  io.Reader
	node    int
	output  chan<- nodeEvent
	recover func(context.Context) (recoveryState, time.Duration, error)
}

func decodeEvent(record []byte) Event {
	word := func(b []byte) int64 {
		if timestampWordSize == 8 {
			return int64(binary.NativeEndian.Uint64(b))
		}
		return int64(int32(binary.NativeEndian.Uint32(b)))
	}
	offset := 2 * timestampWordSize
	return Event{
		Timestamp: time.Duration(word(record))*time.Second + time.Duration(word(record[timestampWordSize:]))*time.Microsecond,
		Type:      binary.NativeEndian.Uint16(record[offset:]),
		Code:      binary.NativeEndian.Uint16(record[offset+2:]),
		Value:     int32(binary.NativeEndian.Uint32(record[offset+4:])),
	}
}

// read retains only a partial native record between reads. os.File.Read
// performs nonblocking descriptor waits through the Go poller.
func (r eventReader) read(ctx context.Context) error {
	var buffer [inputEventSize * 64]byte
	used := 0
	lost := false
	var timestamp time.Duration
	for {
		if ctx.Err() != nil {
			return nil
		}
		n, err := r.source.Read(buffer[used:])
		used += n
		consumed := 0
		for used-consumed >= inputEventSize {
			event := decodeEvent(buffer[consumed : consumed+inputEventSize])
			consumed += inputEventSize
			if lost {
				timestamp = max(timestamp, event.Timestamp)
				if event.Type != EV_SYN || event.Code != SYN_REPORT {
					continue
				}
				for used-consumed >= inputEventSize {
					timestamp = max(timestamp, decodeEvent(buffer[consumed:consumed+inputEventSize]).Timestamp)
					consumed += inputEventSize
				}
				consumed = used
				state, drainedTime, recoveryErr := r.recover(ctx)
				if ctx.Err() != nil {
					return nil
				}
				if recoveryErr != nil {
					return inputReadError(recoveryErr)
				}
				event.Timestamp = max(timestamp, drainedTime)
				if !r.send(ctx, nodeEvent{node: r.node, event: event, replacement: &state}) {
					return nil
				}
				lost = false
				continue
			}
			if !r.send(ctx, nodeEvent{node: r.node, event: event}) {
				return nil
			}
			if event.Type == EV_SYN && event.Code == SYN_DROPPED {
				lost, timestamp = true, event.Timestamp
			}
		}
		used = copy(buffer[:], buffer[consumed:used])
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			return inputReadError(err)
		}
	}
}

func (r eventReader) send(ctx context.Context, event nodeEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case r.output <- event:
		return true
	}
}
