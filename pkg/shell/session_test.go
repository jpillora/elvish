package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type sessionTestOutput struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (o *sessionTestOutput) Write(b []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.b.Write(b)
}
func (o *sessionTestOutput) String() string { o.mu.Lock(); defer o.mu.Unlock(); return o.b.String() }
func newTestSession(t *testing.T, terminal bool) (*Session, *sessionTestOutput, string) {
	t.Helper()
	dir := t.TempDir()
	out := new(sessionTestOutput)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	s, err := NewSession(SessionConfig{Context: ctx, Dir: dir, Env: []string{"HOME=" + dir, "PATH=", "XDG_CONFIG_HOME=" + dir}, Stdout: out, Terminal: terminal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, out, dir
}
func awaitSession(t *testing.T, predicate func() bool, out *sessionTestOutput) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !predicate() {
		if time.Now().After(deadline) {
			t.Fatalf("session stalled: %q", out.String())
		}
		time.Sleep(time.Millisecond)
	}
}
func TestStreamSessionExec(t *testing.T) {
	s, out, _ := newTestSession(t, false)
	s.Write([]byte("pear\napple\n"))
	s.CloseInput()
	code := "cat | sort; echo error >&2; exit 37; echo unreachable"
	if status := s.Run(&code); status != 37 {
		t.Fatalf("exit=%d, output=%s", status, out.String())
	}
	if got := out.String(); got != "apple\npear\nerror\n" {
		t.Fatalf("output=%q", got)
	}
}
func TestStreamSessionEditorInterruptAndTypeahead(t *testing.T) {
	s, out, dir := newTestSession(t, true)
	done := make(chan int, 1)
	go func() { done <- s.Run(nil) }()
	awaitSession(t, func() bool { return strings.Contains(out.String(), "\x1b[?25h") }, out)
	// One packet, no sleeps between Ctrl+C and the replacement command.
	s.Write([]byte("echo unwanted > cancelled\x03echo 雪🙂 > result\recho second > second\r"))
	fileIs := func(name, want string) func() bool {
		return func() bool {
			b, e := os.ReadFile(filepath.Join(dir, name))
			return e == nil && strings.TrimSpace(string(b)) == want
		}
	}
	awaitSession(t, fileIs("second", "second"), out)
	awaitSession(t, fileIs("result", "雪🙂"), out)
	if _, err := os.Stat(filepath.Join(dir, "cancelled")); !os.IsNotExist(err) {
		t.Fatal("cancelled editor input executed")
	}
	// Cancel a builtin that is blocked on terminal stdin, then continue.
	s.Write([]byte("echo started > started; cat > captured\r"))
	awaitSession(t, fileIs("started", "started"), out)
	s.Write([]byte("line\r"))
	awaitSession(t, fileIs("captured", "line"), out)
	s.Write([]byte("\x03echo resumed > resumed\r"))
	awaitSession(t, fileIs("resumed", "resumed"), out)
	// Resize while editing and verify EOF after queued exit doesn't discard code.
	s.Resize(34, 12)
	s.Write([]byte("exit 9\r"))
	s.CloseInput()
	select {
	case status := <-done:
		if status != 9 {
			t.Fatalf("exit=%d output=%s", status, out.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session didn't exit")
	}
}
func TestStreamSessionConcurrentIsolationAndModules(t *testing.T) {
	wd, _ := os.Getwd()
	before, wasSet := os.LookupEnv("ELVISH_SESSION_ISOLATION")
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		s, out, dir := newTestSession(t, false)
		os.Mkdir(filepath.Join(dir, "child"), 0700)
		os.WriteFile(filepath.Join(dir, "child", "mod.elv"), []byte("fn value { echo $E:ELVISH_SESSION_ISOLATION }"), 0600)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code := fmt.Sprintf("cd child; set E:ELVISH_SESSION_ISOLATION = %d; use ./mod; mod:value; use os; use path; use file; var f = (file:open-output result); echo data > $f; file:close $f; echo (path:abs result); echo (os:exists result); echo $pid", i)
			if status := s.Run(&code); status != 0 {
				t.Errorf("exit=%d output=%s", status, out.String())
				return
			}
			want := fmt.Sprintf("%d\n%s\n$true\nvirtual-", i, filepath.Join(dir, "child", "result"))
			if !strings.HasPrefix(out.String(), want) {
				t.Errorf("output=%q want prefix=%q", out.String(), want)
			}
		}(i)
	}
	wg.Wait()
	afterWD, _ := os.Getwd()
	after, isSet := os.LookupEnv("ELVISH_SESSION_ISOLATION")
	if wd != afterWD || before != after || wasSet != isSet {
		t.Fatal("host process state changed")
	}
}
func TestStreamSessionBackgroundCleanup(t *testing.T) {
	s, out, _ := newTestSession(t, false)
	code := "sleep 30 &; echo done"
	start := time.Now()
	status := s.Run(&code)
	if status != 0 || time.Since(start) > time.Second {
		t.Fatalf("exit=%d elapsed=%v output=%s", status, time.Since(start), out.String())
	}
}
func TestStreamSessionEOFAtPrompt(t *testing.T) {
	s, _, _ := newTestSession(t, true)
	s.CloseInput()
	if status := s.Run(nil); status != 0 {
		t.Fatalf("exit=%d", status)
	}
}
func TestStreamSessionNonTerminalBytes(t *testing.T) {
	s, out, _ := newTestSession(t, false)
	input := []byte{0, 3, 4, 13, 10, 255}
	s.Write(input)
	s.CloseInput()
	code := "cat"
	if status := s.Run(&code); status != 0 {
		t.Fatalf("exit=%d", status)
	}
	if !bytes.Equal([]byte(out.String()), input) {
		t.Fatalf("binary stdin changed: %q", out.String())
	}
	if _, err := s.Write([]byte("late")); err != io.ErrClosedPipe {
		t.Fatalf("late write=%v", err)
	}
}

func TestStreamSessionCloseAtPrompt(t *testing.T) {
	s, out, _ := newTestSession(t, true)
	done := make(chan int, 1)
	go func() { done <- s.Run(nil) }()
	awaitSession(t, func() bool { return strings.Contains(out.String(), "\x1b[?25h") }, out)
	s.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close left editor running")
	}
}
