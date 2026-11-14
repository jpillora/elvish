package shell

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/jpillora/elvish/pkg/cli/term"
	"github.com/jpillora/elvish/pkg/sys"
	"github.com/jpillora/elvish/pkg/ui"
)

type streamTTY struct {
	*Session
	term.Writer
	reader  term.StreamReader
	mu      sync.Mutex
	cancel  context.CancelFunc
	signals chan os.Signal
	raw     int
}

func newStreamTTY(s *Session) *streamTTY {
	return &streamTTY{Session: s, Writer: term.NewStreamWriter(s.errOut)}
}
func (t *streamTTY) Setup() (func(), error) {
	ctx, cancel := context.WithCancel(t.Process.Context())
	t.mu.Lock()
	t.cancel = cancel
	t.raw = 0
	t.reader = term.StreamReader{Input: streamBytes{t.Session, ctx}}
	t.mu.Unlock()
	_, width := t.Size()
	// A Windows console child can enable Win32 input records and focus
	// reports. Restore VT key input before the in-process editor resumes.
	_, err := io.WriteString(t.errOut, "\x1b[?9001l\x1b[?1004l")
	if err == nil {
		err = term.SetupStream(t.errOut, width)
	}
	if err != nil {
		cancel()
	}
	return func() { _ = term.RestoreStream(t.errOut) }, err
}
func (t *streamTTY) ReadEvent() (term.Event, error) {
	t.mu.Lock()
	raw := t.raw > 0
	if raw {
		t.raw--
	}
	reader := t.reader
	t.mu.Unlock()
	var e term.Event
	var err error
	if raw {
		e, err = reader.ReadRawEvent()
	} else {
		e, err = reader.ReadEvent()
	}
	if !raw && e == term.K('C', ui.Ctrl) {
		return term.InterruptEvent{}, nil
	}
	return e, err
}
func (t *streamTTY) SetRawInput(n int) { t.mu.Lock(); t.raw = n; t.mu.Unlock() }
func (t *streamTTY) CloseReader() {
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()
}
func (t *streamTTY) NotifySignals() <-chan os.Signal {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.signals = make(chan os.Signal, 1)
	return t.signals
}
func (t *streamTTY) StopSignals() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.signals != nil {
		close(t.signals)
		t.signals = nil
	}
}
func (t *streamTTY) resize() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.signals != nil {
		select {
		case t.signals <- sys.SIGWINCH:
		default:
		}
	}
}

type streamBytes struct {
	session *Session
	ctx     context.Context
}

func (r streamBytes) ReadByteWithTimeout(timeout time.Duration) (byte, error) {
	ctx := r.ctx
	if timeout >= 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var b [1]byte
	_, err := r.session.readRaw(ctx, b[:])
	if errors.Is(err, context.DeadlineExceeded) {
		err = term.ErrReadTimeout
	}
	if errors.Is(err, context.Canceled) {
		if r.session.Process.Context().Err() != nil {
			err = io.EOF
		} else {
			err = term.ErrStopped
		}
	}
	// Remote transports have no ICRNL line discipline.
	if b[0] == '\r' {
		b[0] = '\n'
	}
	return b[0], err
}
