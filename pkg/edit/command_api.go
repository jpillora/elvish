package edit

// Implementation of the editor "command" mode.

import (
	"github.com/jpillora/elvish/pkg/cli/modes"
	"github.com/jpillora/elvish/pkg/eval"
)

func initCommandAPI(ed *Editor, ev *eval.Evaler, nb eval.NsBuilder) {
	bindingVar := newBindingVar(emptyBindingsMap)
	bindings := newMapBindings(ed, ev, bindingVar)
	nb.AddNs("command",
		eval.BuildNsNamed("edit:command").
			AddVar("binding", bindingVar).
			AddGoFns(map[string]any{
				"start": func() {
					w := modes.NewStub(modes.StubSpec{
						Bindings: bindings,
						Name:     " COMMAND ",
					})
					ed.app.PushAddon(w)
				},
			}))
}
