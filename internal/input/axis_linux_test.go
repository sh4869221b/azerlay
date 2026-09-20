package input

import (
	"context"
	"errors"
	"os"
	"slices"
	"syscall"
	"testing"
	"unsafe"

	"github.com/sh4869221b/azerlay/internal/device"
)

func TestAxisInfoKernelLayout(t *testing.T) {
	t.Parallel()
	info, err := getAxisInfo(17, 1, func(fd, request uintptr, raw *[6]int32) error {
		if fd != 17 || request != 0x80184541 || unsafe.Sizeof(*raw) != 24 {
			t.Fatalf("ioctl = fd %d, request %#x, size %d", fd, request, unsafe.Sizeof(*raw))
		}
		*raw = [6]int32{123, -32768, 32767, 4, 128, 9}
		return nil
	})
	if want := (AxisInfo{Minimum: -32768, Maximum: 32767, Flat: 128, Fuzz: 4}); err != nil || info != want {
		t.Fatalf("axis info = %+v, %v; want %+v", info, err, want)
	}
}

func TestAxisInfoIoctlFailure(t *testing.T) {
	t.Parallel()
	info, err := getAxisInfo(17, 0, func(_, _ uintptr, raw *[6]int32) error {
		*raw = [6]int32{123, -100, 100, 1, 5, 0}
		return syscall.ENODEV
	})
	if !errors.Is(err, syscall.ENODEV) || info != (AxisInfo{}) {
		t.Fatalf("failed axis info = %+v, %v", info, err)
	}
}

func TestAxisInfoClosedDescriptor(t *testing.T) {
	t.Parallel()
	nodes, _ := pipeNodes(t, 1)
	if err := nodes[0].File.Close(); err != nil {
		t.Fatal(err)
	}
	if info, err := readAxisInfo(nodes[0].File, []int{0}); err == nil || info != nil {
		t.Fatalf("closed descriptor = %+v, %v", info, err)
	}
}

func TestSessionAxisMetadataOwnership(t *testing.T) {
	t.Parallel()
	nodes, writers := pipeNodes(t, 3)
	nodes[0].Node.Capabilities = map[string][]int{"abs": {0, 1}}
	nodes[1].Node.Capabilities = map[string][]int{"abs": {0}}
	want := []AxisInfo{{Minimum: -100, Maximum: 100, Flat: 3, Fuzz: 1}, {Minimum: 0, Maximum: 255, Flat: 8, Fuzz: 2}}
	source := []map[uint16]AxisInfo{{0: want[0], 1: want[0]}, {0: want[1]}}
	observed := make(chan integrationObservation, 16)
	reads := 0
	session, err := startSession(context.Background(), device.Group{}, Generations{Device: 3, Profile: 5}, sessionOps{
		open:  func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error { return nil },
		axes: func(file *os.File, codes []int) (map[uint16]AxisInfo, error) {
			index := reads
			reads++
			if index >= len(source) || file != nodes[index].File || !slices.Equal(codes, nodes[index].Node.Capabilities["abs"]) {
				t.Fatalf("axis setup did not use retained node %d", index)
			}
			return source[index], nil
		},
		observe: func(node int, event Event, snapshot *Snapshot) {
			observed <- integrationObservation{node, event, snapshot}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	s := integrationSession{session, nodes, writers, observed}
	if reads != 2 {
		t.Fatalf("metadata reads = %d, want 2", reads)
	}
	initial := s.Latest()
	source[0][0] = AxisInfo{}
	delete(source[1], 0)
	for node, want := range want {
		info, present := initial.AxisInfo(node, 0)
		if !present || info != want {
			t.Fatalf("node %d metadata = %+v, %t", node, info, present)
		}
		info.Minimum = 999
		if again, _ := initial.AxisInfo(node, 0); again != want {
			t.Fatal("accessor exposed mutable metadata")
		}
		if _, known := initial.Absolute(node, 0); known {
			t.Fatal("metadata seeded initial raw state")
		}
	}
	for _, node := range []int{-1, 2, 3} {
		if _, present := initial.AxisInfo(node, 0); present {
			t.Fatalf("unexpected metadata for node %d", node)
		}
	}
	if _, present := initial.AxisInfo(0, 2); present {
		t.Fatal("unadvertised axis metadata present")
	}
	s.send(t, 0, Event{Type: EV_ABS, Code: 0, Value: 42})
	other := s.send(t, 1, Event{Type: EV_SYN})
	if _, known := other.Absolute(0, 0); known {
		t.Fatal("other node report committed pending axis")
	}
	retained := s.send(t, 0, Event{Type: EV_SYN})
	if value, known := retained.Absolute(0, 0); !known || value != 42 {
		t.Fatalf("committed axis = %d, %t", value, known)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertIntegrationStopped(t, s, "", 3, Generations{Device: 3, Profile: 5})
	for _, snapshot := range []*Snapshot{initial, other, retained, s.Latest()} {
		for node, want := range want {
			if info, present := snapshot.AxisInfo(node, 0); !present || info != want {
				t.Fatalf("snapshot %d lost node %d metadata: %+v, %t", snapshot.Sequence, node, info, present)
			}
		}
	}
	if _, known := initial.Absolute(0, 0); known {
		t.Fatal("retained initial snapshot gained an axis value")
	}
	if value, known := retained.Absolute(0, 0); !known || value != 42 {
		t.Fatal("shutdown changed retained raw state")
	}
}

func TestSessionAxisSetupFailure(t *testing.T) {
	t.Parallel()
	nodes, _ := pipeNodes(t, 3)
	for i := range nodes {
		nodes[i].Node.Capabilities = map[string][]int{"abs": {0}}
	}
	reads := 0
	session, err := startSession(context.Background(), device.Group{}, Generations{}, sessionOps{
		open:  func(device.Group) ([]device.OpenedNode, error) { return nodes, nil },
		clock: func(*os.File) error { return nil },
		axes: func(*os.File, []int) (map[uint16]AxisInfo, error) {
			reads++
			if reads == 2 {
				return nil, errors.New("private path and raw axis data")
			}
			return map[uint16]AxisInfo{0: {Minimum: -100, Maximum: 100}}, nil
		},
	})
	if session != nil {
		t.Cleanup(func() { _ = session.Close() })
	}
	if session != nil || err == nil || err.Error() != ERR_INPUT_READ || reads != 2 {
		t.Fatalf("axis setup = %v, %v; reads %d", session, err, reads)
	}
	assertFilesClosed(t, nodes)
}
