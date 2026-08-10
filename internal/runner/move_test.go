package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteMoveActionMovesFile(t *testing.T) {
	workspace := t.TempDir()
	content := "move me\n"
	sourcePath := writeTestFile(t, workspace, "source.txt", content)
	action := Action{
		ID:             "move-file",
		Operation:      "move",
		Source:         "source.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	bytesWritten, destinationPath, movedSHA256, responseError := executeMoveAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeMoveAction returned an error: %#v", responseError)
	}
	if destinationPath != "destination.txt" {
		t.Fatalf("destination path = %q, want %q", destinationPath, "destination.txt")
	}
	if bytesWritten != len([]byte(content)) {
		t.Fatalf("bytes written = %d, want %d", bytesWritten, len([]byte(content)))
	}
	if movedSHA256 != action.ExpectedSHA256 {
		t.Fatalf("SHA-256 = %q, want %q", movedSHA256, action.ExpectedSHA256)
	}
	assertPathDoesNotExist(t, sourcePath)
	assertFileContent(t, filepath.Join(workspace, "destination.txt"), content)
}

func TestExecuteMoveActionRenamesFile(t *testing.T) {
	workspace := t.TempDir()
	content := "rename me"
	sourcePath := writeTestFile(t, workspace, "before.txt", content)
	action := Action{
		ID:             "rename-file",
		Operation:      "move",
		Source:         "before.txt",
		Destination:    "after.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	_, destinationPath, _, responseError := executeMoveAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeMoveAction returned an error: %#v", responseError)
	}
	if destinationPath != "after.txt" {
		t.Fatalf("destination path = %q, want %q", destinationPath, "after.txt")
	}
	assertPathDoesNotExist(t, sourcePath)
	assertFileContent(t, filepath.Join(workspace, "after.txt"), content)
}

func TestExecuteMoveActionRejectsChangedSource(t *testing.T) {
	workspace := t.TempDir()
	sourcePath := writeTestFile(t, workspace, "source.txt", "current")
	action := Action{
		ID:             "move-file",
		Operation:      "move",
		Source:         "source.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte("stale")),
	}

	_, _, _, responseError := executeMoveAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_CHANGED")
	assertFileContent(t, sourcePath, "current")
	assertPathDoesNotExist(t, filepath.Join(workspace, "destination.txt"))
}

func TestExecuteMoveActionRejectsExistingDestination(t *testing.T) {
	workspace := t.TempDir()
	content := "source"
	sourcePath := writeTestFile(t, workspace, "source.txt", content)
	destinationPath := writeTestFile(t, workspace, "destination.txt", "existing")
	action := Action{
		ID:             "move-file",
		Operation:      "move",
		Source:         "source.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	_, _, _, responseError := executeMoveAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_ALREADY_EXISTS")
	assertFileContent(t, sourcePath, content)
	assertFileContent(t, destinationPath, "existing")
}

func TestExecuteMoveActionRejectsMissingDestinationParent(t *testing.T) {
	workspace := t.TempDir()
	content := "source"
	sourcePath := writeTestFile(t, workspace, "source.txt", content)
	action := Action{
		ID:             "move-file",
		Operation:      "move",
		Source:         "source.txt",
		Destination:    "missing/destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	_, _, _, responseError := executeMoveAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PARENT_DIRECTORY_NOT_FOUND")
	assertFileContent(t, sourcePath, content)
	assertPathDoesNotExist(t, filepath.Join(workspace, "missing", "destination.txt"))
}

func TestExecuteMoveActionRejectsSymbolicLinkSource(t *testing.T) {
	workspace := t.TempDir()
	content := "source"
	writeTestFile(t, workspace, "source.txt", content)
	linkPath := filepath.Join(workspace, "linked.txt")
	if err := os.Symlink(filepath.Join(workspace, "source.txt"), linkPath); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{
		ID:             "move-file",
		Operation:      "move",
		Source:         "linked.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	_, _, _, responseError := executeMoveAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
	assertFileContent(t, filepath.Join(workspace, "source.txt"), content)
	assertPathDoesNotExist(t, filepath.Join(workspace, "destination.txt"))
}
