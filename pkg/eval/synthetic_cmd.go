package eval

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runSyntheticCommand implements the virtual executables in fsutil's
// synthetic PATH directory. They write bytes to the same ports as external
// commands, but run in the shell process.
func runSyntheticCommand(fm *Frame, name string, args []string) error {
	switch name {
	case "ls", "dir":
		return fm.Evaler.Process.syntheticLs(fm.ByteOutput(), args)
	case "cp":
		return fm.Evaler.Process.syntheticCp(args)
	case "mv":
		return fm.Evaler.Process.syntheticMv(args)
	case "rm":
		return fm.Evaler.Process.syntheticRm(args)
	case "mkdir":
		return fm.Evaler.Process.syntheticMkdir(args)
	case "rmdir":
		return fm.Evaler.Process.syntheticRmdir(args)
	case "touch":
		return fm.Evaler.Process.syntheticTouch(args)
	case "cat":
		return fm.Evaler.Process.syntheticCat(fm.ByteInput(), fm.ByteOutput(), args)
	case "head", "tail":
		return fm.Evaler.Process.syntheticSliceText(name, fm.ByteInput(), fm.ByteOutput(), args)
	case "wc":
		return fm.Evaler.Process.syntheticWc(fm.ByteInput(), fm.ByteOutput(), args)
	case "sort":
		return fm.Evaler.Process.syntheticSort(fm.ByteInput(), fm.ByteOutput(), args)
	case "uniq":
		return fm.Evaler.Process.syntheticUniq(fm.ByteInput(), fm.ByteOutput(), args)
	case "grep":
		return fm.Evaler.Process.syntheticGrep(fm.ByteInput(), fm.ByteOutput(), fm.ErrorOutput(), args)
	case "pwd":
		if len(args) != 0 {
			return fmt.Errorf("pwd: unexpected arguments: %v", args)
		}
		wd, err := fm.Evaler.Process.Getwd()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(fm.ByteOutput(), wd)
		return err
	case "cd":
		return cd(fm, args...)
	default:
		return fmt.Errorf("unknown synthetic command: %s", name)
	}
}

func syntheticFlags(command string, args []string, allowed string) (map[rune]bool, []string, error) {
	flags := make(map[rune]bool)
	operands := make([]string, 0, len(args))
	parsing := true
	for _, arg := range args {
		if parsing && arg == "--" {
			parsing = false
			continue
		}
		if parsing && strings.HasPrefix(arg, "-") && arg != "-" {
			for _, flag := range arg[1:] {
				if !strings.ContainsRune(allowed, flag) {
					return nil, nil, fmt.Errorf("%s: unsupported option -%c", command, flag)
				}
				flags[flag] = true
			}
			continue
		}
		operands = append(operands, arg)
	}
	return flags, operands, nil
}

func (vproc *Process) syntheticLs(out io.Writer, args []string) error {
	flags, paths, err := syntheticFlags("ls", args, "aAld1")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	for i, path := range paths {
		info, err := os.Lstat(vproc.Path(path))
		if err != nil {
			return err
		}
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if len(paths) > 1 && info.IsDir() && !flags['d'] {
			if _, err := fmt.Fprintf(out, "%s:\n", path); err != nil {
				return err
			}
		}
		if !info.IsDir() || flags['d'] {
			if err := vproc.syntheticLsEntry(out, path, filepath.Base(path), info, flags['l']); err != nil {
				return err
			}
			continue
		}
		entries, err := os.ReadDir(vproc.Path(path))
		if err != nil {
			return err
		}
		if flags['a'] {
			for _, name := range []string{".", ".."} {
				stat, err := os.Lstat(vproc.Path(filepath.Join(path, name)))
				if err != nil {
					return err
				}
				if err := vproc.syntheticLsEntry(out, filepath.Join(path, name), name, stat, flags['l']); err != nil {
					return err
				}
			}
		}
		for _, entry := range entries {
			if !flags['a'] && !flags['A'] && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			stat, err := entry.Info()
			if err != nil {
				return err
			}
			if err := vproc.syntheticLsEntry(out, filepath.Join(path, entry.Name()), entry.Name(), stat, flags['l']); err != nil {
				return err
			}
		}
	}
	return nil
}

func (vproc *Process) syntheticLsEntry(out io.Writer, path, name string, info os.FileInfo, long bool) error {
	if long {
		if _, err := fmt.Fprintf(out, "%s %8d %s ", info.Mode(), info.Size(), info.ModTime().Format("Jan _2 15:04")); err != nil {
			return err
		}
	}
	if info.Mode()&os.ModeSymlink != 0 && long {
		target, err := os.Readlink(vproc.Path(path))
		if err == nil {
			name += " -> " + target
		}
	}
	_, err := fmt.Fprintln(out, name)
	return err
}

