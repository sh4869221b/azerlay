package input

import (
	"syscall"
	"unsafe"

	"github.com/sh4869221b/azerlay/internal/device"
)

type recoveryState struct {
	state nodeState
	axes  map[uint16]AxisInfo
}

type recoveryIoctls struct {
	keys func(uintptr, uintptr, *[96]byte) (int, error)
	axes func(uintptr, uintptr, *[6]int32) error
}

func readRecoveryState(node device.OpenedNode) (recoveryState, error) {
	return queryRecoveryState(node, recoveryIoctls{keys: keyStateIoctl, axes: axisIoctl})
}

func queryRecoveryState(node device.OpenedNode, ioctl recoveryIoctls) (recoveryState, error) {
	connection, err := node.File.SyscallConn()
	if err != nil {
		return recoveryState{}, inputReadError(err)
	}
	result := recoveryState{
		state: nodeState{keys: make(map[uint16]bool), absolute: make(map[uint16]int32)},
		axes:  make(map[uint16]AxisInfo),
	}
	var queryErr error
	err = connection.Control(func(fd uintptr) {
		if codes := node.Node.Capabilities["key"]; len(codes) > 0 {
			// EVIOCGKEY: _IOC(_IOC_READ, 'E', 0x18, KEY_CNT / 8).
			var bitmap [96]byte
			n, err := ioctl.keys(fd, 0x80604518, &bitmap)
			if err != nil {
				queryErr = err
				return
			}
			if n != len(bitmap) {
				queryErr = &InputError{Code: ERR_INPUT_READ}
				return
			}
			for _, code := range codes {
				if code >= 0 && code <= 0x2ff {
					result.state.keys[uint16(code)] = bitmap[code/8]&(1<<uint(code%8)) != 0
				}
			}
		}
		for _, code := range node.Node.Capabilities["abs"] {
			if code < 0 || code > 0x3f {
				continue
			}
			raw, info, err := getAxisState(fd, uint16(code), ioctl.axes)
			if err != nil {
				queryErr = err
				return
			}
			result.state.absolute[uint16(code)] = raw
			result.axes[uint16(code)] = info
		}
	})
	if err != nil {
		return recoveryState{}, inputReadError(err)
	}
	if queryErr != nil {
		return recoveryState{}, inputReadError(queryErr)
	}
	return result, nil
}

func keyStateIoctl(fd, request uintptr, bitmap *[96]byte) (int, error) {
	n, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(bitmap)))
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
