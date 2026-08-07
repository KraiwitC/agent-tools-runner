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
	resolvedPath, relativePath, fileInfo, err := resolveWorkspaceInspectPath(workspace, action.Path)
	if err != nil {
		return &ActionData{Path: relativePath}, workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not inspect the requested path.", filepath.ToSlash(filepath.Clean(action.Path)))
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

	content, err := readTextFile(resolvedPath, relativePath, "Could not open the requested file.", "Could not read the requested file.")
	if err != nil {
		return &ActionData{Path: relativePath, Type: "file"}, workspaceResponseError(action, actionIndex, err, "READ_FAILED", "Could not inspect the requested path.", filepath.ToSlash(filepath.Clean(action.Path)))
	}

	return &ActionData{
		Path:      relativePath,
		Type:      "file",
		SizeBytes: int64(len(content)),
		LineCount: countFileLines(content),
		SHA256:    calculateSHA256(content),
	}, nil
}

func resolveWorkspaceInspectPath(workspace string, requestedPath string) (string, string, os.FileInfo, error) {
	cleanRequestedPath := filepath.Clean(requestedPath)
	relativePath := filepath.ToSlash(cleanRequestedPath)

	if filepath.IsAbs(cleanRequestedPath) || filepath.VolumeName(cleanRequestedPath) != "" {
		return "", relativePath, nil, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Absolute and volume-qualified paths are not allowed.", relativePath, nil)
	}

	resolvedPath := filepath.Join(workspace, cleanRequestedPath)
	containedPath, err := filepath.Rel(workspace, resolvedPath)
	if err != nil {
		return "", relativePath, nil, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Could not validate the requested path.", relativePath, err)
	}
	if containedPath == ".." || strings.HasPrefix(containedPath, ".."+string(filepath.Separator)) {
		return "", relativePath, nil, newWorkspaceError("PATH_OUTSIDE_WORKSPACE", "Path is outside the selected workspace.", relativePath, nil)
	}
	if containedPath != "." {
		if err := rejectSymlinkParents(workspace, containedPath); err != nil {
			return "", relativePath, nil, err
		}
	}

	fileInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", relativePath, nil, newWorkspaceError("FILE_NOT_FOUND", "Path was not found.", relativePath, err)
		}
		return "", relativePath, nil, newWorkspaceError("READ_FAILED", "Could not inspect the requested path.", relativePath, err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		return "", relativePath, nil, newWorkspaceError("SYMLINK_NOT_SUPPORTED", "Symbolic links are not supported in Version 1.", relativePath, nil)
	}
	if !fileInfo.IsDir() && !fileInfo.Mode().IsRegular() {
		return "", relativePath, nil, newWorkspaceError("UNSUPPORTED_FILE", "Path is not a regular file or directory.", relativePath, nil)
	}
	if fileInfo.Mode().IsRegular() && fileInfo.Size() > maximumFileSize {
		return "", relativePath, nil, newWorkspaceError("FILE_TOO_LARGE", "File exceeds the 1 MiB Version 1 limit.", relativePath, nil)
	}

	return resolvedPath, relativePath, fileInfo, nil
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
