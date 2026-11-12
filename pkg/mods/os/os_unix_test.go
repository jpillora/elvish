//go:build unix

package os_test

import (
	"testing"

	"github.com/jpillora/elvish/pkg/must"
	"golang.org/x/sys/unix"
)

func mkFifoOrSkip(name string) { must.OK(unix.Mkfifo(name, 0o600)) }

func createWindowsSpecialFileOrSkip(t *testing.T) { t.Skip("not on Windows") }
