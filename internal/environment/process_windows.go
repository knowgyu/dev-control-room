//go:build windows

package environment

import (
	"os/exec"
	"syscall"
	"unsafe"
)

const createNewProcessGroup = 0x00000200
const createNewConsole = 0x00000010

func prepareCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
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
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createJobObject          = kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	terminateJobObject       = kernel32.NewProc("TerminateJobObject")
)

func attachProcessTree(command *exec.Cmd) (func(), func() error, error) {
	jobHandle, _, callErr := createJobObject.Call(0, 0)
	if jobHandle == 0 {
		return func() {}, func() error { return terminateProcessTree(command) }, callErr
	}
	info := jobObjectExtendedLimitInformation{BasicLimitInformation: jobObjectBasicLimitInformation{LimitFlags: jobObjectLimitKillOnJobClose}}
	if result, _, err := setInformationJobObject.Call(jobHandle, jobObjectExtendedLimitInformationClass, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); result == 0 {
		_ = syscall.CloseHandle(syscall.Handle(jobHandle))
		return func() {}, func() error { return terminateProcessTree(command) }, err
	}
	processHandle, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(command.Process.Pid))
	if err != nil {
		_ = syscall.CloseHandle(syscall.Handle(jobHandle))
		return func() {}, func() error { return terminateProcessTree(command) }, err
	}
	if result, _, callErr := assignProcessToJobObject.Call(jobHandle, uintptr(processHandle)); result == 0 {
		_ = syscall.CloseHandle(processHandle)
		_ = syscall.CloseHandle(syscall.Handle(jobHandle))
		return func() {}, func() error { return terminateProcessTree(command) }, callErr
	}
	_ = syscall.CloseHandle(processHandle)
	cleanup := func() { _ = syscall.CloseHandle(syscall.Handle(jobHandle)) }
	cancel := func() error {
		result, _, err := terminateJobObject.Call(jobHandle, 1)
		if result == 0 {
			return err
		}
		return nil
	}
	return cleanup, cancel, nil
}

func terminateProcessTree(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	return command.Process.Kill()
}
