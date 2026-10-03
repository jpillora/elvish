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

func grepStatus(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit ExternalCmdExit
	if !errors.As(err, &exit) {
		t.Fatalf("expected command exit, got %v", err)
	}
	return exit.ExitStatus()
}

func TestSyntheticGrep(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		args        []string
		want        string
		status      int
	}{
		{"basic", "apple\npear\napples", []string{"apple"}, "apple\napples\n", 0},
		{"no match", "apple\n", []string{"pear"}, "", 1},
		{"empty input", "", []string{""}, "", 1},
		{"empty pattern", "one\n\ntwo\n", []string{""}, "one\n\ntwo\n", 0},
		{"newline patterns", "one\ntwo\nthree\n", []string{"-e", "one\ntwo"}, "one\ntwo\n", 0},
		{"trailing empty pattern", "one\ntwo\n", []string{"-e", "one\n"}, "one\ntwo\n", 0},
		{"number case invert", "APPLE\npear\napple\n", []string{"-inv", "apple"}, "2:pear\n", 0},
		{"count", "apple\npear\napple\n", []string{"--count", "apple"}, "2\n", 0},
		{"count none", "apple\n", []string{"-c", "pear"}, "0\n", 1},
		{"quiet", "apple\n", []string{"--quiet", "apple"}, "", 0},
		{"whole", "apple\napples\n", []string{"-x", "apple"}, "apple\n", 0},
		{"whole union", "apple\npear\napples\npears\n", []string{"-x", "-eapple", "-epear"}, "apple\npear\n", 0},
		{"fixed", "a.b\naxb\n", []string{"-F", "a.b"}, "a.b\n", 0},
		{"basic literal operators", "a+b?\nab\n", []string{"a+b?"}, "a+b?\n", 0},
		{"literal internal anchors", "a^b\na$b\n", []string{`a^b\|a$b`}, "a^b\na$b\n", 0},
		{"basic operators", "a\naa\nb\n", []string{`^\(a\|b\)\{2\}$`}, "aa\n", 0},
		{"extended", "a\naa\nb\n", []string{"-E", "^(a|b){2}$"}, "aa\n", 0},
		{"classes", "a+\na?\n1+\n", []string{`^[[:alpha:]+?]*$`}, "a+\na?\n", 0},
		{"initial bracket", "]\na\nb\n", []string{`^[]a]$`}, "]\na\n", 0},
		{"literal class backslash", "\\\nw\nx\n", []string{`^[\w]$`}, "\\\nw\n", 0},
		{"only matches longest", "aaa baaa\n", []string{"-Eon", "a|aaa"}, "1:aaa\n1:aaa\n", 0},
		{"only zero length", "abc\n", []string{"-o", ""}, "", 0},
		{"only inverted", "abc\n", []string{"-vo", "xyz"}, "", 0},
		{"filename stdin", "abc\n", []string{"-Hn", "abc"}, "(standard input):1:abc\n", 0},
		{"list matching stdin", "abc\n", []string{"-l", "abc"}, "(standard input)\n", 0},
		{"list nonmatching", "abc\n", []string{"-L", "xyz"}, "(standard input)\n", 1},
		{"list matching status", "abc\n", []string{"-L", "abc"}, "", 0},
		{"CRLF", "abc\r\nxyz\r\n", []string{"abc"}, "abc\r\n", 0},
		{"Unicode fold", "CAFÉ\ncafé\n", []string{"-i", "café"}, "CAFÉ\ncafé\n", 0},
		{"raw input bytes", "a\xff\nb\xfe\n", []string{"a"}, "a\xff\n", 0},
		{"long lines", strings.Repeat("a", 100000), []string{"a"}, strings.Repeat("a", 100000) + "\n", 0},
		{"literal dash", "-x\ny\n", []string{"--", "-x"}, "-x\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			err := syntheticGrep(strings.NewReader(tc.input), &out, &diagnostic, tc.args)
			if got := grepStatus(t, err); got != tc.status {
				t.Errorf("status %d, want %d; %s", got, tc.status, diagnostic.String())
			}
			if out.String() != tc.want {
				t.Errorf("output %q, want %q", out.String(), tc.want)
			}
			if diagnostic.Len() != 0 {
				t.Errorf("unexpected diagnostic %s", &diagnostic)
			}
		})
	}
}

