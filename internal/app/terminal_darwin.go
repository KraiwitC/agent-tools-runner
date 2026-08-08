//go:build darwin

package app

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func enableNonCanonicalInput() (func(), bool, error) {
	fd := int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return func() {}, false, nil
	}

	oldState := *termios

	// Disable canonical mode (ICANON) to prevent 1024-byte OS input line truncation.
	newState := oldState
	newState.Lflag &^= (unix.ICANON | unix.ECHO)

	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &newState); err != nil {
		return func() {}, false, nil
	}

	restored := false
	restore := func() {
		if !restored {
			restored = true
			_ = unix.IoctlSetTermios(fd, unix.TIOCSETA, &oldState)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigChan
		restore()
		os.Exit(130)
	}()

	return restore, true, nil
}
