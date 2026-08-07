package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type preparedReplacement struct {
	Start   int
	End     int
	NewText string
}

func executeEditAction(workspace string, action Action, actionIndex int) (int, string, *ResponseError) {
	resolvedPath, relativePath, fileInfo, err := resolveWorkspaceFile(workspace, action.Path)
	if err != nil {
		return 0, "", workspaceResponseError(action, actionIndex, err, "WRITE_FAILED", "Could not prepare the requested file edit.", action.Path)
	}

	contentBytes, err := readTextFile(resolvedPath, relativePath, "Could not read the file before editing.", "Could not read the file before editing.")
	if err != nil {
		return 0, "", workspaceResponseError(action, actionIndex, err, "WRITE_FAILED", "Could not prepare the requested file edit.", action.Path)
	}
	content := string(contentBytes)
	if calculateSHA256([]byte(content)) != action.ExpectedSHA256 {
		return 0, "", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "FILE_CHANGED",
			Message:     "File content does not match expectedSha256.",
			Path:        relativePath,
		}
	}

	preparedReplacements, responseError := prepareReplacements(content, action, actionIndex, relativePath)
	if responseError != nil {
		return 0, "", responseError
	}

	updatedContent := applyPreparedReplacements(content, preparedReplacements)
	if err := replaceFile(resolvedPath, updatedContent, fileInfo.Mode().Perm()); err != nil {
		return 0, "", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "WRITE_FAILED",
			Message:     "Could not safely write the edited file.",
			Path:        relativePath,
		}
	}

	return len(preparedReplacements), calculateSHA256([]byte(updatedContent)), nil
}

func prepareReplacements(content string, action Action, actionIndex int, relativePath string) ([]preparedReplacement, *ResponseError) {
	prepared := make([]preparedReplacement, 0, len(action.Replacements))

	for _, replacement := range action.Replacements {
		matchCount := strings.Count(content, replacement.OldText)
		if matchCount == 0 {
			return nil, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "EDIT_TARGET_NOT_FOUND",
				Message:     "The exact edit target was not found.",
				Path:        relativePath,
			}
		}
		if matchCount > 1 {
			return nil, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "EDIT_TARGET_NOT_UNIQUE",
				Message:     "The exact edit target occurs more than once.",
				Path:        relativePath,
			}
		}

		start := strings.Index(content, replacement.OldText)
		prepared = append(prepared, preparedReplacement{
			Start:   start,
			End:     start + len(replacement.OldText),
			NewText: replacement.NewText,
		})
	}

	sort.Slice(prepared, func(left int, right int) bool {
		return prepared[left].Start < prepared[right].Start
	})

	for index := 1; index < len(prepared); index++ {
		if prepared[index].Start < prepared[index-1].End {
			return nil, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "EDIT_TARGET_OVERLAP",
				Message:     "Two edit targets overlap in the original file.",
				Path:        relativePath,
			}
		}
	}

	return prepared, nil
}

func applyPreparedReplacements(content string, replacements []preparedReplacement) string {
	updated := content
	for index := len(replacements) - 1; index >= 0; index-- {
		replacement := replacements[index]
		updated = updated[:replacement.Start] + replacement.NewText + updated[replacement.End:]
	}
	return updated
}

func replaceFile(path string, content string, fileMode os.FileMode) error {
	temporaryFile, err := os.CreateTemp(filepath.Dir(path), ".atr-edit-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}

	temporaryPath := temporaryFile.Name()
	keepTemporaryFile := false
	defer func() {
		if !keepTemporaryFile {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporaryFile.Chmod(fileMode); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporaryFile.WriteString(content); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return fmt.Errorf("flush temporary file: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace target file: %w", err)
	}

	keepTemporaryFile = true
	return nil
}
