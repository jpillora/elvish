package eval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jpillora/elvish/pkg/eval/vars"
	"github.com/jpillora/elvish/pkg/fsutil"
)

// Process is the root process of an embedded shell. Its directory, environment
// and exit state belong to the session, never to the embedding Go process.
// A nil *Process preserves the standalone shell's OS-process behavior.
// This is state isolation, not a filesystem or privilege sandbox.
type Process struct {
	ID       string
	mu       sync.RWMutex
	cwd      string
	env      map[string]string
	ctx      context.Context
	cancel   context.CancelFunc
	exitCode int
	exited   bool
	// RunExternal optionally supplies the terminal-aware child process runner.
	// It must honor ctx and reap the child before returning.
	RunExternal func(ctx context.Context, cmd *exec.Cmd, terminal bool) (*os.ProcessState, error)
	// TerminalPorts identifies the session's terminal streams. Redirected and
	// piped commands use ordinary child process I/O instead.
	TerminalPorts []*Port
	DefaultPorts  []*Port
	TerminalSize  func() (rows, cols int)
	jobs          sync.WaitGroup
}

var processID atomic.Uint64

func NewProcess(ctx context.Context, dir string, env []string) (*Process, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	ctx, cancel := context.WithCancel(ctx)
	p := &Process{ID: fmt.Sprintf("virtual-%d", processID.Add(1)), cwd: dir, env: make(map[string]string), ctx: ctx, cancel: cancel}
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name != "" {
			p.env[envKey(name)] = value
		}
	}
	p.env[envKey("PWD")] = dir
	return p, nil
}

func envKey(name string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(name)
	}
	return name
}

func (p *Process) Wait() { p.jobs.Wait() }

func (p *Process) Context() context.Context { return p.ctx }
func (p *Process) Close()                   { p.cancel() }
func (p *Process) Exit(code int) error {
	p.mu.Lock()
	if !p.exited {
		p.exited = true
		p.exitCode = code
	}
	p.mu.Unlock()
	p.cancel()
	return ProcessExit{code}
}
func (p *Process) ExitStatus() (int, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.exitCode, p.exited
}

// ProcessExit ends only the virtual root process.
type ProcessExit struct{ Code int }

func (e ProcessExit) Error() string { return fmt.Sprintf("shell exited with status %d", e.Code) }

