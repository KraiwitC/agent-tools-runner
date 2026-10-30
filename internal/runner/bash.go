package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

const maximumBashCaptureBytes = int(maximumFileSize)
const bashWaitDelay = 2 * time.Second

const bashForbiddenCharacters = ";&|<>`(){}\r\n"

func executeBashAction(workspace string, action Action, actionIndex int, progress io.Writer) (*ActionData, *ResponseError) {
	config, found, err := LoadConfig(workspace)
	if err != nil {
		return nil, newActionResponseError(action, actionIndex, "BASH_NOT_ALLOWED", "bash is disabled because "+ConfigDisplayPath+" is invalid. Ask the user to fix it.", "")
	}
	if !found {
		return nil, newActionResponseError(action, actionIndex, "BASH_NOT_ALLOWED", "bash is disabled because "+ConfigDisplayPath+" does not exist. Ask the user to run /init and add allowed commands.", "")
	}
	if strings.ContainsAny(action.Command, bashForbiddenCharacters) {
		return nil, newActionResponseError(action, actionIndex, "BASH_NOT_ALLOWED", "The command contains shell control characters (; & | < > ` ( ) { } or a line break), which are not allowed.", "")
	}
	if !isBashCommandAllowed(action.Command, config.Bash.Allow) {
		return nil, newActionResponseError(action, actionIndex, "BASH_NOT_ALLOWED", "The command is not on the bash allowlist. Ask the user to add it to bash.allow in "+ConfigDisplayPath+" or to run it manually.", "")
	}

	timeout := time.Duration(defaultBashTimeoutSeconds) * time.Second
	if action.TimeoutSeconds > 0 {
		timeout = time.Duration(action.TimeoutSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shellName, shellArguments := bashShellCommand(action.Command)
	command := exec.CommandContext(ctx, shellName, shellArguments...)
	command.Dir = workspace
	command.WaitDelay = bashWaitDelay
	configureBashProcess(command)

	console := &consoleWriter{writer: progress}
	console.writeLine("  → Running bash: " + printableCommand(action.Command))
	stdout := &limitedBuffer{limit: maximumBashCaptureBytes}
	stderr := &limitedBuffer{limit: maximumBashCaptureBytes}
	command.Stdout = io.MultiWriter(stdout, console)
	command.Stderr = io.MultiWriter(stderr, console)

	runError := command.Run()
	console.finishLine()

	data := &ActionData{Stdout: stdout.String(), Stderr: stderr.String()}
	if runError == nil {
		exitCode := 0
		data.ExitCode = &exitCode
		return data, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		data.TimedOut = true
		return data, newActionResponseError(action, actionIndex, "BASH_TIMEOUT", "The command exceeded its timeout and was stopped. Partial output is included.", "")
	}
	var exitError *exec.ExitError
	if errors.As(runError, &exitError) {
		exitCode := exitError.ExitCode()
		data.ExitCode = &exitCode
		return data, nil
	}
	return nil, newActionResponseError(action, actionIndex, "BASH_FAILED", "The command could not be started.", "")
}

func isBashCommandAllowed(command string, allow []string) bool {
	commandWords := strings.Fields(command)
	for _, entry := range allow {
		entryWords := strings.Fields(entry)
		if len(entryWords) > 0 && len(entryWords) <= len(commandWords) && slices.Equal(commandWords[:len(entryWords)], entryWords) {
			return true
		}
	}
	return false
}

func bashShellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		script := command + "; $atrSucceeded = $?; if ($LASTEXITCODE) { exit $LASTEXITCODE }; if (-not $atrSucceeded) { exit 1 }"
		return "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-Command", script}
	}
	if bashPath, err := exec.LookPath("bash"); err == nil {
		return bashPath, []string{"-c", command}
	}
	return "sh", []string{"-c", command}
}

func printableCommand(command string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return -1
		}
		return character
	}, command)
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if remaining := b.limit - b.buffer.Len(); remaining > 0 {
		b.buffer.Write(data[:min(len(data), remaining)])
	}
	return len(data), nil
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}

type consoleWriter struct {
	mutex       sync.Mutex
	writer      io.Writer
	wroteOutput bool
	lastByte    byte
}

func (c *consoleWriter) Write(data []byte) (int, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if len(data) > 0 {
		c.wroteOutput = true
		c.lastByte = data[len(data)-1]
	}
	_, _ = c.writer.Write(data)
	return len(data), nil
}

func (c *consoleWriter) writeLine(line string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, _ = io.WriteString(c.writer, line+"\n")
}

func (c *consoleWriter) finishLine() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.wroteOutput && c.lastByte != '\n' {
		_, _ = io.WriteString(c.writer, "\n")
	}
}
