package eval

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"src.elv.sh/pkg/testutil"
)

func TestSyntheticSort(t *testing.T) {
	for _, test := range []struct {
		name, input string
		args        []string
		want        string
	}{
		{"empty", "", nil, ""},
		{"byte ordering", "z\na\nB\n", nil, "B\na\nz\n"},
		{"unterminated line", "b\na", nil, "a\nb\n"},
		{"blank lines", "b\n\na\n\n", nil, "\n\na\nb\n"},
		{"reverse", "b\na\nc\n", []string{"-r"}, "c\nb\na\n"},
		{"unique", "b\na\nb\na\n", []string{"--unique"}, "a\nb\n"},
		{"numeric", "10\n2\n-3\n.5\n-.5\n", []string{"-n"}, "-3\n-.5\n.5\n2\n10\n"},
		{"numeric reverse", "10\n2\n-3\n", []string{"-nr"}, "10\n2\n-3\n"},
		{"numeric exact integers", "9007199254740993\n9007199254740992\n", []string{"-n"}, "9007199254740992\n9007199254740993\n"},
		{"numeric exact fractions", "1.0000000000000000002\n1.0000000000000000001\n", []string{"-n"}, "1.0000000000000000001\n1.0000000000000000002\n"},
		{"numeric prefixes", "  3 suffix\n2e9\n-1.5x\n", []string{"--numeric-sort"}, "-1.5x\n2e9\n  3 suffix\n"},
		{"non-numbers compare as zero", "+2\n0\n1\n2e3\n-1\n", []string{"-ns"}, "-1\n+2\n0\n1\n2e3\n"},
		{"numeric ties compare whole lines", "2 z\n02 a\n2 a\n", []string{"-n"}, "02 a\n2 a\n2 z\n"},
		{"stable numeric ties", "2 z\n02 a\n2 a\n", []string{"-ns"}, "2 z\n02 a\n2 a\n"},
		{"unique numeric keys", "2 z\n02 a\n1 first\n2 a\n", []string{"-nu"}, "1 first\n2 z\n"},
		{"fold case", "b\nB\na\nA\n", []string{"-f"}, "A\na\nB\nb\n"},
		{"stable folded keys", "a\nA\nb\nB\n", []string{"-fs"}, "a\nA\nb\nB\n"},
		{"unique folded keys", "a\nA\nb\nB\n", []string{"-fu"}, "a\nb\n"},
		{"ignore leading blanks", " z\nb\n a\n", []string{"--ignore-leading-blanks"}, " a\nb\n z\n"},
		{"CRLF preserved", "b\r\na\r\n", nil, "a\r\nb\r\n"},
		{"invalid bytes preserved when folding", "a\xff\nA\xff\na\xfe\n", []string{"-fu"}, "a\xfe\na\xff\n"},
		{"very long lines", strings.Repeat("z", 100000) + "\na\n", nil, "a\n" + strings.Repeat("z", 100000) + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := syntheticSort(strings.NewReader(test.input), &out, test.args); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.want {
				t.Errorf("got %q, want %q", out.String(), test.want)
			}
		})
	}
}

func TestSyntheticUniq(t *testing.T) {
	for _, test := range []struct {
		name, input string
		args        []string
		want        string
	}{
		{"empty", "", nil, ""},
		{"adjacent only", "a\na\nb\na\n", nil, "a\nb\na\n"},
		{"count", "a\na\nb\na\n", []string{"-c"}, "      2 a\n      1 b\n      1 a\n"},
		{"repeated", "a\na\nb\nc\nc\n", []string{"-d"}, "a\nc\n"},
		{"unique groups", "a\na\nb\nc\nc\n", []string{"-u"}, "b\n"},
		{"count repeated", "a\na\nb\nc\nc\n", []string{"-cd"}, "      2 a\n      2 c\n"},
		{"count unique", "a\na\nb\nc\nc\n", []string{"-cu"}, "      1 b\n"},
		{"both filters", "a\na\nb\n", []string{"-du"}, ""},
		{"fold case", "Apple\napple\nBANANA\nbanana\n", []string{"--ignore-case"}, "Apple\nBANANA\n"},
		{"blank groups", "\n\na\n\n", []string{"-c"}, "      2 \n      1 a\n      1 \n"},
		{"unterminated duplicate", "a\na", nil, "a\n"},
		{"CRLF preserved", "a\r\na\r\nb\r\n", nil, "a\r\nb\r\n"},
		{"invalid UTF8 preserved", "a\xff\nA\xff\nb\xfe\n", []string{"-i"}, "a\xff\nb\xfe\n"},
		{"very long lines", strings.Repeat("x", 100000) + "\n" + strings.Repeat("x", 100000), nil, strings.Repeat("x", 100000) + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := syntheticUniq(strings.NewReader(test.input), &out, test.args); err != nil {
				t.Fatal(err)
			}
			if out.String() != test.want {
				t.Errorf("got %q, want %q", out.String(), test.want)
			}
		})
	}
}

