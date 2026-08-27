package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const maximumCollectedTreeEntries = 2000

func executeTreeAction(workspace string, action Action, actionIndex int) ([]TreeEntry, string, bool, *ResponseError) {
	resolvedPath, relativePath, err := resolveWorkspaceDirectory(workspace, action.Path)
	if err != nil {
		return nil, relativePath, false, workspaceResponseError(action, actionIndex, err, "TREE_FAILED", "Could not prepare the requested project tree.", filepath.ToSlash(filepath.Clean(action.Path)))
	}

	entries, truncated, err := readWorkspaceTree(workspace, resolvedPath)
	if err != nil {
		return entries, relativePath, truncated, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "TREE_FAILED",
			Message:     "Could not read the requested project tree.",
			Path:        relativePath,
		}
	}

	return entries, relativePath, truncated, nil
}

func resolveWorkspaceDirectory(workspace string, requestedPath string) (string, string, error) {
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

	if containedPath != "." {
		if err := rejectSymlinkParents(workspace, containedPath); err != nil {
			return "", relativePath, err
		}
	}

	directoryInfo, err := os.Lstat(resolvedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", relativePath, newWorkspaceError(
				"FILE_NOT_FOUND",
				"Directory was not found.",
				relativePath,
				err,
			)
		}
		return "", relativePath, newWorkspaceError(
			"TREE_FAILED",
			"Could not inspect the requested directory.",
			relativePath,
			err,
		)
	}
	if directoryInfo.Mode()&os.ModeSymlink != 0 {
		return "", relativePath, newWorkspaceError(
			"SYMLINK_NOT_SUPPORTED",
			"Symbolic links are not supported.",
			relativePath,
			nil,
		)
	}
	if !directoryInfo.IsDir() {
		return "", relativePath, newWorkspaceError(
			"UNSUPPORTED_FILE",
			"Path must identify a directory.",
			relativePath,
			nil,
		)
	}

	return resolvedPath, relativePath, nil
}

func readWorkspaceTree(workspace string, root string) ([]TreeEntry, bool, error) {
	entries := make([]TreeEntry, 0)
	directoryQueue := []string{root}

	for len(directoryQueue) > 0 {
		currentDirectory := directoryQueue[0]
		directoryQueue = directoryQueue[1:]

		directoryEntries, err := os.ReadDir(currentDirectory)
		if err != nil {
			return entries, false, err
		}

		for _, entry := range directoryEntries {
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			if entry.IsDir() && isSearchExcludedDirectory(entry.Name()) {
				continue
			}
			if !entry.IsDir() && !entry.Type().IsRegular() {
				continue
			}

			if len(entries) >= maximumCollectedTreeEntries {
				return entries, true, nil
			}

			path := filepath.Join(currentDirectory, entry.Name())
			relativePath, err := filepath.Rel(workspace, path)
			if err != nil {
				return entries, false, err
			}

			entryType := "file"
			if entry.IsDir() {
				entryType = "directory"
				directoryQueue = append(directoryQueue, path)
			}
			entries = append(entries, TreeEntry{
				Path: filepath.ToSlash(relativePath),
				Type: entryType,
			})
		}
	}

	return entries, false, nil
}
