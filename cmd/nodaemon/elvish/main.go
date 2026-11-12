// Command elvish is an alternative main program of Elvish that does not include
// the daemon subprogram.
package main

import (
	"os"

	"github.com/jpillora/elvish/pkg/buildinfo"
	"github.com/jpillora/elvish/pkg/lsp"
	"github.com/jpillora/elvish/pkg/prog"
	"github.com/jpillora/elvish/pkg/shell"
)

func main() {
	os.Exit(prog.Run(
		[3]*os.File{os.Stdin, os.Stdout, os.Stderr}, os.Args,
		prog.Composite(&buildinfo.Program{}, &lsp.Program{}, &shell.Program{})))
}
