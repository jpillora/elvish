package eval

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PrepareVirtualChild starts the child suspended so it cannot spawn descendants
// before OwnVirtualChild places it into its own kill-on-close job object.
func PrepareVirtualChild(cmd *exec.Cmd, terminal bool) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if !terminal {
		cmd.SysProcAttr.CreationFlags |= detachedProcess
	}
}

// OwnVirtualChild assigns a suspended child to a job and resumes it. Releasing
// the job kills the entire descendant tree, without signaling the Go host.
func OwnVirtualChild(p *os.Process) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (func(), error) { windows.CloseHandle(job); _ = p.Kill(); return nil, err }
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return fail(err)
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(p.Pid))
	if err != nil {
		return fail(err)
	}
	defer windows.CloseHandle(handle)
	if err := windows.AssignProcessToJobObject(job, handle); err != nil {
		return fail(err)
	}
	status, _, _ := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess").Call(uintptr(handle))
	if status != 0 {
		return fail(fmt.Errorf("resume child: NTSTATUS %#x", status))
	}
	var once sync.Once
	return func() { once.Do(func() { _ = windows.CloseHandle(job) }) }, nil
}
