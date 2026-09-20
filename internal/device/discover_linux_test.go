package device

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func hasCode(r *Result, code, severity string) bool {
	for _, d := range r.Diagnostics {
		if d.Code == code && d.Severity == severity {
			return true
		}
	}
	return false
}

func TestDiscoverAdmissionBeforeProbe(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	for i := 1; i <= 3; i++ {
		fixtureNode(t, c, "unit", i, i)
	}
	unrelated := fixtureNode(t, c, "unrelated", 4, 1)
	m := c.metadata(unrelated, filepath.Base(unrelated))
	writeFile(t, filepath.Join(*m.node.USBParent, "idVendor"), "1234")
	writeFile(t, filepath.Join(*m.node.SysfsPath, "id/vendor"), "1234")
	unsupported := fixtureNode(t, c, "unsupported", 5, 1)
	m = c.metadata(unsupported, filepath.Base(unsupported))
	writeFile(t, filepath.Join(*m.node.USBParent, "bcdDevice"), "0112")
	calls := 0
	c.probe = func(n Node) probeResult {
		calls++
		if n.Path == unrelated || n.Path == unsupported {
			t.Fatal("excluded node probed")
		}
		return probeResult{access: "readable"}
	}
	r, err := c.collect("")
	if err != nil || !r.OK() || calls != 3 || len(r.Groups) != 1 || !r.Groups[0].Complete || len(r.Nodes) != 4 || !hasCode(r, ERR_DEVICE_UNSUPPORTED, "warning") {
		t.Fatalf("result=%+v err=%v calls=%d", r, err, calls)
	}
}

func TestDiscoverPartialAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name              string
		probeErr          error
		contradiction     bool
		ok                bool
		admission, access string
	}{
		{"partial readable", nil, false, true, "admitted", "readable"},
		{"denied", syscall.EACCES, false, false, "admitted", "denied"},
		{"disconnected", syscall.ENOENT, false, false, "admitted", "unavailable"},
		{"contradictory", nil, true, false, "indeterminate", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixtureCollector(t)
			path := fixtureNode(t, c, "unit", 1, 1)
			c.probe = func(n Node) probeResult {
				if tc.contradiction {
					return probeResult{access: "unavailable", contradiction: true, findings: []Diagnostic{diagnostic(ERR_DEVICE_METADATA, "probe", &path)}}
				}
				if tc.probeErr != nil {
					d := boundaryDiagnostic(tc.probeErr, "probe", &path)
					access := "unavailable"
					if d.Code == ERR_DEVICE_PERMISSION {
						access = "denied"
					}
					return probeResult{access: access, findings: []Diagnostic{d}}
				}
				return probeResult{access: "readable"}
			}
			r, err := c.collect("")
			if err != nil || r.OK() != tc.ok || len(r.Nodes) != 1 || r.Nodes[0].Admission != tc.admission || r.Nodes[0].Access != tc.access {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			if !tc.contradiction && (len(r.Groups) != 1 || !hasCode(r, WARN_DEVICE_INCOMPLETE, "warning")) {
				t.Fatal("qualifying metadata lost")
			}
			if tc.contradiction && len(r.Groups) != 0 {
				t.Fatal("contradictory node grouped")
			}
		})
	}
}

func TestDiscoverMixedErrorAndRoot(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	c.uid = 0
	one := fixtureNode(t, c, "unit", 1, 1)
	fixtureNode(t, c, "unit", 2, 2)
	c.probe = func(n Node) probeResult {
		if n.Path == one {
			return probeResult{access: "denied", findings: []Diagnostic{diagnostic(ERR_DEVICE_PERMISSION, "probe", &n.Path)}}
		}
		return probeResult{access: "readable"}
	}
	r, err := c.collect("")
	if err != nil || r.OK() || len(r.Groups) != 1 || len(r.Nodes) != 2 || !hasCode(r, WARN_DEVICE_ROOT, "warning") {
		t.Fatalf("mixed result=%+v err=%v", r, err)
	}
}

