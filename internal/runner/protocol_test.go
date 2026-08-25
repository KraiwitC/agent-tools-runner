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

func TestParseAndValidateRequestUsesDefaultTransferLimit(t *testing.T) {
	requestText := `{"version":"1","actions":[{"id":"read","operation":"read","paths":["main.go"]}]}`

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if request.MaxTransferChars != defaultMaximumTransferChars {
		t.Fatalf("expected default maxTransferChars %d, got %d", defaultMaximumTransferChars, request.MaxTransferChars)
	}
}

func TestParseAndValidateRequestPreservesExplicitTransferLimit(t *testing.T) {
	requestText := `{"version":"1","maxTransferChars":64000,"actions":[{"id":"read","operation":"read","paths":["main.go"]}]}`

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if request.MaxTransferChars != 64000 {
		t.Fatalf("expected maxTransferChars 64000, got %d", request.MaxTransferChars)
	}
}

func TestParseAndValidateRequestRejectsTransferLimitBelowMinimum(t *testing.T) {
	requestText := `{"version":"1","maxTransferChars":999,"actions":[{"id":"read","operation":"read","paths":["main.go"]}]}`

	_, err := ParseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected transfer-limit validation error")
	}
}

func TestParseAndValidateRequestAcceptsLargeTransferLimit(t *testing.T) {
	requestText := `{"version":"1","maxTransferChars":5000000,"actions":[{"id":"read","operation":"read","paths":["main.go"]}]}`

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if request.MaxTransferChars != 5000000 {
		t.Fatalf("expected maxTransferChars 5000000, got %d", request.MaxTransferChars)
	}
}

