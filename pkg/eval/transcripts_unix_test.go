//go:build unix

package eval_test

import (
	"os"
	"testing"
	"time"

	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/testutil"
	"golang.org/x/sys/unix"
)

func injectTimeAfterWithSIGINTOrSkip(t *testing.T) {
	testutil.Set(t, eval.TimeAfter,
		func(_ *eval.Frame, d time.Duration) <-chan time.Time {
			go unix.Kill(os.Getpid(), unix.SIGINT)
			return time.After(d)
		})
}
