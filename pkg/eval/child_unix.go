//go:build unix

package eval

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// PrepareVirtualChild isolates a child program from the embedding process.
// For terminal children the PTY backend creates the process group/session.
func PrepareVirtualChild(cmd *exec.Cmd, terminal bool) {
	if !terminal {
		cmd.SysProcAttr = makeSysProcAttr(true)
	}
}

// OwnVirtualChild returns a release function that terminates the child's group.
// Call it on cancellation and on completion to clean up remaining descendants.
func OwnVirtualChild(p *os.Process) (func(), error) {
	var once sync.Once
	return func() { once.Do(func() { _ = syscall.Kill(-p.Pid, syscall.SIGKILL) }) }, nil
}