func TestSyntheticSortAndUniqFiles(t *testing.T) {
	dir := t.TempDir()
	a, b, dest := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "dest")
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	content := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	write(a, "z\na\n")
	write(b, "c\nb\n")
	var out bytes.Buffer
	if err := syntheticSort(strings.NewReader("d\n"), &out, []string{a, "-", b}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "a\nb\nc\nd\nz\n" {
		t.Errorf("sort multiple inputs: %q", out.String())
	}
	if err := syntheticSort(strings.NewReader(""), io.Discard, []string{"-o", a, a}); err != nil {
		t.Fatal(err)
	}
	if got := content(a); got != "a\nz\n" {
		t.Errorf("in-place sort: %q", got)
	}
	write(dest, "keep original\n")
	if err := syntheticSort(strings.NewReader(""), io.Discard, []string{"--output=" + dest, filepath.Join(dir, "missing")}); err == nil {
		t.Fatal("missing input must fail")
	}
	if got := content(dest); got != "keep original\n" {
		t.Errorf("sort truncated output after missing input: %q", got)
	}
	write(a, "a\na\nb\n")
	if err := syntheticUniq(strings.NewReader(""), io.Discard, []string{a, dest}); err != nil {
		t.Fatal(err)
	}
	if got := content(dest); got != "a\nb\n" {
		t.Errorf("uniq output file: %q", got)
	}
	if err := syntheticUniq(strings.NewReader(""), io.Discard, []string{a, a}); err == nil {
		t.Fatal("uniq must refuse identical input and output files")
	}
	if got := content(a); got != "a\na\nb\n" {
		t.Errorf("uniq changed source while rejecting output: %q", got)
	}
}

func TestSyntheticSortAndUniqErrors(t *testing.T) {
	want := errors.New("broken IO")
	broken := syntheticTextBrokenIO{want}
	for name, command := range map[string]func(io.Reader, io.Writer, []string) error{"sort": syntheticSort, "uniq": syntheticUniq} {
		if err := command(broken, io.Discard, nil); !errors.Is(err, want) {
			t.Errorf("%s read error = %v", name, err)
		}
		if err := command(strings.NewReader("x\n"), broken, nil); !errors.Is(err, want) {
			t.Errorf("%s write error = %v", name, err)
		}
		if err := command(strings.NewReader(""), io.Discard, []string{"-Z"}); err == nil {
			t.Errorf("%s unsupported flag must fail", name)
		}
	}
	for _, args := range [][]string{{"-o"}, {"--output"}, {"--output="}, {"--not-a-flag"}} {
		if err := syntheticSort(strings.NewReader(""), io.Discard, args); err == nil {
			t.Errorf("sort %v: expected error", args)
		}
	}
	if err := syntheticUniq(strings.NewReader(""), io.Discard, []string{"a", "b", "c"}); err == nil {
		t.Fatal("uniq must reject excess paths")
	}
	_, paths, err := syntheticSortArgs([]string{"--", "-input"})
	if err != nil || len(paths) != 1 || paths[0] != "-input" {
		t.Errorf("sort --: paths = %v, error = %v", paths, err)
	}
}

func TestSyntheticSortAndUniqDashOutput(t *testing.T) {
	testutil.InTempDir(t)
	var out bytes.Buffer
	if err := syntheticSort(strings.NewReader("b\na\n"), &out, []string{"-o", "-"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("-")
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || string(data) != "a\nb\n" {
		t.Fatalf("sort -o -: stdout = %q, file = %q", out.String(), data)
	}
	if err := syntheticUniq(strings.NewReader("b\nb\na\n"), &out, []string{"-", "-"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "b\na\n" {
		t.Fatalf("uniq - -: stdout = %q", out.String())
	}
}
