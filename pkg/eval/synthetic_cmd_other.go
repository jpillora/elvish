//go:build !windows

package eval

import (
	"errors"
	"syscall"
)

func isSyntheticCrossDeviceError(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}

func syntheticCommandExit(name string, status int) error {
	return NewExternalCmdExit(name, syscall.WaitStatus(status<<8), 0)
}
