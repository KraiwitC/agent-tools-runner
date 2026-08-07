package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const defaultCreatedFileMode os.FileMode = 0o666

func executeCreateAction(workspace string, action Action, actionIndex int) (int, string, *ResponseError) {
	resolvedPath, relativePath, err := resolveWorkspaceCreatePath(workspace, action.Path)
	if err != nil {
		return 0, relativePath, workspaceResponseError(action, actionIndex, err, "WRITE_FAILED", "Could not prepare the requested file creation.", filepath.ToSlash(filepath.Clean(action.Path)))
	}

	content := []byte(action.Content)
	if int64(len(content)) > maximumFileSize {
		return 0, relativePath, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "FILE_TOO_LARGE",
			Message:     "Content exceeds the 1 MiB file-size limit.",
			Path:        relativePath,
		}
	}
	if !isSupportedText(content) {
		return 0, relativePath, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "UNSUPPORTED_FILE",
			Message:     "Content must be valid UTF-8 text without null bytes.",
			Path:        relativePath,
		}
	}

	bytesWritten, err := createFileSafely(resolvedPath, content)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, relativePath, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "FILE_ALREADY_EXISTS",
				Message:     "The target file already exists.",
				Path:        relativePath,
			}
		}

		return 0, relativePath, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "WRITE_FAILED",
			Message:     "Could not safely create the requested file.",
			Path:        relativePath,
		}
	}

	return bytesWritten, relativePath, nil
}

func resolveWorkspaceCreatePath(workspace string, requestedPath string) (string, string, error) {
	cleanRequestedPath := filepath.Clean(requestedPath)
	relativePath := filepath.ToSlash(cleanRequestedPath)

	if filepath.IsAbs(cleanRequestedPath) || filepath.VolumeName(cleanRequestedPath) != "" {
		return "", relativePath, newWorkspaceError(
			"PATH_OUTSIDE_WORKSPACE",
			"Absolute and volume-qualified paths are not allowed.",
			relativePath,
			nil,
		)
	}
	if cleanRequestedPath == "." || cleanRequestedPath == "" {
		return "", relativePath, newWorkspaceError(
			"UNSUPPORTED_FILE",
			"Path must identify a regular file.",
			relativePath,
			nil,
		)
	}

	resolvedPath := filepath.Join(workspace, cleanRequestedPath)
	containedPath, err := filepath.Rel(workspace, resolvedPath)
	if err != nil {
		return "", relativePath, newWorkspaceError(
			"PATH_OUTSIDE_WORKSPACE",
			"Could not validate the requested path.",
			relativePath,
			err,
		)
	}
	if containedPath == ".." || strings.HasPrefix(containedPath, ".."+string(filepath.Separator)) {
		return "", relativePath, newWorkspaceError(
			"PATH_OUTSIDE_WORKSPACE",
			"Path is outside the selected workspace.",
			relativePath,
			nil,
		)
	}

	parentRelativePath := filepath.Dir(containedPath)
	if err := validateCreateParentPath(workspace, parentRelativePath, relativePath); err != nil {
		return "", relativePath, err
	}

	targetInfo, err := os.Lstat(resolvedPath)
	if err == nil {
		if targetInfo.Mode()&os.ModeSymlink != 0 {
			return "", relativePath, newWorkspaceError(
				"SYMLINK_NOT_SUPPORTED",
				"Symbolic links are not supported.",
				relativePath,
				nil,
			)
		}
		return "", relativePath, newWorkspaceError(
			"FILE_ALREADY_EXISTS",
			"The target file already exists.",
			relativePath,
			os.ErrExist,
		)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", relativePath, newWorkspaceError(
			"WRITE_FAILED",
			"Could not inspect the target path.",
			relativePath,
			err,
		)
	}

	return resolvedPath, relativePath, nil
}

func validateCreateParentPath(workspace string, parentRelativePath string, responsePath string) error {
	if parentRelativePath == "." {
		return nil
	}

	currentPath := workspace
	for _, part := range strings.Split(filepath.Clean(parentRelativePath), string(filepath.Separator)) {
		currentPath = filepath.Join(currentPath, part)

		fileInfo, err := os.Lstat(currentPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return newWorkspaceError(
					"PARENT_DIRECTORY_NOT_FOUND",
					"A required parent directory does not exist.",
					responsePath,
					err,
				)
			}
			return newWorkspaceError(
				"WRITE_FAILED",
				"Could not inspect a parent path component.",
				responsePath,
				err,
			)
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return newWorkspaceError(
				"SYMLINK_NOT_SUPPORTED",
				"Symbolic links are not supported.",
				responsePath,
				nil,
			)
		}
		if !fileInfo.IsDir() {
			return newWorkspaceError(
				"UNSUPPORTED_FILE",
				"A parent path component is not a directory.",
				responsePath,
				nil,
			)
		}
	}

	return nil
}

func createFileSafely(path string, content []byte) (int, error) {
	temporaryFile, err := os.CreateTemp(filepath.Dir(path), ".atr-create-*")
	if err != nil {
		return 0, fmt.Errorf("create temporary file: %w", err)
	}

	temporaryPath := temporaryFile.Name()
	defer func() {
		_ = os.Remove(temporaryPath)
	}()

	if _, err := temporaryFile.Write(content); err != nil {
		_ = temporaryFile.Close()
		return 0, fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return 0, fmt.Errorf("flush temporary file: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return 0, fmt.Errorf("close temporary file: %w", err)
	}

	sourceFile, err := os.Open(temporaryPath)
	if err != nil {
		return 0, fmt.Errorf("open prepared temporary file: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, defaultCreatedFileMode)
	if err != nil {
		return 0, fmt.Errorf("exclusively create target file: %w", err)
	}

	keepTarget := false
	defer func() {
		if !keepTarget {
			_ = os.Remove(path)
		}
	}()

	written, copyErr := io.Copy(targetFile, sourceFile)
	if copyErr != nil {
		_ = targetFile.Close()
		return 0, fmt.Errorf("write target file: %w", copyErr)
	}
	if written != int64(len(content)) {
		_ = targetFile.Close()
		return 0, fmt.Errorf("write target file: wrote %d of %d bytes", written, len(content))
	}
	if err := targetFile.Sync(); err != nil {
		_ = targetFile.Close()
		return 0, fmt.Errorf("flush target file: %w", err)
	}
	if err := targetFile.Close(); err != nil {
		return 0, fmt.Errorf("close target file: %w", err)
	}

	keepTarget = true
	return int(written), nil
}
