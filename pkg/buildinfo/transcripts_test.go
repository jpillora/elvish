package buildinfo_test

import (
	"embed"
	"testing"

	"github.com/jpillora/elvish/pkg/buildinfo"
	"github.com/jpillora/elvish/pkg/eval/evaltest"
	"github.com/jpillora/elvish/pkg/prog/progtest"
)

//go:embed *.elvts
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	evaltest.TestTranscriptsInFS(t, transcripts,
		"elvish-in-global", progtest.ElvishInGlobal(&buildinfo.Program{}),
	)
}
