package device

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

type openFixture struct {
	c      collector
	group  Group
	ops    probeOps
	paths  []string
	fds    map[int]string
	closes map[int]int
}

func fixtureOpen(t *testing.T, count int) *openFixture {
	t.Helper()
	f := &openFixture{c: fixtureCollector(t), fds: map[int]string{}, closes: map[int]int{}}
	f.group.USBParent = filepath.Join(f.c.sysRoot, "devices/hub/unit")
	for i := 1; i <= count; i++ {
		f.group.EventPaths = append(f.group.EventPaths, fixtureNode(t, f.c, "unit", i, i))
	}
	f.group.Complete = count == 3
	f.c.stat = func(path string, st *syscall.Stat_t) error {
		number, ok := eventNumber(filepath.Base(path))
		if !ok {
			return syscall.ENOENT
		}
		st.Mode, st.Rdev = syscall.S_IFCHR, 13<<8|64+number
		return nil
	}
	f.ops = probeOps{
		open: func(path string, flags int, mode uint32) (int, error) {
			if flags != syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC || mode != 0 {
				t.Fatalf("unsafe flags: %x mode: %o", flags, mode)
			}
			fd, err := syscall.Open(path, flags, mode)
			if err == nil {
				f.paths = append(f.paths, path)
				f.fds[fd] = path
			}
			return fd, err
		},
		fstat: func(fd int, st *syscall.Stat_t) error {
			path, ok := f.fds[fd]
			if !ok {
				t.Fatalf("unopened descriptor: %d", fd)
			}
			return f.c.stat(path, st)
		},
		id: func(fd int) (InputID, error) {
			if _, ok := f.fds[fd]; !ok {
				t.Fatalf("unopened descriptor: %d", fd)
			}
			return InputID{3, 0x16d0, 0x12f7, 0x111}, nil
		},
		close: func(fd int) error {
			f.closes[fd]++
			return syscall.Close(fd)
		},
	}
	return f
}

func TestOpenGroupSelectedOwnership(t *testing.T) {
	for _, count := range []int{1, 3} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			f := fixtureOpen(t, count)
			fixtureNode(t, f.c, "unselected", 4, 1)
			if count == 3 {
				f.group.EventPaths[0], f.group.EventPaths[2] = f.group.EventPaths[2], f.group.EventPaths[0]
			}
			opened, err := f.c.openGroup(f.group, f.ops)
			if err != nil {
				t.Fatal(err)
			}
			if len(opened) != count || !reflect.DeepEqual(f.paths, f.group.EventPaths) || len(f.closes) != 0 {
				t.Fatalf("opened=%v paths=%v closes=%v", opened, f.paths, f.closes)
			}
			for i, n := range opened {
				fd := int(n.File.Fd())
				if f.fds[fd] != f.group.EventPaths[i] || n.Node.Path != f.group.EventPaths[i] || n.Node.Admission != "admitted" || n.Node.Access != "readable" || *n.Node.USBParent != f.group.USBParent {
					t.Fatalf("wrong transferred metadata/descriptor: %+v", n)
				}
				if _, err := n.File.Stat(); err != nil {
					t.Fatalf("transferred descriptor already closed: %v", err)
				}
				if err := n.File.Close(); err != nil {
					t.Fatal(err)
				}
				var st syscall.Stat_t
				if err := syscall.Fstat(fd, &st); !errors.Is(err, syscall.EBADF) {
					t.Fatalf("caller did not own descriptor: %v", err)
				}
			}
		})
	}
}

