package device

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

func reconnectResult(t *testing.T, c collector) *Result {
	t.Helper()
	if c.probe == nil {
		c.probe = func(Node) probeResult { return probeResult{access: "readable"} }
	}
	r, err := c.collect("")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestReconnectTarget(t *testing.T) {
	t.Parallel()
	f := fixtureOpen(t, 3)
	f.c.probe = func(Node) probeResult { return probeResult{access: "denied"} }
	r := reconnectResult(t, f.c)
	selected := Group{USBParent: f.group.USBParent, EventPaths: []string{f.group.EventPaths[2], f.group.EventPaths[0]}}
	target, err := NewReconnectTarget(r, selected)
	if err != nil || !target.Valid() {
		t.Fatalf("target = %+v, %v", target, err)
	}
	*r.Nodes[0].Serial = "changed-original"
	r.Nodes[0].Roles[0] = "changed-original"
	selected.EventPaths[0] = "changed-original"
	next := fixtureCollector(t)
	one := fixtureNode(t, next, "moved", 30, 1)
	fixtureNode(t, next, "moved", 20, 2)
	three := fixtureNode(t, next, "moved", 10, 3)
	group, err := target.Select(reconnectResult(t, next))
	if err != nil || !slices.Equal(group.EventPaths, []string{three, one}) {
		t.Fatalf("moved/renumbered subset = %+v, %v", group, err)
	}
	group.EventPaths[0] = "changed-result"
	again, err := target.Select(reconnectResult(t, next))
	if err != nil || !slices.Equal(again.EventPaths, []string{three, one}) {
		t.Fatalf("retained selection = %+v, %v", again, err)
	}
}

func TestReconnectTargetSelectionFailures(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		change     func(*testing.T, *openFixture, *Result)
	}{
		{"duplicate-serial", "ERR_DEVICE_AMBIGUOUS", func(t *testing.T, f *openFixture, r *Result) {
			fixtureNode(t, f.c, "duplicate", 4, 1)
			*r = *reconnectResult(t, f.c)
		}},
		{"unreadable-duplicate", "ERR_DEVICE_AMBIGUOUS", func(t *testing.T, f *openFixture, r *Result) {
			fixtureNode(t, f.c, "duplicate", 4, 1)
			f.c.probe = func(n Node) probeResult {
				if filepath.Base(n.Path) == "event4" {
					return probeResult{access: "denied"}
				}
				return probeResult{access: "readable"}
			}
			*r = *reconnectResult(t, f.c)
		}},
		{"missing-serial", ERR_DEVICE_NOT_FOUND, func(t *testing.T, f *openFixture, r *Result) {
			for i := range r.Nodes {
				r.Nodes[i].Serial = nil
			}
		}},
		{"missing-slot", ERR_DEVICE_NOT_FOUND, func(t *testing.T, f *openFixture, r *Result) { r.Groups[0].EventPaths = r.Groups[0].EventPaths[:1] }},
		{"duplicate-slot", "ERR_DEVICE_AMBIGUOUS", func(t *testing.T, f *openFixture, r *Result) {
			n := r.Nodes[0]
			n.Path += "-duplicate"
			r.Nodes = append(r.Nodes, n)
			r.Groups[0].EventPaths = append(r.Groups[0].EventPaths, n.Path)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixtureOpen(t, 2)
			r := reconnectResult(t, f.c)
			target, err := NewReconnectTarget(r, f.group)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(t, f, r)
			group, err := target.Select(r)
			var d *Diagnostic
			if !errors.As(err, &d) || d.Code != tc.code || len(group.EventPaths) != 0 || d.Target != nil || d.Remediation == "" {
				t.Fatalf("selection = %+v, %v", group, err)
			}
		})
	}
}

func TestReconnectTargetNoSerial(t *testing.T) {
	t.Parallel()
	f := fixtureOpen(t, 1)
	if err := os.Remove(filepath.Join(f.group.USBParent, "serial")); err != nil {
		t.Fatal(err)
	}
	r := reconnectResult(t, f.c)
	target, err := NewReconnectTarget(r, f.group)
	if err != nil {
		t.Fatal(err)
	}
	group, err := target.Select(r)
	if err != nil || !slices.Equal(group.EventPaths, f.group.EventPaths) {
		t.Fatalf("same port = %+v, %v", group, err)
	}
	renumbered := filepath.Join(f.c.devRoot, "event9")
	if err := os.Rename(f.group.EventPaths[0], renumbered); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(f.c.sysRoot, "class/input/event1"), filepath.Join(f.c.sysRoot, "class/input/event9")); err != nil {
		t.Fatal(err)
	}
	group, err = target.Select(reconnectResult(t, f.c))
	if err != nil || !slices.Equal(group.EventPaths, []string{renumbered}) {
		t.Fatalf("same port renumbered = %+v, %v", group, err)
	}
	other := fixtureCollector(t)
	fixtureNode(t, other, "other", 9, 1)
	group, err = target.Select(reconnectResult(t, other))
	if err == nil || err.Error() != ERR_DEVICE_NOT_FOUND || len(group.EventPaths) != 0 {
		t.Fatalf("different port = %+v, %v", group, err)
	}
}

