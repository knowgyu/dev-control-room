//go:build !windows

package environment

import (
	"os/exec"
	"syscall"
)

func prepareCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func prepareDetachedCommand(command *exec.Cmd) {
	prepareCommand(command)
}

func attachProcessTree(command *exec.Cmd) (func(), func() error, error) {
	return func() {}, func() error { return terminateProcessTree(command) }, nil
}

func terminateProcessTree(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return command.Process.Kill()
}
