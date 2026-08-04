package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const agentsFileName = "AGENTS.md"

func createBootstrapPrompt(workspace string) (string, bool, error) {
	repositoryInstructions, agentsFileFound, err := readRepositoryInstructions(workspace)
	if err != nil {
		return "", false, err
	}

	prompt := buildBootstrapPrompt(repositoryInstructions, agentsFileFound)

	return prompt, agentsFileFound, nil
}

func readRepositoryInstructions(workspace string) (string, bool, error) {
	agentsPath := filepath.Join(workspace, agentsFileName)
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("read %s: %w", agentsFileName, err)
	}

	return string(content), true, nil
}

func buildBootstrapPrompt(repositoryInstructions string, agentsFileFound bool) string {
	var prompt strings.Builder

	prompt.WriteString("# Agent Tools Runner Instructions\n\n")
	prompt.WriteString("You are the reasoning and planning assistant. Agent Tools Runner (ATR) is a local tool executor. The user manually copies requests and responses between you and ATR.\n\n")
	prompt.WriteString("## Required Workflow\n\n")
	prompt.WriteString("1. Restate the user's requirement and list your assumptions.\n")
	prompt.WriteString("2. Ask clarifying questions when the requirement is ambiguous.\n")
	prompt.WriteString("3. Propose a short investigation and implementation plan.\n")
	prompt.WriteString("4. Wait for the user to approve the plan before requesting tool actions.\n")
	prompt.WriteString("5. Batch related search and read actions when practical.\n")
	prompt.WriteString("6. Read the current target file before proposing an edit.\n")
	prompt.WriteString("7. Do not claim an action succeeded until ATR returns a success result.\n\n")
	prompt.WriteString("## Request Format\n\n")
	prompt.WriteString("Send one JSON object with protocol version 1 and a non-empty ordered actions array. Every action requires a unique id and an operation. Do not wrap the JSON in explanatory text.\n\n")
	prompt.WriteString("### Search\n\n")
	prompt.WriteString("{\"version\":\"1\",\"actions\":[{\"id\":\"find-service\",\"operation\":\"search\",\"query\":\"ServiceName\"}]}\n\n")
	prompt.WriteString("### Read Multiple Files\n\n")
	prompt.WriteString("{\"version\":\"1\",\"actions\":[{\"id\":\"read-files\",\"operation\":\"read\",\"paths\":[\"AGENTS.md\",\"main.go\"]}]}\n\n")
	prompt.WriteString("### Edit One File\n\n")
	prompt.WriteString("{\"version\":\"1\",\"actions\":[{\"id\":\"edit-file\",\"operation\":\"edit\",\"path\":\"main.go\",\"replacements\":[{\"oldText\":\"exact existing text\",\"newText\":\"exact replacement text\"}]}]}\n\n")
	prompt.WriteString("For edit actions, provide exact existing text and exact replacement text. The existing text must occur exactly once. Include enough unchanged context to make the target unique. Do not reformat unrelated code. Do not use placeholders or ellipses inside edit text. An empty newText is allowed when intentionally removing exact text.\n\n")
	prompt.WriteString("The search, read, and edit operations are available.\n\n")
	prompt.WriteString("If ATR returns an error, use successful earlier results, do not assume the failed action succeeded, and correct the request only when the intended correction is clear.\n\n")
	prompt.WriteString("## Repository Instructions\n\n")

	if agentsFileFound {
		prompt.WriteString(repositoryInstructions)
		if !strings.HasSuffix(repositoryInstructions, "\n") {
			prompt.WriteString("\n")
		}
	} else {
		prompt.WriteString("No AGENTS.md file was found in the selected workspace.\n")
	}

	return prompt.String()
}
