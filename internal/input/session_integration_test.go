//go:build linux

package input

import (
	"context"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

type integrationObservation struct {
	node     int
	event    Event
	snapshot *Snapshot
}

type integrationSession struct {
	*Session
	nodes    []device.OpenedNode
	writers  []*os.File
	observed chan integrationObservation
}

func newIntegrationSession(t *testing.T, count int, generations Generations) integrationSession {
	t.Helper()
	nodes, writers := pipeNodes(t, count)
	observed := make(chan integrationObservation, 1024)
	session, err := startSession(context.Background(), device.Group{}, generations, sessionOps{
		open:  func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error { return nil },
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

func (s integrationSession) next(t *testing.T) integrationObservation {
	t.Helper()
	select {
	case observation := <-s.observed:
		return observation
	case <-time.After(3 * time.Second):
		t.Fatal("event was not reduced")
		return integrationObservation{}
	}
}

func (s integrationSession) send(t *testing.T, node int, events ...Event) *Snapshot {
	t.Helper()
	if _, err := s.writers[node].Write(nativeEvents(events...)); err != nil {
		t.Fatal(err)
	}
	var snapshot *Snapshot
	for _, event := range events {
		got := s.next(t)
		if got.node != node || got.event != event {
			t.Fatalf("reduced (%d, %+v), want (%d, %+v)", got.node, got.event, node, event)
		}
		if (got.snapshot != nil) != (event.Type == EV_SYN && event.Code == SYN_REPORT) {
			t.Fatalf("unexpected publication for %+v: %+v", event, got.snapshot)
		}
		snapshot = got.snapshot
	}
	return snapshot
}

func assertIntegrationStopped(t *testing.T, s integrationSession, code string, sequence uint64, generations Generations) {
	t.Helper()
	awaitSession(t, s.Session)
	if err := s.Err(); (code == "" && err != nil) || (code != "" && (err == nil || err.Error() != code)) {
		t.Fatalf("session error = %v, want %q", err, code)
	}
	terminal := s.Latest()
	if terminal.Connected || terminal.Sequence != sequence || terminal.Generations != generations {
		t.Fatalf("terminal metadata = %+v", terminal)
	}
	for node := range s.nodes {
		if _, known := terminal.Key(node, 30); known {
			t.Fatalf("node %d retained a key", node)
		}
		if _, known := terminal.Absolute(node, 0); known {
			t.Fatalf("node %d retained an axis", node)
		}
	}
	if _, known := terminal.Relative(0); known {
		t.Fatal("terminal retained a relative delta")
	}
	if _, present := terminal.ReportingNode(); present || len(terminal.FrameEvents()) != 0 || len(terminal.KeyTransitions()) != 0 {
		t.Fatal("terminal retained a reporting frame")
	}
	assertFilesClosed(t, s.nodes)
	_ = s.Close()
	if !reflect.DeepEqual(s.Latest(), terminal) {
		t.Fatal("publication changed after Done and repeated Close")
	}
	select {
	case extra := <-s.observed:
		t.Fatalf("unexpected extra reduced event: %+v", extra)
	default:
	}
}

func TestSessionIntegrationFrameIsolation(t *testing.T) {
	t.Parallel()
	generations := Generations{Device: 7, Profile: 11}
	s := newIntegrationSession(t, 2, generations)
	initial := s.Latest()
	a := []Event{
		{Timestamp: 20 * time.Microsecond, Type: EV_KEY, Code: 30, Value: 1},
		{Timestamp: 21 * time.Microsecond, Type: EV_ABS, Code: 0, Value: -42},
		{Timestamp: 22 * time.Microsecond, Type: EV_REL, Code: 0, Value: 3},
	}
	s.send(t, 0, a...)
	if !reflect.DeepEqual(s.Latest(), initial) {
		t.Fatal("partial A frame was published")
	}
	b := []Event{
		{Timestamp: 10 * time.Microsecond, Type: EV_KEY, Code: 30, Value: 0},
		{Timestamp: 11 * time.Microsecond, Type: EV_ABS, Code: 0, Value: 99},
		{Timestamp: 12 * time.Microsecond, Type: EV_MSC, Code: MSC_SCAN, Value: 123},
	}
	bReport := s.send(t, 1, append(slices.Clone(b), Event{Timestamp: 13 * time.Microsecond, Type: EV_SYN})...)
	if _, known := bReport.Key(0, 30); known {
		t.Fatal("B report exposed A pending key")
	}
	if _, known := bReport.Absolute(0, 0); known {
		t.Fatal("B report exposed A pending axis")
	}
	if _, known := bReport.Relative(0); known {
		t.Fatal("B report exposed A pending relative delta")
	}
	aReport := s.send(t, 0, Event{Timestamp: 23 * time.Microsecond, Type: EV_SYN})
	repeat := Event{Timestamp: 14 * time.Microsecond, Type: EV_KEY, Code: 30, Value: 2}
	repeatReport := s.send(t, 1, repeat, Event{Timestamp: 15 * time.Microsecond, Type: EV_SYN})
	for i, want := range []struct {
		snapshot  *Snapshot
		node      int
		timestamp time.Duration
		frame     []Event
		action    KeyAction
	}{
		{bReport, 1, 13 * time.Microsecond, b, KeyRelease},
		{aReport, 0, 23 * time.Microsecond, a, KeyPress},
		{repeatReport, 1, 23 * time.Microsecond, []Event{repeat}, KeyRepeat},
	} {
		got := want.snapshot
		if got.Sequence != uint64(i+1) || got.Timestamp != want.timestamp || !got.Connected || got.Generations != generations {
			t.Fatalf("report %d metadata = %+v", i, got)
		}
		if node, present := got.ReportingNode(); !present || node != want.node {
			t.Fatalf("report %d reporting node = %d, %t", i, node, present)
		}
		if !slices.Equal(got.FrameEvents(), want.frame) {
			t.Fatalf("report %d frame = %+v, want %+v", i, got.FrameEvents(), want.frame)
		}
		if transitions := got.KeyTransitions(); !slices.Equal(transitions, []KeyTransition{{Timestamp: want.frame[0].Timestamp, Code: 30, Action: want.action}}) {
			t.Fatalf("report %d transitions = %+v", i, transitions)
		}
	}
	for node, value := range []int32{-42, 99} {
		if got, known := aReport.Absolute(node, 0); !known || got != value {
			t.Fatalf("node %d axis = %d, %t", node, got, known)
		}
		if down, known := aReport.Key(node, 30); !known || down != (node == 0) {
			t.Fatalf("node %d overlapping key = %t, %t", node, down, known)
		}
		if down, known := repeatReport.Key(node, 30); !known || !down {
			t.Fatalf("node %d repeat snapshot key = %t, %t", node, down, known)
		}
	}
	if delta, known := aReport.Relative(0); !known || delta != 3 {
		t.Fatalf("A relative delta = %d, %t", delta, known)
	}
	if _, known := repeatReport.Relative(0); known {
		t.Fatal("A relative delta persisted in B frame")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertIntegrationStopped(t, s, "", 4, generations)
	if down, known := aReport.Key(0, 30); !known || !down || !slices.Equal(aReport.FrameEvents(), a) {
		t.Fatal("shutdown mutated a retained snapshot")
	}
}

func TestSessionIntegrationConcurrentNodeOrder(t *testing.T) {
	t.Parallel()
	const nodes, frames = 3, 64
	generations := Generations{Device: 3, Profile: 5}
	s := newIntegrationSession(t, nodes, generations)
	streams := make([][]Event, nodes)
	start := make(chan struct{})
	written := make(chan error, nodes)
	for node := range nodes {
		for frame := range frames {
			timestamp := time.Duration(frame*nodes+node+1) * time.Microsecond
			streams[node] = append(streams[node],
				Event{Timestamp: timestamp, Type: EV_KEY, Code: 30, Value: int32(1 - frame%2)},
				Event{Timestamp: timestamp, Type: EV_ABS, Code: 0, Value: int32(frame)},
				Event{Timestamp: timestamp, Type: EV_SYN},
			)
		}
		go func() {
			<-start
			_, err := s.writers[node].Write(nativeEvents(streams[node]...))
			written <- err
		}()
	}
	close(start)
	var positions, committed [nodes]int
	var sequence uint64
	var timestamp time.Duration
	for range nodes * frames * 3 {
		got := s.next(t)
		position := positions[got.node]
		if got.event != streams[got.node][position] {
			t.Fatalf("node %d event %d out of order: %+v", got.node, position, got.event)
		}
		positions[got.node]++
		if got.event.Type != EV_SYN {
			if got.snapshot != nil {
				t.Fatal("pending event published")
			}
			continue
		}
		sequence++
		committed[got.node]++
		timestamp = max(timestamp, got.event.Timestamp)
		snapshot := got.snapshot
		if snapshot == nil || snapshot.Sequence != sequence || snapshot.Timestamp != timestamp || snapshot.Generations != generations || !snapshot.Connected {
			t.Fatalf("concurrent report metadata = %+v", snapshot)
		}
		if node, present := snapshot.ReportingNode(); !present || node != got.node || !slices.Equal(snapshot.FrameEvents(), streams[node][position-2:position]) {
			t.Fatalf("concurrent frame mixed nodes: %+v", snapshot.FrameEvents())
		}
		for node, count := range committed {
			axis, known := snapshot.Absolute(node, 0)
			down, keyKnown := snapshot.Key(node, 30)
			if known != (count > 0) || keyKnown != known || (known && (axis != int32(count-1) || down != (count%2 == 1))) {
				t.Fatalf("node %d after %d reports: axis %d/%t, key %t/%t", node, count, axis, known, down, keyKnown)
			}
		}
	}
	for range nodes {
		select {
		case err := <-written:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent writer did not finish")
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertIntegrationStopped(t, s, "", nodes*frames+1, generations)
}

func TestSessionIntegrationPendingNodeInvalidation(t *testing.T) {
	for _, code := range []string{ERR_INPUT_READ, ERR_INPUT_DROPPED} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			generations := Generations{Device: 13, Profile: 17}
			s := newIntegrationSession(t, 2, generations)
			retained := s.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_ABS, Value: 7}, Event{Type: EV_REL, Value: 2}, Event{Type: EV_SYN})
			s.send(t, 1, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_ABS, Value: 9})
			if code == ERR_INPUT_READ {
				if err := s.writers[0].Close(); err != nil {
					t.Fatal(err)
				}
			} else if _, err := s.writers[0].Write(nativeEvents(Event{Type: EV_SYN, Code: SYN_DROPPED}, Event{Type: EV_SYN})); err != nil {
				t.Fatal(err)
			}
			assertIntegrationStopped(t, s, code, 2, generations)
			if down, known := retained.Key(0, 30); !known || !down {
				t.Fatal("failure changed retained key")
			}
			if _, known := retained.Key(1, 30); known {
				t.Fatal("pending node became visible in retained snapshot")
			}
		})
	}
}

func TestSessionIntegrationReplacementAfterDone(t *testing.T) {
	t.Parallel()
	oldGenerations := Generations{Device: 1, Profile: 2}
	old := newIntegrationSession(t, 2, oldGenerations)
	old.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_SYN})
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	assertIntegrationStopped(t, old, "", 2, oldGenerations)
	oldTerminal := old.Latest()
	newGenerations := Generations{Device: 2, Profile: 3}
	next := newIntegrationSession(t, 2, newGenerations)
	initial := next.Latest()
	if initial.Sequence != 0 || !initial.Connected || initial.Generations != newGenerations {
		t.Fatalf("replacement initial metadata = %+v", initial)
	}
	if _, known := initial.Key(0, 30); known {
		t.Fatal("replacement inherited old input state")
	}
	published := next.send(t, 1, Event{Type: EV_KEY, Code: 30, Value: 0}, Event{Type: EV_SYN})
	if published.Sequence != 1 || published.Generations != newGenerations || !reflect.DeepEqual(old.Latest(), oldTerminal) {
		t.Fatal("replacement crossed session publication or generation boundary")
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	assertIntegrationStopped(t, next, "", 2, newGenerations)
}
