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
		if p := fm.Evaler.Process; p != nil && p.TerminalSize != nil {
			_, width = p.TerminalSize()
		} else if f := fm.Port(1).File; f != nil {
			_, width = sys.WinSize(f)
		}
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
