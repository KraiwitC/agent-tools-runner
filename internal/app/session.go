package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	atrclipboard "agent-tools-runner/internal/clipboard"
	"agent-tools-runner/internal/runner"
)

const scannerInitialBufferSize = 64 * 1024
const scannerMaximumBufferSize = 2 * 1024 * 1024

type lineScanner interface {
	Scan() bool
	Text() string
	Err() error
}

func Run(workspace string, bootstrapPrompt string, clipboardReady bool, input io.Reader) {
	cleanup, interactive, _ := enableNonCanonicalInput()
	defer cleanup()

	var scanner lineScanner
	if interactive {
		scanner = newTerminalLineReader(input)
	} else {
		bufScanner := bufio.NewScanner(input)
		bufScanner.Buffer(make([]byte, scannerInitialBufferSize), scannerMaximumBufferSize)
		scanner = bufScanner
	}
	runSession(workspace, bootstrapPrompt, clipboardReady, scanner)
}

type terminalLineReader struct {
	reader *bufio.Reader
	line   string
	err    error
}

func newTerminalLineReader(r io.Reader) *terminalLineReader {
	return &terminalLineReader{
		reader: bufio.NewReader(r),
	}
}

func (t *terminalLineReader) Scan() bool {
	var lineRunes []rune
	for {
		r, _, err := t.reader.ReadRune()
		if err != nil {
			if len(lineRunes) > 0 {
				t.line = string(lineRunes)
				t.err = nil
				return true
			}
			t.err = err
			return false
		}

		if r == '\x7f' || r == '\x08' {
			if len(lineRunes) > 0 {
				lineRunes = lineRunes[:len(lineRunes)-1]
				fmt.Print("\b \b")
			}
			continue
		}

		if r == '\r' {
			fmt.Println()
			t.line = string(lineRunes)
			return true
		}
		if r == '\n' {
			fmt.Println()
			t.line = string(lineRunes)
			return true
		}

		lineRunes = append(lineRunes, r)
		fmt.Print(string(r))
	}
}

func (t *terminalLineReader) Text() string {
	return t.line
}

func (t *terminalLineReader) Err() error {
	if errors.Is(t.err, io.EOF) {
		return nil
	}
	return t.err
}

func runSession(workspace string, bootstrapPrompt string, clipboardReady bool, scanner lineScanner) {
	lastResponse := ""
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			return
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if handleCommand(input, workspace, bootstrapPrompt, lastResponse, clipboardReady) {
				return
			}
			continue
		}

		if !isJSONRequestStart(input) {
			fmt.Println("Input ignored. ATR accepts slash commands or a JSON request beginning with \"{\" or a JSON code fence.")
			continue
		}

		requestText, cancelled, err := collectJSONRequest(input, scanner)
		if err != nil {
			responseText := runner.CreateErrorResponse("INVALID_REQUEST", err.Error())
			lastResponse = responseText
			presentResponse(responseText, clipboardReady)
			continue
		}
		if cancelled {
			fmt.Println("Request cancelled.")
			continue
		}

		request, err := runner.ParseAndValidateRequest(requestText)
		if err != nil {
			responseText := runner.CreateErrorResponse("INVALID_REQUEST", err.Error())
			lastResponse = responseText
			presentResponse(responseText, clipboardReady)
			continue
		}

		responseText := runner.ExecuteRequest(workspace, request)
		lastResponse = responseText
		presentResponse(responseText, clipboardReady)
	}
}

func collectJSONRequest(firstLine string, scanner lineScanner) (string, bool, error) {
	var input strings.Builder
	input.WriteString(firstLine)
	for {
		fmt.Print("... ")
		if !scanner.Scan() {
			if scanner.Err() != nil {
				return "", false, fmt.Errorf("read request: %w", scanner.Err())
			}
			return "", false, errors.New("JSON request ended before an empty terminating line")
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "/cancel" {
			return "", true, nil
		}
		if line == "" {
			return input.String(), false, nil
		}

		input.WriteString("\n")
		input.WriteString(line)
	}
}

func isJSONRequestStart(input string) bool {
	trimmedInput := strings.TrimSpace(input)
	if strings.HasPrefix(trimmedInput, "{") {
		return true
	}

	lowerInput := strings.ToLower(trimmedInput)
	return lowerInput == "```" || lowerInput == "```json" || lowerInput == "~~~" || lowerInput == "~~~json"
}

func handleCommand(command string, workspace string, bootstrapPrompt string, lastResponse string, clipboardReady bool) bool {
	shouldExit := false
	switch command {
	case "/help":
		printHelp()
	case "/prompt":
		if clipboardReady {
			atrclipboard.Copy(bootstrapPrompt)
			fmt.Println("LLM bootstrap prompt copied to clipboard.")
		} else {
			fmt.Println("Clipboard is unavailable. Type /show-prompt to display the bootstrap prompt.")
		}
	case "/show-prompt":
		fmt.Println(bootstrapPrompt)
	case "/copy":
		if lastResponse == "" {
			fmt.Println("No JSON response is available to copy.")
		} else if clipboardReady {
			atrclipboard.Copy(lastResponse)
			fmt.Println("Last JSON response copied to clipboard.")
		} else {
			fmt.Println("Clipboard is unavailable. Type /show to display the last JSON response.")
		}
	case "/show":
		if lastResponse == "" {
			fmt.Println("No JSON response is available to show.")
		} else {
			fmt.Println(lastResponse)
		}
	case "/workspace":
		fmt.Println(workspace)
	case "/clear":
		fmt.Print("\033[H\033[2J")
	case "/exit":
		shouldExit = true
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Type /help for available commands.")
	}
	return shouldExit
}

func presentResponse(responseText string, clipboardReady bool) {
	summary, err := summarizeResponse(responseText)
	if err != nil {
		fmt.Println("Could not summarize the response. Type /show to view the complete JSON response.")
	} else {
		fmt.Println(summary)
	}
	fmt.Println()
	if clipboardReady {
		atrclipboard.Copy(responseText)
		fmt.Println("JSON response copied to clipboard.")
	} else {
		fmt.Println("Clipboard is unavailable. Type /show to view the complete JSON response.")
	}
	fmt.Println()
}

func printHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  /help         Show available commands")
	fmt.Println("  /prompt       Copy the LLM bootstrap prompt")
	fmt.Println("  /show-prompt  Show the LLM bootstrap prompt")
	fmt.Println("  /copy         Copy the last complete JSON response")
	fmt.Println("  /show         Show the last complete JSON response")
	fmt.Println("  /workspace    Show the current workspace")
	fmt.Println("  /clear        Clear the terminal without deleting the last response")
	fmt.Println("  /exit         Exit Agent Tools Runner")
	fmt.Println()
	fmt.Println("Paste a protocol version 1 JSON request, then press Enter on an empty line.")
	fmt.Println("While entering JSON, type /cancel on its own line to discard the request.")
}

func ResolveWorkspace(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	fileInfo, err := os.Lstat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("inspect workspace: %w", err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("workspace must not be a symbolic link")
	}
	if !fileInfo.IsDir() {
		return "", errors.New("workspace is not a directory")
	}
	return filepath.Clean(absolutePath), nil
}
