package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jpillora/elvish/pkg/diag"
	"github.com/jpillora/elvish/pkg/edit"
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/mods"
	"github.com/jpillora/elvish/pkg/parse"
)

// ContextWriter is an optional capability for a session's output streams.
// WriteContext must return promptly when ctx is canceled. Interrupting a
// command cancels its pending writes without closing the caller's stream.
type ContextWriter interface {
	io.Writer
	WriteContext(context.Context, []byte) (int, error)
}

// SessionConfig describes an in-process shell. Env is an explicit snapshot;
// nil means an empty environment. The caller owns the output streams.
type SessionConfig struct {
	Context        context.Context
	Dir            string
	Env            []string
	Stdout, Stderr io.Writer
	Terminal       bool
	Cols, Rows     int
	// RunExternal runs real child programs. terminal is true only when all
	// original terminal ports are inherited. It must honor ctx and reap the child.
	RunExternal func(context.Context, *exec.Cmd, bool) (*os.ProcessState, error)
	// ResizeExternal resizes the current child terminal, if any.
	ResizeExternal func(cols, rows int)
}

// Session is a virtual root process. Write feeds stdin; CloseInput sends EOF;
// Close cancels the session. Run is called exactly once. No package initialization,
// process-wide signals, cwd/environment changes or shell subprocess are needed.
// This is state isolation, not a filesystem or privilege sandbox.
type Session struct {
	Process       *eval.Process
	cfg           SessionConfig
	ev            *eval.Evaler
	out, errOut   *sessionOutput
	started       atomic.Bool
	mu            sync.Mutex
	changed       chan struct{}
	input         []byte
	eof           bool
	commandCancel context.CancelFunc
	outputCtx     context.Context
	outputCancel  context.CancelFunc
	commandRaw    bool
	line          []byte
	lineReady     bool
	lineEOF       bool
	cols, rows    int
	tty           *streamTTY
}

func NewSession(cfg SessionConfig) (*Session, error) {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	p, err := eval.NewProcess(cfg.Context, cfg.Dir, cfg.Env)
	if err != nil {
		return nil, err
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = cfg.Stdout
	}
	if cfg.Cols <= 0 {
		cfg.Cols = 80
	}
	if cfg.Rows <= 0 {
		cfg.Rows = 24
	}
	outputMu := new(sync.Mutex)
	s := &Session{Process: p, cfg: cfg, changed: make(chan struct{}), cols: cfg.Cols, rows: cfg.Rows}
	s.outputCtx, s.outputCancel = context.WithCancel(cfg.Context)
	s.out = &sessionOutput{session: s, mu: outputMu, writer: cfg.Stdout, terminal: cfg.Terminal}
	s.errOut = &sessionOutput{session: s, mu: outputMu, writer: cfg.Stderr, terminal: cfg.Terminal}
	s.ev = eval.NewEvalerWithProcess(p)
	s.configurePaths()
	mods.AddTo(s.ev)
	p.RunExternal = s.runExternal
	p.TerminalSize = s.Size
	level, _ := strconv.Atoi(p.Getenv("SHLVL"))
	_ = p.Setenv("SHLVL", strconv.Itoa(level+1))
	s.tty = newStreamTTY(s)
	return s, nil
}

// Close stops this virtual root and its child commands, without closing output.
func (s *Session) Close() error {
	s.Process.Close()
	s.mu.Lock()
	s.outputCancel()
	s.mu.Unlock()
	return nil
}

func (s *Session) Size() (rows, cols int) { s.mu.Lock(); defer s.mu.Unlock(); return s.rows, s.cols }
func (s *Session) Resize(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	s.mu.Lock()
	s.cols, s.rows = cols, rows
	s.mu.Unlock()
	s.tty.resize()
	if s.cfg.ResizeExternal != nil {
		s.cfg.ResizeExternal(cols, rows)
	}
}

// Interrupt cancels only the active command, or clears the editor's input.
func (s *Session) Interrupt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.commandCancel != nil {
		s.input = nil
		s.cancelCommand()
	} else {
		s.input = append(s.input, 3)
	}
	s.wake()
}

// cancelCommand is called with mu held. Keep the output context canceled
// until the evaluation unwinds, including writes that start after Ctrl-C.
func (s *Session) cancelCommand() {
	s.commandCancel()
	s.outputCancel()
}

// Run evaluates command, or runs an interactive editor when command is nil.
// It drains output and cancels background work before returning the exit status.
func (s *Session) Run(command *string) int {
	if !s.started.CompareAndSwap(false, true) {
		return 1
	}
	ports, cleanup := eval.PortsFromStreams(s, s.out, s.errOut, s.ev.ValuePrefix())
	s.Process.DefaultPorts = ports
	if s.cfg.Terminal {
		s.Process.TerminalPorts = ports
	}
	defer func() { s.Process.Close(); s.Process.Wait(); cleanup(); s.Close() }()
	if command != nil {
		return s.evaluate(*command, "[command]", ports, nil)
	}
	if !s.cfg.Terminal {
		// A shell request without a terminal reads one script from stdin.
		code, err := io.ReadAll(s)
		if err != nil {
			return 1
		}
		return s.evaluate(string(code), "[stdin]", ports, nil)
	}
	ed := edit.NewEditor(s.tty, s.ev, nil)
	s.ev.AddModule("edit", ed.Ns())
	s.ev.BgJobNotify = func(msg string) { fmt.Fprintln(s.errOut, msg) }
	if rc := s.ev.RcPath; rc != "" {
		code, err := os.ReadFile(s.Process.Path(rc))
		if err == nil {
			s.ev.EffectiveRcPath = rc
			s.evaluate(string(code), rc, ports, ed)
		} else if !errors.Is(err, os.ErrNotExist) {
			diag.ShowError(s.errOut, err)
		}
	}
	for s.Process.Context().Err() == nil {
		code, err := ed.ReadCode()
		if err != nil {
			if !errors.Is(err, io.EOF) && s.Process.Context().Err() == nil {
				diag.ShowError(s.errOut, err)
				return 1
			}
			break
		}
		s.evaluate(code, "[interactive]", ports, ed)
	}
	if code, exited := s.Process.ExitStatus(); exited {
		return code
	}
	return 0
}

