package doc_test

import (
	"embed"
	"io/fs"
	"testing"

	"github.com/jpillora/elvish/pkg/elvdoc"
	"github.com/jpillora/elvish/pkg/eval/evaltest"
	"github.com/jpillora/elvish/pkg/mods/doc"
	"github.com/jpillora/elvish/pkg/must"
	"github.com/jpillora/elvish/pkg/testutil"
)

//go:embed fakepkg
var fakepkg embed.FS

//go:embed *.elvts
var transcripts embed.FS

func TestDocExtractionError(t *testing.T) {
	_, err := (*doc.DocsMapWithError)()
	if err != nil {
		t.Errorf("doc extraction has error: %v", err)
	}
}

func TestTranscripts(t *testing.T) {
	testutil.Set(t, doc.DocsMapWithError, func() (map[string]elvdoc.Docs, error) {
		return must.OK1(elvdoc.ExtractAllFromFS(must.OK1(fs.Sub(fakepkg, "fakepkg")))), nil
	})
	// The result of reading the FS is cached. As a result, this override can't
	// be reverted, so we just do it here instead of properly inside a setup
	// function.
	evaltest.TestTranscriptsInFS(t, transcripts)
}
