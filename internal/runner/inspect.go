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
)

func executeInspectAction(workspace string, action Action, actionIndex int) (*ActionData, *ResponseError) {
	resolvedPath, relativePath, err := resolveWorkspaceInspectPath(workspace, action.Path)
	if err != nil {
		return &ActionData{Path: relativePath}, workspaceInspectResponseError(action, actionIndex, err)
	}

	fileInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		return &ActionData{Path: relativePath}, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "READ_FAILED",
			Message:     "Could not inspect the requested path.",
			Path:        relativePath,
		}
	}

	if fileInfo.IsDir() {
		empty, err := isDirectoryEmpty(resolvedPath)
		if err != nil {
			return &ActionData{Path: relativePath, Type: "directory"}, &ResponseError{
				ActionID:    action.ID,
				ActionIndex: actionIndex,
				Code:        "READ_FAILED",
				Message:     "Could not inspect the requested directory.",
				Path:        relativePath,
			}
		}
		return &ActionData{
			Path:  relativePath,
			Type:  "directory",
			Empty: &empty,
		}, nil
	}

	content, err := readInspectFile(resolvedPath, relativePath)
	if err != nil {
		return &ActionData{Path: relativePath, Type: "file"}, workspaceInspectResponseError(action, actionIndex, err)
	}

	return &ActionData{
		Path:      relativePath,
		Type:      "file",
		SizeBytes: int64(len(content)),
		LineCount: countFileLines(content),
		SHA256:    calculateSHA256(content),
	}, nil
}

func resolveWorkspaceInspectPath(workspace string, requestedPath string) (string, string, error) {
	cleanRequestedPath := filepath.Clean(requestedPath)
	relativePath := filepath.ToSlash(cleanRequestedPath)

	if filepath.IsAbs(cleanRequestedPath) || filepath.VolumeName(cleanRequestedPath) != "" {
		return "", relativePath, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Absolute and volume-qualified paths are not allowed.", relativePath, nil)
	}

	resolvedPath := filepath.Join(workspace, cleanRequestedPath)
	containedPath, err := filepath.Rel(workspace, resolvedPath)
	if err != nil {
		return "", relativePath, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Could not validate the requested path.", relativePath, err)
	}
	if containedPath == ".." || strings.HasPrefix(containedPath, ".."+string(filepath.Separator)) {
		return "", relativePath, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Path is outside the selected workspace.", relativePath, nil)
	}
	if containedPath != "." {
		if err := rejectSymlinkPath(workspace, containedPath); err != nil {
			return "", relativePath, err
		}
	}

	fileInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", relativePath, newWorkspaceError("FILE_NOT_FOUND", "Path was not found.", relativePath, err)
		}
		return "", relativePath, newWorkspaceError("READ_FAILED", "Could not inspect the requested path.", relativePath, err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return "", relativePath, newWorkspaceError("SYMLINK_NOT_SUPPORTED", "Symbolic links are not supported in Version 1.", relativePath, nil)
	}
	if !fileInfo.IsDir() && !fileInfo.Mode().IsRegular() {
		return "", relativePath, newWorkspaceError("UNSUPPORTED_FILE", "Path is not a regular file or directory.", relativePath, nil)
	}
	if fileInfo.Mode().IsRegular() && fileInfo.Size() > maximumFileSize {
		return "", relativePath, newWorkspaceError("FILE_TOO_LARGE", "File exceeds the 1 MiB Version 1 limit.", relativePath, nil)
	}

	return resolvedPath, relativePath, nil
}

func readInspectFile(resolvedPath string, relativePath string) ([]byte, error) {
	file, err := os.Open(resolvedPath)
	if err != nil {
		return nil, newWorkspaceError("READ_FAILED", "Could not open the requested file.", relativePath, err)
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maximumFileSize+1))
	if err != nil {
		return nil, newWorkspaceError("READ_FAILED", "Could not read the requested file.", relativePath, err)
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

func countFileLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	lineCount := bytes.Count(content, []byte{'\n'})
	if content[len(content)-1] != '\n' {
		lineCount++
	}
	return lineCount
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

func workspaceInspectResponseError(action Action, actionIndex int, err error) *ResponseError {
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
		Code:        "READ_FAILED",
		Message:     "Could not inspect the requested path.",
		Path:        filepath.ToSlash(filepath.Clean(action.Path)),
	}
}
