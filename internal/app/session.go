package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	atrclipboard "agent-tools-runner/internal/clipboard"
	"agent-tools-runner/internal/prompt"
	"agent-tools-runner/internal/runner"
)

const scannerInitialBufferSize = 64 * 1024
const scannerMaximumBufferSize = 64 * 1024 * 1024

type lineScanner interface {
	Scan() bool
	Text() string
	Err() error
}

func Run(workspace string, clipboardReady bool, input io.Reader, startAutoMode bool) {
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
	runSession(workspace, clipboardReady, newAsyncLineScanner(scanner), startAutoMode)
}

type scanResult struct {
	text string
	err  error
}

type asyncLineScanner struct {
	results <-chan scanResult
	text    string
	err     error
}

func newAsyncLineScanner(scanner lineScanner) *asyncLineScanner {
	results := make(chan scanResult)
	go func() {
		defer close(results)
		for scanner.Scan() {
			results <- scanResult{text: scanner.Text()}
		}
		results <- scanResult{err: scanner.Err()}
	}()
	return &asyncLineScanner{results: results}
}

func (a *asyncLineScanner) Scan() bool {
	result, ok := <-a.results
	if !ok {
		return false
	}
	a.text = result.text
	a.err = result.err
	return result.err == nil
}

func (a *asyncLineScanner) Text() string {
	return a.text
}

func (a *asyncLineScanner) Err() error {
	return a.err
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
			if peek, err := t.reader.Peek(1); err == nil && len(peek) > 0 && peek[0] == '\n' {
				_, _ = t.reader.ReadByte()
			}
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

func runSession(workspace string, clipboardReady bool, scanner *asyncLineScanner, startAutoMode bool) {
	lastResponse := ""
	autoMode := startAutoMode && clipboardReady
	lastClipboardRequest := ""
	autoPromptShown := false
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	if startAutoMode && !clipboardReady {
		fmt.Println("Auto mode was not started because the clipboard is unavailable.")
		fmt.Println()
	}

	for {
		if autoMode {
			if !autoPromptShown {
				fmt.Print("Command > ")
				autoPromptShown = true
			}
			select {
			case result, ok := <-scanner.results:
				if !ok || result.err != nil {
					fmt.Println()
					return
				}
				autoPromptShown = false
				input := strings.TrimSpace(result.text)
				if input == "" {
					continue
				}
				if input == "/auto" {
					autoMode = false
					fmt.Println("Stopped monitoring clipboard.")
					fmt.Println()
					continue
				}
				if !strings.HasPrefix(input, "/") {
					fmt.Println("Input ignored. Only slash commands are accepted while auto mode is active.")
					fmt.Println()
					continue
				}
				if handleCommand(input, workspace, lastResponse, clipboardReady) {
					return
				}
			case <-ticker.C:
				trimmedText := strings.TrimSpace(atrclipboard.Read())
				if looksLikeRequest(trimmedText) && trimmedText != lastClipboardRequest {
					request, err := runner.ParseAndValidateRequest(trimmedText)
					if err == nil {
						lastClipboardRequest = trimmedText
						fmt.Println()
						responseText := runner.ExecuteRequest(workspace, request)
						lastResponse = responseText
						presentResponse(responseText, clipboardReady)
						autoPromptShown = false
					}
				}
			}
			continue
		}

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
			if input == "/auto" {
				if !clipboardReady {
					fmt.Println("Clipboard is unavailable.")
					fmt.Println()
					continue
				}
				autoMode = true
				fmt.Println("Started monitoring clipboard.")
				fmt.Println("Type /auto again to stop.")
				fmt.Println()
				continue
			}

			if handleCommand(input, workspace, lastResponse, clipboardReady) {
				return
			}
			continue
		}

		if !isJSONRequestStart(input) {
			fmt.Println("Input ignored. ATR accepts slash commands or a JSON request beginning with \"{\" or a JSON code fence.")
			fmt.Println()
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
			fmt.Println()
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

func looksLikeRequest(text string) bool {
	if text == "" {
		return false
	}

	if strings.Contains(text, "\"status\"") && strings.Contains(text, "\"results\"") {
		return false
	}

	return strings.Contains(text, "\"version\"") && strings.Contains(text, "\"actions\"")
}

func handleCommand(command string, workspace string, lastResponse string, clipboardReady bool) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}

	shouldExit := false
	switch fields[0] {
	case "/help":
		printHelp()
	case "/prompt":
		bootstrapPrompt, _, _, err := prompt.Create(workspace)
		if err != nil {
			fmt.Printf("Failed to refresh LLM bootstrap prompt: %v\n", err)
		} else if clipboardReady {
			atrclipboard.Copy(bootstrapPrompt)
			fmt.Println("Refreshed LLM bootstrap prompt copied to clipboard.")
			fmt.Println("Start a new conversation and paste the prompt for refreshed project instructions to take effect.")
			fmt.Println()
		} else {
			fmt.Println("Clipboard is unavailable. Type /show-prompt to display a refreshed bootstrap prompt.")
			fmt.Println()
		}
	case "/show-prompt":
		bootstrapPrompt, _, _, err := prompt.Create(workspace)
		if err != nil {
			fmt.Printf("Failed to refresh LLM bootstrap prompt: %v\n", err)
			fmt.Println()
		} else {
			fmt.Println(bootstrapPrompt)
			fmt.Println()
		}
	case "/copy":
		if lastResponse == "" {
			fmt.Println("No JSON response is available to copy.")
			fmt.Println()
		} else if clipboardReady {
			atrclipboard.Copy(lastResponse)
			fmt.Println("Last JSON response copied to clipboard.")
			fmt.Println()
		} else {
			fmt.Println("Clipboard is unavailable. Type /show to display the last JSON response.")
			fmt.Println()
		}
	case "/show":
		if lastResponse == "" {
			fmt.Println("No JSON response is available to show.")
			fmt.Println()
		} else {
			fmt.Println(lastResponse)
			fmt.Println()
		}
	case "/limit":
		if len(fields) == 1 {
			fmt.Printf("Transfer limit: %d characters\n", runner.MaximumTransferChars())
			fmt.Println()
			break
		}
		if len(fields) != 2 {
			fmt.Println("Usage: /limit [characters]")
			fmt.Println()
			break
		}

		limit, err := strconv.Atoi(fields[1])
		if err != nil || limit < 1000 {
			fmt.Println("Transfer limit must be an integer greater than or equal to 1000.")
			fmt.Println()
			break
		}

		runner.SetMaximumTransferChars(limit)
		fmt.Printf("Transfer limit: %d characters\n", limit)
		fmt.Println()
	case "/workspace":
		fmt.Println(workspace)
		fmt.Println()
	case "/clear":
		fmt.Print("\033[H\033[2J")
	case "/cancel":
		fmt.Println("No active JSON request to cancel.")
		fmt.Println()
	case "/exit":
		shouldExit = true
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Type /help for available commands.")
		fmt.Println()
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
	fmt.Println("  /limit [n]    Show or set the transfer limit")
	fmt.Println("  /workspace    Show the current workspace")
	fmt.Println("  /auto         Toggle clipboard monitoring mode")
	fmt.Println("  /clear        Clear the terminal without deleting the last response")
	fmt.Println("  /exit         Exit Agent Tools Runner")
	fmt.Println()
	fmt.Println("Paste a protocol version 1 JSON request, then press Enter on an empty line.")
	fmt.Println("While entering JSON, type /cancel on its own line to discard the request.")
	fmt.Println()
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
