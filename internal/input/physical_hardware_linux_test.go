package input

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
	"golang.org/x/sys/unix"
)

type physicalReconnectAdd struct {
	path  string
	at    time.Duration
	valid bool
}

func scanPhysicalReconnectMonitor(reader io.Reader, ready func(), added func(physicalReconnectAdd)) error {
	scanner := bufio.NewScanner(reader)
	readiness := false
	header, action := false, ""
	var event physicalReconnectAdd
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "KERNEL - the kernel uevent":
			readiness = true
		case line == "":
			if readiness {
				ready()
				readiness = false
			}
			if header && action == "add" {
				added(event)
			}
			header, action, event = false, "", physicalReconnectAdd{}
		case strings.HasPrefix(line, "KERNEL"):
			header = true
			left, right := strings.IndexByte(line, '['), strings.IndexByte(line, ']')
			if left >= 0 && right > left {
				at, err := time.ParseDuration(line[left+1:right] + "s")
				event.at, event.valid = at, err == nil && at >= 0
			}
		case strings.HasPrefix(line, "ACTION="):
			action = strings.TrimPrefix(line, "ACTION=")
		case strings.HasPrefix(line, "DEVPATH="):
			event.path = "/sys" + strings.TrimPrefix(line, "DEVPATH=")
		}
	}
	if scanner.Err() != nil || header {
		return errors.New("monitor stream incomplete")
	}
	return nil
}

func physicalReconnectElapsed(adds []physicalReconnectAdd, endpoint string, degraded, connected time.Duration) (time.Duration, error) {
	count := 0
	var elapsed time.Duration
	for _, event := range adds {
		if event.path != endpoint {
			continue
		}
		if !event.valid {
			return 0, errors.New("matching add timestamp missing or malformed")
		}
		if event.at < degraded {
			continue
		}
		count++
		elapsed = connected - event.at
	}
	if count != 1 {
		return 0, errors.New("reconnect requires exactly one matching add")
	}
	if elapsed < 0 || elapsed > 2*time.Second {
		return elapsed, errors.New("observed reconnect interval " + elapsed.String() + " outside zero to two seconds")
	}
	return elapsed, nil
}

