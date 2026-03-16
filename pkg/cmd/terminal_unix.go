//go:build !windows

package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

func notifyResize(sigCh chan<- os.Signal) {
	signal.Notify(sigCh, syscall.SIGWINCH)
}
