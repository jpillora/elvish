package eval

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

func isSyntheticCrossDeviceError(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}

func syntheticCommandExit(name string, status int) error {
	return NewExternalCmdExit(name, syscall.WaitStatus{ExitCode: uint32(status)}, 0)
}
