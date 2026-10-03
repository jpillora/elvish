//go:build !windows

package eval

import (
	"errors"
	"syscall"
)

func isSyntheticCrossDeviceError(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
