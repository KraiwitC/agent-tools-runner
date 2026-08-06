package runner

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maximumSearchMatches = 100
const scannerInitialBufferSize = 64 * 1024

var defaultSearchExcludedDirectories = map[string]struct{}{
	".git":         {},
	".idea":        {},
	"node_modules": {},
	"target":       {},
	"build":        {},
	"dist":         {},
	"vendor":       {},
}

var errSearchLimitReached = errors.New("search match limit reached")

func executeSearchAction(workspace string, action Action, actionIndex int) ([]SearchMatch, bool, *ResponseError) {
	matches, truncated, err := searchWorkspace(workspace, action.Query)
	if err != nil {
		return matches, truncated, &ResponseError{
			ActionID:    action.ID,
			ActionIndex: actionIndex,
			Code:        "SEARCH_FAILED",
			Message:     "Could not search the selected workspace.",
		}
	}

	return matches, truncated, nil
}

func searchWorkspace(workspace string, query string) ([]SearchMatch, bool, error) {
	matches := make([]SearchMatch, 0)
	truncated := false

	err := filepath.WalkDir(workspace, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == workspace {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if isSearchExcludedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		fileMatches, err := searchFile(workspace, path, query, maximumSearchMatches+1-len(matches))
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		if len(matches) > maximumSearchMatches {
			matches = matches[:maximumSearchMatches]
			truncated = true
			return errSearchLimitReached
		}

		return nil
	})
	if errors.Is(err, errSearchLimitReached) {
		return matches, truncated, nil
	}
	if err != nil {
		return matches, truncated, err
	}

	return matches, truncated, nil
}

func searchFile(workspace string, path string, query string, remainingMatches int) ([]SearchMatch, error) {
	if remainingMatches <= 0 {
		return nil, nil
	}

	fileInfo, err := os.Lstat(path)
	if err != nil || fileInfo.Size() > maximumFileSize {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maximumFileSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maximumFileSize || !utf8.Valid(content) || bytes.IndexByte(content, 0) >= 0 {
		return nil, nil
	}

	relativePath, err := filepath.Rel(workspace, path)
	if err != nil {
		return nil, err
	}

	matches := make([]SearchMatch, 0)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, scannerInitialBufferSize), int(maximumFileSize))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if strings.Contains(line, query) {
			matches = append(matches, SearchMatch{
				Path: filepath.ToSlash(relativePath),
				Line: lineNumber,
				Text: line,
			})
			if len(matches) >= remainingMatches {
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

func isSearchExcludedDirectory(name string) bool {
	_, excluded := defaultSearchExcludedDirectories[name]
	return excluded
}
