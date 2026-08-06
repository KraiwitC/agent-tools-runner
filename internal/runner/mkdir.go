package runner

import (
	"errors"
	"os"
	"path/filepath"
)

const defaultCreatedDirectoryMode os.FileMode = 0o777

func executeMkdirAction(workspace string, action Action, actionIndex int) (string, *ResponseError) {
	resolvedPath, relativePath, err := resolveWorkspaceCreatePath(workspace, action.Path)
	if err != nil {
		return relativePath, workspaceMkdirResponseError(action, actionIndex, err)
	}

	if err := os.Mkdir(resolvedPath, defaultCreatedDirectoryMode); err != nil {
		if errors.Is(err, os.ErrExist) {
			return relativePath, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "FILE_ALREADY_EXISTS",
				Message:     "The target path already exists.",
				Path:        relativePath,
			}
		}
		if errors.Is(err, os.ErrNotExist) {
			return relativePath, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "PARENT_DIRECTORY_NOT_FOUND",
				Message:     "A required parent directory does not exist.",
				Path:        relativePath,
			}
		}
		return relativePath, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "WRITE_FAILED",
			Message:     "Could not create the requested directory.",
			Path:        relativePath,
		}
	}

	return relativePath, nil
}

func workspaceMkdirResponseError(action Action, actionIndex int, err error) *ResponseError {
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
		Code:        "WRITE_FAILED",
		Message:     "Could not prepare the requested directory creation.",
		Path:        filepath.ToSlash(filepath.Clean(action.Path)),
	}
}
