package device

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	c := collector{devRoot: filepath.Join(root, "dev/input"), sysRoot: filepath.Join(root, "sys"), stat: syscall.Stat, uid: 1000}
	for _, dir := range []string{c.devRoot, filepath.Join(c.sysRoot, "bus/input"), filepath.Join(c.sysRoot, "bus/usb")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func fixtureNode(t *testing.T, c collector, device string, number, iface int) string {
	t.Helper()
	parent := filepath.Join(c.sysRoot, "devices/hub", device)
	writeFile(t, filepath.Join(parent, "uevent"), "DEVTYPE=usb_device\n")
	writeFile(t, filepath.Join(parent, "idVendor"), "16d0\n")
	writeFile(t, filepath.Join(parent, "idProduct"), "12f7\n")
	writeFile(t, filepath.Join(parent, "bcdDevice"), "0111\n")
	writeFile(t, filepath.Join(parent, "serial"), "synthetic-serial\n")
	if _, err := os.Lstat(filepath.Join(parent, "subsystem")); os.IsNotExist(err) {
		link(t, filepath.Join(c.sysRoot, "bus/usb"), filepath.Join(parent, "subsystem"))
	}
	intf := filepath.Join(parent, fmt.Sprintf("interface-%d", iface))
	writeFile(t, filepath.Join(intf, "uevent"), "DEVTYPE=usb_interface\n")
	link(t, filepath.Join(c.sysRoot, "bus/usb"), filepath.Join(intf, "subsystem"))
	writeFile(t, filepath.Join(intf, "bInterfaceNumber"), fmt.Sprintf("%02x", iface))
	writeFile(t, filepath.Join(intf, "bInterfaceClass"), "03")
	subclass, protocol := "00", "00"
	if iface == 2 {
		subclass, protocol = "01", "02"
	}
	writeFile(t, filepath.Join(intf, "bInterfaceSubClass"), subclass)
	writeFile(t, filepath.Join(intf, "bInterfaceProtocol"), protocol)
	input := filepath.Join(intf, "hid/input", fmt.Sprintf("input%d", number))
	writeFile(t, filepath.Join(input, "name"), "Synthetic Cyborg II")
	writeFile(t, filepath.Join(input, "phys"), "synthetic/input0")
	for field, value := range map[string]string{"bustype": "0003", "vendor": "16d0", "product": "12f7", "version": "0111"} {
		writeFile(t, filepath.Join(input, "id", field), value)
	}
	caps := map[string]string{"ev": "f", "key": "40000000", "rel": "40", "abs": "3", "msc": "0", "sw": "0", "led": "0", "ff": "0", "snd": "0"}
	if iface == 2 {
		caps["ev"], caps["key"], caps["rel"], caps["abs"] = "7", "10000 0 0 0 0", "3", "0"
	}
	if iface == 3 {
		caps["ev"], caps["key"], caps["rel"] = "b", "100000000 0 0 0 0", "0"
	}
	for family, value := range caps {
		writeFile(t, filepath.Join(input, "capabilities", family), value)
	}
	event := fmt.Sprintf("event%d", number)
	eventDir := filepath.Join(input, event)
	writeFile(t, filepath.Join(eventDir, "dev"), fmt.Sprintf("13:%d", 64+number))
	link(t, filepath.Join(c.sysRoot, "bus/input"), filepath.Join(eventDir, "subsystem"))
	link(t, input, filepath.Join(eventDir, "device"))
	link(t, eventDir, filepath.Join(c.sysRoot, "class/input", event))
	link(t, eventDir, filepath.Join(c.sysRoot, "dev/char", fmt.Sprintf("13:%d", 64+number)))
	path := filepath.Join(c.devRoot, event)
	writeFile(t, path, "")
	return path
}

func TestMetadataTopology(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	for i := 1; i <= 3; i++ {
		path := fixtureNode(t, c, "unit", i, i)
		m := c.metadata(path, filepath.Base(path))
		if len(m.findings) != 0 {
			t.Fatalf("findings: %+v", m.findings)
		}
		if m.node.InputID == nil || *m.node.InputID != (InputID{3, 0x16d0, 0x12f7, 0x111}) || m.node.USBID == nil || *m.node.USBID != (USBID{0x16d0, 0x12f7, 0x111}) {
			t.Fatalf("IDs: %+v", m.node)
		}
		if *m.node.USBParent != filepath.Join(c.sysRoot, "devices/hub/unit") || m.node.Interface.Number != uint8(i) || *m.node.PhysicalPath != "synthetic/input0" {
			t.Fatalf("ancestry: %+v", m.node)
		}
	}
	codes, err := parseBitmap("8000000000000001 8000000000000001")
	if err != nil || !reflect.DeepEqual(codes, []int{0, 63, 64, 127}) {
		t.Fatalf("bitmap: %v %v", codes, err)
	}
}

func TestMetadataFailures(t *testing.T) {
	for _, tc := range []struct {
		name, field, value string
		remove             bool
	}{
		{"malformed ID", "id/vendor", "oops", false}, {"malformed bitmap", "capabilities/key", "zz", false}, {"missing role", "capabilities/ev", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixtureCollector(t)
			path := fixtureNode(t, c, "unit", 1, 1)
			m := c.metadata(path, filepath.Base(path))
			field := filepath.Join(*m.node.SysfsPath, tc.field)
			if tc.remove {
				if err := os.Remove(field); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, field, tc.value)
			}
			m = c.metadata(path, filepath.Base(path))
			if len(m.findings) == 0 || m.node.Admission != "indeterminate" {
				t.Fatalf("accepted malformed metadata: %+v", m)
			}
			if tc.field == "id/vendor" && m.node.InputID != nil {
				t.Fatal("fabricated input ID")
			}
		})
	}
}

