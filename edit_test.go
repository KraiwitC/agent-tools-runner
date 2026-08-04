package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecuteEditActionReplacesUniqueText(t *testing.T) {
	workspace := t.TempDir()
	path := writeTestFile(t, workspace, "edit.txt", "first\nold value\nlast\n")

	action := Action{
		ID:        "edit-file",
		Operation: "edit",
		Path:      "edit.txt",
		Replacements: []Replacement{
			{OldText: "old value", NewText: "new value"},
		},
	}

	applied, responseError := executeEditAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeEditAction returned an error: %#v", responseError)
	}
	if applied != 1 {
		t.Fatalf("expected one replacement, got %d", applied)
	}
	assertFileContent(t, path, "first\nnew value\nlast\n")
	assertNoTemporaryEditFiles(t, workspace)
}

func TestExecuteEditActionAppliesMultipleNonOverlappingReplacements(t *testing.T) {
	workspace := t.TempDir()
	path := writeTestFile(t, workspace, "edit.txt", "alpha beta gamma")

	action := Action{
		ID:        "edit-file",
		Operation: "edit",
		Path:      "edit.txt",
		Replacements: []Replacement{
			{OldText: "alpha", NewText: "A"},
			{OldText: "gamma", NewText: "G"},
		},
	}

	applied, responseError := executeEditAction(workspace, action, 0)
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
	path := writeTestFile(t, workspace, "edit.txt", "keep remove keep")

	action := Action{
		ID:        "edit-file",
		Operation: "edit",
		Path:      "edit.txt",
		Replacements: []Replacement{
			{OldText: " remove", NewText: ""},
		},
	}

	_, responseError := executeEditAction(workspace, action, 0)
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
		ID:        "edit-file",
		Operation: "edit",
		Path:      "edit.txt",
		Replacements: []Replacement{
			{OldText: "missing", NewText: "replacement"},
		},
	}

	_, responseError := executeEditAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "EDIT_TARGET_NOT_FOUND")
	assertFileContent(t, path, original)
	assertNoTemporaryEditFiles(t, workspace)
}

func TestExecuteEditActionRejectsNonUniqueTargetWithoutChangingFile(t *testing.T) {
	workspace := t.TempDir()
	original := "same and same"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:        "edit-file",
		Operation: "edit",
		Path:      "edit.txt",
		Replacements: []Replacement{
			{OldText: "same", NewText: "changed"},
		},
	}

	_, responseError := executeEditAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "EDIT_TARGET_NOT_UNIQUE")
	assertFileContent(t, path, original)
}

func TestExecuteEditActionRejectsOverlappingTargetsWithoutChangingFile(t *testing.T) {
	workspace := t.TempDir()
	original := "abcdef"
	path := writeTestFile(t, workspace, "edit.txt", original)

	action := Action{
		ID:        "edit-file",
		Operation: "edit",
		Path:      "edit.txt",
		Replacements: []Replacement{
			{OldText: "abcd", NewText: "one"},
			{OldText: "cdef", NewText: "two"},
		},
	}

	_, responseError := executeEditAction(workspace, action, 0)
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
		ID:        "edit-file",
		Operation: "edit",
		Path:      "edit.txt",
		Replacements: []Replacement{
			{OldText: "old", NewText: "new"},
		},
	}

	_, responseError := executeEditAction(workspace, action, 0)
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
