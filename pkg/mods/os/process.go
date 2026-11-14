package os

import (
	"github.com/jpillora/elvish/pkg/eval"
	"github.com/jpillora/elvish/pkg/eval/vals"
	"os"
	"path/filepath"
	"runtime"
)

func NsForProcess(p *eval.Process) *eval.Ns {
	if p == nil {
		return Ns
	}
	tempOpts := func(opts mktempOpt) mktempOpt {
		if opts.Dir != "" {
			opts.Dir = p.Path(opts.Dir)
		} else {
			for _, key := range []string{"TMPDIR", "TEMP", "TMP"} {
				if dir := p.Getenv(key); dir != "" {
					opts.Dir = p.Path(dir)
					break
				}
			}
		}
		if opts.Dir == "" {
			if runtime.GOOS == "windows" {
				opts.Dir, _ = p.Getwd()
			} else {
				opts.Dir = "/tmp"
			}
		}
		return opts
	}
	return eval.CombineNs(Ns, eval.BuildNsNamed("os").AddGoFns(map[string]any{
		"mkdir":         func(opts mkdirOpts, path string) error { return mkdir(opts, p.Path(path)) },
		"mkdir-all":     func(opts mkdirOpts, path string) error { return mkdirAll(opts, p.Path(path)) },
		"symlink":       func(target, link string) error { return os.Symlink(target, p.Path(link)) },
		"remove":        func(path string) error { return remove(p.Path(path)) },
		"remove-all":    func(path string) error { return removeAll(p.Path(path)) },
		"rename":        func(old, new string) error { return os.Rename(p.Path(old), p.Path(new)) },
		"chmod":         func(opts chmodOpts, perm int, path string) error { return chmod(opts, perm, p.Path(path)) },
		"stat":          func(opts statOpts, path string) (vals.Map, error) { return stat(opts, p.Path(path)) },
		"exists":        func(opts statOpts, path string) bool { return exists(opts, p.Path(path)) },
		"is-dir":        func(opts statOpts, path string) bool { return IsDir(opts, p.Path(path)) },
		"is-regular":    func(opts statOpts, path string) bool { return IsRegular(opts, p.Path(path)) },
		"eval-symlinks": func(path string) (string, error) { return filepath.EvalSymlinks(p.Path(path)) },
		"temp-dir":      func(opts mktempOpt, args ...string) (string, error) { return TempDir(tempOpts(opts), args...) },
		"temp-file":     func(opts mktempOpt, args ...string) (*os.File, error) { return TempFile(tempOpts(opts), args...) },
	}).Ns())
}
