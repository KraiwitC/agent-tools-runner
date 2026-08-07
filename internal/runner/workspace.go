package runner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
			return files, workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not read the requested file.", filepath.ToSlash(requestedPath))
		}

		files = append(files, file)
	}

	return files, nil
}

func readWorkspaceFile(workspace string, requestedPath string) (ReadFileResult, error) {
	resolvedPath, relativePath, _, err := resolveWorkspaceFile(workspace, requestedPath)
	if err != nil {
		return ReadFileResult{}, err
	}

	content, err := readTextFile(resolvedPath, relativePath, "Could not open the requested file.", "Could not read the requested file.")
	if err != nil {
		return ReadFileResult{}, err
	}

	return ReadFileResult{
		Path:    relativePath,
		Content: string(content),
		SHA256:  calculateSHA256(content),
	}, nil
}

func resolveWorkspaceFile(workspace string, requestedPath string) (string, string, os.FileInfo, error) {
	cleanRequestedPath := filepath.Clean(requestedPath)
	relativePath := filepath.ToSlash(cleanRequestedPath)

	if filepath.IsAbs(cleanRequestedPath) || filepath.VolumeName(cleanRequestedPath) != "" {
		return "", relativePath, nil, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Absolute and volume-qualified paths are not allowed.", relativePath, nil)
	}
	if cleanRequestedPath == "." || cleanRequestedPath == "" {
		return "", relativePath, nil, newWorkspaceError("UNSUPPORTED_FILE", "Path must identify a regular file.", relativePath, nil)
	}

	resolvedPath := filepath.Join(workspace, cleanRequestedPath)
	containedPath, err := filepath.Rel(workspace, resolvedPath)
	if err != nil {
		return "", relativePath, nil, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Could not validate the requested path.", relativePath, err)
	}
	if containedPath == ".." || strings.HasPrefix(containedPath, ".."+string(filepath.Separator)) {
		return "", relativePath, nil, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Path is outside the selected workspace.", relativePath, nil)
	}

	if err := rejectSymlinkParents(workspace, containedPath); err != nil {
		return "", relativePath, nil, err
	}

	fileInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", relativePath, nil, newWorkspaceError("FILE_NOT_FOUND", "File was not found.", relativePath, err)
		}
		return "", relativePath, nil, newWorkspaceError("READ_FAILED", "Could not inspect the requested file.", relativePath, err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return "", relativePath, nil, newWorkspaceError("SYMLINK_NOT_SUPPORTED", "Symbolic links are not supported in Version 1.", relativePath, nil)
	}
	if !fileInfo.Mode().IsRegular() {
		return "", relativePath, nil, newWorkspaceError("UNSUPPORTED_FILE", "Path does not identify a regular file.", relativePath, nil)
	}
	if fileInfo.Size() > maximumFileSize {
		return "", relativePath, nil, newWorkspaceError("FILE_TOO_LARGE", "File exceeds the 1 MiB Version 1 limit.", relativePath, nil)
	}

	return resolvedPath, relativePath, fileInfo, nil
}

func readTextFile(path string, relativePath string, openMessage string, readMessage string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, newWorkspaceError("FILE_NOT_FOUND", "File was not found.", relativePath, err)
		}
		return nil, newWorkspaceError("READ_FAILED", openMessage, relativePath, err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maximumFileSize+1))
	if err != nil {
		return nil, newWorkspaceError("READ_FAILED", readMessage, relativePath, err)
	}
	if int64(len(content)) > maximumFileSize {
		return nil, newWorkspaceError("FILE_TOO_LARGE", "File exceeds the 1 MiB Version 1 limit.", relativePath, nil)
	}
	if !isSupportedText(content) {
		return nil, newWorkspaceError("UNSUPPORTED_FILE", "File is not supported UTF-8 text.", relativePath, nil)
	}
	return content, nil
}

func calculateSHA256(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func isDirectoryEmpty(path string) (bool, error) {
	directory, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer directory.Close()

	_, err = directory.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

func rejectSymlinkParents(workspace string, relativePath string) error {
	parentPath := filepath.Dir(filepath.Clean(relativePath))
	if parentPath == "." {
		return nil
	}

	currentPath := workspace
	for _, part := range strings.Split(parentPath, string(filepath.Separator)) {
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

func workspaceResponseError(action Action, actionIndex int, err error, fallbackCode string, fallbackMessage string, fallbackPath string) *ResponseError {
	var pathError *workspaceError
	if errors.As(err, &pathError) {
		return newActionResponseError(action, actionIndex, pathError.Code, pathError.Message, pathError.Path)
	}
	return newActionResponseError(action, actionIndex, fallbackCode, fallbackMessage, fallbackPath)
}

func newActionResponseError(action Action, actionIndex int, code string, message string, path string) *ResponseError {
	return &ResponseError{
		ActionID:    action.ID,
		ActionIndex: actionIndex,
		Code:        code,
		Message:     message,
		Path:        path,
	}
}

func isSupportedText(content []byte) bool {
	return utf8.Valid(content) && bytes.IndexByte(content, 0) < 0
}
