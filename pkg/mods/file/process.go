package file

import (
	"github.com/jpillora/elvish/pkg/eval"
	"os"
)

// NsForProcess binds filesystem operations to a virtual process directory.
func NsForProcess(p *eval.Process) *eval.Ns {
	if p == nil {
		return Ns
	}
	return eval.CombineNs(Ns, eval.BuildNsNamed("file").AddGoFns(map[string]any{
		"open":        func(name string) (*os.File, error) { return open(p.Path(name)) },
		"open-output": func(opts openOutputOpts, name string) (*os.File, error) { return openOutput(opts, p.Path(name)) },
	}).Ns())
}
