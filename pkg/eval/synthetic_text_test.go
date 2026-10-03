package eval

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticHeadAndTail(t *testing.T) {
	input := "one\ntwo\nthree\nfour"
	for _, test := range []struct {
		name, command string
		args          []string
		input, want   string
	}{
		{"head defaults", "head", nil, input, input},
		{"head default ten", "head", nil, strings.Repeat("a\n", 10) + "b\nb\n", strings.Repeat("a\n", 10)},
		{"head lines", "head", []string{"-n", "2"}, input, "one\ntwo\n"},
		{"head joined count", "head", []string{"-n2"}, input, "one\ntwo\n"},
		{"head legacy count", "head", []string{"-2"}, input, "one\ntwo\n"},
		{"head plus count", "head", []string{"--lines=+2"}, input, "one\ntwo\n"},
		{"head zero", "head", []string{"-n0"}, input, ""},
		{"head excluding lines", "head", []string{"-n", "-2"}, input, "one\ntwo\n"},
		{"head excluding all", "head", []string{"-n-4"}, input, ""},
		{"head excluding zero", "head", []string{"-n-0"}, input, input},
		{"head bytes", "head", []string{"-c5"}, input, "one\nt"},
		{"head excluding bytes", "head", []string{"--bytes", "-5"}, input, "one\ntwo\nthree"},
		{"head excluding more bytes", "head", []string{"-c-100"}, input, ""},
		{"tail defaults", "tail", nil, input, input},
		{"tail default ten", "tail", nil, "a\na\n" + strings.Repeat("b\n", 10), strings.Repeat("b\n", 10)},
		{"tail lines", "tail", []string{"-n2"}, input, "three\nfour"},
		{"tail negative count", "tail", []string{"-n-2"}, input, "three\nfour"},
		{"tail legacy count", "tail", []string{"-2"}, input, "three\nfour"},
		{"tail zero", "tail", []string{"-n0"}, input, ""},
		{"tail from line", "tail", []string{"-n+2"}, input, "two\nthree\nfour"},
		{"tail from first", "tail", []string{"-n+1"}, input, input},
		{"tail from zero", "tail", []string{"-n+0"}, input, input},
		{"tail from missing line", "tail", []string{"-n+9"}, input, ""},
		{"tail bytes", "tail", []string{"-c5"}, input, "\nfour"},
		{"tail from byte", "tail", []string{"--bytes=+5"}, input, "two\nthree\nfour"},
		{"tail from missing byte", "tail", []string{"-c+99"}, input, ""},
		{"tail zero bytes", "tail", []string{"-c0"}, input, ""},
		{"tail empty input", "tail", nil, "", ""},
		{"head CRLF preserved", "head", []string{"-n1"}, "one\r\ntwo\r\n", "one\r\n"},
		{"tail CRLF preserved", "tail", []string{"-n1"}, "one\r\ntwo\r\n", "two\r\n"},
		{"head very long line", "head", []string{"-n1"}, strings.Repeat("x", 100000) + "\nlast\n", strings.Repeat("x", 100000) + "\n"},
		{"tail long line", "tail", []string{"-n1"}, "first\n" + strings.Repeat("x", 100000), strings.Repeat("x", 100000)},
		{"tail multi chunk bytes", "tail", []string{"-c40000"}, strings.Repeat("a", 50000) + strings.Repeat("b", 50000), strings.Repeat("b", 40000)},
		{"head excluding multi chunk bytes", "head", []string{"-c-40000"}, strings.Repeat("a", 50000) + strings.Repeat("b", 50000), strings.Repeat("a", 50000) + strings.Repeat("b", 10000)},
		{"tail huge count small input", "tail", []string{"-n9223372036854775807"}, input, input},
		{"head huge byte count", "head", []string{"-c9223372036854775807"}, input, input},
		{"last count wins", "tail", []string{"-n2", "-c4"}, input, "four"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := syntheticSliceText(test.command, strings.NewReader(test.input), &out, test.args); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.want {
				t.Errorf("got %q, want %q", out.String(), test.want)
			}
		})
	}
}

