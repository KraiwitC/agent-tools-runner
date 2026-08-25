package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecuteTreeActionReturnsSortedProjectEntries(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "z-last.txt", "last")
	writeTestFile(t, workspace, "nested/b.txt", "b")
	writeTestFile(t, workspace, "nested/a.txt", "a")
	writeTestFile(t, workspace, "a-first.txt", "first")
	action := Action{ID: "project-tree", Operation: "tree", Path: "."}

	entries, relativePath, truncated, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	if relativePath != "." {
		t.Fatalf("expected root path, got %q", relativePath)
	}
	if truncated {
		t.Fatal("did not expect tree result to be truncated")
	}

	expected := []TreeEntry{
		{Path: "a-first.txt", Type: "file"},
		{Path: "nested", Type: "directory"},
		{Path: "z-last.txt", Type: "file"},
		{Path: "nested/a.txt", Type: "file"},
		{Path: "nested/b.txt", Type: "file"},
	}
	assertTreeEntries(t, entries, expected)
}

func TestExecuteTreeActionReturnsUpperLevelEntriesBeforeDeepDescendants(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "a-first/deeper/file.txt", "deep")
	writeTestFile(t, workspace, "z-last/file.txt", "upper")
	action := Action{ID: "project-tree", Operation: "tree", Path: "."}

	entries, _, truncated, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	if truncated {
		t.Fatal("did not expect tree result to be truncated")
	}
	assertTreeEntries(t, entries, []TreeEntry{
		{Path: "a-first", Type: "directory"},
		{Path: "z-last", Type: "directory"},
		{Path: "a-first/deeper", Type: "directory"},
		{Path: "z-last/file.txt", Type: "file"},
		{Path: "a-first/deeper/file.txt", Type: "file"},
	})
}

func TestExecuteTreeActionReadsRequestedSubdirectory(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "included/file.txt", "included")
	writeTestFile(t, workspace, "excluded/file.txt", "excluded")
	action := Action{ID: "subtree", Operation: "tree", Path: "included"}

	entries, relativePath, truncated, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	if relativePath != "included" {
		t.Fatalf("expected included path, got %q", relativePath)
	}
	if truncated {
		t.Fatal("did not expect tree result to be truncated")
	}
	assertTreeEntries(t, entries, []TreeEntry{{Path: "included/file.txt", Type: "file"}})
}

func TestExecuteTreeActionUsesForwardSlashPaths(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "nested/deeper/file.txt", "content")
	action := Action{ID: "project-tree", Operation: "tree", Path: "."}

	entries, _, _, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	if entries[len(entries)-1].Path != "nested/deeper/file.txt" {
		t.Fatalf("expected forward-slash path, got %q", entries[len(entries)-1].Path)
	}
}

func TestExecuteTreeActionExcludesDefaultDirectories(t *testing.T) {
	workspace := t.TempDir()
	for directory := range defaultSearchExcludedDirectories {
		writeTestFile(t, workspace, directory+"/hidden.txt", "hidden")
	}
	writeTestFile(t, workspace, ".vscode/settings.json", "visible")
	action := Action{ID: "project-tree", Operation: "tree", Path: "."}

	entries, _, _, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	assertTreeEntries(t, entries, []TreeEntry{
		{Path: ".vscode", Type: "directory"},
		{Path: ".vscode/settings.json", Type: "file"},
	})
}

func TestExecuteTreeActionSkipsSymbolicLinks(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "real/file.txt", "content")
	if err := os.Symlink(filepath.Join(workspace, "real"), filepath.Join(workspace, "linked")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{ID: "project-tree", Operation: "tree", Path: "."}

	entries, _, _, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	assertTreeEntries(t, entries, []TreeEntry{
		{Path: "real", Type: "directory"},
		{Path: "real/file.txt", Type: "file"},
	})
}

