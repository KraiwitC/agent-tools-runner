package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteMkdirActionCreatesOneDirectory(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-directory", Operation: "mkdir", Path: "generated"}

	relativePath, responseError := executeMkdirAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeMkdirAction returned an error: %#v", responseError)
	}
	if relativePath != "generated" {
		t.Fatalf("expected path generated, got %q", relativePath)
	}
	fileInfo, err := os.Lstat(filepath.Join(workspace, "generated"))
	if err != nil {
		t.Fatalf("inspect created directory: %v", err)
	}
	if !fileInfo.IsDir() {
		t.Fatal("expected created path to be a directory")
	}
}

func TestExecuteMkdirActionCreatesDirectoryUnderExistingParent(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "parent"), 0o700); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	action := Action{ID: "create-directory", Operation: "mkdir", Path: filepath.Join("parent", "child")}

	relativePath, responseError := executeMkdirAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeMkdirAction returned an error: %#v", responseError)
	}
	if relativePath != "parent/child" {
		t.Fatalf("expected forward-slash path, got %q", relativePath)
	}
}

func TestExecuteMkdirActionRejectsExistingDirectory(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "existing"), 0o700); err != nil {
		t.Fatalf("create existing directory: %v", err)
	}
	action := Action{ID: "create-directory", Operation: "mkdir", Path: "existing"}

	_, responseError := executeMkdirAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_ALREADY_EXISTS")
}

func TestExecuteMkdirActionRejectsExistingFile(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "existing.txt", "content")
	action := Action{ID: "create-directory", Operation: "mkdir", Path: "existing.txt"}

	_, responseError := executeMkdirAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_ALREADY_EXISTS")
}

func TestExecuteMkdirActionRejectsMissingParent(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-directory", Operation: "mkdir", Path: filepath.Join("missing", "child")}

	_, responseError := executeMkdirAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PARENT_DIRECTORY_NOT_FOUND")
	assertPathDoesNotExist(t, filepath.Join(workspace, "missing"))
}

func TestExecuteMkdirActionRejectsWorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-directory", Operation: "mkdir", Path: "."}

	_, responseError := executeMkdirAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "UNSUPPORTED_FILE")
}

func TestExecuteMkdirActionRejectsOutsidePath(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-directory", Operation: "mkdir", Path: filepath.Join("..", "outside")}

	_, responseError := executeMkdirAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}

func TestExecuteMkdirActionRejectsSymbolicLinkParent(t *testing.T) {
	workspace := t.TempDir()
	realDirectory := filepath.Join(workspace, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatalf("create real directory: %v", err)
	}
	if err := os.Symlink(realDirectory, filepath.Join(workspace, "linked")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{ID: "create-directory", Operation: "mkdir", Path: filepath.Join("linked", "child")}

	_, responseError := executeMkdirAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
	assertPathDoesNotExist(t, filepath.Join(realDirectory, "child"))
}
