package path

import (
	"github.com/jpillora/elvish/pkg/eval"
	osmod "github.com/jpillora/elvish/pkg/mods/os"
)

func NsForProcess(p *eval.Process) *eval.Ns {
	if p == nil {
		return Ns
	}
	ns := eval.BuildNsNamed("path").AddGoFns(map[string]any{"abs": p.Abs})
	osns := osmod.NsForProcess(p)
	for _, name := range []string{"eval-symlinks", "is-dir", "is-regular", "temp-dir", "temp-file"} {
		ns.AddVar(name+eval.FnSuffix, osns.IndexString(name+eval.FnSuffix))
	}
	return eval.CombineNs(Ns, ns.Ns())
}
