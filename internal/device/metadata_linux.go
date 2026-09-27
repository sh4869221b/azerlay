package device

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var errMetadata = errors.New("invalid device metadata")

type metadata struct {
	node            Node
	findings        []Diagnostic
	identityUnknown bool
}

func (m *metadata) failure(err error) {
	m.findings = append(m.findings, boundaryDiagnostic(err, "metadata", &m.node.Path))
}

func (m *metadata) hex(path string, bits int) uint64 {
	data, err := os.ReadFile(path)
	if err == nil {
		var value uint64
		value, err = strconv.ParseUint(strings.TrimSpace(string(data)), 16, bits)
		if err == nil {
			return value
		}
	}
	m.failure(err)
	return 0
}

func (m *metadata) optional(path string) *string {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		m.failure(err)
		return nil
	}
	value := strings.TrimSuffix(string(data), "\n")
	return &value
}

func (c collector) metadata(path, event string) metadata {
	m := metadata{node: Node{Path: path, Roles: []string{}, Admission: "indeterminate", Access: "not_checked"}}
	class := filepath.Join(c.sysRoot, "class/hidraw", event)
	if valid, err := subsystem(class, "hidraw"); err != nil || !valid {
		if err == nil {
			err = errMetadata
		}
		m.failure(err)
		return m
	}
	input, err := filepath.EvalSymlinks(filepath.Join(class, "device"))
	if err != nil {
		m.failure(err)
		return m
	}
	m.node.SysfsPath = &input
	m.node.deviceNumber, err = readDeviceNumber(filepath.Join(class, "dev"))
	if err != nil {
		m.failure(err)
	}
	data, readErr := os.ReadFile(filepath.Join(input, "report_descriptor"))
	if readErr != nil {
		m.failure(readErr)
	} else {
		if string(data) == physicalDescriptor {
			m.node.ReportDescriptor = &ReportDescriptor{0xff01, 0x0101, 64, false}
		}
	}
	uevent, readErr := os.ReadFile(filepath.Join(input, "uevent"))
	if readErr != nil {
		m.failure(readErr)
	} else {
		for _, line := range strings.Split(string(uevent), "\n") {
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			switch key {
			case "HID_NAME":
				m.node.Name = &value
			case "HID_PHYS":
				m.node.PhysicalPath = &value
			}
		}
	}
	m.ancestry(input)
	return m
}

func subsystem(path, expected string) (bool, error) {
	resolved, err := filepath.EvalSymlinks(filepath.Join(path, "subsystem"))
	return err == nil && filepath.Base(resolved) == expected, err
}

func (m *metadata) ancestry(input string) {
	before := len(m.findings)
	defer func() {
		if len(m.findings) > before && m.node.USBID == nil {
			m.identityUnknown = true
		}
	}()
	for path := filepath.Dir(input); path != filepath.Dir(path); path = filepath.Dir(path) {
		usb, err := subsystem(path, "usb")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			m.failure(err)
			return
		}
		if !usb {
			continue
		}
		data, err := os.ReadFile(filepath.Join(path, "uevent"))
		if err != nil {
			m.failure(err)
			return
		}
		kind := ""
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "DEVTYPE=") {
				kind = strings.TrimPrefix(line, "DEVTYPE=")
			}
		}
		switch kind {
		case "usb_interface":
			if m.node.Interface != nil {
				m.failure(errMetadata)
				return
			}
			before := len(m.findings)
			i := Interface{Number: uint8(m.hex(filepath.Join(path, "bInterfaceNumber"), 8)), Class: uint8(m.hex(filepath.Join(path, "bInterfaceClass"), 8)), Subclass: uint8(m.hex(filepath.Join(path, "bInterfaceSubClass"), 8)), Protocol: uint8(m.hex(filepath.Join(path, "bInterfaceProtocol"), 8))}
			if len(m.findings) == before {
				m.node.Interface = &i
			}
		case "usb_device":
			m.node.USBParent = &path
			before := len(m.findings)
			id := USBID{Vendor: uint16(m.hex(filepath.Join(path, "idVendor"), 16)), Product: uint16(m.hex(filepath.Join(path, "idProduct"), 16)), Release: uint16(m.hex(filepath.Join(path, "bcdDevice"), 16))}
			if len(m.findings) == before {
				m.node.USBID = &id
			}
			m.node.Serial = m.optional(filepath.Join(path, "serial"))
			return // Never borrow an identity from an upstream USB hub.
		default:
			m.failure(errMetadata)
			return
		}
	}
}

func readDeviceNumber(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(strings.TrimSpace(string(data)), ":")
	if len(parts) != 2 {
		return 0, errMetadata
	}
	major, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, errMetadata
	}
	minor, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return 0, errMetadata
	}
	return (major&0xfff)<<8 | (major&0xfffff000)<<32 | minor&0xff | (minor&0xffffff00)<<12, nil
}
