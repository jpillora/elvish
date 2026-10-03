package eval

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSyntheticCrossDeviceError(t *testing.T) {
	if !isSyntheticCrossDeviceError(fmt.Errorf("rename: %w", windows.ERROR_NOT_SAME_DEVICE)) {
		t.Fatal("Windows cross-volume rename error was not recognized")
	}
	if isSyntheticCrossDeviceError(windows.ERROR_ACCESS_DENIED) {
		t.Fatal("access denied must not trigger a copy-and-remove move")
	}
}
