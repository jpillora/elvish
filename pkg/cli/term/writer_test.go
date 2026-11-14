package term

import (
	"strings"
	"testing"

	"github.com/jpillora/elvish/pkg/ui"
)

func TestWriter(t *testing.T) {
	sb := &strings.Builder{}
	testOutput := func(want string) {
		t.Helper()
		if sb.String() != want {
			t.Errorf("got %q, want %q", sb.String(), want)
		}
		sb.Reset()
	}

	w := NewWriter(sb)
	w.UpdateBuffer(
		ui.T("note 1"),
		NewBufferBuilder(10).Write("line 1").SetDotHere().Buffer(),
		false)
	testOutput(hideCursor + "\r \033[J\r\033[?7h\033[mnote 1\n\033[?7lline 1\r\033[6C" + showCursor)
}

func TestReflowedCursorLine(t *testing.T) {
	for _, tc := range []struct {
		name        string
		buf         *Buffer
		width, want int
	}{
		{"wrapped input", &Buffer{Width: 90, Lines: [][]Cell{{{Text: "abcdefghij"}}}, Dot: Pos{0, 9}}, 4, 2},
		{"cursor before right prompt", &Buffer{Width: 90, Lines: [][]Cell{{{Text: "abcdefghij"}}}, Dot: Pos{0, 1}}, 4, 0},
		{"multiple rows and wide text", &Buffer{Width: 90, Lines: [][]Cell{{{Text: "界界界"}}, nil, {{Text: "abcdef"}}}, Dot: Pos{2, 5}}, 4, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reflowedCursorLine(tc.buf, tc.width); got != tc.want {
				t.Fatalf("cursor line = %d, want %d", got, tc.want)
			}
		})
	}
}