func TestExecuteTreeActionRejectsSymbolicLinkRoot(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "real"), 0o700); err != nil {
		t.Fatalf("create real directory: %v", err)
	}
	if err := os.Symlink(filepath.Join(workspace, "real"), filepath.Join(workspace, "linked")); err != nil {
		t.Skipf("symbolic links are unavailable in this environment: %v", err)
	}
	action := Action{ID: "project-tree", Operation: "tree", Path: "linked"}

	_, _, _, responseError := executeTreeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "SYMLINK_NOT_SUPPORTED")
}

func TestExecuteTreeActionRejectsMissingDirectory(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "project-tree", Operation: "tree", Path: "missing"}

	_, _, _, responseError := executeTreeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "FILE_NOT_FOUND")
}

func TestExecuteTreeActionRejectsRegularFile(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, workspace, "file.txt", "content")
	action := Action{ID: "project-tree", Operation: "tree", Path: "file.txt"}

	_, _, _, responseError := executeTreeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "UNSUPPORTED_FILE")
}

func TestExecuteTreeActionRejectsAbsolutePath(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "project-tree", Operation: "tree", Path: workspace}

	_, _, _, responseError := executeTreeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}

func TestExecuteTreeActionRejectsParentTraversal(t *testing.T) {
	workspace := t.TempDir()
	action := Action{ID: "project-tree", Operation: "tree", Path: ".."}

	_, _, _, responseError := executeTreeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}

func TestExecuteTreeActionRejectsWindowsVolumeQualifiedPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows volume-qualified path behavior is platform-specific")
	}

	workspace := t.TempDir()
	action := Action{ID: "project-tree", Operation: "tree", Path: `C:\project`}

	_, _, _, responseError := executeTreeAction(workspace, action, 0)
	assertResponseErrorCode(t, responseError, "PATH_OUTSIDE_WORKSPACE")
}

func TestExecuteTreeActionKeepsLaterUpperLevelDirectoryWhenDeepTreeExceedsLimit(t *testing.T) {
	workspace := t.TempDir()
	for index := 0; index < maximumCollectedTreeEntries; index++ {
		writeTestFile(t, workspace, fmt.Sprintf("a-deep/file-%03d.txt", index), "content")
	}
	writeTestFile(t, workspace, "z-upper/file.txt", "upper")
	action := Action{ID: "project-tree", Operation: "tree", Path: "."}

	entries, _, truncated, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	if !truncated {
		t.Fatal("expected tree result to be truncated")
	}
	if len(entries) != maximumCollectedTreeEntries {
		t.Fatalf("expected %d entries, got %d", maximumCollectedTreeEntries, len(entries))
	}
	if entries[0] != (TreeEntry{Path: "a-deep", Type: "directory"}) {
		t.Fatalf("expected first upper-level directory, got %#v", entries[0])
	}
	if entries[1] != (TreeEntry{Path: "z-upper", Type: "directory"}) {
		t.Fatalf("expected later upper-level directory before deep entries, got %#v", entries[1])
	}
}

func TestExecuteTreeActionTruncatesAtLimit(t *testing.T) {
	workspace := t.TempDir()
	for index := 0; index < maximumCollectedTreeEntries+1; index++ {
		writeTestFile(t, workspace, fmt.Sprintf("file-%03d.txt", index), "content")
	}
	action := Action{ID: "project-tree", Operation: "tree", Path: "."}

	entries, _, truncated, responseError := executeTreeAction(workspace, action, 0)
	if responseError != nil {
		t.Fatalf("executeTreeAction returned an error: %#v", responseError)
	}
	if !truncated {
		t.Fatal("expected tree result to be truncated")
	}
	if len(entries) != maximumCollectedTreeEntries {
		t.Fatalf("expected %d entries, got %d", maximumCollectedTreeEntries, len(entries))
	}
}

func assertTreeEntries(t *testing.T, actual []TreeEntry, expected []TreeEntry) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %#v", len(expected), len(actual), actual)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("entry %d: expected %#v, got %#v", index, expected[index], actual[index])
		}
	}
}
