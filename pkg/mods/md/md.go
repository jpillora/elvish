// Package md exposes functionality from github.com/jpillora/elvish/pkg/md.
package md

import (
	"github.com/jpillora/elvish/pkg/elvdoc"
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/md"
	"github.com/jpillora/elvish/pkg/sys"
)

// Ns is the namespace for the md: module.
var Ns = eval.BuildNsNamed("md").
	AddGoFns(map[string]any{
		"show": show,
	}).Ns()

type showOpts struct {
	Width int
}

func (*showOpts) SetDefaultOptions() {}

func show(fm *eval.Frame, opts showOpts, markdown string) error {
	width := opts.Width
	if width <= 0 {
		_, width = sys.WinSize(fm.Port(1).File)
		if width <= 0 {
			width = 80
		}
	}
	codec := &md.TTYCodec{
		Width:              width,
		HighlightCodeBlock: elvdoc.HighlightCodeBlock,
	}
	_, err := fm.ByteOutput().WriteString(md.RenderString(markdown, codec))
	return err
}
