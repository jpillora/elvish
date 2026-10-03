package eval

import (
	"errors"

	"golang.org/x/sys/windows"
)

func isSyntheticCrossDeviceError(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}
