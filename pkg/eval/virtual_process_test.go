package eval_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/parse"
)

func TestVirtualProcessesIsolateDirectoryEnvironmentAndFiles(t *testing.T) {
	hostDir, _ := os.Getwd()
	hostValue, hostSet := os.LookupEnv("ELVISH_VIRTUAL_TEST")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "child"), 0700); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func(i int, dir string) {
			defer wg.Done()
			p, err := eval.NewProcess(context.Background(), dir, []string{"PATH=", "HOME=" + dir})
			if err != nil {
				t.Error(err)
				return
			}
			defer p.Close()
			ev := eval.NewEvalerWithProcess(p)
			code := fmt.Sprintf("cd child; set E:ELVISH_VIRTUAL_TEST = session%d; echo $E:ELVISH_VIRTUAL_TEST > input.txt; cp input.txt copy.txt; cat *.txt > result; pwd > location", i)
			if err := ev.Eval(parse.Source{Name: "isolation", Code: code}, eval.EvalCfg{}); err != nil {
				t.Error(err)
				return
			}
			result, err := os.ReadFile(filepath.Join(dir, "child", "result"))
			if err != nil || strings.TrimSpace(string(result)) != fmt.Sprintf("session%d\nsession%d", i, i) {
				t.Errorf("result = %q, %v", result, err)
			}
			location, err := os.ReadFile(filepath.Join(dir, "child", "location"))
			if err != nil || strings.TrimSpace(string(location)) != filepath.Join(dir, "child") {
				t.Errorf("location = %q, %v", location, err)
			}
		}(i, dir)
	}
	wg.Wait()
	if dir, _ := os.Getwd(); dir != hostDir {
		t.Errorf("host directory changed to %s", dir)
	}
	if v, ok := os.LookupEnv("ELVISH_VIRTUAL_TEST"); v != hostValue || ok != hostSet {
		t.Error("host environment changed")
	}
}

func TestVirtualExitStopsOnlyItsSession(t *testing.T) {
	dir := t.TempDir()
	p, err := eval.NewProcess(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ev := eval.NewEvalerWithProcess(p)
	_ = ev.Eval(parse.Source{Name: "exit", Code: "exit 37; echo BAD > after-exit"}, eval.EvalCfg{})
	if code, exited := p.ExitStatus(); !exited || code != 37 {
		t.Fatalf("exit = %d, %v", code, exited)
	}
	if _, err := os.Stat(filepath.Join(dir, "after-exit")); !os.IsNotExist(err) {
		t.Fatalf("command after exit ran: %v", err)
	}
	q, err := eval.NewProcess(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if err := eval.NewEvalerWithProcess(q).Eval(parse.Source{Name: "alive", Code: "echo ALIVE > other-session"}, eval.EvalCfg{}); err != nil {
		t.Fatal(err)
	}
}
