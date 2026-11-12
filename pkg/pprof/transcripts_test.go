package pprof_test

import (
	"embed"
	"os"
	"testing"

	"github.com/jpillora/elvish/pkg/eval/evaltest"
	"github.com/jpillora/elvish/pkg/pprof"
	"github.com/jpillora/elvish/pkg/prog"
	"github.com/jpillora/elvish/pkg/prog/progtest"
)

//go:embed *.elvts
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	evaltest.TestTranscriptsInFS(t, transcripts,
		"elvish-in-global", progtest.ElvishInGlobal(
			prog.Composite(&pprof.Program{}, noopProgram{})),
	)
}

type noopProgram struct{}

func (noopProgram) RegisterFlags(*prog.FlagSet)     {}
func (noopProgram) Run([3]*os.File, []string) error { return nil }
