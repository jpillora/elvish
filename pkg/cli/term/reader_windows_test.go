package term

import (
	"testing"
	"unicode/utf16"

	"github.com/jpillora/elvish/pkg/sys/ewindows"
	"github.com/jpillora/elvish/pkg/tt"
	"github.com/jpillora/elvish/pkg/ui"
)

func TestConvertEvent(t *testing.T) {
	r1, r2 := utf16.EncodeRune('😀')

	tt.Test(t, convertEvent,
		Args(&ewindows.WindowBufferSizeEvent{}).Rets(ResizeEvent{}),
		// Mouse events are currently ignored.
		Args(&ewindows.MouseEvent{}).Rets(nil),
		// Ignore ordinary keyup events but accept Alt+numpad characters.
		Args(&ewindows.KeyEvent{BKeyDown: 0}).Rets(nil),
		Args(&ewindows.KeyEvent{BKeyDown: 0, WVirtualKeyCode: 'A', UChar: [2]byte{'a', 0}}).Rets(nil),
		Args(altRelease('é')).Rets(K('é')),
		Args(altRelease(uint16(r1))).Rets(surrogateKeyEvent{r1}),
		Args(altRelease(uint16(r2))).Rets(surrogateKeyEvent{r2}),

		Args(charKeyEvent('a', 0)).Rets(K('a')),
		Args(charKeyEvent('A', shift)).Rets(K('A')),
		Args(charKeyEvent('µ', leftCtrl|rightAlt)).Rets(K('µ')),
		Args(charKeyEvent('ẞ', leftCtrl|rightAlt|shift)).Rets(K('ẞ')),

		Args(charKeyEvent(uint16(r1), 0)).Rets(surrogateKeyEvent{r1}),
		Args(charKeyEvent(uint16(r2), 0)).Rets(surrogateKeyEvent{r2}),

		Args(funcKeyEvent(0x1b, 0)).Rets(K('[', ui.Ctrl)),

		// Functional key with modifiers
		Args(funcKeyEvent(0x08, 0)).Rets(K(ui.Backspace)),
		Args(funcKeyEvent(0x08, leftCtrl)).Rets(K(ui.Backspace, ui.Ctrl)),
		Args(funcKeyEvent(0x08, leftCtrl|leftAlt|shift)).Rets(K(ui.Backspace, ui.Ctrl, ui.Alt, ui.Shift)),

		// Functional keys with an alphanumeric base
		Args(funcKeyEvent('2', leftCtrl)).Rets(K('2', ui.Ctrl)),
		Args(funcKeyEvent('A', leftCtrl)).Rets(K('A', ui.Ctrl)),
		Args(funcKeyEvent('A', leftAlt)).Rets(K('a', ui.Alt)),

		// Unrecognized functional key
		Args(funcKeyEvent(0, 0)).Rets(nil),
	)
}

func altRelease(r uint16) *ewindows.KeyEvent {
	return &ewindows.KeyEvent{
		WVirtualKeyCode: 0x12, UChar: [2]byte{byte(r), byte(r >> 8)}}
}

func charKeyEvent(r uint16, mod uint32) *ewindows.KeyEvent {
	return &ewindows.KeyEvent{
		BKeyDown: 1, DwControlKeyState: mod, UChar: [2]byte{byte(r), byte(r >> 8)}}
}

func funcKeyEvent(code uint16, mod uint32) *ewindows.KeyEvent {
	return &ewindows.KeyEvent{
		BKeyDown: 1, DwControlKeyState: mod, WVirtualKeyCode: code}
}
