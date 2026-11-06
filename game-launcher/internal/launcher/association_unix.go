//go:build !windows

package launcher

import (
	"os/exec"
	"runtime"
)

func openAssociated(path, folder string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	cmd := exec.Command(opener, path)
	cmd.Dir = folder
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