func (vproc *Process) syntheticCp(args []string) error {
	flags, paths, err := syntheticFlags("cp", args, "rR")
	if err != nil {
		return err
	}
	if len(paths) < 2 {
		return errors.New("cp: source and destination required")
	}
	dest := paths[len(paths)-1]
	sources := paths[:len(paths)-1]
	destInfo, destErr := os.Stat(vproc.Path(dest))
	if len(sources) > 1 && (destErr != nil || !destInfo.IsDir()) {
		return errors.New("cp: destination must be a directory for multiple sources")
	}
	for _, src := range sources {
		target := dest
		if destErr == nil && destInfo.IsDir() {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := vproc.syntheticCopyPath(src, target, flags['r'] || flags['R']); err != nil {
			return fmt.Errorf("cp: %w", err)
		}
	}
	return nil
}

func (vproc *Process) syntheticCopyPath(src, dest string, recursive bool) error {
	return vproc.syntheticCopyPathWithTimes(src, dest, recursive, false)
}

func (vproc *Process) syntheticCopyPathWithTimes(src, dest string, recursive, preserveTimes bool) (err error) {
	info, err := os.Lstat(vproc.Path(src))
	if err != nil {
		return err
	}
	// Rename preserves modification times; its cross-volume fallback must do
	// the same. Do not follow symlinks when restoring metadata.
	if preserveTimes && info.Mode()&os.ModeSymlink == 0 {
		defer func() {
			if err == nil {
				err = os.Chtimes(vproc.Path(dest), info.ModTime(), info.ModTime())
			}
		}()
	}
	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("%s is a directory (use -r)", src)
		}
		absSrc, err := vproc.Abs(src)
		if err != nil {
			return err
		}
		absDest, err := vproc.Abs(dest)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(absSrc, absDest)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("cannot copy %s into itself", src)
		}
		return vproc.syntheticCopyDir(src, dest, info.Mode().Perm(), preserveTimes)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(vproc.Path(src))
		if err != nil {
			return err
		}
		return os.Symlink(target, vproc.Path(dest))
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("cannot copy special file %s", src)
	}
	return vproc.syntheticCopyFile(src, dest, info.Mode().Perm())
}

func (vproc *Process) syntheticCopyDir(src, dest string, mode os.FileMode, preserveTimes bool) error {
	if err := os.Mkdir(vproc.Path(dest), mode|0700); err != nil && !os.IsExist(err) {
		return err
	}
	entries, err := os.ReadDir(vproc.Path(src))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := vproc.syntheticCopyPathWithTimes(filepath.Join(src, entry.Name()), filepath.Join(dest, entry.Name()), true, preserveTimes); err != nil {
			return err
		}
	}
	return os.Chmod(vproc.Path(dest), mode)
}

