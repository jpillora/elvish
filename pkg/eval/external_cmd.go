package eval

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jpillora/elvish/pkg/eval/errs"
	"github.com/jpillora/elvish/pkg/eval/vals"
	"github.com/jpillora/elvish/pkg/fsutil"
	"github.com/jpillora/elvish/pkg/parse"
	"github.com/jpillora/elvish/pkg/persistent/hash"
)

var (
	// ErrExternalCmdOpts is thrown when an external command is passed Elvish
	// options.
	//
	// TODO: Catch this kind of errors at compilation time.
	ErrExternalCmdOpts = errors.New("external commands don't accept elvish options")
	// ErrImplicitCdNoArg is thrown when an implicit cd form is passed arguments.
	ErrImplicitCdNoArg = errors.New("implicit cd accepts no arguments")
)

// externalCmd is an external command.
type externalCmd struct {
	Name string
}

// NewExternalCmd returns a callable that executes the named external command.
//
// An external command converts all arguments to strings, and does not accept
// any option.
func NewExternalCmd(name string) Callable {
	return externalCmd{name}
}

func (e externalCmd) Kind() string {
	return "fn"
}

func (e externalCmd) Equal(a any) bool {
	return e == a
}

func (e externalCmd) Hash() uint32 {
	return hash.String(e.Name)
}

func (e externalCmd) Repr(int) string {
	return "<external " + parse.Quote(e.Name) + ">"
}

// Call calls an external command.
func (e externalCmd) Call(fm *Frame, argVals []any, opts map[string]any) error {
	if len(opts) > 0 {
		return ErrExternalCmdOpts
	}
	if fsutil.DontSearch(e.Name) {
		stat, err := os.Stat(fm.Evaler.Process.Path(e.Name))
		if err == nil && stat.IsDir() {
			// implicit cd
			if len(argVals) > 0 {
				return ErrImplicitCdNoArg
			}
			fm.Deprecate("implicit cd is deprecated; use cd or location mode instead", fm.traceback.Head, 21)
			return fm.Evaler.Chdir(e.Name)
		}
	}

	files := make([]*os.File, len(fm.ports))
	for i, port := range fm.ports {
		if port != nil {
			files[i] = port.File
		}
	}

	args := make([]string, len(argVals)+1)
	for i, a := range argVals {
		// TODO: Maybe we should enforce string arguments instead of coercing
		// all args to strings.
		args[i+1] = vals.ToString(a)
	}

	path, err := fm.Evaler.Process.LookPath(e.Name)
	if err != nil {
		return err
	}
	if fsutil.IsSyntheticPath(path) {
		return runSyntheticCommand(fm, filepath.Base(path), args[1:])
	}

	if runtime.GOOS == "windows" && !filepath.IsAbs(path) {
		// For some reason, Windows's CreateProcess API doesn't like forward
		// slashes in relative paths: ".\foo.bat" works but "./foo.bat" results
		// in an error message that "'.' is not recognized as an internal or
		// external command, operable program or batch file."
		//
		// There seems to be no good reason for this behavior, so we work around
		// it by replacing forward slashes with backslashes. PowerShell seems to
		// be something similar to support "./foo.bat".
		path = strings.ReplaceAll(path, "/", "\\")
	}

	args[0] = path
	if p := fm.Evaler.Process; p != nil {
		dir, _ := p.Getwd()
		cmd := exec.Command(path, args[1:]...)
		cmd.Dir, cmd.Env = dir, p.Environ()
		cmd.Stdin, cmd.Stdout, cmd.Stderr = fm.ByteInput(), fm.ByteOutput(), fm.ErrorOutput()
		terminal := len(p.TerminalPorts) == 3 && len(fm.ports) == 3
		if terminal {
			for i := range fm.ports {
				if fm.ports[i] != p.TerminalPorts[i] {
					terminal = false
				}
			}
		}
		run := p.RunExternal
		if run == nil {
			run = RunVirtualChild
		}
		state, err := run(fm.Context(), cmd, terminal)
		if fm.Canceled() {
			return ErrInterrupted
		}
		if err != nil {
			return err
		}
		return NewExternalCmdExit(e.Name, state.Sys().(syscall.WaitStatus), state.Pid())
	}

	sys := makeSysProcAttr(fm.background)
	proc, err := os.StartProcess(path, args, &os.ProcAttr{Files: files, Sys: sys})
	if err != nil {
		return err
	}

	state, err := proc.Wait()
	if err != nil {
		// This should be a can't happen situation. Nonetheless, treat it as a
		// soft error rather than panicking since the Go documentation is not
		// explicit that this can only happen if we make a mistake. Such as
		// calling `Wait` twice on a particular process object.
		return err
	}
	ws := state.Sys().(syscall.WaitStatus)
	if ws.Signaled() && isSIGPIPE(ws.Signal()) {
		readerGone := fm.ports[1].readerGone
		if readerGone != nil && readerGone.Load() {
			return errs.ReaderGone{}
		}
	}
	return NewExternalCmdExit(e.Name, state.Sys().(syscall.WaitStatus), proc.Pid)
}

// ContextReader can stop a pending read without closing a shared session stream.
type ContextReader interface {
	ReadContext(context.Context, []byte) (int, error)
}

type contextInput struct {
	ctx    context.Context
	reader ContextReader
}

func (r contextInput) Read(b []byte) (int, error) { return r.reader.ReadContext(r.ctx, b) }
func (r contextInput) ReadContext(ctx context.Context, b []byte) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	defer cancel()
	return r.reader.ReadContext(ctx, b)
}

// RunVirtualChild runs a real child with explicit session environment and cwd.
func RunVirtualChild(ctx context.Context, cmd *exec.Cmd, _ bool) (*os.ProcessState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	PrepareVirtualChild(cmd, false)
	cmd.WaitDelay = 200 * time.Millisecond
	var stdin io.ReadCloser
	var pumpDone chan struct{}
	var pipe io.WriteCloser
	if input, ok := cmd.Stdin.(ContextReader); ok {
		cmd.Stdin = nil
		var err error
		pipe, err = cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdin = io.NopCloser(contextInput{ctx, input})
	}
	if err := cmd.Start(); err != nil {
		if pipe != nil {
			pipe.Close()
		}
		return nil, err
	}
	stopChild, err := OwnVirtualChild(cmd.Process)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if pipe != nil {
			pipe.Close()
		}
		return nil, err
	}
	defer stopChild()
	if stdin != nil {
		pumpDone = make(chan struct{})
		go func() { defer close(pumpDone); io.Copy(pipe, stdin); pipe.Close() }()
	}
	killed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { stopChild(); close(killed) })
	err = cmd.Wait()
	if !stop() {
		<-killed
	}
	cancel()
	if pipe != nil {
		pipe.Close()
		<-pumpDone
	}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) && !errors.Is(err, exec.ErrWaitDelay) {
		return cmd.ProcessState, err
	}
	return cmd.ProcessState, nil
}

func virtualExec(fm *Frame, args ...any) error {
	if len(args) == 0 {
		return errs.ArityMismatch{What: "arguments", ValidLow: 1, ValidHigh: -1, Actual: 0}
	}
	err := NewExternalCmd(vals.ToString(args[0])).Call(fm, args[1:], NoOpts)
	if err != nil {
		if status, ok := err.(ExternalCmdExit); ok {
			return fm.Evaler.Process.Exit(status.WaitStatus.ExitStatus())
		}
		return err
	}
	return fm.Evaler.Process.Exit(0)
}
