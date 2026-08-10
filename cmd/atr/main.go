package main

import (
	"flag"
	"fmt"
	"os"

	"agent-tools-runner/internal/app"
	atrclipboard "agent-tools-runner/internal/clipboard"
	"agent-tools-runner/internal/prompt"
)

const (
	applicationName    = "Agent Tools Runner"
	applicationVersion = "v0.3.2"
)

func main() {
	workspaceFlag := flag.String("workspace", ".", "project workspace directory")
	flag.Parse()

	workspace, err := app.ResolveWorkspace(*workspaceFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open workspace: %v\n", err)
		os.Exit(1)
	}

	bootstrapPrompt, agentsFileFound, err := prompt.Create(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bootstrap prompt: %v\n", err)
		os.Exit(1)
	}

	clipboardReady, clipboardErr := atrclipboard.Initialize()

	fmt.Println(applicationName + " " + applicationVersion)
	fmt.Printf("Workspace: %s\n", workspace)
	if agentsFileFound {
		fmt.Printf("Project instructions: %s found\n", prompt.AgentsFileName)
	} else {
		fmt.Printf("Project instructions: no %s found\n", prompt.AgentsFileName)
	}
	fmt.Println()
	if clipboardReady {
		atrclipboard.Copy(bootstrapPrompt)
		fmt.Println("LLM bootstrap prompt copied to clipboard.")
		fmt.Println("Paste it into your chatbot to begin.")
	} else {
		fmt.Printf("Clipboard is unavailable: %v\n", clipboardErr)
		fmt.Println("Type /show-prompt to display the bootstrap prompt.")
	}
	fmt.Println()
	fmt.Println("Paste a JSON request, then press Enter on an empty line to run it.")
	fmt.Println("Type /help for commands.")

	app.Run(workspace, bootstrapPrompt, clipboardReady, os.Stdin)
}
