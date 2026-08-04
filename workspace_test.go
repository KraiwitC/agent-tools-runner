package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWorkspaceFile(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "notes.txt", "first line\nsecond line\n")

	result, err := readWorkspaceFile(workspace, "notes.txt")
	if err != nil {
		t.Fatalf("readWorkspaceFile returned an error: %v", err)
	}
	if result.Path != "notes.txt" {
		t.Fatalf("expected path notes.txt, got %q", result.Path)
	}
	if result.Content != "first line\nsecond line\n" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestExecuteReadActionPreservesEarlierFilesOnFailure(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "first.txt", "first")

	action := Action{
		ID:        "read-files",
		Operation: "read",
		Paths:     []string{"first.txt", "missing.txt", "not-read.txt"},
	}

	files, responseError := executeReadAction(workspace, action, 2)
	if responseError == nil {
		t.Fatal("expected a response error")
	}
	if responseError.Code != "FILE_NOT_FOUND" {
		t.Fatalf("expected FILE_NOT_FOUND, got %q", responseError.Code)
	}
	if responseError.ActionIndex != 2 {
		t.Fatalf("expected action index 2, got %d", responseError.ActionIndex)
	}
	if len(files) != 1 || files[0].Path != "first.txt" {
		t.Fatalf("expected only first.txt to be preserved, got %#v", files)
	}
}

func TestReadWorkspaceFileRejectsOutsidePath(t *testing.T) {
	workspace := t.TempDir()
	outsideDirectory := t.TempDir()
	outsidePath := filepath.Join(outsideDirectory, "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	_, err := readWorkspaceFile(workspace, filepath.Join("..", filepath.Base(outsideDirectory), "outside.txt"))
	assertWorkspaceErrorCode(t, err, "PATH_OUTSIDE_WORKSPACE")
}

func TestReadWorkspaceFileRejectsAbsolutePath(t *testing.T) {
	workspace := t.TempDir()
	absolutePath := filepath.Join(workspace, "absolute.txt")
	writeTestFile(t, workspace, "absolute.txt", "content")

	_, err := readWorkspaceFile(workspace, absolutePath)
	assertWorkspaceErrorCode(t, err, "PATH_OUTSIDE_WORKSPACE")
}

func TestReadWorkspaceFileRejectsDirectory(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "directory"), 0o700); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	_, err := readWorkspaceFile(workspace, "directory")
	assertWorkspaceErrorCode(t, err, "UNSUPPORTED_FILE")
}

func TestReadWorkspaceFileRejectsOversizedFile(t *testing.T) {
	workspace := t.TempDir()
	content := strings.Repeat("a", int(maximumFileSize)+1)
	writeTestFile(t, workspace, "large.txt", content)

	_, err := readWorkspaceFile(workspace, "large.txt")
	assertWorkspaceErrorCode(t, err, "FILE_TOO_LARGE")
}

func TestReadWorkspaceFileRejectsBinaryFile(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "binary.dat")
	if err := os.WriteFile(path, []byte{'a', 0, 'b'}, 0o600); err != nil {
		t.Fatalf("write binary file: %v", err)
	}

	_, err := readWorkspaceFile(workspace, "binary.dat")
	assertWorkspaceErrorCode(t, err, "UNSUPPORTED_FILE")
}

func TestReadWorkspaceFileRejectsSymlink(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "target.txt", "target")
	linkPath := filepath.Join(workspace, "link.txt")
	if err := os.Symlink(filepath.Join(workspace, "target.txt"), linkPath); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}

	_, err := readWorkspaceFile(workspace, "link.txt")
	assertWorkspaceErrorCode(t, err, "SYMLINK_NOT_SUPPORTED")
}

func writeTestFile(t *testing.T, workspace string, relativePath string, content string) string {
	t.Helper()
	path := filepath.Join(workspace, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}

func assertWorkspaceErrorCode(t *testing.T, err error, expectedCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected workspace error %s", expectedCode)
	}

	var pathError *workspaceError
	if !errors.As(err, &pathError) {
		t.Fatalf("expected workspaceError, got %T: %v", err, err)
	}
	if pathError.Code != expectedCode {
		t.Fatalf("expected code %s, got %s", expectedCode, pathError.Code)
	}
}
