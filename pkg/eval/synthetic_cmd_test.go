package eval

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyntheticCopyForMovePreservesModificationTimes(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	if err := os.Mkdir(src, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(src, "file")
	if err := os.WriteFile(file, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	wantFileTime := time.Unix(1000000000, 0)
	wantDirTime := time.Unix(1100000000, 0)
	for path, timestamp := range map[string]time.Time{file: wantFileTime, src: wantDirTime} {
		if err := os.Chtimes(path, timestamp, timestamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := syntheticCopyPathWithTimes(src, dst, true, true); err != nil {
		t.Fatal(err)
	}
	for path, timestamp := range map[string]time.Time{filepath.Join(dst, "file"): wantFileTime, dst: wantDirTime} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(timestamp) {
			t.Errorf("%s: modification time = %v, want %v", path, info.ModTime(), timestamp)
		}
	}
}

func TestSyntheticCopyAndRemove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.Mkdir(src, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(src, "file")
	if err := os.WriteFile(file, []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syntheticCopyPath(file, file, false); err == nil {
		t.Fatal("copying a file over itself did not fail")
	}
	if content, err := os.ReadFile(file); err != nil || string(content) != "content\n" {
		t.Fatalf("copy over itself changed the source: %q, %v", content, err)
	}
	if err := syntheticCp([]string{src, filepath.Join(dir, "dst")}); err == nil {
		t.Fatal("copying a directory without -r did not fail")
	}
	dst := filepath.Join(dir, "dst")
	if err := syntheticCp([]string{"-r", src, dst}); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(dst, "file")); err != nil || string(content) != "content\n" {
		t.Fatalf("recursive copy = %q, %v", content, err)
	}
	if err := syntheticCopyPath(src, filepath.Join(src, "nested"), true); err == nil {
		t.Fatal("copying a directory into itself did not fail")
	}
	if err := syntheticRm([]string{dst}); err == nil {
		t.Fatal("removing a directory without -r did not fail")
	}
	if err := syntheticRm([]string{"-rf", dst}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination still exists: %v", err)
	}
}

func TestSyntheticLsAndCat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := syntheticLs(&output, []string{dir}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "visible\n" {
		t.Fatalf("ls output = %q", output.String())
	}
	output.Reset()
	if err := syntheticLs(&output, []string{"-a", dir}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), ".hidden\n") {
		t.Fatalf("ls -a omitted hidden file: %q", output.String())
	}
	output.Reset()
	if err := syntheticCat(strings.NewReader("stdin"), &output, []string{filepath.Join(dir, "visible"), "-"}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "contentstdin" {
		t.Fatalf("cat output = %q", output.String())
	}
}
