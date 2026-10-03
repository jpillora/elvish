package fsutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"src.elv.sh/pkg/env"
)

// SyntheticCommands are in-process fallbacks for common filesystem commands.
// Keep this list in sync with eval.runSyntheticCommand.
var SyntheticCommands = []string{
	"cat", "cd", "cp", "dir", "head", "ls", "mkdir", "mv", "pwd", "rm", "rmdir", "tail", "touch", "wc",
}

var syntheticPath = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil || strings.ContainsRune(exe, filepath.ListSeparator) {
		return filepath.Join(os.TempDir(), "elvish-synthetic-bin-"+strconv.Itoa(os.Getpid()))
	}
	// exe is a file, so the directory below it cannot exist on disk. It is a
	// recognizable entry in PATH rather than a directory of runnable files.
	return filepath.Join(exe, "synthetic-bin")
})

// SyntheticPath returns the virtual directory used for in-process commands.
func SyntheticPath() string { return syntheticPath() }

// AppendSyntheticPath appends the virtual directory to PATH and returns a
// function that restores the original PATH.
func AppendSyntheticPath() func() {
	old, had := os.LookupEnv(env.PATH)
	for _, dir := range filepath.SplitList(old) {
		if samePath(dir, SyntheticPath()) {
			return func() {}
		}
	}
	value := SyntheticPath()
	if old != "" {
		value = old + string(filepath.ListSeparator) + value
	}
	_ = os.Setenv(env.PATH, value)
	return func() {
		if had {
			_ = os.Setenv(env.PATH, old)
		} else {
			_ = os.Unsetenv(env.PATH)
		}
	}
}

// LookPath resolves real programs first, then the in-process fallback at the
// end of PATH. This preserves the user's installed commands on all platforms.
func LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}
	if IsSyntheticPath(name) && hasSyntheticPath() {
		return name, nil
	}
	if !errors.Is(err, exec.ErrNotFound) || DontSearch(name) || !hasSyntheticPath() || !isSyntheticCommand(name) {
		return "", err
	}
	return filepath.Join(SyntheticPath(), name), nil
}

// IsSyntheticPath reports whether path refers to one of the virtual commands.
func IsSyntheticPath(path string) bool {
	return samePath(filepath.Dir(path), SyntheticPath()) && isSyntheticCommand(filepath.Base(path))
}

func isSyntheticCommand(name string) bool {
	for _, cmd := range SyntheticCommands {
		if name == cmd {
			return true
		}
	}
	return false
}

func hasSyntheticPath() bool {
	for _, dir := range searchPaths() {
		if samePath(dir, SyntheticPath()) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
