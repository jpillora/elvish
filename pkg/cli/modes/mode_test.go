package modes

import (
	"errors"
	"testing"

	"github.com/jpillora/elvish/pkg/cli"
	"github.com/jpillora/elvish/pkg/cli/clitest"
	"github.com/jpillora/elvish/pkg/cli/tk"
	"github.com/jpillora/elvish/pkg/tt"
	"github.com/jpillora/elvish/pkg/ui"
)

var Args = tt.Args

func TestModeLine(t *testing.T) {
	testModeLine(t, modeLine)
}

func TestModePrompt(t *testing.T) {
	prompt := func(s string, b bool) ui.Text { return modePrompt(s, b)() }
	testModeLine(t, tt.Fn(prompt).Named("prompt"))

}

func testModeLine(t *testing.T, fn any) {
	tt.Test(t, fn,
		Args("TEST", false).Rets(
			ui.T("TEST", ui.Bold, ui.FgWhite, ui.BgMagenta)),
		Args("TEST", true).Rets(
			ui.Concat(
				ui.T("TEST", ui.Bold, ui.FgWhite, ui.BgMagenta),
				ui.T(" "))),
	)
}

// Common test utilities.

var errMock = errors.New("mock error")

var withNonCodeAreaAddon = clitest.WithSpec(func(spec *cli.AppSpec) {
	spec.State.Addons = []tk.Widget{tk.Label{}}
})

func startMode(app cli.App, w tk.Widget, err error) {
	if w != nil {
		app.PushAddon(w)
		app.Redraw()
	}
	if err != nil {
		app.Notify(ErrorText(err))
	}
}

func testFocusedWidgetNotCodeArea(t *testing.T, fn func(cli.App) error) {
	t.Helper()

	f := clitest.Setup(withNonCodeAreaAddon)
	defer f.Stop()
	if err := fn(f.App); err != ErrFocusedWidgetNotCodeArea {
		t.Errorf("should return ErrFocusedWidgetNotCodeArea, got %v", err)
	}
}
