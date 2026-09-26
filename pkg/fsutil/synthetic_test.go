package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSyntheticPath(t *testing.T) {
	t.Setenv("PATH", "")
	if _, err := LookPath("ls"); err == nil {
		t.Fatal("ls resolved before synthetic PATH was added")
	}
	restore := AppendSyntheticPath()
	t.Cleanup(restore)
	if paths := filepath.SplitList(os.Getenv("PATH")); len(paths) != 1 || !samePath(paths[0], SyntheticPath()) {
		t.Fatalf("PATH = %q, want synthetic directory", paths)
	}
	path, err := LookPath("ls")
	if err != nil || !IsSyntheticPath(path) {
		t.Fatalf("LookPath(ls) = %q, %v", path, err)
	}
	if _, err := LookPath("not-a-synthetic-command"); err == nil {
		t.Fatal("unknown command resolved")
	}
	var commands []string
	EachExternal(func(name string) { commands = append(commands, name) })
	if !slices.Equal(commands, SyntheticCommands) {
		t.Fatalf("external commands = %q, want %q", commands, SyntheticCommands)
	}
	restore()
	if os.Getenv("PATH") != "" {
		t.Fatalf("PATH not restored: %q", os.Getenv("PATH"))
	}
}

func TestSyntheticPathStaysAtEnd(t *testing.T) {
	bin := t.TempDir()
	name := "ls"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	realPath := filepath.Join(bin, name)
	if err := os.WriteFile(realPath, []byte("fake executable"), 0755); err != nil {
		t.Fatal(err)
	}
	original := strings.Join([]string{bin, t.TempDir()}, string(filepath.ListSeparator))
	t.Setenv("PATH", original)
	restore := AppendSyntheticPath()
	t.Cleanup(restore)
	paths := filepath.SplitList(os.Getenv("PATH"))
	if len(paths) != 3 || !samePath(paths[2], SyntheticPath()) {
		t.Fatalf("PATH = %q", paths)
	}
	if path, err := LookPath("ls"); err != nil || !samePath(path, realPath) {
		t.Fatalf("LookPath(ls) = %q, %v; want real executable %q", path, err, realPath)
	}
	AppendSyntheticPath()() // Do not add a duplicate or change the original cleanup.
	if len(filepath.SplitList(os.Getenv("PATH"))) != 3 {
		t.Fatal("synthetic PATH was duplicated")
	}
}