func TestMetadataNoAncestorBorrowing(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 1)
	parent := filepath.Join(c.sysRoot, "devices/hub/unit")
	writeFile(t, filepath.Join(parent, "idVendor"), "1234")
	hub := filepath.Dir(parent)
	link(t, filepath.Join(c.sysRoot, "bus/usb"), filepath.Join(hub, "subsystem"))
	writeFile(t, filepath.Join(hub, "uevent"), "DEVTYPE=usb_device")
	writeFile(t, filepath.Join(hub, "idVendor"), "16d0")
	m := c.metadata(path, filepath.Base(path))
	if *m.node.USBParent != parent || m.node.USBID.Vendor != 0x1234 {
		t.Fatalf("borrowed hub: %+v", m.node)
	}
	if err := os.Remove(filepath.Join(parent, "serial")); err != nil {
		t.Fatal(err)
	}
	m = c.metadata(path, filepath.Base(path))
	if m.node.Serial != nil || len(m.findings) > 0 {
		t.Fatal("missing serial rejected")
	}
}

func TestMetadataExplicitResolver(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 1)
	if _, err := c.resolve(path); err == nil {
		t.Fatal("regular file resolved")
	}
	c.stat = func(_ string, st *syscall.Stat_t) error { st.Mode = syscall.S_IFCHR; st.Rdev = 13<<8 | 65; return nil }
	if event, err := c.resolve("synthetic-alias"); err != nil || event != "event1" {
		t.Fatalf("alias: %q %v", event, err)
	}
	c.stat = func(_ string, st *syscall.Stat_t) error { st.Mode = syscall.S_IFCHR; st.Rdev = 1<<8 | 3; return nil }
	if _, err := c.resolve("other-character-device"); err == nil {
		t.Fatal("other device resolved")
	}
}

func TestMetadataUnresolvedInput(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 1)
	deviceLink := filepath.Join(c.sysRoot, "class/input/event1/device")
	if err := os.Remove(deviceLink); err != nil {
		t.Fatal(err)
	}
	m := c.metadata(path, "event1")
	if len(m.findings) != 1 || m.node.SysfsPath != nil || m.node.Admission != "indeterminate" {
		t.Fatalf("unresolved input: %+v", m)
	}
}
