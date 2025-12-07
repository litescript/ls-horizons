//go:build windows

package ui

import (
	"fmt"
	"os"
)

// RestartPending indicates the app should restart after quitting.
var RestartPending bool

// DoRestart on Windows prints a message since seamless exec isn't available.
// Windows doesn't support replacing the current process like Unix does.
func DoRestart() error {
	if !RestartPending {
		return nil
	}

	// Windows doesn't have syscall.Exec equivalent
	// Just inform the user to restart manually
	fmt.Fprintln(os.Stderr, "\nUpdate installed. Please restart ls-horizons to use the new version.")
	return nil
}
