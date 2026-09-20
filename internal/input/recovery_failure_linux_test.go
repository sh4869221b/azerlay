package input

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/sh4869221b/azerlay/internal/device"
)

func TestRecoveryQueryFailure(t *testing.T) {
	for _, laterAxis := range []bool{false, true} {
		t.Run(map[bool]string{false: "default-pipe-ioctl", true: "later-axis"}[laterAxis], func(t *testing.T) {
			t.Parallel()
			var recover func(context.Context, device.OpenedNode) (recoveryState, time.Duration, error)
			if laterAxis {
				recover = func(ctx context.Context, node device.OpenedNode) (recoveryState, time.Duration, error) {
					timestamp, err := drainEvents(ctx, node.File, syscall.Read)
					if err != nil {
						return recoveryState{}, timestamp, err
					}
					state, err := queryRecoveryState(node, recoveryIoctls{
						keys: func(_, _ uintptr, raw *[96]byte) (int, error) { raw[3] = 255; return len(raw), nil },
						axes: func(_, request uintptr, raw *[6]int32) error {
							if request == 0x80184541 {
								return syscall.ENODEV
							}
							*raw = [6]int32{55, -100, 100}
							return nil
						},
					})
					return state, timestamp, err
				}
			}
			s := newRecoverySession(t, recover)
			retained := s.send(t, 0, Event{Type: EV_KEY, Code: 30, Value: 1}, Event{Type: EV_SYN})
			if _, err := s.writers[0].Write(nativeEvents(Event{Type: EV_SYN, Code: SYN_DROPPED}, Event{Type: EV_SYN})); err != nil {
				t.Fatal(err)
			}
			lost := s.next(t).snapshot
			assertRecoveryPublication(t, lost)
			assertIntegrationStopped(t, s, ERR_INPUT_READ, 3, Generations{Device: 3, Profile: 5})
			if laterAxis && !errors.Is(s.Err(), syscall.ENODEV) {
				t.Fatalf("lost disconnection cause: %v", s.Err())
			}
			if _, known := lost.Key(0, 30); known {
				t.Fatal("failed query published partial keys")
			}
			if down, known := retained.Key(0, 30); !down || !known {
				t.Fatal("query failure mutated retained state")
			}
		})
	}
}

func TestRecoveryCancellation(t *testing.T) {
	for _, phase := range []string{"drain", "query-result"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			entered, exited := make(chan struct{}), make(chan struct{})
			s := newRecoverySession(t, func(ctx context.Context, node device.OpenedNode) (recoveryState, time.Duration, error) {
				defer close(exited)
				if phase == "drain" {
					timestamp, err := drainEvents(ctx, node.File, func(int, []byte) (int, error) { close(entered); <-ctx.Done(); return 0, syscall.EINTR })
					return recoveryState{}, timestamp, err
				}
				timestamp, err := drainEvents(ctx, node.File, syscall.Read)
				if err != nil {
					return recoveryState{}, timestamp, err
				}
				close(entered)
				<-ctx.Done()
				return recoveryState{state: nodeState{keys: map[uint16]bool{30: true}}}, timestamp, nil
			})
			if _, err := s.writers[0].Write(nativeEvents(Event{Type: EV_SYN, Code: SYN_DROPPED}, Event{Type: EV_SYN})); err != nil {
				t.Fatal(err)
			}
			assertRecoveryPublication(t, s.next(t).snapshot)
			awaitRecovery(t, entered)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			awaitRecovery(t, exited)
			assertIntegrationStopped(t, s, "", 2, Generations{Device: 3, Profile: 5})
		})
	}
}

func TestRecoveryDrain(t *testing.T) {
	for _, mode := range []string{"queued-frames", "truncated", "eof"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			nodes, writers := pipeNodes(t, 1)
			if mode == "eof" {
				if err := writers[0].Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				data := []byte{1, 2, 3}
				if mode == "queued-frames" {
					var events []Event
					for i := range 70 {
						events = append(events, Event{Type: EV_SYN, Timestamp: time.Duration(i+1) * time.Microsecond})
					}
					data = nativeEvents(events...)
				}
				if _, err := writers[0].Write(data); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			timestamp, err := drainEvents(context.Background(), nodes[0].File, func(fd int, data []byte) (int, error) {
				calls++
				if calls == 1 {
					return 0, syscall.EINTR
				}
				return syscall.Read(fd, data)
			})
			if mode == "queued-frames" {
				if err != nil || timestamp != 70*time.Microsecond || calls != 4 {
					t.Fatalf("drain = %v, %v; calls=%d", timestamp, err, calls)
				}
			} else if err == nil || err.Error() != ERR_INPUT_READ {
				t.Fatalf("invalid drain = %v", err)
			}
		})
	}
}
