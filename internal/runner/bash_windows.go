//go:build windows

package runner

import (
	"os/exec"
	"strconv"
)

func configureBashProcess(command *exec.Cmd) {
	command.Cancel = func() error {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
		if err := kill.Run(); err != nil {
			return command.Process.Kill()
		}
		return nil
	}
}
