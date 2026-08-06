package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecuteEditActionReplacesUniqueText(t *testing.T) {
	workspace := t.TempDir()
	original := "first\nold value\nlast\n"
	updated := "first\nnew value\nlast\n"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte(original)),
		Replacements: []Replacement{
			{OldText: "old value", NewText: "new value"},
		},
	}

	applied, updatedSHA256, responseError := executeEditAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeEditAction returned an error: %#v", responseError)
	}
	if applied != 1 {
		t.Fatalf("expected one replacement, got %d", applied)
	}
	if updatedSHA256 != calculateSHA256([]byte(updated)) {
		t.Fatalf("unexpected updated SHA-256: %q", updatedSHA256)
	}
	assertFileContent(t, path, updated)
	assertNoTemporaryEditFiles(t, workspace)
}

func TestExecuteEditActionAppliesMultipleNonOverlappingReplacements(t *testing.T) {
	workspace := t.TempDir()
	original := "alpha beta gamma"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte(original)),
		Replacements: []Replacement{
			{OldText: "alpha", NewText: "A"},
			{OldText: "gamma", NewText: "G"},
		},
	}

	applied, _, responseError := executeEditAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeEditAction returned an error: %#v", responseError)
	}
	if applied != 2 {
		t.Fatalf("expected two replacements, got %d", applied)
	}
	assertFileContent(t, path, "A beta G")
}

func TestExecuteEditActionAllowsEmptyNewText(t *testing.T) {
	workspace := t.TempDir()
	original := "keep remove keep"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte(original)),
		Replacements: []Replacement{
			{OldText: " remove", NewText: ""},
		},
	}

	_, _, responseError := executeEditAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeEditAction returned an error: %#v", responseError)
	}
	assertFileContent(t, path, "keep keep")
}

func TestExecuteEditActionRejectsMissingTargetWithoutChangingFile(t *testing.T) {
	workspace := t.TempDir()
	original := "original content"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte(original)),
		Replacements: []Replacement{
			{OldText: "missing", NewText: "replacement"},
		},
	}

	_, _, responseError := executeEditAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "EDIT_TARGET_NOT_FOUND")
	assertFileContent(t, path, original)
	assertNoTemporaryEditFiles(t, workspace)
}

func TestExecuteEditActionRejectsNonUniqueTargetWithoutChangingFile(t *testing.T) {
	workspace := t.TempDir()
	original := "same and same"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte(original)),
		Replacements: []Replacement{
			{OldText: "same", NewText: "changed"},
		},
	}

	_, _, responseError := executeEditAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "EDIT_TARGET_NOT_UNIQUE")
	assertFileContent(t, path, original)
}

func TestExecuteEditActionRejectsOverlappingTargetsWithoutChangingFile(t *testing.T) {
	workspace := t.TempDir()
	original := "abcdef"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte(original)),
		Replacements: []Replacement{
			{OldText: "abcd", NewText: "one"},
			{OldText: "cdef", NewText: "two"},
		},
	}

	_, _, responseError := executeEditAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "EDIT_TARGET_OVERLAP")
	assertFileContent(t, path, original)
}

func TestExecuteEditActionPreservesFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits consistently")
	}

	workspace := t.TempDir()
	path := writeTestFile(t, workspace, "edit.txt", "old")
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("set test permissions: %v", err)
	}

	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte("old")),
		Replacements: []Replacement{
			{OldText: "old", NewText: "new"},
		},
	}

	_, _, responseError := executeEditAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeEditAction returned an error: %#v", responseError)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("inspect edited file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o640 {
		t.Fatalf("expected permissions 0640, got %04o", fileInfo.Mode().Perm())
	}
}

func TestExecuteEditActionRejectsChangedFile(t *testing.T) {
	workspace := t.TempDir()
	path := writeTestFile(t, workspace, "edit.txt", "current")
	action := Action{
		ID:             "edit-file",
		Operation:      "edit",
		Path:           "edit.txt",
		ExpectedSHA256: calculateSHA256([]byte("stale")),
		Replacements: []Replacement{
			{OldText: "current", NewText: "changed"},
		},
	}

	_, _, responseError := executeEditAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_CHANGED")
	assertFileContent(t, path, "current")
}

func assertFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != expected {
		t.Fatalf("expected %q, got %q", expected, string(content))
	}
}

func assertResponseErrorCode(t *testing.T, responseError *ResponseError, expectedCode string) {
	t.Helper()
	if responseError == nil {
		t.Fatalf("expected response error %s", expectedCode)
	}
	if responseError.Code != expectedCode {
		t.Fatalf("expected code %s, got %s", expectedCode, responseError.Code)
	}
}

func assertNoTemporaryEditFiles(t *testing.T, workspace string) {
	t.Helper()
	entries, err := os.ReadDir(workspace)
	if err != nil {
		t.Fatalf("read workspace directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".atr-edit-") {
			t.Fatalf("temporary edit file was not removed: %s", filepath.Join(workspace, entry.Name()))
		}
	}
}
