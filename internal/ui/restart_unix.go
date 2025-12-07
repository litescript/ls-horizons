//go:build !windows

package ui

import (
	"os"
	"os/exec"
	"syscall"
)

// RestartPending indicates the app should restart after quitting.
var RestartPending bool

// DoRestart replaces the current process with a fresh instance.
// On Unix, this uses syscall.Exec for seamless restart.
// Call this from main() after Bubble Tea has exited.
func DoRestart() error {
	if !RestartPending {
		return nil
	}

	// Find the binary path
	binary, err := exec.LookPath("ls-horizons")
	if err != nil {
		// Try the current executable as fallback
		binary, err = os.Executable()
		if err != nil {
			return err
		}
	}

	// Replace current process with new instance
	// This preserves the terminal, PID, and args
	return syscall.Exec(binary, os.Args, os.Environ())
}
