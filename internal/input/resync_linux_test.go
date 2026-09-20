package input

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"syscall"
	"testing"
)

func TestReadRecoveryState(t *testing.T) {
	t.Parallel()
	nodes, _ := pipeNodes(t, 1)
	node := nodes[0]
	node.Node.Capabilities = map[string][]int{"key": {30, 31, 0x2ff, 0x300}, "abs": {0, 0x3f, 0x40}}
	keyCalls, axisCalls := 0, 0
	ops := recoveryIoctls{
		keys: func(fd, request uintptr, raw *[96]byte) (int, error) {
			keyCalls++
			if fd != node.File.Fd() || request != 0x80604518 {
				t.Fatalf("key ioctl fd/request = %d/%#x", fd, request)
			}
			raw[30/8] |= 1 << (30 % 8)
			raw[32/8] |= 1 << (32 % 8)
			raw[0x2ff/8] |= 1 << (0x2ff % 8)
			return len(raw), nil
		},
		axes: func(fd, request uintptr, raw *[6]int32) error {
			axisCalls++
			if fd != node.File.Fd() || (request != 0x80184540 && request != 0x8018457f) {
				t.Fatalf("axis ioctl fd/request = %d/%#x", fd, request)
			}
			*raw = [6]int32{-55, -100, 100, 3, 10, 9}
			return nil
		},
	}
	first, err := queryRecoveryState(node, ops)
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(first.state.keys, map[uint16]bool{30: true, 31: false, 0x2ff: true}) ||
		!maps.Equal(first.state.absolute, map[uint16]int32{0: -55, 0x3f: -55}) || keyCalls != 1 || axisCalls != 2 {
		t.Fatalf("recovery = %+v, calls %d/%d", first, keyCalls, axisCalls)
	}
	second, err := queryRecoveryState(node, ops)
	if err != nil {
		t.Fatal(err)
	}
	second.state.keys[30] = false
	second.state.absolute[0] = 99
	second.axes[0] = AxisInfo{}
	if !first.state.keys[30] || first.state.absolute[0] != -55 || first.axes[0] != (AxisInfo{Minimum: -100, Maximum: 100, Flat: 10, Fuzz: 3}) {
		t.Fatal("second result mutated retained recovery state")
	}
}

func TestRecoveryStateNormalization(t *testing.T) {
	t.Parallel()
	nodes, _ := pipeNodes(t, 1)
	nodes[0].Node.Capabilities = map[string][]int{"abs": {0}}
	state, err := queryRecoveryState(nodes[0], recoveryIoctls{
		keys: func(uintptr, uintptr, *[96]byte) (int, error) { t.Fatal("unadvertised key query"); return 0, nil },
		axes: func(_, _ uintptr, raw *[6]int32) error {
			*raw = [6]int32{-55, -100, 100, 3, 10, 0}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{nodes: []nodeState{state.state}, axisInfo: []map[uint16]AxisInfo{state.axes}}
	if raw, normalized, known, valid := snapshot.NormalizedAbsolute(0, 0); raw != -55 || normalized != -0.5 || !known || !valid {
		t.Fatalf("normalized = %d, %g, %t, %t", raw, normalized, known, valid)
	}
	if _, _, known, valid := snapshot.NormalizedAbsolute(0, 1); known || valid {
		t.Fatal("unadvertised axis became known")
	}
}

func TestReadRecoveryStateFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		keyErr    error
		keyBytes  int
		axisErr   error
		wantCause error
	}{
		{"permission", &os.PathError{Op: "ioctl", Path: "private-device-path", Err: syscall.EACCES}, 96, nil, fs.ErrPermission},
		{"revoked", syscall.ENODEV, 96, nil, syscall.ENODEV},
		{"short-bitmap", nil, 1, nil, nil},
		{"later-axis", nil, 96, syscall.ENXIO, syscall.ENODEV},
		{"private-error", nil, 96, errors.New("private raw axis contents"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			nodes, _ := pipeNodes(t, 1)
			nodes[0].Node.Capabilities = map[string][]int{"key": {30}, "abs": {0, 1}}
			axisCalls := 0
			state, err := queryRecoveryState(nodes[0], recoveryIoctls{
				keys: func(_, _ uintptr, raw *[96]byte) (int, error) { raw[3] = 255; return tc.keyBytes, tc.keyErr },
				axes: func(_, _ uintptr, raw *[6]int32) error {
					axisCalls++
					*raw = [6]int32{-55, -100, 100, 3, 10, 0}
					if axisCalls == 2 {
						return tc.axisErr
					}
					return nil
				},
			})
			var inputErr *InputError
			if !errors.As(err, &inputErr) || err.Error() != ERR_INPUT_READ || state.state.keys != nil || state.state.absolute != nil || state.axes != nil {
				t.Fatalf("failed query = %+v, %v", state, err)
			}
			if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Fatalf("missing typed cause %v: %v", tc.wantCause, err)
			}
			if tc.wantCause == nil && errors.Unwrap(err) != nil {
				t.Fatalf("retained private error: %v", errors.Unwrap(err))
			}
			wantCalls := 2
			if tc.keyErr != nil || tc.keyBytes != 96 {
				wantCalls = 0
			}
			if axisCalls != wantCalls {
				t.Fatalf("axis calls = %d, want %d", axisCalls, wantCalls)
			}
			if _, err := nodes[0].File.Stat(); err != nil {
				t.Fatalf("query closed caller-owned descriptor: %v", err)
			}
		})
	}
}

func TestReadRecoveryStateDescriptor(t *testing.T) {
	t.Parallel()
	nodes, _ := pipeNodes(t, 1)
	nodes[0].Node.Capabilities = map[string][]int{"key": {30}}
	for _, closed := range []bool{false, true} {
		if closed {
			if err := nodes[0].File.Close(); err != nil {
				t.Fatal(err)
			}
		}
		state, err := readRecoveryState(nodes[0])
		if err == nil || err.Error() != ERR_INPUT_READ || state.state.keys != nil || state.state.absolute != nil || state.axes != nil {
			t.Fatalf("pipe/closed query = %+v, %v", state, err)
		}
	}
}
