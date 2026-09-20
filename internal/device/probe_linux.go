package device

import (
	"bytes"
	"errors"
	"syscall"
	"unsafe"
)

// Linux input.h: _IOR('E', 0x02, struct input_id) and EVIOCGNAME(len).
const (
	eviocgid   = uintptr(0x80084502)
	eviocgname = uintptr(0x81004506)
)

type probeOps struct {
	open  func(string, int, uint32) (int, error)
	fstat func(int, *syscall.Stat_t) error
	id    func(int) (InputID, error)
	name  func(int) (string, error)
	close func(int) error
}

func probeNode(n Node) probeResult {
	return (probeOps{syscall.Open, syscall.Fstat, ioctlID, ioctlName, syscall.Close}).probe(n)
}

func (ops probeOps) probe(n Node) (result probeResult) {
	result.access = "unavailable"
	fail := func(err error) {
		d := boundaryDiagnostic(err, "probe", &n.Path)
		if d.Code == ERR_DEVICE_PERMISSION {
			result.access = "denied"
		}
		result.findings = append(result.findings, d)
	}
	fd, err := ops.open(n.Path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		fail(err)
		return
	}
	defer func() {
		if err := ops.close(fd); err != nil {
			fail(err)
			if result.access == "readable" {
				result.access = "unavailable"
			}
		}
	}()
	var st syscall.Stat_t
	if err := ops.fstat(fd, &st); err != nil {
		fail(err)
		return
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFCHR || st.Rdev != n.deviceNumber {
		fail(errMetadata)
		return
	}
	id, err := ops.id(fd)
	if err != nil {
		fail(err)
		return
	}
	if id != (InputID{3, 0x16d0, 0x12f7, 0x111}) || n.InputID != nil && id != *n.InputID {
		result.contradiction = true
		fail(errMetadata)
		return
	}
	name, err := ops.name(fd)
	if errors.Is(err, syscall.ENOTTY) || errors.Is(err, syscall.EINVAL) {
		d := diagnostic(ERR_DEVICE_METADATA, "probe", &n.Path)
		d.Severity = "warning"
		d.Summary = "Device name ioctl is unavailable; retained sysfs name."
		result.findings = append(result.findings, d)
	} else if err != nil {
		fail(err)
		return
	} else {
		result.name = &name
	}
	result.access = "readable"
	return
}

func ioctlID(fd int) (InputID, error) {
	// Four uint16 fields match struct input_id; the buffer lives through syscall.
	var id InputID
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), eviocgid, uintptr(unsafe.Pointer(&id)))
	if errno != 0 {
		return InputID{}, errno
	}
	return id, nil
}

func ioctlName(fd int) (string, error) {
	var name [256]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), eviocgname, uintptr(unsafe.Pointer(&name[0])))
	if errno != 0 {
		return "", errno
	}
	end := bytes.IndexByte(name[:], 0)
	if end < 0 {
		end = len(name)
	}
	return string(name[:end]), nil
}