// syntheticCopyFile adapts CopyFile from github.com/gookit/goutil/fsutil,
// published under the MIT license. See SYNTHETIC_LICENSE.md.
func (vproc *Process) syntheticCopyFile(srcPath, dstPath string, mode os.FileMode) (err error) {
	srcFile, err := os.OpenFile(vproc.Path(srcPath), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	if dstInfo, err := os.Stat(vproc.Path(dstPath)); err == nil {
		if srcInfo, err := srcFile.Stat(); err == nil && os.SameFile(srcInfo, dstInfo) {
			return fmt.Errorf("%s and %s are the same file", srcPath, dstPath)
		}
	}
	dstFile, err := os.OpenFile(vproc.Path(dstPath), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dstFile.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (vproc *Process) syntheticMv(args []string) error {
	_, paths, err := syntheticFlags("mv", args, "")
	if err != nil {
		return err
	}
	if len(paths) < 2 {
		return errors.New("mv: source and destination required")
	}
	dest := paths[len(paths)-1]
	sources := paths[:len(paths)-1]
	destInfo, destErr := os.Stat(vproc.Path(dest))
	if len(sources) > 1 && (destErr != nil || !destInfo.IsDir()) {
		return errors.New("mv: destination must be a directory for multiple sources")
	}
	for _, src := range sources {
		target := dest
		if destErr == nil && destInfo.IsDir() {
			target = filepath.Join(dest, filepath.Base(src))
		}
		if err := os.Rename(vproc.Path(src), vproc.Path(target)); err != nil {
			// Rename can fail when moving between filesystems.
			var linkErr *os.LinkError
			if !errors.As(err, &linkErr) || !isSyntheticCrossDeviceError(linkErr.Err) {
				return fmt.Errorf("mv: %w", err)
			}
			if err := vproc.syntheticCopyPathWithTimes(src, target, true, true); err != nil {
				return fmt.Errorf("mv: %w", err)
			}
			if err := os.RemoveAll(vproc.Path(src)); err != nil {
				return fmt.Errorf("mv: %w", err)
			}
		}
	}
	return nil
}

func (vproc *Process) syntheticRm(args []string) error {
	flags, paths, err := syntheticFlags("rm", args, "rRf")
	if err != nil {
		return err
	}
	if len(paths) == 0 && !flags['f'] {
		return errors.New("rm: path required")
	}
	for _, path := range paths {
		clean := filepath.Clean(path)
		if clean == "." || clean == ".." || filepath.Dir(clean) == clean {
			return fmt.Errorf("rm: refusing to remove %s", path)
		}
		info, err := os.Lstat(vproc.Path(path))
		if err != nil {
			if flags['f'] && os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.IsDir() && !(flags['r'] || flags['R']) {
			return fmt.Errorf("rm: %s is a directory (use -r)", path)
		}
		if flags['r'] || flags['R'] {
			err = os.RemoveAll(vproc.Path(path))
		} else {
			err = os.Remove(vproc.Path(path))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (vproc *Process) syntheticMkdir(args []string) error {
	flags, paths, err := syntheticFlags("mkdir", args, "p")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("mkdir: path required")
	}
	for _, path := range paths {
		if flags['p'] {
			err = os.MkdirAll(vproc.Path(path), 0755)
		} else {
			err = os.Mkdir(vproc.Path(path), 0755)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (vproc *Process) syntheticRmdir(args []string) error {
	_, paths, err := syntheticFlags("rmdir", args, "")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("rmdir: path required")
	}
	for _, path := range paths {
		info, err := os.Lstat(vproc.Path(path))
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("rmdir: %s is not a directory", path)
		}
		if err := os.Remove(vproc.Path(path)); err != nil {
			return err
		}
	}
	return nil
}

func (vproc *Process) syntheticTouch(args []string) error {
	_, paths, err := syntheticFlags("touch", args, "")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("touch: path required")
	}
	for _, path := range paths {
		f, err := os.OpenFile(vproc.Path(path), os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		now := time.Now()
		if err := os.Chtimes(vproc.Path(path), now, now); err != nil {
			return err
		}
	}
	return nil
}

func (vproc *Process) syntheticCat(in io.Reader, out io.Writer, args []string) error {
	_, paths, err := syntheticFlags("cat", args, "")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	for _, path := range paths {
		if path == "-" {
			if _, err := io.Copy(out, in); err != nil {
				return err
			}
			continue
		}
		f, err := os.Open(vproc.Path(path))
		if err != nil {
			return err
		}
		_, err = io.Copy(out, f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func syntheticLs(out io.Writer, args []string) error {
	return (*Process)(nil).syntheticLs(out, args)
}

func syntheticLsEntry(out io.Writer, path, name string, info os.FileInfo, long bool) error {
	return (*Process)(nil).syntheticLsEntry(out, path, name, info, long)
}

func syntheticCp(args []string) error {
	return (*Process)(nil).syntheticCp(args)
}

func syntheticCopyPath(src, dest string, recursive bool) error {
	return (*Process)(nil).syntheticCopyPath(src, dest, recursive)
}

func syntheticCopyPathWithTimes(src, dest string, recursive, preserveTimes bool) (err error) {
	return (*Process)(nil).syntheticCopyPathWithTimes(src, dest, recursive, preserveTimes)
}

func syntheticCopyDir(src, dest string, mode os.FileMode, preserveTimes bool) error {
	return (*Process)(nil).syntheticCopyDir(src, dest, mode, preserveTimes)
}

func syntheticCopyFile(srcPath, dstPath string, mode os.FileMode) (err error) {
	return (*Process)(nil).syntheticCopyFile(srcPath, dstPath, mode)
}

func syntheticMv(args []string) error {
	return (*Process)(nil).syntheticMv(args)
}

func syntheticRm(args []string) error {
	return (*Process)(nil).syntheticRm(args)
}

func syntheticMkdir(args []string) error {
	return (*Process)(nil).syntheticMkdir(args)
}

func syntheticRmdir(args []string) error {
	return (*Process)(nil).syntheticRmdir(args)
}

func syntheticTouch(args []string) error {
	return (*Process)(nil).syntheticTouch(args)
}

func syntheticCat(in io.Reader, out io.Writer, args []string) error {
	return (*Process)(nil).syntheticCat(in, out, args)
}
