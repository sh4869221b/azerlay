package input

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

// TestPhysicalHardwareLifecycle is opt-in: the owner operates only the selected
// left-hand device, with the official software in SOFTWARE mode. It logs no
// serial, paths, profile contents, or unrelated reports.
func TestPhysicalHardwareLifecycle(t *testing.T) {
	if os.Getenv("AZERLAY_TEST_PHYSICAL_HID") != "1" {
		t.Skip("requires owner-operated qualified physical HID and official SOFTWARE mode")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
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
	states := make(chan ManagedSnapshot, 16)
	m, err := startManaged(ctx, target, Generations{Device: 1}, managedOps{
		discover: device.Discover,
		session: sessionOps{open: target.Open, observe: func(s *Snapshot) {
			select {
			case observations <- s:
			case <-ctx.Done():
			}
		}},
		observe: func(s ManagedSnapshot) {
			select {
			case states <- s:
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
	nextState := func(want ManagedState) ManagedSnapshot {
		t.Helper()
		for {
			select {
			case state := <-states:
				if state.State == want {
					return state
				}
				if state.State == ManagedStopped {
					t.Fatal("managed reader stopped")
				}
			case <-ctx.Done():
				t.Fatal("hardware stage timed out")
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
					event, present := s.LastObservation()
					observed = s.Generations.Device == generation && present && event.RegionID == step.region && event.Down == step.down
				case <-ctx.Done():
					t.Fatal("physical observation timed out")
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
	t.Log("STAGE reopened_unknown; ready for second button sequence")
	exercise(reopened.Snapshot.Generations.Device)
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if stopped := m.Latest(); stopped.State != ManagedStopped || stopped.Snapshot.Connected || len(stopped.Snapshot.Controls()) != 0 {
		t.Fatal("close did not clear input")
	}
	t.Log("STAGE closed; production reader joined and descriptor closed")
}
