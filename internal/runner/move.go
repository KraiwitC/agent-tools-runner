package runner

import (
	"errors"
	"os"
	"path/filepath"
)

func executeMoveAction(workspace string, action Action, actionIndex int) (int, string, string, *ResponseError) {
	resolvedSource, sourcePath, _, err := resolveWorkspaceFile(workspace, action.Source)
	if err != nil {
		return 0, filepath.ToSlash(filepath.Clean(action.Destination)), "", workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not prepare the requested file move.", sourcePath)
	}

	content, err := readTextFile(resolvedSource, sourcePath, "Could not open the source file.", "Could not read the source file.")
	if err != nil {
		return 0, filepath.ToSlash(filepath.Clean(action.Destination)), "", workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not prepare the requested file move.", sourcePath)
	}

	contentSHA256 := calculateSHA256(content)
	if contentSHA256 != action.ExpectedSHA256 {
		return 0, filepath.ToSlash(filepath.Clean(action.Destination)), "", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "FILE_CHANGED",
			Message:     "Source file content does not match expectedSha256.",
			Path:        sourcePath,
		}
	}

	resolvedDestination, destinationPath, err := resolveWorkspaceCreatePath(workspace, action.Destination)
	if err != nil {
		return 0, destinationPath, "", workspaceResponseError(action, actionIndex, err, "WRITE_FAILED", "Could not prepare the move destination.", destinationPath)
	}

	bytesWritten, err := createFileSafely(resolvedDestination, content)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, destinationPath, "", &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "FILE_ALREADY_EXISTS",
				Message:     "The move destination already exists.",
				Path:        destinationPath,
			}
		}
		return 0, destinationPath, "", workspaceResponseError(action, actionIndex, err, "WRITE_FAILED", "Could not safely prepare the moved file.", destinationPath)
	}

	currentContent, err := readTextFile(resolvedSource, sourcePath, "Could not reopen the source file.", "Could not recheck the source file.")
	if err != nil {
		_ = os.Remove(resolvedDestination)
		return 0, destinationPath, "", workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not verify the source file before removal.", sourcePath)
	}
	if calculateSHA256(currentContent) != action.ExpectedSHA256 {
		_ = os.Remove(resolvedDestination)
		return 0, destinationPath, "", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "FILE_CHANGED",
			Message:     "Source file content changed before removal.",
			Path:        sourcePath,
		}
	}

	if err := os.Remove(resolvedSource); err != nil {
		_ = os.Remove(resolvedDestination)
		return 0, destinationPath, "", &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "DELETE_FAILED",
			Message:     "Could not remove the source file after preparing the destination.",
			Path:        sourcePath,
		}
	}

	return bytesWritten, destinationPath, contentSHA256, nil
}
