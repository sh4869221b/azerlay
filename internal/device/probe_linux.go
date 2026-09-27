package device

import (
	"bytes"
	"errors"
	"syscall"
	"unsafe"
)

// Linux hidraw.h: kernel-held identity, name and report descriptor getters.
const (
	hidiocgrawinfo = uintptr(0x80084803)
	hidiocgrawname = uintptr(0x81004804)
)

type probeOps struct {
	open       func(string, int, uint32) (int, error)
	fstat      func(int, *syscall.Stat_t) error
	id         func(int) (RawID, error)
	name       func(int) (string, error)
	descriptor func(int) ([]byte, error)
	close      func(int) error
}

func probeNode(n Node) probeResult {
	return (probeOps{syscall.Open, syscall.Fstat, ioctlID, ioctlName, ioctlDescriptor, syscall.Close}).probe(n)
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
	result.contradiction, err = ops.validate(fd, n)
	if err != nil {
		fail(err)
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

func (ops probeOps) validate(fd int, n Node) (contradiction bool, err error) {
	var st syscall.Stat_t
	if err := ops.fstat(fd, &st); err != nil {
		return false, err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFCHR || st.Rdev != n.deviceNumber {
		return false, errMetadata
	}
	id, err := ops.id(fd)
	if err != nil {
		return false, err
	}
	if id != (RawID{3, 0x16d0, 0x12f7}) {
		return true, errMetadata
	}
	descriptor, err := ops.descriptor(fd)
	if err != nil {
		return false, err
	}
	if string(descriptor) != physicalDescriptor {
		return true, errMetadata
	}
	return false, nil
}

func ioctlID(fd int) (RawID, error) {
	// The uint32 bus and two uint16 IDs match struct hidraw_devinfo.
	var id RawID
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), hidiocgrawinfo, uintptr(unsafe.Pointer(&id)))
	if errno != 0 {
		return RawID{}, errno
	}
	return id, nil
}

func ioctlName(fd int) (string, error) {
	var name [256]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), hidiocgrawname, uintptr(unsafe.Pointer(&name[0])))
	if errno != 0 {
		return "", errno
	}
	end := bytes.IndexByte(name[:], 0)
	if end < 0 {
		end = len(name)
	}
	return string(name[:end]), nil
}

func ioctlDescriptor(fd int) ([]byte, error) {
	var size uint32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x80044801, uintptr(unsafe.Pointer(&size)))
	if errno != 0 {
		return nil, errno
	}
	if size != uint32(len(physicalDescriptor)) {
		return nil, errUnsupported
	}
	var descriptor struct {
		Size  uint32
		Value [4096]byte
	}
	descriptor.Size = size
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x90044802, uintptr(unsafe.Pointer(&descriptor)))
	if errno != 0 {
		return nil, errno
	}
	if descriptor.Size != size {
		return nil, errMetadata
	}
	return descriptor.Value[:size], nil
}
