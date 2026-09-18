package runner

import (
	"os"
	"path/filepath"
	"strings"
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

func TestExecuteCopyActionRejectsUnsupportedPathsAndSources(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, workspace string) Action
		expectedCode string
	}{
		{
			name: "symbolic link source",
			setup: func(t *testing.T, workspace string) Action {
				content := "source"
				writeTestFile(t, workspace, "source.txt", content)
				if err := os.Symlink(filepath.Join(workspace, "source.txt"), filepath.Join(workspace, "linked.txt")); err != nil {
					t.Skipf("symbolic links are unavailable in this environment: %v", err)
				}
				return Action{ID: "copy-file", Operation: "copy", Source: "linked.txt", Destination: "destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
			},
			expectedCode: "SYMLINK_NOT_SUPPORTED",
		},
		{
			name: "binary source",
			setup: func(t *testing.T, workspace string) Action {
				content := "before\x00after"
				writeTestFile(t, workspace, "source.dat", content)
				return Action{ID: "copy-file", Operation: "copy", Source: "source.dat", Destination: "destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
			},
			expectedCode: "UNSUPPORTED_FILE",
		},
		{
			name: "oversized source",
			setup: func(t *testing.T, workspace string) Action {
				content := strings.Repeat("a", int(maximumFileSize)+1)
				writeTestFile(t, workspace, "source.txt", content)
				return Action{ID: "copy-file", Operation: "copy", Source: "source.txt", Destination: "destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
			},
			expectedCode: "FILE_TOO_LARGE",
		},
		{
			name: "destination parent is a file",
			setup: func(t *testing.T, workspace string) Action {
				content := "source"
				writeTestFile(t, workspace, "source.txt", content)
				writeTestFile(t, workspace, "parent", "not a directory")
				return Action{ID: "copy-file", Operation: "copy", Source: "source.txt", Destination: "parent/destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
			},
			expectedCode: "UNSUPPORTED_FILE",
		},
		{
			name: "symbolic link destination parent",
			setup: func(t *testing.T, workspace string) Action {
				content := "source"
				writeTestFile(t, workspace, "source.txt", content)
				realDirectory := filepath.Join(workspace, "real")
				if err := os.Mkdir(realDirectory, 0o700); err != nil {
					t.Fatalf("create real directory: %v", err)
				}
				if err := os.Symlink(realDirectory, filepath.Join(workspace, "linked")); err != nil {
					t.Skipf("symbolic links are unavailable in this environment: %v", err)
				}
				return Action{ID: "copy-file", Operation: "copy", Source: "source.txt", Destination: "linked/destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
			},
			expectedCode: "SYMLINK_NOT_SUPPORTED",
		},
		{
			name: "source and destination are the same path",
			setup: func(t *testing.T, workspace string) Action {
				content := "source"
				writeTestFile(t, workspace, "source.txt", content)
				return Action{ID: "copy-file", Operation: "copy", Source: "source.txt", Destination: "source.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
			},
			expectedCode: "FILE_ALREADY_EXISTS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			action := test.setup(t, workspace)

			_, _, _, responseError := executeCopyAction(workspace, action, 0)
			assertResponseErrorCode(t, responseError, test.expectedCode)
			assertPathDoesNotExist(t, filepath.Join(workspace, "destination.txt"))
		})
	}
}
