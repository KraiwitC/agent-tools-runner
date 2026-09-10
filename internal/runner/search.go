package runner

import (
	"bufio"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maximumCollectedSearchMatches = 1000
const scannerInitialBufferSize = 64 * 1024

var defaultSearchExcludedDirectories = map[string]struct{}{
	".git":         {},
	".idea":        {},
	"node_modules": {},
	"target":       {},
	"build":        {},
	"dist":         {},
	"vendor":       {},
	".gradle":      {},
	".m2":          {},
	".mvn":         {},
	"out":          {},
	"bin":          {},
	".metadata":    {},
	".settings":    {},
	".venv":        {},
	"venv":         {},
	"__pycache__":  {},
	".next":        {},
	".turbo":       {},
	".cache":       {},
	"coverage":     {},
	".output":      {},
	".terraform":   {},
}

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
	collectionLimit := maximumCollectedSearchMatches + 1
	matches := make([]SearchMatch, 0, collectionLimit)
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
		if entry.IsDir() && isSearchExcludedDirectory(entry.Name()) {
			return filepath.SkipDir
		}

		relativePath, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		normalizedPath := filepath.ToSlash(relativePath)
		if strings.Contains(normalizedPath, query) {
			matches = append(matches, SearchMatch{
				Path:        normalizedPath,
				MatchTarget: "path",
			})
			if len(matches) >= collectionLimit {
				return fs.SkipAll
			}
		}

		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		remainingMatches := collectionLimit - len(matches)
		fileMatches, err := searchFile(workspace, path, query, remainingMatches)
		if err != nil {
			return err
		}
		matches = append(matches, fileMatches...)
		if len(matches) >= collectionLimit {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	truncated := len(matches) > maximumCollectedSearchMatches
	if truncated {
		matches = matches[:maximumCollectedSearchMatches]
	}
	return matches, truncated, nil
}

func searchFile(workspace string, path string, query string, matchLimit int) ([]SearchMatch, error) {
	if matchLimit <= 0 {
		return nil, nil
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
				Path:        filepath.ToSlash(relativePath),
				Line:        lineNumber,
				Text:        line,
				MatchTarget: "content",
			})
			if len(matches) >= matchLimit {
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
