package store_test

import (
	"testing"

	"github.com/jpillora/elvish/pkg/store"
	"github.com/jpillora/elvish/pkg/store/storetest"
)

func TestDir(t *testing.T) {
	storetest.TestDir(t, store.MustTempStore(t))
}
