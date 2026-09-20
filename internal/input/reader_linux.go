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
	node  int
	event Event
	err   error
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

// readEvents retains only a partial native record between reads. os.File.Read
// performs nonblocking descriptor waits through the Go poller.
func readEvents(ctx context.Context, source io.Reader, node int, output chan<- nodeEvent) error {
	var buffer [inputEventSize * 64]byte
	used := 0
	for {
		n, err := source.Read(buffer[used:])
		used += n
		consumed := 0
		for used-consumed >= inputEventSize {
			event := decodeEvent(buffer[consumed : consumed+inputEventSize])
			select {
			case <-ctx.Done():
				return nil
			case output <- nodeEvent{node: node, event: event}:
			}
			consumed += inputEventSize
		}
		used = copy(buffer[:], buffer[consumed:used])
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			return &InputError{Code: ERR_INPUT_READ}
		}
	}
}
