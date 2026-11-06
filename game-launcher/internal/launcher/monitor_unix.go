//go:build !windows

package launcher

import (
	"os/exec"
	"syscall"
)

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func detachUtility(cmd *exec.Cmd) { detach(cmd) }
