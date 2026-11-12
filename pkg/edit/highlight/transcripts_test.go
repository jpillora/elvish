package highlight_test

import (
	"embed"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jpillora/elvish/pkg/edit/highlight"
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/eval/evaltest"
	"github.com/jpillora/elvish/pkg/must"
	"github.com/jpillora/elvish/pkg/testutil"
	"github.com/jpillora/elvish/pkg/ui"
	"github.com/jpillora/elvish/pkg/ui/styledown"
)

//go:embed *.elvts
var transcripts embed.FS

func TestTranscripts(t *testing.T) {
	var validCommands []string
	evaltest.TestTranscriptsInFS(t, transcripts,
		"with-max-block-for-late", func(t *testing.T, s string) {
			testutil.Set(t, highlight.MaxBlockForLate,
				testutil.Scaled(must.OK1(time.ParseDuration(s))))
		},
		"with-known-commands", func(arg string) {
			validCommands = strings.Fields(arg)
		},
		"highlight-in-global", evaltest.GoFnInGlobal("highlight", func(fm *eval.Frame, s string) {
			hl := highlight.NewHighlighter(highlight.Config{
				HasCommand: func(name string) bool { return slices.Contains(validCommands, name) },
			})
			text, tips := hl.Get(s)
			fmt.Fprint(fm.ByteOutput(), toStyledown(text))
			for i, tip := range tips {
				fmt.Fprintf(fm.ByteOutput(), "= tip %d:\n%s", i, toStyledown(tip))
			}
		}),
	)
}

var styleDefs = `
? fg-bright-white bg-red
G fg-green
R fg-red
M fg-magenta
Y fg-yellow
C fg-cyan
`[1:]

func toStyledown(t ui.Text) string { return must.OK1(styledown.Derender(t, styleDefs)) }
