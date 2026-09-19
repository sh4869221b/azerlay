package control

import (
	"net"
	"os"
	"syscall"
)

func sameUID(uid uint32) bool { return uint64(uid) == uint64(os.Geteuid()) }

func checkPeer(conn *net.UnixConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return NewError(ERR_CONTROL_PERMISSION)
	}
	var credentials *syscall.Ucred
	var credentialErr error
	err = raw.Control(func(fd uintptr) {
		credentials, credentialErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil || credentialErr != nil || credentials == nil || !sameUID(credentials.Uid) {
		return NewError(ERR_CONTROL_PERMISSION)
	}
	return nil
}
