package runner

import (
	"errors"
	"os"
	"path/filepath"
)

func executeCopyAction(workspace string, action Action, actionIndex int) (int, string, string, *ResponseError) {
	resolvedSource, sourcePath, _, err := resolveWorkspaceFile(workspace, action.Source)
	if err != nil {
		return 0, filepath.ToSlash(filepath.Clean(action.Destination)), "", workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not prepare the requested file copy.", sourcePath)
	}

	content, err := readTextFile(resolvedSource, sourcePath, "Could not open the source file.", "Could not read the source file.")
	if err != nil {
		return 0, filepath.ToSlash(filepath.Clean(action.Destination)), "", workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not prepare the requested file copy.", sourcePath)
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
		return 0, destinationPath, "", workspaceResponseError(action, actionIndex, err, "WRITE_FAILED", "Could not prepare the copy destination.", destinationPath)
	}

	bytesWritten, err := createFileSafely(resolvedDestination, content)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, destinationPath, "", &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "FILE_ALREADY_EXISTS",
				Message:     "The copy destination already exists.",
				Path:        destinationPath,
			}
		}
		return 0, destinationPath, "", workspaceResponseError(action, actionIndex, err, "WRITE_FAILED", "Could not safely copy the requested file.", destinationPath)
	}

	return bytesWritten, destinationPath, contentSHA256, nil
}
