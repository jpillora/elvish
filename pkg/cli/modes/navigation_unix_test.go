//go:build unix

package modes

import (
	"os"
	"syscall"
	"testing"

	. "github.com/jpillora/elvish/pkg/cli/clitest"
	"github.com/jpillora/elvish/pkg/cli/term"
	"github.com/jpillora/elvish/pkg/must"
	"github.com/jpillora/elvish/pkg/testutil"
)

func TestNavigation_NoPreviewForNamedPipes(t *testing.T) {
	// A previous implementation tried to call os.Open on named pipes, which can
	// block indefinitely. Ensure that this no longer happens.
	testutil.InTempDir(t)
	must.OK(os.Mkdir("d", 0777))
	must.OK(syscall.Mkfifo("d/pipe", 0666))
	must.OK(os.Chdir("d"))
	f := setupNav(t)
	defer f.Stop()

	// Use the default real FS cursor.
	startNavigation(f.App, NavigationSpec{})
	f.TestTTY(t,
		"", term.DotHere, "\n",
		" NAVIGATING  \n", Styles,
		"************ ",
		" d    pipe          no preview for named\n", Styles,
		"#### ++++++++++++++ !!!!!!!!!!!!!!!!!!!!",
		"                     pipe", Styles,
		"                    !!!!!",
	)
}
