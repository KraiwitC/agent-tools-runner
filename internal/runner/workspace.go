package runner

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maximumFileSize int64 = 1024 * 1024

type workspaceError struct {
	Code    string
	Message string
	Path    string
	Cause   error
}

func (err *workspaceError) Error() string {
	return err.Message
}

func (err *workspaceError) Unwrap() error {
	return err.Cause
}

func executeReadAction(workspace string, action Action, actionIndex int) ([]ReadFileResult, *ResponseError) {
	files := make([]ReadFileResult, 0, len(action.Paths))

	for _, requestedPath := range action.Paths {
		file, err := readWorkspaceFile(workspace, requestedPath)
		if err != nil {
			var pathError *workspaceError
			if errors.As(err, &pathError) {
				return files, &ResponseError{
					ActionID:    action.ID,
					ActionIndex: actionIndex,
					Code:        pathError.Code,
					Message:     pathError.Message,
					Path:        pathError.Path,
				}
			}

			return files, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "READ_FAILED",
				Message:     "Could not read the requested file.",
				Path:        filepath.ToSlash(requestedPath),
			}
		}

		files = append(files, file)
	}

	return files, nil
}

func readWorkspaceFile(workspace string, requestedPath string) (ReadFileResult, error) {
	resolvedPath, relativePath, err := resolveWorkspaceFile(workspace, requestedPath)
	if err != nil {
		return ReadFileResult{}, err
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ReadFileResult{}, newWorkspaceError("FILE_NOT_FOUND", "File was not found.", relativePath, err)
		}
		return ReadFileResult{}, newWorkspaceError("READ_FAILED", "Could not open the requested file.", relativePath, err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maximumFileSize+1))
	if err != nil {
		return ReadFileResult{}, newWorkspaceError("READ_FAILED", "Could not read the requested file.", relativePath, err)
	}
	if int64(len(content)) > maximumFileSize {
		return ReadFileResult{}, newWorkspaceError("FILE_TOO_LARGE", "File exceeds the 1 MiB Version 1 limit.", relativePath, nil)
	}
	if !isSupportedText(content) {
		return ReadFileResult{}, newWorkspaceError("UNSUPPORTED_FILE", "File is not supported UTF-8 text.", relativePath, nil)
	}

	return ReadFileResult{
		Path:    relativePath,
		Content: string(content),
		SHA256:  calculateSHA256(content),
	}, nil
}

func resolveWorkspaceFile(workspace string, requestedPath string) (string, string, error) {
	cleanRequestedPath := filepath.Clean(requestedPath)
	relativePath := filepath.ToSlash(cleanRequestedPath)

	if filepath.IsAbs(cleanRequestedPath) || filepath.VolumeName(cleanRequestedPath) != "" {
		return "", relativePath, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Absolute and volume-qualified paths are not allowed.", relativePath, nil)
	}
	if cleanRequestedPath == "." || cleanRequestedPath == "" {
		return "", relativePath, newWorkspaceError("UNSUPPORTED_FILE", "Path must identify a regular file.", relativePath, nil)
	}

	resolvedPath := filepath.Join(workspace, cleanRequestedPath)
	containedPath, err := filepath.Rel(workspace, resolvedPath)
	if err != nil {
		return "", relativePath, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Could not validate the requested path.", relativePath, err)
	}
	if containedPath == ".." || strings.HasPrefix(containedPath, ".."+string(filepath.Separator)) {
		return "", relativePath, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Path is outside the selected workspace.", relativePath, nil)
	}

	if err := rejectSymlinkPath(workspace, containedPath); err != nil {
		return "", relativePath, err
	}

	fileInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", relativePath, newWorkspaceError("FILE_NOT_FOUND", "File was not found.", relativePath, err)
		}
		return "", relativePath, newWorkspaceError("READ_FAILED", "Could not inspect the requested file.", relativePath, err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return "", relativePath, newWorkspaceError("SYMLINK_NOT_SUPPORTED", "Symbolic links are not supported in Version 1.", relativePath, nil)
	}
	if !fileInfo.Mode().IsRegular() {
		return "", relativePath, newWorkspaceError("UNSUPPORTED_FILE", "Path does not identify a regular file.", relativePath, nil)
	}
	if fileInfo.Size() > maximumFileSize {
		return "", relativePath, newWorkspaceError("FILE_TOO_LARGE", "File exceeds the 1 MiB Version 1 limit.", relativePath, nil)
	}

	return resolvedPath, relativePath, nil
}

func rejectSymlinkPath(workspace string, relativePath string) error {
	currentPath := workspace
	parts := strings.Split(filepath.Clean(relativePath), string(filepath.Separator))

	for _, part := range parts {
		currentPath = filepath.Join(currentPath, part)
		fileInfo, err := os.Lstat(currentPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return newWorkspaceError("FILE_NOT_FOUND", "File was not found.", filepath.ToSlash(relativePath), err)
			}
			return newWorkspaceError("READ_FAILED", "Could not inspect the requested path.", filepath.ToSlash(relativePath), err)
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return newWorkspaceError("SYMLINK_NOT_SUPPORTED", "Symbolic links are not supported in Version 1.", filepath.ToSlash(relativePath), nil)
		}
	}

	return nil
}

func newWorkspaceError(code string, message string, path string, cause error) error {
	return &workspaceError{
		Code:    code,
		Message: message,
		Path:    filepath.ToSlash(path),
		Cause:   cause,
	}
}

func isSupportedText(content []byte) bool {
	return utf8.Valid(content) && bytes.IndexByte(content, 0) < 0
}
