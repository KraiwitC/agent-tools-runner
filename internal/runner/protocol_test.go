package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestParseAndValidateRequestAcceptsValidBatch(t *testing.T) {
	requestText := `{
		"version": "1",
		"actions": [
			{"id": "search", "operation": "search", "query": "needle"},
			{"id": "ranked-search", "operation": "ranked_search", "query": "executeMoveActions"},
			{"id": "read", "operation": "read", "paths": ["main.go"]},
			{"id": "edit", "operation": "edit", "path": "main.go", "expectedSha256": "0000000000000000000000000000000000000000000000000000000000000000", "replacements": [{"oldText": "old", "newText": "new"}]},
			{"id": "create", "operation": "create", "path": "created.txt", "content": "created content"},
			{"id": "copy", "operation": "copy", "source": "source.txt", "destination": "copied.txt", "expectedSha256": "0000000000000000000000000000000000000000000000000000000000000000"},
			{"id": "move", "operation": "move", "source": "copied.txt", "destination": "moved.txt", "expectedSha256": "0000000000000000000000000000000000000000000000000000000000000000"},
			{"id": "tree", "operation": "tree", "path": "."}
		]
	}`

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if len(request.Actions) != 8 {
		t.Fatalf("expected eight actions, got %d", len(request.Actions))
	}
}

func TestParseAndValidateRequestRejectsUnknownField(t *testing.T) {
	requestText := `{"version":"1","actions":[],"status":"success"}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected unknown field validation error")
	}
}

func TestParseAndValidateRequestAcceptsSingleCharacterLiteralSearch(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"search","operation":"search","query":"a"}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
}

func TestParseAndValidateRequestAcceptsRankedSearch(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"ranked-search","operation":"ranked_search","query":"ID"}]}`

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if request.Actions[0].Operation != "ranked_search" {
		t.Fatalf("unexpected operation: %q", request.Actions[0].Operation)
	}
}

func TestParseAndValidateRequestRejectsRankedSearchWithoutLettersOrDigits(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"ranked-search","operation":"ranked_search","query":"_-"}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected ranked_search query validation error")
	}
}

func TestParseAndValidateRequestRejectsTooManyActions(t *testing.T) {
	actions := make([]Action, maximumActions+1)
	for index := range actions {
		actions[index] = Action{
			ID:        fmt.Sprintf("action-%d", index),
			Operation: "search",
			Query:     "needle",
		}
	}

	err := validateRequest(Request{Version: protocolVersion, Actions: actions})
	if err == nil {
		t.Fatal("expected action limit validation error")
	}
}

func TestParseAndValidateRequestRejectsTooManyEditReplacements(t *testing.T) {
	replacements := make([]Replacement, maximumEditReplacements+1)
	for index := range replacements {
		replacements[index] = Replacement{OldText: fmt.Sprintf("old-%d", index), NewText: "new"}
	}

	err := validateRequest(Request{
		Version: protocolVersion,
		Actions: []Action{
			{ID: "edit", Operation: "edit", Path: "main.go", ExpectedSHA256: "0000000000000000000000000000000000000000000000000000000000000000", Replacements: replacements},
		},
	})
	if err == nil {
		t.Fatal("expected edit replacement limit validation error")
	}
}

