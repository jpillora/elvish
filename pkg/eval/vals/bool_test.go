package vals

import (
	"testing"

	"github.com/jpillora/elvish/pkg/tt"
)

type customBooler struct{ b bool }

func (b customBooler) Bool() bool { return b.b }

type customNonBooler struct{}

func TestBool(t *testing.T) {
	tt.Test(t, Bool,
		Args(nil).Rets(false),

		Args(true).Rets(true),
		Args(false).Rets(false),

		Args(customBooler{true}).Rets(true),
		Args(customBooler{false}).Rets(false),

		Args(customNonBooler{}).Rets(true),
	)
}
