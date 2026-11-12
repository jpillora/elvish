package eval

import (
	"testing"
	"unsafe"

	"github.com/jpillora/elvish/pkg/eval/vals"
	"github.com/jpillora/elvish/pkg/persistent/hash"
)

func TestGoFnAsValue(t *testing.T) {
	fn1 := NewGoFn("fn1", func() {})
	fn2 := NewGoFn("fn2", func() {})
	vals.TestValue(t, fn1).
		Kind("fn").
		Hash(hash.Pointer(unsafe.Pointer(fn1.(*goFn)))).
		Equal(fn1).
		NotEqual(fn2).
		Repr("<builtin fn1>")
}