func TestDiscoverIndeterminateAndUnrelated(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 1)
	m := c.metadata(path, filepath.Base(path))
	writeFile(t, filepath.Join(*m.node.SysfsPath, "capabilities/ev"), "invalid")
	c.probe = func(n Node) probeResult { t.Fatal("indeterminate node probed"); return probeResult{} }
	r, err := c.collect("")
	if err != nil || r.OK() || len(r.Nodes) != 1 || !hasCode(r, ERR_DEVICE_METADATA, "error") {
		t.Fatalf("indeterminate: %+v %v", r, err)
	}
	// A valid unrelated input identity makes this non-USB input irrelevant.
	writeFile(t, filepath.Join(*m.node.SysfsPath, "id/vendor"), "1234")
	if err := os.Remove(filepath.Join(*m.node.USBParent, "subsystem")); err != nil {
		t.Fatal(err)
	}
	intf := filepath.Join(*m.node.USBParent, "interface-1")
	if err := os.Remove(filepath.Join(intf, "subsystem")); err != nil {
		t.Fatal(err)
	}
	r, err = c.collect("")
	if err != nil || len(r.Nodes) != 0 || hasCode(r, ERR_DEVICE_METADATA, "error") {
		t.Fatalf("unrelated error: %+v %v", r, err)
	}
}

func TestInspectOnlyRequestedNode(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	fixtureNode(t, c, "unit", 1, 1)
	fixtureNode(t, c, "unit", 2, 2)
	c.stat = func(path string, st *syscall.Stat_t) error {
		st.Mode = syscall.S_IFCHR
		st.Rdev = 13<<8 | 65
		return nil
	}
	calls := 0
	c.probe = func(n Node) probeResult {
		calls++
		if n.Path != "literal-alias" {
			t.Fatalf("locator lost: %s", n.Path)
		}
		return probeResult{access: "readable"}
	}
	r, err := c.collect("literal-alias")
	if err != nil || !r.OK() || calls != 1 || len(r.Nodes) != 1 {
		t.Fatalf("inspect=%+v %v calls=%d", r, err, calls)
	}
}

func TestInspectExcludedAndResolution(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 1)
	c.probe = func(n Node) probeResult { t.Fatal("invalid explicit target probed"); return probeResult{} }
	if r, d := c.collect(path); r != nil || d == nil || d.Code != ERR_DEVICE_METADATA {
		t.Fatalf("regular file accepted: %+v %+v", r, d)
	}
	if r, d := c.collect(filepath.Join(c.devRoot, "missing")); r != nil || d == nil || d.Code != ERR_DEVICE_NOT_FOUND {
		t.Fatalf("missing target: %+v %+v", r, d)
	}
	c.stat = func(path string, st *syscall.Stat_t) error {
		st.Mode = syscall.S_IFCHR
		st.Rdev = 13<<8 | 65
		return nil
	}
	m := c.metadata(path, filepath.Base(path))
	writeFile(t, filepath.Join(*m.node.USBParent, "bcdDevice"), "0112")
	r, d := c.collect(path)
	if d != nil || r.OK() || !hasCode(r, ERR_DEVICE_UNSUPPORTED, "error") {
		t.Fatalf("unsupported explicit: %+v %+v", r, d)
	}
}

func TestDiscoverUnknownUSBIdentity(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 1)
	m := c.metadata(path, filepath.Base(path))
	writeFile(t, filepath.Join(*m.node.SysfsPath, "id/vendor"), "1234")
	writeFile(t, filepath.Join(*m.node.USBParent, "bcdDevice"), "invalid")
	c.probe = func(n Node) probeResult { t.Fatal("unknown USB identity probed"); return probeResult{} }
	r, d := c.collect("")
	if d != nil || len(r.Nodes) != 1 || !hasCode(r, ERR_DEVICE_METADATA, "error") {
		t.Fatalf("unknown relevance omitted: %+v %+v", r, d)
	}
}

func TestInspectPermissionResolution(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	c.stat = func(string, *syscall.Stat_t) error { return syscall.EACCES }
	r, d := c.collect("synthetic-event")
	if r != nil || d == nil || d.Code != ERR_DEVICE_PERMISSION {
		t.Fatalf("permission cause lost: %+v %+v", r, d)
	}
}
