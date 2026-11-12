package readline_binding_test

import (
	"embed"
	"os"
	"testing"

	"github.com/jpillora/elvish/pkg/cli"
	"github.com/jpillora/elvish/pkg/edit"
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/eval/evaltest"
	"github.com/jpillora/elvish/pkg/mods"
)

//go:embed *.elvts
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	evaltest.TestTranscriptsInFS(t, transcripts,
		"prepare-deps",
		func(ev *eval.Evaler) {
			mods.AddTo(ev)
			ed := edit.NewEditor(cli.NewTTY(os.Stdin, os.Stderr), ev, nil)
			ev.ExtendBuiltin(eval.BuildNs().AddNs("edit", ed))
		})
}
