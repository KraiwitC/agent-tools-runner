package runner

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
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
	var files []string
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
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	if len(files) == 0 {
		return []SearchMatch{}, false, nil
	}

	workerCount := runtime.NumCPU()
	if workerCount > len(files) {
		workerCount = len(files)
	}

	jobs := make(chan string, len(files))
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	var mu sync.Mutex
	var allMatches []SearchMatch
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				fileMatches, err := searchFile(workspace, path, query, maximumSearchMatches+1)
				if err != nil || len(fileMatches) == 0 {
					continue
				}
				mu.Lock()
				allMatches = append(allMatches, fileMatches...)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(allMatches, func(i, j int) bool {
		if allMatches[i].Path != allMatches[j].Path {
			return allMatches[i].Path < allMatches[j].Path
		}
		return allMatches[i].Line < allMatches[j].Line
	})

	truncated := len(allMatches) > maximumSearchMatches
	if truncated {
		allMatches = allMatches[:maximumSearchMatches]
	}

	return allMatches, truncated, nil
}

func searchFile(workspace string, path string, query string, remainingMatches int) ([]SearchMatch, error) {
	if remainingMatches <= 0 {
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
