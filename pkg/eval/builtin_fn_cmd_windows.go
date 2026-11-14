package eval

import "errors"

var errNotSupportedOnWindows = errors.New("not supported on Windows")

func execFn(fm *Frame, args ...any) error {
	if fm.Evaler.Process != nil {
		return virtualExec(fm, args...)
	}
	return errNotSupportedOnWindows
}

func fg(...int) error {
	return errNotSupportedOnWindows
}
