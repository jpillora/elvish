package shell

import (
	"bytes"
	"context"
	"io"
)

const sessionInputLimit = 64 * 1024

// wake is called with mu held. Every waiter gets a new snapshot after waking.
func (s *Session) wake()             { close(s.changed); s.changed = make(chan struct{}) }
func (s *Session) CloseInput() error { s.mu.Lock(); s.eof = true; s.wake(); s.mu.Unlock(); return nil }
func (s *Session) interceptInterrupt() {
	if i := bytes.LastIndexByte(s.input, 3); i >= 0 {
		s.input = s.input[i+1:]
		s.line = nil
		s.lineReady = false
		s.commandCancel()
		s.wake()
	}
}
func (s *Session) Write(b []byte) (int, error) {
	n := 0
	for len(b) > 0 {
		if err := s.Process.Context().Err(); err != nil {
			return n, io.ErrClosedPipe
		}
		s.mu.Lock()
		if s.eof {
			s.mu.Unlock()
			return n, io.ErrClosedPipe
		}
		space := sessionInputLimit - len(s.input)
		if space > 0 {
			size := min(space, len(b))
			s.input = append(s.input, b[:size]...)
			n += size
			b = b[size:]
			if s.cfg.Terminal && s.commandCancel != nil {
				s.interceptInterrupt()
			}
			s.wake()
			s.mu.Unlock()
			continue
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-changed:
		case <-s.Process.Context().Done():
			return n, io.ErrClosedPipe
		}
	}
	return n, nil
}
func (s *Session) Read(b []byte) (int, error) { return s.ReadContext(s.Process.Context(), b) }

// ReadContext is used by evaluator commands; editor reads use readRaw instead.
func (s *Session) ReadContext(ctx context.Context, b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		s.mu.Lock()
		if !s.cfg.Terminal || s.commandRaw {
			s.mu.Unlock()
			return s.readRaw(ctx, b)
		}
		if s.lineReady {
			n := copy(b, s.line)
			s.line = s.line[n:]
			if len(s.line) == 0 {
				s.lineReady = false
			}
			s.mu.Unlock()
			return n, nil
		}
		if s.lineEOF {
			s.mu.Unlock()
			return 0, io.EOF
		}
		echo := make([]byte, 0)
		for len(s.input) > 0 && !s.lineReady && !s.lineEOF {
			c := s.input[0]
			s.input = s.input[1:]
			switch c {
			case '\r', '\n':
				s.line = append(s.line, '\n')
				s.lineReady = true
				echo = append(echo, '\n')
			case 4:
				if len(s.line) > 0 {
					s.lineReady = true
				} else {
					s.lineEOF = true
				}
			case 8, 127:
				if len(s.line) > 0 {
					i := len(s.line) - 1
					for i > 0 && s.line[i]&0xc0 == 0x80 {
						i--
					}
					s.line = s.line[:i]
					echo = append(echo, '\b', ' ', '\b')
				}
			default:
				if len(s.line) < sessionInputLimit {
					s.line = append(s.line, c)
					echo = append(echo, c)
				}
			}
		}
		ready := s.lineReady || s.lineEOF
		eof := s.eof
		if eof && len(s.line) > 0 {
			s.lineReady = true
			ready = true
		}
		s.wake()
		changed := s.changed
		s.mu.Unlock()
		if len(echo) > 0 {
			if _, err := s.out.Write(echo); err != nil {
				return 0, err
			}
		}
		if ready {
			continue
		}
		if eof {
			return 0, io.EOF
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-s.Process.Context().Done():
			return 0, io.EOF
		}
	}
}
func (s *Session) readRaw(ctx context.Context, b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if s.Process.Context().Err() != nil {
			return 0, io.EOF
		}
		s.mu.Lock()
		if len(s.input) > 0 {
			n := copy(b, s.input)
			s.input = s.input[n:]
			s.wake()
			s.mu.Unlock()
			return n, nil
		}
		if s.eof {
			s.mu.Unlock()
			return 0, io.EOF
		}
		changed := s.changed
		s.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-s.Process.Context().Done():
			return 0, io.EOF
		}
	}
}