func TestOpenGroupRejectsAndRollsBack(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *openFixture)
		code   string
		opens  int
	}{
		{"empty group", func(t *testing.T, f *openFixture) { f.group.EventPaths = nil }, ERR_DEVICE_METADATA, 0},
		{"empty path", func(t *testing.T, f *openFixture) { f.group.EventPaths[1] = "" }, ERR_DEVICE_METADATA, 0},
		{"duplicate path", func(t *testing.T, f *openFixture) { f.group.EventPaths[1] = f.group.EventPaths[0] }, ERR_DEVICE_METADATA, 0},
		{"wrong parent", func(t *testing.T, f *openFixture) { f.group.USBParent += "-stale" }, ERR_DEVICE_METADATA, 0},
		{"unrelated keyboard", func(t *testing.T, f *openFixture) {
			writeFile(t, filepath.Join(f.group.USBParent, "idVendor"), "1234")
		}, ERR_DEVICE_UNSUPPORTED, 0},
		{"forged complete group", func(t *testing.T, f *openFixture) {
			f.group.Complete = true
			writeFile(t, filepath.Join(f.group.USBParent, "interface-1/bInterfaceClass"), "ff")
		}, ERR_DEVICE_UNSUPPORTED, 0},
		{"later permission", func(t *testing.T, f *openFixture) {
			open := f.ops.open
			f.ops.open = func(path string, flags int, mode uint32) (int, error) {
				if path == f.group.EventPaths[1] {
					return -1, syscall.EACCES
				}
				return open(path, flags, mode)
			}
		}, ERR_DEVICE_PERMISSION, 1},
		{"wrong descriptor", func(t *testing.T, f *openFixture) {
			fstat := f.ops.fstat
			f.ops.fstat = func(fd int, st *syscall.Stat_t) error {
				err := fstat(fd, st)
				st.Rdev++
				return err
			}
		}, ERR_DEVICE_METADATA, 1},
		{"wrong ID", func(t *testing.T, f *openFixture) {
			f.ops.id = func(int) (InputID, error) { return InputID{3, 1, 1, 1}, nil }
		}, ERR_DEVICE_METADATA, 1},
		{"metadata changed after open", func(t *testing.T, f *openFixture) {
			id := f.ops.id
			f.ops.id = func(fd int) (InputID, error) {
				writeFile(t, filepath.Join(f.group.USBParent, "idVendor"), "1234")
				return id(fd)
			}
		}, ERR_DEVICE_UNSUPPORTED, 1},
		{"parent moved after open", func(t *testing.T, f *openFixture) {
			id := f.ops.id
			f.ops.id = func(fd int) (InputID, error) {
				old := filepath.Join(f.group.USBParent, "interface-1")
				moved := filepath.Join(f.c.sysRoot, "devices/hub/other/interface-1")
				fixtureNode(t, f.c, "other", 4, 2)
				if err := os.Rename(old, moved); err != nil {
					t.Fatal(err)
				}
				event := filepath.Join(moved, "hid/input/input1/event1")
				for _, path := range []string{filepath.Join(f.c.sysRoot, "class/input/event1"), filepath.Join(f.c.sysRoot, "dev/char/13:65"), filepath.Join(event, "device")} {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				link(t, event, filepath.Join(f.c.sysRoot, "class/input/event1"))
				link(t, event, filepath.Join(f.c.sysRoot, "dev/char/13:65"))
				link(t, filepath.Dir(event), filepath.Join(event, "device"))
				return id(fd)
			}
		}, ERR_DEVICE_METADATA, 1},
		{"path remapped after open", func(t *testing.T, f *openFixture) {
			id := f.ops.id
			stat := f.c.stat
			remapped := false
			f.c.stat = func(path string, st *syscall.Stat_t) error {
				if remapped {
					st.Mode, st.Rdev = syscall.S_IFCHR, 13<<8|66
					return nil
				}
				return stat(path, st)
			}
			f.ops.id = func(fd int) (InputID, error) {
				remapped = true
				return id(fd)
			}
			f.ops.fstat = func(_ int, st *syscall.Stat_t) error {
				st.Mode, st.Rdev = syscall.S_IFCHR, 13<<8|65
				return nil
			}
		}, ERR_DEVICE_METADATA, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixtureOpen(t, 2)
			tc.change(t, f)
			opened, err := f.c.openGroup(f.group, f.ops)
			var d *Diagnostic
			if opened != nil || !errors.As(err, &d) || d.Code != tc.code || err.Error() != tc.code {
				t.Fatalf("opened=%v err=%v diagnostic=%+v", opened, err, d)
			}
			if len(f.paths) != tc.opens || len(f.closes) != tc.opens {
				t.Fatalf("opens=%v closes=%v", f.paths, f.closes)
			}
			for fd := range f.fds {
				var st syscall.Stat_t
				if f.closes[fd] != 1 || !errors.Is(syscall.Fstat(fd, &st), syscall.EBADF) {
					t.Fatalf("descriptor %d not closed exactly once", fd)
				}
			}
		})
	}
}