func TestParseAndValidateRequestRejectsDuplicateActionIDs(t *testing.T) {
	requestText := `{
		"version": "1",
		"actions": [
			{"id": "same", "operation": "search", "query": "first"},
			{"id": "same", "operation": "search", "query": "second"}
		]
	}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected duplicate action ID validation error")
	}
}

func TestExecuteRequestPreservesResultWithinTransferLimit(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "small.txt", "content")
	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{ID: "read-small", Operation: "read", Paths: []string{"small.txt"}},
		},
	}

	responseText := ExecuteRequest(workspace, request)
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "success" || len(response.Results) != 1 || response.Results[0].Data == nil {
		t.Fatalf("unexpected successful response: %#v", response)
	}
}

func TestExecuteRequestDoesNotModifyWorkspaceWithoutResponseCapacity(t *testing.T) {
	workspace := t.TempDir()
	action := Action{
		ID:        strings.Repeat("x", 1000),
		Operation: "create",
		Path:      "created.txt",
		Content:   "content",
	}
	request := Request{
		Version: protocolVersion,
		Actions: []Action{action},
	}
	candidate := Response{
		Version: protocolVersion,
		Status:  "limit",
		Results: []ActionResult{maximumModifyingActionResult(action)},
		Error:   newTransferLimitError(action, 0),
	}

	originalLimit := MaximumTransferChars()
	SetMaximumTransferChars(len(marshalResponse(candidate)) - 1)
	t.Cleanup(func() { SetMaximumTransferChars(originalLimit) })

	response := executeRequestForTest(t, workspace, request)
	if response.Status != "limit" || response.Error == nil || response.Error.Code != "TRANSFER_LIMIT_EXCEEDED" {
		t.Fatalf("unexpected response: %#v", response)
	}
	assertPathDoesNotExist(t, workspace+"/created.txt")
}

func TestExecuteRequestReturnsPartialSecondRead(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "first.txt", strings.Repeat("a", 200))
	writeTestFile(t, workspace, "second.txt", strings.Repeat("b", 5000))
	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{ID: "first", Operation: "read", Paths: []string{"first.txt"}},
			{ID: "second", Operation: "read", Paths: []string{"second.txt"}},
		},
	}

	originalLimit := MaximumTransferChars()
	SetMaximumTransferChars(1500)
	t.Cleanup(func() { SetMaximumTransferChars(originalLimit) })

	response := executeRequestForTest(t, workspace, request)
	if response.Status != "limit" || len(response.Results) != 2 {
		t.Fatalf("unexpected response: %#v", response)
	}
	secondResult := response.Results[1]
	if secondResult.Data == nil || !secondResult.Data.Truncated || len(secondResult.Data.Files) != 1 {
		t.Fatalf("unexpected second result: %#v", secondResult)
	}
	content := secondResult.Data.Files[0].Content
	if content == "" || len(content) >= 5000 {
		t.Fatalf("expected partial second file, got %d characters", len(content))
	}
}

func TestExecuteRequestTruncatesReadRangeAtLineBoundary(t *testing.T) {
	workspace := t.TempDir()
	line := strings.Repeat("x", 1000) + "\n"
	content := line + line
	writeTestFile(t, workspace, "lines.txt", content)
	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{ID: "read-range", Operation: "read_range", Path: "lines.txt", StartLine: 1, EndLine: 2},
		},
	}
	oneLineResponse := Response{
		Version: protocolVersion,
		Status:  "limit",
		Results: []ActionResult{
			{
				ID:        "read-range",
				Operation: "read_range",
				Status:    "success",
				Data: &ActionData{
					Path:       "lines.txt",
					Content:    line,
					StartLine:  1,
					EndLine:    1,
					TotalLines: 2,
					SHA256:     calculateSHA256([]byte(content)),
					Truncated:  true,
				},
			},
		},
		Error: newTransferLimitError(request.Actions[0], 0),
	}

	originalLimit := MaximumTransferChars()
	SetMaximumTransferChars(len(marshalResponse(oneLineResponse)))
	t.Cleanup(func() { SetMaximumTransferChars(originalLimit) })

	response := executeRequestForTest(t, workspace, request)
	if response.Status != "limit" || len(response.Results) != 1 {
		t.Fatalf("unexpected response: %#v", response)
	}
	data := response.Results[0].Data
	if data == nil || !data.Truncated || data.Content != line || data.EndLine != 1 {
		t.Fatalf("unexpected truncated range: %#v", data)
	}
}

func TestExecuteRequestDropsOversizedErrorResult(t *testing.T) {
	workspace := t.TempDir()
	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{ID: strings.Repeat("x", 2000), Operation: "read", Paths: []string{"missing.txt"}},
		},
	}

	originalLimit := MaximumTransferChars()
	SetMaximumTransferChars(1000)
	t.Cleanup(func() { SetMaximumTransferChars(originalLimit) })

	response := executeRequestForTest(t, workspace, request)
	if response.Status != "limit" || response.Error == nil || response.Error.Code != "TRANSFER_LIMIT_EXCEEDED" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if len(response.Results) != 0 {
		t.Fatalf("expected oversized error result to be removed, got %d results", len(response.Results))
	}
}

func TestExecuteRequestAllowsMultipleEditsToSameFileWithOriginalHash(t *testing.T) {
	workspace := t.TempDir()
	original := "alpha beta gamma"
	path := writeTestFile(t, workspace, "edit.txt", original)
	originalSHA256 := calculateSHA256([]byte(original))

	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{
				ID:             "edit-alpha",
				Operation:      "edit",
				Path:           "edit.txt",
				ExpectedSHA256: originalSHA256,
				Replacements: []Replacement{
					{OldText: "alpha", NewText: "A"},
				},
			},
			{
				ID:             "edit-gamma",
				Operation:      "edit",
				Path:           "edit.txt",
				ExpectedSHA256: originalSHA256,
				Replacements: []Replacement{
					{OldText: "gamma", NewText: "G"},
				},
			},
		},
	}

	response := executeRequestForTest(t, workspace, request)
	if response.Status != "success" || len(response.Results) != 2 {
		t.Fatalf("unexpected response: %#v", response)
	}
	firstExpectedSHA256 := calculateSHA256([]byte("A beta gamma"))
	if response.Results[0].Data == nil || response.Results[0].Data.SHA256 != firstExpectedSHA256 {
		t.Fatalf("unexpected first edit result: %#v", response.Results[0])
	}
	finalExpectedSHA256 := calculateSHA256([]byte("A beta G"))
	if response.Results[1].Data == nil || response.Results[1].Data.SHA256 != finalExpectedSHA256 {
		t.Fatalf("unexpected second edit result: %#v", response.Results[1])
	}
	assertFileContent(t, path, "A beta G")
}

func TestExecuteRequestAllowsMultipleEditsThroughCaseVariantPath(t *testing.T) {
	workspace := t.TempDir()
	original := "alpha beta gamma"
	path := writeTestFile(t, workspace, "edit.txt", original)
	caseVariantPath := workspace + "/EDIT.TXT"

	originalInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("inspect original path: %v", err)
	}
	caseVariantInfo, err := os.Stat(caseVariantPath)
	if err != nil || !os.SameFile(originalInfo, caseVariantInfo) {
		t.Skip("filesystem is case-sensitive")
	}

	originalSHA256 := calculateSHA256([]byte(original))
	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{
				ID:             "edit-alpha",
				Operation:      "edit",
				Path:           "edit.txt",
				ExpectedSHA256: originalSHA256,
				Replacements: []Replacement{
					{OldText: "alpha", NewText: "A"},
				},
			},
			{
				ID:             "edit-gamma",
				Operation:      "edit",
				Path:           "EDIT.TXT",
				ExpectedSHA256: originalSHA256,
				Replacements: []Replacement{
					{OldText: "gamma", NewText: "G"},
				},
			},
		},
	}

	response := executeRequestForTest(t, workspace, request)
	if response.Status != "success" || len(response.Results) != 2 {
		t.Fatalf("unexpected response: %#v", response)
	}
	assertFileContent(t, path, "A beta G")
}

func TestExecuteRequestRejectsLaterSameFileEditWithDifferentOriginalHash(t *testing.T) {
	workspace := t.TempDir()
	original := "alpha beta gamma"
	path := writeTestFile(t, workspace, "edit.txt", original)

	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{
				ID:             "edit-alpha",
				Operation:      "edit",
				Path:           "edit.txt",
				ExpectedSHA256: calculateSHA256([]byte(original)),
				Replacements: []Replacement{
					{OldText: "alpha", NewText: "A"},
				},
			},
			{
				ID:             "edit-gamma",
				Operation:      "edit",
				Path:           "edit.txt",
				ExpectedSHA256: calculateSHA256([]byte("different original")),
				Replacements: []Replacement{
					{OldText: "gamma", NewText: "G"},
				},
			},
		},
	}

	response := executeRequestForTest(t, workspace, request)
	if response.Status != "error" || response.Error == nil || response.Error.Code != "FILE_CHANGED" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if len(response.Results) != 2 || response.Results[0].Status != "success" || response.Results[1].Status != "error" {
		t.Fatalf("unexpected action results: %#v", response.Results)
	}
	assertFileContent(t, path, "A beta gamma")
}

func TestExecuteRequestStopsAfterFirstFailure(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "first.txt", "first")
	writeTestFile(t, workspace, "later.txt", "later")

	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{ID: "first", Operation: "read", Paths: []string{"first.txt"}},
			{ID: "failure", Operation: "read", Paths: []string{"missing.txt"}},
			{ID: "later", Operation: "read", Paths: []string{"later.txt"}},
		},
	}

	responseText := ExecuteRequest(workspace, request)
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "error" {
		t.Fatalf("expected error status, got %q", response.Status)
	}
	if len(response.Results) != 2 {
		t.Fatalf("expected two executed actions, got %d", len(response.Results))
	}
	if response.Results[0].Status != "success" || response.Results[1].Status != "error" {
		t.Fatalf("unexpected action statuses: %#v", response.Results)
	}
	if response.Error == nil || response.Error.Code != "FILE_NOT_FOUND" {
		t.Fatalf("expected FILE_NOT_FOUND response error, got %#v", response.Error)
	}
}

func TestCreateErrorResponseProducesValidJSON(t *testing.T) {
	responseText := CreateErrorResponse("INVALID_REQUEST", "bad request")
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "error" || response.Error == nil || response.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestParseAndValidateRequestAcceptsFencedJSON(t *testing.T) {
	requestText := "```json\n{\"version\":\"1\",\"actions\":[{\"id\":\"find\",\"operation\":\"search\",\"query\":\"needle\"}]}\n```"

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if len(request.Actions) != 1 || request.Actions[0].ID != "find" {
		t.Fatalf("unexpected request: %#v", request)
	}
}

func TestParseAndValidateRequestAcceptsTildeFencedJSON(t *testing.T) {
	requestText := "~~~json\n{\"version\":\"1\",\"actions\":[{\"id\":\"find\",\"operation\":\"search\",\"query\":\"needle\"}]}\n~~~"

	_, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
}

func TestParseAndValidateRequestAcceptsTrailingBacktickArtifacts(t *testing.T) {
	for _, suffix := range []string{"`", "``", "```", "\n```", "\n```json", "\n~~~", "\n~~~json"} {
		t.Run(suffix, func(t *testing.T) {
			requestText := "{\"version\":\"1\",\"actions\":[{\"id\":\"find\",\"operation\":\"search\",\"query\":\"needle\"}]}" + suffix

			_, err := ParseAndValidateRequest(requestText)
			if err != nil {
				t.Fatalf("parseAndValidateRequest returned an error: %v", err)
			}
		})
	}
}

func TestParseAndValidateRequestPreservesBackticksInsideJSONStrings(t *testing.T) {
	requestText := "{\"version\":\"1\",\"actions\":[{\"id\":\"edit\",\"operation\":\"edit\",\"path\":\"README.md\",\"expectedSha256\":\"0000000000000000000000000000000000000000000000000000000000000000\",\"replacements\":[{\"oldText\":\"Use `go test`\",\"newText\":\"Use `go test ./...`\"}]}]}"

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if request.Actions[0].Replacements[0].OldText != "Use `go test`" {
		t.Fatalf("backticks inside JSON string were changed: %#v", request.Actions[0].Replacements[0])
	}
}

func TestParseAndValidateRequestRejectsExplanatoryProse(t *testing.T) {
	requestText := "Here is the request:\n{\"version\":\"1\",\"actions\":[{\"id\":\"find\",\"operation\":\"search\",\"query\":\"needle\"}]}"

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected explanatory prose to be rejected")
	}
}

