// Package daemon implements the builtin daemon: module.
package daemon

import (
	"strconv"

	"github.com/jpillora/elvish/pkg/daemon/daemondefs"
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/eval/vars"
)

// Ns makes the daemon: namespace.
func Ns(d daemondefs.Client) *eval.Ns {
	getPid := func() (string, error) {
		pid, err := d.Pid()
		return string(strconv.Itoa(pid)), err
	}

	// TODO: Deprecate the variable in favor of the function.
	getPidVar := func() any {
		pid, err := getPid()
		if err != nil {
			return "-1"
		}
		return pid
	}

	return eval.BuildNsNamed("daemon").
		AddVars(map[string]vars.Var{
			"pid":  vars.FromGet(getPidVar),
			"sock": vars.NewReadOnly(string(d.SockPath())),
		}).
		AddGoFns(map[string]any{
			"pid": getPid,
		}).Ns()
}
