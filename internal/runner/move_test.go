package runner

import (
	"os"
	"path/filepath"
	"strings"
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

func TestExecuteMoveActionRejectsUnsupportedPathsAndSources(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T, workspace string) (Action, string, string)
		expectedCode string
	}{
		{
			name: "symbolic link source",
			setup: func(t *testing.T, workspace string) (Action, string, string) {
				content := "source"
				sourcePath := writeTestFile(t, workspace, "source.txt", content)
				if err := os.Symlink(sourcePath, filepath.Join(workspace, "linked.txt")); err != nil {
					t.Skipf("symbolic links are unavailable in this environment: %v", err)
				}
				action := Action{ID: "move-file", Operation: "move", Source: "linked.txt", Destination: "destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
				return action, sourcePath, content
			},
			expectedCode: "SYMLINK_NOT_SUPPORTED",
		},
		{
			name: "binary source",
			setup: func(t *testing.T, workspace string) (Action, string, string) {
				content := "before\x00after"
				sourcePath := writeTestFile(t, workspace, "source.dat", content)
				action := Action{ID: "move-file", Operation: "move", Source: "source.dat", Destination: "destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
				return action, sourcePath, content
			},
			expectedCode: "UNSUPPORTED_FILE",
		},
		{
			name: "oversized source",
			setup: func(t *testing.T, workspace string) (Action, string, string) {
				content := strings.Repeat("a", int(maximumFileSize)+1)
				sourcePath := writeTestFile(t, workspace, "source.txt", content)
				action := Action{ID: "move-file", Operation: "move", Source: "source.txt", Destination: "destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
				return action, sourcePath, content
			},
			expectedCode: "FILE_TOO_LARGE",
		},
		{
			name: "destination parent is a file",
			setup: func(t *testing.T, workspace string) (Action, string, string) {
				content := "source"
				sourcePath := writeTestFile(t, workspace, "source.txt", content)
				writeTestFile(t, workspace, "parent", "not a directory")
				action := Action{ID: "move-file", Operation: "move", Source: "source.txt", Destination: "parent/destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
				return action, sourcePath, content
			},
			expectedCode: "UNSUPPORTED_FILE",
		},
		{
			name: "symbolic link destination parent",
			setup: func(t *testing.T, workspace string) (Action, string, string) {
				content := "source"
				sourcePath := writeTestFile(t, workspace, "source.txt", content)
				realDirectory := filepath.Join(workspace, "real")
				if err := os.Mkdir(realDirectory, 0o700); err != nil {
					t.Fatalf("create real directory: %v", err)
				}
				if err := os.Symlink(realDirectory, filepath.Join(workspace, "linked")); err != nil {
					t.Skipf("symbolic links are unavailable in this environment: %v", err)
				}
				action := Action{ID: "move-file", Operation: "move", Source: "source.txt", Destination: "linked/destination.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
				return action, sourcePath, content
			},
			expectedCode: "SYMLINK_NOT_SUPPORTED",
		},
		{
			name: "source and destination are the same path",
			setup: func(t *testing.T, workspace string) (Action, string, string) {
				content := "source"
				sourcePath := writeTestFile(t, workspace, "source.txt", content)
				action := Action{ID: "move-file", Operation: "move", Source: "source.txt", Destination: "source.txt", ExpectedSHA256: calculateSHA256([]byte(content))}
				return action, sourcePath, content
			},
			expectedCode: "FILE_ALREADY_EXISTS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			action, sourcePath, sourceContent := test.setup(t, workspace)

			_, _, _, responseError := executeMoveAction(workspace, action, 0)
			assertResponseErrorCode(t, responseError, test.expectedCode)
			assertFileContent(t, sourcePath, sourceContent)
			if action.Destination != action.Source {
				assertPathDoesNotExist(t, filepath.Join(workspace, action.Destination))
			}
		})
	}
}
