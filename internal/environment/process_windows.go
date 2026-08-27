//go:build windows

package environment

import (
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

const createNewProcessGroup = 0x00000200
const createNewConsole = 0x00000010
const createSuspended = 0x00000004

func prepareCommand(command *exec.Cmd) {
	// Keep the process suspended until attachProcessTree has placed it in the
	// kill-on-close Job Object. This closes the Start-to-Assign window in which
	// a provider could create a child outside the tree we later terminate.
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | createSuspended}
}

func prepareDetachedCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | createNewConsole}
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakJobMemoryUsed     uintptr
	PeakProcessMemoryUsed uintptr
}

const (
	jobObjectExtendedLimitInformationClass = 9
	jobObjectLimitKillOnJobClose           = 0x2000
	processSetQuota                        = 0x0100
	processTerminate                       = 0x0001
	threadSuspendResume                    = 0x0002
	threadSnapshot                         = 0x00000004
)

type threadEntry32 struct {
	Size           uint32
	Usage          uint32
	ThreadID       uint32
	OwnerProcessID uint32
	BasePriority   int32
	DeltaPriority  int32
	Flags          uint32
}

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createJobObject          = kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	terminateJobObject       = kernel32.NewProc("TerminateJobObject")
	createToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	thread32First            = kernel32.NewProc("Thread32First")
	thread32Next             = kernel32.NewProc("Thread32Next")
	openThread               = kernel32.NewProc("OpenThread")
	resumeThread             = kernel32.NewProc("ResumeThread")
)

func attachProcessTree(command *exec.Cmd) (func(), func() error, error) {
	fallback := func() error { return terminateProcessTree(command) }
	jobHandle, _, callErr := createJobObject.Call(0, 0)
	if jobHandle == 0 {
		return func() {}, fallback, windowsCallError(callErr, "create job object failed")
	}
	attached := false
	defer func() {
		if !attached {
			// Closing a configured kill-on-close Job Object also terminates any
			// process that was assigned before a later setup step failed.
			_ = syscall.CloseHandle(syscall.Handle(jobHandle))
		}
	}()
	info := jobObjectExtendedLimitInformation{BasicLimitInformation: jobObjectBasicLimitInformation{LimitFlags: jobObjectLimitKillOnJobClose}}
	if result, _, err := setInformationJobObject.Call(jobHandle, jobObjectExtendedLimitInformationClass, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); result == 0 {
		return func() {}, fallback, windowsCallError(err, "configure job object failed")
	}
	processHandle, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(command.Process.Pid))
	if err != nil {
		return func() {}, fallback, err
	}
	if result, _, callErr := assignProcessToJobObject.Call(jobHandle, uintptr(processHandle)); result == 0 {
		_ = syscall.CloseHandle(processHandle)
		return func() {}, fallback, windowsCallError(callErr, "assign process to job object failed")
	}
	_ = syscall.CloseHandle(processHandle)
	if err := resumeSuspendedProcess(uint32(command.Process.Pid)); err != nil {
		return func() {}, fallback, err
	}
	attached = true
	var closeOnce sync.Once
	cleanup := func() { closeOnce.Do(func() { _ = syscall.CloseHandle(syscall.Handle(jobHandle)) }) }
	cancel := func() error {
		result, _, err := terminateJobObject.Call(jobHandle, 1)
		if result == 0 {
			return windowsCallError(err, "terminate job object failed")
		}
		return nil
	}
	return cleanup, cancel, nil
}

// resumeSuspendedProcess resumes the primary thread created with
// CREATE_SUSPENDED. os/exec intentionally closes that thread handle before it
// returns, so enumerate the still-suspended process's threads and resume the
// first one owned by the process. No user code can run before this point, so
// the process has only its primary thread and cannot create an escaping child.
func resumeSuspendedProcess(pid uint32) error {
	snapshot, _, callErr := createToolhelp32Snapshot.Call(threadSnapshot, 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return windowsCallError(callErr, "create thread snapshot failed")
	}
	defer func() { _ = syscall.CloseHandle(syscall.Handle(snapshot)) }()

	entry := threadEntry32{Size: uint32(unsafe.Sizeof(threadEntry32{}))}
	result, _, callErr := thread32First.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for result != 0 {
		if entry.OwnerProcessID == pid {
			threadHandle, _, openErr := openThread.Call(threadSuspendResume, 0, uintptr(entry.ThreadID))
			if threadHandle == 0 {
				return windowsCallError(openErr, "open suspended process thread failed")
			}
			defer func() { _ = syscall.CloseHandle(syscall.Handle(threadHandle)) }()
			previousCount, _, resumeErr := resumeThread.Call(threadHandle)
			if previousCount == uintptr(^uint32(0)) {
				return windowsCallError(resumeErr, "resume thread failed")
			}
			return nil
		}
		result, _, callErr = thread32Next.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	if callErr == nil {
		return errors.New("suspended process thread was not found")
	}
	return callErr
}

func windowsCallError(err error, fallback string) error {
	if err != nil {
		return err
	}
	return errors.New(fallback)
}

func terminateProcessTree(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	return command.Process.Kill()
}
