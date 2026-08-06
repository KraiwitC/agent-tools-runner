package runner

import "testing"

func TestExecuteReadRangeActionReturnsRequestedLines(t *testing.T) {
	workspace := t.TempDir()
	content := "first\nsecond\nthird\nfourth\n"
	writeTestFile(t, workspace, "lines.txt", content)
	action := Action{
		ID:        "read-range",
		Operation: "read_range",
		Path:      "lines.txt",
		StartLine: 2,
		EndLine:   3,
	}

	data, responseError := executeReadRangeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeReadRangeAction returned an error: %#v", responseError)
	}
	if data.Path != "lines.txt" {
		t.Fatalf("expected path lines.txt, got %q", data.Path)
	}
	if data.Content != "second\nthird\n" {
		t.Fatalf("unexpected range content: %q", data.Content)
	}
	if data.StartLine != 2 || data.EndLine != 3 || data.TotalLines != 4 {
		t.Fatalf("unexpected range metadata: %#v", data)
	}
	if data.SHA256 != calculateSHA256([]byte(content)) {
		t.Fatalf("unexpected complete-file SHA-256: %q", data.SHA256)
	}
}

func TestExecuteReadRangeActionPreservesFinalLineWithoutNewline(t *testing.T) {
	workspace := t.TempDir()
	content := "first\nsecond"
	writeTestFile(t, workspace, "lines.txt", content)
	action := Action{
		ID:        "read-range",
		Operation: "read_range",
		Path:      "lines.txt",
		StartLine: 2,
		EndLine:   2,
	}

	data, responseError := executeReadRangeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeReadRangeAction returned an error: %#v", responseError)
	}
	if data.Content != "second" {
		t.Fatalf("unexpected final-line content: %q", data.Content)
	}
	if data.TotalLines != 2 {
		t.Fatalf("expected two total lines, got %d", data.TotalLines)
	}
}

func TestExecuteReadRangeActionClampsEndLineToFile(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "lines.txt", "first\nsecond\nthird\n")
	action := Action{
		ID:        "read-range",
		Operation: "read_range",
		Path:      "lines.txt",
		StartLine: 2,
		EndLine:   10,
	}

	data, responseError := executeReadRangeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeReadRangeAction returned an error: %#v", responseError)
	}
	if data.Content != "second\nthird\n" || data.EndLine != 3 || data.TotalLines != 3 {
		t.Fatalf("unexpected clamped range: %#v", data)
	}
}

func TestExecuteReadRangeActionRejectsStartBeyondFile(t *testing.T) {
	workspace := t.TempDir()
	content := "first\nsecond\n"
	writeTestFile(t, workspace, "lines.txt", content)
	action := Action{
		ID:        "read-range",
		Operation: "read_range",
		Path:      "lines.txt",
		StartLine: 3,
		EndLine:   3,
	}

	data, responseError := executeReadRangeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "RANGE_OUT_OF_BOUNDS")
	if data.TotalLines != 2 || data.SHA256 != calculateSHA256([]byte(content)) {
		t.Fatalf("unexpected out-of-bounds metadata: %#v", data)
	}
}

func TestExecuteReadRangeActionRejectsEmptyFile(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "empty.txt", "")
	action := Action{
		ID:        "read-range",
		Operation: "read_range",
		Path:      "empty.txt",
		StartLine: 1,
		EndLine:   1,
	}

	data, responseError := executeReadRangeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "RANGE_OUT_OF_BOUNDS")
	if data.TotalLines != 0 || data.SHA256 != calculateSHA256(nil) {
		t.Fatalf("unexpected empty-file metadata: %#v", data)
	}
}

func TestExecuteReadRangeActionRejectsOutsidePath(t *testing.T) {
	workspace := t.TempDir()
	action := Action{
		ID:        "read-range",
		Operation: "read_range",
		Path:      "..",
		StartLine: 1,
		EndLine:   1,
	}

	_, responseError := executeReadRangeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}
