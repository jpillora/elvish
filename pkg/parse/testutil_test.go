package parse

import (
	"reflect"

	"github.com/jpillora/elvish/pkg/tt"
)

var Args = tt.Args

var nodeType = reflect.TypeOf((*Node)(nil)).Elem()
