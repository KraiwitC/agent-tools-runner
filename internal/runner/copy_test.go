package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecuteCopyActionCopiesFileAndPreservesSource(t *testing.T) {
	workspace := t.TempDir()
	content := "copy me\n"
	sourcePath := writeTestFile(t, workspace, "source.txt", content)
	action := Action{
		ID:             "copy-file",
		Operation:      "copy",
		Source:         "source.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	bytesWritten, destinationPath, copiedSHA256, responseError := executeCopyAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeCopyAction returned an error: %#v", responseError)
	}
	if destinationPath != "destination.txt" {
		t.Fatalf("destination path = %q, want %q", destinationPath, "destination.txt")
	}
	if bytesWritten != len([]byte(content)) {
		t.Fatalf("bytes written = %d, want %d", bytesWritten, len([]byte(content)))
	}
	if copiedSHA256 != action.ExpectedSHA256 {
		t.Fatalf("SHA-256 = %q, want %q", copiedSHA256, action.ExpectedSHA256)
	}
	assertFileContent(t, sourcePath, content)
	assertFileContent(t, filepath.Join(workspace, "destination.txt"), content)
}

func TestExecuteCopyActionRejectsChangedSource(t *testing.T) {
	workspace := t.TempDir()
	sourcePath := writeTestFile(t, workspace, "source.txt", "current")
	action := Action{
		ID:             "copy-file",
		Operation:      "copy",
		Source:         "source.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte("stale")),
	}

	_, _, _, responseError := executeCopyAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_CHANGED")
	assertFileContent(t, sourcePath, "current")
	assertPathDoesNotExist(t, filepath.Join(workspace, "destination.txt"))
}

func TestExecuteCopyActionRejectsExistingDestination(t *testing.T) {
	workspace := t.TempDir()
	content := "source"
	sourcePath := writeTestFile(t, workspace, "source.txt", content)
	destinationPath := writeTestFile(t, workspace, "destination.txt", "existing")
	action := Action{
		ID:             "copy-file",
		Operation:      "copy",
		Source:         "source.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	_, _, _, responseError := executeCopyAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_ALREADY_EXISTS")
	assertFileContent(t, sourcePath, content)
	assertFileContent(t, destinationPath, "existing")
}

func TestExecuteCopyActionRejectsMissingDestinationParent(t *testing.T) {
	workspace := t.TempDir()
	content := "source"
	sourcePath := writeTestFile(t, workspace, "source.txt", content)
	action := Action{
		ID:             "copy-file",
		Operation:      "copy",
		Source:         "source.txt",
		Destination:    "missing/destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	_, _, _, responseError := executeCopyAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PARENT_DIRECTORY_NOT_FOUND")
	assertFileContent(t, sourcePath, content)
	assertPathDoesNotExist(t, filepath.Join(workspace, "missing", "destination.txt"))
}

func TestExecuteCopyActionRejectsSymbolicLinkSource(t *testing.T) {
	workspace := t.TempDir()
	content := "source"
	writeTestFile(t, workspace, "source.txt", content)
	linkPath := filepath.Join(workspace, "linked.txt")
	if err := os.Symlink(filepath.Join(workspace, "source.txt"), linkPath); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{
		ID:             "copy-file",
		Operation:      "copy",
		Source:         "linked.txt",
		Destination:    "destination.txt",
		ExpectedSHA256: calculateSHA256([]byte(content)),
	}

	_, _, _, responseError := executeCopyAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
	assertFileContent(t, filepath.Join(workspace, "source.txt"), content)
	assertPathDoesNotExist(t, filepath.Join(workspace, "destination.txt"))
}