func TestSyntheticHeadAndTailFiles(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("a1\na2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("b1\nb2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		command string
		args    []string
		want    string
	}{
		{"head", []string{"-n1", a, b}, "==> " + a + " <==\na1\n\n==> " + b + " <==\nb1\n"},
		{"tail", []string{"-qn1", a, b}, "a2\nb2\n"},
		{"head", []string{"-vn1", a}, "==> " + a + " <==\na1\n"},
		{"tail", []string{"--verbose", "-n1", "-"}, "==> standard input <==\nstdin\n"},
		{"head", []string{"--verbose", "--quiet", "-n1", a}, "a1\n"},
		{"head", []string{"--", a}, "a1\na2\n"},
	} {
		var out bytes.Buffer
		if err := syntheticSliceText(test.command, strings.NewReader("stdin\n"), &out, test.args); err != nil {
			t.Fatal(err)
		}
		if out.String() != test.want {
			t.Errorf("%s %v: got %q, want %q", test.command, test.args, out.String(), test.want)
		}
	}
}

func TestSyntheticHeadAndTailErrors(t *testing.T) {
	for _, args := range [][]string{{"-n"}, {"-c"}, {"--lines"}, {"--bytes"}, {"-nno"}, {"-n+-2"}, {"-n++2"}, {"-n9223372036854775808"}, {"-n2.5"}, {"-Z"}, {"--follow"}, {"--quiet=yes"}} {
		for _, command := range []string{"head", "tail"} {
			if err := syntheticSliceText(command, strings.NewReader("input"), io.Discard, args); err == nil {
				t.Errorf("%s %v: expected error", command, args)
			}
		}
	}
}

func TestSyntheticWc(t *testing.T) {
	for _, test := range []struct {
		name, input string
		args        []string
		want        string
	}{
		{"empty", "", nil, "0 0 0\n"},
		{"default", "hello world\nlast", nil, "1 3 16\n"},
		{"lines count newlines not unterminated last line", "hello\nlast", []string{"-l"}, "1\n"},
		{"words Unicode whitespace", "café\u2003world\n", []string{"-w"}, "2\n"},
		{"characters differ from bytes", "café\n", []string{"-mc"}, "5 6\n"},
		{"CRLF bytes", "a\r\nb\r\n", []string{"-lwc"}, "2 2 6\n"},
		{"invalid UTF8 bytes", "\xff\xfe\n", []string{"-mc"}, "3 3\n"},
		{"stdin named", "a\n", []string{"-l", "-"}, "1 -\n"},
		{"all selected in fixed order", "café\n", []string{"-cmwl"}, "1 1 5 6\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := syntheticWc(strings.NewReader(test.input), &out, test.args); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.want {
				t.Errorf("got %q, want %q", out.String(), test.want)
			}
		})
	}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("one\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("two\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := syntheticWc(strings.NewReader(""), &out, []string{"-l", a, b}); err != nil {
		t.Fatal(err)
	}
	want := "1 " + a + "\n2 " + b + "\n3 total\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
	if err := syntheticWc(strings.NewReader(""), io.Discard, []string{"-Z"}); err == nil {
		t.Fatal("unsupported option must fail")
	}
}

type syntheticTextBrokenIO struct{ err error }

func (b syntheticTextBrokenIO) Read([]byte) (int, error)  { return 0, b.err }
func (b syntheticTextBrokenIO) Write([]byte) (int, error) { return 0, b.err }

func TestSyntheticTextPropagatesIOErrors(t *testing.T) {
	want := errors.New("broken IO")
	broken := syntheticTextBrokenIO{want}
	for _, command := range []string{"head", "tail"} {
		for _, args := range [][]string{{"-n1"}, {"-c1"}} {
			if err := syntheticSliceText(command, broken, io.Discard, args); !errors.Is(err, want) {
				t.Errorf("%s %v: read error = %v", command, args, err)
			}
			if err := syntheticSliceText(command, strings.NewReader("x\n"), broken, args); !errors.Is(err, want) {
				t.Errorf("%s %v: write error = %v", command, args, err)
			}
		}
	}
	if err := syntheticWc(broken, io.Discard, nil); !errors.Is(err, want) {
		t.Errorf("wc read error = %v", err)
	}
	if err := syntheticWc(strings.NewReader("x"), broken, nil); !errors.Is(err, want) {
		t.Errorf("wc write error = %v", err)
	}
}
