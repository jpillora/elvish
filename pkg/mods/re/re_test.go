package re_test

import (
	"embed"
	"testing"

	"github.com/jpillora/elvish/pkg/eval/evaltest"
)

//go:embed *.elvts *.elv
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	evaltest.TestTranscriptsInFS(t, transcripts)
}