func TestParseAndValidateRequestRejectsMultipleObjects(t *testing.T) {
	requestText := "{\"version\":\"1\",\"actions\":[{\"id\":\"first\",\"operation\":\"search\",\"query\":\"one\"}]}\n{\"version\":\"1\",\"actions\":[{\"id\":\"second\",\"operation\":\"search\",\"query\":\"two\"}]}"

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected multiple JSON objects to be rejected")
	}
}

func TestValidateRequestReadRangeBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		startLine int
		endLine   int
		wantError bool
	}{
		{name: "exactly one thousand lines", startLine: 1, endLine: 1000},
		{name: "one thousand lines from offset", startLine: 50, endLine: 1049},
		{name: "one thousand and one lines", startLine: 1, endLine: 1001, wantError: true},
		{name: "zero start line", startLine: 0, endLine: 1, wantError: true},
		{name: "negative start line", startLine: -1, endLine: 1, wantError: true},
		{name: "end before start", startLine: 2, endLine: 1, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRequest(Request{
				Version: protocolVersion,
				Actions: []Action{
					{ID: "read-range", Operation: "read_range", Path: "file.txt", StartLine: test.startLine, EndLine: test.endLine},
				},
			})
			if test.wantError && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateRequestRejectsUnsupportedOperationFields(t *testing.T) {
	tests := []struct {
		name   string
		action Action
	}{
		{name: "search with path", action: Action{ID: "search", Operation: "search", Query: "needle", Path: "file.txt"}},
		{name: "ranked search with paths", action: Action{ID: "ranked-search", Operation: "ranked_search", Query: "needle", Paths: []string{"file.txt"}}},
		{name: "read with query", action: Action{ID: "read", Operation: "read", Paths: []string{"file.txt"}, Query: "needle"}},
		{name: "read range with content", action: Action{ID: "read-range", Operation: "read_range", Path: "file.txt", StartLine: 1, EndLine: 1, Content: "content"}},
		{name: "edit with source", action: Action{ID: "edit", Operation: "edit", Path: "file.txt", Source: "source.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength), Replacements: []Replacement{{OldText: "old", NewText: "new"}}}},
		{name: "create with expected hash", action: Action{ID: "create", Operation: "create", Path: "file.txt", Content: "content", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "copy with path", action: Action{ID: "copy", Operation: "copy", Path: "file.txt", Source: "source.txt", Destination: "destination.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "move with replacements", action: Action{ID: "move", Operation: "move", Source: "source.txt", Destination: "destination.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength), Replacements: []Replacement{{OldText: "old", NewText: "new"}}}},
		{name: "tree with query", action: Action{ID: "tree", Operation: "tree", Path: ".", Query: "needle"}},
		{name: "inspect with content", action: Action{ID: "inspect", Operation: "inspect", Path: "file.txt", Content: "content"}},
		{name: "mkdir with expected hash", action: Action{ID: "mkdir", Operation: "mkdir", Path: "directory", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "delete with content", action: Action{ID: "delete", Operation: "delete", Path: "file.txt", Content: "content"}},
		{name: "search with range fields", action: Action{ID: "search", Operation: "search", Query: "needle", StartLine: 1, EndLine: 1}},
		{name: "tree with source", action: Action{ID: "tree", Operation: "tree", Path: ".", Source: "source.txt"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRequest(Request{Version: protocolVersion, Actions: []Action{test.action}})
			if err == nil {
				t.Fatal("expected unsupported field validation error")
			}
		})
	}
}

func TestValidateRequestRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		action Action
	}{
		{name: "search query", action: Action{ID: "search", Operation: "search"}},
		{name: "read paths", action: Action{ID: "read", Operation: "read"}},
		{name: "read empty path", action: Action{ID: "read", Operation: "read", Paths: []string{""}}},
		{name: "read range path", action: Action{ID: "read-range", Operation: "read_range", StartLine: 1, EndLine: 1}},
		{name: "edit path", action: Action{ID: "edit", Operation: "edit", ExpectedSHA256: strings.Repeat("0", sha256HexLength), Replacements: []Replacement{{OldText: "old", NewText: "new"}}}},
		{name: "edit replacements", action: Action{ID: "edit", Operation: "edit", Path: "file.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "create path", action: Action{ID: "create", Operation: "create", Content: "content"}},
		{name: "create content", action: Action{ID: "create", Operation: "create", Path: "file.txt"}},
		{name: "copy source", action: Action{ID: "copy", Operation: "copy", Destination: "destination.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "copy destination", action: Action{ID: "copy", Operation: "copy", Source: "source.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "move source", action: Action{ID: "move", Operation: "move", Destination: "destination.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "move destination", action: Action{ID: "move", Operation: "move", Source: "source.txt", ExpectedSHA256: strings.Repeat("0", sha256HexLength)}},
		{name: "tree path", action: Action{ID: "tree", Operation: "tree"}},
		{name: "inspect path", action: Action{ID: "inspect", Operation: "inspect"}},
		{name: "mkdir path", action: Action{ID: "mkdir", Operation: "mkdir"}},
		{name: "delete path", action: Action{ID: "delete", Operation: "delete"}},
		{name: "action ID", action: Action{Operation: "search", Query: "needle"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRequest(Request{Version: protocolVersion, Actions: []Action{test.action}})
			if err == nil {
				t.Fatal("expected missing field validation error")
			}
		})
	}
}

func TestValidateRequestRejectsInvalidSHA256Variants(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		action func(hash string) Action
	}{
		{name: "edit missing hash", value: "", action: func(hash string) Action {
			return Action{ID: "edit", Operation: "edit", Path: "file.txt", ExpectedSHA256: hash, Replacements: []Replacement{{OldText: "old", NewText: "new"}}}
		}},
		{name: "edit short hash", value: "abc", action: func(hash string) Action {
			return Action{ID: "edit", Operation: "edit", Path: "file.txt", ExpectedSHA256: hash, Replacements: []Replacement{{OldText: "old", NewText: "new"}}}
		}},
		{name: "copy uppercase hash", value: strings.Repeat("A", sha256HexLength), action: func(hash string) Action {
			return Action{ID: "copy", Operation: "copy", Source: "source.txt", Destination: "destination.txt", ExpectedSHA256: hash}
		}},
		{name: "move non-hex hash", value: strings.Repeat("z", sha256HexLength), action: func(hash string) Action {
			return Action{ID: "move", Operation: "move", Source: "source.txt", Destination: "destination.txt", ExpectedSHA256: hash}
		}},
		{name: "delete invalid optional hash", value: "invalid", action: func(hash string) Action {
			return Action{ID: "delete", Operation: "delete", Path: "file.txt", ExpectedSHA256: hash}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRequest(Request{Version: protocolVersion, Actions: []Action{test.action(test.value)}})
			if err == nil {
				t.Fatal("expected SHA-256 validation error")
			}
		})
	}
}

func executeRequestForTest(t *testing.T, workspace string, request Request) Response {
	t.Helper()
	responseText := ExecuteRequest(workspace, request)
	if len(responseText) > MaximumTransferChars() {
		t.Fatalf("response length = %d, want at most %d", len(responseText), MaximumTransferChars())
	}
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
