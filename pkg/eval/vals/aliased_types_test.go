package vals

import (
	"testing"

	"github.com/jpillora/elvish/pkg/testutil"
	"github.com/jpillora/elvish/pkg/tt"
)

var Args = tt.Args

func TestMakeMap_PanicsWithOddNumberOfArguments(t *testing.T) {
	tt.Test(t, testutil.Recover,
		//lint:ignore SA5012 testing panic
		Args(func() { MakeMap("foo") }).Rets("odd number of arguments to MakeMap"),
	)
}
