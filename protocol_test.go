package main

import (
	"encoding/json"
	"testing"
)

func TestParseAndValidateRequestAcceptsValidBatch(t *testing.T) {
	requestText := `{
		"version": "1",
		"actions": [
			{"id": "search", "operation": "search", "query": "needle"},
			{"id": "read", "operation": "read", "paths": ["main.go"]},
			{"id": "edit", "operation": "edit", "path": "main.go", "replacements": [{"oldText": "old", "newText": "new"}]}
		]
	}`

	request, err := parseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if len(request.Actions) != 3 {
		t.Fatalf("expected three actions, got %d", len(request.Actions))
	}
}

func TestParseAndValidateRequestRejectsUnknownField(t *testing.T) {
	requestText := `{"version":"1","actions":[],"status":"success"}`

	_, err := parseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected unknown field validation error")
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

	_, err := parseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected duplicate action ID validation error")
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

	responseText := executeRequest(workspace, request)
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
	responseText := createErrorResponse("INVALID_REQUEST", "bad request")
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

	request, err := parseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if len(request.Actions) != 1 || request.Actions[0].ID != "find" {
		t.Fatalf("unexpected request: %#v", request)
	}
}

func TestParseAndValidateRequestAcceptsTildeFencedJSON(t *testing.T) {
	requestText := "~~~json\n{\"version\":\"1\",\"actions\":[{\"id\":\"find\",\"operation\":\"search\",\"query\":\"needle\"}]}\n~~~"

	_, err := parseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
}

func TestParseAndValidateRequestAcceptsTrailingBacktickArtifacts(t *testing.T) {
	for _, suffix := range []string{"`", "``", "```"} {
		t.Run(suffix, func(t *testing.T) {
			requestText := "{\"version\":\"1\",\"actions\":[{\"id\":\"find\",\"operation\":\"search\",\"query\":\"needle\"}]}" + suffix

			_, err := parseAndValidateRequest(requestText)
			if err != nil {
				t.Fatalf("parseAndValidateRequest returned an error: %v", err)
			}
		})
	}
}

func TestParseAndValidateRequestPreservesBackticksInsideJSONStrings(t *testing.T) {
	requestText := "{\"version\":\"1\",\"actions\":[{\"id\":\"edit\",\"operation\":\"edit\",\"path\":\"README.md\",\"replacements\":[{\"oldText\":\"Use `go test`\",\"newText\":\"Use `go test ./...`\"}]}]}"

	request, err := parseAndValidateRequest(requestText)
	if err != nil {
		t.Fatalf("parseAndValidateRequest returned an error: %v", err)
	}
	if request.Actions[0].Replacements[0].OldText != "Use `go test`" {
		t.Fatalf("backticks inside JSON string were changed: %#v", request.Actions[0].Replacements[0])
	}
}

func TestParseAndValidateRequestRejectsExplanatoryProse(t *testing.T) {
	requestText := "Here is the request:\n{\"version\":\"1\",\"actions\":[{\"id\":\"find\",\"operation\":\"search\",\"query\":\"needle\"}]}"

	_, err := parseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected explanatory prose to be rejected")
	}
}

func TestParseAndValidateRequestRejectsMultipleObjects(t *testing.T) {
	requestText := "{\"version\":\"1\",\"actions\":[{\"id\":\"first\",\"operation\":\"search\",\"query\":\"one\"}]}\n{\"version\":\"1\",\"actions\":[{\"id\":\"second\",\"operation\":\"search\",\"query\":\"two\"}]}"

	_, err := parseAndValidateRequest(requestText)
	if err == nil {
		t.Fatal("expected multiple JSON objects to be rejected")
	}
}
