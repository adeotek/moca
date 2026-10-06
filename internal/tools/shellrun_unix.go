//go:build !windows

package tools

import (
	"os/exec"
	"syscall"
)

func shellCommand(command string) (string, []string, error) {
	return "bash", []string{"-c", command}, nil
}

// SetProcessGroup puts cmd in its own process group so KillProcessGroup can
// take down the command with its descendants (shared with the MCP stdio
// transport).
func SetProcessGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
