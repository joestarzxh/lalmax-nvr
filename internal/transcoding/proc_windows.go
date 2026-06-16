//go:build windows

package transcoding

import (
	"log/slog"
	"os/exec"
	"strings"
	"syscall"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	setPriorityClass = kernel32.NewProc("SetPriorityClass")
)

const (
	belowNormalPriorityClass = 0x00004000
	processSetInformation    = 0x0200
)

func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func setLowPriority(pid int) error {
	handle, err := syscall.OpenProcess(processSetInformation, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)

	r, _, callErr := setPriorityClass.Call(uintptr(handle), uintptr(belowNormalPriorityClass))
	if r == 0 {
		return callErr
	}
	return nil
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if err := cmd.Process.Kill(); err != nil {
		slog.Warn("failed to kill ffmpeg process", "pid", cmd.Process.Pid, "error", err)
	}
}

func isCrossDeviceError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "different disk drive") ||
		strings.Contains(msg, "cross-device") ||
		strings.Contains(msg, "EXDEV")
}
