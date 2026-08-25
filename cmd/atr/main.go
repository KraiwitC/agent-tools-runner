package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"agent-tools-runner/internal/app"
	atrclipboard "agent-tools-runner/internal/clipboard"
	"agent-tools-runner/internal/prompt"
)

const (
	applicationName    = "Agent Tools Runner"
	applicationVersion = "v0.7.0"
)

type options struct {
	workspace string
	version   bool
}

func main() {
	options, err := parseOptions(os.Args[1:], os.Stderr)
	if err != nil {
		os.Exit(2)
	}
	if options.version {
		fmt.Println(applicationVersion)
		return
	}

	workspace, err := app.ResolveWorkspace(options.workspace)
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

	app.Run(workspace, clipboardReady, os.Stdin)
}

func parseOptions(args []string, output io.Writer) (options, error) {
	flagSet := flag.NewFlagSet("atr", flag.ContinueOnError)
	flagSet.SetOutput(output)

	workspace := flagSet.String("workspace", ".", "project workspace directory")
	version := flagSet.Bool("version", false, "show application version")
	if err := flagSet.Parse(args); err != nil {
		return options{}, err
	}

	return options{
		workspace: *workspace,
		version:   *version,
	}, nil
}
