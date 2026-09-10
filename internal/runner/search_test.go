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
	if matches[0].Path != "nested/example.txt" || matches[0].Line != 2 || matches[0].Text != "find me here" || matches[0].MatchTarget != "content" {
		t.Fatalf("unexpected match: %#v", matches[0])
	}
}

func TestSearchWorkspaceFindsMatchingFoldersAndFiles(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "services/payment-api/config.txt", "unrelated content\n")
	writeTestFile(t, workspace, "services/payment-api/payment-api.yaml", "payment-api\n")

	matches, truncated, err := searchWorkspace(workspace, "payment-api")
	if err != nil {
		t.Fatalf("searchWorkspace returned an error: %v", err)
	}
	if truncated {
		t.Fatal("did not expect search results to be truncated")
	}
	if len(matches) != 4 {
		t.Fatalf("expected four matches, got %#v", matches)
	}
	if matches[0].Path != "services/payment-api" || matches[0].MatchTarget != "path" {
		t.Fatalf("unexpected folder match: %#v", matches[0])
	}
	if matches[1].Path != "services/payment-api/config.txt" || matches[1].MatchTarget != "path" {
		t.Fatalf("unexpected descendant file match: %#v", matches[1])
	}
	if matches[2].Path != "services/payment-api/payment-api.yaml" || matches[2].MatchTarget != "path" {
		t.Fatalf("unexpected file match: %#v", matches[2])
	}
	if matches[3].Path != "services/payment-api/payment-api.yaml" || matches[3].Line != 1 || matches[3].MatchTarget != "content" {
		t.Fatalf("unexpected content match: %#v", matches[3])
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

func TestSearchWorkspaceRetainsDeterministicMatchesAtGlobalLimit(t *testing.T) {
	workspace := t.TempDir()
	content := strings.Repeat("needle\n", maximumCollectedSearchMatches+1)
	writeTestFile(t, workspace, "a-first.txt", content)
	writeTestFile(t, workspace, "z-later.txt", "needle\n")

	for attempt := 0; attempt < 5; attempt++ {
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
		if matches[0].Path != "a-first.txt" || matches[len(matches)-1].Path != "a-first.txt" {
			t.Fatalf("search retained nondeterministic files: first=%#v last=%#v", matches[0], matches[len(matches)-1])
		}
	}
}

func TestSearchWorkspaceDoesNotLetLaterPathMatchesCrowdOutEarlierContent(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "a-first.txt", "needle\n")
	for index := 0; index < maximumCollectedSearchMatches+1; index++ {
		writeTestFile(t, workspace, fmt.Sprintf("z-needle-%04d.txt", index), "unrelated\n")
	}

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
	if matches[0].Path != "a-first.txt" || matches[0].Line != 1 || matches[0].MatchTarget != "content" {
		t.Fatalf("earlier content match was crowded out: %#v", matches[0])
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
