package app

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-tools-runner/internal/runner"
)

func TestSummarizeResponseFormatsSuccessfulActions(t *testing.T) {
	empty := true
	response := runner.Response{
		Version: "1",
		Status:  "success",
		Results: []runner.ActionResult{
			{
				ID:        "edit-file",
				Operation: "edit",
				Status:    "success",
				Data: &runner.ActionData{
					Path:                "abc.py",
					ReplacementsApplied: 2,
				},
			},
			{
				ID:        "create-file",
				Operation: "create",
				Status:    "success",
				Data: &runner.ActionData{
					Path:         "config.yaml",
					BytesWritten: 154,
				},
			},
			{
				ID:        "inspect-directory",
				Operation: "inspect",
				Status:    "success",
				Data: &runner.ActionData{
					Path:  "generated",
					Type:  "directory",
					Empty: &empty,
				},
			},
			{
				ID:        "delete-file",
				Operation: "delete",
				Status:    "success",
				Data: &runner.ActionData{
					Path: "obsolete.txt",
					Type: "file",
				},
			},
		},
	}

	responseText := marshalSummaryTestResponse(t, response)
	summary, err := summarizeResponse(responseText)
	if err != nil {
		t.Fatalf("summarizeResponse returned an error: %v", err)
	}

	expected := "SUCCESS\n\n" +
		"  EDIT      abc.py (2 replacements)\n" +
		"  CREATE    config.yaml (154 bytes)\n" +
		"  INSPECT   generated (directory, empty)\n" +
		"  DELETE    obsolete.txt (file)"
	if summary != expected {
		t.Fatalf("unexpected summary:\n%s\n\nexpected:\n%s", summary, expected)
	}
}

func TestSummarizeResponseFormatsReadSearchTreeAndRange(t *testing.T) {
	response := runner.Response{
		Version: "1",
		Status:  "success",
		Results: []runner.ActionResult{
			{
				ID:        "read-files",
				Operation: "read",
				Status:    "success",
				Data: &runner.ActionData{
					Files: []runner.ReadFileResult{
						{Path: "first.go"},
						{Path: "second.go"},
					},
				},
			},
			{
				ID:        "search-code",
				Operation: "search",
				Status:    "success",
				Data: &runner.ActionData{
					Query:     "needle",
					Matches:   []runner.SearchMatch{{Path: "main.go", Line: 5}},
					Truncated: true,
				},
			},
			{
				ID:        "read-tree",
				Operation: "tree",
				Status:    "success",
				Data: &runner.ActionData{
					Path:      ".",
					Entries:   []runner.TreeEntry{{Path: "main.go", Type: "file"}, {Path: "internal", Type: "directory"}},
					Truncated: true,
				},
			},
			{
				ID:        "read-range",
				Operation: "read_range",
				Status:    "success",
				Data: &runner.ActionData{
					Path:       "service.go",
					StartLine:  20,
					EndLine:    80,
					TotalLines: 240,
				},
			},
		},
	}

	responseText := marshalSummaryTestResponse(t, response)
	summary, err := summarizeResponse(responseText)
	if err != nil {
		t.Fatalf("summarizeResponse returned an error: %v", err)
	}

	expected := "SUCCESS\n\n" +
		"  READ      2 files\n" +
		"  SEARCH    \"needle\" (1 match), truncated\n" +
		"  TREE      . (2 entries), truncated\n" +
		"  RANGE     service.go (lines 20-80 of 240)"
	if summary != expected {
		t.Fatalf("unexpected summary:\n%s\n\nexpected:\n%s", summary, expected)
	}
}

func TestSummarizeResponseFormatsEarlierSuccessAndFailedAction(t *testing.T) {
	response := runner.Response{
		Version: "1",
		Status:  "error",
		Results: []runner.ActionResult{
			{
				ID:        "read-file",
				Operation: "read",
				Status:    "success",
				Data: &runner.ActionData{
					Files: []runner.ReadFileResult{{Path: "abc.py"}},
				},
			},
			{
				ID:        "edit-file",
				Operation: "edit",
				Status:    "error",
				Data: &runner.ActionData{
					Path: "asdf.txt",
				},
			},
		},
		Error: &runner.ResponseError{
			ActionID:    "edit-file",
			ActionIndex: 1,
			Code:        "FILE_CHANGED",
			Message:     "File content does not match expectedSha256.",
			Path:        "asdf.txt",
		},
	}

	responseText := marshalSummaryTestResponse(t, response)
	summary, err := summarizeResponse(responseText)
	if err != nil {
		t.Fatalf("summarizeResponse returned an error: %v", err)
	}

	expected := "ERROR\n\n" +
		"  READ      abc.py\n" +
		"  EDIT      asdf.txt [ERROR]\n" +
		"            FILE_CHANGED: File content does not match expectedSha256."
	if summary != expected {
		t.Fatalf("unexpected summary:\n%s\n\nexpected:\n%s", summary, expected)
	}
}

func TestSummarizeResponseFormatsRequestErrorWithoutActionResults(t *testing.T) {
	response := runner.Response{
		Version: "1",
		Status:  "error",
		Results: []runner.ActionResult{},
		Error: &runner.ResponseError{
			Code:    "INVALID_REQUEST",
			Message: "actions must contain at least one action",
		},
	}

	responseText := marshalSummaryTestResponse(t, response)
	summary, err := summarizeResponse(responseText)
	if err != nil {
		t.Fatalf("summarizeResponse returned an error: %v", err)
	}

	expected := "ERROR\n\n  INVALID_REQUEST: actions must contain at least one action"
	if summary != expected {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestSummarizeResponseRejectsMalformedJSON(t *testing.T) {
	responseText := "not-json"

	_, err := summarizeResponse(responseText)
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
	if responseText != "not-json" {
		t.Fatalf("response text was changed: %q", responseText)
	}
}

func TestSummarizeResponseDoesNotExposeFileContentOrMatchText(t *testing.T) {
	response := runner.Response{
		Version: "1",
		Status:  "success",
		Results: []runner.ActionResult{
			{
				ID:        "read-secret",
				Operation: "read",
				Status:    "success",
				Data: &runner.ActionData{
					Files: []runner.ReadFileResult{{Path: "config.txt", Content: "sensitive-content"}},
				},
			},
			{
				ID:        "search-secret",
				Operation: "search",
				Status:    "success",
				Data: &runner.ActionData{
					Query:   "token",
					Matches: []runner.SearchMatch{{Path: "config.txt", Line: 1, Text: "secret-match-text"}},
				},
			},
		},
	}

	responseText := marshalSummaryTestResponse(t, response)
	summary, err := summarizeResponse(responseText)
	if err != nil {
		t.Fatalf("summarizeResponse returned an error: %v", err)
	}
	if strings.Contains(summary, "sensitive-content") || strings.Contains(summary, "secret-match-text") {
		t.Fatalf("summary exposed file content or search-match text: %q", summary)
	}
}

func marshalSummaryTestResponse(t *testing.T, response runner.Response) string {
	t.Helper()
	content, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return string(content)
}
