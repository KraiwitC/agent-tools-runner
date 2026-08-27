package runner

import (
	"fmt"
	"strings"
	"testing"
)

func TestRankSearchLineFindsExactMatch(t *testing.T) {
	matchType, score, matched := rankSearchLine("executeMoveAction", "func executeMoveAction()")
	if !matched {
		t.Fatal("expected exact match")
	}
	if matchType != "exact" || score != 1 {
		t.Fatalf("unexpected match: type=%q score=%v", matchType, score)
	}
}

func TestRankSearchLineFindsCaseInsensitiveMatch(t *testing.T) {
	matchType, score, matched := rankSearchLine("executemoveaction", "func ExecuteMoveAction()")
	if !matched {
		t.Fatal("expected case-insensitive match")
	}
	if matchType != "case_insensitive" || score != 0.99 {
		t.Fatalf("unexpected match: type=%q score=%v", matchType, score)
	}
}

func TestRankSearchLineFindsIdentifierStyleMatch(t *testing.T) {
	matchType, score, matched := rankSearchLine("execute move action", "func execute_move_action()")
	if !matched {
		t.Fatal("expected identifier match")
	}
	if matchType != "identifier" || score != 0.98 {
		t.Fatalf("unexpected match: type=%q score=%v", matchType, score)
	}
}

func TestRankSearchLineFindsFuzzyIdentifierMatch(t *testing.T) {
	matchType, score, matched := rankSearchLine("executeMoveActions", "func executeMoveAction()")
	if !matched {
		t.Fatal("expected fuzzy identifier match")
	}
	if matchType != "fuzzy_identifier" {
		t.Fatalf("match type = %q, want fuzzy_identifier", matchType)
	}
	if score <= 0 || score >= 0.98 {
		t.Fatalf("unexpected fuzzy identifier score: %v", score)
	}
}

func TestRankSearchLineRejectsUnrelatedLine(t *testing.T) {
	_, _, matched := rankSearchLine("executeMoveAction", "const maximumTransferChars = 120000")
	if matched {
		t.Fatal("did not expect unrelated line to match")
	}
}

func TestRankSearchLineDisablesFuzzyMatchingForTwoCharacterQuery(t *testing.T) {
	_, _, matched := rankSearchLine("ID", "var IP string")
	if matched {
		t.Fatal("did not expect fuzzy matching for a two-character query")
	}
}

func TestTokenizeIdentifierSupportsCommonStyles(t *testing.T) {
	expected := []string{"execute", "move", "action"}
	values := []string{
		"executeMoveAction",
		"ExecuteMoveAction",
		"execute_move_action",
		"execute-move-action",
		"execute move action",
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			tokens := tokenizeIdentifier(value)
			if strings.Join(tokens, ",") != strings.Join(expected, ",") {
				t.Fatalf("tokens = %#v, want %#v", tokens, expected)
			}
		})
	}
}

func TestRankedSearchWorkspaceOrdersMatchesDeterministically(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "b.go", "func execute_move_action() {}\n")
	writeTestFile(t, workspace, "a.go", "func ExecuteMoveAction() {}\n")
	writeTestFile(t, workspace, "c.go", "func executeMoveActions() {}\n")

	matches, truncated, err := rankedSearchWorkspace(workspace, "executeMoveAction")
	if err != nil {
		t.Fatalf("rankedSearchWorkspace returned an error: %v", err)
	}
	if truncated {
		t.Fatal("did not expect ranked results to be truncated")
	}
	if len(matches) != 3 {
		t.Fatalf("matches = %d, want 3", len(matches))
	}
	if matches[0].Path != "c.go" || matches[0].MatchType != "exact" {
		t.Fatalf("unexpected first match: %#v", matches[0])
	}
	if matches[1].Path != "a.go" || matches[1].MatchType != "case_insensitive" {
		t.Fatalf("unexpected second match: %#v", matches[1])
	}
	if matches[2].Path != "b.go" || matches[2].MatchType != "identifier" {
		t.Fatalf("unexpected third match: %#v", matches[2])
	}
}

