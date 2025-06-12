//go:build windows
// +build windows

package rest

import (
	"syscall"
)

func reusePort(network, address string, c syscall.RawConn) error {
	// SO_REUSEPORT is not available on Windows
	return nil
}