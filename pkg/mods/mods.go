// Package mods collects standard library modules.
package mods

import (
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/mods/doc"
	"github.com/jpillora/elvish/pkg/mods/epm"
	"github.com/jpillora/elvish/pkg/mods/file"
	"github.com/jpillora/elvish/pkg/mods/flag"
	"github.com/jpillora/elvish/pkg/mods/math"
	"github.com/jpillora/elvish/pkg/mods/md"
	"github.com/jpillora/elvish/pkg/mods/os"
	"github.com/jpillora/elvish/pkg/mods/path"
	"github.com/jpillora/elvish/pkg/mods/platform"
	"github.com/jpillora/elvish/pkg/mods/re"
	readline_binding "github.com/jpillora/elvish/pkg/mods/readline-binding"
	"github.com/jpillora/elvish/pkg/mods/runtime"
	"github.com/jpillora/elvish/pkg/mods/str"
	"github.com/jpillora/elvish/pkg/mods/unix"
)

// AddTo adds all standard library modules to the Evaler.
//
// Some modules (the runtime module for now) may rely on properties set on the
// Evaler, so any mutations afterwards may not be properly reflected.
func AddTo(ev *eval.Evaler) {
	ev.AddModule("runtime", runtime.Ns(ev))
	ev.AddModule("math", math.Ns)
	ev.AddModule("path", path.NsForProcess(ev.Process))
	ev.AddModule("platform", platform.Ns)
	ev.AddModule("re", re.Ns)
	ev.AddModule("str", str.Ns)
	ev.AddModule("file", file.NsForProcess(ev.Process))
	ev.AddModule("flag", flag.Ns)
	ev.AddModule("doc", doc.Ns)
	ev.AddModule("os", os.NsForProcess(ev.Process))
	ev.AddModule("md", md.Ns)
	// OS-wide umask, resource limits and foreground job control do not belong
	// to a virtual root process. Never expose the host-mutating namespace.
	if unix.ExposeUnixNs && ev.Process == nil {
		ev.AddModule("unix", unix.Ns)
	}
	ev.BundledModules["epm"] = epm.Code
	ev.BundledModules["readline-binding"] = readline_binding.Code
}
