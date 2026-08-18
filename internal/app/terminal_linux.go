//go:build linux

package app

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
)

func enableNonCanonicalInput() (func(), bool, error) {
	fd := int(os.Stdin.Fd())
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return func() {}, false, nil
	}

	oldState := *termios

	// Disable canonical mode (ICANON) to prevent OS input line truncation.
	newState := oldState
	newState.Lflag &^= (unix.ICANON | unix.ECHO)

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return func() {}, false, nil
	}

	restored := false
	restore := func() {
		if !restored {
			restored = true
			_ = unix.IoctlSetTermios(fd, unix.TCSETS, &oldState)
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