func TestReconnectTargetMalformed(t *testing.T) {
	var zero ReconnectTarget
	if _, err := zero.Select(&Result{}); err == nil {
		t.Fatal("zero target selected a group")
	}
	if _, err := zero.open(Group{}, func(Group) ([]OpenedNode, error) { t.Fatal("zero target opened descriptors"); return nil, nil }); err == nil {
		t.Fatal("zero target opened a group")
	}
	for _, tc := range []struct {
		name   string
		change func(*Result, *Group)
	}{
		{"empty", func(r *Result, g *Group) { g.EventPaths = nil }},
		{"duplicate-path", func(r *Result, g *Group) { g.EventPaths[1] = g.EventPaths[0] }},
		{"missing-path", func(r *Result, g *Group) { g.EventPaths[1] += "-missing" }},
		{"duplicate-node", func(r *Result, g *Group) { r.Nodes = append(r.Nodes, r.Nodes[0]) }},
		{"wrong-parent", func(r *Result, g *Group) { g.USBParent += "-wrong" }},
		{"missing-identity", func(r *Result, g *Group) { r.Nodes[0].USBID = nil }},
		{"unsupported", func(r *Result, g *Group) { r.Nodes[0].Interface.Class = 255 }},
		{"conflicting-serial", func(r *Result, g *Group) { r.Nodes[0].Serial = nil }},
		{"duplicate-slot", func(r *Result, g *Group) { n := r.Nodes[0]; n.Path = r.Nodes[1].Path; r.Nodes[1] = n }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixtureOpen(t, 2)
			r := reconnectResult(t, f.c)
			tc.change(r, &f.group)
			target, err := NewReconnectTarget(r, f.group)
			if err == nil || target.Valid() {
				t.Fatalf("malformed target = %+v, %v", target, err)
			}
		})
	}
}

func TestReconnectTargetPermissionRetry(t *testing.T) {
	t.Parallel()
	f := fixtureOpen(t, 2)
	target, err := NewReconnectTarget(reconnectResult(t, f.c), f.group)
	if err != nil {
		t.Fatal(err)
	}
	denied := f.ops
	denied.open = func(path string, flags int, mode uint32) (int, error) {
		if path == f.group.EventPaths[1] {
			return -1, syscall.EACCES
		}
		return f.ops.open(path, flags, mode)
	}
	opened, err := target.open(f.group, func(g Group) ([]OpenedNode, error) { return f.c.openGroup(g, denied) })
	if opened != nil || err == nil || err.Error() != ERR_DEVICE_PERMISSION {
		t.Fatalf("permission = %v, %v", opened, err)
	}
	for fd := range f.fds {
		var st syscall.Stat_t
		if !errors.Is(syscall.Fstat(fd, &st), syscall.EBADF) || f.closes[fd] != 1 {
			t.Fatalf("permission rollback leaked fd %d", fd)
		}
	}
	group, err := target.Select(reconnectResult(t, f.c))
	if err != nil {
		t.Fatal(err)
	}
	opened, err = target.open(group, func(g Group) ([]OpenedNode, error) { return f.c.openGroup(g, f.ops) })
	if err != nil || len(opened) != 2 {
		t.Fatalf("permission retry = %v, %v", opened, err)
	}
	for _, node := range opened {
		if err := node.File.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReconnectTargetOpen(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "retained", true: "replaced"}[changed], func(t *testing.T) {
			f := fixtureOpen(t, 2)
			target, err := NewReconnectTarget(reconnectResult(t, f.c), f.group)
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				writeFile(t, filepath.Join(f.group.USBParent, "serial"), "replacement-private-serial")
			}
			opened, err := target.open(f.group, func(g Group) ([]OpenedNode, error) { return f.c.openGroup(g, f.ops) })
			if changed {
				if err == nil || err.Error() != ERR_DEVICE_METADATA || opened != nil {
					t.Fatalf("replacement = %v, %v", opened, err)
				}
			} else {
				if err != nil || len(opened) != 2 {
					t.Fatalf("open = %v, %v", opened, err)
				}
				for _, n := range opened {
					if _, err := n.File.Stat(); err != nil {
						t.Fatal(err)
					}
					if err := n.File.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !slices.Equal(f.paths, f.group.EventPaths) {
				t.Fatalf("opened unintended paths: %v", f.paths)
			}
			for fd := range f.fds {
				var st syscall.Stat_t
				if !errors.Is(syscall.Fstat(fd, &st), syscall.EBADF) {
					t.Fatalf("fd %d leaked", fd)
				}
			}
		})
	}
}
