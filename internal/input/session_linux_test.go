package input

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

func pipeNodes(t *testing.T, count int) ([]device.OpenedNode, []*os.File) {
	t.Helper()
	nodes := make([]device.OpenedNode, count)
	writers := make([]*os.File, count)
	for i := range nodes {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
		nodes[i].File = reader
		writers[i] = writer
	}
	return nodes, writers
}

func awaitSession(t *testing.T, session *Session) {
	t.Helper()
	select {
	case <-session.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("session shutdown blocked")
	}
}

func assertFilesClosed(t *testing.T, nodes []device.OpenedNode) {
	t.Helper()
	for i, node := range nodes {
		if _, err := node.File.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Errorf("node %d not closed: %v", i, err)
		}
	}
}

func TestSessionClockFailureRollback(t *testing.T) {
	nodes, _ := pipeNodes(t, 3)
	setups := 0
	session, err := startSession(context.Background(), device.Group{}, Generations{}, sessionOps{
		open: func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error {
			setups++
			if setups == 2 {
				return errors.New("private path")
			}
			return nil
		},
	})
	if session != nil || err == nil || err.Error() != ERR_INPUT_READ || setups != 2 {
		t.Fatalf("start = %v, %v, setups %d", session, err, setups)
	}
	assertFilesClosed(t, nodes)
}

func TestSessionAxisReadFailureRollback(t *testing.T) {
	nodes, _ := pipeNodes(t, 3)
	nodes[1].Node.Capabilities = map[string][]int{"abs": {0}}
	session, err := startSession(context.Background(), device.Group{}, Generations{}, sessionOps{
		open:  func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error { return nil },
		axes:  readAxisInfo,
	})
	if session != nil {
		t.Cleanup(func() { _ = session.Close() })
	}
	if session != nil || err == nil || err.Error() != ERR_INPUT_READ {
		t.Fatalf("ABS ioctl on pipe: session=%v, error=%v", session, err)
	}
	assertFilesClosed(t, nodes)
}

func TestSessionIdleCancellationAndIdempotentClose(t *testing.T) {
	nodes, _ := pipeNodes(t, 2)
	ctx, cancel := context.WithCancel(context.Background())
	setups := 0
	session, err := startSession(ctx, device.Group{}, Generations{Device: 7, Profile: 9}, sessionOps{
		open:  func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error { setups++; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if setups != 2 {
		t.Fatal(setups)
	}
	initial := session.Latest()
	initial.Sequence = 100
	initial.Connected = false
	if got := session.Latest(); got.Sequence != 0 || !got.Connected {
		t.Fatalf("mutable latest: %#v", got)
	}
	cancel()
	awaitSession(t, session)
	var closers sync.WaitGroup
	for range 5 {
		closers.Go(func() {
			if err := session.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	closers.Wait()
	terminal := session.Latest()
	if terminal.Connected || terminal.Sequence != 1 || terminal.Generations != (Generations{Device: 7, Profile: 9}) || session.Err() != nil {
		t.Fatalf("terminal = %#v; error=%v", terminal, session.Err())
	}
	assertFilesClosed(t, nodes)
}

func TestSessionFatalInvalidation(t *testing.T) {
	for _, test := range []struct {
		name        string
		tail        []byte
		code        string
		closeWriter bool
	}{
		{"dropped", nativeEvents(Event{Type: EV_SYN, Code: SYN_DROPPED}, Event{Type: EV_SYN, Code: SYN_REPORT}), ERR_INPUT_DROPPED, false},
		{"invalid-key", nativeEvents(Event{Type: EV_KEY, Value: 3}), ERR_INPUT_EVENT, false},
		{"truncated", []byte{1, 2, 3}, ERR_INPUT_READ, true},
		{"eof", nil, ERR_INPUT_READ, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			nodes, writers := pipeNodes(t, 2)
			published := make(chan *Snapshot, 4)
			session, err := startSession(context.Background(), device.Group{}, Generations{Device: 2}, sessionOps{
				open: func(device.Group) ([]device.OpenedNode, error) { return nodes, nil }, clock: func(*os.File) error { return nil },
				observe: func(_ int, _ Event, snapshot *Snapshot) {
					if snapshot != nil {
						published <- snapshot
					}
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = session.Close() })
			if _, err := writers[0].Write(nativeEvents(Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_ABS, Value: 50}, Event{Type: EV_SYN})); err != nil {
				t.Fatal(err)
			}
			var retained *Snapshot
			select {
			case retained = <-published:
			case <-time.After(3 * time.Second):
				t.Fatal("publication blocked")
			}
			if _, err := writers[0].Write(test.tail); err != nil {
				t.Fatal(err)
			}
			if test.closeWriter {
				_ = writers[0].Close()
			}
			awaitSession(t, session)
			if err := session.Err(); err == nil || err.Error() != test.code {
				t.Fatalf("failure = %v", err)
			}
			terminal := session.Latest()
			if terminal.Connected || terminal.Sequence != 2 || terminal.Generations.Device != 2 || len(terminal.FrameEvents()) != 0 {
				t.Fatalf("terminal=%#v", terminal)
			}
			if _, known := terminal.Key(0, 30); known {
				t.Fatal("terminal retained key")
			}
			if _, known := terminal.Absolute(0, 0); known {
				t.Fatal("terminal retained axis")
			}
			if down, known := retained.Key(0, 30); !known || !down {
				t.Fatal("retained snapshot changed")
			}
			if err := session.Close(); err == nil || err.Error() != test.code {
				t.Fatalf("Close lost first error: %v", err)
			}
			if session.Latest().Sequence != 2 {
				t.Fatal("extra terminal publication")
			}
			assertFilesClosed(t, nodes)
		})
	}
}
