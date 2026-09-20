package input

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
	"github.com/sh4869221b/azerlay/internal/profile"
)

func newRecoverySession(t *testing.T, recover func(context.Context, device.OpenedNode) (recoveryState, time.Duration, error)) integrationSession {
	t.Helper()
	nodes, writers := pipeNodes(t, 2)
	for i := range nodes {
		nodes[i].Node.Capabilities = map[string][]int{"key": {30, 31}, "abs": {0, 1}}
	}
	observed := make(chan integrationObservation, 32)
	session, err := startSession(context.Background(), device.Group{}, Generations{Device: 3, Profile: 5}, sessionOps{
		open:  func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error { return nil },
		axes: func(*os.File, []int) (map[uint16]AxisInfo, error) {
			return map[uint16]AxisInfo{0: {Minimum: -10, Maximum: 10}, 1: {Minimum: -10, Maximum: 10}}, nil
		},
		recover: recover,
		observe: func(node int, event Event, snapshot *Snapshot) {
			observed <- integrationObservation{node, event, snapshot}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return integrationSession{session, nodes, writers, observed}
}

func awaitRecovery(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("recovery synchronization blocked")
	}
}

func assertRecoveryPublication(t *testing.T, s *Snapshot) {
	t.Helper()
	if s == nil || !s.Connected {
		t.Fatalf("recovery snapshot = %+v", s)
	}
	if _, present := s.ReportingNode(); present || len(s.FrameEvents()) != 0 || len(s.KeyTransitions()) != 0 {
		t.Fatal("recovery fabricated event history")
	}
	if _, present := s.Relative(0); present {
		t.Fatal("recovery retained relative motion")
	}
}

func TestRecoveryLoss(t *testing.T) {
	t.Parallel()
	s := newIntegrationSessionWithAxes(t, 2, Generations{Device: 3, Profile: 5}, map[uint16]AxisInfo{0: {Minimum: -100, Maximum: 100}})
	retained := s.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_ABS, Value: 42}, Event{Type: EV_SYN})
	s.send(t, 1, Event{Type: EV_KEY, Code: 31, Value: 1})
	if _, err := s.writers[0].Write(nativeEvents(Event{Type: EV_SYN, Code: SYN_DROPPED})); err != nil {
		t.Fatal(err)
	}
	select {
	case lost := <-s.observed:
		if lost.snapshot == nil || !lost.snapshot.Connected || lost.snapshot.Sequence != 2 {
			t.Fatalf("loss publication = %+v", lost)
		}
		if _, known := lost.snapshot.Key(0, 30); known {
			t.Fatal("lost node retained key")
		}
		if _, known := lost.snapshot.Absolute(0, 0); known {
			t.Fatal("lost node retained axis")
		}
		if _, present := lost.snapshot.AxisInfo(0, 0); !present {
			t.Fatal("loss removed metadata")
		}
		if down, known := retained.Key(0, 30); !down || !known {
			t.Fatal("loss mutated retained snapshot")
		}
	case <-s.Done():
		t.Fatalf("SYN_DROPPED terminated session: %v", s.Err())
	case <-time.After(3 * time.Second):
		t.Fatal("loss publication blocked")
	}
	other := s.send(t, 1, Event{Type: EV_SYN})
	if down, known := other.Key(1, 31); !down || !known {
		t.Fatal("loss cleared another node's pending frame")
	}
}

