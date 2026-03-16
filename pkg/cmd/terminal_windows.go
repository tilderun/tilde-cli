package cmd

import "os"

func notifyResize(_ chan<- os.Signal) {
	// SIGWINCH is not supported on Windows; terminal resize detection is a no-op.
}
