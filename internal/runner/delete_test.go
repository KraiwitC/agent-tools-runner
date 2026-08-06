package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteDeleteActionDeletesFileWithMatchingHash(t *testing.T) {
	workspace := t.TempDir()
	content := "delete me"
	path := writeTestFile(t, workspace, "delete.txt", content)
	action := Action{
		ID:             "delete-file",
		Operation:      "delete",
		Path:           "delete.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	relativePath, pathType, responseError := executeDeleteAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeDeleteAction returned an error: %#v", responseError)
	}
	if relativePath != "delete.txt" || pathType != "file" {
		t.Fatalf("unexpected delete result: path=%q type=%q", relativePath, pathType)
	}
	assertPathDoesNotExist(t, path)
}

func TestExecuteDeleteActionRejectsFileWithoutHash(t *testing.T) {
	workspace := t.TempDir()
	path := writeTestFile(t, workspace, "keep.txt", "content")
	action := Action{ID: "delete-file", Operation: "delete", Path: "keep.txt"}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "INVALID_REQUEST")
	assertFileContent(t, path, "content")
}

func TestExecuteDeleteActionRejectsChangedFile(t *testing.T) {
	workspace := t.TempDir()
	path := writeTestFile(t, workspace, "keep.txt", "current")
	action := Action{
		ID:             "delete-file",
		Operation:      "delete",
		Path:           "keep.txt",
		ExpectedSHA256: calculateSHA256([]byte("stale")),
	}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_CHANGED")
	assertFileContent(t, path, "current")
}

func TestExecuteDeleteActionDeletesEmptyDirectory(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "empty")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create empty directory: %v", err)
	}
	action := Action{ID: "delete-directory", Operation: "delete", Path: "empty"}

	relativePath, pathType, responseError := executeDeleteAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeDeleteAction returned an error: %#v", responseError)
	}
	if relativePath != "empty" || pathType != "directory" {
		t.Fatalf("unexpected delete result: path=%q type=%q", relativePath, pathType)
	}
	assertPathDoesNotExist(t, path)
}

func TestExecuteDeleteActionRejectsNonEmptyDirectory(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "directory/file.txt", "content")
	action := Action{ID: "delete-directory", Operation: "delete", Path: "directory"}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "DIRECTORY_NOT_EMPTY")
	assertFileContent(t, filepath.Join(workspace, "directory", "file.txt"), "content")
}

func TestExecuteDeleteActionRejectsDirectoryHash(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "empty")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create empty directory: %v", err)
	}
	action := Action{
		ID:             "delete-directory",
		Operation:      "delete",
		Path:           "empty",
		ExpectedSHA256: calculateSHA256(nil),
	}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "INVALID_REQUEST")
	fileInfo, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("inspect preserved directory: %v", err)
	}
	if !fileInfo.IsDir() {
		t.Fatal("expected directory to remain")
	}
}

func TestExecuteDeleteActionRejectsWorkspaceRoot(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "delete-directory", Operation: "delete", Path: "."}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "UNSUPPORTED_FILE")
}

func TestExecuteDeleteActionRejectsOutsidePath(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "delete-path", Operation: "delete", Path: ".."}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}

func TestExecuteDeleteActionRejectsMissingPath(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "delete-path", Operation: "delete", Path: "missing.txt"}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_NOT_FOUND")
}

func TestExecuteDeleteActionRejectsSymbolicLink(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "target.txt", "content")
	linkPath := filepath.Join(workspace, "linked.txt")
	if err := os.Symlink(filepath.Join(workspace, "target.txt"), linkPath); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{
		ID:             "delete-file",
		Operation:      "delete",
		Path:           "linked.txt",
		ExpectedSHA256: calculateSHA256([]byte("content")),
	}

	_, _, responseError := executeDeleteAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
	assertFileContent(t, filepath.Join(workspace, "target.txt"), "content")
}
