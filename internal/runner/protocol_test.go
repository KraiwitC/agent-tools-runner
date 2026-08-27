package runner

import (
	"encoding/json"
	"fmt"
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

func TestParseAndValidateRequestRejectsShortRankedSearchQuery(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"ranked-search","operation":"ranked_search","query":"a"}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected short ranked_search query validation error")
	}
}

func TestParseAndValidateRequestRejectsRankedSearchWithoutLettersOrDigits(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"ranked-search","operation":"ranked_search","query":"_-"}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected ranked_search query validation error")
	}
}

func TestParseAndValidateRequestRejectsRankedSearchPath(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"ranked-search","operation":"ranked_search","query":"move","path":"move.go"}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected unsupported ranked_search path validation error")
	}
}

func TestParseAndValidateRequestRejectsEmptyCreateContent(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"create","operation":"create","path":"created.txt","content":""}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected empty create content validation error")
	}
}

func TestParseAndValidateRequestRejectsCreateReplacements(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"create","operation":"create","path":"created.txt","content":"content","replacements":[{"oldText":"old","newText":"new"}]}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected create replacements validation error")
	}
}

func TestParseAndValidateRequestRejectsCopyWithoutDestination(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"copy","operation":"copy","source":"source.txt","expectedSha256":"0000000000000000000000000000000000000000000000000000000000000000"}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected missing copy destination validation error")
	}
}

func TestParseAndValidateRequestRejectsMoveWithoutSHA256(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"move","operation":"move","source":"source.txt","destination":"moved.txt"}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected missing move SHA-256 validation error")
	}
}

func TestParseAndValidateRequestRejectsEditContent(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"edit","operation":"edit","path":"main.go","content":"content","expectedSha256":"0000000000000000000000000000000000000000000000000000000000000000","replacements":[{"oldText":"old","newText":"new"}]}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected edit content validation error")
	}
}

func TestParseAndValidateRequestRejectsEmptyEditOldText(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"edit","operation":"edit","path":"main.go","expectedSha256":"0000000000000000000000000000000000000000000000000000000000000000","replacements":[{"oldText":"","newText":"new"}]}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected empty edit oldText validation error")
	}
}

func TestParseAndValidateRequestRejectsMissingEditSHA256(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"edit","operation":"edit","path":"main.go","replacements":[{"oldText":"old","newText":"new"}]}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected missing edit SHA-256 validation error")
	}
}

func TestParseAndValidateRequestRejectsInvalidEditSHA256(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"edit","operation":"edit","path":"main.go","expectedSha256":"INVALID","replacements":[{"oldText":"old","newText":"new"}]}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected invalid edit SHA-256 validation error")
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