func TestParseAndValidateRequestAcceptsTransferLimitMinimum(t *testing.T) {
	requestText := fmt.Sprintf(`{"version":"1","maxTransferChars":%d,"actions":[{"id":"read","operation":"read","paths":["main.go"]}]}`, minimumTransferChars)

	request, err := ParseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if request.MaxTransferChars != minimumTransferChars {
		t.Fatalf("expected maxTransferChars %d, got %d", minimumTransferChars, request.MaxTransferChars)
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

func TestExecuteRequestRejectsOversizedActionResult(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "large.txt", strings.Repeat("a", 2000))
	request := Request{
		Version:          protocolVersion,
		MaxTransferChars: minimumTransferChars,
		Actions: []Action{
			{ID: "read-large", Operation: "read", Paths: []string{"large.txt"}},
		},
	}

	responseText := ExecuteRequest(workspace, request)
	if len(responseText) > minimumTransferChars {
		t.Fatalf("expected response within %d characters, got %d", minimumTransferChars, len(responseText))
	}
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "error" || response.Error == nil || response.Error.Code != "TRANSFER_LIMIT_EXCEEDED" {
		t.Fatalf("unexpected transfer-limit response: %#v", response)
	}
	if len(response.Results) != 1 || response.Results[0].Status != "error" || response.Results[0].Data != nil {
		t.Fatalf("unexpected transfer-limit action result: %#v", response.Results)
	}
}

func TestExecuteRequestPreservesResultWithinTransferLimit(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "small.txt", "content")
	request := Request{
		Version:          protocolVersion,
		MaxTransferChars: minimumTransferChars,
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

func TestExecuteRequestReturnsMoreReadOnlyItemsWhenTransferBudgetAllows(t *testing.T) {
	workspace := t.TempDir()
	var content strings.Builder
	for index := 0; index < 150; index++ {
		fmt.Fprintf(&content, "needle executeMoveAction %03d\n", index)
		writeTestFile(t, workspace, fmt.Sprintf("tree/file-%03d.txt", index), "content")
	}
	writeTestFile(t, workspace, "matches.txt", content.String())

	tests := []struct {
		name         string
		action       Action
		minimumItems int
		resultItems  func(*ActionData) int
	}{
		{
			name:         "tree",
			action:       Action{ID: "tree", Operation: "tree", Path: "."},
			minimumItems: 151,
			resultItems:  func(data *ActionData) int { return len(data.Entries) },
		},
		{
			name:         "search",
			action:       Action{ID: "search", Operation: "search", Query: "needle"},
			minimumItems: 150,
			resultItems:  func(data *ActionData) int { return len(data.Matches) },
		},
		{
			name:         "ranked_search",
			action:       Action{ID: "ranked", Operation: "ranked_search", Query: "executeMoveAction"},
			minimumItems: 150,
			resultItems:  func(data *ActionData) int { return len(data.RankedMatches) },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := Request{
				Version:          protocolVersion,
				MaxTransferChars: 500000,
				Actions:          []Action{test.action},
			}

			response := executeRequestForTest(t, workspace, request)
			if response.Status != "success" || len(response.Results) != 1 || response.Results[0].Data == nil {
				t.Fatalf("unexpected response: %#v", response)
			}
			if response.Results[0].Data.Truncated {
				t.Fatal("did not expect result to be truncated")
			}
			if itemCount := test.resultItems(response.Results[0].Data); itemCount < test.minimumItems {
				t.Fatalf("items = %d, want at least %d", itemCount, test.minimumItems)
			}
		})
	}
}

func TestExecuteRequestTruncatesReadOnlyItemsToTransferBudget(t *testing.T) {
	workspace := t.TempDir()
	var content strings.Builder
	for index := 0; index < 200; index++ {
		fmt.Fprintf(&content, "needle result with enough text to consume transfer space %03d\n", index)
	}
	writeTestFile(t, workspace, "many.txt", content.String())
	request := Request{
		Version:          protocolVersion,
		MaxTransferChars: minimumTransferChars,
		Actions: []Action{
			{ID: "search", Operation: "search", Query: "needle"},
		},
	}

	responseText := ExecuteRequest(workspace, request)
	if len(responseText) > minimumTransferChars {
		t.Fatalf("response length = %d, want at most %d", len(responseText), minimumTransferChars)
	}
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status != "success" || len(response.Results) != 1 || response.Results[0].Data == nil {
		t.Fatalf("unexpected response: %#v", response)
	}
	data := response.Results[0].Data
	if !data.Truncated {
		t.Fatal("expected transfer-budget truncation")
	}
	if len(data.Matches) == 0 || len(data.Matches) >= 200 {
		t.Fatalf("matches = %d, want a non-empty truncated result", len(data.Matches))
	}
	for index, match := range data.Matches {
		if match.Line != index+1 {
			t.Fatalf("match %d line = %d, want %d", index, match.Line, index+1)
		}
	}
}

func TestExecuteRequestUsesRemainingTransferBudgetForLaterReadOnlyAction(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "first.txt", strings.Repeat("a", 400))
	var content strings.Builder
	for index := 0; index < 100; index++ {
		fmt.Fprintf(&content, "needle result %03d with additional transfer content\n", index)
	}
	writeTestFile(t, workspace, "many.txt", content.String())

	searchAction := Action{ID: "search", Operation: "search", Query: "needle"}
	searchOnly := executeRequestForTest(t, workspace, Request{
		Version:          protocolVersion,
		MaxTransferChars: 2000,
		Actions:          []Action{searchAction},
	})
	withPriorResult := executeRequestForTest(t, workspace, Request{
		Version:          protocolVersion,
		MaxTransferChars: 2000,
		Actions: []Action{
			{ID: "read", Operation: "read", Paths: []string{"first.txt"}},
			searchAction,
		},
	})

	if searchOnly.Status != "success" || len(searchOnly.Results) != 1 || searchOnly.Results[0].Data == nil {
		t.Fatalf("unexpected search-only response: %#v", searchOnly)
	}
	if withPriorResult.Status != "success" || len(withPriorResult.Results) != 2 || withPriorResult.Results[1].Data == nil {
		t.Fatalf("unexpected multi-action response: %#v", withPriorResult)
	}
	searchOnlyMatches := len(searchOnly.Results[0].Data.Matches)
	laterMatches := len(withPriorResult.Results[1].Data.Matches)
	if laterMatches >= searchOnlyMatches {
		t.Fatalf("later action matches = %d, want fewer than search-only matches %d", laterMatches, searchOnlyMatches)
	}
	if !withPriorResult.Results[1].Data.Truncated {
		t.Fatal("expected later search result to be truncated")
	}
}

func executeRequestForTest(t *testing.T, workspace string, request Request) Response {
	t.Helper()
	responseText := ExecuteRequest(workspace, request)
	if len(responseText) > effectiveMaximumTransferChars(request.MaxTransferChars) {
		t.Fatalf("response length = %d, want at most %d", len(responseText), effectiveMaximumTransferChars(request.MaxTransferChars))
	}
	var response Response
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}
