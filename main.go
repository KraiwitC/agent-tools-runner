package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const applicationName = "Agent Tools Runner"
const scannerInitialBufferSize = 64 * 1024
const scannerMaximumBufferSize = 2 * 1024 * 1024

func main() {
	workspaceFlag := flag.String("workspace", ".", "project workspace directory")
	flag.Parse()
	workspace, err := resolveWorkspace(*workspaceFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open workspace: %v\n", err)
		os.Exit(1)
	}

	bootstrapPrompt, agentsFileFound, err := createBootstrapPrompt(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bootstrap prompt: %v\n", err)
		os.Exit(1)
	}

	clipboardReady, clipboardErr := initializeClipboard()

	fmt.Println(applicationName)
	fmt.Printf("Workspace: %s\n", workspace)
	if agentsFileFound {
		fmt.Printf("Project instructions: %s found\n", agentsFileName)
	} else {
		fmt.Printf("Project instructions: no %s found\n", agentsFileName)
	}
	fmt.Println()
	if clipboardReady {
		copyText(bootstrapPrompt)
		fmt.Println("LLM bootstrap prompt copied to clipboard.")
		fmt.Println("Paste it into your chatbot to begin.")
	} else {
		fmt.Printf("Clipboard is unavailable: %v\n", clipboardErr)
		fmt.Println("Type /show-prompt to display the bootstrap prompt.")
	}
	fmt.Println()
	fmt.Println("Paste a JSON request, then press Enter on an empty line to run it.")
	fmt.Println("Type /help for commands.")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, scannerInitialBufferSize), scannerMaximumBufferSize)
	runSession(workspace, bootstrapPrompt, clipboardReady, scanner)
}

func resolveWorkspace(path string) (string, error) {
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

func runSession(workspace string, bootstrapPrompt string, clipboardReady bool, scanner *bufio.Scanner) {
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
			shouldExit := handleCommand(input, workspace, bootstrapPrompt, lastResponse, clipboardReady)
			if shouldExit {
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
			responseText := createErrorResponse("INVALID_REQUEST", err.Error())
			lastResponse = responseText
			presentResponse(responseText, clipboardReady)
			continue
		}
		if cancelled {
			fmt.Println("Request cancelled.")
			continue
		}

		request, err := parseAndValidateRequest(requestText)
		if err != nil {
			responseText := createErrorResponse("INVALID_REQUEST", err.Error())
			lastResponse = responseText
			presentResponse(responseText, clipboardReady)
			continue
		}

		responseText := executeRequest(workspace, request)
		lastResponse = responseText
		presentResponse(responseText, clipboardReady)
	}
}

func handleCommand(command string, workspace string, bootstrapPrompt string, lastResponse string, clipboardReady bool) bool {
	shouldExit := false
	switch command {
	case "/help":
		printHelp()
	case "/prompt":
		if clipboardReady {
			copyText(bootstrapPrompt)
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
			copyText(lastResponse)
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
	fmt.Println(responseText)
	if clipboardReady {
		copyText(responseText)
		fmt.Println("Response copied to clipboard.")
	} else {
		fmt.Println("Clipboard is unavailable. Copy the JSON response shown above.")
	}
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