func TestRecoveryResync(t *testing.T) {
	t.Parallel()
	beforeDrain, allowDrain := make(chan struct{}), make(chan struct{})
	query, allowQuery := make(chan struct{}), make(chan struct{})
	var queried recoveryState
	queries := 0
	s := newRecoverySession(t, func(ctx context.Context, node device.OpenedNode) (recoveryState, time.Duration, error) {
		close(beforeDrain)
		select {
		case <-allowDrain:
		case <-ctx.Done():
			return recoveryState{}, 0, ctx.Err()
		}
		timestamp, err := drainEvents(ctx, node.File, syscall.Read)
		if err != nil {
			return recoveryState{}, timestamp, err
		}
		close(query)
		select {
		case <-allowQuery:
		case <-ctx.Done():
			return recoveryState{}, timestamp, ctx.Err()
		}
		queries++
		queried, err = queryRecoveryState(node, recoveryIoctls{
			keys: func(_, _ uintptr, raw *[96]byte) (int, error) { raw[31/8] = 1 << (31 % 8); return len(raw), nil },
			axes: func(_, request uintptr, raw *[6]int32) error {
				*raw = [6]int32{0, -100, 100, 3, 10, 0}
				if request == 0x80184540 {
					raw[0] = -55
				}
				return nil
			},
		})
		return queried, timestamp, err
	})
	retained := s.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_ABS, Value: 7}, Event{Type: EV_SYN, Timestamp: 10 * time.Microsecond})
	s.send(t, 1, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_SYN, Timestamp: 12 * time.Microsecond})
	s.send(t, 0, Event{Type: EV_KEY, Code: 32, Value: 1})
	s.send(t, 1, Event{Type: EV_KEY, Code: 31, Value: 1})
	batch := []Event{{Type: EV_SYN, Code: SYN_DROPPED, Timestamp: 20 * time.Microsecond}, {Type: EV_KEY, Value: 99}, {Type: EV_SYN, Code: SYN_DROPPED}, {Type: EV_SYN, Timestamp: 23 * time.Microsecond}}
	for len(batch) < 64 {
		batch = append(batch, Event{Type: EV_KEY, Code: 30, Value: 1, Timestamp: 80 * time.Microsecond})
	}
	if _, err := s.writers[0].Write(nativeEvents(batch...)); err != nil {
		t.Fatal(err)
	}
	lost := s.next(t).snapshot
	assertRecoveryPublication(t, lost)
	if lost.Sequence != 3 || lost.Timestamp != 20*time.Microsecond {
		t.Fatalf("loss metadata = %+v", lost)
	}
	awaitRecovery(t, beforeDrain)
	if _, err := s.writers[0].Write(nativeEvents(Event{Type: EV_ABS, Value: 99}, Event{Type: EV_REL, Value: 4}, Event{Type: EV_SYN, Timestamp: 100 * time.Microsecond})); err != nil {
		t.Fatal(err)
	}
	close(allowDrain)
	awaitRecovery(t, query)
	if _, known := s.Latest().Key(0, 30); known {
		t.Fatal("query wait retained stale key")
	}
	other := s.send(t, 1, Event{Type: EV_SYN})
	if down, known := other.Key(1, 31); !down || !known || other.Sequence != 4 {
		t.Fatal("unaffected pending node did not commit")
	}
	close(allowQuery)
	restored := s.next(t).snapshot
	assertRecoveryPublication(t, restored)
	if restored.Sequence != 5 || restored.Timestamp != 100*time.Microsecond || restored.Generations != (Generations{Device: 3, Profile: 5}) {
		t.Fatalf("replacement metadata = %+v", restored)
	}
	if down, known := restored.Key(0, 30); down || !known {
		t.Fatal("query release lost to stale buffer")
	}
	if down, known := restored.Key(0, 31); !down || !known {
		t.Fatal("query press missing")
	}
	if _, known := restored.Key(0, 32); known {
		t.Fatal("pending lost key survived")
	}
	stick := ProjectStick(restored, profile.StickBinding{Mode: profile.StickModeXbox}, StickSource{Xbox: XboxStickSource{Node: 0, X: 0, Y: 1}})
	if !stick.Known || stick.X != -0.5 || stick.Y != 0 || stick.AxisX.Raw != -55 {
		t.Fatalf("recovered stick = %+v", stick)
	}
	queried.state.keys[30] = true
	queried.axes[0] = AxisInfo{}
	next := s.send(t, 0, Event{Type: EV_ABS, Value: -100}, Event{Type: EV_SYN, Timestamp: 50 * time.Microsecond})
	if next.Sequence != 6 || next.Timestamp != 100*time.Microsecond {
		t.Fatalf("resumed frame = %+v", next)
	}
	if down, known := next.Key(0, 30); down || !known {
		t.Fatal("replacement input storage was borrowed")
	}
	if raw, _, _, valid := restored.NormalizedAbsolute(0, 0); raw != -55 || !valid {
		t.Fatal("retained replacement was mutated")
	}
	if down, known := retained.Key(0, 30); !down || !known {
		t.Fatal("retained old state changed")
	}
	if _, known := lost.Key(0, 30); known {
		t.Fatal("retained unknown state changed")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertIntegrationStopped(t, s, "", 7, Generations{Device: 3, Profile: 5})
	if queries != 1 {
		t.Fatalf("query count=%d", queries)
	}
}
