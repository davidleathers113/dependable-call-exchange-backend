//go:build !windows
// +build !windows

package rest

import (
	"syscall"
)

func reusePort(network, address string, c syscall.RawConn) error {
	var err error
	c.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
	})
	return err
}