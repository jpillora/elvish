//go:build unix

package term

import (
	"os"
)

// reader reads terminal escape sequences and decodes them into events.
type reader struct {
	fr fileReader
}

func newReader(f *os.File) *reader {
	fr, err := newFileReader(f)
	if err != nil {
		// TODO(xiaq): Do not panic.
		panic(err)
	}
	return &reader{fr}
}

func (rd *reader) ReadEvent() (Event, error) {
	return readEvent(rd.fr)
}

func (rd *reader) ReadRawEvent() (Event, error) {
	r, err := readRune(rd.fr, -1)
	return K(r), err
}

func (rd *reader) Close() {
	rd.fr.Stop()
	rd.fr.Close()
}
