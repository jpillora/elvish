package path_test

import (
	"embed"
	"testing"

	"github.com/jpillora/elvish/pkg/eval/evaltest"
	"github.com/jpillora/elvish/pkg/testutil"
)

//go:embed *.elvts
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	evaltest.TestTranscriptsInFS(t, transcripts,
		"in-temp-dir-with-d-f", func(t *testing.T) {
			testutil.InTempDir(t)
			testutil.ApplyDir(testutil.Dir{
				"d": testutil.Dir{
					"f": "",
				},
			})
		},
	)
}