func TestRankedSearchWorkspaceUsesSearchExclusions(t *testing.T) {
	workspace := t.TempDir()
	for directory := range defaultSearchExcludedDirectories {
		writeTestFile(t, workspace, directory+"/hidden.go", "func executeMoveAction() {}")
	}
	writeTestFile(t, workspace, ".vscode/settings.json", "executeMoveAction")

	matches, _, err := rankedSearchWorkspace(workspace, "executeMoveAction")
	if err != nil {
		t.Fatalf("rankedSearchWorkspace returned an error: %v", err)
	}
	if len(matches) != 1 || matches[0].Path != ".vscode/settings.json" {
		t.Fatalf("expected only .vscode match, got %#v", matches)
	}
}

func TestRankedSearchWorkspaceRetainsLateBestMatchWithinFileLimit(t *testing.T) {
	workspace := t.TempDir()
	var content strings.Builder
	for index := 0; index < maximumCollectedRankedSearchMatches+1; index++ {
		fmt.Fprintf(&content, "EXECUTEMOVEACTION %d\n", index)
	}
	content.WriteString("executeMoveAction\n")
	writeTestFile(t, workspace, "matches.go", content.String())

	matches, truncated, err := rankedSearchWorkspace(workspace, "executeMoveAction")
	if err != nil {
		t.Fatalf("rankedSearchWorkspace returned an error: %v", err)
	}
	if !truncated {
		t.Fatal("expected ranked results to be truncated")
	}
	if len(matches) != maximumCollectedRankedSearchMatches {
		t.Fatalf("matches = %d, want %d", len(matches), maximumCollectedRankedSearchMatches)
	}
	if matches[0].MatchType != "exact" || matches[0].Line != maximumCollectedRankedSearchMatches+2 {
		t.Fatalf("late best match was not retained: %#v", matches[0])
	}
}

func TestRankedSearchWorkspaceRetainsBestMatchesWithinGlobalLimit(t *testing.T) {
	workspace := t.TempDir()
	var lowerPriority strings.Builder
	for index := 0; index < maximumCollectedRankedSearchMatches+1; index++ {
		fmt.Fprintf(&lowerPriority, "EXECUTEMOVEACTION %d\n", index)
	}
	writeTestFile(t, workspace, "a-lower.go", lowerPriority.String())
	writeTestFile(t, workspace, "z-best.go", "executeMoveAction\n")

	matches, truncated, err := rankedSearchWorkspace(workspace, "executeMoveAction")
	if err != nil {
		t.Fatalf("rankedSearchWorkspace returned an error: %v", err)
	}
	if !truncated {
		t.Fatal("expected ranked results to be truncated")
	}
	if len(matches) != maximumCollectedRankedSearchMatches {
		t.Fatalf("matches = %d, want %d", len(matches), maximumCollectedRankedSearchMatches)
	}
	if matches[0].Path != "z-best.go" || matches[0].MatchType != "exact" {
		t.Fatalf("best match was not retained: %#v", matches[0])
	}
}

func TestRankedSearchWorkspaceTruncatesAtCollectionLimit(t *testing.T) {
	workspace := t.TempDir()
	var content strings.Builder
	for index := 0; index < maximumCollectedRankedSearchMatches+1; index++ {
		fmt.Fprintf(&content, "executeMoveAction %d\n", index)
	}
	writeTestFile(t, workspace, "many.go", content.String())

	matches, truncated, err := rankedSearchWorkspace(workspace, "executeMoveAction")
	if err != nil {
		t.Fatalf("rankedSearchWorkspace returned an error: %v", err)
	}
	if !truncated {
		t.Fatal("expected ranked results to be truncated")
	}
	if len(matches) != maximumCollectedRankedSearchMatches {
		t.Fatalf("matches = %d, want %d", len(matches), maximumCollectedRankedSearchMatches)
	}
}