func (p *Process) Getwd() (string, error) {
	if p == nil {
		return getwd()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cwd, nil
}
func (p *Process) Path(path string) string {
	if p == nil || path == "" || filepath.IsAbs(path) {
		return path
	}
	wd, _ := p.Getwd()
	if runtime.GOOS == "windows" {
		// Root-relative paths inherit this session's drive, not the host's.
		if strings.HasPrefix(path, `\`) || strings.HasPrefix(path, "/") {
			return filepath.VolumeName(wd) + path
		}
		if volume := filepath.VolumeName(path); volume != "" {
			if strings.EqualFold(volume, filepath.VolumeName(wd)) {
				path = path[len(volume):]
			} else {
				return volume + `\` + path[len(volume):]
			}
		}
	}
	return filepath.Join(wd, path)
}
func (p *Process) Abs(path string) (string, error) {
	if p == nil {
		return filepath.Abs(path)
	}
	if path == "" {
		return p.Getwd()
	}
	return filepath.Clean(p.Path(path)), nil
}
func (p *Process) Chdir(path string) error {
	if p == nil {
		return os.Chdir(path)
	}
	path = p.Path(path)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.cwd = path
	p.env[envKey("PWD")] = path
	p.mu.Unlock()
	return nil
}
func (p *Process) LookupEnv(name string) (string, bool) {
	if p == nil {
		return os.LookupEnv(name)
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.env[envKey(name)]
	return v, ok
}
func (p *Process) Getenv(name string) string { v, _ := p.LookupEnv(name); return v }
func (p *Process) Setenv(name, value string) error {
	if p == nil {
		return os.Setenv(name, value)
	}
	if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, 0) {
		return errors.New("invalid environment variable")
	}
	p.mu.Lock()
	p.env[envKey(name)] = value
	p.mu.Unlock()
	return nil
}
func (p *Process) Unsetenv(name string) error {
	if p == nil {
		return os.Unsetenv(name)
	}
	p.mu.Lock()
	delete(p.env, envKey(name))
	p.mu.Unlock()
	return nil
}
func (p *Process) Environ() []string {
	if p == nil {
		return os.Environ()
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, 0, len(p.env))
	for k, v := range p.env {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
func (p *Process) GetHome(name string) (string, error) {
	if p == nil {
		return getHome(name)
	}
	if name == "" {
		for _, key := range []string{"HOME", "USERPROFILE"} {
			if home := p.Getenv(key); home != "" {
				return p.Path(home), nil
			}
		}
		return "", errors.New("home directory is not set")
	}
	u, err := user.Lookup(name)
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}
func (p *Process) LookPath(name string) (string, error) {
	if p == nil {
		return fsutil.LookPath(name)
	}
	if fsutil.IsSyntheticPath(name) {
		return name, nil
	}
	check := func(path string) string {
		paths := []string{path}
		if runtime.GOOS == "windows" && filepath.Ext(path) == "" {
			exts := p.Getenv("PATHEXT")
			if exts == "" {
				exts = ".COM;.EXE;.BAT;.CMD"
			}
			for _, ext := range strings.Split(exts, ";") {
				if ext != "" {
					paths = append(paths, path+ext)
				}
			}
		}
		for _, candidate := range paths {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0111 != 0) {
				return candidate
			}
		}
		return ""
	}
	if fsutil.DontSearch(name) || filepath.VolumeName(name) != "" {
		if path := check(p.Path(name)); path != "" {
			return path, nil
		}
		return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
	for _, dir := range filepath.SplitList(p.Getenv("PATH")) {
		if dir == fsutil.SyntheticPath() {
			continue
		}
		base, _ := p.Abs(dir)
		if path := check(filepath.Join(base, name)); path != "" {
			if runtime.GOOS == "windows" && name == "sort" && strings.EqualFold(filepath.Dir(path), filepath.Join(p.Getenv("SystemRoot"), "System32")) {
				break
			}
			return path, nil
		}
	}
	for _, fallback := range fsutil.SyntheticCommands {
		if fallback == name {
			return filepath.Join(fsutil.SyntheticPath(), name), nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// NewEvalerWithProcess creates an evaluator with session-local process state.
func NewEvalerWithProcess(p *Process) *Evaler {
	ev := NewEvaler()
	ev.Process = p
	ev.ExtendBuiltin(BuildNs().AddVar("paths", vars.NewEnvListVarWithEnv("PATH", p)))
	if p != nil {
		ev.ExtendBuiltin(BuildNs().AddVar("pid", vars.NewReadOnly(p.ID)).AddGoFns(map[string]any{"tilde-abbr": p.TildeAbbr}))
	}
	if p != nil {
		unavailable := func() error { return errors.New("process-wide debugging is unavailable in a virtual shell") }
		ev.ExtendBuiltin(BuildNs().AddGoFns(map[string]any{
			"-gc": unavailable, "-stack": unavailable,
			"-log": func(string) error { return unavailable() },
		}))
	}
	ev.AddModule("builtin", ev.Builtin())
	return ev
}

func (p *Process) TildeAbbr(path string) string {
	if p == nil {
		return fsutil.TildeAbbr(path)
	}
	home, err := p.GetHome("")
	if err != nil {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

func (p *Process) EachExternal(f func(string)) {
	if p == nil {
		fsutil.EachExternal(f)
		return
	}
	for _, dir := range filepath.SplitList(p.Getenv("PATH")) {
		base, _ := p.Abs(dir)
		entries, _ := os.ReadDir(base)
		for _, entry := range entries {
			if info, err := entry.Info(); err == nil && fsutil.IsExecutable(info) {
				f(entry.Name())
			}
		}
	}
	for _, name := range fsutil.SyntheticCommands {
		f(name)
	}
}
