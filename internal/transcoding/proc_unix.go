//go:build !windows

package transcoding

import (
	"errors"
	"log/slog"
	"os/exec"
	"syscall"
)

func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func setLowPriority(pid int) error {
	return syscall.Setpriority(syscall.PRIO_PROCESS, pid, 10)
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		slog.Warn("failed to kill ffmpeg process group", "pid", cmd.Process.Pid, "error", err)
	}
}

func isCrossDeviceError(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}
