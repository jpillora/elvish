package edit

import (
	"testing"

	"github.com/jpillora/elvish/pkg/cli/term"
	"github.com/jpillora/elvish/pkg/ui"
)

func TestCommandMode(t *testing.T) {
	f := setup(t)

	evals(f.Evaler, `set edit:insert:binding[Ctrl-'['] = $edit:command:start~`)
	feedInput(f.TTYCtrl, "echo")
	f.TTYCtrl.Inject(term.K('[', ui.Ctrl))
	f.TestTTY(t,
		"~> echo", Styles,
		"   vvvv", term.DotHere, "\n",
		" COMMAND ", Styles,
		"*********",
	)

	f.TTYCtrl.Inject(term.K('b'))
	f.TestTTY(t,
		"~> ", term.DotHere,
		"echo\n", Styles,
		"vvvv",
		" COMMAND ", Styles,
		"*********",
	)
}
