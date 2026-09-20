package input

import (
	"os"
	"syscall"
	"unsafe"
)

func readAxisInfo(file *os.File, codes []int) (map[uint16]AxisInfo, error) {
	connection, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	info := make(map[uint16]AxisInfo, len(codes))
	var ioctlErr error
	err = connection.Control(func(fd uintptr) {
		for _, code := range codes {
			axis, err := getAxisInfo(fd, uint16(code), axisIoctl)
			if err != nil {
				ioctlErr = err
				return
			}
			info[uint16(code)] = axis
		}
	})
	if err != nil {
		return nil, err
	}
	if ioctlErr != nil {
		return nil, ioctlErr
	}
	return info, nil
}

func getAxisInfo(fd uintptr, code uint16, ioctl func(uintptr, uintptr, *[6]int32) error) (AxisInfo, error) {
	_, info, err := getAxisState(fd, code, ioctl)
	return info, err
}

func getAxisState(fd uintptr, code uint16, ioctl func(uintptr, uintptr, *[6]int32) error) (int32, AxisInfo, error) {
	// EVIOCGABS is _IOR('E', 0x40 + code, struct input_absinfo).
	// The six fields are value, minimum, maximum, fuzz, flat, resolution.
	var raw [6]int32
	if err := ioctl(fd, 0x80184500|uintptr(0x40+code), &raw); err != nil {
		return 0, AxisInfo{}, err
	}
	return raw[0], AxisInfo{Minimum: raw[1], Maximum: raw[2], Flat: raw[4], Fuzz: raw[3]}, nil
}

func axisIoctl(fd, request uintptr, info *[6]int32) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(info)))
	if errno != 0 {
		return errno
	}
	return nil
}
