//go:build windows

package app

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/windows"
)

func enableNonCanonicalInput() (func(), bool, error) {
	handle := windows.Handle(os.Stdin.Fd())
	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return func() {}, false, nil
	}

	// Disable line input and echo input to prevent console paste truncation.
	newMode := oldMode &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT)
	if err := windows.SetConsoleMode(handle, newMode); err != nil {
		return func() {}, false, nil
	}

	restored := false
	restore := func() {
		if !restored {
			restored = true
			_ = windows.SetConsoleMode(handle, oldMode)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		restore()
		os.Exit(130)
	}()

	return restore, true, nil
}