func (s *Session) evaluate(code, name string, ports []*eval.Port, ed *edit.Editor) int {
	ctx, cancel := context.WithCancel(s.Process.Context())
	s.mu.Lock()
	s.commandCancel = cancel
	s.commandRaw = false
	s.line = nil
	s.lineReady = false
	s.lineEOF = false
	// Ctrl+C may already be queued behind Enter when the editor hands off.
	if s.cfg.Terminal {
		s.interceptInterrupt()
	}
	s.mu.Unlock()
	src := parse.Source{Name: name, Code: code}
	start := time.Now()
	err := s.ev.Eval(src, eval.EvalCfg{Ports: ports, Interrupts: ctx})
	interrupted := ctx.Err() != nil
	s.mu.Lock()
	s.commandCancel = nil
	if s.outputCtx.Err() != nil && s.Process.Context().Err() == nil {
		s.outputCtx, s.outputCancel = context.WithCancel(s.cfg.Context)
	}
	s.commandRaw = false
	s.line = nil
	s.lineReady = false
	s.lineEOF = false
	s.mu.Unlock()
	cancel()
	if status, exited := s.Process.ExitStatus(); exited {
		return status
	}
	if ed != nil && s.Process.Context().Err() == nil {
		ed.RunAfterCommandHooks(src, time.Since(start).Seconds(), err)
	}
	if err != nil {
		if interrupted {
			if s.cfg.Terminal {
				fmt.Fprintln(s.errOut, "^C")
			}
			return 130
		}
		diag.ShowError(s.errOut, err)
		if exc, ok := err.(eval.Exception); ok {
			if status, ok := exc.Reason().(eval.ExternalCmdExit); ok {
				if code := status.WaitStatus.ExitStatus(); code >= 0 {
					return code
				}
			}
		}
		return 1
	}
	return 0
}

func (s *Session) runExternal(ctx context.Context, cmd *exec.Cmd, terminal bool) (*os.ProcessState, error) {
	if terminal {
		s.mu.Lock()
		s.commandRaw = true
		s.mu.Unlock()
		defer func() { s.mu.Lock(); s.commandRaw = false; s.mu.Unlock() }()
	}
	run := s.cfg.RunExternal
	if run == nil {
		run = eval.RunVirtualChild
	}
	return run(ctx, cmd, terminal)
}

func (s *Session) configurePaths() {
	p := s.Process
	home, _ := p.GetHome("")
	config := p.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		if runtime.GOOS == "windows" {
			config = p.Getenv("APPDATA")
		} else if home != "" {
			config = filepath.Join(home, ".config")
		}
	}
	if config != "" {
		s.ev.RcPath = filepath.Join(p.Path(config), "elvish", "rc.elv")
		s.ev.LibDirs = append(s.ev.LibDirs, filepath.Join(p.Path(config), "elvish", "lib"))
	}
	data := p.Getenv("XDG_DATA_HOME")
	if data == "" {
		if runtime.GOOS == "windows" {
			data = p.Getenv("LOCALAPPDATA")
		} else if home != "" {
			data = filepath.Join(home, ".local", "share")
		}
	}
	if data != "" {
		s.ev.LibDirs = append(s.ev.LibDirs, filepath.Join(p.Path(data), "elvish", "lib"))
	}
	if dirs := p.Getenv("XDG_DATA_DIRS"); dirs != "" {
		for _, dir := range filepath.SplitList(dirs) {
			s.ev.LibDirs = append(s.ev.LibDirs, filepath.Join(p.Path(dir), "elvish", "lib"))
		}
	} else {
		s.ev.LibDirs = append(s.ev.LibDirs, defaultDataDirs...)
	}
}

// sessionOutput serializes writes and supplies terminal OPOST/ONLCR behavior
// without an OS terminal driver. Existing CRLF output is left intact.
type sessionOutput struct {
	session  *Session
	mu       *sync.Mutex
	writer   io.Writer
	terminal bool
	lastCR   bool
}

func (w *sessionOutput) Write(b []byte) (int, error) {
	write := w.writer.Write
	if writer, ok := w.writer.(ContextWriter); ok {
		w.session.mu.Lock()
		ctx := w.session.outputCtx
		w.session.mu.Unlock()
		write = func(p []byte) (int, error) { return writer.WriteContext(ctx, p) }
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.terminal {
		return write(b)
	}
	out := make([]byte, 0, len(b))
	lastCR := w.lastCR
	for _, c := range b {
		if c == '\n' && !lastCR {
			out = append(out, '\r')
		}
		out = append(out, c)
		lastCR = c == '\r'
	}
	n, err := write(out)
	if n > 0 {
		w.lastCR = out[n-1] == '\r'
	}
	if err == nil && n != len(out) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return 0, err
	}
	return len(b), nil
}
