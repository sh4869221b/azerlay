package device

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func writeFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}
func link(t *testing.T, target, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
func fixtureCollector(t *testing.T) collector {
	t.Helper()
	root := t.TempDir()
	c := collector{devRoot: filepath.Join(root, "dev"), sysRoot: filepath.Join(root, "sys"), uid: 1000}
	for _, name := range []string{c.devRoot, filepath.Join(c.sysRoot, "bus/usb"), filepath.Join(c.sysRoot, "class/hidraw")} {
		if err := os.MkdirAll(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	c.stat = func(path string, st *syscall.Stat_t) error {
		n, ok := hidrawNumber(filepath.Base(path))
		if !ok {
			return syscall.ENOENT
		}
		st.Mode = syscall.S_IFCHR
		st.Rdev = 240<<8 | n
		return nil
	}
	c.probe = func(Node) probeResult { return probeResult{access: "readable"} }
	return c
}
func fixtureNode(t *testing.T, c collector, unit string, n, iface int) string {
	t.Helper()
	parent := filepath.Join(c.sysRoot, "devices", unit)
	for key, value := range map[string]string{"uevent": "DEVTYPE=usb_device\n", "idVendor": "16d0", "idProduct": "12f7", "bcdDevice": "0111", "serial": "synthetic"} {
		writeFile(t, filepath.Join(parent, key), value)
	}
	if _, err := os.Lstat(filepath.Join(parent, "subsystem")); os.IsNotExist(err) {
		link(t, filepath.Join(c.sysRoot, "bus/usb"), filepath.Join(parent, "subsystem"))
	}
	intf := filepath.Join(parent, fmt.Sprintf("interface-%d-%d", iface, n))
	link(t, filepath.Join(c.sysRoot, "bus/usb"), filepath.Join(intf, "subsystem"))
	for key, value := range map[string]string{"uevent": "DEVTYPE=usb_interface\n", "bInterfaceNumber": fmt.Sprintf("%02x", iface), "bInterfaceClass": "03", "bInterfaceSubClass": "00", "bInterfaceProtocol": "00"} {
		writeFile(t, filepath.Join(intf, key), value)
	}
	hid := filepath.Join(intf, "hid")
	writeFile(t, filepath.Join(hid, "report_descriptor"), physicalDescriptor)
	writeFile(t, filepath.Join(hid, "uevent"), "HID_NAME=Synthetic device\nHID_PHYS=synthetic/usb\n")
	name := fmt.Sprintf("hidraw%d", n)
	node := filepath.Join(hid, "hidraw", name)
	writeFile(t, filepath.Join(node, "dev"), fmt.Sprintf("240:%d", n))
	link(t, filepath.Join(c.sysRoot, "class/hidraw"), filepath.Join(node, "subsystem"))
	link(t, hid, filepath.Join(node, "device"))
	link(t, node, filepath.Join(c.sysRoot, "class/hidraw", name))
	link(t, node, filepath.Join(c.sysRoot, "dev/char", fmt.Sprintf("240:%d", n)))
	path := filepath.Join(c.devRoot, name)
	writeFile(t, path, "")
	return path
}
func TestRawDiscoveryAdmission(t *testing.T) {
	for _, tc := range []struct{ name, field, value string }{{"supported", "", ""}, {"release", "bcdDevice", "0112"}, {"vendor", "idVendor", "1234"}, {"descriptor", "report_descriptor", "unsupported"}, {"interface", "bInterfaceNumber", "03"}} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixtureCollector(t)
			path := fixtureNode(t, c, "unit", 2, 4)
			m := c.metadata(path, "hidraw2")
			if tc.field != "" {
				root := *m.node.USBParent
				if tc.field == "report_descriptor" {
					root = *m.node.SysfsPath
				}
				if tc.field == "bInterfaceNumber" {
					root = filepath.Dir(*m.node.SysfsPath)
				}
				writeFile(t, filepath.Join(root, tc.field), tc.value)
			}
			calls := 0
			c.probe = func(n Node) probeResult {
				calls++
				if n.Path != path {
					t.Fatal("unexpected open")
				}
				return probeResult{access: "readable"}
			}
			r, d := c.collect("")
			if d != nil {
				t.Fatal(d)
			}
			if tc.name == "supported" {
				if !r.OK() || calls != 1 || len(r.Groups) != 1 || !r.Groups[0].Complete {
					t.Fatalf("%+v calls=%d", r, calls)
				}
			} else if r.OK() || calls != 0 {
				t.Fatalf("unsupported opened: %+v %d", r, calls)
			}
		})
	}
}
func TestRawInspectAliasAndEventRejection(t *testing.T) {
	c := fixtureCollector(t)
	fixtureNode(t, c, "unit", 2, 4)
	c.stat = func(_ string, st *syscall.Stat_t) error { st.Mode = syscall.S_IFCHR; st.Rdev = 240<<8 | 2; return nil }
	r, d := c.collect("relative-alias")
	if d != nil || !r.OK() || r.Nodes[0].Path != "relative-alias" {
		t.Fatalf("alias=%+v %v", r, d)
	}
	event := filepath.Join(c.sysRoot, "devices/event0")
	writeFile(t, filepath.Join(event, "dev"), "13:64")
	link(t, event, filepath.Join(c.sysRoot, "dev/char/13:64"))
	c.stat = func(_ string, st *syscall.Stat_t) error { st.Mode = syscall.S_IFCHR; st.Rdev = 13<<8 | 64; return nil }
	c.probe = func(Node) probeResult { t.Fatal("event node opened"); return probeResult{} }
	r, d = c.collect("/dev/input/event0")
	if r != nil || d == nil || d.Code != ERR_DEVICE_UNSUPPORTED {
		t.Fatalf("%+v %v", r, d)
	}
}
