package main

import (
	"embed"
	"strings"
	"testing"

	"github.com/jpillora/elvish/pkg/elvdoc"
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/eval/evaltest"
)

//go:embed *.elvts
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	evaltest.TestTranscriptsInFS(t, transcripts,
		"elvdoc-to-md-in-global", func(ev *eval.Evaler) {
			ev.ExtendGlobal(eval.BuildNs().AddGoFn("elvdoc-to-md", elvdocToMd))
		},
	)
}

func elvdocToMd(fm *eval.Frame, src string) error {
	docs, err := elvdoc.Extract(strings.NewReader(src), "")
	if err != nil {
		return err
	}
	var sb strings.Builder
	writeElvdocSections(&sb, docs)
	_, err = fm.ByteOutput().WriteString(sb.String())
	return err
}
