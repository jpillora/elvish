package re

import (
	"github.com/jpillora/elvish/pkg/eval/vals"
)

type matchStruct struct {
	Text   string
	Start  int
	End    int
	Groups vals.List
}

type submatchStruct struct {
	Text  string
	Start int
	End   int
}
