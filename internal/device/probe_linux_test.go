package device

import (
	"syscall"
	"testing"
	"unsafe"
)

func TestProbeBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		openErr, idErr, nameErr, closeErr error
		wrongDevice, wrongID              bool
		access, code                      string
		idCalls, nameCalls, closes        int
	}{
		{name: "readable", access: "readable", idCalls: 1, nameCalls: 1, closes: 1},
		{name: "permission", openErr: syscall.EACCES, access: "denied", code: ERR_DEVICE_PERMISSION},
		{name: "removed", openErr: syscall.ENOENT, access: "unavailable", code: ERR_DEVICE_DISCONNECTED},
		{name: "descriptor mismatch", wrongDevice: true, access: "unavailable", code: ERR_DEVICE_METADATA, closes: 1},
		{name: "ID contradiction", wrongID: true, access: "unavailable", code: ERR_DEVICE_METADATA, idCalls: 1, closes: 1},
		{name: "ID ioctl failed", idErr: syscall.EIO, access: "unavailable", code: ERR_DEVICE_METADATA, idCalls: 1, closes: 1},
		{name: "ID ioctl permission", idErr: syscall.EPERM, access: "denied", code: ERR_DEVICE_PERMISSION, idCalls: 1, closes: 1},
		{name: "name unsupported", nameErr: syscall.ENOTTY, access: "readable", code: ERR_DEVICE_METADATA, idCalls: 1, nameCalls: 1, closes: 1},
		{name: "name failed", nameErr: syscall.ENODEV, access: "unavailable", code: ERR_DEVICE_DISCONNECTED, idCalls: 1, nameCalls: 1, closes: 1},
		{name: "close failed", closeErr: syscall.EIO, access: "unavailable", code: ERR_DEVICE_METADATA, idCalls: 1, nameCalls: 1, closes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idCalls, nameCalls, closes := 0, 0, 0
			n := Node{Path: "synthetic-event", deviceNumber: 123, InputID: &InputID{3, 0x16d0, 0x12f7, 0x111}}
			ops := probeOps{
				open: func(path string, flags int, mode uint32) (int, error) {
					if path != n.Path || flags != syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC || mode != 0 {
						t.Fatalf("unsafe open: %q %x %o", path, flags, mode)
					}
					return 7, tc.openErr
				},
				fstat: func(fd int, st *syscall.Stat_t) error {
					st.Mode = syscall.S_IFCHR
					st.Rdev = n.deviceNumber
					if tc.wrongDevice {
						st.Rdev++
					}
					return nil
				},
				id: func(fd int) (InputID, error) {
					idCalls++
					id := *n.InputID
					if tc.wrongID {
						id.Bus++
					}
					return id, tc.idErr
				},
				name: func(fd int) (string, error) { nameCalls++; return "Synthetic name", tc.nameErr },
				close: func(fd int) error {
					if fd != 7 {
						t.Fatalf("wrong fd closed: %d", fd)
					}
					closes++
					return tc.closeErr
				},
			}
			p := ops.probe(n)
			if p.access != tc.access || p.contradiction != tc.wrongID || idCalls != tc.idCalls || nameCalls != tc.nameCalls || closes != tc.closes {
				t.Fatalf("probe=%+v calls=%d/%d/%d", p, idCalls, nameCalls, closes)
			}
			if tc.code == "" {
				if len(p.findings) != 0 {
					t.Fatalf("unexpected findings: %+v", p.findings)
				}
			} else if len(p.findings) != 1 || p.findings[0].Code != tc.code {
				t.Fatalf("findings: %+v", p.findings)
			}
			if tc.nameErr == syscall.ENOTTY && (p.name != nil || p.findings[0].Severity != "warning") {
				t.Fatal("unsupported name did not preserve sysfs value")
			}
		})
	}
}

func TestProbeInputIDLayout(t *testing.T) {
	if unsafe.Sizeof(InputID{}) != 8 {
		t.Fatal("Linux input.h layout mismatch")
	}
}
