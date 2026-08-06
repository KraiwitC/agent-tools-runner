package runner

import (
	"errors"
	"os"
	"path/filepath"
)

func executeDeleteAction(workspace string, action Action, actionIndex int) (string, string, *ResponseError) {
	resolvedPath, relativePath, err := resolveWorkspaceInspectPath(workspace, action.Path)
	if err != nil {
		return relativePath, "", workspaceDeleteResponseError(action, actionIndex, err)
	}
	if relativePath == "." {
		return relativePath, "directory", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "UNSUPPORTED_FILE",
			Message:     "The workspace root cannot be deleted.",
			Path:        relativePath,
		}
	}

	fileInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		return relativePath, "", workspaceDeleteResponseError(action, actionIndex, err)
	}
	if fileInfo.IsDir() {
		return deleteEmptyDirectory(resolvedPath, relativePath, action, actionIndex)
	}
	return deleteFile(resolvedPath, relativePath, action, actionIndex)
}

func deleteFile(resolvedPath string, relativePath string, action Action, actionIndex int) (string, string, *ResponseError) {
	if action.ExpectedSHA256 == "" {
		return relativePath, "file", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "INVALID_REQUEST",
			Message:     "expectedSha256 is required when deleting a file.",
			Path:        relativePath,
		}
	}

	content, err := readInspectFile(resolvedPath, relativePath)
	if err != nil {
		return relativePath, "file", workspaceDeleteResponseError(action, actionIndex, err)
	}
	if calculateSHA256(content) != action.ExpectedSHA256 {
		return relativePath, "file", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "FILE_CHANGED",
			Message:     "File content does not match expectedSha256.",
			Path:        relativePath,
		}
	}
	if err := os.Remove(resolvedPath); err != nil {
		return relativePath, "file", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "DELETE_FAILED",
			Message:     "Could not delete the requested file.",
			Path:        relativePath,
		}
	}
	return relativePath, "file", nil
}

func deleteEmptyDirectory(resolvedPath string, relativePath string, action Action, actionIndex int) (string, string, *ResponseError) {
	if action.ExpectedSHA256 != "" {
		return relativePath, "directory", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "INVALID_REQUEST",
			Message:     "expectedSha256 is not supported when deleting a directory.",
			Path:        relativePath,
		}
	}

	empty, err := isDirectoryEmpty(resolvedPath)
	if err != nil {
		return relativePath, "directory", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "DELETE_FAILED",
			Message:     "Could not inspect the requested directory before deletion.",
			Path:        relativePath,
		}
	}
	if !empty {
		return relativePath, "directory", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "DIRECTORY_NOT_EMPTY",
			Message:     "Only an empty directory can be deleted.",
			Path:        relativePath,
		}
	}
	if err := os.Remove(resolvedPath); err != nil {
		return relativePath, "directory", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "DELETE_FAILED",
			Message:     "Could not delete the requested directory.",
			Path:        relativePath,
		}
	}
	return relativePath, "directory", nil
}

func workspaceDeleteResponseError(action Action, actionIndex int, err error) *ResponseError {
	var pathError *workspaceError
	if errors.As(err, &pathError) {
		return &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        pathError.Code,
			Message:     pathError.Message,
			Path:        pathError.Path,
		}
	}

	return &ResponseError{
		ActionID:    action.ID,
		ActionIndex: actionIndex,
		Code:        "DELETE_FAILED",
		Message:     "Could not prepare the requested deletion.",
		Path:        filepath.ToSlash(filepath.Clean(action.Path)),
	}
}
