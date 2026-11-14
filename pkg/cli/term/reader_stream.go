package term

import "time"

// ByteReader supports VT input from a remote terminal on every host platform.
// Negative timeouts mean no deadline. A timeout returns ErrReadTimeout.
type ByteReader interface {
	ReadByteWithTimeout(time.Duration) (byte, error)
}

var ErrReadTimeout = errTimeout

// StreamReader decodes terminal bytes without touching an OS console. The
// caller owns cancellation of its ByteReader and the underlying transport.
type StreamReader struct{ Input ByteReader }

func (r StreamReader) ReadEvent() (Event, error) { return readEvent(r.Input) }
func (r StreamReader) ReadRawEvent() (Event, error) {
	c, err := readRune(r.Input, -1)
	return K(c), err
}
