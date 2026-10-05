//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

func shellCommand(command string) (string, []string, error) {
	return "bash", []string{"-c", command}, nil
}

func setProcessGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
