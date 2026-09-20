package input

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func nativeEvents(events ...Event) []byte {
	var data []byte
	for _, event := range events {
		record := make([]byte, inputEventSize)
		word := func(target []byte, value int64) {
			if timestampWordSize == 8 {
				binary.NativeEndian.PutUint64(target, uint64(value))
			} else {
				binary.NativeEndian.PutUint32(target, uint32(value))
			}
		}
		word(record, int64(event.Timestamp/time.Second))
		word(record[timestampWordSize:], int64(event.Timestamp%time.Second/time.Microsecond))
		offset := 2 * timestampWordSize
		binary.NativeEndian.PutUint16(record[offset:], event.Type)
		binary.NativeEndian.PutUint16(record[offset+2:], event.Code)
		binary.NativeEndian.PutUint32(record[offset+4:], uint32(event.Value))
		data = append(data, record...)
	}
	return data
}

func TestDecodeNativeEvent(t *testing.T) {
	// Linux amd64 input_event: 64-bit seconds/useconds, uint16 type/code,
	// and signed int32 value. Literal bytes are independent of nativeEvents.
	record := []byte{
		1, 0, 0, 0, 0, 0, 0, 0,
		2, 0, 0, 0, 0, 0, 0, 0,
		3, 0, 0x20, 0, 0xfe, 0xff, 0xff, 0xff,
	}
	if timestampWordSize == 4 {
		record = []byte{
			1, 0, 0, 0,
			2, 0, 0, 0,
			3, 0, 0x20, 0, 0xfe, 0xff, 0xff, 0xff,
		}
	}
	expected := Event{Timestamp: time.Second + 2*time.Microsecond, Type: EV_ABS, Code: 0x20, Value: -2}
	if got := decodeEvent(record); got != expected {
		t.Fatalf("decoded = %#v, want %#v", got, expected)
	}
}

type interruptedReader struct {
	source      io.Reader
	interrupted bool
}

func (r *interruptedReader) Read(data []byte) (int, error) {
	if !r.interrupted {
		r.interrupted = true
		return 0, syscall.EINTR
	}
	return r.source.Read(data)
}

func TestReaderSplitRecordsAndTruncation(t *testing.T) {
	events := []Event{{Type: EV_KEY, Code: 30, Value: 1}, {Type: EV_SYN, Code: SYN_REPORT, Timestamp: time.Second}}
	data := nativeEvents(events...)
	for _, truncated := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "truncated"}[truncated], func(t *testing.T) {
			input := append([]byte(nil), data...)
			if truncated {
				input = append(input, 1, 2, 3)
			}
			parts := []io.Reader{bytes.NewReader(input[:3]), bytes.NewReader(input[3 : inputEventSize+7]), bytes.NewReader(input[inputEventSize+7:])}
			output := make(chan nodeEvent)
			done := make(chan error, 1)
			go func() {
				done <- readEvents(context.Background(), &interruptedReader{source: io.MultiReader(parts...)}, 2, output)
			}()
			var got []Event
			for range events {
				select {
				case item := <-output:
					if item.node != 2 {
						t.Fatal(item.node)
					}
					got = append(got, item.event)
				case <-time.After(3 * time.Second):
					t.Fatal("record delivery blocked")
				}
			}
			if !reflect.DeepEqual(got, events) {
				t.Fatalf("events = %#v", got)
			}
			select {
			case err := <-done:
				if err == nil || err.Error() != ERR_INPUT_READ {
					t.Fatalf("error = %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("read did not terminate")
			}
		})
	}
}

type notifyingReader struct {
	file *os.File
	read chan struct{}
}

func (r notifyingReader) Read(data []byte) (int, error) {
	n, err := r.file.Read(data)
	close(r.read)
	return n, err
}

func TestReaderCancelBlockedSend(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	read := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- readEvents(ctx, notifyingReader{reader, read}, 0, make(chan nodeEvent)) }()
	if _, err := writer.Write(nativeEvents(Event{Type: EV_SYN})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-read:
	case <-time.After(3 * time.Second):
		t.Fatal("read blocked")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("send did not cancel")
	}
}

func TestReaderClockIoctl(t *testing.T) {
	sentinel := errors.New("ioctl failed")
	err := setClock(42, func(fd, request uintptr, clock *int32) error {
		if fd != 42 || request != 0x400445a0 || *clock != 1 {
			t.Fatalf("clock ioctl: %v %x %v", fd, request, *clock)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
}
