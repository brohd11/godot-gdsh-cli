//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setDetached starts cmd in a session of its own, so the terminal's Ctrl-C and hangup skip it.
func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
