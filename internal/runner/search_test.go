package runner

import (
	"fmt"
	"strings"
	"testing"
)

func TestSearchWorkspaceFindsLiteralText(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "nested/example.txt", "first\nfind me here\nlast\n")

	matches, truncated, err := searchWorkspace(workspace, "find me")
	if err != nil {
		t.Fatalf("searchWorkspace returned an error: %v", err)
	}
	if truncated {
		t.Fatal("did not expect search results to be truncated")
	}
	if len(matches) != 1 {
		t.Fatalf("expected one match, got %d", len(matches))
	}
	if matches[0].Path != "nested/example.txt" || matches[0].Line != 2 || matches[0].Text != "find me here" {
		t.Fatalf("unexpected match: %#v", matches[0])
	}
}

func TestSearchWorkspaceIsCaseSensitive(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "example.txt", "Value\n")

	matches, _, err := searchWorkspace(workspace, "value")
	if err != nil {
		t.Fatalf("searchWorkspace returned an error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no case-insensitive matches, got %#v", matches)
	}
}

func TestSearchWorkspaceExcludesDefaultDirectories(t *testing.T) {
	workspace := t.TempDir()
	for directory := range defaultSearchExcludedDirectories {
		writeTestFile(t, workspace, directory+"/hidden.txt", "needle")
	}
	writeTestFile(t, workspace, ".vscode/settings.json", "needle")

	matches, _, err := searchWorkspace(workspace, "needle")
	if err != nil {
		t.Fatalf("searchWorkspace returned an error: %v", err)
	}
	if len(matches) != 1 || matches[0].Path != ".vscode/settings.json" {
		t.Fatalf("expected only .vscode match, got %#v", matches)
	}
}

func TestSearchWorkspaceSkipsBinaryAndOversizedFiles(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "valid.txt", "needle")
	writeTestFile(t, workspace, "binary.dat", "needle\x00binary")
	writeTestFile(t, workspace, "large.txt", strings.Repeat("x", int(maximumFileSize)+1)+"needle")

	matches, _, err := searchWorkspace(workspace, "needle")
	if err != nil {
		t.Fatalf("searchWorkspace returned an error: %v", err)
	}
	if len(matches) != 1 || matches[0].Path != "valid.txt" {
		t.Fatalf("expected only valid.txt match, got %#v", matches)
	}
}

func TestSearchWorkspaceTruncatesAtCollectionLimit(t *testing.T) {
	workspace := t.TempDir()
	var content strings.Builder
	for index := 0; index < maximumCollectedSearchMatches+1; index++ {
		fmt.Fprintf(&content, "needle %d\n", index)
	}
	writeTestFile(t, workspace, "many.txt", content.String())

	matches, truncated, err := searchWorkspace(workspace, "needle")
	if err != nil {
		t.Fatalf("searchWorkspace returned an error: %v", err)
	}
	if !truncated {
		t.Fatal("expected truncated search result")
	}
	if len(matches) != maximumCollectedSearchMatches {
		t.Fatalf("expected %d matches, got %d", maximumCollectedSearchMatches, len(matches))
	}
}
