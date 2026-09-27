package device

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRawOpenOwnershipAndRollback(t *testing.T) {
	for _, tc := range []string{"success", "permission", "identity", "descriptor", "replacement", "fstat", "close"} {
		t.Run(tc, func(t *testing.T) {
			c := fixtureCollector(t)
			path := fixtureNode(t, c, "unit", 2, 4)
			r, d := c.collect("")
			if d != nil {
				t.Fatal(d)
			}
			opens, closes := 0, 0
			fd := -1
			ops := probeOps{
				open: func(p string, flags int, mode uint32) (int, error) {
					if p != path || flags != syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC || mode != 0 {
						t.Fatal("unsafe open")
					}
					opens++
					if tc == "permission" {
						return -1, syscall.EACCES
					}
					var err error
					fd, err = syscall.Open(p, flags, mode)
					return fd, err
				},
				fstat: func(_ int, st *syscall.Stat_t) error {
					st.Mode = syscall.S_IFCHR
					st.Rdev = 240<<8 | 2
					if tc == "fstat" {
						st.Rdev++
					}
					return nil
				},
				id: func(int) (RawID, error) {
					if tc == "identity" {
						return RawID{}, nil
					}
					if tc == "replacement" {
						writeFile(t, filepath.Join(r.Groups[0].USBParent, "bcdDevice"), "0112")
					}
					return RawID{3, 0x16d0, 0x12f7}, nil
				},
				descriptor: func(int) ([]byte, error) {
					if tc == "descriptor" || tc == "close" {
						return nil, syscall.EIO
					}
					return []byte(physicalDescriptor), nil
				},
				close: func(n int) error {
					closes++
					err := syscall.Close(n)
					if tc == "close" {
						return errors.Join(err, syscall.EIO)
					}
					return err
				},
			}
			opened, err := c.openGroup(r.Groups[0], ops)
			if tc == "success" {
				if err != nil || len(opened) != 1 || closes != 0 {
					t.Fatalf("%v %v", opened, err)
				}
				if err := opened[0].File.Close(); err != nil {
					t.Fatal(err)
				}
			} else if err == nil || opened != nil || closes != map[bool]int{true: 0, false: 1}[tc == "permission"] {
				t.Fatalf("%v %v closes=%d", opened, err, closes)
			}
			if opens != 1 {
				t.Fatal(opens)
			}
			if fd >= 0 {
				var st syscall.Stat_t
				if !errors.Is(syscall.Fstat(fd, &st), syscall.EBADF) {
					t.Fatal("fd leaked")
				}
			}
		})
	}
}
func TestRawProbeDoesNotRead(t *testing.T) {
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 4)
	m := c.metadata(path, "hidraw1")
	closed := 0
	ops := probeOps{open: func(p string, flags int, _ uint32) (int, error) {
		if p != path || flags != syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC {
			t.Fatal("unsafe open")
		}
		return 7, nil
	}, fstat: func(_ int, st *syscall.Stat_t) error {
		st.Mode = syscall.S_IFCHR
		st.Rdev = m.node.deviceNumber
		return nil
	}, id: func(int) (RawID, error) { return RawID{3, 0x16d0, 0x12f7}, nil }, descriptor: func(int) ([]byte, error) { return []byte(physicalDescriptor), nil }, name: func(int) (string, error) { return "synthetic", nil }, close: func(int) error { closed++; return nil }}
	p := ops.probe(m.node)
	if p.access != "readable" || closed != 1 {
		t.Fatalf("%+v closes=%d", p, closed)
	}
}
