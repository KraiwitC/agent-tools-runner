package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExecuteInspectActionReturnsFileMetadata(t *testing.T) {
	workspace := t.TempDir()
	content := "first line\nsecond line\n"
	writeTestFile(t, workspace, "notes.txt", content)
	action := Action{ID: "inspect-file", Operation: "inspect", Path: "notes.txt"}

	data, responseError := executeInspectAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeInspectAction returned an error: %#v", responseError)
	}
	if data.Path != "notes.txt" || data.Type != "file" {
		t.Fatalf("unexpected inspect identity: %#v", data)
	}
	if data.SizeBytes != int64(len([]byte(content))) {
		t.Fatalf("expected %d bytes, got %d", len([]byte(content)), data.SizeBytes)
	}
	if data.LineCount != 2 {
		t.Fatalf("expected two lines, got %d", data.LineCount)
	}
	expectedHash := sha256.Sum256([]byte(content))
	if data.SHA256 != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("unexpected SHA-256: %q", data.SHA256)
	}
}

func TestExecuteInspectActionPreviewsOversizedTextFile(t *testing.T) {
	workspace := t.TempDir()
	content := strings.Repeat("a", 4095) + "ก" + strings.Repeat("b", int(maximumFileSize))
	writeTestFile(t, workspace, "large.txt", content)
	action := Action{ID: "inspect-file", Operation: "inspect", Path: "large.txt"}

	data, responseError := executeInspectAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeInspectAction returned an error: %#v", responseError)
	}
	if data.SizeBytes != int64(len(content)) || !data.Truncated {
		t.Fatalf("unexpected oversized file metadata: %#v", data)
	}
	if len(data.Content) != 4095 || !utf8.ValidString(data.Content) || data.SHA256 != "" || data.LineCount != 0 {
		t.Fatalf("unexpected oversized file preview: %#v", data)
	}
}

func TestExecuteInspectActionReturnsEmptyFileMetadata(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "empty.txt", "")
	action := Action{ID: "inspect-file", Operation: "inspect", Path: "empty.txt"}

	data, responseError := executeInspectAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeInspectAction returned an error: %#v", responseError)
	}
	if data.SizeBytes != 0 || data.LineCount != 0 {
		t.Fatalf("unexpected empty file metadata: %#v", data)
	}
	if data.SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("unexpected empty file SHA-256: %q", data.SHA256)
	}
}

func TestExecuteInspectActionReturnsEmptyDirectoryMetadata(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "empty"), 0o700); err != nil {
		t.Fatalf("create empty directory: %v", err)
	}
	action := Action{ID: "inspect-directory", Operation: "inspect", Path: "empty"}

	data, responseError := executeInspectAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeInspectAction returned an error: %#v", responseError)
	}
	if data.Type != "directory" || data.Empty == nil || !*data.Empty {
		t.Fatalf("unexpected empty directory metadata: %#v", data)
	}
}

func TestExecuteInspectActionReturnsNonEmptyDirectoryMetadata(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "directory/file.txt", "content")
	action := Action{ID: "inspect-directory", Operation: "inspect", Path: "directory"}

	data, responseError := executeInspectAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeInspectAction returned an error: %#v", responseError)
	}
	if data.Type != "directory" || data.Empty == nil || *data.Empty {
		t.Fatalf("unexpected non-empty directory metadata: %#v", data)
	}
}

func TestExecuteInspectActionRejectsOutsidePath(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "inspect-path", Operation: "inspect", Path: ".."}

	_, responseError := executeInspectAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}

func TestExecuteInspectActionRejectsMissingPath(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "inspect-path", Operation: "inspect", Path: "missing.txt"}

	_, responseError := executeInspectAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_NOT_FOUND")
}

func TestExecuteInspectActionRejectsSymbolicLink(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "target.txt", "content")
	if err := os.Symlink(filepath.Join(workspace, "target.txt"), filepath.Join(workspace, "linked.txt")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{ID: "inspect-path", Operation: "inspect", Path: "linked.txt"}

	_, responseError := executeInspectAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
}

func TestExecuteInspectActionRejectsSymbolicLinkParent(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "real/file.txt", "content")
	if err := os.Symlink(filepath.Join(workspace, "real"), filepath.Join(workspace, "linked")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{ID: "inspect-path", Operation: "inspect", Path: filepath.Join("linked", "file.txt")}

	_, responseError := executeInspectAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
}

func TestExecuteInspectActionRejectsOversizedPreviewWithNullByte(t *testing.T) {
	workspace := t.TempDir()
	content := append([]byte("before\x00after"), []byte(strings.Repeat("a", int(maximumFileSize)))...)
	path := filepath.Join(workspace, "large.txt")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write oversized test file: %v", err)
	}
	action := Action{ID: "inspect-file", Operation: "inspect", Path: "large.txt"}

	data, responseError := executeInspectAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "UNSUPPORTED_FILE")
	if data == nil || data.Path != "large.txt" || data.Type != "file" {
		t.Fatalf("unexpected inspect error metadata: %#v", data)
	}
}

func TestExecuteInspectActionTrimsInvalidUTF8FromOversizedPreview(t *testing.T) {
	workspace := t.TempDir()
	content := append([]byte{0xff, 0xfe}, []byte(strings.Repeat("a", int(maximumFileSize)))...)
	path := filepath.Join(workspace, "large.txt")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write oversized test file: %v", err)
	}
	action := Action{ID: "inspect-file", Operation: "inspect", Path: "large.txt"}

	data, responseError := executeInspectAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeInspectAction returned an error: %#v", responseError)
	}
	if data.Path != "large.txt" || data.Type != "file" || !data.Truncated {
		t.Fatalf("unexpected inspect metadata: %#v", data)
	}
	if data.Content != "" || !utf8.ValidString(data.Content) {
		t.Fatalf("invalid UTF-8 preview was not trimmed safely: %q", data.Content)
	}
}

func TestReadWorkspaceFileReturnsSHA256(t *testing.T) {
	workspace := t.TempDir()
	content := "content"
	writeTestFile(t, workspace, "file.txt", content)

	result, err := readWorkspaceFile(workspace, "file.txt")
	if err != nil {
		t.Fatalf("readWorkspaceFile returned an error: %v", err)
	}
	expectedHash := sha256.Sum256([]byte(content))
	if result.SHA256 != hex.EncodeToString(expectedHash[:]) {
		t.Fatalf("unexpected SHA-256: %q", result.SHA256)
	}
}
