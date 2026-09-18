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
	tests := []struct {
		name     string
		value    string
		expected []string
	}{
		{name: "camel case", value: "executeMoveAction", expected: []string{"execute", "move", "action"}},
		{name: "Pascal case", value: "ExecuteMoveAction", expected: []string{"execute", "move", "action"}},
		{name: "snake case", value: "execute_move_action", expected: []string{"execute", "move", "action"}},
		{name: "kebab case", value: "execute-move-action", expected: []string{"execute", "move", "action"}},
		{name: "space separated", value: "execute move action", expected: []string{"execute", "move", "action"}},
		{name: "empty identifier", value: "_-", expected: []string{}},
		{name: "acronym prefix", value: "HTTPServer", expected: []string{"httpserver"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens := tokenizeIdentifier(test.value)
			if strings.Join(tokens, ",") != strings.Join(test.expected, ",") {
				t.Fatalf("tokens = %#v, want %#v", tokens, test.expected)
			}
		})
	}
}

func TestRankSearchLineHandlesUnicodeAndEmptyIdentifiers(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		line          string
		matched       bool
		wantMatchType string
	}{
		{name: "Unicode exact match", query: "ค้นหาไฟล์", line: "func ค้นหาไฟล์()", matched: true, wantMatchType: "exact"},
		{name: "empty identifier query", query: "_-", line: "func executeMoveAction()", matched: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matchType, _, matched := rankSearchLine(test.query, test.line)
			if matched != test.matched {
				t.Fatalf("matched = %t, want %t", matched, test.matched)
			}
			if matchType != test.wantMatchType {
				t.Fatalf("match type = %q, want %q", matchType, test.wantMatchType)
			}
		})
	}
}

func TestSortRankedSearchMatchesUsesDeterministicTieBreakers(t *testing.T) {
	matches := []RankedSearchMatch{
		{Path: "same.go", Line: 2, MatchTarget: "content", MatchType: "exact", Score: 1},
		{Path: "same.go", Line: 1, MatchTarget: "content", MatchType: "exact", Score: 1},
		{Path: "same.go", MatchTarget: "path", MatchType: "exact", Score: 1},
		{Path: "later.go", Line: 1, MatchTarget: "content", MatchType: "exact", Score: 1},
	}

	sortRankedSearchMatches(matches)

	expected := []RankedSearchMatch{
		{Path: "later.go", Line: 1, MatchTarget: "content", MatchType: "exact", Score: 1},
		{Path: "same.go", MatchTarget: "path", MatchType: "exact", Score: 1},
		{Path: "same.go", Line: 1, MatchTarget: "content", MatchType: "exact", Score: 1},
		{Path: "same.go", Line: 2, MatchTarget: "content", MatchType: "exact", Score: 1},
	}
	for index := range expected {
		if matches[index] != expected[index] {
			t.Fatalf("match %d = %#v, want %#v", index, matches[index], expected[index])
		}
	}
}

func TestRankedSearchWorkspaceFindsMatchingFoldersAndFiles(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "services/payment_api/config.txt", "unrelated content\n")
	writeTestFile(t, workspace, "services/payment_api/payment-api.yaml", "paymentApi\n")

	matches, truncated, err := rankedSearchWorkspace(workspace, "payment api")
	if err != nil {
		t.Fatalf("rankedSearchWorkspace returned an error: %v", err)
	}
	if truncated {
		t.Fatal("did not expect ranked results to be truncated")
	}
	if len(matches) != 4 {
		t.Fatalf("expected four matches, got %#v", matches)
	}

	foundFolder := false
	foundDescendantFile := false
	foundMatchingFile := false
	foundContent := false
	for _, match := range matches {
		switch {
		case match.Path == "services/payment_api" && match.MatchTarget == "path":
			foundFolder = true
		case match.Path == "services/payment_api/config.txt" && match.MatchTarget == "path":
			foundDescendantFile = true
		case match.Path == "services/payment_api/payment-api.yaml" && match.MatchTarget == "path":
			foundMatchingFile = true
		case match.Path == "services/payment_api/payment-api.yaml" && match.Line == 1 && match.MatchTarget == "content":
			foundContent = true
		}
	}
	if !foundFolder || !foundDescendantFile || !foundMatchingFile || !foundContent {
		t.Fatalf("missing expected path or content matches: %#v", matches)
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

func TestRankedSearchWorkspaceRetainsLateBestPathWithinLimit(t *testing.T) {
	workspace := t.TempDir()
	for index := 0; index < maximumCollectedRankedSearchMatches+1; index++ {
		writeTestFile(t, workspace, fmt.Sprintf("a-EXECUTEMOVEACTION-%03d/file.txt", index), "unrelated\n")
	}
	writeTestFile(t, workspace, "z-executeMoveAction/file.txt", "unrelated\n")

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
	if matches[0].Path != "z-executeMoveAction" || matches[0].MatchTarget != "path" || matches[0].MatchType != "exact" {
		t.Fatalf("late best path match was not retained: %#v", matches[0])
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
