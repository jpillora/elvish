package store

import (
	"embed"
	"testing"

	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/eval/evaltest"
	"github.com/jpillora/elvish/pkg/must"
	"github.com/jpillora/elvish/pkg/store"
	"github.com/jpillora/elvish/pkg/testutil"
)

//go:embed *.elvts
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	evaltest.TestTranscriptsInFS(t, transcripts,
		"use-store-brand-new", func(t *testing.T, ev *eval.Evaler) {
			testutil.InTempDir(t)
			s := must.OK1(store.NewStore("db"))
			ev.ExtendGlobal(eval.BuildNs().AddNs("store", Ns(s)))
		},
	)
}