// TestPhysicalHardwareLifecycle is opt-in: the owner operates only the selected
// left-hand device, with the official software in SOFTWARE mode. It logs no
// serial, paths, profile contents, or unrelated reports.
func TestPhysicalHardwareLifecycle(t *testing.T) {
	if os.Getenv("AZERLAY_TEST_PHYSICAL_HID") != "1" {
		t.Skip("requires owner-operated qualified physical HID and official SOFTWARE mode")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	version, err := exec.CommandContext(ctx, "udevadm", "--version").Output()
	if err != nil {
		t.Fatal("udevadm unavailable")
	}
	t.Logf("TOOL udevadm=%s", strings.TrimSpace(string(version)))
	monitorCtx, stopMonitor := context.WithCancel(ctx)
	monitor := exec.CommandContext(monitorCtx, "udevadm", "monitor", "--kernel", "--subsystem-match=hidraw", "--property")
	stream, err := monitor.StdoutPipe()
	if err != nil {
		stopMonitor()
		t.Fatal("monitor pipe failed")
	}
	if err := monitor.Start(); err != nil {
		stopMonitor()
		t.Fatal("monitor start failed")
	}
	ready, monitorDone := make(chan struct{}), make(chan struct{})
	addChanged := make(chan struct{}, 1)
	var addMu sync.Mutex
	var adds []physicalReconnectAdd
	var monitorErr error
	go func() {
		defer close(monitorDone)
		scanErr := scanPhysicalReconnectMonitor(stream, func() { close(ready) }, func(event physicalReconnectAdd) {
			addMu.Lock()
			adds = append(adds, event)
			addMu.Unlock()
			select {
			case addChanged <- struct{}{}:
			default:
			}
		})
		waitErr := monitor.Wait()
		if monitorCtx.Err() == nil || waitErr == nil {
			monitorErr = errors.New("monitor exited before cancellation")
		} else if scanErr != nil {
			monitorErr = scanErr
		}
	}()
	t.Cleanup(func() {
		stopMonitor()
		<-monitorDone
		if monitorErr != nil {
			t.Error(monitorErr)
		}
	})
	select {
	case <-ready:
	case <-monitorDone:
		t.Fatal("monitor exited before readiness")
	case <-ctx.Done():
		t.Fatal("monitor readiness timed out")
	}
	result, failure := device.Discover()
	if failure != nil {
		t.Fatal(failure.Code)
	}
	if len(result.Groups) != 1 || !result.Groups[0].Complete {
		t.Fatal("requires exactly one qualified physical HID group")
	}
	target, err := device.NewReconnectTarget(result, result.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	observations := make(chan *Snapshot, 64)
	type timedState struct {
		ManagedSnapshot
		at       time.Duration
		endpoint string
		clockErr error
	}
	states := make(chan timedState, 16)
	var endpoint string
	m, err := startManaged(ctx, target, Generations{Device: 1, Profile: 9}, managedOps{
		discover: device.Discover,
		session: sessionOps{open: func(group device.Group) ([]device.OpenedNode, error) {
			nodes, err := target.Open(group)
			if err != nil {
				return nil, err
			}
			endpoint, err = filepath.EvalSymlinks(filepath.Join("/sys/class/hidraw", filepath.Base(nodes[0].Node.Path)))
			if err != nil {
				for _, node := range nodes {
					err = errors.Join(err, node.File.Close())
				}
				return nil, errors.New("fresh hidraw endpoint resolution failed")
			}
			return nodes, nil
		}, observe: func(s *Snapshot) {
			select {
			case observations <- s:
			case <-ctx.Done():
			}
		}},
		observe: func(s ManagedSnapshot) {
			var now unix.Timespec
			clockErr := unix.ClockGettime(unix.CLOCK_MONOTONIC, &now)
			select {
			case states <- timedState{s, time.Duration(now.Nano()), endpoint, clockErr}:
			case <-ctx.Done():
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if err := m.Close(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	nextState := func(want ManagedState) timedState {
		t.Helper()
		for {
			select {
			case state := <-states:
				if state.clockErr != nil || state.Snapshot.Generations.Profile != 9 {
					t.Fatal("clock failed or Profile generation changed")
				}
				if state.State == want {
					return state
				}
				if state.State == ManagedStopped {
					t.Fatal("managed reader stopped")
				}
			case <-ctx.Done():
				t.Fatal("hardware stage timed out")
			case <-monitorDone:
				t.Fatal("monitor exited during hardware stage")
			}
		}
	}
	initial := nextState(ManagedConnected)
	if initial.Snapshot.Availability != Unconfirmed || len(initial.Snapshot.Controls()) != 0 {
		t.Fatal("initial state is not unknown")
	}
	exercise := func(generation uint64) {
		t.Helper()
		for _, step := range []struct {
			id     int
			region string
			down   bool
		}{{15, "grid.c4.r3", true}, {15, "grid.c4.r3", false}, {16, "grid.c4.r2", true}, {16, "grid.c4.r2", false}} {
			t.Logf("ACTION source=%d down=%t", step.id, step.down)
			observed := false
			for !observed {
				select {
				case s := <-observations:
					if s.Generations.Profile != 9 {
						t.Fatal("report changed Profile generation")
					}
					event, present := s.LastObservation()
					observed = s.Generations.Device == generation && present && event.RegionID == step.region && event.Down == step.down
				case <-ctx.Done():
					t.Fatal("physical observation timed out")
				case <-monitorDone:
					t.Fatal("monitor exited during physical observation")
				}
			}
			t.Logf("OBSERVED source=%d down=%t", step.id, step.down)
		}
	}
	t.Log("STAGE initial_unknown; ready for first button sequence")
	exercise(initial.Snapshot.Generations.Device)
	t.Log("STAGE disconnect_requested; unplug the selected device")
	disconnected := nextState(ManagedDegraded)
	if disconnected.Snapshot.Connected || len(disconnected.Snapshot.Controls()) != 0 {
		t.Fatal("disconnect retained known state")
	}
	t.Log("STAGE disconnected_unknown; reconnect and restore official SOFTWARE mode if needed")
	reopened := nextState(ManagedConnected)
	if reopened.Snapshot.Generations.Device <= initial.Snapshot.Generations.Device || reopened.Snapshot.Availability != Unconfirmed || len(reopened.Snapshot.Controls()) != 0 {
		t.Fatal("reopen retained stale observations")
	}
	var elapsed time.Duration
	for {
		addMu.Lock()
		elapsed, err = physicalReconnectElapsed(adds, reopened.endpoint, disconnected.at, reopened.at)
		matched := false
		for _, event := range adds {
			matched = matched || event.path == reopened.endpoint && (!event.valid || event.at >= disconnected.at)
		}
		addMu.Unlock()
		if matched {
			break
		}
		select {
		case <-addChanged:
		case <-monitorDone:
			t.Fatal("monitor exited before matching add")
		case <-ctx.Done():
			t.Fatal("matching add timed out")
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("RESULT KERNEL_add_receive_handler_to_ManagedConnected=%s within_2s=true", elapsed)
	t.Log("STAGE reopened_unknown; ready for second button sequence")
	exercise(reopened.Snapshot.Generations.Device)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if stopped := m.Latest(); stopped.State != ManagedStopped || stopped.Snapshot.Generations.Profile != 9 || stopped.Snapshot.Connected || len(stopped.Snapshot.Controls()) != 0 {
		t.Fatal("close did not clear input")
	}
	stopMonitor()
	<-monitorDone
	if monitorErr != nil {
		t.Fatal(monitorErr)
	}
	addMu.Lock()
	_, err = physicalReconnectElapsed(adds, reopened.endpoint, disconnected.at, reopened.at)
	addMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("STAGE closed; production reader joined and descriptor closed")
}

func TestPhysicalReconnectTiming(t *testing.T) {
	for _, tc := range []struct {
		name, stamp, path string
		ok                bool
	}{
		{"matching", "[11.250000]", "/devices/selected/hidraw/hidraw9", true},
		{"unrelated", "[11.250000]", "/devices/other/hidraw/hidraw9", false},
		{"missing", "", "/devices/selected/hidraw/hidraw9", false},
		{"malformed", "[invalid]", "/devices/selected/hidraw/hidraw9", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := "monitor will print the received events for:\nKERNEL - the kernel uevent\n\nKERNEL" + tc.stamp + " add endpoint (hidraw)\nACTION=add\nDEVPATH=" + tc.path + "\n\n"
			ready := false
			var adds []physicalReconnectAdd
			if err := scanPhysicalReconnectMonitor(strings.NewReader(text), func() { ready = true }, func(event physicalReconnectAdd) { adds = append(adds, event) }); err != nil || !ready {
				t.Fatal("monitor fixture failed", err)
			}
			elapsed, err := physicalReconnectElapsed(adds, "/sys/devices/selected/hidraw/hidraw9", 10*time.Second, 12*time.Second)
			if (err == nil) != tc.ok || tc.ok && elapsed != 750*time.Millisecond {
				t.Fatal("measurement selection", elapsed, err)
			}
		})
	}
}
