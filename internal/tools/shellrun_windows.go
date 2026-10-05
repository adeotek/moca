//go:build windows

package tools

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
)

func shellCommand(command string) (string, []string, error) {
	for _, sh := range []string{"pwsh", "powershell.exe"} {
		if p, err := exec.LookPath(sh); err == nil {
			return p, []string{"-NoProfile", "-Command", command}, nil
		}
	}
	return "", nil, errors.New("neither pwsh nor powershell.exe found on PATH")
}

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}
