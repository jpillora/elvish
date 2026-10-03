package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticSortReplacesWindowsSystemUtility(t *testing.T) {
	root := t.TempDir()
	system := filepath.Join(root, "System32")
	user := filepath.Join(root, "user-bin")
	for _, dir := range []string{system, user} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sort.exe"), nil, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SystemRoot", root)
	t.Setenv("PATH", system)
	restore := AppendSyntheticPath()
	t.Cleanup(restore)
	path, err := LookPath("sort")
	if err != nil || !IsSyntheticPath(path) {
		t.Fatalf("bare sort = %q, %v; want fallback", path, err)
	}
	path, err = LookPath("sort.exe")
	if err != nil || !samePath(path, filepath.Join(system, "sort.exe")) {
		t.Fatalf("explicit sort.exe = %q, %v; want Windows utility", path, err)
	}
	if !isNativeWindowsSort(strings.ToUpper(filepath.Join(system, "sort.exe"))) {
		t.Fatal("system utility matching must be case-insensitive")
	}
	t.Setenv("PATH", user+string(filepath.ListSeparator)+os.Getenv("PATH"))
	path, err = LookPath("sort")
	if err != nil || !samePath(path, filepath.Join(user, "sort.exe")) {
		t.Fatalf("user sort = %q, %v; want installed executable", path, err)
	}
	t.Setenv("PATH", system)
	path, err = LookPath("sort")
	if err != nil || !samePath(path, filepath.Join(system, "sort.exe")) {
		t.Fatalf("disabled fallback: sort = %q, %v; want Windows utility", path, err)
	}
}
