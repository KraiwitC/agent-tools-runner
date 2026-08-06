package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecuteCreateActionCreatesExactUTF8Content(t *testing.T) {
	workspace := t.TempDir()
	content := "hello \"ATR\"\nสวัสดี\n"
	action := Action{ID: "create-file", Operation: "create", Path: "created.txt", Content: content}

	bytesWritten, relativePath, responseError := executeCreateAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeCreateAction returned an error: %#v", responseError)
	}
	if relativePath != "created.txt" {
		t.Fatalf("expected path created.txt, got %q", relativePath)
	}
	if bytesWritten != len([]byte(content)) {
		t.Fatalf("expected %d bytes written, got %d", len([]byte(content)), bytesWritten)
	}
	assertFileContent(t, filepath.Join(workspace, "created.txt"), content)
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteCreateActionReturnsForwardSlashPath(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "nested"), 0o700); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	action := Action{ID: "create-file", Operation: "create", Path: filepath.Join("nested", "created.txt"), Content: "content"}

	_, relativePath, responseError := executeCreateAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeCreateAction returned an error: %#v", responseError)
	}
	if relativePath != "nested/created.txt" {
		t.Fatalf("expected forward-slash path, got %q", relativePath)
	}
}

func TestExecuteCreateActionRejectsExistingTargetWithoutChangingIt(t *testing.T) {
	workspace := t.TempDir()
	path := writeTestFile(t, workspace, "existing.txt", "original")
	action := Action{ID: "create-file", Operation: "create", Path: "existing.txt", Content: "replacement"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_ALREADY_EXISTS")
	assertFileContent(t, path, "original")
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteCreateActionRejectsMissingParentDirectory(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-file", Operation: "create", Path: "missing/created.txt", Content: "content"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PARENT_DIRECTORY_NOT_FOUND")
	assertPathDoesNotExist(t, filepath.Join(workspace, "missing", "created.txt"))
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteCreateActionRejectsAbsolutePath(t *testing.T) {
	workspace := t.TempDir()
	absolutePath := filepath.Join(workspace, "created.txt")
	action := Action{ID: "create-file", Operation: "create", Path: absolutePath, Content: "content"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
	assertPathDoesNotExist(t, absolutePath)
}

func TestExecuteCreateActionRejectsParentTraversal(t *testing.T) {
	workspace := t.TempDir()
	outsidePath := filepath.Join(filepath.Dir(workspace), "outside-created.txt")
	_ = os.Remove(outsidePath)
	t.Cleanup(func() { _ = os.Remove(outsidePath) })
	action := Action{ID: "create-file", Operation: "create", Path: filepath.Join("..", "outside-created.txt"), Content: "content"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
	assertPathDoesNotExist(t, outsidePath)
}

func TestExecuteCreateActionRejectsWindowsVolumeQualifiedPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows volume-qualified path behavior is platform-specific")
	}

	workspace := t.TempDir()
	action := Action{ID: "create-file", Operation: "create", Path: `C:\created.txt`, Content: "content"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}

func TestExecuteCreateActionRejectsSymbolicLinkParent(t *testing.T) {
	workspace := t.TempDir()
	realDirectory := filepath.Join(workspace, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatalf("create real directory: %v", err)
	}
	linkPath := filepath.Join(workspace, "linked")
	if err := os.Symlink(realDirectory, linkPath); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{ID: "create-file", Operation: "create", Path: filepath.Join("linked", "created.txt"), Content: "content"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
	assertPathDoesNotExist(t, filepath.Join(realDirectory, "created.txt"))
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteCreateActionRejectsSymbolicLinkTarget(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "target.txt", "original")
	linkPath := filepath.Join(workspace, "link.txt")
	if err := os.Symlink(filepath.Join(workspace, "target.txt"), linkPath); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{ID: "create-file", Operation: "create", Path: "link.txt", Content: "content"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
	assertFileContent(t, filepath.Join(workspace, "target.txt"), "original")
}

func TestExecuteCreateActionRejectsInvalidUTF8(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-file", Operation: "create", Path: "invalid.txt", Content: string([]byte{0xff, 0xfe})}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "UNSUPPORTED_FILE")
	assertPathDoesNotExist(t, filepath.Join(workspace, "invalid.txt"))
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteCreateActionRejectsNullByte(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-file", Operation: "create", Path: "binary.txt", Content: "before\x00after"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "UNSUPPORTED_FILE")
	assertPathDoesNotExist(t, filepath.Join(workspace, "binary.txt"))
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteCreateActionRejectsOversizedContent(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-file", Operation: "create", Path: "large.txt", Content: strings.Repeat("a", int(maximumFileSize)+1)}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_TOO_LARGE")
	assertPathDoesNotExist(t, filepath.Join(workspace, "large.txt"))
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteCreateActionLeavesNoTemporaryFileAfterSuccess(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "create-file", Operation: "create", Path: "created.txt", Content: "content"}

	_, _, responseError := executeCreateAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeCreateAction returned an error: %#v", responseError)
	}
	assertNoTemporaryCreateFiles(t, workspace)
}

func TestExecuteRequestStopsAfterFailedCreateAndPreservesEarlierResult(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "existing.txt", "original")
	request := Request{
		Version: protocolVersion,
		Actions: []Action{
			{ID: "first", Operation: "create", Path: "first.txt", Content: "first"},
			{ID: "failure", Operation: "create", Path: "existing.txt", Content: "replacement"},
			{ID: "later", Operation: "create", Path: "later.txt", Content: "later"},
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
	if response.Results[0].Data == nil || response.Results[0].Data.Path != "first.txt" || response.Results[0].Data.BytesWritten != len("first") {
		t.Fatalf("unexpected first create result: %#v", response.Results[0].Data)
	}
	if response.Results[0].Data.SHA256 != calculateSHA256([]byte("first")) {
		t.Fatalf("unexpected first create SHA-256: %q", response.Results[0].Data.SHA256)
	}
	if response.Error == nil || response.Error.Code != "FILE_ALREADY_EXISTS" || response.Error.ActionID != "failure" || response.Error.ActionIndex != 1 {
		t.Fatalf("unexpected response error: %#v", response.Error)
	}
	assertFileContent(t, filepath.Join(workspace, "first.txt"), "first")
	assertFileContent(t, filepath.Join(workspace, "existing.txt"), "original")
	assertPathDoesNotExist(t, filepath.Join(workspace, "later.txt"))
	assertNoTemporaryCreateFiles(t, workspace)
}

func assertNoTemporaryCreateFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.HasPrefix(entry.Name(), ".atr-create-") {
			t.Fatalf("temporary create file was not removed: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect workspace for temporary create files: %v", err)
	}
}

func assertPathDoesNotExist(t *testing.T, path string) {
	t.Helper()
	_, err := os.Lstat(path)
	if !os.IsNotExist(err) {
		t.Fatalf("expected path not to exist, got error %v", err)
	}
}
