package eval

import (
	"errors"
	"os"

	"github.com/jpillora/elvish/pkg/eval/errs"
	"github.com/jpillora/elvish/pkg/fsutil"
)

// Command and process control.

// TODO(xiaq): Document "fg".

func init() {
	addBuiltinFns(map[string]any{
		// Command resolution
		"external":        external,
		"has-external":    func(fm *Frame, cmd string) bool { _, err := fm.Evaler.Process.LookPath(cmd); return err == nil },
		"search-external": func(fm *Frame, cmd string) (string, error) { return fm.Evaler.Process.LookPath(cmd) },

		// Process control
		"fg": func(fm *Frame, pids ...int) error {
			if fm.Evaler.Process != nil {
				return errors.New("fg requires a standalone shell process")
			}
			return fg(pids...)
		},
		"exec": execFn,
		"exit": exit,
	})
}

func external(cmd string) Callable {
	return NewExternalCmd(cmd)
}

func hasExternal(cmd string) bool {
	_, err := fsutil.LookPath(cmd)
	return err == nil
}

func searchExternal(cmd string) (string, error) {
	return fsutil.LookPath(cmd)
}

// Can be overridden in tests.
var osExit = os.Exit

func exit(fm *Frame, codes ...int) error {
	code := 0
	switch len(codes) {
	case 0:
	case 1:
		code = codes[0]
	default:
		return errs.ArityMismatch{What: "arguments", ValidLow: 0, ValidHigh: 1, Actual: len(codes)}
	}
	if fm.Evaler.Process != nil {
		return fm.Evaler.Process.Exit(code)
	}

	fm.Evaler.PreExit()
	osExit(code)
	return nil
}
