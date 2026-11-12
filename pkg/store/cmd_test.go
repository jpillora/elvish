package store_test

import (
	"testing"

	"github.com/jpillora/elvish/pkg/store"
	"github.com/jpillora/elvish/pkg/store/storetest"
)

func TestCmd(t *testing.T) {
	storetest.TestCmd(t, store.MustTempStore(t))
}
