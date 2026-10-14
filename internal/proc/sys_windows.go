//go:build windows

package proc

import (
	"os/exec"
	"time"
)

func configureChild(cmd *exec.Cmd) {}

func killProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Kill()
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(shutdownWait):
		return nil
	}
}
