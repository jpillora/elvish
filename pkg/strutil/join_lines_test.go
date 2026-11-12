package strutil_test

import (
	"testing"

	"github.com/jpillora/elvish/pkg/strutil"
	"github.com/jpillora/elvish/pkg/tt"
)

var Args = tt.Args

func TestJoinLines(t *testing.T) {
	tt.Test(t, strutil.JoinLines,
		Args([]string(nil)).Rets(""),
		Args([]string{"foo"}).Rets("foo\n"),
		Args([]string{"foo", "bar"}).Rets("foo\nbar\n"),
	)
}