func TestSyntheticGrepFiles(t *testing.T) {
	dir := t.TempDir()
	a, b, p, empty := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "patterns"), filepath.Join(dir, "empty")
	for path, data := range map[string]string{a: "apple\npear\n", b: "pear\n", p: "apple\n", empty: ""} {
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		args   []string
		want   string
		status int
	}{
		{[]string{"-n", "apple", a, b}, a + ":1:apple\n", 0},
		{[]string{"-hc", "apple", a, b}, "1\n0\n", 0},
		{[]string{"-cl", "apple", a, b}, a + "\n", 0},
		{[]string{"-L", "apple", a, b}, b + "\n", 0},
		{[]string{"-f", p, a}, "apple\n", 0},
		{[]string{"-f", empty, a}, "", 1},
		{[]string{"-v", "-f", empty, a}, "apple\npear\n", 0},
		{[]string{"-f", p, "-e", "pear", a}, "apple\npear\n", 0},
		{[]string{"-f", "-", a}, "apple\n", 0},
	} {
		var out, diagnostic bytes.Buffer
		err := syntheticGrep(strings.NewReader("apple\n"), &out, &diagnostic, tc.args)
		if got := grepStatus(t, err); got != tc.status || out.String() != tc.want || diagnostic.Len() != 0 {
			t.Errorf("%v: output %q, status %d, diagnostic %s", tc.args, &out, got, &diagnostic)
		}
	}
	// NUL patterns from files must not be mistaken for a pattern-file sentinel.
	os.WriteFile(p, []byte("\x00file:any\n"), 0644)
	var out, diagnostic bytes.Buffer
	err := syntheticGrep(strings.NewReader("\x00file:any\n"), &out, &diagnostic, []string{"-Ff", p})
	if grepStatus(t, err) != 0 || out.String() != "\x00file:any\n" {
		t.Fatalf("NUL pattern: %v %q %s", err, &out, &diagnostic)
	}
}

func TestSyntheticGrepErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"[[.a.]]"}, {"[[=a=]]"}, {"-Z"}, {"--recursive"}, {"-e"}, {"-f"}, {"-EF", "a"}, {"["}, {`\(a\)\1`}, {"-f", filepath.Join(t.TempDir(), "missing")}, {"a", filepath.Join(t.TempDir(), "missing")}} {
		var diagnostic bytes.Buffer
		err := syntheticGrep(strings.NewReader("a\n"), io.Discard, &diagnostic, args)
		if grepStatus(t, err) != 2 || diagnostic.Len() == 0 {
			t.Errorf("%v: %v, %s", args, err, &diagnostic)
		}
	}
	broken := syntheticTextBrokenIO{errors.New("broken IO")}
	var diagnostic bytes.Buffer
	if err := syntheticGrep(broken, io.Discard, &diagnostic, []string{"a"}); grepStatus(t, err) != 2 {
		t.Error(err)
	}
	if err := syntheticGrep(strings.NewReader("a\n"), broken, io.Discard, []string{"a"}); !errors.Is(err, broken.err) {
		t.Errorf("write error: %v", err)
	}
	if err := syntheticGrep(strings.NewReader(""), io.Discard, broken, nil); !errors.Is(err, broken.err) {
		t.Errorf("diagnostic error: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	diagnostic.Reset()
	if err := syntheticGrep(strings.NewReader("a\n"), io.Discard, &diagnostic, []string{"-q", "a", missing, "-"}); err != nil || diagnostic.Len() == 0 {
		t.Errorf("quiet match overrides file error: %v %s", err, &diagnostic)
	}
}
